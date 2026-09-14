import type {
  AudioTrack,
  Playlist,
} from "$lib/bindings/changeme/backend/models";
import {
  AddToPlaylist,
  AddTracksToPlaylist,
  ClearPlaylist,
  CreatePlaylist,
  FadeOutStop,
  GetDeviceId,
  ImportTrack,
  ListDevices,
  ListPlaylists,
  ListTracks,
  Next,
  PickAudioFiles,
  Play,
  Prev,
  RemoveFromPlaylist,
  RemovePlaylist,
  RemoveTrack,
  RenamePlaylist,
  ReorderPlaylist,
  Seek,
  SetDevice,
  SetPlaylistFlags,
  SetVolume,
  State,
  Stop,
  Toggle,
  UpdateTrack,
} from "$lib/bindings/changeme/audioservice";
import type {
  AudioDevice,
  ImportTrackResult,
  PlayerState,
  PlaylistFlagsInput,
  TrackInput,
} from "$lib/bindings/changeme/models";
import { Events } from "@wailsio/runtime";

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

// audioExtensions — зеркало backend audioFilePattern (audio_import.go):
// четыре нативных формата beep плюс те, что при импорте конвертируются в
// FLAC через ffmpeg. Используется ТОЛЬКО для перетаскивания файлов из
// системы (правка 2) — фильтр по расширению отсеивает явно не то ДО
// хеширования/копирования, а не вместо серверной проверки: ImportTrack
// всё равно откажет с понятной ошибкой на файле с "правильным"
// расширением, но чужим содержимым (см. audio_import.go).
const audioExtensions = new Set([
  ".mp3",
  ".wav",
  ".flac",
  ".ogg",
  ".oga",
  ".m4a",
  ".aac",
  ".wma",
  ".opus",
  ".aiff",
  ".aif",
  ".alac",
  ".wv",
]);

// fileExtension — путь может прийти с обратными слэшами (Windows) даже в
// строке, полученной от Go, поэтому режем по обоим разделителям, а не
// полагаемся на браузерный path.
function fileExtension(path: string): string {
  const name = path.split(/[/\\]/).pop() ?? path;
  const idx = name.lastIndexOf(".");
  return idx > 0 ? name.slice(idx).toLowerCase() : "";
}

function hasAudioExtension(path: string): boolean {
  return audioExtensions.has(fileExtension(path));
}

