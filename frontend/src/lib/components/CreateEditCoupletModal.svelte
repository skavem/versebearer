<script lang="ts">
  import type { Couplet } from "$lib/bindings/changeme/backend/models";
  import {
    CreateCouplet,
    UpdateCouplet,
  } from "$lib/bindings/changeme/dbhandler";
  import { songsStore } from "$lib/stores/songsStore.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  let {
    isModalOpen = $bindable(),
    selected = $bindable(),
    isEdit = $bindable(),
    number = $bindable(),
  }: {
    isModalOpen: boolean;
    selected: Couplet | null;
    isEdit: boolean;
    number: number;
  } = $props();

  let songId = $derived(songsStore.songs.active?.ID);

  // let number = $state(1);
  let text = $state("");
  let label = $state("");
  $effect(() => {
    isModalOpen;
    if (!selected) {
      text = "";
      label = "";
      return;
    }
    if (!isEdit) {
      text = "";
      label = "";
      return;
    }
    text = selected.text;
    label = selected.label;
    number = selected.number;
  });

  const close = () => {
    isModalOpen = false;
  };
</script>

<dialog class="modal" open={isModalOpen} onclose={close}>
  <div class="modal-box max-w-full lg:w-1/2">
    <div class="mb-4 text-lg font-bold">Добавить куплет</div>

    <button
      class="btn btn-sm btn-circle btn-ghost absolute right-2 top-2"
      onclick={close}
    >
      <MuiIcon name="close" />
    </button>

    <div class="flex flex-col gap-4">
      <label class="form-control w-full">
        <div class="label pt-0">
          <span class="label-text">Тип</span>
        </div>
        <div class="flex gap-2">
          <input
            bind:value={label}
            type="text"
            placeholder="Тип куплета"
            class="input input-bordered w-full"
            onkeydown={(e) => {
              if (e.code === "Escape") {
                close();
              }
              e.stopPropagation();
            }}
          />
          <button
            class="btn btn-outline btn-secondary"
            onclick={() => (label = "Куплет")}>Куплет</button
          >
          <button
            class="btn btn-outline btn-secondary"
            onclick={() => (label = "Припев")}>Припев</button
          >
          <button
            class="btn btn-outline btn-secondary"
            onclick={() => (label = "Бридж")}>Бридж</button
          >
        </div>
      </label>

      <label class="form-control">
        <div class="label pt-0">
          <span class="label-text">Текст</span>
        </div>
        <textarea
          bind:value={text}
          class="textarea textarea-bordered"
          placeholder="Bio"
          rows="8"
          onkeydown={(e) => {
            if (e.code === "Escape") {
              close();
            }
            e.stopPropagation();
          }}
        >
        </textarea>
      </label>

      <div class="flex flex-grow flex-row items-center justify-end">
        <button
          class="btn btn-neutral"
          onclick={async () => {
            if (!songId) return;
            if (!isEdit) {
              await CreateCouplet(text, label, number, songId);
            } else if (selected) {
              await UpdateCouplet(selected.ID, label, text, number);
            }
            close();
          }}
        >
          {isEdit ? "Сохранить" : "Создать"}
        </button>
      </div>
    </div>
  </div>

  <form method="dialog" class="modal-backdrop">
    <button class="bg-black/70">close</button>
  </form>
</dialog>
