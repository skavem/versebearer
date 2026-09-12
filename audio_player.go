package main

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/gen2brain/malgo"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
)

// ⚠️ Инварианты конкурентности (audio-playlist-implementation.md, раздел
// «Инварианты конкурентности») — первая конкурентная подсистема в проекте,
// и большинство способов её сломать не падают с трейсом, а вешают окно или
// щёлкают в динамик посреди служения.
//
//	И1. beep ничего не защищает сам: Mixer — голый слайс без мьютекса
//	    (beep/v2@v2.1.1/mixer.go:6-25). Блокировку создаём мы: data-колбэк
//	    malgo (onSamples, ниже в этом файле) держит p.mu на всём цикле
//	    Stream. Поля mix/ctrl/mvol/src читаются и пишутся ТОЛЬКО под p.mu —
//	    в том числе на время seek: пока decode.Seek() переставляет курсор
//	    декодера, chain, ссылающийся на этот же src, должен быть уже вынут
//	    из mix (см. snapshotForSeek), иначе data-колбэк конкурентно дёрнет
//	    src.Stream() поверх того же файла/декодера, которым в этот момент
//	    рулит Seek() — гонка по позиции и по offset файлового дескриптора.
//	И2. Под p.mu нельзя звать malgo (Start/Stop/Uninit), файловый ввод-вывод
//	    и GORM. device.Stop()/Uninit() ждут worker-поток
//	    (malgo@v0.11.26/device.go:164-188), а наш data-колбэк выполняется
//	    НА НЁМ ЖЕ (device.go:195-217) — вызов Stop() из-под p.mu = дедлок
//	    навсегда. Паттерн для stop/close/setDevice: взять p.mu, скопировать
//	    указатель в локальную переменную и обнулить поле, отпустить mu, и
//	    только потом трогать malgo/файл.
//	И3. Цепочка (decode -> Resample -> Volume -> Seq) собирается ВНЕ p.mu:
//	    gomp3.NewDecoder проходит весь файл в конструкторе — дорого. Под
//	    p.mu — только mix.Clear(), mix.Add(chain) и присваивание полей.
//	    Старый src.Close() — после отпускания mu. То же для seek.
//	И4. Монотонный gen uint64, инкремент под p.mu на каждом play/stop/seek —
//	    В ТОЙ ЖЕ критической секции, что и снимок/расчистка mix для этой
//	    операции (иначе Stop() успевает проскочить между "прочитали src" и
//	    "зарезервировали gen", и устаревший Seek оживляет только что
//	    остановленный трек). done — канал с буфером 1: чужое (устаревшее)
//	    поколение молча отбрасывается — иначе оператор успевает нажать
//	    Стоп/выбрать другой трек, пока «трек доиграл» ещё летит из колбэка в
//	    горутину. Ровно по той же причине буфер-1 обязан дренироваться
//	    неблокирующим чтением в момент, когда старая цепочка вынимается из
//	    mix (stop/startTrack/snapshotForSeek) — иначе устаревший gen,
//	    осевший в буфере, займёт единственное место, и ЧЕСТНЫЙ done
//	    следующего трека попадёт в "default" и потеряется молча навсегда
//	    (beep.Callback зануляет свою функцию после первого вызова).
//	И5. Ровно один активный трек: play() начинается с mix.Clear(). Повторный
//	    Play на уже играющий itemId — no-op, не рестарт.
//	И6. Тестируемость: онсэмплс сведён к ~50 строкам конвертации буфера
//	    (audio_device.go), а весь микс/DSP движок здесь тестируется как
//	    обычный beep.Streamer — прямым вызовом Stream()/startTrack() без
//	    звуковой карты.
type player struct {
	mu  sync.Mutex
	ctx *malgo.AllocatedContext
	dev *malgo.Device
	// deviceOpen отделяет "устройство уже поднято" от валидности dev/ctx:
	// тесты подменяют openDevice на функцию, возвращающую (nil, nil, rate,
	// nil) — тогда deviceOpen=true, а dev/ctx остаются nil и ни разу не
	// разыменовываются (все обращения к ним — под "if dev != nil").
	deviceOpen bool
	// openMu сериализует ensureDevice целиком (включая сам вызов
	// openDevice — InitContext+InitDevice+Start, десятки миллисекунд).
	// Отдельный от p.mu мьютекс: аудио-колбэк его не берёт, поэтому И2 не
	// страдает. Без него два быстрых клика "играть" на первом за сессию
	// воспроизведении поднимали бы ДВА устройства, и проигравшая горутина
	// останавливала бы своё — то самое, которое победитель только что
	// начал использовать.
	openMu sync.Mutex
	// openDevice — точка внедрения для тестов (И6 в применении к самому
	// устройству): по умолчанию реально открывает malgo (audio_device.go,
	// defaultOpenDevice), в TestAudioServiceWithoutDevice подменяется на
	// функцию, возвращающую ошибку — без звуковой карты.
	// openDevice принимает выбранное устройство (malgo DeviceID.String(),
	// "" — системное по умолчанию) и возвращает вместе с контекстом/девайсом
	// его человекочитаемое имя — этап 3. Сопоставление строки с реальным
	// malgo.DeviceID делает сама реализация (defaultOpenDevice,
	// audio_device.go), потому что DeviceID.String() необратим (обрезает
	// хвостовые нули) и восстановить байты id из строки нельзя — только
	// найти совпадение перебором свежего списка устройств.
	openDevice func(deviceId string) (*malgo.AllocatedContext, *malgo.Device, uint32, string, error)
	// listPlaybackDevices — точка внедрения для тестов (И6), как и openDevice:
	// по умолчанию реально опрашивает malgo (audio_device.go,
	// defaultListPlaybackDevices), в тестах подменяется.
	listPlaybackDevices func() ([]malgo.DeviceInfo, error)

	// selectedDeviceId — устройство, которое должно открыться в следующем
	// ensureDevice (или уже открыто под этим именем). Не участвует в
	// realtime-цикле (onSamples его не читает), но живёт под тем же p.mu,
	// что и остальные поля текущего состояния плеера — здесь это просто
	// разделяемая между горутинами строка, а не что-то, что нужно защищать
	// отдельным мьютексом.
	selectedDeviceId string
	// deviceName — человекочитаемое имя УЖЕ открытого устройства (или
	// последнего, что открывалось), выставляется в ensureDevice. PlayerState
	// читает его в State(), а не пересчитывает через ListDevices() — тот
	// ходит в malgo и не должен вызываться 4 раза в секунду.
	deviceName string
	// deviceLost — этап 3: устройство пропало само (see onDeviceStopped),
	// не наш Stop(). atomic — читается из State() без p.mu, как peak/pos.
	// Сбрасывается в false при следующем успешном ensureDevice (см.
	// комментарий про expectStop в ensureDevice — деталь та же: общий на
	// весь плеер флаг обязан сбрасываться на каждое удачное открытие,
	// иначе после первой же смены устройства детект пропажи умирает
	// навсегда).
	deviceLost atomic.Bool

	mix  *beep.Mixer     // постоянный источник data-колбэка: пустой микшер отдаёт тишину (Mixer.stopWhenEmpty=false по умолчанию)
	mvol *effects.Volume // общая громкость, живёт между треками
	ctrl *beep.Ctrl      // пауза

	src      beep.StreamSeekCloser // декодер текущего трека — только он умеет Seek
	fileRate beep.SampleRate       // частота файла — для src.Seek при перемотке
	devRate  beep.SampleRate       // частота устройства — родная, не жёсткие 48000 (см. audio_device.go)
	gainDb   float64               // громкость текущего трека — нужна повторно при пересборке цепочки на seek
	status   playerStatus
	gen      uint64 // см. И4

	trackId    uint
	playlistId uint
	itemId     uint
	durationMs int
	nextTitle  string // считается один раз при смене трека — этап 4 (автопереход); пока всегда ""

	buf  [][2]float64  // выделен один раз при старте, чтобы САМ движок не аллоцировал в колбэке; конкретные декодеры внутри chain (например wav) всё равно могут аллоцировать на каждый Stream() — это их особенность, не повод удивляться профилю
	peak atomic.Uint64 // max(|L|,|R|) с момента прошлого чтения State(), сбрасывается при чтении (хранит float64 через math.Float64bits)
	pos  atomic.Int64  // кадров, реально отданных на устройство; State() читает без p.mu — гонка с UI-горутиной иначе

	done       chan uint64 // "трек доиграл": несёт поколение, см. И4. Буфер 1 — в канале максимум одно событие
	lost       chan struct{}
	expectStop atomic.Bool // отличает наш Stop()/ServiceShutdown() от пропажи устройства (этап 3)
}

