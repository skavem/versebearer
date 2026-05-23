<script lang="ts">
  import type { VisualStyle } from "$lib/bindings/changeme/models";
  import type { Font } from "$lib/bindings/changeme/backend/models";

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

  // Debounce helper
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

  // Text shadow decomposed state (derived from style.textShadow)
  let shadowColor = $state("#000000");
  let shadowX = $state(0);
  let shadowY = $state(2);
  let shadowBlur = $state(4);

  $effect(() => {
    if (style.textShadow) {
      // parse "color offsetX offsetY blur" — simple format
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
</script>

<div class="flex flex-col gap-4 rounded-lg border border-base-300 p-4">
  <h3 class="text-lg font-semibold">{title}</h3>

  <!-- Background color + opacity -->
  <div class="flex flex-col gap-1">
    <span class="text-sm font-medium">Фон</span>
    <div class="flex items-center gap-3">
      <input
        type="color"
        class="input input-bordered h-10 w-14 cursor-pointer p-1"
        value={style.bgColor}
        oninput={(e) => debounce("bgColor", () => onUpdate({ bgColor: (e.target as HTMLInputElement).value }), 150)}
      />
      <div class="flex flex-1 flex-col gap-1">
        <span class="text-xs text-base-content/60">Непрозрачность: {Math.round(style.bgOpacity * 100)}%</span>
        <input
          type="range"
          class="range range-sm"
          min="0"
          max="100"
          step="1"
          value={Math.round(style.bgOpacity * 100)}
          oninput={(e) =>
            debounce("bgOpacity", () => onUpdate({ bgOpacity: parseInt((e.target as HTMLInputElement).value) / 100 }), 150)}
        />
      </div>
    </div>
  </div>

  <!-- Text color -->
  <div class="flex flex-col gap-1">
    <span class="text-sm font-medium">Цвет текста</span>
    <input
      type="color"
      class="input input-bordered h-10 w-14 cursor-pointer p-1"
      value={style.textColor}
      oninput={(e) => debounce("textColor", () => onUpdate({ textColor: (e.target as HTMLInputElement).value }), 150)}
    />
  </div>

  <!-- Font family -->
  <div class="flex flex-col gap-1">
    <span class="text-sm font-medium">Шрифт</span>
    <select
      class="select select-bordered w-full"
      value={style.fontId?.toString() ?? ""}
      onchange={(e) => {
        const val = (e.target as HTMLSelectElement).value;
        const id = fontIdFromSelect(val);
        onUpdate({ fontId: id === null ? undefined : id } as any);
      }}
    >
      <option value="">По умолчанию (Century Gothic)</option>
      {#each fonts as font}
        <option value={font.ID.toString()}>{font.name}</option>
      {/each}
    </select>
  </div>

  <!-- Border -->
  <div class="flex flex-col gap-1">
    <span class="text-sm font-medium">Рамка</span>
    <div class="grid grid-cols-2 gap-2">
      <div class="flex flex-col gap-1">
        <span class="text-xs text-base-content/60">Цвет</span>
        <input
          type="color"
          class="input input-bordered h-10 w-14 cursor-pointer p-1"
          value={style.borderColor}
          oninput={(e) => debounce("borderColor", () => onUpdate({ borderColor: (e.target as HTMLInputElement).value }), 150)}
        />
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-xs text-base-content/60">Стиль</span>
        <select
          class="select select-bordered select-sm w-full"
          value={style.borderStyle}
          onchange={(e) => onUpdate({ borderStyle: (e.target as HTMLSelectElement).value })}
        >
          <option value="solid">solid</option>
          <option value="dashed">dashed</option>
          <option value="dotted">dotted</option>
          <option value="none">none</option>
        </select>
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-xs text-base-content/60">Толщина (px)</span>
        <input
          type="number"
          class="input input-bordered input-sm w-full"
          min="0"
          value={style.borderWidth}
          oninput={(e) =>
            debounce("borderWidth", () => onUpdate({ borderWidth: parseInt((e.target as HTMLInputElement).value) || 0 }), 150)}
        />
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-xs text-base-content/60">Скругление (px)</span>
        <input
          type="number"
          class="input input-bordered input-sm w-full"
          min="0"
          value={style.borderRadius}
          oninput={(e) =>
            debounce("borderRadius", () => onUpdate({ borderRadius: parseInt((e.target as HTMLInputElement).value) || 0 }), 150)}
        />
      </div>
    </div>
  </div>

  <!-- Padding -->
  <div class="flex flex-col gap-1">
    <span class="text-sm font-medium">Отступ (px)</span>
    <input
      type="number"
      class="input input-bordered input-sm w-full"
      min="0"
      value={style.padding}
      oninput={(e) =>
        debounce("padding", () => onUpdate({ padding: parseInt((e.target as HTMLInputElement).value) || 0 }), 150)}
    />
  </div>

  <!-- Text shadow -->
  <div class="flex flex-col gap-1">
    <span class="text-sm font-medium">Тень текста</span>
    <div class="grid grid-cols-2 gap-2">
      <div class="flex flex-col gap-1">
        <span class="text-xs text-base-content/60">Цвет</span>
        <input
          type="color"
          class="input input-bordered h-10 w-14 cursor-pointer p-1"
          value={shadowColor}
          oninput={(e) => { shadowColor = (e.target as HTMLInputElement).value; onShadowChange(); }}
        />
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-xs text-base-content/60">Смещение X (px)</span>
        <input
          type="number"
          class="input input-bordered input-sm w-full"
          value={shadowX}
          oninput={(e) => { shadowX = parseInt((e.target as HTMLInputElement).value) || 0; onShadowChange(); }}
        />
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-xs text-base-content/60">Смещение Y (px)</span>
        <input
          type="number"
          class="input input-bordered input-sm w-full"
          value={shadowY}
          oninput={(e) => { shadowY = parseInt((e.target as HTMLInputElement).value) || 0; onShadowChange(); }}
        />
      </div>
      <div class="flex flex-col gap-1">
        <span class="text-xs text-base-content/60">Размытие (px)</span>
        <input
          type="number"
          class="input input-bordered input-sm w-full"
          min="0"
          value={shadowBlur}
          oninput={(e) => { shadowBlur = parseInt((e.target as HTMLInputElement).value) || 0; onShadowChange(); }}
        />
      </div>
    </div>
  </div>

  <!-- Reset button -->
  <button class="btn btn-outline btn-sm mt-2 w-full" onclick={onReset}>
    Сбросить
  </button>
</div>
