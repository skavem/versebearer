package inits

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"changeme/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestNoInitFunc — Р2 ревизии, заменяет неработающий сторож инварианта 1.
//
// Снимок пакетной переменной (`var dbAtStartup = DB` на уровне файла) НЕ
// ловит init(): согласно спецификации Go переменные уровня пакета
// инициализируются ДО любого init() этого пакета, так что такой снимок
// зелёный и при наличии init(), и без него — тест-сторож, который создаёт
// ложное ощущение защищённости у инварианта, ради которого затевалась
// половина рефакторинга. Поэтому здесь — разбор исходников пакета через
// go/parser: падает на любом top-level func init() в *.go каталога (кроме
// *_test.go, где init() — обычный тестовый хук, а не открытие базы).
func TestNoInitFunc(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("filepath.Glob(\"*.go\") ничего не нашёл — тест не проверяет то, что заявлено")
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		node, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.Name == "init" {
					t.Errorf("%s содержит func init() — импорт этого пакета не должен открывать базу; открытие обязано остаться явным вызовом Open()/OpenAt() из main()", f)
				}
			case *ast.GenDecl:
				// LOW обзора: "var _ = mustOpenDB()" на уровне пакета
				// нарушает тот же инвариант, что и func init() — Go
				// инициализирует package-level var ДО init(), так что
				// такая переменная исполняет свой вызов при простом импорте
				// пакета точно так же незаметно. Голая проверка на func
				// init() не ловит этот случай и создаёт ложное ощущение
				// защищённости — падаем на любом вызове функции внутри
				// инициализатора top-level var.
				if d.Tok != token.VAR {
					continue
				}
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, val := range vs.Values {
						ast.Inspect(val, func(n ast.Node) bool {
							if _, ok := n.(*ast.CallExpr); ok {
								t.Errorf("%s: инициализатор package-level var вызывает функцию — тот же инвариант, что и func init(): импорт пакета не должен исполнять код, который может открыть базу", f)
								return false
							}
							return true
						})
					}
				}
			}
		}
	}
}

// automigrateFullSchema прогоняет по db тот же список моделей, что и OpenAt,
// — полная схема, включая все таблицы из copyVerifyTables.
func automigrateFullSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&models.Translation{}, &models.Book{}, &models.Chapter{}, &models.Verse{},
		&models.Song{}, &models.Couplet{}, &models.Screen{}, &models.Theme{},
		&models.GlobalState{}, &models.Font{}, &models.Image{}, &models.Output{},
		&models.AudioTrack{}, &models.Playlist{}, &models.PlaylistItem{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
}

// mustCreate вставляет строку или валит тест. Сидинг из-за неё читается
// списком сущностей, а не чередой одинаковых проверок ошибки; what попадает в
// сообщение, чтобы упавшая вставка называла себя.
func mustCreate(t *testing.T, db *gorm.DB, what string, value any) {
	t.Helper()
	if err := db.Create(value).Error; err != nil {
		t.Fatalf("seed %s: %v", what, err)
	}
}

// openSQLite открывает (создавая, если файла нет) SQLite-базу по path — и для
// наполнения тестовыми данными, и для проверки результата переноса. Закрывать
// соединение остаётся на вызывающем: на Windows незакрытый хендл не даёт
// t.TempDir() убрать за собой каталог.
func openSQLite(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	return db
}

// buildLegacySourceDB создаёт по path валидный SQLite-файл с полной схемой и
// по одной строке-маркеру в песнях/куплетах и стихах/переводе, закрывая
// соединение перед возвратом — copyLegacyDB ожидает открыть источник сам.
func buildLegacySourceDB(t *testing.T, path string) {
	t.Helper()
	db := openSQLite(t, path)
	automigrateFullSchema(t, db)

	song := models.Song{Title: "Великий Бог", Number: 1}
	mustCreate(t, db, "song", &song)
	mustCreate(t, db, "couplet", &models.Couplet{SongId: song.ID, Number: 1, Text: "куплет-маркер", Label: "1"})

	translation := models.Translation{Name: "Синодальный", ShortName: "SND"}
	mustCreate(t, db, "translation", &translation)
	book := models.Book{Title: "Иоанна", ShortName: "Ин", Number: 43, TranslationId: translation.ID}
	mustCreate(t, db, "book", &book)
	chapter := models.Chapter{Number: 3, BookId: book.ID}
	mustCreate(t, db, "chapter", &chapter)
	mustCreate(t, db, "verse", &models.Verse{Number: 16, ChapterId: chapter.ID, Text: "стих-маркер"})
	mustCreate(t, db, "global state", &models.GlobalState{Version: "8"})

	closeGormDB(db)
}

