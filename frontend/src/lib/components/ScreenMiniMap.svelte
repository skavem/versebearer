<script lang="ts">
  import { screenId, screenStore } from "$lib/stores/screenStore.svelte";
  import type { Screens } from "@wailsio/runtime";

  type Screen = Screens.Screen;

  const screens = $derived(screenStore.list);

  const VIEW_W = 640;
  const VIEW_H = 200;
  const PAD = 16;

  const bounds = $derived.by(() => {
    if (screens.length === 0) {
      return { minX: 0, minY: 0, maxX: 1, maxY: 1, scale: 1 };
    }
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (const s of screens) {
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

  function toggle(s: Screen) {
    screenStore.requestToggle(s);
  }
</script>

<div class="rounded-2xl border border-base-300 bg-base-200/40 p-4">
  <div class="mb-3 flex items-baseline justify-between">
    <h3 class="text-sm font-semibold uppercase tracking-wide text-base-content/70">
      Расположение мониторов
    </h3>
    <span class="text-xs text-base-content/50">
      кликни — переключить трансляцию
    </span>
  </div>
  <svg
    viewBox="0 0 {VIEW_W} {VIEW_H}"
    class="h-auto w-full select-none"
    role="img"
    aria-label="Спатиал-карта мониторов"
  >
    {#each screens as s, i (s.ID)}
      {@const rect = project(s)}
      {@const id = screenId(s)}
      {@const projecting = screenStore.activeScreens.includes(id)}
      {@const isCurrent = screenStore.currentScreenID === s.ID}
      <g
        class="cursor-pointer"
        onclick={() => toggle(s)}
        onkeydown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
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
            projecting ? "fill-neutral" : "fill-base-100",
            "stroke-2",
            isCurrent ? "stroke-secondary" : "stroke-base-300",
          ]}
        />
        <text
          x={rect.x + rect.w / 2}
          y={rect.y + rect.h / 2 - 6}
          text-anchor="middle"
          dominant-baseline="middle"
          class={[
            "text-[16px] font-bold",
            projecting ? "fill-neutral-content" : "fill-base-content",
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
            projecting ? "fill-neutral-content/80" : "fill-base-content/60",
          ]}
        >
          {s.Bounds.Width}×{s.Bounds.Height}
        </text>
        {#if s.IsPrimary}
          <text
            x={rect.x + 6}
            y={rect.y + 12}
            class={[
              "text-[8px] font-medium uppercase",
              projecting ? "fill-neutral-content/60" : "fill-base-content/60",
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
            class="fill-secondary text-[8px] font-bold uppercase"
          >
            здесь
          </text>
        {/if}
      </g>
    {/each}
  </svg>
</div>
