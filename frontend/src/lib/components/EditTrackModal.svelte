<script lang="ts">
  import type { AudioTrack } from "$lib/bindings/changeme/backend/models";
  import { audioStore } from "$lib/stores/audioStore.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  let { track = $bindable(null) }: { track: AudioTrack | null } = $props();

  let title = $state("");
  let artist = $state("");
  let trimStartMs = $state(0);
  let trimEndMs = $state(0);
  let gainDb = $state(0);

  $effect(() => {
    if (track) {
      title = track.title;
      artist = track.artist;
      trimStartMs = track.trimStartMs;
      trimEndMs = track.trimEndMs;
      gainDb = track.gainDb;
    }
  });

  // "м:сс" — план, этап 5: оператор задаёт границы обрезки на слух, секунды
  // от начала файла удобнее читать и печатать, чем сырые миллисекунды.
  function msToMinSec(ms: number): string {
    if (!ms || ms <= 0) return "0:00";
    const totalSec = Math.round(ms / 1000);
    const min = Math.floor(totalSec / 60);
    const sec = totalSec % 60;
    return `${min}:${sec.toString().padStart(2, "0")}`;
  }

  function minSecToMs(text: string): number {
    const m = text.trim().match(/^(\d+):([0-5]?\d)$/);
    if (!m) return 0;
    const min = parseInt(m[1], 10);
    const sec = parseInt(m[2], 10);
    return (min * 60 + sec) * 1000;
  }

  // «Взять текущую позицию» доступна, только пока играет (или на паузе)
  // именно ЭТОТ трек: PlayerState.PositionMs считается ОТ TrimStartMs
  // (audio-playlist-implementation.md, этап 5: "прогресс считается от
  // TrimStartMs"), поэтому абсолютная позиция в исходном файле — это
  // ИСХОДНЫЙ (ещё не сохранённый правкой) track.trimStartMs плюс текущий
  // PositionMs, а не сам сырой PositionMs и не редактируемый trimStartMs.
  const player = $derived(audioStore.player.state);
  const isThisTrackLoaded = $derived(!!track && player?.trackId === track.ID);
  const currentAbsoluteMs = $derived(
    isThisTrackLoaded && track
      ? track.trimStartMs + (player?.positionMs ?? 0)
      : null,
  );

  const close = () => {
    track = null;
  };

  const save = async () => {
    if (!track) return;
    const updated = await audioStore.update(track.ID, {
      title: title.trim(),
      artist: artist.trim(),
      trimStartMs,
      trimEndMs,
      gainDb,
    });
    // Отказ (audioStore.update ловит его сам и кладёт в audioStore.error) —
    // не закрываем модалку молча: закрытие выглядело бы как «сохранено».
    if (updated) close();
  };
</script>

<svelte:window onkeydown={(e) => track && e.key === "Escape" && close()} />

{#if track}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-2 flex items-center justify-between">
        <h3 class="text-lg font-bold">Правка фонограммы</h3>
        <button
          class="btn btn-ghost btn-sm btn-square"
          onclick={close}
          aria-label="Закрыть"
        >
          <MuiIcon name="close" />
        </button>
      </div>

      <div class="flex flex-col gap-3">
        <label class="form-control">
          <div class="label py-1">
            <span class="label-text font-medium">Название</span>
          </div>
          <input
            type="text"
            bind:value={title}
            class="input input-bordered w-full"
          />
        </label>

        <label class="form-control">
          <div class="label py-1">
            <span class="label-text font-medium">Исполнитель</span>
          </div>
          <input
            type="text"
            bind:value={artist}
            class="input input-bordered w-full"
          />
        </label>

        <div class="flex gap-2">
          <label class="form-control flex-1">
            <div class="label py-1">
              <span class="label-text text-xs">Начало, м:сс</span>
            </div>
            <div class="join w-full">
              <input
                type="text"
                value={msToMinSec(trimStartMs)}
                onchange={(e) =>
                  (trimStartMs = minSecToMs(e.currentTarget.value))}
                placeholder="0:00"
                class="input input-sm input-bordered join-item w-full"
              />
              <button
                type="button"
                class="btn btn-sm join-item"
                disabled={currentAbsoluteMs === null}
                title="Взять текущую позицию воспроизведения"
                onclick={() => {
                  if (currentAbsoluteMs !== null) trimStartMs = currentAbsoluteMs;
                }}
              >
                <MuiIcon name="my_location" style="font-size: 1rem" />
              </button>
            </div>
          </label>
          <label class="form-control flex-1">
            <div class="label py-1">
              <span class="label-text text-xs">Конец, м:сс (0 = до конца)</span>
            </div>
            <div class="join w-full">
              <input
                type="text"
                value={msToMinSec(trimEndMs)}
                onchange={(e) =>
                  (trimEndMs = minSecToMs(e.currentTarget.value))}
                placeholder="0:00"
                class="input input-sm input-bordered join-item w-full"
              />
              <button
                type="button"
                class="btn btn-sm join-item"
                disabled={currentAbsoluteMs === null}
                title="Взять текущую позицию воспроизведения"
                onclick={() => {
                  if (currentAbsoluteMs !== null) trimEndMs = currentAbsoluteMs;
                }}
              >
                <MuiIcon name="my_location" style="font-size: 1rem" />
              </button>
            </div>
          </label>
        </div>

        <label class="form-control">
          <div class="label py-1">
            <span class="label-text font-medium">Громкость</span>
            <span class="label-text-alt font-mono"
              >{gainDb > 0 ? "+" : ""}{gainDb.toFixed(1)} дБ</span
            >
          </div>
          <input
            type="range"
            step="0.5"
            min="-12"
            max="12"
            bind:value={gainDb}
            class="range range-sm"
          />
        </label>
      </div>

      <div class="modal-action">
        <button class="btn btn-ghost" onclick={close}>Отмена</button>
        <button class="btn btn-neutral" onclick={save}>
          <MuiIcon name="save" />
          Сохранить
        </button>
      </div>
    </div>
    <button class="modal-backdrop" onclick={close} aria-label="Закрыть"></button>
  </div>
{/if}
