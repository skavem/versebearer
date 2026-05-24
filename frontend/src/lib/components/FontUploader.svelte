<script lang="ts">
  import type { Font } from "$lib/bindings/changeme/backend/models";
  import MuiIcon from "./MuiIcon.svelte";

  let {
    fonts,
    onUpload,
    onDelete,
  }: {
    fonts: Font[];
    onUpload: (file: File) => Promise<string | null>;
    onDelete: (id: number) => void;
  } = $props();

  let fileInput = $state<HTMLInputElement | null>(null);
  let uploading = $state(false);
  let errorMsg = $state<string | null>(null);
  let dragOver = $state(false);

  async function uploadOne(file: File | null | undefined) {
    if (!file) return;
    errorMsg = null;
    uploading = true;
    const err = await onUpload(file);
    uploading = false;
    if (err) errorMsg = err;
  }

  async function handleFileChange(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = "";
    await uploadOne(file);
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = "copy";
    dragOver = true;
  }

  function handleDragLeave(e: DragEvent) {
    if (e.currentTarget === e.target) dragOver = false;
  }

  async function handleDrop(e: DragEvent) {
    e.preventDefault();
    dragOver = false;
    const file = e.dataTransfer?.files?.[0];
    await uploadOne(file);
  }
</script>

<div class="flex flex-col gap-3 rounded-xl border border-base-300 bg-base-100 p-5 shadow-sm">
  <div class="flex items-center justify-between">
    <div class="flex items-center gap-2">
      <MuiIcon name="text_format" />
      <h3 class="text-lg font-semibold">Шрифты</h3>
    </div>
    <span class="text-xs text-base-content/50">{fonts.length} шт. · до 5 MB</span>
  </div>

  <!-- Drop zone -->
  <button
    type="button"
    class="dropzone {dragOver ? 'dropzone--active' : ''}"
    class:dropzone--busy={uploading}
    disabled={uploading}
    onclick={() => fileInput?.click()}
    ondragover={handleDragOver}
    ondragleave={handleDragLeave}
    ondrop={handleDrop}
  >
    {#if uploading}
      <span class="loading loading-spinner loading-md text-primary"></span>
      <span class="text-sm font-medium">Загрузка...</span>
    {:else}
      <MuiIcon name="cloud_upload" style="font-size: 2.25rem" />
      <div class="flex flex-col items-center gap-0.5">
        <span class="text-sm font-medium">Перетащите .woff2 или .ttf сюда</span>
        <span class="text-xs text-base-content/50">или нажмите чтобы выбрать</span>
      </div>
    {/if}
    <input
      bind:this={fileInput}
      type="file"
      accept=".woff2,.ttf"
      class="hidden"
      onchange={handleFileChange}
    />
  </button>

  {#if errorMsg}
    <div class="alert alert-error py-2 text-sm">
      <MuiIcon name="error" />
      <span>{errorMsg}</span>
    </div>
  {/if}

  {#if fonts.length > 0}
    <div class="flex flex-wrap gap-2">
      {#each fonts as font}
        <div class="font-chip">
          <MuiIcon name="font_download" style="font-size: 1.1rem" />
          <span class="font-chip__name">{font.name}</span>
          <span class="font-chip__size">{(font.sizeBytes / 1024).toFixed(0)} KB</span>
          <button
            class="font-chip__remove"
            onclick={() => onDelete(font.ID)}
            title="Удалить шрифт"
            aria-label="Удалить {font.name}"
          >
            <MuiIcon name="close" style="font-size: 1rem" />
          </button>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .dropzone {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.625rem;
    padding: 1.75rem 1rem;
    border: 2px dashed oklch(var(--b3));
    border-radius: 0.75rem;
    background: transparent;
    color: oklch(var(--bc) / 0.7);
    transition: border-color 0.15s, background-color 0.15s, color 0.15s;
    cursor: pointer;
  }
  .dropzone:hover:not(:disabled) {
    border-color: oklch(var(--bc) / 0.4);
    background-color: oklch(var(--b2) / 0.5);
  }
  .dropzone--active {
    border-color: oklch(var(--p));
    background-color: oklch(var(--p) / 0.06);
    color: oklch(var(--p));
  }
  .dropzone--busy {
    cursor: progress;
  }

  .font-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.25rem 0.25rem 0.25rem 0.625rem;
    border-radius: 9999px;
    background-color: oklch(var(--b2));
    border: 1px solid oklch(var(--b3));
    font-size: 0.8125rem;
  }
  .font-chip__name {
    font-weight: 500;
  }
  .font-chip__size {
    font-size: 0.7rem;
    color: oklch(var(--bc) / 0.5);
  }
  .font-chip__remove {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.25rem;
    height: 1.25rem;
    border-radius: 9999px;
    border: none;
    background: transparent;
    color: oklch(var(--bc) / 0.6);
    cursor: pointer;
    transition: background-color 0.12s, color 0.12s;
  }
  .font-chip__remove:hover {
    background-color: oklch(var(--er) / 0.15);
    color: oklch(var(--er));
  }
</style>
