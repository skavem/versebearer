<script lang="ts">
  import { CoupletInput } from "$lib/bindings/changeme";
  import { ReplaceCouplets } from "$lib/bindings/changeme/dbhandler";
  import { parseSongText, serializeSongText } from "$lib/songText";
  import { songsStore } from "$lib/stores/songsStore.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  let {
    isModalOpen = $bindable(),
  }: {
    isModalOpen: boolean;
  } = $props();

  const songs = $derived(songsStore.songs);
  const couplets = $derived(songsStore.couplets);

  let value = $state("");

  $effect(() => {
    if (isModalOpen) {
      value = serializeSongText(couplets.list);
    }
  });

  const close = () => {
    isModalOpen = false;
  };

  const save = async () => {
    if (!songs.active) return;
    const blocks = parseSongText(value).map(
      (b) => new CoupletInput(b),
    );
    await ReplaceCouplets(songs.active.ID, blocks);
    close();
  };
</script>

<dialog class="modal" open={isModalOpen} onclose={close}>
  <div class="modal-box flex h-[85vh] max-h-none w-11/12 max-w-4xl flex-col">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-lg font-bold">
        Редактирование песни
        {#if songs.active}
          <span class="text-base-content/60 font-normal"
            >№{songs.active.number} «{songs.active.title}»</span
          >
        {/if}
      </h3>
      <button class="btn btn-sm btn-circle btn-ghost" onclick={close}>
        <MuiIcon name="close" />
      </button>
    </div>

    <p class="text-base-content/70 mb-2 text-sm">
      Куплеты разделяются пустой строкой. Первая строка каждого блока — название
      («Куплет 1», «Припев», «Бридж» и т. п.), далее идёт текст.
    </p>

    <textarea
      bind:value
      class="textarea textarea-bordered w-full flex-1 resize-none font-mono text-sm leading-relaxed"
      placeholder="Куплет 1&#10;Строка 1&#10;Строка 2&#10;&#10;Припев&#10;Строка припева"
      onkeydown={(e) => {
        if (e.code === "Escape") {
          close();
        }
        e.stopPropagation();
      }}
    ></textarea>

    <div class="modal-action mt-4">
      <button class="btn btn-ghost" onclick={close}>Отмена</button>
      <button class="btn btn-primary" onclick={save}>
        <MuiIcon name="save" />
        Сохранить
      </button>
    </div>
  </div>

  <form method="dialog" class="modal-backdrop">
    <button class="bg-black/70" onclick={close}>close</button>
  </form>
</dialog>
