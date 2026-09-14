package inits

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// copyVerifyTables — таблицы, по которым verifyCopy сверяет число строк
// между источником и копией (Р3 ревизии, обоснование — в комментарии к
// verifyCopy).
var copyVerifyTables = []string{
	"verses", "couplets", "songs", "books", "chapters", "translations",
	"audio_tracks", "playlists", "playlist_items", "themes", "outputs",
	"global_states",
}

// ourTables — таблицы, наличие которых в sqlite_master отличает "нашу" базу
// от постороннего файла, случайно названного test.db (Р1.1 ревизии).
var ourTables = []string{"verses", "couplets", "global_states"}

const sqliteMagic = "SQLite format 3\x00"

// AmbiguousCandidatesError — Р1.2: найдено больше одного валидного кандидата
// на перенос. Ошибка выбора источника необратима по построению (после неё
// побеждает AppData и повторная миграция невозможна), поэтому программа не
// угадывает, а отказывается стартовать и показывает список.
type AmbiguousCandidatesError struct {
	Candidates []legacyCandidate
}

func (e *AmbiguousCandidatesError) Error() string {
	return fmt.Sprintf("найдено несколько баз-кандидатов на перенос (%d)", len(e.Candidates))
}

// DialogBody — текст для стартового диалога (Р11).
func (e *AmbiguousCandidatesError) DialogBody() string {
	var b strings.Builder
	b.WriteString("Не удалось определить, какую базу переносить.\n\nНайдено несколько:\n")
	for _, c := range e.Candidates {
		b.WriteString(fmt.Sprintf("%s — %.1f МБ, %s\n", c.path, float64(c.size)/(1024*1024), c.modTime.Format("02.01.2006 15:04")))
	}
	b.WriteString("\nДанные не потеряны. Уберите лишние файлы, оставив только нужный, и запустите программу снова.")
	return b.String()
}

// CorruptDestinationError — Р4: файл на месте назначения существует, но не
// является базой SQLite. Валидный SQLite без наших таблиц (первая установка)
// сюда не попадает — это нормальный путь, AutoMigrate его достроит.
type CorruptDestinationError struct {
	Path string
	// Src — путь к найденному кандидату на перенос, если он есть (для текста
	// диалога). Может быть пустым.
	Src string
	// IsDir — на месте назначения оказался каталог, а не файл. Текст
	// диалога для этого случая другой: "переименуйте этот файл" был бы
	// бессмысленным советом для каталога (LOW-находка обзора).
	IsDir bool
}

func (e *CorruptDestinationError) Error() string {
	if e.IsDir {
		return fmt.Sprintf("путь базы данных занят каталогом: %s", e.Path)
	}
	return fmt.Sprintf("файл базы данных повреждён: %s", e.Path)
}

// DialogBody — текст для стартового диалога (Р11).
func (e *CorruptDestinationError) DialogBody() string {
	action := "Переименуйте этот файл (например, добавив .bad в конце)"
	problem := fmt.Sprintf("База данных повреждена: %s", e.Path)
	if e.IsDir {
		action = "Переименуйте или уберите этот каталог с дороги"
		problem = fmt.Sprintf("Путь базы данных занят каталогом, а не файлом: %s", e.Path)
	}
	if e.Src == "" {
		return fmt.Sprintf("%s\n\n%s и запустите программу снова.", problem, action)
	}
	return fmt.Sprintf(
		"%s\n\nДанные не потеряны. %s и запустите программу снова — "+
			"база будет перенесена заново из %s.",
		problem, action, e.Src)
}

// MigrationFailedError — копирование источника сорвалось на любом из шагов
// (открытие, VACUUM INTO, сверка строк, переименование). Оригинал в это
// время ни разу не тронут.
type MigrationFailedError struct {
	Src string
	Err error
}

func (e *MigrationFailedError) Error() string {
	return fmt.Sprintf("перенос базы из %s не удался: %v", e.Src, e.Err)
}

func (e *MigrationFailedError) Unwrap() error { return e.Err }

// DialogBody — текст для стартового диалога (Р11).
func (e *MigrationFailedError) DialogBody() string {
	return fmt.Sprintf(
		"Не удалось перенести базу данных.\n%v\n\n"+
			"Исходная база не тронута и осталась в %s. Запустите программу снова; "+
			"если ошибка повторяется, освободите место на диске и проверьте, что база не открыта другой программой.",
		e.Err, e.Src)
}

