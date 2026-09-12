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
              <span class="label-text text-xs">Начало, мс</span>
            </div>
            <input
              type="number"
              min="0"
              bind:value={trimStartMs}
              class="input input-sm input-bordered w-full"
            />
          </label>
          <label class="form-control flex-1">
            <div class="label py-1">
              <span class="label-text text-xs">Конец, мс (0 = до конца)</span>
            </div>
            <input
              type="number"
              min="0"
              bind:value={trimEndMs}
              class="input input-sm input-bordered w-full"
            />
          </label>
        </div>

        <label class="form-control">
          <div class="label py-1">
            <span class="label-text font-medium">Громкость, дБ</span>
          </div>
          <input
            type="number"
            step="0.1"
            min="-12"
            max="12"
            bind:value={gainDb}
            class="input input-bordered w-full"
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
