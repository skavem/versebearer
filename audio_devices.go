package main

import (
	"fmt"

	"changeme/backend/inits"
	"changeme/backend/models"

	"github.com/gen2brain/malgo"
)

// AudioDeviceFormat — одна поддерживаемая устройством комбинация
// частота/каналы/формат сэмплов (правка 4: модалка выбора устройства вместо
// выпадашки, "максимум информации по каждому"). malgo.DataFormat отдаёт их
// как есть, без агрегации — оператору интереснее видеть все поддерживаемые
// частоты, чем одну "типичную".
type AudioDeviceFormat struct {
	SampleRate int    `json:"sampleRate"`
	Channels   int    `json:"channels"`
	Format     string `json:"format"` // человекочитаемо, см. formatTypeName
}

// AudioDevice — один пункт списка устройств вывода вкладки «Звук» (этап 3).
// ID — malgo DeviceID.String(); "" зарезервирована за системным по
// умолчанию и ListDevices всегда ставит её первой записью.
type AudioDevice struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	// Missing — устройство было выбрано (GlobalState.AudioDeviceId), но не
	// нашлось в свежем перечислении: выдернули, отключили в системе и т.п.
	// Сопоставление только по ID.String() — см. defaultOpenDevice.
	Missing bool `json:"missing"`
	// Formats — правка 4: поддерживаемые устройством форматы. Пусто, если
	// перечисление их не дало (см. defaultListPlaybackDevices) — фронт тогда
	// просто не показывает эти поля, а не рисует "0 Гц".
	Formats []AudioDeviceFormat `json:"formats,omitempty"`
	// ActiveSampleRate — частота, на которой устройство РЕАЛЬНО открыто
	// сейчас (0, если это не то устройство, что сейчас открыто плеером, или
	// плеер вообще ничего ещё не открывал в этой сессии). Отдельно от
	// Formats: то, что устройство ПОДДЕРЖИВАЕТ, и то, на чём оно реально
	// заиграло в shared-режиме WASAPI, может отличаться (план: "на машине
	// пользователя это 96000 Гц — полезно видеть").
	ActiveSampleRate int `json:"activeSampleRate,omitempty"`
}

// formatTypeName — человекочитаемое имя malgo.FormatType для модалки выбора
// устройства (правка 4).
func formatTypeName(f malgo.FormatType) string {
	switch f {
	case malgo.FormatU8:
		return "8 бит"
	case malgo.FormatS16:
		return "16 бит"
	case malgo.FormatS24:
		return "24 бита"
	case malgo.FormatS32:
		return "32 бита"
	case malgo.FormatF32:
		return "32 бита (float)"
	default:
		return "неизвестный формат"
	}
}

// deviceFormats конвертирует malgo.DataFormat в AudioDeviceFormat. nil, если
// подробностей нет вовсе — отличаем "нет данных" от "пустой список" на
// уровне JSON через omitempty, а не гадаем на фронте по длине.
func deviceFormats(info malgo.DeviceInfo) []AudioDeviceFormat {
	if len(info.Formats) == 0 {
		return nil
	}
	out := make([]AudioDeviceFormat, 0, len(info.Formats))
	for _, f := range info.Formats {
		out = append(out, AudioDeviceFormat{
			SampleRate: int(f.SampleRate),
			Channels:   int(f.Channels),
			Format:     formatTypeName(f.Format),
		})
	}
	return out
}

