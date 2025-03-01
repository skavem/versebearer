<script lang="ts">
  import { songsStore } from "$lib/stores/songsStore.svelte";
  import Select from "./Select.svelte";

  const songs = $derived(songsStore.songs);
</script>

<Select
  items={songs.list}
  activeItem={songs.active}
  getName={(i) => `${i.number} - ${i.title}`}
  getSearchLabel={(i) => `${i.number} ${i.title}`}
  setActiveItem={(i) => (songs.active = i)}
/>