// OpenFailedError — OpenAt() не смог открыть/подготовить базу по Path (после
// того как файл уже выбран — миграция либо не требовалась, либо прошла
// успешно). Отдельный тип, а не голый fmt.Errorf: даёт оператору то же
// действие, что при повреждённом приёмнике (Р11), вместо голого текста
// ошибки без единого следующего шага — а заодно убирает задвоение префикса
// "не удалось открыть базу", которое раньше получалось из-за того, что
// текст этой же ошибки ещё раз оборачивался в main.go общим шаблоном.
type OpenFailedError struct {
	Path string
	Err  error
}

func (e *OpenFailedError) Error() string {
	return fmt.Sprintf("не удалось открыть базу %s: %v", e.Path, e.Err)
}

func (e *OpenFailedError) Unwrap() error { return e.Err }

// DialogBody — текст для стартового диалога (Р11). Если рядом с программой
// (см. legacyDBCandidates) сохранился след успешной миграции — оригинал,
// переименованный в .migrated, — называет его прямо: это самый быстрый путь
// назад, если новая база на новом месте оказалась повреждена уже после
// переноса.
func (e *OpenFailedError) DialogBody() string {
	action := "Переименуйте этот файл (например, добавив .bad в конце) — на его месте будет создана новая пустая база."
	if mig := findMigratedOriginal(); mig != "" {
		action = fmt.Sprintf(
			"Переименуйте этот файл (например, добавив .bad в конце), затем верните %s обратно в %s.",
			filepath.Base(mig), strings.TrimSuffix(mig, ".migrated"))
	}
	return fmt.Sprintf("Не удалось открыть базу данных.\n%s: %v\n\n%s и запустите программу снова.", e.Path, e.Err, action)
}

// findMigratedOriginal ищет рядом с программой (места из legacyDBCandidates)
// след успешной миграции — исходник, переименованный в .migrated (Р9).
// Возвращает первый найденный, "" если ни одного.
func findMigratedOriginal() string {
	for _, c := range legacyDBCandidates() {
		m := c + ".migrated"
		if info, err := os.Stat(m); err == nil && !info.IsDir() {
			return m
		}
	}
	return ""
}

// legacyCandidate — кандидат на перенос вместе с метаданными, нужными для
// сообщения об отказе при нескольких валидных кандидатах (Р1.2).
type legacyCandidate struct {
	path    string
	size    int64
	modTime time.Time
}

// legacyDBCandidates возвращает места, где могла остаться старая test.db:
// рабочий каталог процесса, затем каталог исполняемого файла — в этом
// порядке. При `wails3 task dev`/`task dev` CWD — корень репозитория с живой
// базой, а бинарь лежит в bin/ без неё; при запуске ярлыком/двойным кликом
// оба пути совпадают (дедуплицируются ниже); при запуске из произвольного
// каталога верным оказывается только каталог exe. Дубликаты убираются через
// filepath.Clean + сравнение.
func legacyDBCandidates() []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" {
			return
		}
		clean := filepath.Clean(p)
		if seen[clean] {
			return
		}
		seen[clean] = true
		out = append(out, clean)
	}

	if cwd, err := os.Getwd(); err == nil {
		add(filepath.Join(cwd, "test.db"))
	}
	if exe, err := os.Executable(); err == nil {
		add(filepath.Join(filepath.Dir(exe), "test.db"))
	}
	return out
}

// looksLikeSQLiteFile проверяет только магические байты заголовка — этого
// достаточно, чтобы отличить SQLite-файл от постороннего мусора с тем же
// именем, но недостаточно, чтобы отличить НАШУ базу от чужой (см.
// looksLikeOurDB).
func looksLikeSQLiteFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, len(sqliteMagic))
	if _, err := io.ReadFull(f, buf); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return false, nil
		}
		return false, err
	}
	return string(buf) == sqliteMagic, nil
}

