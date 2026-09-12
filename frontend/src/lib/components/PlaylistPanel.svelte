<script lang="ts">
  import type {
    Playlist,
    PlaylistItem,
  } from "$lib/bindings/changeme/backend/models";
  import { audioStore } from "$lib/stores/audioStore.svelte";
  import { formatDuration } from "$lib/timeFormat";
  import ConfirmDeleteModal from "./ConfirmDeleteModal.svelte";
  import MuiIcon from "./MuiIcon.svelte";
  import Select from "./Select.svelte";

  // selectedItemId — подсветка для клавиатурной навигации вкладки «Звук»
  // (план, этап 6: ArrowUp/ArrowDown ходят по списку плейлиста, Enter играет
  // выбранный). Отдельно от isPlayingItem: выбор клавиатурой и то, что
  // сейчас реально звучит, — разные вещи, поэтому и подсветки разные
  // (синий = выбор, янтарь = в эфире).
  //
  // modalOpen — наружу для Audio.svelte: её собственный document-level
  // обработчик клавиш обязан знать, открыта ли модалка удаления плейлиста
  // (ниже), чтобы не пропустить Escape дальше на fadeOutStop() (ревью:
  // isFromModal(e) по e.target ненадёжен — фокус при открытии модалки
  // остаётся на кнопке в списке, вне .modal).
  let {
    selectedItemId = $bindable(null),
    modalOpen = $bindable(false),
  }: { selectedItemId?: number | null; modalOpen?: boolean } = $props();

  const playlists = $derived(audioStore.playlists);
  const tracks = $derived(audioStore.tracks);
  const player = $derived(audioStore.player.state);

  let newPlaylistName = $state("");
  let playlistToDelete = $state<Playlist | null>(null);
  let renamingId = $state<number | null>(null);
  let renameValue = $state("");

  $effect(() => {
    modalOpen = playlistToDelete !== null;
  });

  // Трек для добавления, выбранный через Select — сбрасывается после
  // каждого добавления, чтобы не пришлось руками очищать поле.
  let trackToAdd = $state<(typeof tracks.list)[number] | null>(null);

  // dragItemId — id элемента, который сейчас тащат (нативный HTML5
  // drag-and-drop, без библиотек — план, этап 4).
  let dragItemId = $state<number | null>(null);

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

  const confirmDelete = async () => {
    if (!playlistToDelete) return;
    const id = playlistToDelete.ID;
    playlistToDelete = null;
    await audioStore.removePlaylist(id);
  };

  const addTrack = async () => {
    const playlist = playlists.active;
    if (!playlist || !trackToAdd) return;
    const trackId = trackToAdd.ID;
    trackToAdd = null;
    await audioStore.addToPlaylist(playlist.ID, trackId);
  };

  // isPlayingItem — «этот элемент сейчас загружен в плеер», включая паузу:
  // кнопка на строке тогда переключает паузу, а не стартует трек заново.
  function isPlayingItem(item: PlaylistItem): boolean {
    return !!player && player.status !== "idle" && player.itemId === item.ID;
  }

  // Янтарь = «в эфире» (загруженный трек), синий = клавиатурный выбор —
  // оператору не спутать «что звучит» с «на чём сейчас стоит курсор».
  function itemHighlight(item: PlaylistItem): string {
    if (isPlayingItem(item)) return "border-secondary bg-secondary/10";
    if (selectedItemId === item.ID) return "border-primary bg-primary/5";
    return "border-transparent hover:bg-base-200";
  }

  async function playItem(playlist: Playlist, item: PlaylistItem) {
    if (isPlayingItem(item)) {
      await audioStore.toggle();
      return;
    }
    await audioStore.play(playlist.ID, item.ID);
  }

  function onDragStart(item: PlaylistItem) {
    dragItemId = item.ID;
  }

  function onDragOver(e: DragEvent) {
    e.preventDefault(); // разрешить drop — иначе браузер его отклоняет
  }

  async function onDrop(playlist: Playlist, target: PlaylistItem) {
    const draggedId = dragItemId;
    dragItemId = null;
    if (draggedId === null || draggedId === target.ID) return;
    const items = playlist.items ?? [];
    const order = items.map((i) => i.ID);
    const from = order.indexOf(draggedId);
    const to = order.indexOf(target.ID);
    if (from === -1 || to === -1) return;
    order.splice(to, 0, ...order.splice(from, 1));
    await audioStore.reorderPlaylist(playlist.ID, order);
  }
</script>

