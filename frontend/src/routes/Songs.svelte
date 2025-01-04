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

<div class="flex h-[calc(100vh-4rem)] flex-row gap-2 p-4">
  <div class="flex w-1/3 flex-col gap-2">
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
        <span class="badge badge-neutral font-semibold">{i.number}</span>
      {/snippet}
      {#snippet rightMark(i)}
        <button
          class="btn btn-neutral btn-xs invisible text-white group-hover/item:visible"
          onclick={(e) => {
            favorites.add(i);
            e.preventDefault();
          }}><MuiIcon name="star" style="font-size: 1rem" /></button
        >
      {/snippet}
    </List>

    <CreateSongModal />
  </div>

  <div class="flex w-2/3 flex-col gap-2">
    <Select
      items={couplets.list}
      activeItem={couplets.active}
      getName={(i) => i.text}
      setActiveItem={(i) => (couplets.active = i)}
    />

    <CoupletsList />

    <div class="flex justify-center gap-2">
      <button
        class="bg-seawave flex cursor-pointer gap-2 rounded p-2 px-4 text-white"
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

    <div class="h-1/3 w-full">
      <List
        items={favorites.list}
        activeItem={null}
        getName={(i) => i.title}
        onClick={(i) => (songs.active = i)}
      >
        {#snippet leftMark(i)}
          <span class="badge badge-neutral text-white">{i.number}</span>
        {/snippet}
      </List>
    </div>
  </div>
</div>