// ⚠️ Отступление от образца (songsStore.svelte.ts): при module-init тянем
// только треки. Устройства (ListDevices) и плейлисты (ListPlaylists)
// сознательно НЕ тянутся здесь: ListDevices запустил бы malgo.InitContext
// ещё до открытия вкладки «Звук», и обещание «на машине без звуковой карты
// старт не ломается» перестало бы выполняться. Обе загрузки запускает сама
// вкладка из своего $effect (см. Audio.svelte) — refreshDevices/refreshPlaylists
// ниже, вызываемые оттуда, а не при импорте модуля.
const createAudioStore = () => {
  let tracksList = $state<AudioTrack[]>([]);
  let tracksLoading = $state(true);

  let importing = $state(false);
  let importProgress = $state<{ done: number; total: number } | null>(null);
  let importCancelRequested = false;

  let devicesList = $state<AudioDevice[]>([]);
  let devicesLoading = $state(true);
  let selectedDeviceId = $state("");

  let playlistsList = $state<Playlist[]>([]);
  let playlistsLoading = $state(true);
  // activePlaylistId — источник истины "какой плейлист активен" держим как
  // ПРИМИТИВ, не как объект: playlists.list пересоздаёт весь массив (и все
  // объекты в нём) новыми ссылками на каждый audio_playlists_update, даже
  // если содержимое не изменилось. Раньше зеркальный activePlaylist
  // ($state<Playlist|null>) переприсваивался на КАЖДУЮ такую мутацию — и
  // любой $effect/$derived, читавший playlists.active, срабатывал заново
  // просто от смены ссылки, а не значения (баг: перестановка сбрасывала
  // клавиатурный выбор в Audio.svelte). playlists.active ниже — чистый
  // геттер поверх этого id + актуального списка, безопасный к пересозданию
  // массива.
  let activePlaylistId = $state<number | null>(null);

  // playerState — снимок PlayerState, опрашиваемый по таймеру, пока вкладка
  // «Звук» открыта (startPolling/stopPolling, из $effect Audio.svelte — план,
  // этап 2: "State() не должен ходить в БД", но опрос всё равно не бесплатен,
  // поэтому не крутится, пока никто не смотрит).
  let playerState = $state<PlayerState | null>(null);
  let pollTimer: ReturnType<typeof setInterval> | null = null;

  // Последняя ошибка бэка — и асинхронная (audio_error от фоновых задач:
  // замер громкости, чтение медиатеки), и из явных действий оператора
  // (RemoveTrack/UpdateTrack). Компонент вкладки показывает её одним общим
  // алертом, чтобы отказ не проходил бесследно.
  let lastError = $state<string | null>(null);

  // Полный TrackInput из частичного патча — незаданные поля идут как null,
  // что бэк (UpdateTrack в audio_service.go) трактует как «не трогать» (см.
  // тот же приём в visualStore.svelte.ts для StyleInput).
  function toTrackInput(patch: Partial<TrackInput>): TrackInput {
    return {
      title: patch.title ?? null,
      artist: patch.artist ?? null,
      trimStartMs: patch.trimStartMs ?? null,
      trimEndMs: patch.trimEndMs ?? null,
      gainDb: patch.gainDb ?? null,
    };
  }

  const tracks = {
    get loading() {
      return tracksLoading;
    },
    get list() {
      return tracksList;
    },
    set list(v: AudioTrack[]) {
      tracksList = v;
      tracksLoading = false;
    },
  };

  const devices = {
    get loading() {
      return devicesLoading;
    },
    get list() {
      return devicesList;
    },
    get selectedId() {
      return selectedDeviceId;
    },
  };

  const playlists = {
    get loading() {
      return playlistsLoading;
    },
    get list() {
      return playlistsList;
    },
    set list(v: Playlist[]) {
      playlistsList = v;
      playlistsLoading = false;
    },
    // active — вычисляется каждый раз из activePlaylistId + актуального
    // списка, а не хранится отдельным $state-зеркалом (см. комментарий у
    // activePlaylistId выше). Если ID пропал из списка (плейлист удалили
    // где-то ещё) — активного просто нет, без отдельной синхронизации.
    get active() {
      return (
        playlistsList.find((p) => p.ID === activePlaylistId) ?? null
      );
    },
    set active(v: Playlist | null) {
      activePlaylistId = v?.ID ?? null;
    },
  };

  const player = {
    get state() {
      return playerState;
    },
  };

  // runCommand — общая обёртка команд транспорта, у которых нет результата,
  // только факт отказа: ошибка бэка попадает в общий алерт вкладки
  // (lastError), а не в unhandled rejection. Одна обёртка вместо копии
  // try/catch в каждой команде — иначе первая же правка (скажем, префикс в
  // тексте ошибки) применилась бы к части из них.
  async function runCommand(call: () => Promise<unknown>): Promise<void> {
    try {
      await call();
    } catch (e) {
      lastError = errorMessage(e);
    }
  }

  async function refreshPlayerState() {
    try {
      playerState = await State();
    } catch (e) {
      // Без try/catch reject из setInterval (см. startPolling) уходил бы в
      // unhandled rejection и замораживал бы последний снимок навсегда —
      // кнопки транспорта выглядели бы активными, а «Стоп» визуально не
      // срабатывал бы, хотя бэк уже остановился.
      lastError = errorMessage(e);
    }
  }

  Events.On(
    "audio_tracks_update",
    ({ data }: { data: AudioTrack[] }) => (tracks.list = data ?? []),
  );
  Events.On("audio_error", ({ data }: { data: string }) => {
    lastError = data;
  });
  Events.On(
    "audio_playlists_update",
    ({ data }: { data: Playlist[] }) => (playlists.list = data ?? []),
  );
  // audio_track_changed несёт готовый PlayerState (Play уже собрал его через
  // State() на бэке) — используем как есть, не дожидаясь следующего опроса.
  Events.On(
    "audio_track_changed",
    ({ data }: { data: PlayerState }) => (playerState = data),
  );
  // audio_stopped/audio_device_lost несут nil — перечитываем явно, чтобы не
  // рисовать устаревший "играет" до следующего тика опроса.
  Events.On("audio_stopped", () => {
    refreshPlayerState();
  });
  Events.On("audio_device_lost", () => {
    lastError = "устройство вывода пропало";
    refreshPlayerState();
  });
  // audio_files_dropped — правка 2 (перетаскивание файлов из системы прямо
  // в плейлист): main.go пересылает СЮДА настоящие пути (webview-инпуту они
  // недоступны), а какой плейлист активен — знает только фронт, поэтому
  // импорт в него запускается уже здесь, не в Go. handleFilesDropped — метод
  // самого стора (ниже), не отдельная замыкающая функция: ему нужен доступ
  // к importing/importProgress, которые уже показывает Audio.svelte.
  Events.On("audio_files_dropped", ({ data }: { data: string[] }) => {
    void store.handleFilesDropped(data ?? []);
  });

  const store = {
    tracks,

    get importing() {
      return importing;
    },
    get importProgress() {
      return importProgress;
    },
    get error() {
      return lastError;
    },
    clearError() {
      lastError = null;
    },

    async pickFiles(): Promise<string[]> {
      return (await PickAudioFiles()) ?? [];
    },

    /**
     * Импортирует файлы один за другим, а не разом: при 50 файлах синхронный
     * замер громкости на бэке занял бы минуты без возможности отмены — поэтому
     * громкость там измеряется в фоне, а здесь цикл на фронте даёт «обработано
     * N из M» и точку для отмены между файлами (см.
     * .omc/plans/audio-playlist-implementation.md, п. 1.5).
     */
    async importFiles(paths: string[]): Promise<ImportTrackResult[]> {
      if (importing) {
        // MEDIUM №4 обзора: повторный вход (вторая пачка брошена, пока
        // первая ещё не завершилась) — раньше обе конкурентные цепочки
        // насчитывали пересекающиеся Position (см. AddTracksToPlaylist), а
        // первая, завершившись, снимала importing по finally и «Отмена»
        // переставала действовать на вторую. Проще и безопаснее отказать
        // второй пачке целиком, чем пытаться их тихо сериализовать.
        lastError = "импорт уже идёт — дождитесь его окончания";
        return [];
      }
      importing = true;
      importCancelRequested = false;
      importProgress = { done: 0, total: paths.length };

      const results: ImportTrackResult[] = [];
      try {
        for (let i = 0; i < paths.length; i++) {
          if (importCancelRequested) break;
          const res = await ImportTrack(paths[i]);
          if (res) results.push(res);
          importProgress = { done: i + 1, total: paths.length };
        }
      } finally {
        // try/finally: любой reject (например, отменённый CancellablePromise)
        // обязан снять importing, иначе кнопка «Импорт» блокируется до
        // перезапуска приложения.
        importing = false;
        importProgress = null;
      }
      return results;
    },

    cancelImport() {
      importCancelRequested = true;
    },

    /**
     * Импортирует файлы И сразу добавляет каждый успешно импортированный
     * трек в указанный плейлист — правка 1 ("Файлы добавляются сразу в
     * плейлист, а не сначала в библиотеку"). Дубликаты (Duplicate: true) тоже
     * добавляются: оператор мог намеренно перетащить уже импортированный
     * файл ещё раз, в другой плейлист или в этот же — запрета на два
     * элемента одного трека в списке нет.
     *
     * Ошибки отдельных файлов агрегируются в lastError здесь же (общий
     * алерт вкладки), а не пробрасываются наружу отдельным состоянием: и
     * кнопка «Импорт», и перетаскивание (handleFilesDropped ниже) идут через
     * этот метод, так что дублировать агрегацию в каждом вызывающем не нужно.
     *
     * Один AddTracksToPlaylist пачкой, а не AddToPlaylist на каждый файл:
     * последний на бэке эмитит полный audio_playlists_update на КАЖДЫЙ
     * вызов — на 50 файлах это 50 лишних полных перерисовок вкладки, пока
     * играет звук. Пачка — одна транзакция, один emit.
     */
    async importFilesToPlaylist(
      paths: string[],
      playlistId: number,
    ): Promise<ImportTrackResult[]> {
      const results = await this.importFiles(paths);
      const importedTrackIds = results
        .filter((r) => r.track && !r.error)
        .map((r) => r.track!.ID);
      if (importedTrackIds.length > 0) {
        await AddTracksToPlaylist(playlistId, importedTrackIds);
      }
      await this.refreshPlaylists();

      const errors = results.filter((r) => r.error).map((r) => r.error);
      if (errors.length === 1) {
        lastError = `не удалось импортировать файл: ${errors[0]}`;
      } else if (errors.length > 1) {
        lastError = `не удалось импортировать ${errors.length} файл(ов): ${errors[0]}`;
      }
      return results;
    },

    /**
     * Обрабатывает audio_files_dropped (правка 2, перетаскивание файлов из
     * системы на список плейлиста) — не-аудио расширения отфильтровываются
     * здесь же, до попытки импорта (audioExtensions), остальное идёт тем же
     * потоком, что и кнопка «Импорт». Нет активного плейлиста — оператор ещё
     * не выбрал/не создал ни одного: явная ошибка вместо тихого игнора.
     */
    async handleFilesDropped(paths: string[]) {
      if (paths.length === 0) return;
      const active = playlists.active;
      if (!active) {
        lastError =
          "выберите или создайте плейлист слева, прежде чем перетаскивать файлы";
        return;
      }
      const supported = paths.filter(hasAudioExtension);
      const unsupportedCount = paths.length - supported.length;
      if (supported.length > 0) {
        await this.importFilesToPlaylist(supported, active.ID);
      }
      if (unsupportedCount > 0 && supported.length === 0) {
        lastError = `формат не поддерживается: ${unsupportedCount} файл(ов)`;
      } else if (unsupportedCount > 0) {
        lastError = `${unsupportedCount} файл(ов) пропущено — формат не поддерживается`;
      }
    },

    async update(id: number, patch: Partial<TrackInput>) {
      try {
        const updated = await UpdateTrack(id, toTrackInput(patch));
        if (updated) {
          tracks.list = tracksList.map((t) => (t.ID === id ? updated : t));
        }
        return updated;
      } catch (e) {
        lastError = errorMessage(e);
        return null;
      }
    },

    async remove(id: number): Promise<boolean> {
      try {
        await RemoveTrack(id);
        return true;
      } catch (e) {
        lastError = errorMessage(e);
        return false;
      }
    },

    devices,
    async refreshDevices() {
      try {
        const [list, id] = await Promise.all([ListDevices(), GetDeviceId()]);
        devicesList = list ?? [];
        selectedDeviceId = id ?? "";
      } catch (e) {
        lastError = errorMessage(e);
      } finally {
        devicesLoading = false;
      }
    },
    async setDevice(id: string): Promise<boolean> {
      try {
        await SetDevice(id);
        selectedDeviceId = id;
        return true;
      } catch (e) {
        lastError = errorMessage(e);
        return false;
      }
    },

    playlists,
    async refreshPlaylists() {
      try {
        const list = (await ListPlaylists()) ?? [];
        playlists.list = list;
        // LOW обзора: миграция версии 8 сеет один плейлист по умолчанию
        // именно затем, чтобы импорту было куда идти (seedDefaultPlaylist,
        // backend/inits/db.go) — без автовыбора первого замысел доставлен
        // наполовину: activePlaylistId остаётся null после старта, кнопка
        // импорта недоступна, а бросок файлов отвечает "выберите плейлист",
        // хотя плейлист уже есть.
        if (activePlaylistId === null && list.length > 0) {
          playlists.active = list[0];
        }
      } catch (e) {
        lastError = errorMessage(e);
        playlists.list = [];
      }
    },
    async createPlaylist(name: string) {
      const created = await CreatePlaylist(name);
      if (created) playlists.active = created;
      return created;
    },
    async renamePlaylist(id: number, name: string) {
      await RenamePlaylist(id, name);
    },
    async removePlaylist(id: number) {
      // Активный плейлист снимается только ПОСЛЕ ответа бэка и только если
      // он реально пропал из списка — RemovePlaylist теперь может отказать
      // (последний плейлист, LOW обзора) и вернуть список без изменений;
      // раньше active обнулялся заранее и безусловно, и отказ бэка оставлял
      // оператора с несуществующим "выбором" при живом плейлисте.
      const list = await RemovePlaylist(id);
      if (playlists.active?.ID === id && !(list ?? []).some((p) => p.ID === id)) {
        playlists.active = null;
      }
    },
    // clearPlaylist — «Удалить всё» (правка 1): очищает список ПЛЮС удаляет
    // из медиатеки+с диска треки, которые перестали использоваться хоть
    // где-то (ClearPlaylist на бэке, тем же правилом, что и одиночный
    // removeFromPlaylist).
    async clearPlaylist(id: number) {
      await ClearPlaylist(id);
    },
    async setPlaylistFlags(id: number, patch: Partial<PlaylistFlagsInput>) {
      await SetPlaylistFlags(id, {
        autoAdvance: patch.autoAdvance ?? null,
        loop: patch.loop ?? null,
        fadeMs: patch.fadeMs ?? null,
      });
    },
    async addToPlaylist(playlistId: number, trackId: number) {
      await AddToPlaylist(playlistId, trackId);
    },
    async removeFromPlaylist(itemId: number) {
      await RemoveFromPlaylist(itemId);
    },
    async reorderPlaylist(playlistId: number, itemIds: number[]) {
      await ReorderPlaylist(playlistId, itemIds);
    },

    player,
    // isPlaylistOnAir — играет (или на паузе) элемент ИМЕННО этого плейлиста.
    // В сторе, а не в компонентах: спрашивают оба куска вкладки (сайдбар — в
    // тексте модалки удаления плейлиста, панель — в тексте «Удалить всё»), и
    // разъезжаться этим двум ответам нельзя — оператор по ним решает, оборвёт
    // ли его действие эфир.
    isPlaylistOnAir(playlistId: number): boolean {
      return (
        !!playerState &&
        playerState.status !== "idle" &&
        playerState.playlistId === playlistId
      );
    },
    refreshPlayerState,
    // startPolling/stopPolling — вызываются из $effect Audio.svelte (запуск
    // при монтировании вкладки, остановка при уходе с неё): опрос State()
    // не бесплатен и не должен крутиться, пока оператор смотрит на другую
    // вкладку (план, этап 2: транспорт и индикаторы — только на "Звук").
    startPolling() {
      if (pollTimer) return;
      refreshPlayerState();
      pollTimer = setInterval(refreshPlayerState, 500);
    },
    stopPolling() {
      if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
    },
    async play(playlistId: number, itemId: number): Promise<boolean> {
      try {
        await Play(playlistId, itemId);
        return true;
      } catch (e) {
        lastError = errorMessage(e);
        return false;
      }
    },
    async stop() {
      await runCommand(Stop);
    },
    async toggle() {
      await runCommand(Toggle);
    },
    // playOrToggleSelected — единая логика кнопки play/pause мини-плеера И
    // клавиши Space на вкладке (правка 4, второй раунд): играет — пауза; на
    // паузе — продолжить; ничего не загружено, но что-то выделено — играть
    // выделенное; ничего не загружено и ничего не выделено — молча ничего
    // не делать (кнопка в этом случае просто disabled, Space — no-op).
    // Вынесено в стор, а не продублировано в MiniPlayer.svelte и
    // Audio.svelte по отдельности, — чтобы эти два места не разошлись в
    // поведении (Enter на вкладке остаётся отдельной, более явной командой
    // "играть выбранное" — playSelectedPlaylistItem в Audio.svelte, её эта
    // функция не заменяет).
    async playOrToggleSelected(selectedItemId: number | null) {
      if (playerState && playerState.status !== "idle") {
        await this.toggle();
        return;
      }
      const playlist = playlists.active;
      const item = (playlist?.items ?? []).find((i) => i.ID === selectedItemId);
      if (!playlist || !item) return;
      await this.play(playlist.ID, item.ID);
    },
    async next() {
      await runCommand(Next);
    },
    async prev() {
      await runCommand(Prev);
    },
    // fadeOutStop — «Стоп» мини-плеера (этап 6): единственная кнопка стопа,
    // т.к. при инварианте «ровно один активный трек» (И5) «стоп всё»
    // совпадает со «стоп с фейдом».
    async fadeOutStop() {
      await runCommand(FadeOutStop);
    },
    // seek обновляет playerState.positionMs оптимистично, не дожидаясь
    // следующего опроса (до 500 мс, см. startPolling) — по образцу setVolume
    // ниже, для той же проблемы: несколько быстрых нажатий ArrowLeft/Right
    // (seekRelative в Audio.svelte) внутри одного окна опроса иначе считали
    // бы дельту от одной и той же устаревшей positionMs, и только последнее
    // нажатие имело бы эффект — оператор жмёт четыре раза, трек прыгает один
    // раз на 5 секунд.
    async seek(ms: number) {
      if (playerState) playerState = { ...playerState, positionMs: ms };
      await runCommand(() => Seek(ms));
    },
    // setVolume обновляет playerState.volume оптимистично, не дожидаясь
    // следующего опроса (до 500 мс, см. startPolling): без этого слайдер
    // громкости на каждый tick перескакивал бы обратно к устаревшему
    // значению, пока опрос не подтвердит новое.
    async setVolume(v: number) {
      if (playerState) playerState = { ...playerState, volume: v };
      await runCommand(() => SetVolume(v));
    },
  };

  return store;
};

export const audioStore = createAudioStore();
ListTracks()
  .then((t) => (audioStore.tracks.list = t ?? []))
  .catch(() => {
    // Отказ самого вызова (не путать с Go-стороной, которая на ошибке БД
    // шлёт audio_error и возвращает nil) — оставлять tracksLoading=true
    // навсегда означало бы вечный спиннер на вкладке «Звук».
    audioStore.tracks.list = [];
  });