type playerStatus string

const (
	statusIdle    playerStatus = "idle"
	statusPlaying playerStatus = "playing"
	statusPaused  playerStatus = "paused"
)

// trackMeta — то, что startTrack записывает в player поверх собранной вне
// p.mu цепочки. gainDb и fileRate кэшируются здесь же: Seek пересобирает
// Resample+Volume поверх того же декодера и не должен ходить в БД за ними.
type trackMeta struct {
	trackId    uint
	playlistId uint
	itemId     uint
	durationMs int
	fileRate   beep.SampleRate
	gainDb     float64
}

// newPlayer создаёт движок с постоянным микшером/громкостью/паузой —
// этим полям не нужно устройство, поэтому SetVolume работает сразу после
// создания сервиса, ещё до первого Play. initialVolume — общая громкость,
// initialDeviceId — последнее выбранное устройство, оба прочитаны из
// GlobalState при старте сервиса (см. audio_service.go).
func newPlayer(initialVolume float64, initialDeviceId string) *player {
	p := &player{
		mix:              &beep.Mixer{},
		buf:              make([][2]float64, 1024),
		done:             make(chan uint64, 1),
		lost:             make(chan struct{}, 1),
		selectedDeviceId: initialDeviceId,
	}
	p.ctrl = &beep.Ctrl{Streamer: p.mix}
	p.mvol = &effects.Volume{Streamer: p.ctrl}
	p.status = statusIdle
	p.openDevice = p.defaultOpenDevice
	p.listPlaybackDevices = defaultListPlaybackDevices
	p.setVolumeLocked(initialVolume)
	return p
}

