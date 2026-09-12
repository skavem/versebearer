<script lang="ts">
  import type { AudioTrack } from "$lib/bindings/changeme/backend/models";
  import AudioDeviceSelect from "$lib/components/AudioDeviceSelect.svelte";
  import EditTrackModal from "$lib/components/EditTrackModal.svelte";
  import List from "$lib/components/List.svelte";
  import MiniPlayer from "$lib/components/MiniPlayer.svelte";
  import MuiIcon from "$lib/components/MuiIcon.svelte";
  import PlaylistPanel from "$lib/components/PlaylistPanel.svelte";
  import { isFromModal, isTypingTarget } from "$lib/keyboard";
  import { audioStore } from "$lib/stores/audioStore.svelte";

  const tracks = $derived(audioStore.tracks);

  let editingTrack = $state<AudioTrack | null>(null);
  let trackToDelete = $state<AudioTrack | null>(null);
  let importErrors = $state<string[]>([]);

  // subView переключает вкладку между медиатекой (этап 1) и плейлистами
  // (этап 4) — устройство вывода и ошибка общие для обеих, поэтому живут в
  // шапке выше переключателя, а не дублируются в каждой подвкладке.
  let subView = $state<"library" | "playlists">("library");

  // selectedPlaylistItemId — клавиатурный выбор строки плейлиста (план,
  // этап 6). Живёт здесь, а не в PlaylistPanel: клавиши разбираются в общем
  // document-level обработчике вкладки, ему нужен прямой доступ к значению.
  let selectedPlaylistItemId = $state<number | null>(null);
  $effect(() => {
    audioStore.playlists.active?.ID; // сброс выбора при смене плейлиста
    selectedPlaylistItemId = null;
  });

  function movePlaylistSelection(delta: number) {
    const playlist = audioStore.playlists.active;
    const items = playlist?.items ?? [];
    if (!playlist || items.length === 0) return;
    const idx = items.findIndex((i) => i.ID === selectedPlaylistItemId);
    const next =
      idx === -1
        ? delta > 0
          ? 0
          : items.length - 1
        : Math.min(items.length - 1, Math.max(0, idx + delta));
    selectedPlaylistItemId = items[next].ID;
  }

  function playSelectedPlaylistItem() {
    const playlist = audioStore.playlists.active;
    const item = (playlist?.items ?? []).find(
      (i) => i.ID === selectedPlaylistItemId,
    );
    if (!playlist || !item) return;
    audioStore.play(playlist.ID, item.ID);
  }

  function seekRelative(deltaMs: number) {
    const state = audioStore.player.state;
    if (!state || state.status === "idle") return;
    const next = Math.max(
      0,
      Math.min(state.durationMs, state.positionMs + deltaMs),
    );
    audioStore.seek(next);
  }

  function volumeStep(delta: number) {
    const current = audioStore.player.state?.volume ?? 1;
    audioStore.setVolume(Math.max(0, Math.min(1, current + delta)));
  }

  // Свой document-level обработчик, как Songs.svelte:65-98 — вкладки
  // взаимоисключающие (+page.svelte монтирует ровно одну), второго
  // постоянного слушателя не возникает, поэтому централизованный реестр не
  // нужен (план, этап 6). Разбор по e.code, не по e.key — на русской
  // раскладке key отдаёт символ раскладки (см. keyboard.ts).
  $effect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (isFromModal(e)) return;
      // Ползунки прогресса/громкости в MiniPlayer — это <input type=range>,
      // т.е. HTMLInputElement: isTypingTarget их тоже глушит. Они сами
      // делают blur() после change (см. MiniPlayer.svelte), так что этот
      // return не застревает на них дольше одного отпускания мыши.
      if (isTypingTarget(e)) return;

      switch (e.code) {
        case "ArrowUp":
          if (subView === "playlists") movePlaylistSelection(-1);
          e.preventDefault();
          return;
        case "ArrowDown":
          if (subView === "playlists") movePlaylistSelection(1);
          e.preventDefault();
          return;
        case "Enter":
          playSelectedPlaylistItem();
          e.preventDefault();
          return;
        case "Escape":
          audioStore.fadeOutStop();
          e.preventDefault();
          return;
        case "Space":
          audioStore.toggle();
          e.preventDefault(); // иначе Space заодно прокрутит список
          return;
        case "ArrowLeft":
          seekRelative(-5000);
          e.preventDefault();
          return;
        case "ArrowRight":
          seekRelative(5000);
          e.preventDefault();
          return;
        case "Equal":
        case "NumpadAdd":
          volumeStep(0.05);
          e.preventDefault();
          return;
        case "Minus":
        case "NumpadSubtract":
          volumeStep(-0.05);
          e.preventDefault();
          return;
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  });

  // Устройства/плейлисты запрашиваются лениво, только пока открыта вкладка
  // «Звук» — не при module-init audioStore (см. комментарий там):
  // ListDevices() поднимает malgo.InitContext, а это не должно случаться на
  // машине без звуковой карты раньше, чем оператор реально сюда зашёл.
  // Опрос PlayerState (startPolling) по той же причине не крутится, пока
  // вкладка не открыта.
  $effect(() => {
    audioStore.refreshDevices();
    audioStore.refreshPlaylists();
    audioStore.startPolling();
    return () => audioStore.stopPolling();
  });

  function formatDuration(ms: number): string {
    if (!ms || ms <= 0) return "—";
    const totalSec = Math.round(ms / 1000);
    const min = Math.floor(totalSec / 60);
    const sec = totalSec % 60;
    return `${min}:${sec.toString().padStart(2, "0")}`;
  }

  function formatSize(bytes: number): string {
    if (!bytes) return "";
    return `${(bytes / (1024 * 1024)).toFixed(1)} МБ`;
  }

  const doImport = async () => {
    const paths = await audioStore.pickFiles();
    if (paths.length === 0) return; // диалог закрыли — молча выходим
    importErrors = [];
    const results = await audioStore.importFiles(paths);
    importErrors = results.filter((r) => r.error).map((r) => r.error);
  };

  const confirmDelete = async () => {
    if (!trackToDelete) return;
    const id = trackToDelete.ID;
    trackToDelete = null;
    await audioStore.remove(id);
  };
