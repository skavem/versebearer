<script lang="ts">
  import AudioDeviceSelect from "$lib/components/AudioDeviceSelect.svelte";
  import MiniPlayer from "$lib/components/MiniPlayer.svelte";
  import MuiIcon from "$lib/components/MuiIcon.svelte";
  import PlaylistPanel from "$lib/components/PlaylistPanel.svelte";
  import { isTypingTarget } from "$lib/keyboard";
  import { audioStore } from "$lib/stores/audioStore.svelte";

  // selectedPlaylistItemId — клавиатурный выбор строки плейлиста (план,
  // этап 6). Живёт здесь, а не в PlaylistPanel: клавиши разбираются в общем
  // document-level обработчике вкладки, ему нужен прямой доступ к значению.
  let selectedPlaylistItemId = $state<number | null>(null);

  // previousPlaylistId/previousItems — обычные (не $state) замыкания:
  // эффект ниже сравнивает их с текущим снимком САМ, вручную, примитивами —
  // а не полагается на реактивность Svelte по ссылке. audioStore.playlists.list
  // пересоздаёт и массив, и каждый Playlist/PlaylistItem в нём новыми
  // объектами на КАЖДЫЙ audio_playlists_update, даже когда содержимое не
  // изменилось (перестановка соседнего элемента, правка чужого трека и
  // т.п.) — эффект, зависящий от самого объекта playlists.active, срабатывал
  // бы на каждое такое событие и сбрасывал выбор оператора вслепую.
  let previousPlaylistId: number | null = null;
  let previousItems: { ID: number }[] = [];
  $effect(() => {
    const playlist = audioStore.playlists.active;
    const items = playlist?.items ?? [];
    const playlistId = playlist?.ID ?? null;

    if (playlistId !== previousPlaylistId) {
      // Сменился сам активный плейлист — выбор целиком теряет смысл.
      selectedPlaylistItemId = null;
    } else if (
      selectedPlaylistItemId !== null &&
      !items.some((i) => i.ID === selectedPlaylistItemId)
    ) {
      // Тот же плейлист, но выбранный элемент исчез (убрали из плейлиста,
      // трек удалили и т.п.) — курсор переезжает на соседа по прежней
      // позиции, а не обнуляется целиком: оператор жмёт ArrowDown/Enter
      // подряд, и полный сброс заставлял бы каждый раз заново заходить в
      // список с края.
      const oldIndex = previousItems.findIndex(
        (i) => i.ID === selectedPlaylistItemId,
      );
      selectedPlaylistItemId =
        items.length === 0 || oldIndex === -1
          ? null
          : items[Math.min(oldIndex, items.length - 1)].ID;
    }

    previousPlaylistId = playlistId;
    previousItems = items;
  });

  function movePlaylistSelection(delta: number) {
    const playlist = audioStore.playlists.active;
    const items = playlist?.items ?? [];
    if (!playlist || items.length === 0) return;
    const idx = items.findIndex((i) => i.ID === selectedPlaylistItemId);
    // Ничего ещё не выбрано — заходим в список с того края, откуда идём:
    // вниз с первого элемента, вверх с последнего.
    let next: number;
    if (idx === -1) {
      next = delta > 0 ? 0 : items.length - 1;
    } else {
      next = Math.min(items.length - 1, Math.max(0, idx + delta));
    }
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
      // Источник истины "открыта ли модалка" — DOM, а не проброшенный
      // булев проп: у вкладки их теперь три источника (удаление/очистка
      // плейлиста, правка трека — все внутри PlaylistPanel, плюс выбор
      // устройства — AudioDeviceSelect), и плодить bindable на каждую
      // хрупко. Все они рендерят daisyUI `.modal.modal-open`.
      if (document.querySelector(".modal.modal-open")) return;
      // Ползунки прогресса/громкости в MiniPlayer — это <input type=range>,
      // т.е. HTMLInputElement: isTypingTarget их тоже глушит. Они сами
      // делают blur() после change (см. MiniPlayer.svelte), так что этот
      // return не застревает на них дольше одного отпускания мыши.
      if (isTypingTarget(e)) return;

      switch (e.code) {
        case "ArrowUp":
          movePlaylistSelection(-1);
          e.preventDefault();
          return;
        case "ArrowDown":
          movePlaylistSelection(1);
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
          // playOrToggleSelected, не голый toggle() (правка 4, второй
          // раунд): та же логика, что теперь у кнопки play/pause в
          // MiniPlayer, — иначе на простое нажатие Space, когда ничего не
          // загружено, но трек выделен стрелками, оно бы молча не
          // срабатывало (Toggle() на бэке без активного трека — ошибка).
          audioStore.playOrToggleSelected(selectedPlaylistItemId);
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
</script>

<div class="flex h-full flex-col gap-2 p-4">
  <div class="flex items-center justify-between gap-2">
    <h2 class="text-lg font-semibold">Звук</h2>
    <AudioDeviceSelect />
  </div>

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

  <div class="min-h-0 flex-1">
    <PlaylistPanel bind:selectedItemId={selectedPlaylistItemId} />
  </div>

  <MiniPlayer selectedItemId={selectedPlaylistItemId} />
</div>
