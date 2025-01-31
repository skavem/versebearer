<script lang="ts">
  import { CloseScreen, ShowScreen } from "$lib/bindings/changeme/dbhandler";
  import { screenStore } from "$lib/stores/screenStore.svelte";
  import type { Screens } from "@wailsio/runtime";
  import MuiIcon from "./MuiIcon.svelte";

  type screen = Screens.Screen;

  const { scr } = $props<{
    scr: screen;
  }>();
  const id = `screen ${scr.ID}`;

  let activeScreens = $derived(screenStore.activeScreens);
  let toggled = $derived(screenStore.activeScreens.includes(id));
</script>

<button
  class={["btn btn-outline h-max p-2", toggled && "btn-active"]}
  onclick={() => {
    if (!toggled) {
      const rect = scr.Bounds;
      ShowScreen(rect.X, rect.Y, rect.Width, rect.Height, id);
      activeScreens.push(id);
    } else {
      CloseScreen(id);
      screenStore.activeScreens = activeScreens.filter((s) => s !== id);
    }
  }}
>
  <div class="flex flex-col gap-1">
    <MuiIcon name="monitor" style="font-size: 3rem" />
    <div>Имя: {scr.Name.replace(/[\\\.]/g, "")}</div>
    <div>ID: {scr.ID}</div>
    <div>
      X: {scr.Bounds.X}
      Y: {scr.Bounds.Y}
    </div>
  </div>
</button>
