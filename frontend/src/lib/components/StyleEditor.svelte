<script lang="ts">
  import type { VisualStyle } from "$lib/bindings/changeme/models";
  import type { Font } from "$lib/bindings/changeme/backend/models";
  import { createDebouncer } from "$lib/debounce";
  import MuiIcon from "./MuiIcon.svelte";
  import NumberStepper from "./NumberStepper.svelte";
  import PopoverSelect from "./PopoverSelect.svelte";

  let {
    title,
    style,
    fonts,
    onUpdate,
    onReset,
    onUploadFont,
    onDeleteFont,
  }: {
    title: string;
    style: VisualStyle;
    fonts: Font[];
    onUpdate: (patch: Partial<VisualStyle>) => void;
    onReset: () => void;
    onUploadFont: (file: File) => Promise<string | null>;
    onDeleteFont: (id: number) => void;
  } = $props();

  const { debounce, cancelAll } = createDebouncer();
  $effect(() => cancelAll);

  // Shadow decomposed
  let shadowColor = $state("#000000");
  let shadowX = $state(0);
  let shadowY = $state(2);
  let shadowBlur = $state(4);

  $effect(() => {
    if (style.textShadow) {
      const parts = style.textShadow.trim().split(/\s+/);
      if (parts.length >= 4) {
        shadowColor = parts[0];
        shadowX = parseInt(parts[1]) || 0;
        shadowY = parseInt(parts[2]) || 0;
        shadowBlur = parseInt(parts[3]) || 0;
      }
    }
  });

  function buildTextShadow(): string {
    if (shadowX === 0 && shadowY === 0 && shadowBlur === 0) return "";
    return `${shadowColor} ${shadowX}px ${shadowY}px ${shadowBlur}px`;
  }

  function onShadowChange() {
    debounce("textShadow", () => onUpdate({ textShadow: buildTextShadow() }), 150);
  }

  function hexToRgba(hex: string, opacity: number): string {
    const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
    if (!m) return `rgba(0,0,0,${opacity})`;
    const i = parseInt(m[1], 16);
    return `rgba(${(i >> 16) & 0xff}, ${(i >> 8) & 0xff}, ${i & 0xff}, ${opacity})`;
  }

  const selectedFont = $derived(
    style.fontId == null ? null : (fonts.find((x) => x.ID === style.fontId) ?? null),
  );
  const previewFontFamily = $derived(
    selectedFont
      ? `"${selectedFont.name}", "Century Gothic", sans-serif`
      : '"Century Gothic", sans-serif',
  );
  const selectedFontName = $derived(selectedFont?.name ?? "По умолчанию");

  // --- Font picker popover ---
  let fontMenuOpen = $state(false);
  let fontFileInput = $state<HTMLInputElement | null>(null);
  let fontUploading = $state(false);
  let fontError = $state<string | null>(null);

  function pickFont(id: number | null) {
    onUpdate({ fontId: id === null ? undefined : id } as Partial<VisualStyle>);
    fontMenuOpen = false;
  }

  async function handleFontFileChange(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    fontError = null;
    fontUploading = true;
    const err = await onUploadFont(file);
    fontUploading = false;
    if (err) fontError = err;
  }

  const borderStyleOptions: { value: string; label: string }[] = [
    { value: "solid", label: "сплошная" },
    { value: "dashed", label: "штрих" },
    { value: "dotted", label: "точки" },
    { value: "none", label: "нет" },
  ];
</script>