// seedDestinationDB создаёт по dst готовую базу назначения — валидный SQLite со
// схемой GlobalState, ту, которую inspectDestination обязан признать годной.
// Непустой version добавляет строку-маркер с этой версией: так тест потом
// отличает нетронутую базу назначения от подменённой переносом.
func seedDestinationDB(t *testing.T, dst, version string) {
	t.Helper()
	db := openSQLite(t, dst)
	if err := db.AutoMigrate(&models.GlobalState{}); err != nil {
		t.Fatalf("automigrate dst: %v", err)
	}
	if version != "" {
		mustCreate(t, db, "marker", &models.GlobalState{Version: version})
	}
	closeGormDB(db)
}

// seedForeignPart кладёт рядом с dst огрызок `<dst>.part.<чужой pid>` возрастом
// age и возвращает его путь. Возраст задаётся явно, потому что от него зависит
// весь смысл проверки: sweepForeignParts подметает только то, что старше
// sweepPartMinAge, а более молодой файл может принадлежать живому соседу.
func seedForeignPart(t *testing.T, dst string, age time.Duration) string {
	t.Helper()
	foreign := fmt.Sprintf("%s.part.%d", dst, os.Getpid()+1)
	if err := os.WriteFile(foreign, []byte("чужой огрызок"), 0o644); err != nil {
		t.Fatalf("seed foreign .part: %v", err)
	}
	if age > 0 {
		stamp := time.Now().Add(-age)
		if err := os.Chtimes(foreign, stamp, stamp); err != nil {
			t.Fatalf("chtimes foreign .part: %v", err)
		}
	}
	return foreign
}

// TestMigrateFreshInstall — dst нет, кандидатов нет: migrateLegacyDB не
// создаёт файл и не переносит ничего, OpenAt на пустом пути заводит базу с
// GlobalState id=1 и version="8". Плейлист по умолчанию здесь не проверяется
// — он покрыт TestSeedDefaultPlaylistIdempotent (db_test.go).
func TestMigrateFreshInstall(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "versebearer.db")

	migratedFrom, err := migrateLegacyDB(dst, nil)
	if err != nil {
		t.Fatalf("migrateLegacyDB: %v", err)
	}
	if migratedFrom != "" {
		t.Errorf("migrateLegacyDB migratedFrom = %q, want \"\" (кандидатов нет)", migratedFrom)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Errorf("migrateLegacyDB создал файл на пустой установке: stat err = %v", statErr)
	}

	if err := OpenAt(dst); err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	// Закрыть перед тем, как t.TempDir() уберёт каталог — иначе на Windows
	// уборка временного каталога падает: "The process cannot access the
	// file because it is being used by another process."
	t.Cleanup(func() { closeGormDB(DB) })

	var gs models.GlobalState
	if err := DB.First(&gs, 1).Error; err != nil {
		t.Fatalf("GlobalState id=1: %v", err)
	}
	if gs.Version != "8" {
		t.Errorf("version = %q, want %q", gs.Version, "8")
	}

	// Единственное покрытие СОДЕРЖИМОГО блоков версии 4 и 5: db_test.go
	// проверяет только seedDefaultPlaylist и versionLT сами по себе, а
	// TestOpenAtOnUpgradedDBIsNoOp — что повторный OpenAt ничего не меняет.
	// Ниже — то, что эта охрана защищает на первом проходе.
	var themes []models.Theme
	if err := DB.Find(&themes).Error; err != nil {
		t.Fatalf("список тем: %v", err)
	}
	if len(themes) != 1 {
		t.Fatalf("тем = %d, want 1: %+v", len(themes), themes)
	}
	if themes[0].Name != "По умолчанию" || !themes[0].IsDefault {
		t.Errorf("тема по умолчанию неверна: %+v", themes[0])
	}
	if gs.ActiveThemeId == nil || *gs.ActiveThemeId != themes[0].ID {
		t.Errorf("ActiveThemeId = %v, want %d", gs.ActiveThemeId, themes[0].ID)
	}

	var outputs []models.Output
	if err := DB.Find(&outputs).Error; err != nil {
		t.Fatalf("список output-ов: %v", err)
	}
	if len(outputs) != 1 {
		t.Fatalf("output-ов = %d, want 1: %+v", len(outputs), outputs)
	}
	if outputs[0].Name != "Экран" || outputs[0].ThemeId == nil || *outputs[0].ThemeId != themes[0].ID {
		t.Errorf("Output «Экран» неверен: %+v", outputs[0])
	}
}

