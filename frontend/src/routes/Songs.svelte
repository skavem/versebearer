<script lang="ts">
  import {
    HideCouplet,
    HideQR,
    ShowCouplet,
    ShowQR,
  } from "$lib/bindings/changeme/dbhandler";
  import CoupletsList from "$lib/components/CoupletsList.svelte";
  import CreateSongModal from "$lib/components/CreateSongModal.svelte";
  import List from "$lib/components/List.svelte";
  import MuiIcon from "$lib/components/MuiIcon.svelte";
  import Select from "$lib/components/Select.svelte";
  import SongsSelect from "$lib/components/SongsSelect.svelte";
  import { songsStore } from "$lib/stores/songsStore.svelte";

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
  <div class="flex w-1/3 flex-col gap-2 lg:w-1/5">
    <SongsSelect />

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
          class="btn btn-neutral btn-xs hidden px-1 text-white group-hover/item:block"
          onclick={(e) => {
            favorites.add(i);
            e.stopPropagation();
          }}><MuiIcon name="star" style="font-size: 1rem" /></button
        >
      {/snippet}
    </List>

    <CreateSongModal />
  </div>

  <div class="flex w-2/3 flex-col gap-2 lg:w-4/5">
    <div class="flex h-4/5 flex-col gap-2">
      <Select
        items={couplets.list}
        activeItem={couplets.active}
        getName={(i) => i.text}
        setActiveItem={(i) => (couplets.active = i)}
      />

      <CoupletsList />

      <div class="flex justify-center gap-2">
        <button
          class="btn btn-neutral btn-sm"
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

        <button
          class={[
            "btn btn-sm btn-secondary btn-square",
            {
              "btn-outline": !songsStore.qr,
            },
          ]}
          onclick={() => {
            songsStore.qr = !songsStore.qr;
            if (songsStore.qr) {
              ShowQR();
            } else {
              HideQR();
            }
          }}
        >
          <MuiIcon name="qr_code" />
        </button>
      </div>
    </div>

    <div class="h-1/5 w-full">
      <List
        items={favorites.list}
        activeItem={favorites.active}
        getName={(i) => i.title}
        getActiveKey={(i) => i.localId}
        onClick={(i) => {
          songs.active = i;
          favorites.active = i;
        }}
        getKey={(_, n) => n.toString()}
      >
        {#snippet leftMark(i)}
          <span class="badge badge-neutral text-white">{i.number}</span>
        {/snippet}
        {#snippet rightMark(s)}
          <div class="flex flex-row gap-1">
            <button
              class="btn btn-neutral btn-xs hidden px-1 text-white group-hover/item:block"
              onclick={(e) => {
                favorites.remove(s.localId);
                e.stopPropagation();
              }}><MuiIcon name="delete" style="font-size: 1rem" /></button
            >

            <button
              class="btn btn-neutral btn-xs hidden px-1 text-white group-hover/item:block"
              onclick={(e) => {
                favorites.moveDown(s);
                e.stopPropagation();
              }}
            >
              <MuiIcon name="arrow_downward" style="font-size: 1rem" />
            </button>

            <button
              class="btn btn-neutral btn-xs hidden px-1 text-white group-hover/item:block"
              onclick={(e) => {
                favorites.moveUp(s);
                e.stopPropagation();
              }}
            >
              <MuiIcon name="arrow_upward" style="font-size: 1rem" />
            </button>
          </div>
        {/snippet}
      </List>
    </div>
  </div>
</div>
