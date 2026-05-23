<script lang="ts">
  import { screenId, screenStore } from "$lib/stores/screenStore.svelte";
  import type { Screens } from "@wailsio/runtime";
  import MuiIcon from "./MuiIcon.svelte";

  type Screen = Screens.Screen;

  const { scr, index } = $props<{
    scr: Screen;
    index: number;
  }>();
  const id = screenId(scr);

  const projecting = $derived(screenStore.activeScreens.includes(id));
  const isCurrent = $derived(screenStore.currentScreenID === scr.ID);
  const aspectRatio = $derived((scr.Bounds.Width / scr.Bounds.Height).toFixed(2));
  const scalePct = $derived(Math.round(scr.ScaleFactor * 100));
</script>

<div
  class={[
    "card border bg-base-100 transition-all",
    projecting ? "border-neutral shadow-md shadow-neutral/20" : "border-base-300",
    isCurrent && "ring-2 ring-secondary ring-offset-2 ring-offset-base-100",
  ]}
>
  <div class="card-body p-4">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2">
        <div
          class={[
            "flex h-10 w-10 items-center justify-center rounded-lg",
            projecting ? "bg-neutral text-neutral-content" : "bg-base-200",
          ]}
        >
          <MuiIcon name="monitor" style="font-size: 1.5rem" />
        </div>
        <div>
          <div class="text-lg font-bold leading-tight">
            Монитор {index + 1}
          </div>
          <div class="text-xs leading-tight text-base-content/60">
            {scr.Name.replace(/[\\.]/g, "")}
          </div>
        </div>
      </div>
      <div class="flex flex-col items-end gap-1">
        {#if scr.IsPrimary}
          <span class="badge badge-sm badge-ghost">Основной</span>
        {/if}
        {#if isCurrent}
          <span class="badge badge-sm badge-secondary">Здесь</span>
        {/if}
        {#if projecting}
          <span class="badge badge-sm badge-neutral">В эфире</span>
        {/if}
      </div>
    </div>

    <div class="mt-2 grid grid-cols-2 gap-2 text-sm">
      <div class="flex flex-col rounded-md bg-base-200/60 p-2">
        <span class="text-[10px] uppercase tracking-wide text-base-content/50">
          Разрешение
        </span>
        <span class="font-mono font-semibold">
          {scr.Bounds.Width}×{scr.Bounds.Height}
        </span>
      </div>
      <div class="flex flex-col rounded-md bg-base-200/60 p-2">
        <span class="text-[10px] uppercase tracking-wide text-base-content/50">
          Масштаб
        </span>
        <span class="font-mono font-semibold">{scalePct}%</span>
      </div>
      <div class="flex flex-col rounded-md bg-base-200/60 p-2">
        <span class="text-[10px] uppercase tracking-wide text-base-content/50">
          Позиция
        </span>
        <span class="font-mono font-semibold">
          {scr.Bounds.X}, {scr.Bounds.Y}
        </span>
      </div>
      <div class="flex flex-col rounded-md bg-base-200/60 p-2">
        <span class="text-[10px] uppercase tracking-wide text-base-content/50">
          Соотношение
        </span>
        <span class="font-mono font-semibold">{aspectRatio}:1</span>
      </div>
    </div>

    <button
      onclick={() => screenStore.requestToggle(scr)}
      class={[
        "btn btn-sm mt-2 w-full",
        projecting ? "btn-outline btn-error" : "btn-neutral",
      ]}
    >
      <MuiIcon
        name={projecting ? "cancel_presentation" : "present_to_all"}
        style="font-size: 1.1rem"
      />
      {projecting ? "Остановить трансляцию" : "Транслировать"}
    </button>
  </div>
</div>