// readOnlyDSN строит DSN mattn/go-sqlite3 для открытия файла на чтение
// (?mode=ro). Путь экранируется через net/url, а не простой конкатенацией:
// обратные слэши Windows не являются валидным разделителем в file:-URI,
// а пробелы и кириллица в пути профиля (C:\Users\<Имя>\...) требуют
// percent-encoding.
//
// UNC-пути (\\server\share\...) обрабатываются отдельно, ДО filepath.Abs:
// filepath.IsAbs на Windows возвращает false для них (проверено), так что
// Abs молча подставляет текущий диск — "\\server\share\test.db" превращается
// в "C:\server\share\test.db", кандидат отбрасывается как "не наш", хотя
// исходный файл просто не на этом диске. Запуск с сетевой шары — реальный
// сценарий, не гипотетический.
func readOnlyDSN(path string) (string, error) {
	if isUNCPath(path) {
		return uncReadOnlyDSN(path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	u := url.URL{Path: filepath.ToSlash(abs)}
	return "file:" + u.EscapedPath() + "?mode=ro", nil
}

// isUNCPath проверяет префикс \\ или // — оба варианта встречаются (Go
// иногда уже нормализует один в другой при передаче через filepath.Clean).
func isUNCPath(path string) bool {
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//")
}

// uncReadOnlyDSN строит file:-URI для UNC-пути в форме, которую понимает
// SQLite на Windows: "file://host/share/rest?mode=ro" — эквивалент
// "\\host\share\rest". У UNC-пути нет диска, поэтому здесь нет и Abs: путь
// уже абсолютен по построению (\\host\share\...).
func uncReadOnlyDSN(path string) (string, error) {
	trimmed := filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(path, `\\`), "//"))
	host, rest, found := strings.Cut(trimmed, "/")
	if !found || host == "" || rest == "" {
		return "", fmt.Errorf("не удалось разобрать UNC-путь %q", path)
	}
	escapedRest := (&url.URL{Path: "/" + rest}).EscapedPath()
	return "file://" + host + escapedRest + "?mode=ro", nil
}

// candidateVerdict — три исхода проверки кандидата (HIGH-1 обзора), а не два.
// Слитые "не наш файл" и "не смог проверить" делают состояние необратимым:
// если живую test.db в момент запуска держит эксклюзивным хендлом другой
// процесс (незавершённый прошлый экземпляр, антивирус, бэкап-агент), ошибка
// открытия раньше маскировалась под "не наш файл", кандидат пропускался,
// заводилась пустая "чистая установка" — а на СЛЕДУЮЩЕМ запуске
// inspectDestination увидит валидный SQLite на месте назначения, и повторная
// миграция станет невозможна уже никогда. Живые данные при этом останутся
// на месте нетронутыми, но недостижимыми.
type candidateVerdict int

const (
	// candidateNotOurs — определённо не наша база: либо не SQLite вообще,
	// либо SQLite без наших таблиц. Молча пропускается — это нормально,
	// имя "test.db" ничего не гарантирует.
	candidateNotOurs candidateVerdict = iota
	// candidateOurs — прошёл все проверки, годится в кандидаты.
	candidateOurs
	// candidateUnknown — прочитать не удалось (файл занят другой
	// программой, отозваны права и т.п.). Это НЕ "чужой файл" — не наша ли
	// это база, сказать невозможно, а после ложной "чистой установки"
	// повторной миграции не будет. Обязано валить старт.
	candidateUnknown
)

// looksLikeOurDB — Р1.1: кандидат обязан не только быть SQLite-файлом, но и
// содержать наши таблицы. Открывается на чтение (с тем же ro→rw запасным
// путём, что и копирование, см. openReadPreferRO — иначе кандидат с горячим
// -journal отбрасывался бы как "не наш" раньше, чем запасной путь Р6 успел
// бы сработать), соединение закрывается до возврата — на этом этапе
// кандидат ещё не выбран, трогать его нельзя.
func looksLikeOurDB(path string) (candidateVerdict, error) {
	ok, err := looksLikeSQLiteFile(path)
	if err != nil {
		return candidateUnknown, err
	}
	if !ok {
		return candidateNotOurs, nil
	}

	db, _, err := openReadPreferRO(path)
	if err != nil {
		return candidateUnknown, err
	}
	defer closeGormDB(db)

	for _, table := range ourTables {
		var count int64
		if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count).Error; err != nil {
			return candidateUnknown, err
		}
		if count == 0 {
			return candidateNotOurs, nil
		}
	}
	return candidateOurs, nil
}

