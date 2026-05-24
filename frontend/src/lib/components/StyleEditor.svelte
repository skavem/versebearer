<script lang="ts">
  import type { VisualStyle } from "$lib/bindings/changeme/models";
  import type { Font } from "$lib/bindings/changeme/backend/models";
  import MuiIcon from "./MuiIcon.svelte";

  let {
    title,
    style,
    fonts,
    onUpdate,
    onReset,
  }: {
    title: string;
    style: VisualStyle;
    fonts: Font[];
    onUpdate: (patch: Partial<VisualStyle>) => void;
    onReset: () => void;
  } = $props();

  let timers: Record<string, ReturnType<typeof setTimeout>> = {};
  function debounce(key: string, fn: () => void, ms = 150) {
    clearTimeout(timers[key]);
    timers[key] = setTimeout(fn, ms);
  }

  $effect(() => {
    return () => {
      Object.values(timers).forEach(clearTimeout);
    };
  });

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

  function fontIdFromSelect(val: string): number | null {
    if (val === "") return null;
    const n = parseInt(val);
    return isNaN(n) ? null : n;
  }

  function hexToRgba(hex: string, opacity: number): string {
    const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
    if (!m) return `rgba(0,0,0,${opacity})`;
    const i = parseInt(m[1], 16);
    return `rgba(${(i >> 16) & 0xff}, ${(i >> 8) & 0xff}, ${i & 0xff}, ${opacity})`;
  }

  const previewFontFamily = $derived.by(() => {
    if (style.fontId == null) return '"Century Gothic", sans-serif';
    const f = fonts.find((x) => x.ID === style.fontId);
    return f ? `"${f.name}", "Century Gothic", sans-serif` : '"Century Gothic", sans-serif';
  });
</script>

