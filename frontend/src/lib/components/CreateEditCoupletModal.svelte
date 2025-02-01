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
  }: {
    isModalOpen: boolean;
    selected: Couplet | null;
    isEdit: boolean;
  } = $props();

  let songId = $derived(songsStore.songs.active?.ID);

  let number = $state(1);
  let text = $state("");
  let label = $state("");
  $effect(() => {
    if (!selected) return;
    number = selected.number + 1;
    if (!isEdit) return;
    text = selected.text;
    label = selected.label;
    number = selected.number;
  });

  const close = () => {
    isModalOpen = false;
    text = "";
    label = "";
  };
</script>

<dialog class="modal" open={isModalOpen} onclose={close}>
  <div class="modal-box">
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
        <input
          bind:value={label}
          type="text"
          placeholder="Тип куплета"
          class="input input-bordered w-full"
        />
      </label>

      <label class="form-control">
        <div class="label pt-0">
          <span class="label-text">Текст</span>
        </div>
        <textarea
          class="textarea textarea-bordered h-24"
          placeholder="Bio"
          bind:value={text}
          rows="6"
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