// CandidateUnreadableError — HIGH-1: кандидат непонятного статуса (не смогли
// прочитать), а не заведомо чужой файл. Фатально — молчание здесь
// необратимо (см. candidateVerdict).
type CandidateUnreadableError struct {
	Path string
	Err  error
}

func (e *CandidateUnreadableError) Error() string {
	return fmt.Sprintf("не удалось проверить возможный источник миграции %s: %v", e.Path, e.Err)
}

func (e *CandidateUnreadableError) Unwrap() error { return e.Err }

// DialogBody — текст для стартового диалога (Р11).
func (e *CandidateUnreadableError) DialogBody() string {
	return fmt.Sprintf(
		"Не удалось прочитать файл базы данных: %s\n%v\n\n"+
			"Похоже, он открыт другой программой (антивирусом, службой синхронизации, "+
			"незавершённым предыдущим запуском). Закройте её и запустите программу снова.",
		e.Path, e.Err)
}

// findLegacyDB отбирает среди candidates те, что существуют, непусты и
// проходят looksLikeOurDB. Ровно один валидный кандидат — возвращается его
// путь. Ни одного — "", nil (переносить нечего). Больше одного —
// *AmbiguousCandidatesError (Р1.2): угадывать здесь нельзя, ошибка выбора
// необратима. Кандидат, который не удалось прочитать (а не заведомо не наш,
// см. candidateVerdict) — *CandidateUnreadableError, тоже фатально.
func findLegacyDB(candidates []string) (string, error) {
	var valid []legacyCandidate
	for _, p := range candidates {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() || info.Size() == 0 {
			continue
		}
		verdict, err := looksLikeOurDB(p)
		switch verdict {
		case candidateOurs:
			valid = append(valid, legacyCandidate{path: p, size: info.Size(), modTime: info.ModTime()})
		case candidateUnknown:
			return "", &CandidateUnreadableError{Path: p, Err: err}
		default: // candidateNotOurs
			log.Printf("inits: %s не похож на нашу базу — пропускаю", p)
		}
	}

	switch len(valid) {
	case 0:
		return "", nil
	case 1:
		return valid[0].path, nil
	default:
		return "", &AmbiguousCandidatesError{Candidates: valid}
	}
}

// sweepPartMinAge — .part.* моложе этого возраста не подметается: возможно,
// это живой сосед, стартовавший секундой позже (два процесса — normальный
// случай Р7, не гонка). Час с большим запасом дольше любого разумного
// копирования (десятки мегабайт).
//
// Возраст, а не безусловная уборка любого чужого файла (как было раньше) —
// на Windows это была бы просто трата шанса впустую, но на linux/darwin
// unlink открытого файла успешен немедленно (в отличие от Windows), и
// живой сосед дописал бы копию в уже отвязанный inode, а на финальном
// os.Rename упал бы с виду ни на чём, хотя данные остались целы. Р7 обещала
// "худший исход — впустую сделанная работа", а не фатальный диалог у
// живого соседа.
const sweepPartMinAge = time.Hour

// ownPartPath — имя временной копии этого процесса: `<dst>.part.<pid>` (Р7).
// Соглашение об имени живёт здесь одно на двоих: copyLegacyDB в этот файл
// пишет, sweepForeignParts по нему же узнаёт свой файл среди чужих — разъедься
// эти два выражения, и уборка начала бы сносить собственную копию на ходу.
func ownPartPath(dst string) string {
	return fmt.Sprintf("%s.part.%d", dst, os.Getpid())
}

// sweepForeignParts подметает чужие остаточные `<dst>.part.<pid>` старше
// sweepPartMinAge — файлы, оставленные другими процессами/прошлыми
// запусками (Р7). Свой файл (по текущему PID) не трогает. Делается
// best-effort и на ветке "миграция не нужна" тоже: иначе огрызок до
// нескольких десятков мегабайт остаётся в %LOCALAPPDATA% навсегда.
func sweepForeignParts(dst string) {
	matches, err := filepath.Glob(dst + ".part.*")
	if err != nil {
		return
	}
	own := ownPartPath(dst)
	for _, m := range matches {
		if m == own {
			continue
		}
		info, statErr := os.Stat(m)
		if statErr != nil {
			continue
		}
		if time.Since(info.ModTime()) < sweepPartMinAge {
			continue
		}
		if err := os.Remove(m); err != nil {
			log.Printf("inits: не удалось убрать остаточный %s: %v", m, err)
		}
	}
}