// selectedDevice читает выбранное устройство под p.mu — GetDeviceId и
// ensureDevice.
func (p *player) selectedDevice() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.selectedDeviceId
}

// setSelectedDevice — SetDevice: запоминает новый выбор. Не трогает
// deviceOpen/dev/ctx сама по себе — вызывающий (AudioService.SetDevice)
// обязан отдельно закрыть уже открытое устройство (closeDevice, вне p.mu —
// И2), чтобы следующий ensureDevice открыл именно это, а не продолжал
// молча играть на старом.
func (p *player) setSelectedDevice(id string) {
	p.mu.Lock()
	p.selectedDeviceId = id
	p.mu.Unlock()
	p.deviceLost.Store(false) // оператор сам разобрался с пропажей, выбрав устройство
}

// currentItem — playlistId/itemId сейчас загруженного (играющего или на
// паузе) элемента, если такой есть. Использует Next/Prev/SetPlaylistFlags,
// которым нужно знать, что именно сейчас играет, не читая приватные поля
// напрямую.
func (p *player) currentItem() (playlistId, itemId uint, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playlistId, p.itemId, p.status != statusIdle
}

// isCurrentItem — как isCurrentTrack, но по itemId элемента плейлиста:
// RemoveFromPlaylist должен остановить воспроизведение именно того элемента,
// который удаляют, даже если formально это тот же trackId, что где-то ещё в
// плейлисте (isCurrentTrack такое совпадение не различил бы).
func (p *player) isCurrentItem(itemId uint) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status != statusIdle && p.itemId == itemId
}

// isCurrentPlaylist — RemovePlaylist/ReorderPlaylist: остановить, если сейчас
// играет (или на паузе) что-то из именно этого плейлиста.
func (p *player) isCurrentPlaylist(playlistId uint) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status != statusIdle && p.playlistId == playlistId
}

// setNextTitle — этап 4: заголовок элемента, который автопереход поставит
// следующим (или "", если следующего нет/автопереход выключен). Отдельный
// сеттер, а не прямое присваивание поля: вызывается ПОСЛЕ startTrack, который
// сам обнуляет nextTitle как часть сброса нового трека — порядок вызовов
// важен и хочется, чтобы он был виден в одном месте.
func (p *player) setNextTitle(title string) {
	p.mu.Lock()
	p.nextTitle = title
	p.mu.Unlock()
}