// ListDevices перечисляет устройства вывода. Системное по умолчанию — всегда
// первым пунктом, независимо от того, что вернул malgo. Если сейчас выбрано
// (SetDevice/GlobalState) устройство, которого нет в свежем списке — в конец
// добавляется запись с Missing: true, чтобы UI мог показать «выбранное
// устройство пропало», а не молча забыть о выборе оператора.
func (a *AudioService) ListDevices() []AudioDevice {
	selected := a.GetDeviceId()
	devices := []AudioDevice{{ID: "", Name: systemDefaultDeviceName, IsDefault: true}}

	infos, err := a.pl.listPlaybackDevices()
	if err != nil {
		// Отсутствие звуковой карты/бэкенда — не повод падать: список из
		// одного системного пункта достаточен, чтобы вкладка «Звук» вообще
		// открылась (план: старт на машине без звуковой карты не должен
		// ломаться).
		infos = nil
	}

	open, openId, openRate := a.pl.openDeviceSnapshot()

	foundSelected := selected == ""
	for _, info := range infos {
		id := info.ID.String()
		if id == selected {
			foundSelected = true
		}
		device := AudioDevice{
			ID:        id,
			Name:      info.Name(),
			IsDefault: info.IsDefault != 0,
			Formats:   deviceFormats(info),
		}
		if open && openId == id {
			device.ActiveSampleRate = int(openRate)
		}
		devices = append(devices, device)
		if info.IsDefault != 0 {
			// «Системное по умолчанию» (ID=="") физически — то же самое
			// устройство: те же поддерживаемые форматы, и та же фактическая
			// частота, если плеер сейчас открыт именно КАК "по умолчанию"
			// (openId=="", а не как это же устройство, выбранное явно по id).
			devices[0].Formats = device.Formats
			if open && openId == "" {
				devices[0].ActiveSampleRate = int(openRate)
			}
		}
	}
	if !foundSelected {
		devices = append(devices, AudioDevice{ID: selected, Missing: true})
	}
	return devices
}

// GetDeviceId возвращает текущий выбор (malgo DeviceID.String(), "" —
// системное по умолчанию).
func (a *AudioService) GetDeviceId() string {
	return a.pl.selectedDevice()
}

// SetDevice меняет устройство вывода. Смена ВСЕГДА останавливает
// воспроизведение (план, этап 3, осознанное решение) — продолжение с той же
// позиции на другом устройстве не входит в первую версию, и попытка
// «бесшовно» перевести играющий трек добавила бы риска ради сценария,
// которого никто не просил. Устройство закрывается (closeDevice, вне p.mu —
// И2), чтобы следующий Play() поднял именно новое через ensureDevice, а не
// продолжил молча играть на старом.
func (a *AudioService) SetDevice(id string) error {
	// HIGH №3 обзора: модалка выбора устройства подсвечивает карточку
	// текущего выбора карточкой-кнопкой — клик по НЕЙ ЖЕ (посмотреть частоту,
	// не поменять устройство) раньше безусловно рвал звук в зале. Ранний
	// возврат защищает всех вызывающих разом (и модалку, и любой будущий
	// путь). Оговорка про deviceLost: устройство могло пропасть физически
	// (deviceLost==true) при том же id — тогда «выбрать то же самое» это
	// осознанная попытка оператора переоткрыть его, и её нельзя гасить
	// молча.
	if id == a.pl.selectedDevice() && !a.pl.deviceLost.Load() {
		return nil
	}

	// Порядок важен (MEDIUM №5 обзора): сначала обрываем звук физически
	// (немедленная тишина), и только потом ждём decodeExt заранее
	// подготовленного "следующего" — иначе оператор слышал бы недодоигранные
	// ~0.5с трека, пока dropPending блокируется на <-pn.ready.
	oldSrc := a.pl.stop()
	a.dropPending() // устройство меняется — то, что играло, обрывается физически (см. выше), считать следующий не для чего
	if oldSrc != nil {
		oldSrc.Close()
	}
	a.pl.closeDevice()
	a.pl.setSelectedDevice(id)

	if err := inits.DB.Model(&models.GlobalState{}).Where("id = ?", 1).Update("audio_device_id", id).Error; err != nil {
		return fmt.Errorf("не удалось сохранить выбор устройства: %w", err)
	}
	a.emit("audio_stopped", nil)
	return nil
}