// inspectDestination проверяет состояние файла назначения (Р4):
//   - файла нет, или он есть но размером 0 (огрызок) — не готов, размер-0
//     огрызок удаляется, возвращается (false, nil);
//   - файл есть и это валидный SQLite — готов, миграция не нужна: (true, nil);
//   - файл есть, но это не SQLite — фатально: (false, *CorruptDestinationError).
//
// "Наши ли таблицы" у dst НЕ проверяется: валидный SQLite без наших таблиц —
// нормальный путь первой установки, AutoMigrate его достроит.
func inspectDestination(dst string) (ready bool, err error) {
	info, statErr := os.Stat(dst)
	if statErr != nil {
		return false, nil
	}
	if info.IsDir() {
		return false, &CorruptDestinationError{Path: dst, IsDir: true}
	}
	if info.Size() == 0 {
		if err := os.Remove(dst); err != nil {
			log.Printf("inits: не удалось убрать пустой огрызок %s: %v", dst, err)
		}
		return false, nil
	}

	ok, checkErr := looksLikeSQLiteFile(dst)
	if checkErr != nil {
		return false, fmt.Errorf("не удалось проверить %s: %w", dst, checkErr)
	}
	if !ok {
		return false, &CorruptDestinationError{Path: dst}
	}
	return true, nil
}

// openReadPreferRO открывает path на чтение (?mode=ro): "оригинал не
// изменяется" (инвариант). Единственное узаконенное исключение (Р6) —
// горячий -journal от упавшего прошлого запуска: read-only не может его
// откатить, и открытие падает. Тогда — один явный повтор на чтение-запись с
// обязательной строкой в лог (пишется здесь же, чтобы не задваиваться у
// двух вызывающих); SQLite восстановит журнал сам при открытии.
//
// Общий для двух вызывающих: looksLikeOurDB (проверка кандидата) и
// copyLegacyDB (сам перенос через VACUUM INTO) — раньше запасной путь жил
// только в copyLegacyDB и был недостижим на практике: looksLikeOurDB
// открывала файл ТЕМ ЖЕ readOnlyDSN самостоятельно, и кандидат с горячим
// журналом отбрасывался как нечитаемый до того, как до copyLegacyDB вообще
// доходило дело.
func openReadPreferRO(path string) (db *gorm.DB, usedReadWrite bool, err error) {
	if dsn, dsnErr := readOnlyDSN(path); dsnErr == nil {
		if db, roErr := gorm.Open(sqlite.Open(dsn), &gorm.Config{}); roErr == nil {
			return db, false, nil
		}
	}

	db, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, false, err
	}
	log.Printf("inits: %s открыт на запись — SQLite восстанавливает журнал прошлого падения", path)
	return db, true, nil
}

