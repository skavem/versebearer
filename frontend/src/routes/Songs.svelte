<script lang="ts">
  import {
    GetSongs,
    HideCouplet,
    ShowCouplet,
  } from "$lib/bindings/changeme/dbhandler";
  import CoupletsList from "$lib/components/CoupletsList.svelte";
  import CreateSongModal from "$lib/components/CreateSongModal.svelte";
  import List from "$lib/components/List.svelte";
  import MuiIcon from "$lib/components/MuiIcon.svelte";
  import Select from "$lib/components/Select.svelte";
  import { songsStore } from "$lib/stores/songsStore.svelte";

  GetSongs().then((s) => (songsStore.songs.list = s));
  const songs = $derived(songsStore.songs);
  const couplets = $derived(songsStore.couplets);
  const shown = $derived(songsStore.couplets.shown);
  const favorites = $derived(songsStore.favorites);

  const showCouplet = () => {
    if (couplets.active) {
      ShowCouplet(couplets.active.ID);
    }
  };

  $effect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      switch (e.code) {
        case "Escape":
          HideCouplet();
          e.preventDefault();
          return;
        case "Enter":
          showCouplet();
          e.preventDefault();
          return;
        case "ArrowDown":
          couplets.next();
          e.preventDefault();
          return;
        case "ArrowUp":
          couplets.prev();
          e.preventDefault();
          return;
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  });
</script>

<div class="flex flex-row p-4 h-[calc(100vh-4rem)] gap-2">
  <div class="flex flex-col w-1/3 gap-2">
    <Select
      items={songs.list}
      activeItem={songs.active}
      getName={(i) => `${i.number} - ${i.title}`}
      getSearchLabel={(i) => `${i.number} ${i.title}`}
      setActiveItem={(i) => (songs.active = i)}
    />

    <List
      items={songs.list}
      activeItem={songs.active}
      getName={(i) => i.title}
      onClick={(i) => (songs.active = i)}
    >
      {#snippet leftMark(i)}
        <span class="bg-sealight px-1 rounded text-white">{i.number}</span>
      {/snippet}
      {#snippet rightMark(i)}
        <button
          class="hidden h-full items-center group-hover/item:flex hover:bg-sealight hover:text-white p-1 rounded"
          onclick={(e) => {
            favorites.add(i);
            e.preventDefault();
          }}><MuiIcon name="star" style="font-size: 1rem" /></button
        >
      {/snippet}
    </List>

    <CreateSongModal />
  </div>

  <div class="flex flex-col gap-2 w-2/3">
    <Select
      items={couplets.list}
      activeItem={couplets.active}
      getName={(i) => i.text}
      setActiveItem={(i) => (couplets.active = i)}
    />

    <CoupletsList />

    <div class="flex justify-center gap-2">
      <button
        class="bg-seawave text-white rounded p-2 px-4 flex gap-2 cursor-pointer"
        onclick={() => {
          console.log(shown?.ID, couplets.active?.ID);
          if (!shown) {
            showCouplet();
          } else {
            HideCouplet();
          }
        }}
      >
        <MuiIcon name={shown ? "visibility_off" : "visibility"} />
        {shown ? "СКРЫТЬ" : "ПОКАЗАТЬ"}
      </button>
    </div>

    <div class="w-full h-1/3">
      <List
        items={favorites.list}
        activeItem={null}
        getName={(i) => i.title}
        onClick={(i) => (songs.active = i)}
      >
        {#snippet leftMark(i)}
          <span class="bg-sealight rounded text-white px-2">{i.number}</span>
        {/snippet}
      </List>
    </div>
  </div>
</div>
