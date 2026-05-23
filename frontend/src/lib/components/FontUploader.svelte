<script lang="ts">
  import type { Font } from "$lib/bindings/changeme/backend/models";

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

  async function handleFileChange(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    input.value = "";
    errorMsg = null;
    uploading = true;
    const err = await onUpload(file);
    uploading = false;
    if (err) {
      errorMsg = err;
    }
  }
</script>

<div class="flex flex-col gap-3 rounded-lg border border-base-300 p-4">
  <div class="flex items-center justify-between">
    <h3 class="text-lg font-semibold">Шрифты</h3>
    <button
      class="btn btn-sm btn-primary"
      disabled={uploading}
      onclick={() => fileInput?.click()}
    >
      {#if uploading}
        <span class="loading loading-spinner loading-xs"></span>
      {/if}
      Загрузить шрифт
    </button>
    <input
      bind:this={fileInput}
      type="file"
      accept=".woff2,.ttf"
      class="hidden"
      onchange={handleFileChange}
    />
  </div>

  {#if errorMsg}
    <div class="alert alert-error py-2 text-sm">
      {errorMsg}
    </div>
  {/if}

  {#if fonts.length === 0}
    <p class="text-sm text-base-content/50">Нет загруженных шрифтов</p>
  {:else}
    <ul class="flex flex-col gap-2">
      {#each fonts as font}
        <li class="flex items-center justify-between rounded border border-base-200 px-3 py-2">
          <span class="text-sm">{font.name}</span>
          <div class="flex items-center gap-2">
            <span class="text-xs text-base-content/40">{(font.sizeBytes / 1024).toFixed(0)} KB</span>
            <button
              class="btn btn-xs btn-ghost text-error"
              onclick={() => onDelete(font.ID)}
            >
              Удалить
            </button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>
