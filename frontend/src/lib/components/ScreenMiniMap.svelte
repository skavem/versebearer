<script lang="ts">
  import { outputStore } from "$lib/stores/outputStore.svelte";
  import type { Screens } from "@wailsio/runtime";
  import type { Output } from "$lib/bindings/changeme/backend/models";
  import MuiIcon from "./MuiIcon.svelte";

  type Screen = Screens.Screen;

  const monitors = $derived(outputStore.monitors);

  const VIEW_W = 640;
  const VIEW_H = 200;
  const PAD = 16;

  const bounds = $derived.by(() => {
    if (monitors.length === 0) {
      return { minX: 0, minY: 0, maxX: 1, maxY: 1, scale: 1 };
    }
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (const s of monitors) {
      minX = Math.min(minX, s.Bounds.X);
      minY = Math.min(minY, s.Bounds.Y);
      maxX = Math.max(maxX, s.Bounds.X + s.Bounds.Width);
      maxY = Math.max(maxY, s.Bounds.Y + s.Bounds.Height);
    }
    const realW = maxX - minX;
    const realH = maxY - minY;
    const usableW = VIEW_W - PAD * 2;
    const usableH = VIEW_H - PAD * 2;
    const scale = Math.min(usableW / realW, usableH / realH);
    return { minX, minY, maxX, maxY, scale };
  });

  function project(s: Screen) {
    const { minX, minY, scale } = bounds;
    const realW = bounds.maxX - bounds.minX;
    const realH = bounds.maxY - bounds.minY;
    const offX = (VIEW_W - realW * scale) / 2;
    const offY = (VIEW_H - realH * scale) / 2;
    return {
      x: offX + (s.Bounds.X - minX) * scale,
      y: offY + (s.Bounds.Y - minY) * scale,
      w: s.Bounds.Width * scale,
      h: s.Bounds.Height * scale,
    };
  }

  // First output assigned to this monitor, if any — the minimap toggles
  // that single output on click. Monitors without an assigned output are
  // shown but not interactive.
  function outputFor(s: Screen): Output | undefined {
    return outputStore.outputs.find((o) => o.screenId === s.ID);
  }

  function toggle(s: Screen) {
    const o = outputFor(s);
    if (o) outputStore.requestToggle(o);
  }
</script>

<div class="flex flex-col gap-3 rounded-xl border border-base-300 bg-base-100 p-4 shadow-sm">
  <div class="flex items-baseline justify-between border-b border-base-300 pb-2.5">
    <div class="section-head">
      <MuiIcon name="dashboard" style="font-size: 1rem" />
      Расположение мониторов
    </div>
    <span class="text-xs text-base-content/40">
      клик — переключить трансляцию назначенного выхода
    </span>
  </div>
  <svg
    viewBox="0 0 {VIEW_W} {VIEW_H}"
    class="h-auto w-full select-none"
    role="img"
    aria-label="Спатиал-карта мониторов"
  >
    {#each monitors as s, i (s.ID)}
      {@const rect = project(s)}
      {@const output = outputFor(s)}
      {@const projecting = !!output && outputStore.isActive(output)}
      {@const isCurrent = outputStore.currentScreenID === s.ID}
      <g
        class={output ? "cursor-pointer" : "cursor-default"}
        onclick={() => toggle(s)}
        onkeydown={(e) => {
          if (output && (e.key === "Enter" || e.key === " ")) {
            e.preventDefault();
            toggle(s);
          }
        }}
        role="button"
        tabindex="0"
        aria-label="Монитор {i + 1}"
      >
        <rect
          x={rect.x}
          y={rect.y}
          width={rect.w}
          height={rect.h}
          rx="6"
          ry="6"
          class={[
            "transition-all",
            // Янтарь = эфир, синий = «окно оператора здесь» — как в OutputCard.
            projecting ? "fill-secondary" : "fill-base-200",
            "stroke-2",
            isCurrent ? "stroke-primary" : "stroke-base-300",
            !output && "opacity-50",
          ]}
        />
        <text
          x={rect.x + rect.w / 2}
          y={rect.y + rect.h / 2 - 6}
          text-anchor="middle"
          dominant-baseline="middle"
          class={[
            "text-[16px] font-bold",
            projecting ? "fill-secondary-content" : "fill-base-content",
          ]}
        >
          {i + 1}
        </text>
        <text
          x={rect.x + rect.w / 2}
          y={rect.y + rect.h / 2 + 12}
          text-anchor="middle"
          dominant-baseline="middle"
          class={[
            "text-[9px]",
            projecting ? "fill-secondary-content/80" : "fill-base-content/60",
          ]}
        >
          {output ? output.name : s.Bounds.Width + "×" + s.Bounds.Height}
        </text>
        {#if s.IsPrimary}
          <text
            x={rect.x + 6}
            y={rect.y + 12}
            class={[
              "text-[8px] font-medium uppercase",
              projecting ? "fill-secondary-content/60" : "fill-base-content/60",
            ]}
          >
            primary
          </text>
        {/if}
        {#if isCurrent}
          <text
            x={rect.x + rect.w - 6}
            y={rect.y + 12}
            text-anchor="end"
            class="fill-primary text-[8px] font-bold uppercase"
          >
            здесь
          </text>
        {/if}
      </g>
    {/each}
  </svg>
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
</style>
