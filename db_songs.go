package main

import (
	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/search"
	"gorm.io/gorm"
	"log"
)

func (g *DbHandler) GetSongs() []models.Song {
	songs := []models.Song{}
	err := inits.DB.Order("number").Find(&songs).Error
	if err != nil {
		log.Println("Error searching songs", err.Error())
		return nil
	}

	if len(songs) != 0 {
		couplets, err := g.getCouplets(songs[0].ID)
		if err != nil {
			log.Println("Error getting couplets for first song", err.Error())
			return nil
		}
		songs[0].Couplets = couplets
	}

	return songs
}

func (g *DbHandler) CreateSong(number int, title string) *models.Song {
	dbSong := models.Song{Number: number, Title: title}
	err := inits.DB.Create(&dbSong).Error
	if err != nil {
		log.Println("Error creating song", err.Error())
		return nil
	}

	songs := g.GetSongs()
	g.emit("songs_update", songs)
	return &dbSong
}

func (g *DbHandler) RemoveSong(songId int) {
	song := models.Song{}
	if err := inits.DB.Find(&song, songId).Error; err != nil {
		log.Println("Error finding song", err.Error())
		return
	}

	if g.coupletB.state != nil && g.coupletB.state.Song.ID == uint(songId) {
		g.hideCoupletInternal()
	}

	// Идентификаторы нужны поисковому индексу, а после удаления взять их
	// будет негде — собираем заранее. Ошибка здесь означала бы пустой список и
	// документы, зависшие в индексе до ручной пересборки, поэтому не удаляем.
	var coupletIds []uint
	if err := inits.DB.Model(&models.Couplet{}).Where("song_id = ?", songId).Pluck("id", &coupletIds).Error; err != nil {
		log.Println("Error collecting couplet ids", err.Error())
		return
	}

	if err := inits.DB.Where("song_id = ?", songId).Delete(&models.Couplet{}).Error; err != nil {
		log.Println("Error deleting couplets for song", err.Error())
		return
	}
	g.unindexSilent(search.KindCouplet, coupletIds...)

	if err := inits.DB.Delete(&models.Song{}, songId).Error; err != nil {
		log.Println("Error deleting song", err.Error())
		return
	}

	songs := g.GetSongs()
	g.emit("songs_update", songs)
}

func (g *DbHandler) getCouplets(songId uint) ([]models.Couplet, error) {
	return findByParent[models.Couplet]("song_id", songId, "number ASC")
}

func (g *DbHandler) GetCouplets(songId float32) []models.Couplet {
	couplets, err := g.getCouplets(uint(songId))
	if err != nil {
		log.Println("Error getting couplets")
		return nil
	}

	return couplets
}

func (g *DbHandler) ShowCouplet(coupletFloatId float32) *ShownCouplet {
	coupletId := uint(coupletFloatId)

	couplet := &models.Couplet{}
	err := inits.DB.First(couplet, coupletId).Error
	if err != nil {
		log.Println("Error showing couplet", err.Error())
		return nil
	}

	song := &models.Song{}
	err = inits.DB.First(song, couplet.SongId).Error
	if err != nil {
		log.Println("Error showing song", err.Error())
		return nil
	}

	g.showCoupletInternal(&ShownCouplet{
		Couplet: *couplet,
		Song:    *song,
	})

	return g.coupletB.state
}

// reloadAndEmitSong re-reads a song with its couplets ordered by number and
// pushes the fresh state to the operator UI. Shared by the couplet mutations.
func (g *DbHandler) reloadAndEmitSong(songId uint) {
	song := models.Song{}
	if err := inits.DB.Preload("Couplets", addAscByNumber).Find(&song, songId).Error; err != nil {
		log.Println("Error getting new song state", err.Error())
		return
	}
	g.emit("song_update", song)
}