{#snippet swatch(color: string, onPick: (v: string) => void)}
  <label class="swatch">
    <span class="swatch-color" style:background-color={color}></span>
    <span class="swatch-hex">{color.toUpperCase()}</span>
    <input
      type="color"
      class="swatch-input"
      value={color}
      oninput={(e) => onPick((e.target as HTMLInputElement).value)}
    />
  </label>
{/snippet}

<div class="flex flex-col gap-4 rounded-xl border border-base-300 bg-base-100 p-4 shadow-sm">
  <!-- Header -->
  <div class="flex items-center justify-between border-b border-base-300 pb-2.5">
    <h3 class="text-sm font-semibold uppercase tracking-wide">{title}</h3>
    <button class="btn btn-xs btn-ghost gap-1" onclick={onReset} title="Сбросить к дефолту">
      <MuiIcon name="restart_alt" style="font-size: 1rem" />
      <span class="text-xs">Сбросить</span>
    </button>
  </div>

  <!-- Live preview -->
  <div
    class="checker flex h-20 items-center justify-center overflow-hidden rounded-lg"
    aria-hidden="true"
  >
    <div
      class="flex h-full w-full items-center justify-center"
      style:background-color={hexToRgba(style.bgColor, style.bgOpacity)}
      style:color={style.textColor}
      style:font-family={previewFontFamily}
      style:border-color={style.borderColor}
      style:border-width="{style.borderWidth}px"
      style:border-radius="{style.borderRadius}px"
      style:border-style={style.borderStyle}
      style:padding="{Math.min(style.padding, 24)}px"
      style:text-shadow={style.textShadow}
    >
      <span class="text-center text-lg font-bold">Пример текста</span>
    </div>
  </div>

  <!-- Фон -->
  <section class="flex flex-col gap-1.5">
    <div class="section-head">
      <MuiIcon name="format_color_fill" style="font-size: 1rem" />
      Плашка (за текстом)
    </div>
    <div class="flex flex-wrap items-center gap-2">
      {@render swatch(style.bgColor, (v) => debounce("bgColor", () => onUpdate({ bgColor: v }), 150))}
      <div class="flex min-w-[9rem] flex-1 flex-col gap-1">
        <span class="text-xs text-base-content/60">Непрозрачность: {Math.round(style.bgOpacity * 100)}%</span>
        <input
          type="range"
          class="range range-xs range-primary"
          min="0"
          max="100"
          step="1"
          value={Math.round(style.bgOpacity * 100)}
          oninput={(e) =>
            debounce("bgOpacity", () => onUpdate({ bgOpacity: parseInt((e.target as HTMLInputElement).value) / 100 }), 150)}
        />
      </div>
    </div>
  </section>

  <!-- Текст -->
  <section class="flex flex-col gap-1.5">
    <div class="section-head">
      <MuiIcon name="text_fields" style="font-size: 1rem" />
      Текст
    </div>
    <div class="flex flex-wrap items-center gap-2">
      {@render swatch(style.textColor, (v) => debounce("textColor", () => onUpdate({ textColor: v }), 150))}

      <div class="dd min-w-[10rem] flex-1">
        <button
          type="button"
          class="dd-trigger"
          onclick={() => (fontMenuOpen = !fontMenuOpen)}
          aria-haspopup="listbox"
          aria-expanded={fontMenuOpen}
          aria-label="Шрифт"
        >
          <MuiIcon name="text_format" style="font-size: 1rem" />
          <span class="dd-trigger__name">{selectedFontName}</span>
          <MuiIcon name="expand_more" style="font-size: 1.1rem" />
        </button>

        {#if fontMenuOpen}
          <button
            class="dd-backdrop"
            onclick={() => (fontMenuOpen = false)}
            aria-label="Закрыть выбор шрифта"
          ></button>
          <div class="dd-panel" role="listbox" aria-label="Шрифты">
            <button
              type="button"
              class="dd-option {style.fontId == null ? 'dd-option--active' : ''}"
              onclick={() => pickFont(null)}
              role="option"
              aria-selected={style.fontId == null}
            >
              {#if style.fontId == null}
                <MuiIcon name="check" style="font-size: 0.9rem" />
              {:else}
                <span class="dd-spacer"></span>
              {/if}
              По умолчанию
            </button>

            {#if fonts.length > 0}
              <div class="dd-divider"></div>
              {#each fonts as font (font.ID)}
                <div class="dd-row">
                  <button
                    type="button"
                    class="dd-option {style.fontId === font.ID ? 'dd-option--active' : ''}"
                    onclick={() => pickFont(font.ID)}
                    role="option"
                    aria-selected={style.fontId === font.ID}
                  >
                    {#if style.fontId === font.ID}
                      <MuiIcon name="check" style="font-size: 0.9rem" />
                    {:else}
                      <span class="dd-spacer"></span>
                    {/if}
                    <span class="dd-option__name">{font.name}</span>
                  </button>
                  <button
                    class="dd-delete"
                    onclick={() => onDeleteFont(font.ID)}
                    title="Удалить шрифт"
                    aria-label="Удалить {font.name}"
                  >
                    <MuiIcon name="close" style="font-size: 0.9rem" />
                  </button>
                </div>
              {/each}
            {/if}

            <div class="dd-divider"></div>
            <button
              type="button"
              class="dd-upload"
              disabled={fontUploading}
              onclick={() => fontFileInput?.click()}
            >
              {#if fontUploading}
                <span class="loading loading-spinner loading-xs"></span>
                Загрузка...
              {:else}
                <MuiIcon name="cloud_upload" style="font-size: 1rem" />
                Загрузить шрифт
              {/if}
            </button>
            <input
              bind:this={fontFileInput}
              type="file"
              accept=".woff2,.ttf"
              class="hidden"
              onchange={handleFontFileChange}
            />
            {#if fontError}
              <p class="dd-error">{fontError}</p>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  </section>

  <!-- Рамка -->
  <section class="flex flex-col gap-1.5">
    <div class="section-head">
      <MuiIcon name="rounded_corner" style="font-size: 1rem" />
      Рамка
    </div>
    <div class="flex flex-wrap items-center gap-2">
      {@render swatch(style.borderColor, (v) => debounce("borderColor", () => onUpdate({ borderColor: v }), 150))}
      <PopoverSelect
        classes="w-32"
        value={style.borderStyle}
        options={borderStyleOptions}
        ariaLabel="Стиль рамки"
        onChange={(v) => onUpdate({ borderStyle: v })}
      />
      <NumberStepper
        label="Толщина"
        unit="px"
        min={0}
        value={style.borderWidth}
        onChange={(n) => debounce("borderWidth", () => onUpdate({ borderWidth: n }), 150)}
      />
      <NumberStepper
        label="Скругление"
        unit="px"
        min={0}
        value={style.borderRadius}
        onChange={(n) => debounce("borderRadius", () => onUpdate({ borderRadius: n }), 150)}
      />
    </div>
  </section>

  <!-- Отступы -->
  <section class="flex flex-col gap-1.5">
    <div class="section-head">
      <MuiIcon name="format_size" style="font-size: 1rem" />
      Отступы
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <NumberStepper
        label="Внутр."
        unit="px"
        min={0}
        value={style.padding}
        onChange={(n) => debounce("padding", () => onUpdate({ padding: n }), 150)}
      />
      <NumberStepper
        label="Внешн."
        unit="px"
        min={0}
        value={style.margin}
        onChange={(n) => debounce("margin", () => onUpdate({ margin: n }), 150)}
      />
    </div>
  </section>

  <!-- Тень -->
  <section class="flex flex-col gap-1.5">
    <div class="section-head">
      <MuiIcon name="blur_on" style="font-size: 1rem" />
      Тень текста
    </div>
    <div class="flex flex-wrap items-center gap-2">
      {@render swatch(shadowColor, (v) => { shadowColor = v; onShadowChange(); })}
      <NumberStepper
        label="X"
        unit="px"
        value={shadowX}
        onChange={(n) => { shadowX = n; onShadowChange(); }}
      />
      <NumberStepper
        label="Y"
        unit="px"
        value={shadowY}
        onChange={(n) => { shadowY = n; onShadowChange(); }}
      />
      <NumberStepper
        label="Размытие"
        unit="px"
        min={0}
        value={shadowBlur}
        onChange={(n) => { shadowBlur = n; onShadowChange(); }}
      />
    </div>
  </section>
</div>

<svelte:window onkeydown={(e) => fontMenuOpen && e.key === "Escape" && (fontMenuOpen = false)} />

<style>
  .section-head {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: oklch(var(--bc) / 0.6);
  }

  .swatch {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    height: 2rem;
    padding: 0 0.5rem;
    border: 1px solid oklch(var(--b3));
    border-radius: 0.5rem;
    cursor: pointer;
    transition: background-color 0.12s;
  }
  .swatch:hover {
    background-color: oklch(var(--b2));
  }
  .swatch-color {
    display: block;
    width: 1.25rem;
    height: 1.25rem;
    border-radius: 0.25rem;
    border: 1px solid oklch(var(--b3));
    background-image:
      linear-gradient(45deg, #d4d4d4 25%, transparent 25%),
      linear-gradient(-45deg, #d4d4d4 25%, transparent 25%),
      linear-gradient(45deg, transparent 75%, #d4d4d4 75%),
      linear-gradient(-45deg, transparent 75%, #d4d4d4 75%);
    background-size: 8px 8px;
    background-position: 0 0, 0 4px, 4px -4px, -4px 0;
  }
  .swatch-hex {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 0.75rem;
    letter-spacing: 0.025em;
  }
  .swatch-input {
    position: absolute;
    inset: 0;
    opacity: 0;
    cursor: pointer;
  }

  .checker {
    background-image:
      linear-gradient(45deg, oklch(var(--b2)) 25%, transparent 25%),
      linear-gradient(-45deg, oklch(var(--b2)) 25%, transparent 25%),
      linear-gradient(45deg, transparent 75%, oklch(var(--b2)) 75%),
      linear-gradient(-45deg, transparent 75%, oklch(var(--b2)) 75%);
    background-size: 16px 16px;
    background-position: 0 0, 0 8px, 8px -8px, -8px 0;
  }
</style>