// TestMigrateCopiesLegacyDB — источник с данными переносится целиком: dst
// получает те же строки, src переименован в .migrated, .part не остался.
func TestMigrateCopiesLegacyDB(t *testing.T) {
	src := filepath.Join(t.TempDir(), "test.db")
	dst := filepath.Join(t.TempDir(), "versebearer.db")
	buildLegacySourceDB(t, src)

	migratedFrom, err := migrateLegacyDB(dst, []string{src})
	if err != nil {
		t.Fatalf("migrateLegacyDB: %v", err)
	}
	if migratedFrom != src {
		t.Errorf("migratedFrom = %q, want %q", migratedFrom, src)
	}

	if _, statErr := os.Stat(dst); statErr != nil {
		t.Fatalf("dst не создан: %v", statErr)
	}
	if _, statErr := os.Stat(src); !os.IsNotExist(statErr) {
		t.Errorf("оригинал всё ещё на месте: %v", statErr)
	}
	if _, statErr := os.Stat(src + ".migrated"); statErr != nil {
		t.Errorf("ожидался %s.migrated: %v", src, statErr)
	}
	if matches, _ := filepath.Glob(dst + ".part.*"); len(matches) != 0 {
		t.Errorf("остался .part: %v", matches)
	}

	dstDB := openSQLite(t, dst)
	defer closeGormDB(dstDB)

	var couplet models.Couplet
	if err := dstDB.Where("text = ?", "куплет-маркер").First(&couplet).Error; err != nil {
		t.Errorf("куплет-маркер не найден в dst: %v", err)
	}
	var verse models.Verse
	if err := dstDB.Where("text = ?", "стих-маркер").First(&verse).Error; err != nil {
		t.Errorf("стих-маркер не найден в dst: %v", err)
	}
}

// buildOldSchemaSourceDB создаёт источник по схеме "до появления плейлистов"
// — без Output/AudioTrack/Playlist/PlaylistItem вовсе (таблиц playlists,
// playlist_items, audio_tracks, outputs в источнике НЕТ). Это архивная
// копия/база с другой машины/откат: ourTables (verses/couplets/
// global_states) на месте, современных таблиц нет.
func buildOldSchemaSourceDB(t *testing.T, path string) {
	t.Helper()
	db := openSQLite(t, path)
	if err := db.AutoMigrate(
		&models.Translation{}, &models.Book{}, &models.Chapter{}, &models.Verse{},
		&models.Song{}, &models.Couplet{}, &models.Screen{}, &models.Theme{},
		&models.GlobalState{}, &models.Font{}, &models.Image{},
	); err != nil {
		t.Fatalf("automigrate %s: %v", path, err)
	}

	song := models.Song{Title: "Древняя песня", Number: 1}
	mustCreate(t, db, "song", &song)
	mustCreate(t, db, "couplet", &models.Couplet{SongId: song.ID, Number: 1, Text: "куплет из старой сборки", Label: "1"})
	mustCreate(t, db, "global state", &models.GlobalState{Version: "3"})

	closeGormDB(db)
}