func closeGormDB(db *gorm.DB) {
	if db == nil {
		return
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}

// sourceTables читает имена таблиц из sqlite_master db.
func sourceTables(db *gorm.DB) (map[string]bool, error) {
	var names []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type = 'table'").Scan(&names).Error; err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set, nil
}

// verifyCopy сверяет число строк между источником (ещё открытым) и только
// что записанной копией part. HIGH-2 обзора: считаются не все
// copyVerifyTables безусловно, а только те, что реально есть в источнике —
// база от сборки до появления плейлистов/аудио (архивная копия, база с
// другой машины, откат) не содержит playlist_items/audio_tracks/playlists/
// outputs вовсе, и Count по отсутствующей таблице возвращает "no such
// table", а не 0 — старая версия валила перенос такой базы безусловной и
// неисправимой ошибкой на каждом запуске.
//
// ourTables (verses/couplets/global_states) обязаны быть в источнике все —
// без этого пересечение может выродиться до одной третьестепенной таблицы
// и пропустить пустую/бессмысленную копию (ровно то, ради чего писалась
// Р3): findLegacyDB это уже гарантирует для настоящей миграции, проверка
// здесь — на случай прямого вызова verifyCopy в обход неё.
//
// PRAGMA integrity_check намеренно не используется (Р3): VACUUM INTO
// пересобирает b-деревья с нуля, поэтому его выход структурно валиден почти
// по построению — integrity_check прошёл бы даже пустую 4-килобайтную
// копию.
func verifyCopy(srcDB *gorm.DB, part string) error {
	partDB, err := gorm.Open(sqlite.Open(part), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("не удалось открыть копию %s: %w", part, err)
	}
	defer closeGormDB(partDB)

	srcTables, err := sourceTables(srcDB)
	if err != nil {
		return fmt.Errorf("не удалось прочитать список таблиц источника: %w", err)
	}

	for _, table := range ourTables {
		if !srcTables[table] {
			return fmt.Errorf("источник не содержит обязательную таблицу %s — сверка копии невозможна", table)
		}
	}

	checked := 0
	for _, table := range copyVerifyTables {
		if !srcTables[table] {
			continue
		}
		checked++
		var srcCount, partCount int64
		if err := srcDB.Table(table).Count(&srcCount).Error; err != nil {
			return fmt.Errorf("не удалось посчитать строки %s в источнике: %w", table, err)
		}
		if err := partDB.Table(table).Count(&partCount).Error; err != nil {
			return fmt.Errorf("не удалось посчитать строки %s в копии: %w", table, err)
		}
		if srcCount != partCount {
			return fmt.Errorf("копия неполна: таблица %s — %d строк в источнике, %d в копии", table, srcCount, partCount)
		}
	}
	if checked == 0 {
		return fmt.Errorf("источник не содержит ни одной проверяемой таблицы — сверка копии невозможна")
	}
	return nil
}

// renameWithRetry переименовывает part в dst с повторами. Это единственная
// точка коммита во всей миграции, и она идёт сразу после закрытия только что
// записанного файла на десятки мегабайт — антивирус (Defender) сканирует
// такой файл по закрытию хендла и может на мгновение отдать
// ERROR_SHARING_VIOLATION (тот же класс ошибки, что уже разобран в
// audio_import.go:303-328). 5 попыток по 200 мс — единственное место в
// задаче, где ремень и подтяжки оправданы безусловно: цена промаха — отказ
// стартовать на ровном месте. Сна после последней попытки нет — незачем
// платить 200 мс к неуспешному старту, если следующей попытки всё равно
// не будет.
func renameWithRetry(from, to string) error {
	const attempts = 5
	var err error
	for i := 0; i < attempts; i++ {
		if err = os.Rename(from, to); err == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}
	return err
}

// renameOriginalAside переименовывает перенесённый оригинал в src+".migrated",
// не затирая уже существующий .migrated (Р9): при занятом имени пробует
// .migrated.2, .migrated.3, и так далее. Ограничено maxMigratedSuffix
// попыток: единственный выход из исходного цикла был "os.Stat вернул
// IsNotExist" — ЛЮБАЯ другая ошибка Stat (отозванные права на каталог,
// сеть, исчерпание хендлов) делала условие ложным на каждом имени подряд, и
// цикл крутился вечно, не находя свободного имени и не давая ошибки. Точка
// коммита к этому моменту уже пройдена, данные в безопасности — но Open()
// не возвращался бы никогда: ни диалога, ни падения, просто зависшая
// программа. Не-IsNotExist здесь трактуется как "имя занято, пробуем
// следующее" (не как "точно свободно") — не смогли посмотреть отличить от
// действительно существующего файла всё равно нельзя, а исчерпание
// попыток — обычная ошибка, вызывающий код её только логирует и не валит
// запуск (см. copyLegacyDB).
func renameOriginalAside(src string) error {
	const maxMigratedSuffix = 500
	target := src + ".migrated"
	for attempt := 0; attempt < maxMigratedSuffix; attempt++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			return os.Rename(src, target)
		}
		target = fmt.Sprintf("%s.migrated.%d", src, attempt+2)
	}
	return fmt.Errorf("не нашлось свободного имени вида %s.migrated[.N] за %d попыток", src, maxMigratedSuffix)
}