// setVolumeLocked применяет общую громкость (слайдер 0..1) к mvol. Вызывать
// только под p.mu (mvol — одно из полей, защищённых И1).
//
// Base: 2, Volume: 10*(v-1) — эмпирическая кривая: 0..1 отображается на
// примерно -60..0 дБ с плавным, а не резким на конце спадом. Silent
// обязателен отдельным флагом: math.Pow(base, x) математически никогда не
// даёт ровно 0, и на v=0 без Silent игра продолжилась бы чуть слышно.
func (p *player) setVolumeLocked(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.mvol.Base = 2
	p.mvol.Volume = 10 * (v - 1)
	p.mvol.Silent = v <= 0
}

// currentVolumeLocked восстанавливает слайдер 0..1 из mvol.Volume по
// формуле, обратной setVolumeLocked. Вызывать только под p.mu.
func (p *player) currentVolumeLocked() float64 {
	if p.mvol.Silent {
		return 0
	}
	return p.mvol.Volume/10 + 1
}

// nextGen резервирует новое поколение (И4) — вызывается в самом начале
// play/stop/seek, ДО дорогой работы (декодирование файла и т.п.), чтобы
// уже летящая операция, которую эта только что отменила, могла это
// обнаружить в startTrack/resumeChain/finishIfCurrent.
func (p *player) nextGen() uint64 {
	p.mu.Lock()
	p.gen++
	g := p.gen
	p.mu.Unlock()
	return g
}

// deviceRateSnapshot читает частоту устройства под p.mu — нужна Play() для
// построения Resample() до захвата mu (И3): сама сборка цепочки дорогая и
// не должна происходить под блокировкой.
func (p *player) deviceRateSnapshot() beep.SampleRate {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.devRate
}

// isPlayingItem — И5: повторный Play на уже играющий itemId должен быть
// no-op, а не рестартом. Намеренно НЕ считает статус paused: клик "играть"
// по строке списка на приостановленном треке рестартует его с начала — это
// отдельное, осознанное решение (возобновление с места — только Toggle()).
func (p *player) isPlayingItem(itemId uint) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status == statusPlaying && p.itemId == itemId
}

// isCurrentTrack сообщает, загружен ли (играет или на паузе) именно этот
// trackId прямо сейчас — используется RemoveTrack, чтобы синхронно
// остановить воспроизведение перед удалением файла (иначе os.Remove на
// Windows падает с sharing violation, пока открыт декодер).
func (p *player) isCurrentTrack(trackId uint) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status != statusIdle && p.trackId == trackId
}

// drainDoneLocked неблокирующе вычитывает done, если в буфере (ёмкостью 1)
// осел чужой gen, который уже никто не заберёт: p.mix.Clear(), после
// которого эта функция обычно вызывается, гарантирует, что старая цепочка
// (и её beep.Callback) больше никогда не выполнится и второго done для неё
// не будет — самое время освободить единственное место в буфере для
// честного done следующего трека (см. И4). Вызывать только под p.mu.
func (p *player) drainDoneLocked() {
	select {
	case <-p.done:
	default:
	}
}

// resetTrackFieldsLocked приводит поля текущего трека в состояние покоя.
// Не трогает mix/gen/устройство — вызывающий решает, что делать с ними.
// Вызывать только под p.mu.
func (p *player) resetTrackFieldsLocked() {
	p.src = nil
	p.status = statusIdle
	p.trackId, p.playlistId, p.itemId = 0, 0, 0
	p.durationMs = 0
	p.nextTitle = ""
	p.ctrl.Paused = false
	p.pos.Store(0)
	p.peak.Store(0)
}

// buildChain собирает общую хвостовую часть цепочки — Resample -> Volume ->
// Seq(..., Callback(done<-gen)) — используемую и Play() (после decode), и
// Seek() (после src.Seek() на уже открытом декодере). Вынесена отдельно,
// чтобы её можно было прогнать в тесте (TestSeekRebuildNoStaleTail) без
// похода в БД/файлы — источником достаточно любого beep.Streamer.
//
// Base: 10, Volume: GainDb/20 — ПРАВИЛЬНО (амплитуда = 10^(дБ/20)).
// Собственная документация beep (effects/volume.go:12-13) советует dB/10 —
// не верить, это дало бы двукратную ошибку в децибелах.
func buildChain(src beep.Streamer, fileRate, devRate beep.SampleRate, gainDb float64, done chan<- uint64, gen uint64) beep.Streamer {
	resampled := beep.Resample(4, fileRate, devRate, src)
	withGain := &effects.Volume{Streamer: resampled, Base: 10, Volume: gainDb / 20}
	return beep.Seq(withGain, beep.Callback(func() {
		select {
		case done <- gen:
		default:
		}
	}))
}

