<script lang="ts">
  import ScreenToggler from "$lib/components/ScreenToggler.svelte";
  import { screenStore } from "$lib/stores/screenStore.svelte";

  const screens = $derived(screenStore.list);
</script>

<div class="flex flex-col gap-4 p-4">
  <div class="flex select-none flex-row gap-2">
    {#each screens as scr, ind}
      <ScreenToggler {scr} />
    {/each}
  </div>
</div>
