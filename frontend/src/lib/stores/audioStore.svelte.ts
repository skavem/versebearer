import type { AudioTrack } from "$lib/bindings/changeme/backend/models";
import {
  ImportTrack,
  ListTracks,
  PickAudioFiles,
  RemoveTrack,
  UpdateTrack,
} from "$lib/bindings/changeme/audioservice";
import type { ImportTrackResult, TrackInput } from "$lib/bindings/changeme/models";
import { Events } from "@wailsio/runtime";

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

// ⚠️ Отступление от образца (songsStore.svelte.ts): при module-init тянем
// только треки. ListDevices() сюда сознательно не входит — это стартовало бы
// malgo.InitContext ещё до открытия вкладки «Звук», и обещание «на машине без
// звуковой карты старт не ломается» перестало бы выполняться. Устройства
// появятся в этапе 3 и будут запрашиваться из $effect самой вкладки.
// Плейлисты тоже не тянутся здесь — ListPlaylists появится в этапе 4.
const createAudioStore = () => {
  let tracksList = $state<AudioTrack[]>([]);
  let tracksLoading = $state(true);
  let activeTrack = $state<AudioTrack | null>(null);

  let importing = $state(false);
  let importProgress = $state<{ done: number; total: number } | null>(null);
  let importCancelRequested = false;

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

  Events.On(
    "audio_tracks_update",
    ({ data }: { data: AudioTrack[] }) => (tracks.list = data ?? []),
  );
  Events.On("audio_error", ({ data }: { data: string }) => {
    lastError = data;
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