// TestMigrateOldSchemaSourceWithoutPlaylists — HIGH-2 обзора. verifyCopy
// раньше безусловно считал строки по всем copyVerifyTables, включая
// playlist_items/audio_tracks/playlists/outputs — таблицы, которых в базе
// от сборки до их появления просто нет. Count по отсутствующей таблице
// возвращает ошибку "no such table", а не 0: перенос архивной/старой базы
// падал на сверке копии, диалог советовал "освободите место на диске", а
// повторный запуск давал тот же отказ дословно — выхода у оператора не
// было. Пересечение с реальным списком таблиц источника обязано пропустить
// такую базу.
func TestMigrateOldSchemaSourceWithoutPlaylists(t *testing.T) {
	src := filepath.Join(t.TempDir(), "test.db")
	dst := filepath.Join(t.TempDir(), "versebearer.db")
	buildOldSchemaSourceDB(t, src)

	migratedFrom, err := migrateLegacyDB(dst, []string{src})
	if err != nil {
		t.Fatalf("migrateLegacyDB: %v (источник без плейлистов обязан переноситься успешно)", err)
	}
	if migratedFrom != src {
		t.Errorf("migratedFrom = %q, want %q", migratedFrom, src)
	}

	dstDB := openSQLite(t, dst)
	defer closeGormDB(dstDB)

	var couplet models.Couplet
	if err := dstDB.Where("text = ?", "куплет из старой сборки").First(&couplet).Error; err != nil {
		t.Errorf("куплет не найден в dst: %v", err)
	}
}

// TestMigrateLeftoverPartIsReplaced — Р7: остаточный `<dst>.part.<наш pid>`
// от сорвавшейся прошлой попытки этого же процесса не должен заклинивать
// миграцию навсегда (VACUUM INTO отказывается писать в существующий файл).
func TestMigrateLeftoverPartIsReplaced(t *testing.T) {
	src := filepath.Join(t.TempDir(), "test.db")
	dst := filepath.Join(t.TempDir(), "versebearer.db")
	buildLegacySourceDB(t, src)

	leftover := fmt.Sprintf("%s.part.%d", dst, os.Getpid())
	if err := os.WriteFile(leftover, []byte("огрызок сорвавшейся попытки"), 0o644); err != nil {
		t.Fatalf("seed leftover .part: %v", err)
	}

	if _, err := migrateLegacyDB(dst, []string{src}); err != nil {
		t.Fatalf("migrateLegacyDB: %v (остаточный .part не должен заклинивать старт)", err)
	}
	if _, statErr := os.Stat(dst); statErr != nil {
		t.Fatalf("dst не создан: %v", statErr)
	}
}

// TestMigrateSkippedWhenDestExists — «AppData выигрывает»: заранее
// заполненный dst не трогается, даже если рядом нашёлся валидный кандидат.
func TestMigrateSkippedWhenDestExists(t *testing.T) {
	src := filepath.Join(t.TempDir(), "test.db")
	dst := filepath.Join(t.TempDir(), "versebearer.db")
	buildLegacySourceDB(t, src)

	seedDestinationDB(t, dst, "маркер-appdata")

	migratedFrom, err := migrateLegacyDB(dst, []string{src})
	if err != nil {
		t.Fatalf("migrateLegacyDB: %v", err)
	}
	if migratedFrom != "" {
		t.Errorf("migratedFrom = %q, want \"\" — AppData должен победить", migratedFrom)
	}

	if _, statErr := os.Stat(src); statErr != nil {
		t.Errorf("src исчез, хотя миграция не должна была случиться: %v", statErr)
	}
	if _, statErr := os.Stat(src + ".migrated"); !os.IsNotExist(statErr) {
		t.Error("src переименован, хотя не должен был")
	}

	checkDB := openSQLite(t, dst)
	defer closeGormDB(checkDB)
	var got models.GlobalState
	if err := checkDB.First(&got, 1).Error; err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if got.Version != "маркер-appdata" {
		t.Errorf("dst изменился: Version = %q, want %q", got.Version, "маркер-appdata")
	}
}

