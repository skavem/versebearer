import type {
  AudioTrack,
  Playlist,
} from "$lib/bindings/changeme/backend/models";
import {
  AddToPlaylist,
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
  let activeTrack = $state<AudioTrack | null>(null);

  let importing = $state(false);
  let importProgress = $state<{ done: number; total: number } | null>(null);
  let importCancelRequested = false;

  let devicesList = $state<AudioDevice[]>([]);
  let devicesLoading = $state(true);
  let selectedDeviceId = $state("");

  let playlistsList = $state<Playlist[]>([]);
  let playlistsLoading = $state(true);
  let activePlaylist = $state<Playlist | null>(null);

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
      const prevActive = activeTrack;
      tracksList = v;
      activeTrack =
        (prevActive && tracksList.find((t) => t.ID === prevActive.ID)) ?? null;
      tracksLoading = false;
    },
    get active() {
      return activeTrack;
    },
    set active(v: AudioTrack | null) {
      activeTrack = v;
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
      const prevActive = activePlaylist;
      playlistsList = v;
      activePlaylist =
        (prevActive && playlistsList.find((p) => p.ID === prevActive.ID)) ??
        null;
      playlistsLoading = false;
    },
    get active() {
      return activePlaylist;
    },
    set active(v: Playlist | null) {
      activePlaylist = v;
    },
  };

  const player = {
    get state() {
      return playerState;
    },
  };

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

  return {
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
        playlists.list = (await ListPlaylists()) ?? [];
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
      if (playlists.active?.ID === id) playlists.active = null;
      await RemovePlaylist(id);
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
      try {
        await Stop();
      } catch (e) {
        lastError = errorMessage(e);
      }
    },
    async toggle() {
      try {
        await Toggle();
      } catch (e) {
        lastError = errorMessage(e);
      }
    },
    async next() {
      try {
        await Next();
      } catch (e) {
        lastError = errorMessage(e);
      }
    },
    async prev() {
      try {
        await Prev();
      } catch (e) {
        lastError = errorMessage(e);
      }
    },
    // fadeOutStop — «Стоп» мини-плеера (этап 6): единственная кнопка стопа,
    // т.к. при инварианте «ровно один активный трек» (И5) «стоп всё»
    // совпадает со «стоп с фейдом».
    async fadeOutStop() {
      try {
        await FadeOutStop();
      } catch (e) {
        lastError = errorMessage(e);
      }
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
      try {
        await Seek(ms);
      } catch (e) {
        lastError = errorMessage(e);
      }
    },
    // setVolume обновляет playerState.volume оптимистично, не дожидаясь
    // следующего опроса (до 500 мс, см. startPolling): без этого слайдер
    // громкости на каждый tick перескакивал бы обратно к устаревшему
    // значению, пока опрос не подтвердит новое.
    async setVolume(v: number) {
      if (playerState) playerState = { ...playerState, volume: v };
      try {
        await SetVolume(v);
      } catch (e) {
        lastError = errorMessage(e);
      }
    },
  };
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
