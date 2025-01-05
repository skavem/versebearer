<script lang="ts">
  import { CloseScreen, ShowScreen } from "$lib/bindings/changeme/dbhandler";
  import type { Screens as S } from "@wailsio/runtime";
  import { Screens } from "@wailsio/runtime";
  type Screen = S.Screen;

  let selected = $state<Screen | null>(null);
  $effect(() => {
    if (selected) {
      const rect = selected.Bounds;
      console.log(rect);
      ShowScreen(rect.X, rect.Y, rect.Width, rect.Height);
    }
  });
  let screens = $state<Screen[]>([]);
  Screens.GetAll().then((s) => (screens = s));
</script>

<div class="flex select-none flex-row gap-2 p-4">
  {#each screens as scr, ind}
    <button
      class="cursor-pointer p-4 shadow-lg"
      onclick={() => {
        selected = scr;
      }}
    >
      <div>Имя: {scr.Name}</div>
      <div>
        X: {scr.Bounds.X}
        Y: {scr.Bounds.Y}
      </div>
    </button>
  {/each}
</div>

<button
  onclick={() => {
    CloseScreen();
  }}>hide screen</button
>