// TestMigrateFailureLeavesOriginal — копирование срывается, потому что
// каталог назначения невозможно создать (на его месте лежит обычный файл),
// и оригинал остаётся нетронутым: инвариант 3 держится даже на самом раннем
// отказе. Более "глубокий" отказ (реальная блокировка исходного файла на
// чтение — антивирус, другая программа) проверяется отдельно на Windows в
// migrate_windows_test.go: портируемого и одновременно надёжного способа
// заставить сорваться именно VACUUM INTO без блокировки файла на уровне ОС
// нет (наш же os.Remove(part) в начале copyLegacyDB убирает с дороги любой
// одиночный файл/пустую директорию раньше, чем до них добирается что-то
// ещё) — сама проверка строк копии покрыта напрямую TestVerifyCopyCatchesEmptyCopy.
func TestMigrateFailureLeavesOriginal(t *testing.T) {
	src := filepath.Join(t.TempDir(), "test.db")
	buildLegacySourceDB(t, src)

	srcInfoBefore, err := os.Stat(src)
	if err != nil {
		t.Fatalf("stat src: %v", err)
	}

	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("файл на месте будущего каталога данных"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	dst := filepath.Join(blocker, "sub", "versebearer.db")

	if _, err := migrateLegacyDB(dst, []string{src}); err == nil {
		t.Fatal("migrateLegacyDB = nil, want error (каталог назначения невозможно создать)")
	}

	srcInfoAfter, statErr := os.Stat(src)
	if statErr != nil {
		t.Fatalf("src исчез: %v", statErr)
	}
	if srcInfoAfter.Size() != srcInfoBefore.Size() {
		t.Errorf("src изменился в размере: было %d, стало %d", srcInfoBefore.Size(), srcInfoAfter.Size())
	}
	if _, statErr := os.Stat(src + ".migrated"); !os.IsNotExist(statErr) {
		t.Error("src переименован, хотя миграция не должна была даже начаться")
	}
}

// TestRenameOriginalAsideNumbersOnConflict — Р9: занятое имя .migrated не
// затирается, следующая попытка — .migrated.2.
func TestRenameOriginalAsideNumbersOnConflict(t *testing.T) {
	src := filepath.Join(t.TempDir(), "test.db")
	if err := os.WriteFile(src, []byte("оригинал"), 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	if err := os.WriteFile(src+".migrated", []byte("прошлая миграция"), 0o644); err != nil {
		t.Fatalf("seed occupied .migrated: %v", err)
	}

	if err := renameOriginalAside(src); err != nil {
		t.Fatalf("renameOriginalAside: %v", err)
	}

	if _, statErr := os.Stat(src); !os.IsNotExist(statErr) {
		t.Errorf("src не переименован: stat err = %v", statErr)
	}
	if content, err := os.ReadFile(src + ".migrated"); err != nil || string(content) != "прошлая миграция" {
		t.Errorf("существующий .migrated затёрт: content=%q err=%v", content, err)
	}
	if content, err := os.ReadFile(src + ".migrated.2"); err != nil || string(content) != "оригинал" {
		t.Errorf(".migrated.2 не создан или не тот: content=%q err=%v", content, err)
	}
}

// TestRenameOriginalAsideTerminatesWhenAllNamesOccupied — MEDIUM обзора:
// единственный выход из исходного цикла был "os.Stat вернул IsNotExist".
// Любая другая ошибка Stat (или, как здесь, ЗАНЯТОЕ имя — os.Stat без
// ошибки) на каждой попытке подряд делала бы условие ложным бесконечно:
// точка коммита уже пройдена, данные в безопасности, но Open() не
// возвращался бы никогда — ни диалога, ни падения, просто зависшая
// программа. Функция обязана завершиться ошибкой за конечное число попыток,
// а не крутиться вечно.
func TestRenameOriginalAsideTerminatesWhenAllNamesOccupied(t *testing.T) {
	src := filepath.Join(t.TempDir(), "test.db")
	if err := os.WriteFile(src, []byte("оригинал"), 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	if err := os.WriteFile(src+".migrated", []byte("x"), 0o644); err != nil {
		t.Fatalf("seed .migrated: %v", err)
	}
	// Занимает .migrated.2 .. .migrated.999 — заведомо больше внутреннего
	// предела попыток функции, чтобы гарантированно исчерпать его, не зная
	// точного числа изнутри теста.
	for i := 2; i <= 999; i++ {
		p := fmt.Sprintf("%s.migrated.%d", src, i)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
	}

	done := make(chan error, 1)
	go func() { done <- renameOriginalAside(src) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("renameOriginalAside = nil при всех именах занятых, want error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("renameOriginalAside не вернулся за 10с — похоже на бесконечный цикл")
	}

	// Оригинал остаётся на месте — ошибка здесь только логируется вызывающим
	// кодом (copyLegacyDB), не валит запуск, но и не должна тихо потерять
	// файл.
	if _, statErr := os.Stat(src); statErr != nil {
		t.Errorf("src пропал при исчерпании попыток: %v", statErr)
	}
}

// TestLegacyCandidateOrder — CWD идёт первым кандидатом: единственный режим
// запуска, где пути расходятся (`task dev`), и в нём правильный ответ — CWD.
func TestLegacyCandidateOrder(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	cwdCandidate := filepath.Join(cwd, "test.db")

	exe, err := os.Executable()
	if err != nil {
		t.Skip("os.Executable недоступен в этом окружении")
	}
	exeCandidate := filepath.Join(filepath.Dir(exe), "test.db")
	if filepath.Clean(exeCandidate) == filepath.Clean(cwdCandidate) {
		t.Skip("каталог exe совпадает с cwd в этом окружении — дедуп проверяется в TestLegacyCandidateOrderDedup")
	}

	got := legacyDBCandidates()
	if len(got) != 2 || got[0] != filepath.Clean(cwdCandidate) || got[1] != filepath.Clean(exeCandidate) {
		t.Errorf("legacyDBCandidates() = %v, want [%s, %s]", got, filepath.Clean(cwdCandidate), filepath.Clean(exeCandidate))
	}
}

// TestLegacyCandidateOrderDedup — при совпадении CWD и каталога exe (запуск
// ярлыком/двойным кликом) список не должен содержать один и тот же путь
// дважды.
func TestLegacyCandidateOrderDedup(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("os.Executable недоступен в этом окружении")
	}
	exeDir := filepath.Dir(exe)
	prevCwd, err := os.Getwd()
	if err != nil {
		t.Skip("os.Getwd недоступен в этом окружении")
	}
	if err := os.Chdir(exeDir); err != nil {
		t.Skip("нет доступа на chdir в каталог exe: " + err.Error())
	}
	t.Cleanup(func() { os.Chdir(prevCwd) })

	got := legacyDBCandidates()
	if len(got) != 1 {
		t.Errorf("legacyDBCandidates() при совпадении cwd и каталога exe = %v, want ровно 1 путь", got)
	}
}

// foreignTable — таблица, которой не бывает в наших базах. Используется
// только для TestCandidateRejectsForeignDB, чтобы получить валидный SQLite
// без наших таблиц.
type foreignTable struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

// TestCandidateRejectsForeignDB — Р17: посторонняя (но настоящая) SQLite-база
// без наших таблиц молча отбрасывается кандидатом, а не уезжает в AppData.
func TestCandidateRejectsForeignDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db := openSQLite(t, path)
	if err := db.AutoMigrate(&foreignTable{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := db.Create(&foreignTable{Name: "чужая программа"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	closeGormDB(db)

	got, err := findLegacyDB([]string{path})
	if err != nil {
		t.Fatalf("findLegacyDB: %v", err)
	}
	if got != "" {
		t.Errorf("findLegacyDB выбрал чужую базу: %q", got)
	}
}

// TestCandidateRejectsNonSQLite — Р17/Р1.1: посторонний файл с именем
// test.db (не SQLite вообще) не делает программу незапускаемой — он просто
// не выбирается кандидатом. Это ключевое отличие от исходного плана, где
// такой файл кодифицировал отказ старта как желаемое поведение.
func TestCandidateRejectsNonSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if err := os.WriteFile(path, []byte("это не база данных, просто текстовый файл с тем же именем"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := findLegacyDB([]string{path})
	if err != nil {
		t.Fatalf("findLegacyDB вернул ошибку на постороннем файле: %v — это не должно быть фатально", err)
	}
	if got != "" {
		t.Errorf("findLegacyDB выбрал посторонний файл: %q", got)
	}
}

// TestAmbiguousCandidatesRefuse — Р17/Р1.2: несколько валидных кандидатов —
// ошибка, ни один не выбран, ни один не тронут. Ошибка выбора источника
// необратима по построению (после неё побеждает AppData), поэтому программа
// не угадывает.
func TestAmbiguousCandidatesRefuse(t *testing.T) {
	p1 := filepath.Join(t.TempDir(), "test.db")
	p2 := filepath.Join(t.TempDir(), "test.db")
	buildLegacySourceDB(t, p1)
	buildLegacySourceDB(t, p2)

	got, err := findLegacyDB([]string{p1, p2})
	if got != "" {
		t.Errorf("findLegacyDB выбрал %q вместо отказа", got)
	}
	var ambiguous *AmbiguousCandidatesError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("findLegacyDB error = %v (%T), want *AmbiguousCandidatesError", err, err)
	}
	if len(ambiguous.Candidates) != 2 {
		t.Errorf("Candidates len = %d, want 2", len(ambiguous.Candidates))
	}
	if _, statErr := os.Stat(p1); statErr != nil {
		t.Errorf("p1 тронут: %v", statErr)
	}
	if _, statErr := os.Stat(p2); statErr != nil {
		t.Errorf("p2 тронут: %v", statErr)
	}
}

// TestVerifyCopyCatchesEmptyCopy — Р3/Р17: копия с полной схемой, но без
// единой строки данных, не принимается. Это ровно тот класс, который
// PRAGMA integrity_check пропускает: VACUUM INTO пересобирает b-деревья с
// нуля, так что его выход структурно валиден почти по построению — пустая
// 4-килобайтная база проходит integrity_check со статусом ok (см. Р0).
// verifyCopy обязан ловить расхождение по числу строк, а не по структурной
// валидности.
func TestVerifyCopyCatchesEmptyCopy(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "src.db")
	buildLegacySourceDB(t, srcPath)

	srcDB := openSQLite(t, srcPath)
	defer closeGormDB(srcDB)

	emptyPath := filepath.Join(t.TempDir(), "empty.db")
	emptyDB := openSQLite(t, emptyPath)
	automigrateFullSchema(t, emptyDB)
	closeGormDB(emptyDB)

	if err := verifyCopy(srcDB, emptyPath); err == nil {
		t.Fatal("verifyCopy(пустая копия с полной схемой) = nil, want error")
	}
}

// TestLeftoverPartSwept — Р7/Р17: чужой `.part.*` подметается даже на ветке
// «миграция не нужна» — иначе огрызок остаётся в %LOCALAPPDATA% навсегда,
// потому что исходный план выходил из миграции раньше, чем доходил до уборки.
func TestLeftoverPartSwept(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "versebearer.db")

	seedDestinationDB(t, dst, "")

	// Огрызок старше sweepPartMinAge — иначе он неотличим от живого соседа,
	// который только начал копировать, и подметаться не должен (см.
	// TestFreshForeignPartIsNotSwept).
	foreign := seedForeignPart(t, dst, 2*sweepPartMinAge)

	if _, err := migrateLegacyDB(dst, nil); err != nil {
		t.Fatalf("migrateLegacyDB: %v", err)
	}

	if _, statErr := os.Stat(foreign); !os.IsNotExist(statErr) {
		t.Errorf("чужой .part не подметён на ветке «миграция не нужна»: stat err = %v", statErr)
	}
}

// TestFreshForeignPartIsNotSwept — MEDIUM обзора: свежий (моложе
// sweepPartMinAge) чужой .part обязан пережить migrateLegacyDB нетронутым —
// это может быть рабочий файл живого соседа, стартовавшего секундой позже
// (цену безусловной уборки см. в комментарии к sweepPartMinAge).
func TestFreshForeignPartIsNotSwept(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "versebearer.db")

	seedDestinationDB(t, dst, "")

	// Возраст 0 — только что созданный огрызок: ровно то, что может
	// принадлежать живому соседу, стартовавшему секундой позже.
	foreign := seedForeignPart(t, dst, 0)

	if _, err := migrateLegacyDB(dst, nil); err != nil {
		t.Fatalf("migrateLegacyDB: %v", err)
	}

	if _, statErr := os.Stat(foreign); statErr != nil {
		t.Errorf("свежий .part живого соседа снесён: %v", statErr)
	}
}

// TestReadOnlySourceWithSpacesAndCyrillic — риск, явно названный в ревизии
// (Р6): путь источника может содержать пробелы и кириллицу (профиль
// пользователя вида C:\Users\<Имя>\...). readOnlyDSN обязан правильно
// экранировать такой путь, иначе открытие на чтение падает и всякая
// миграция для такого пользователя уходила бы в запасной путь на
// чтение-запись без необходимости.
func TestReadOnlySourceWithSpacesAndCyrillic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "папка с пробелом")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := filepath.Join(dir, "test.db")
	buildLegacySourceDB(t, src)

	verdict, err := looksLikeOurDB(src)
	if err != nil {
		t.Fatalf("looksLikeOurDB: %v", err)
	}
	if verdict != candidateOurs {
		t.Errorf("looksLikeOurDB = %v, want candidateOurs для валидной базы по пути с пробелом и кириллицей", verdict)
	}

	db, usedReadWrite, err := openReadPreferRO(src)
	if err != nil {
		t.Fatalf("openReadPreferRO: %v", err)
	}
	defer closeGormDB(db)
	if usedReadWrite {
		t.Error("openReadPreferRO откатился на чтение-запись для целой базы без горячего журнала — путь неверно экранирован")
	}
}

// TestReadOnlyDSNHandlesUNC — MEDIUM обзора: filepath.IsAbs на Windows
// возвращает false для UNC-путей (\\server\share\...) — это не наш баг,
// проверено отдельно на голом filepath.Abs: он молча подставляет текущий
// диск, "\\server\share\test.db" превращается в "C:\server\share\test.db".
// DSN на такой путь указывал бы в несуществующее место, и валидный
// кандидат с сетевой шары отбрасывался бы как "не наш файл". Запуск с
// сетевой шары — реальный сценарий (сетевые профили, шара для бэкапов).
//
// Открытие настоящего сетевого пути в этом окружении не проверить (нет
// поднятой SMB-шары) — тест сверяет ТОЧНО то, что было сломано: что DSN
// не содержит подставленный диск и правильно кодирует хост/путь, включая
// кириллицу и пробелы в имени шары.
func TestReadOnlyDSNHandlesUNC(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{`\\server\share\versebearer\test.db`, "file://server/share/versebearer/test.db?mode=ro"},
		{`\\nas.local\Данные\test.db`, "file://nas.local/%D0%94%D0%B0%D0%BD%D0%BD%D1%8B%D0%B5/test.db?mode=ro"},
		{`\\host\share with spaces\test.db`, "file://host/share%20with%20spaces/test.db?mode=ro"},
	}
	for _, c := range cases {
		if !isUNCPath(c.path) {
			t.Errorf("isUNCPath(%q) = false, want true", c.path)
			continue
		}
		got, err := readOnlyDSN(c.path)
		if err != nil {
			t.Errorf("readOnlyDSN(%q): %v", c.path, err)
			continue
		}
		if got != c.want {
			t.Errorf("readOnlyDSN(%q) = %q, want %q", c.path, got, c.want)
		}
		if strings.Contains(got, `C:`) || strings.Contains(got, "c:") {
			t.Errorf("readOnlyDSN(%q) = %q — похоже, диск подставлен вместо UNC-хоста", c.path, got)
		}
	}
}

