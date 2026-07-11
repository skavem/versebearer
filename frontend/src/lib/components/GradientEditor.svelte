<script lang="ts">
  import {
    GRADIENT_PRESETS,
    gradientToCss,
    parseGradient,
    stringifyGradient,
    type Gradient,
    type GradientType,
  } from "$lib/gradient";
  import MuiIcon from "./MuiIcon.svelte";

  let {
    value,
    onChange,
  }: {
    value: string;
    onChange: (json: string) => void;
  } = $props();

  let g = $state<Gradient>(parseGradient(value));
  // Plain (non-reactive) tracker of the last json we emitted/synced, so the
  // effect below only re-parses on genuine external changes.
  let lastJson = stringifyGradient(parseGradient(value));

  // Re-sync when the value changes from outside (theme switch/reset), but not
  // as an echo of our own emit.
  $effect(() => {
    if (value && value !== lastJson) {
      g = parseGradient(value);
      lastJson = value;
    }
  });

  function emit() {
    lastJson = stringifyGradient(g);
    onChange(lastJson);
  }

  function setType(t: GradientType) {
    g.type = t;
    emit();
  }

  function addStop() {
    if (g.stops.length >= 5) return;
    const last = g.stops[g.stops.length - 1];
    g.stops = [...g.stops, { color: last?.color ?? "#ffffff", pos: 100 }];
    emit();
  }

  function removeStop(i: number) {
    if (g.stops.length <= 2) return;
    g.stops = g.stops.filter((_, idx) => idx !== i);
    emit();
  }

  function applyPreset(preset: Gradient) {
    g = structuredClone(preset);
    emit();
  }

  const types: { key: GradientType; label: string }[] = [
    { key: "linear", label: "Линейный" },
    { key: "radial", label: "Радиальный" },
    { key: "conic", label: "Конический" },
  ];

  const css = $derived(gradientToCss(g));
</script>

<div class="flex flex-col gap-3">
  <!-- Preview -->
  <div class="grad-preview" style:background={css}></div>

  <!-- Type -->
  <div class="join">
    {#each types as t (t.key)}
      <button
        class="btn btn-xs join-item {g.type === t.key ? 'btn-neutral' : 'btn-outline'}"
        onclick={() => setType(t.key)}
      >
        {t.label}
      </button>
    {/each}
  </div>

  <!-- Angle (linear / conic) -->
  {#if g.type !== "radial"}
    <div class="flex items-center gap-2">
      <span class="text-xs text-base-content/60 whitespace-nowrap">Угол: {g.angle}°</span>
      <input
        type="range"
        class="range range-xs range-primary"
        min="0"
        max="360"
        step="1"
        value={g.angle}
        oninput={(e) => {
          g.angle = parseInt((e.target as HTMLInputElement).value) || 0;
          emit();
        }}
      />
    </div>
  {/if}

  <!-- Stops -->
  <div class="flex flex-col gap-2">
    {#each g.stops as stop, i (i)}
      <div class="flex items-center gap-2">
        <label class="swatch">
          <span class="swatch-color" style:background-color={stop.color}></span>
          <input
            type="color"
            class="swatch-input"
            value={stop.color}
            oninput={(e) => {
              g.stops[i].color = (e.target as HTMLInputElement).value;
              emit();
            }}
          />
        </label>
        <input
          type="range"
          class="range range-xs range-primary flex-1"
          min="0"
          max="100"
          step="1"
          value={stop.pos}
          oninput={(e) => {
            g.stops[i].pos = parseInt((e.target as HTMLInputElement).value) || 0;
            emit();
          }}
        />
        <span class="w-10 text-right text-xs text-base-content/60">{stop.pos}%</span>
        <button
          class="btn btn-ghost btn-xs btn-square"
          onclick={() => removeStop(i)}
          disabled={g.stops.length <= 2}
          title="Убрать точку"
          aria-label="Убрать точку"
        >
          <MuiIcon name="close" style="font-size: 0.9rem" />
        </button>
      </div>
    {/each}
    <button
      class="btn btn-ghost btn-xs w-fit gap-1"
      onclick={addStop}
      disabled={g.stops.length >= 5}
    >
      <MuiIcon name="add" style="font-size: 1rem" />
      Добавить цвет
    </button>
  </div>

  <!-- Presets -->
  <div class="flex flex-col gap-1">
    <span class="text-xs text-base-content/60">Пресеты</span>
    <div class="flex flex-wrap gap-1.5">
      {#each GRADIENT_PRESETS as preset (preset.name)}
        <button
          class="preset"
          style:background={gradientToCss(preset.gradient)}
          onclick={() => applyPreset(preset.gradient)}
          title={preset.name}
          aria-label={preset.name}
        ></button>
      {/each}
    </div>
  </div>
</div>

<style>
  .grad-preview {
    height: 3rem;
    border-radius: 0.5rem;
    border: 1px solid oklch(var(--b3));
  }

  .swatch {
    position: relative;
    display: inline-flex;
    cursor: pointer;
  }
  .swatch-color {
    display: block;
    width: 1.75rem;
    height: 1.75rem;
    border-radius: 0.375rem;
    border: 1px solid oklch(var(--b3));
  }
  .swatch-input {
    position: absolute;
    inset: 0;
    opacity: 0;
    cursor: pointer;
  }

  .preset {
    width: 2.25rem;
    height: 1.5rem;
    border-radius: 0.375rem;
    border: 1px solid oklch(var(--b3));
    cursor: pointer;
    transition: transform 0.08s;
  }
  .preset:hover {
    transform: scale(1.06);
  }
</style>