func (g *DbHandler) CreateCouplet(text, label string, number, songId uint) {
	couplet := models.Couplet{
		Text:   text,
		Number: int(number),
		Label:  label,
		SongId: songId,
	}

	if err := inits.DB.Model(&models.Couplet{}).Where(
		"song_id = ? AND number >= ?", songId, number,
	).Update(
		"number", gorm.Expr("number + 1"),
	).Error; err != nil {
		log.Println("Error updating couplet numbers", err.Error())
		return
	}

	// Выходим при ошибке: без этого дальше индексировался бы куплет с нулевым
	// идентификатором — документ «couplet:0» с настоящим текстом, который потом
	// находится поиском, занимает место в выдаче и молча отбрасывается при
	// сборке результата. Убрать его может только полная пересборка индекса.
	if err := inits.DB.Create(&couplet).Error; err != nil {
		log.Println("Error creating couplet", err.Error())
		return
	}
	g.indexCoupletSilent(couplet)

	g.reloadAndEmitSong(songId)
}

func (g *DbHandler) UpdateCouplet(coupletId int, label string, text string, number int) {
	couplet := &models.Couplet{}
	err := inits.DB.First(couplet, coupletId).Error
	if err != nil {
		log.Println("Error getting couplet", err.Error())
		return
	}

	couplet.Label = label
	couplet.Text = text
	couplet.Number = int(number)

	if err := inits.DB.Save(couplet).Error; err != nil {
		log.Println("Error updating couplet", err.Error())
		return
	}
	g.indexCoupletSilent(*couplet)

	g.reloadAndEmitSong(couplet.SongId)
}

func (g *DbHandler) ReplaceCouplets(songId int, blocks []CoupletInput) {
	song := models.Song{}
	if err := inits.DB.Find(&song, songId).Error; err != nil {
		log.Println("Error finding song", err.Error())
		return
	}

	if g.coupletB.state != nil && g.coupletB.state.Song.ID == uint(songId) {
		g.hideCoupletInternal()
	}

	// Замена блоков пересоздаёт куплеты с новыми идентификаторами, поэтому
	// старые документы надо убрать из индекса адресно — по списку, снятому до
	// транзакции. Порядок «сначала снять список, потом удалять» безопасен ещё и
	// потому, что удаление здесь мягкое (gorm.Model): строки остаются, номера не
	// переиспользуются и новый куплет не может получить идентификатор старого.
	var oldIds []uint
	if err := inits.DB.Model(&models.Couplet{}).Where("song_id = ?", songId).Pluck("id", &oldIds).Error; err != nil {
		log.Println("Error collecting couplet ids", err.Error())
		return
	}

	err := inits.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("song_id = ?", songId).Delete(&models.Couplet{}).Error; err != nil {
			return err
		}
		for i, b := range blocks {
			c := models.Couplet{
				Text:   b.Text,
				Label:  b.Label,
				Number: i + 1,
				SongId: uint(songId),
			}
			if err := tx.Create(&c).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Println("Error replacing couplets", err.Error())
		return
	}
	g.unindexSilent(search.KindCouplet, oldIds...)
	g.reindexSongSilent(uint(songId))

	g.reloadAndEmitSong(uint(songId))
}

func (g *DbHandler) RemoveCouplet(coupletId int) {
	couplet := models.Couplet{}
	if err := inits.DB.Find(&couplet, coupletId).Error; err != nil {
		log.Println("Error finding couplet", err.Error())
		return
	}

	if err := inits.DB.Delete(&models.Couplet{}, coupletId).Error; err != nil {
		log.Println("Error deleting couplet", err.Error())
		return
	}
	g.unindexSilent(search.KindCouplet, uint(coupletId))

	song := models.Song{}
	if err := inits.DB.Preload("Couplets", addAscByNumber).Find(&song, couplet.SongId).Error; err != nil {
		log.Println("Error getting new song state", err.Error())
		return
	}
	for i, c := range song.Couplets {
		c.Number = i + 1
		if err := inits.DB.Save(&c).Error; err != nil {
			log.Println("Error updating couplet number", err.Error())
			return
		}
	}
	g.emit("song_update", song)
}

func (g *DbHandler) GetShownCouplet() *ShownCouplet {
	return g.coupletB.state
}

func (g *DbHandler) HideCouplet() {
	g.hideCoupletInternal()
}

func (g *DbHandler) ShowQR() {
	r := true
	g.qr <- &r
}

func (g *DbHandler) HideQR() {
	r := false
	g.qr <- &r
}