// TestOpenReturnsErrorOnFailedMigration — Р17/инвариант 7: сорвавшаяся
// миграция возвращается именно из Open(), а не приводит к тихому запуску с
// чистой базой. Единственный тест в этом пакете, который вызывает Open() —
// а значит и единственный, кто трогает paths.DataDir() (закешированный
// sync.Once на весь процесс теста): другие тесты этого файла работают с
// migrateLegacyDB/OpenAt напрямую и paths не касаются, так что порядок
// выполнения тестов эту установку VERSEBEARER_DATA не портит.
func TestOpenReturnsErrorOnFailedMigration(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("VERSEBEARER_DATA", dataDir)

	dbPath := filepath.Join(dataDir, "versebearer.db")
	if err := os.WriteFile(dbPath, []byte("не база данных"), 0o644); err != nil {
		t.Fatalf("seed corrupt dst: %v", err)
	}

	dbBefore := DB
	err := Open()
	if err == nil {
		t.Fatal("Open() = nil, want error (dst повреждён)")
	}
	var corrupt *CorruptDestinationError
	if !errors.As(err, &corrupt) {
		t.Fatalf("Open() error = %v (%T), want *CorruptDestinationError", err, err)
	}
	if DB != dbBefore {
		t.Error("Open() сорвался, но пакетная переменная DB изменилась — инвариант 7 нарушен: программа не должна тихо завести чистую базу вместо отказа")
	}
}
