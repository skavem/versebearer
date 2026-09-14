<script lang="ts">
  // PlaylistSidebar — левая треть вкладки «Звук»: выбор плейлиста и всё, что
  // делается с самим плейлистом как со строкой списка (создать,
  // переименовать, удалить). Выделено из PlaylistPanel, который вырос до
  // двух независимых половин: здесь — какой плейлист активен, там — что
  // внутри активного. Общее у них ровно одно — audioStore.playlists, и оба
  // читают его напрямую из стора, так что пробрасывать между ними нечего.
  import type { Playlist } from "$lib/bindings/changeme/backend/models";
  import { audioStore } from "$lib/stores/audioStore.svelte";
  import ConfirmDeleteModal from "./ConfirmDeleteModal.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  const playlists = $derived(audioStore.playlists);

  let newPlaylistName = $state("");
  let playlistToDelete = $state<Playlist | null>(null);
  let renamingId = $state<number | null>(null);
  let renameValue = $state("");

  const createPlaylist = async () => {
    const name = newPlaylistName.trim();
    if (!name) return;
    newPlaylistName = "";
    await audioStore.createPlaylist(name);
  };

  const startRename = (p: Playlist) => {
    renamingId = p.ID;
    renameValue = p.name;
  };

  const commitRename = async () => {
    if (renamingId === null) return;
    const id = renamingId;
    const name = renameValue.trim();
    renamingId = null;
    if (name) await audioStore.renamePlaylist(id, name);
  };

  const confirmDeletePlaylist = async () => {
    if (!playlistToDelete) return;
    const id = playlistToDelete.ID;
    playlistToDelete = null;
    await audioStore.removePlaylist(id);
  };
</script>

<div class="flex w-1/3 flex-col gap-2 lg:w-1/4">
  <div class="flex gap-1">
    <input
      type="text"
      class="input input-bordered input-sm w-full"
      placeholder="Новый плейлист"
      bind:value={newPlaylistName}
      onkeydown={(e) => e.key === "Enter" && createPlaylist()}
    />
    <button
      class="btn btn-neutral btn-sm"
      disabled={!newPlaylistName.trim()}
      onclick={createPlaylist}
    >
      <MuiIcon name="add" style="font-size: 1.1rem" />
    </button>
  </div>

  <!-- Рамка списка (правка 5, третий раунд) — тот же визуальный язык, что
  у List.svelte:85-86 (вкладка «Песни»): border-base-300 + rounded-lg. Сам
  компонент List сюда не переиспользуем — у него своя виртуализация и
  модель данных, нужен только внешний вид. -->
  <div class="min-h-0 flex-1 overflow-y-auto rounded-lg border border-base-300">
    {#if playlists.loading}
      <div class="p-4 text-center opacity-60">Загрузка…</div>
    {:else if playlists.list.length === 0}
      <div class="p-4 text-center text-sm opacity-60">
        Пока нет плейлистов — создайте его выше, файлы импортируются сразу
        в него
      </div>
    {:else}
      <ul class="flex flex-col gap-1">
        {#each playlists.list as p (p.ID)}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
          <li
            class={[
              "group/item flex items-center gap-1 rounded border-2 p-2",
              p.ID === playlists.active?.ID
                ? "border-primary bg-primary/10"
                : "cursor-pointer border-transparent hover:bg-base-200",
            ]}
            onclick={() => (playlists.active = p)}
          >
            {#if renamingId === p.ID}
              <input
                type="text"
                class="input input-bordered input-xs flex-1"
                bind:value={renameValue}
                onclick={(e) => e.stopPropagation()}
                onkeydown={(e) => e.key === "Enter" && commitRename()}
                onblur={commitRename}
              />
            {:else}
              <span class="flex-1 overflow-hidden text-ellipsis whitespace-nowrap"
                >{p.name}</span
              >
              {#if p.autoAdvance}
                <MuiIcon
                  name="repeat"
                  style="font-size: 0.9rem"
                  classes="opacity-60"
                />
              {/if}
              <button
                class="btn btn-ghost btn-xs hidden px-1 group-hover/item:block"
                onclick={(e) => {
                  e.stopPropagation();
                  startRename(p);
                }}
                title="Переименовать"
                ><MuiIcon name="edit" style="font-size: 0.9rem" /></button
              >
              <button
                class="btn btn-ghost btn-xs hidden px-1 text-error group-hover/item:block"
                onclick={(e) => {
                  e.stopPropagation();
                  playlistToDelete = p;
                }}
                title="Удалить плейлист"
                ><MuiIcon name="delete" style="font-size: 0.9rem" /></button
              >
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>

{#if playlistToDelete}
  <ConfirmDeleteModal
    title="Удалить плейлист?"
    onConfirm={confirmDeletePlaylist}
    onCancel={() => (playlistToDelete = null)}
  >
    <span class="font-semibold">«{playlistToDelete.name}»</span> будет удалён
    безвозвратно. Фонограммы, которые не используются ни в одном другом
    плейлисте, будут удалены вместе с файлами.
    {#if audioStore.isPlaylistOnAir(playlistToDelete.ID)}
      <br /><span class="font-semibold text-error"
        >Этот плейлист сейчас в эфире — воспроизведение остановится.</span
      >
    {/if}
  </ConfirmDeleteModal>
{/if}