// startTrack встраивает уже собранную (вне p.mu — И3) цепочку chain как
// единственный активный источник микшера (И5: mix.Clear() первым делом) и
// обновляет метаданные текущего трека. gen должен быть тем самым, что
// resurvirovan nextGen() перед сборкой chain: если к этому моменту кто-то
// успел начать более новую операцию (play/stop/seek), p.gen уже другой —
// chain отбрасывается как устаревший, и вызывающий обязан закрыть
// переданный src сам (возвращается тем же значением в staleSrc).
//
// installed==true: staleSrc — старый p.src (или nil), который теперь тоже
// на совести вызывающего — закрыть его нужно ПОСЛЕ отпускания p.mu (И2/И3).
func (p *player) startTrack(gen uint64, chain beep.Streamer, src beep.StreamSeekCloser, meta trackMeta) (installed bool, staleSrc beep.StreamSeekCloser) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gen != gen {
		return false, src
	}
	staleSrc = p.src
	p.mix.Clear()
	p.drainDoneLocked() // старая цепочка больше никогда не выполнится — см. И4
	p.mix.Add(chain)
	p.src = src
	p.fileRate = meta.fileRate
	p.gainDb = meta.gainDb
	p.trackId, p.playlistId, p.itemId = meta.trackId, meta.playlistId, meta.itemId
	p.durationMs = meta.durationMs
	p.nextTitle = ""
	p.status = statusPlaying
	p.ctrl.Paused = false
	p.pos.Store(0)
	p.peak.Store(0)
	return true, staleSrc
}

// seekSnapshot — то, что Seek читает под p.mu перед пересборкой ресемплера
// и громкости (И3: "то же для seek" — пересборка вне p.mu, здесь только
// чтение исходных данных для неё).
type seekSnapshot struct {
	src      beep.StreamSeekCloser
	fileRate beep.SampleRate
	devRate  beep.SampleRate
	gainDb   float64
}

// snapshotForSeek — единственная правильная точка входа для перемотки.
// Атомарно, одной критической секцией под p.mu:
//  1. резервирует новое поколение (И4) — в ТОЙ ЖЕ секции, что и снимок,
//     иначе Stop() успел бы проскочить между "прочитали src" и
//     "зарезервировали gen", и устаревший Seek оживил бы только что
//     остановленный трек (см. resumeChain, который дополнительно сверяет
//     src);
//  2. отцепляет текущую цепочку от микшера (mix.Clear()) — КРИТИЧНО (И1):
//     data-колбэк держит p.mu на всём цикле Stream, но сам цикл вызывается
//     БЕЗ mu тем, кто зовёт Seek(); если оставить старую цепочку (а значит
//     и src) в mix на время src.Seek() вне p.mu, колбэк на аудиопотоке
//     конкурентно вызовет chain.Stream() -> src.Stream() на том же самом
//     декодере, которым в этот момент рулит Seek(), — гонка по позиции
//     чтения и по offset файлового дескриптора (для wav буквально
//     одновременные Read()/Seek() на одном *os.File). После mix.Clear()
//     декодер физически недостижим для колбэка, и снаружи mu с ним можно
//     работать сколько угодно;
//  3. дренирует done (см. drainDoneLocked) — старая цепочка больше не
//     сыграет и не пришлёт своё "доиграл".
//
// ok=false, если сейчас нечего перематывать — тогда ничего не тронуто.
func (p *player) snapshotForSeek() (snap seekSnapshot, gen uint64, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.src == nil {
		return seekSnapshot{}, 0, false
	}
	p.gen++
	p.mix.Clear()
	p.drainDoneLocked()
	return seekSnapshot{src: p.src, fileRate: p.fileRate, devRate: p.devRate, gainDb: p.gainDb}, p.gen, true
}