</script>

<div class="flex h-[calc(100vh-4rem)] flex-col gap-2 p-4">
  <div class="flex items-center justify-between gap-2">
    <div class="tabs tabs-boxed tabs-sm w-fit">
      <button
        class="tab {subView === 'library' ? 'tab-active' : ''}"
        onclick={() => (subView = "library")}
      >
        Медиатека
      </button>
      <button
        class="tab {subView === 'playlists' ? 'tab-active' : ''}"
        onclick={() => (subView = "playlists")}
      >
        Плейлисты
      </button>
    </div>
    <AudioDeviceSelect />
  </div>

  {#if subView === "library"}
    <div class="flex items-center justify-between">
      <h2 class="text-lg font-semibold">Медиатека фонограмм</h2>

      {#if audioStore.importing}
        <div class="flex items-center gap-2">
          <progress
            class="progress progress-primary w-40"
            value={audioStore.importProgress?.done ?? 0}
            max={audioStore.importProgress?.total ?? 1}
          ></progress>
          <span class="text-xs opacity-70">
            {audioStore.importProgress?.done ?? 0} из {audioStore
              .importProgress?.total ?? 0}
          </span>
          <button
            class="btn btn-ghost btn-xs"
            onclick={() => audioStore.cancelImport()}
          >
            Отмена
          </button>
        </div>
      {:else}
        <button class="btn btn-outline btn-sm gap-1" onclick={doImport}>
          <MuiIcon name="add" style="font-size: 1.15rem" />
          Импорт
        </button>
      {/if}
    </div>
  {/if}

  {#if importErrors.length > 0}
    <div class="alert alert-error py-2 text-sm">
      Не удалось импортировать {importErrors.length}
      {importErrors.length === 1 ? "файл" : "файл(ов)"}: {importErrors[0]}
      {#if importErrors.length > 1}
        и ещё {importErrors.length - 1}…
      {/if}
    </div>
  {/if}

  {#if audioStore.error}
    <div class="alert alert-error py-2 text-sm">
      <span class="flex-1">{audioStore.error}</span>
      <button
        class="btn btn-ghost btn-xs"
        onclick={() => audioStore.clearError()}
        aria-label="Закрыть"
      >
        <MuiIcon name="close" style="font-size: 1rem" />
      </button>
    </div>
  {/if}

  {#if subView === "library"}
    <div class="min-h-0 flex-1">
      {#if tracks.loading}
        <div class="flex h-full items-center justify-center opacity-60">
          Загрузка…
        </div>
      {:else if tracks.list.length === 0}
        <div class="flex h-full items-center justify-center opacity-60">
          Пока нет фонограмм — нажмите «Импорт»
        </div>
      {:else}
        <List
          items={tracks.list}
          activeItem={tracks.active}
          getName={(t) => t.title || t.fileName}
          onClick={(t) => (tracks.active = t)}
        >
          {#snippet leftMark(t)}
            <span class="badge badge-neutral badge-sm font-mono"
              >{formatDuration(t.durationMs)}</span
            >
          {/snippet}
          {#snippet rightMark(t)}
            <div class="flex flex-row items-center gap-1">
              <span
                class="hidden text-xs opacity-50 sm:inline group-hover/item:hidden"
                >{formatSize(t.sizeBytes)}</span
              >
              <button
                class="btn btn-neutral btn-xs hidden px-1 text-white group-hover/item:block"
                onclick={(e) => {
                  editingTrack = t;
                  e.stopPropagation();
                }}
                title="Редактировать"
                ><MuiIcon name="edit" style="font-size: 1rem" /></button
              >
              <button
                class="btn btn-error btn-xs hidden px-1 text-white group-hover/item:block"
                onclick={(e) => {
                  trackToDelete = t;
                  e.stopPropagation();
                }}
                title="Удалить фонограмму"
                ><MuiIcon name="delete" style="font-size: 1rem" /></button
              >
            </div>
          {/snippet}
        </List>
      {/if}
    </div>
  {:else}
    <div class="min-h-0 flex-1">
      <PlaylistPanel bind:selectedItemId={selectedPlaylistItemId} />
    </div>
  {/if}

  <MiniPlayer />
</div>

<EditTrackModal bind:track={editingTrack} />

<svelte:window
  onkeydown={(e) =>
    trackToDelete && e.key === "Escape" && (trackToDelete = null)}
/>

{#if trackToDelete}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-2 flex items-center gap-3">
        <div
          class="flex h-10 w-10 items-center justify-center rounded-full bg-error/10 text-error"
        >
          <MuiIcon name="delete" />
        </div>
        <h3 class="text-lg font-bold">Удалить фонограмму?</h3>
      </div>

      <p class="py-2">
        <span class="font-semibold">«{trackToDelete.title}»</span> будет удалена
        безвозвратно вместе с файлом.
      </p>

      <div class="modal-action">
        <button class="btn btn-ghost" onclick={() => (trackToDelete = null)}>
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
      onclick={() => (trackToDelete = null)}
      aria-label="Закрыть"
    ></button>
  </div>
{/if}