<div class="flex flex-col gap-4 rounded-xl border border-base-300 bg-base-100 p-5 shadow-sm">
  <!-- Header -->
  <div class="flex items-center justify-between border-b border-base-300 pb-3">
    <h3 class="text-lg font-semibold">{title}</h3>
    <button class="btn btn-sm btn-ghost gap-1" onclick={onReset} title="Сбросить к дефолту">
      <MuiIcon name="restart_alt" style="font-size: 1.1rem" />
      <span class="text-xs">Сбросить</span>
    </button>
  </div>

  <!-- Live preview -->
  <div
    class="checker flex h-24 items-center justify-center overflow-hidden rounded-lg"
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
      <span class="text-center text-xl font-bold">Пример текста</span>
    </div>
  </div>

  <!-- Фон -->
  <section class="flex flex-col gap-2">
    <div class="section-head">
      <MuiIcon name="format_color_fill" style="font-size: 1rem" />
      Фон
    </div>
    <div class="flex items-center gap-3">
      <label class="swatch">
        <span class="swatch-color" style:background-color={style.bgColor}></span>
        <span class="swatch-hex">{style.bgColor.toUpperCase()}</span>
        <input
          type="color"
          class="swatch-input"
          value={style.bgColor}
          oninput={(e) => debounce("bgColor", () => onUpdate({ bgColor: (e.target as HTMLInputElement).value }), 150)}
        />
      </label>
      <div class="flex flex-1 flex-col gap-1">
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
  <section class="flex flex-col gap-2">
    <div class="section-head">
      <MuiIcon name="text_fields" style="font-size: 1rem" />
      Текст
    </div>
    <div class="flex items-center gap-3">
      <label class="swatch">
        <span class="swatch-color" style:background-color={style.textColor}></span>
        <span class="swatch-hex">{style.textColor.toUpperCase()}</span>
        <input
          type="color"
          class="swatch-input"
          value={style.textColor}
          oninput={(e) => debounce("textColor", () => onUpdate({ textColor: (e.target as HTMLInputElement).value }), 150)}
        />
      </label>
      <select
        class="select select-bordered select-sm flex-1"
        value={style.fontId?.toString() ?? ""}
        onchange={(e) => {
          const id = fontIdFromSelect((e.target as HTMLSelectElement).value);
          onUpdate({ fontId: id === null ? undefined : id } as any);
        }}
      >
        <option value="">По умолчанию (Century Gothic)</option>
        {#each fonts as font}
          <option value={font.ID.toString()}>{font.name}</option>
        {/each}
      </select>
    </div>
  </section>

  <!-- Рамка -->
  <section class="flex flex-col gap-2">
    <div class="section-head">
      <MuiIcon name="rounded_corner" style="font-size: 1rem" />
      Рамка
    </div>
    <div class="grid grid-cols-2 gap-2">
      <label class="swatch">
        <span class="swatch-color" style:background-color={style.borderColor}></span>
        <span class="swatch-hex">{style.borderColor.toUpperCase()}</span>
        <input
          type="color"
          class="swatch-input"
          value={style.borderColor}
          oninput={(e) => debounce("borderColor", () => onUpdate({ borderColor: (e.target as HTMLInputElement).value }), 150)}
        />
      </label>
      <select
        class="select select-bordered select-sm"
        value={style.borderStyle}
        onchange={(e) => onUpdate({ borderStyle: (e.target as HTMLSelectElement).value })}
      >
        <option value="solid">сплошная</option>
        <option value="dashed">штрих</option>
        <option value="dotted">точки</option>
        <option value="none">нет</option>
      </select>
      <label class="num-field">
        <span class="num-label">Толщина</span>
        <input
          type="number"
          min="0"
          value={style.borderWidth}
          oninput={(e) =>
            debounce("borderWidth", () => onUpdate({ borderWidth: parseInt((e.target as HTMLInputElement).value) || 0 }), 150)}
        />
        <span class="num-unit">px</span>
      </label>
      <label class="num-field">
        <span class="num-label">Скругление</span>
        <input
          type="number"
          min="0"
          value={style.borderRadius}
          oninput={(e) =>
            debounce("borderRadius", () => onUpdate({ borderRadius: parseInt((e.target as HTMLInputElement).value) || 0 }), 150)}
        />
        <span class="num-unit">px</span>
      </label>
    </div>
  </section>

  <!-- Отступы -->
  <section class="flex flex-col gap-2">
    <div class="section-head">
      <MuiIcon name="format_size" style="font-size: 1rem" />
      Отступы
    </div>
    <div class="grid grid-cols-2 gap-2">
      <label class="num-field">
        <span class="num-label">Внутр.</span>
        <input
          type="number"
          min="0"
          value={style.padding}
          oninput={(e) =>
            debounce("padding", () => onUpdate({ padding: parseInt((e.target as HTMLInputElement).value) || 0 }), 150)}
        />
        <span class="num-unit">px</span>
      </label>
      <label class="num-field">
        <span class="num-label">Внешн.</span>
        <input
          type="number"
          min="0"
          value={style.margin}
          oninput={(e) =>
            debounce("margin", () => onUpdate({ margin: parseInt((e.target as HTMLInputElement).value) || 0 }), 150)}
        />
        <span class="num-unit">px</span>
      </label>
    </div>
  </section>

  <!-- Тень -->
  <section class="flex flex-col gap-2">
    <div class="section-head">
      <MuiIcon name="blur_on" style="font-size: 1rem" />
      Тень текста
    </div>
    <div class="grid grid-cols-2 gap-2">
      <label class="swatch">
        <span class="swatch-color" style:background-color={shadowColor}></span>
        <span class="swatch-hex">{shadowColor.toUpperCase()}</span>
        <input
          type="color"
          class="swatch-input"
          value={shadowColor}
          oninput={(e) => { shadowColor = (e.target as HTMLInputElement).value; onShadowChange(); }}
        />
      </label>
      <label class="num-field">
        <span class="num-label">X</span>
        <input
          type="number"
          value={shadowX}
          oninput={(e) => { shadowX = parseInt((e.target as HTMLInputElement).value) || 0; onShadowChange(); }}
        />
        <span class="num-unit">px</span>
      </label>
      <label class="num-field">
        <span class="num-label">Y</span>
        <input
          type="number"
          value={shadowY}
          oninput={(e) => { shadowY = parseInt((e.target as HTMLInputElement).value) || 0; onShadowChange(); }}
        />
        <span class="num-unit">px</span>
      </label>
      <label class="num-field">
        <span class="num-label">Размытие</span>
        <input
          type="number"
          min="0"
          value={shadowBlur}
          oninput={(e) => { shadowBlur = parseInt((e.target as HTMLInputElement).value) || 0; onShadowChange(); }}
        />
        <span class="num-unit">px</span>
      </label>
    </div>
  </section>
</div>

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
    padding: 0.375rem 0.5rem;
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
    width: 1.5rem;
    height: 1.5rem;
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
  .swatch-color {
    background-blend-mode: normal;
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

  .num-field {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.25rem 0.625rem;
    border: 1px solid oklch(var(--b3));
    border-radius: 0.5rem;
    background: oklch(var(--b1));
    min-height: 2rem;
  }
  .num-field:focus-within {
    border-color: oklch(var(--p));
  }
  .num-label {
    font-size: 0.7rem;
    color: oklch(var(--bc) / 0.6);
    white-space: nowrap;
  }
  .num-field input {
    flex: 1;
    min-width: 0;
    background: transparent;
    border: none;
    outline: none;
    font-size: 0.875rem;
    color: inherit;
  }
  .num-unit {
    font-size: 0.7rem;
    color: oklch(var(--bc) / 0.4);
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
