<script lang="ts">
  import type { Backdrop, BackdropInput } from "$lib/bindings/changeme/models";
  import type { Image } from "$lib/bindings/changeme/backend/models";
  import {
    defaultGradient,
    gradientToCss,
    parseGradient,
    stringifyGradient,
  } from "$lib/gradient";
  import { imageUrl } from "$lib/receiver";
  import GradientEditor from "./GradientEditor.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  let {
    backdrop,
    images,
    onUpdate,
    onReset,
    onUploadImage,
    onDeleteImage,
  }: {
    backdrop: Backdrop;
    images: Image[];
    onUpdate: (patch: Partial<BackdropInput>) => void;
    onReset: () => void;
    onUploadImage: (file: File) => Promise<string | null>;
    onDeleteImage: (id: number) => void;
  } = $props();

  let timers: Record<string, ReturnType<typeof setTimeout>> = {};
  function debounce(key: string, fn: () => void, ms = 150) {
    clearTimeout(timers[key]);
    timers[key] = setTimeout(fn, ms);
  }
  $effect(() => () => Object.values(timers).forEach(clearTimeout));

  let fileInput = $state<HTMLInputElement | null>(null);
  let uploading = $state(false);
  let uploadError = $state<string | null>(null);
  let dragOver = $state(false);

  async function uploadOne(file: File | null | undefined) {
    if (!file) return;
    uploadError = null;
    uploading = true;
    const err = await onUploadImage(file);
    uploading = false;
    if (err) uploadError = err;
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

  function setBgType(type: "none" | "gradient" | "image") {
    if (type === "gradient" && !backdrop.bgGradient) {
      // Seed a visible gradient so switching to this mode shows something.
      onUpdate({ bgType: type, bgGradient: stringifyGradient(defaultGradient()) });
    } else {
      onUpdate({ bgType: type });
    }
  }

  function onGradientChange(json: string) {
    debounce("bgGradient", () => onUpdate({ bgGradient: json }), 150);
  }

  const preview = $derived.by(() => {
    if (backdrop.bgType === "gradient") return gradientToCss(parseGradient(backdrop.bgGradient));
    if (backdrop.bgType === "image" && backdrop.bgImageId != null)
      return `center / cover no-repeat url("${imageUrl(backdrop.bgImageId)}")`;
    return null;
  });
</script>

<div class="flex flex-col gap-4 rounded-xl border border-base-300 bg-base-100 p-5 shadow-sm">
  <!-- Header -->
  <div class="flex items-center justify-between border-b border-base-300 pb-3">
    <div class="flex items-center gap-2">
      <MuiIcon name="wallpaper" />
      <h3 class="text-lg font-semibold">Фон экрана</h3>
    </div>
    <button class="btn btn-sm btn-ghost gap-1" onclick={onReset} title="Убрать фон">
      <MuiIcon name="restart_alt" style="font-size: 1.1rem" />
      <span class="text-xs">Сбросить</span>
    </button>
  </div>

  <p class="-mt-1 text-xs text-base-content/50">
    Постоянная подложка на весь экран проектора, позади текста. Читаемость —
    через непрозрачность плашки в стилях стиха/куплета.
  </p>

  <!-- Preview -->
  {#if preview}
    <div class="h-20 rounded-lg border border-base-300" style:background={preview}></div>
  {/if}

  <!-- Type -->
  <div class="join">
    <button
      class="btn btn-sm join-item {backdrop.bgType === 'none' ? 'btn-neutral' : 'btn-outline'}"
      onclick={() => setBgType("none")}
    >
      Нет
    </button>
    <button
      class="btn btn-sm join-item {backdrop.bgType === 'gradient' ? 'btn-neutral' : 'btn-outline'}"
      onclick={() => setBgType("gradient")}
    >
      Градиент
    </button>
    <button
      class="btn btn-sm join-item {backdrop.bgType === 'image' ? 'btn-neutral' : 'btn-outline'}"
      onclick={() => setBgType("image")}
    >
      Картинка
    </button>
  </div>

  {#if backdrop.bgType === "gradient"}
    <GradientEditor value={backdrop.bgGradient} onChange={onGradientChange} />
  {:else if backdrop.bgType === "image"}
    <div class="flex flex-wrap gap-2">
      {#each images as image (image.ID)}
        <div class="img-pick-wrap">
          <button
            class="img-pick {backdrop.bgImageId === image.ID ? 'img-pick--active' : ''}"
            onclick={() => onUpdate({ bgImageId: image.ID })}
            title={image.name}
            aria-label={image.name}
            aria-pressed={backdrop.bgImageId === image.ID}
          >
            <img src={imageUrl(image.ID)} alt={image.name} />
            {#if backdrop.bgImageId === image.ID}
              <span class="img-pick__check">
                <MuiIcon name="check" style="font-size: 1rem" />
              </span>
            {/if}
          </button>
          <button
            class="img-pick__remove"
            onclick={() => onDeleteImage(image.ID)}
            title="Удалить картинку"
            aria-label="Удалить {image.name}"
          >
            <MuiIcon name="close" style="font-size: 0.9rem" />
          </button>
        </div>
      {/each}

      <button
        type="button"
        class="img-add {dragOver ? 'img-add--active' : ''}"
        class:img-add--busy={uploading}
        disabled={uploading}
        onclick={() => fileInput?.click()}
        ondragover={handleDragOver}
        ondragleave={handleDragLeave}
        ondrop={handleDrop}
        title="Загрузить картинку"
        aria-label="Загрузить картинку"
      >
        {#if uploading}
          <span class="loading loading-spinner loading-sm"></span>
        {:else}
          <MuiIcon name="add_photo_alternate" style="font-size: 1.4rem" />
        {/if}
        <input
          bind:this={fileInput}
          type="file"
          accept=".png,.jpg,.jpeg,.webp,.gif"
          class="hidden"
          onchange={handleFileChange}
        />
      </button>
    </div>

    {#if uploadError}
      <div class="alert alert-error py-2 text-sm">
        <MuiIcon name="error" />
        <span>{uploadError}</span>
      </div>
    {/if}

    <p class="-mt-1 text-xs text-base-content/40">.png, .jpg, .webp, .gif — до 10 МБ</p>
  {/if}
</div>

<style>
  .img-pick-wrap {
    position: relative;
    width: 6rem;
    height: 4rem;
  }
  .img-pick {
    position: relative;
    width: 100%;
    height: 100%;
    border-radius: 0.5rem;
    border: 2px solid oklch(var(--b3));
    overflow: hidden;
    cursor: pointer;
    padding: 0;
  }
  .img-pick img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .img-pick--active {
    border-color: oklch(var(--p));
  }
  .img-pick__check {
    position: absolute;
    top: 0.2rem;
    right: 0.2rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.35rem;
    height: 1.35rem;
    border-radius: 9999px;
    background: oklch(var(--p));
    color: oklch(var(--pc));
  }

  .img-pick__remove {
    position: absolute;
    top: 0.25rem;
    right: 0.25rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.25rem;
    height: 1.25rem;
    border-radius: 9999px;
    border: none;
    background: oklch(var(--b1) / 0.9);
    color: oklch(var(--bc) / 0.7);
    cursor: pointer;
    opacity: 0;
    transition:
      opacity 0.12s,
      background-color 0.12s,
      color 0.12s;
  }
  .img-pick-wrap:hover .img-pick__remove,
  .img-pick__remove:focus-visible {
    opacity: 1;
  }
  .img-pick__remove:hover {
    background-color: oklch(var(--er));
    color: oklch(var(--erc, var(--b1)));
  }

  .img-add {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.25rem;
    width: 6rem;
    height: 4rem;
    border-radius: 0.5rem;
    border: 2px dashed oklch(var(--b3));
    background: transparent;
    color: oklch(var(--bc) / 0.45);
    cursor: pointer;
    transition:
      border-color 0.15s,
      background-color 0.15s,
      color 0.15s;
  }
  .img-add:hover:not(:disabled) {
    border-color: oklch(var(--bc) / 0.4);
    background-color: oklch(var(--b2) / 0.5);
    color: oklch(var(--bc) / 0.7);
  }
  .img-add--active {
    border-color: oklch(var(--p));
    background-color: oklch(var(--p) / 0.06);
    color: oklch(var(--p));
  }
  .img-add--busy {
    cursor: progress;
  }
</style>
