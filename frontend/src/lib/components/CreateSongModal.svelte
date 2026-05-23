<script lang="ts">
  import { CreateSong } from "$lib/bindings/changeme/dbhandler";
  import { songsStore } from "$lib/stores/songsStore.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  const songs = $derived(songsStore.songs);

  let isOpen = $state(false);
  let title = $state("");
  let number = $state(0);

  $effect(() => {
    if (isOpen) {
      number = (songs.list.at(-1)?.number ?? 0) + 1;
      title = "";
    }
  });

  async function submit() {
    if (!title.trim()) return;
    const created = await CreateSong(number, title.trim());
    if (created) {
      songs.active = created;
    }
    isOpen = false;
  }
</script>

<button
  class="btn btn-outline btn-sm w-full"
  onclick={() => (isOpen = true)}
>
  <MuiIcon name="add" />
  Добавить песню
</button>

{#if isOpen}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-2 flex items-center justify-between">
        <h3 class="text-lg font-bold">Новая песня</h3>
        <button
          class="btn btn-ghost btn-sm btn-square"
          onclick={() => (isOpen = false)}
          aria-label="Закрыть"
        >
          <MuiIcon name="close" />
        </button>
      </div>

      <div class="flex flex-col gap-3">
        <label class="form-control">
          <div class="label py-1">
            <span class="label-text font-medium">Номер</span>
          </div>
          <input
            type="number"
            bind:value={number}
            class="input input-bordered w-full"
          />
        </label>

        <label class="form-control">
          <div class="label py-1">
            <span class="label-text font-medium">Название</span>
          </div>
          <input
            type="text"
            bind:value={title}
            placeholder="Например: Великий Бог"
            class="input input-bordered w-full"
            onkeydown={(e) => {
              if (e.key === "Enter") submit();
            }}
          />
        </label>
      </div>

      <div class="modal-action">
        <button class="btn btn-ghost" onclick={() => (isOpen = false)}>
          Отмена
        </button>
        <button
          class="btn btn-neutral"
          disabled={!title.trim()}
          onclick={submit}
        >
          Создать
        </button>
      </div>
    </div>
    <button
      class="modal-backdrop"
      onclick={() => (isOpen = false)}
      aria-label="Закрыть"
    ></button>
  </div>
{/if}