// resumeChain — как startTrack, но для seek: тот же трек и тот же src
// (только что переставленный на новую позицию его собственным Seek), новая
// цепочка Resample+Volume поверх него. beep.Resampler не сбрасывается
// (resample.go:78-86, buf1/buf2/off/pos) — поэтому при seek всегда
// строится НОВЫЙ Resample, а не переиспользуется старый: иначе первые
// доли секунды после перемотки звучал бы хвост старого места.
//
// expectedSrc — тот же snap.src, что вернул snapshotForSeek. Сверка
// p.src == expectedSrc — вторая половина защиты от воскрешения (первая —
// gen, зарезервированный в snapshotForSeek): если между снимком и этим
// вызовом кто-то успел Stop() (p.src стал nil) или начать новый Play()
// (p.src указывает на другой декодер), gen там тоже уже другой — эта
// проверка избыточна для чистого gen-инварианта, но не позволяет по
// ошибке сравнить с самим собой при случайном совпадении счётчика после
// переполнения uint64 (за пределами разумного, но бесплатно).
func (p *player) resumeChain(gen uint64, chain beep.Streamer, expectedSrc beep.StreamSeekCloser, posFrames int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gen != gen || p.src != expectedSrc {
		return false
	}
	p.mix.Clear() // уже пусто после snapshotForSeek, но явное И5 дешевле догадок
	p.mix.Add(chain)
	p.pos.Store(posFrames)
	p.peak.Store(0)
	return true
}

// finishIfCurrent обрабатывает "трек доиграл" (done <- gen из Seq/Callback
// в собранной цепочке). Чужое (устаревшее) поколение молча отбрасывается —
// И4: оператор мог успеть нажать Стоп или запустить другой трек, пока это
// уведомление летело из data-колбэка в горутину-получателя.
//
// finished — playlistId/itemId/trackId только что завершившегося элемента:
// AudioService.tryAdvance (этап 4) использует его, чтобы понять, какой
// заранее подготовленный "следующий" трек имелся в виду, ДО того как
// resetTrackFieldsLocked обнулит эти поля.
//
// nextGen — новое поколение, зарезервированное В ТОЙ ЖЕ критической секции
// (см. И4, по аналогии со snapshotForSeek): если tryAdvance решит поставить
// следующий трек, он обязан использовать именно nextGen, а не звать nextGen()
// отдельно. Иначе между "трек доиграл" и "поставили следующий" остаётся окно,
// где конкурентный Stop()/Play() резервирует СВОЙ, более новый gen, а
// tryAdvance следом всё равно резервирует свой (ещё более новый) и
// перезаписывает решение оператора. Резервируя nextGen здесь же, под тем же
// p.mu, любой конкурентный Stop()/Play() либо успевает раньше (и его gen
// оказывается больше nextGen — startTrack в tryAdvance тогда откажет), либо
// позже (и увидит уже расставленное tryAdvance состояние) — обычная
// сериализация мьютексом, без отдельного зазора для гонки.
func (p *player) finishIfCurrent(gen uint64) (oldSrc beep.StreamSeekCloser, finished trackMeta, nextGen uint64, changed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if gen != p.gen {
		return nil, trackMeta{}, 0, false
	}
	oldSrc = p.src
	finished = trackMeta{trackId: p.trackId, playlistId: p.playlistId, itemId: p.itemId, durationMs: p.durationMs}
	p.mix.Clear()
	p.resetTrackFieldsLocked()
	p.gen++
	return oldSrc, finished, p.gen, true
}

// stop — общая часть AudioService.Stop: инвалидирует поколение (И4),
// расчищает микшер (И5) и возвращает старый src, который вызывающий обязан
// закрыть уже вне p.mu (И2/И3).
func (p *player) stop() (oldSrc beep.StreamSeekCloser) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gen++
	oldSrc = p.src
	p.mix.Clear()
	p.drainDoneLocked() // см. И4 — старая цепочка больше не выполнится
	p.resetTrackFieldsLocked()
	return oldSrc
}

// forceIdleOnDeviceLost — то же самое, что stop(), но по инициативе
// data-устройства (StopProc, см. audio_device.go), а не оператора. Отдельное
// имя — для читаемости на месте вызова, поведение идентично stop().
func (p *player) forceIdleOnDeviceLost() (oldSrc beep.StreamSeekCloser) {
	return p.stop()
}

