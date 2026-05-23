<script lang="ts">
  import type { Song } from "$lib/bindings/changeme/backend/models";
  import {
    HideCouplet,
    HideQR,
    RemoveSong,
    ShowCouplet,
    ShowQR,
  } from "$lib/bindings/changeme/dbhandler";
  import CoupletsList from "$lib/components/CoupletsList.svelte";
  import CreateSongModal from "$lib/components/CreateSongModal.svelte";
  import EditSongTextModal from "$lib/components/EditSongTextModal.svelte";
  import List from "$lib/components/List.svelte";
  import MuiIcon from "$lib/components/MuiIcon.svelte";
  import Select from "$lib/components/Select.svelte";
  import SongsSelect from "$lib/components/SongsSelect.svelte";
  import { songsStore } from "$lib/stores/songsStore.svelte";

  const songs = $derived(songsStore.songs);
  const couplets = $derived(songsStore.couplets);
  const shown = $derived(songsStore.couplets.shown);
  const favorites = $derived(songsStore.favorites);

  let songToDelete = $state<Song | null>(null);
  let isEditSongTextOpen = $state(false);

  const confirmDelete = async () => {
    if (!songToDelete) return;
    const id = songToDelete.ID;
    if (songs.active?.ID === id) {
      const list = songs.list;
      const idx = list.findIndex((s) => s.ID === id);
      songs.active = list[idx - 1] ?? list[idx + 1] ?? null;
    }
    songToDelete = null;
    await RemoveSong(id);
  };

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
        <div class="flex flex-row gap-1">
          <button
            class="btn btn-neutral btn-xs hidden px-1 text-white group-hover/item:block"
            onclick={(e) => {
              favorites.add(i);
              e.stopPropagation();
            }}
            title="В избранное"
            ><MuiIcon name="star" style="font-size: 1rem" /></button
          >
          <button
            class="btn btn-error btn-xs hidden px-1 text-white group-hover/item:block"
            onclick={(e) => {
              songToDelete = i;
              e.stopPropagation();
            }}
            title="Удалить песню"
            ><MuiIcon name="delete" style="font-size: 1rem" /></button
          >
        </div>
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
          class={[
            "btn btn-wide",
            shown ? "btn-outline btn-error" : "btn-neutral",
          ]}
          onclick={() => {
            if (!shown) {
              showCouplet();
            } else {
              HideCouplet();
            }
          }}
        >
          <MuiIcon name={shown ? "visibility_off" : "visibility"} />
          {shown ? "Скрыть куплет" : "Показать куплет"}
        </button>

        <button
          class={[
            "btn btn-square",
            songsStore.qr ? "btn-secondary" : "btn-outline btn-secondary",
          ]}
          onclick={() => {
            songsStore.qr = !songsStore.qr;
            if (songsStore.qr) {
              ShowQR();
            } else {
              HideQR();
            }
          }}
          title="QR-код"
        >
          <MuiIcon name="qr_code" />
        </button>

        <button
          class="btn btn-square btn-outline btn-neutral"
          disabled={!songs.active}
          onclick={() => (isEditSongTextOpen = true)}
          title="Редактировать всю песню"
        >
          <MuiIcon name="lyrics" />
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

<EditSongTextModal bind:isModalOpen={isEditSongTextOpen} />

{#if songToDelete}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-2 flex items-center gap-3">
        <div
          class="flex h-10 w-10 items-center justify-center rounded-full bg-error/10 text-error"
        >
          <MuiIcon name="delete" />
        </div>
        <h3 class="text-lg font-bold">Удалить песню?</h3>
      </div>

      <p class="py-2">
        Песня <span class="font-semibold"
          >№{songToDelete.number} «{songToDelete.title}»</span
        >
        и все её куплеты будут удалены безвозвратно.
      </p>

      <div class="modal-action">
        <button class="btn btn-ghost" onclick={() => (songToDelete = null)}>
          Отмена
        </button>
        <button class="btn btn-error" onclick={confirmDelete}>
          <MuiIcon name="delete" />
          Удалить
        </button>
      </div>
    </div>
    <button
      class="modal-backdrop"
      onclick={() => (songToDelete = null)}
      aria-label="Закрыть"
    ></button>
  </div>
{/if}