// copyLegacyDB переносит src в dst через VACUUM INTO. part получает
// уникальное имя `<dst>.part.<pid>` (Р7) вместо файла-лока: при двух
// одновременно запущенных процессах оба сделают независимые копии, и
// финальный renameWithRetry сам послужит точкой сериализации — кто успел,
// того и dst. Худший исход — впустую проделанная работа, а не незапускаемое
// состояние (которым обернулся бы файл-лок, оставшийся от упавшего запуска).
func copyLegacyDB(src, dst string) (string, error) {
	part := ownPartPath(dst)
	// Остаток от прошлого падения этого же PID — маловероятно (PID должен
	// повториться между запусками), но VACUUM INTO отказывается писать в уже
	// существующий файл, так что дёшево подстраховаться.
	os.Remove(part)

	srcDB, _, err := openReadPreferRO(src)
	if err != nil {
		return "", &MigrationFailedError{Src: src, Err: fmt.Errorf("не удалось открыть исходную базу: %w", err)}
	}

	// abort — отказ, пока источник ещё открыт. Закрыть ДО удаления part, не
	// после: сорвавшийся на середине VACUUM INTO может не успеть DETACH
	// внутреннюю привязку к part — на Windows srcDB тогда всё ещё держит на
	// него хендл, и os.Remove падает с sharing violation до того, как
	// соединение закрыто. Порядок этих двух шагов важнее, чем выглядит,
	// поэтому он записан один раз на оба ранних отказа.
	abort := func(err error) error {
		closeGormDB(srcDB)
		os.Remove(part)
		return &MigrationFailedError{Src: src, Err: err}
	}

	if err := srcDB.Exec("VACUUM INTO ?", part).Error; err != nil {
		return "", abort(fmt.Errorf("не удалось скопировать базу: %w", err))
	}

	if err := verifyCopy(srcDB, part); err != nil {
		return "", abort(err)
	}

	// Закрыть источник ДО любых переименований: на Windows нельзя
	// переименовать файл с открытым хендлом (тот же урок, что в
	// audio_import.go:303-328).
	closeGormDB(srcDB)

	// Источник уже закрыт — поэтому здесь не abort, а только уборка копии.
	if err := renameWithRetry(part, dst); err != nil {
		os.Remove(part)
		return "", &MigrationFailedError{Src: src, Err: fmt.Errorf("не удалось зафиксировать перенесённую базу: %w", err)}
	}

	// Точка коммита пройдена — данные в безопасности на новом месте. Ошибка
	// переименования оригинала в сторону — не повод падать: оставшийся
	// test.db безвреден, при следующем запуске выигрывает AppData.
	if err := renameOriginalAside(src); err != nil {
		log.Printf("inits: не удалось переименовать перенесённый оригинал %s в сторону: %v (данные уже на новом месте, это не мешает работе)", src, err)
	}

	return src, nil
}

// migrateLegacyDB — точка входа миграции. dst и candidates — явные параметры
// (не paths), поэтому вся функция тестируется на временных каталогах, ни
// разу не трогая настоящий %LOCALAPPDATA%. Возвращает путь, откуда перенесли
// ("" — переносить было нечего или dst уже готов).
func migrateLegacyDB(dst string, candidates []string) (string, error) {
	dataDir := filepath.Dir(dst)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("не удалось создать каталог данных %s: %w", dataDir, err)
	}

	sweepForeignParts(dst)

	src, findErr := findLegacyDB(candidates)

	ready, dstErr := inspectDestination(dst)
	if dstErr != nil {
		if ce, ok := dstErr.(*CorruptDestinationError); ok {
			ce.Src = src
		}
		return "", dstErr
	}

	if ready {
		switch {
		case findErr != nil:
			log.Printf("inits: рядом с программой найдено несколько баз-кандидатов, но %s уже используется — миграция не нужна, кандидаты не тронуты: %v", dst, findErr)
		case src != "":
			log.Printf("inits: база рядом с программой (%s) игнорируется, используется %s", src, dst)
		}
		return "", nil
	}

	if findErr != nil {
		return "", findErr
	}
	if src == "" {
		return "", nil
	}

	return copyLegacyDB(src, dst)
}