// toggle переключает паузу. No-op, если ничего не загружено или трек уже
// доиграл (idle).
func (p *player) toggle() {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch p.status {
	case statusPlaying:
		p.ctrl.Paused = true
		p.status = statusPaused
	case statusPaused:
		p.ctrl.Paused = false
		p.status = statusPlaying
	}
}

// onSamples — data-колбэк malgo (DataProc), выполняется на worker-потоке
// malgo (device.go:195-217). Держит p.mu на ВСЁМ цикле Stream (И1): это
// единственное место, где mvol.Stream() (а через него — ctrl/mix/src)
// вызывается на аудиопотоке, и единственное место, где play/stop/seek
// могут на долю миллисекунды заблокироваться, ожидая конца текущего
// буфера. Здесь нет и не должно быть malgo/файлового IO/БД сверх обычных
// буферизованных чтений decoder-а внутри Stream() — те не подпадают под
// запрет И2 (он про os.Open/конструктор декодера, дорогие и одноразовые;
// см. И3), а не про штатное чтение следующего чанса потока.
func (p *player) onSamples(out, _ []byte, frames uint32) {
	// Байтовый буфер malgo рассчитан на ФАКТИЧЕСКИЙ канал/формат устройства
	// (device.go: sampleCount = frameCount*channels, sizeInBytes по
	// формату) — сейчас мы всегда просим 2 канала float32 (audio_device.go),
	// поэтому len(out) обязан быть не меньше frames*2*4 байт. Явная
	// проверка — на случай будущей смены конфигурации (этап 3, другое
	// устройство): без неё unsafeFloat32Slice молча уедет за границу
	// выделенной malgo памяти.
	if frames == 0 || len(out) < int(frames)*2*4 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	dst := unsafeFloat32Slice(out, int(frames)*2)
	var peak float64
	done := 0
	for done < int(frames) {
		chunk := p.buf
		if rest := int(frames) - done; rest < len(chunk) {
			chunk = chunk[:rest]
		}
		n, ok := p.mvol.Stream(chunk)
		for i := 0; i < n; i++ {
			l, r := chunk[i][0], chunk[i][1]
			dst[(done+i)*2] = float32(l)
			dst[(done+i)*2+1] = float32(r)
			if a := math.Abs(l); a > peak {
				peak = a
			}
			if a := math.Abs(r); a > peak {
				peak = a
			}
		}
		done += n
		if !ok || n == 0 {
			for i := done; i < int(frames); i++ {
				dst[i*2], dst[i*2+1] = 0, 0
			}
			break
		}
	}

	// Позиция двигается, только пока реально что-то играет. Проверять один
	// Paused недостаточно: resetTrackFieldsLocked (Stop/finishIfCurrent)
	// ставит Paused=false, а пустой Mixer по умолчанию честно отдаёт
	// "тишину, ok=true" (stopWhenEmpty=false) — то есть done продолжал бы
	// расти и после Стоп, пока идёт хоть один цикл колбэка. status —
	// единственный надёжный признак "что-то реально загружено и не на
	// паузе".
	if p.status == statusPlaying {
		p.pos.Add(int64(done))
	}
	// Пик — max с момента прошлого чтения State(), а не мгновенная выборка
	// (иначе индикатор уровня 4 раза в секунду показывал бы случайные
	// отсчёты). CAS-цикл: peak читается конкурентно из State() без p.mu.
	for {
		oldBits := p.peak.Load()
		if peak <= math.Float64frombits(oldBits) {
			break
		}
		if p.peak.CompareAndSwap(oldBits, math.Float64bits(peak)) {
			break
		}
	}
}

// onDeviceStopped — StopProc malgo, тоже выполняется на worker-потоке.
// expectStop отличает наш собственный Stop()-путь (ServiceShutdown) от
// того, что устройство пропало само (выдернули провод, забрало другое
// приложение — обрабатывается в этапе 3). И2: ни p.mu, ни emit синхронно
// здесь — только атомик и неблокирующая отправка в канал с буфером 1.
func (p *player) onDeviceStopped() {
	if p.expectStop.Load() {
		return
	}
	select {
	case p.lost <- struct{}{}:
	default:
	}
}
