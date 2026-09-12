<script lang="ts">
  import type { AudioTrack } from "$lib/bindings/changeme/backend/models";
  import EditTrackModal from "$lib/components/EditTrackModal.svelte";
  import List from "$lib/components/List.svelte";
  import MuiIcon from "$lib/components/MuiIcon.svelte";
  import { audioStore } from "$lib/stores/audioStore.svelte";

  const tracks = $derived(audioStore.tracks);

  let editingTrack = $state<AudioTrack | null>(null);
  let trackToDelete = $state<AudioTrack | null>(null);
  let importErrors = $state<string[]>([]);

  function formatDuration(ms: number): string {
    if (!ms || ms <= 0) return "—";
    const totalSec = Math.round(ms / 1000);
    const min = Math.floor(totalSec / 60);
    const sec = totalSec % 60;
    return `${min}:${sec.toString().padStart(2, "0")}`;
  }

  function formatSize(bytes: number): string {
    if (!bytes) return "";
    return `${(bytes / (1024 * 1024)).toFixed(1)} МБ`;
  }

  const doImport = async () => {
    const paths = await audioStore.pickFiles();
    if (paths.length === 0) return; // диалог закрыли — молча выходим
    importErrors = [];
    const results = await audioStore.importFiles(paths);
    importErrors = results.filter((r) => r.error).map((r) => r.error);
  };

  const confirmDelete = async () => {
    if (!trackToDelete) return;
    const id = trackToDelete.ID;
    trackToDelete = null;
    await audioStore.remove(id);
  };
</script>

<div class="flex h-[calc(100vh-4rem)] flex-col gap-2 p-4">
  <div class="flex items-center justify-between">
    <h2 class="text-lg font-semibold">Медиатека фонограмм</h2>

    {#if audioStore.importing}
      <div class="flex items-center gap-2">
        <progress
          class="progress progress-primary w-40"
          value={audioStore.importProgress?.done ?? 0}
          max={audioStore.importProgress?.total ?? 1}
        ></progress>
        <span class="text-xs opacity-70">
          {audioStore.importProgress?.done ?? 0} из {audioStore.importProgress
            ?.total ?? 0}
        </span>
        <button
          class="btn btn-ghost btn-xs"
          onclick={() => audioStore.cancelImport()}
        >
          Отмена
        </button>
      </div>
    {:else}
      <button class="btn btn-outline btn-sm gap-1" onclick={doImport}>
        <MuiIcon name="add" style="font-size: 1.15rem" />
        Импорт
      </button>
    {/if}
  </div>

  {#if importErrors.length > 0}
    <div class="alert alert-error py-2 text-sm">
      Не удалось импортировать {importErrors.length}
      {importErrors.length === 1 ? "файл" : "файл(ов)"}: {importErrors[0]}
      {#if importErrors.length > 1}
        и ещё {importErrors.length - 1}…
      {/if}
    </div>
  {/if}

  {#if audioStore.error}
    <div class="alert alert-error py-2 text-sm">
      <span class="flex-1">{audioStore.error}</span>
      <button
        class="btn btn-ghost btn-xs"
        onclick={() => audioStore.clearError()}
        aria-label="Закрыть"
      >
        <MuiIcon name="close" style="font-size: 1rem" />
      </button>
    </div>
  {/if}

  <div class="min-h-0 flex-1">
    {#if tracks.loading}
      <div class="flex h-full items-center justify-center opacity-60">
        Загрузка…
      </div>
    {:else if tracks.list.length === 0}
      <div class="flex h-full items-center justify-center opacity-60">
        Пока нет фонограмм — нажмите «Импорт»
      </div>
    {:else}
      <List
        items={tracks.list}
        activeItem={tracks.active}
        getName={(t) => t.title || t.fileName}
        onClick={(t) => (tracks.active = t)}
      >
        {#snippet leftMark(t)}
          <span class="badge badge-neutral badge-sm font-mono"
            >{formatDuration(t.durationMs)}</span
          >
        {/snippet}
        {#snippet rightMark(t)}
          <div class="flex flex-row items-center gap-1">
            <span class="hidden text-xs opacity-50 sm:inline group-hover/item:hidden"
              >{formatSize(t.sizeBytes)}</span
            >
            <button
              class="btn btn-neutral btn-xs hidden px-1 text-white group-hover/item:block"
              onclick={(e) => {
                editingTrack = t;
                e.stopPropagation();
              }}
              title="Редактировать"
              ><MuiIcon name="edit" style="font-size: 1rem" /></button
            >
            <button
              class="btn btn-error btn-xs hidden px-1 text-white group-hover/item:block"
              onclick={(e) => {
                trackToDelete = t;
                e.stopPropagation();
              }}
              title="Удалить фонограмму"
              ><MuiIcon name="delete" style="font-size: 1rem" /></button
            >
          </div>
        {/snippet}
      </List>
    {/if}
  </div>
</div>

<EditTrackModal bind:track={editingTrack} />

<svelte:window
  onkeydown={(e) =>
    trackToDelete && e.key === "Escape" && (trackToDelete = null)}
/>

{#if trackToDelete}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-2 flex items-center gap-3">
        <div
          class="flex h-10 w-10 items-center justify-center rounded-full bg-error/10 text-error"
        >
          <MuiIcon name="delete" />
        </div>
        <h3 class="text-lg font-bold">Удалить фонограмму?</h3>
      </div>

      <p class="py-2">
        <span class="font-semibold">«{trackToDelete.title}»</span> будет удалена
        безвозвратно вместе с файлом.
      </p>

      <div class="modal-action">
        <button class="btn btn-ghost" onclick={() => (trackToDelete = null)}>
          Отмена
        </button>
        <button class="btn btn-error" onclick={confirmDelete}>
          <MuiIcon name="delete" />
          Удалить
        </button>
      </div>
    </div>
    <button
      class="modal-backdrop"
      onclick={() => (trackToDelete = null)}
      aria-label="Закрыть"
    ></button>
  </div>
{/if}