<div class="flex h-full flex-row gap-2">
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

    <div class="min-h-0 flex-1 overflow-y-auto">
      {#if playlists.loading}
        <div class="p-4 text-center opacity-60">Загрузка…</div>
      {:else if playlists.list.length === 0}
        <div class="p-4 text-center opacity-60">Пока нет плейлистов</div>
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

  <div class="flex min-w-0 flex-1 flex-col gap-2">
    {#if !playlists.active}
      <div class="flex h-full items-center justify-center opacity-60">
        Выберите плейлист слева
      </div>
    {:else}
      {@const playlist = playlists.active}
      <div class="flex items-center justify-between gap-2">
        <h2 class="truncate text-lg font-semibold">{playlist.name}</h2>
        <div class="flex items-center gap-3 text-sm">
          <label class="flex cursor-pointer items-center gap-1">
            <input
              type="checkbox"
              class="toggle toggle-sm"
              checked={playlist.autoAdvance}
              onchange={(e) =>
                audioStore.setPlaylistFlags(playlist.ID, {
                  autoAdvance: e.currentTarget.checked,
                })}
            />
            Автопереход
          </label>
          <label class="flex cursor-pointer items-center gap-1">
            <input
              type="checkbox"
              class="toggle toggle-sm"
              checked={playlist.loop}
              onchange={(e) =>
                audioStore.setPlaylistFlags(playlist.ID, {
                  loop: e.currentTarget.checked,
                })}
            />
            По кругу
          </label>
          <label class="flex items-center gap-1">
            Фейд
            <input
              type="number"
              min="0"
              step="50"
              value={playlist.fadeMs}
              title="Длительность плавного перехода на границе треков, мс — 0 отключает фейд"
              onchange={(e) =>
                audioStore.setPlaylistFlags(playlist.ID, {
                  fadeMs: Math.max(0, Number(e.currentTarget.value) || 0),
                })}
              class="input input-bordered input-xs w-16"
            />
            мс
          </label>
        </div>
      </div>

      <div class="flex gap-1">
        <div class="flex-1">
          <Select
            items={tracks.list}
            activeItem={trackToAdd}
            setActiveItem={(t) => (trackToAdd = t)}
            getName={(t) => t.title || t.fileName}
          />
        </div>
        <button
          class="btn btn-outline btn-sm"
          disabled={!trackToAdd}
          onclick={addTrack}
        >
          <MuiIcon name="playlist_add" style="font-size: 1.1rem" />
          Добавить
        </button>
      </div>

      <div class="min-h-0 flex-1 overflow-y-auto">
        {#if (playlist.items ?? []).length === 0}
          <div class="flex h-full items-center justify-center opacity-60">
            Плейлист пуст — добавьте фонограммы выше
          </div>
        {:else}
          <ul class="flex flex-col gap-1">
            {#each playlist.items ?? [] as item (item.ID)}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
              <li
                class={[
                  "group/item flex items-center gap-2 rounded border-2 p-2",
                  itemHighlight(item),
                ]}
                onclick={() => (selectedItemId = item.ID)}
                ondragover={onDragOver}
                ondrop={() => onDrop(playlist, item)}
              >
                <!-- Ручка перетаскивания — ТОЛЬКО эта иконка, не вся строка
                (ревью): draggable на всём <li> означал промах мышью в
                несколько пикселей = случайный drag, а перетаскивание/
                удаление играющего элемента на бэке делает ЖЁСТКИЙ Stop() без
                фейда. ondragend — не только onDrop — иначе отменённый
                (сброшенный мимо валидной цели) drag оставлял dragItemId
                залипшим до следующего перетаскивания. -->
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <span
                  draggable="true"
                  ondragstart={() => onDragStart(item)}
                  ondragend={() => (dragItemId = null)}
                  style="cursor: grab;"
                >
                  <MuiIcon
                    name="drag_indicator"
                    style="font-size: 1rem"
                    classes="opacity-40"
                  />
                </span>
                <span class="w-6 text-right font-mono text-xs opacity-60"
                  >{item.position}</span
                >
                <button
                  class="btn btn-ghost btn-xs px-1"
                  onclick={() => playItem(playlist, item)}
                  title={isPlayingItem(item) && player?.status === "playing"
                    ? "Пауза"
                    : "Играть"}
                >
                  <MuiIcon
                    name={isPlayingItem(item) && player?.status === "playing"
                      ? "pause"
                      : "play_arrow"}
                    style="font-size: 1.1rem"
                  />
                </button>
                <span
                  class="flex-1 overflow-hidden text-ellipsis whitespace-nowrap"
                  >{item.track?.title || item.track?.fileName}</span
                >
                <span class="badge badge-neutral badge-sm font-mono"
                  >{formatDuration(item.track?.durationMs ?? 0)}</span
                >
                <button
                  class="btn btn-ghost btn-xs hidden px-1 text-error group-hover/item:block"
                  onclick={() => audioStore.removeFromPlaylist(item.ID)}
                  title="Убрать из плейлиста"
                  ><MuiIcon name="close" style="font-size: 0.9rem" /></button
                >
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {/if}
  </div>
</div>

{#if playlistToDelete}
  <ConfirmDeleteModal
    title="Удалить плейлист?"
    onConfirm={confirmDelete}
    onCancel={() => (playlistToDelete = null)}
  >
    <span class="font-semibold">«{playlistToDelete.name}»</span> будет удалён
    безвозвратно вместе со списком треков (сами фонограммы останутся в медиатеке).
  </ConfirmDeleteModal>
{/if}
