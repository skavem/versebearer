<script lang="ts">
  // PlaylistPanel — содержимое АКТИВНОГО плейлиста: его флаги, список
  // элементов, импорт, перестановка. Выбор самого плейлиста (создать/
  // переименовать/удалить) — в PlaylistSidebar слева; общее у них только
  // audioStore.playlists, который оба читают напрямую из стора.
  import type {
    AudioTrack,
    Playlist,
    PlaylistItem,
  } from "$lib/bindings/changeme/backend/models";
  import { audioStore } from "$lib/stores/audioStore.svelte";
  import { formatDuration } from "$lib/timeFormat";
  import ConfirmDeleteModal from "./ConfirmDeleteModal.svelte";
  import EditTrackModal from "./EditTrackModal.svelte";
  import MuiIcon from "./MuiIcon.svelte";
  import PlaylistSidebar from "./PlaylistSidebar.svelte";

  // selectedItemId — подсветка для клавиатурной навигации вкладки «Звук»
  // (план, этап 6: ArrowUp/ArrowDown ходят по списку плейлиста, Enter играет
  // выбранный). Отдельно от isPlayingItem: выбор клавиатурой и то, что
  // сейчас реально звучит, — разные вещи, поэтому и подсветки разные
  // (синий = выбор, янтарь = в эфире).
  //
  // Ни одна модалка вкладки (очистка плейлиста, правка трека, удаление
  // плейлиста в PlaylistSidebar, выбор устройства в AudioDeviceSelect)
  // наружу отдельным bindable НЕ пробрасывается: держать в Audio.svelte
  // отдельный bool на каждую из четырёх стало хрупко — реальный источник
  // истины "открыта ли хоть одна модалка" — DOM (`.modal.modal-open`), его
  // и проверяет document-level обработчик клавиш вкладки напрямую.
  let {
    selectedItemId = $bindable(null),
  }: { selectedItemId?: number | null } = $props();

  const playlists = $derived(audioStore.playlists);
  const tracks = $derived(audioStore.tracks);
  const player = $derived(audioStore.player.state);

  // playlistToClear — «Удалить всё» (правка 1): очищает список плейлиста и,
  // как одиночное «Убрать из плейлиста», удаляет с диска треки, которые
  // перестали использоваться в любом плейлисте — предупреждение об этом
  // ключевое, поэтому подтверждение обязательно (как у удаления плейлиста
  // целиком), в отличие от одиночного «Убрать из плейлиста» (× на строке),
  // который остаётся без модалки — как и раньше.
  let playlistToClear = $state<Playlist | null>(null);
  // editingTrack — правка трека (название/trim/gain) теперь открывается
  // прямо со строки плейлиста: отдельного экрана медиатеки больше нет.
  let editingTrack = $state<AudioTrack | null>(null);

  const confirmClearPlaylist = async () => {
    if (!playlistToClear) return;
    const id = playlistToClear.ID;
    playlistToClear = null;
    await audioStore.clearPlaylist(id);
  };

  // doImport — «Импорт» (правка 1): выбранные файлы импортируются и сразу
  // добавляются в АКТИВНЫЙ плейлист, одним потоком с прогрессом/отменой
  // (audioStore.importing/importProgress, показаны ниже). Импортировать
  // некуда без активного плейлиста — кнопка недоступна, пока его нет (см.
  // разметку).
  const doImport = async () => {
    const playlist = playlists.active;
    if (!playlist) return;
    const paths = await audioStore.pickFiles();
    if (paths.length === 0) return; // диалог закрыли — молча выходим
    await audioStore.importFilesToPlaylist(paths, playlist.ID);
  };

  // trackFor — правка 1: строка плейлиста показывает ЖИВЫЕ данные трека
  // (tracks.list, обновляется по audio_tracks_update), а не снимок
  // item.track из последнего ListPlaylists — иначе правка названия/trim
  // через EditTrackModal не отражалась бы в списке плейлиста без лишнего
  // audio_playlists_update.
  function trackFor(item: PlaylistItem): AudioTrack | undefined {
    return tracks.list.find((t) => t.ID === item.trackId) ?? item.track;
  }

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

  // itemToRemove — × на строке плейлиста удаляет БЕЗ подтверждения молча,
  // если убираемый элемент никак не привязан к звуку прямо сейчас — но
  // раньше это было верно ВСЕГДА, включая играющий элемент и последнюю
  // ссылку на файл (после чего файл реально стирается с диска, см.
  // deleteTrackIfUnused). Обе ситуации теперь требуют подтверждения.
  let itemToRemove = $state<PlaylistItem | null>(null);

  // referenceCount — сколько элементов ВО ВСЕХ плейлистах ссылаются на этот
  // trackId (дедупликация по хешу — один трек, несколько PlaylistItem).
  // Ровно та же величина, что бэк проверяет в deleteTrackIfUnused перед
  // удалением файла — <=1 значит "это последняя ссылка, файл будет стёрт".
  function referenceCount(trackId: number): number {
    return playlists.list.reduce(
      (n, pl) => n + (pl.items ?? []).filter((i) => i.trackId === trackId).length,
      0,
    );
  }

  function requestRemoveItem(item: PlaylistItem) {
    if (isPlayingItem(item) || referenceCount(item.trackId) <= 1) {
      itemToRemove = item;
      return;
    }
    audioStore.removeFromPlaylist(item.ID);
  }

  function confirmRemoveItem() {
    if (!itemToRemove) return;
    const id = itemToRemove.ID;
    itemToRemove = null;
    audioStore.removeFromPlaylist(id);
  }

  // moveSelectedItem — перестановка ОДНИМ комплектом кнопок справа от
  // списка (правка 2, второй раунд), по образцу CoupletsList.svelte:75-115:
  // там одна колонка кнопок сбоку двигает активный куплет, а не свой набор
  // на каждой строке. Действует на selectedItemId — то же понятие
  // "выбранного", которым управляют ArrowUp/ArrowDown на вкладке
  // (Audio.svelte) — второй source of truth не заводим. Бэкенд остаётся
  // НАШ: один транзакционный ReorderPlaylist(ids) со всем порядком, а не
  // два отдельных UpdateCouplet, как у куплетов.
  function moveSelectedItem(playlist: Playlist, delta: number) {
    const items = playlist.items ?? [];
    const index = items.findIndex((i) => i.ID === selectedItemId);
    const target = index + delta;
    if (index === -1 || target < 0 || target >= items.length) return;
    const order = items.map((i) => i.ID);
    [order[index], order[target]] = [order[target], order[index]];
    audioStore.reorderPlaylist(playlist.ID, order);
  }

  // Верхняя граница поля «Фейд» — правка 6: подобрана на глаз, длиннее
  // разумного перехода между треками не бывает. Нижняя граница держится на
  // нуле (не выше) намеренно: 0 — осмысленное значение, полностью
  // отключающее фейд (FadeOutStop в audio_service.go: FadeMs<=0 сводится к
  // обычному Stop() без effects.Transition — при нулевой длине она бы дала
  // NaN).
  const MAX_FADE_MS = 10_000;

  function setFadeMs(playlist: Playlist, ms: number) {
    const clamped = Math.min(MAX_FADE_MS, Math.max(0, Math.round(ms) || 0));
    audioStore.setPlaylistFlags(playlist.ID, { fadeMs: clamped });
  }
</script>

<div class="flex h-full flex-row gap-2">
  <PlaylistSidebar />

  <div class="flex min-w-0 flex-1 flex-col gap-2">
    {#if !playlists.active}
      <div class="flex h-full items-center justify-center opacity-60">
        Выберите плейлист слева
      </div>
    {:else}
      {@const playlist = playlists.active}
      {@const selectedIndex = (playlist.items ?? []).findIndex(
        (i) => i.ID === selectedItemId,
      )}
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

          <!-- Разделитель: флажки задают правила перехода, фейд — его звучание.
          Без него все три контрола читаются одним рядом, и «Фейд» выглядит
          третьим переключателем. -->
          <div class="h-5 w-px bg-base-300"></div>

          <label class="flex items-center gap-1">
            Фейд
            <!-- Свои −50/+50 вместо нативных спиннеров (правка 6): те были
            мелкие и плохо целились мышью. join — обычная daisyUI склейка
            кнопка+инпут+кнопка. -->
            <div class="join">
              <button
                type="button"
                class="btn btn-xs join-item"
                onclick={() => setFadeMs(playlist, playlist.fadeMs - 50)}
                title="Уменьшить на 50 мс"
                aria-label="Уменьшить фейд на 50 мс"
              >
                −50
              </button>
              <input
                type="number"
                min="0"
                max={MAX_FADE_MS}
                step="50"
                value={playlist.fadeMs}
                title="Длительность плавного перехода на границе треков, мс — 0 полностью отключает фейд"
                onchange={(e) => setFadeMs(playlist, Number(e.currentTarget.value))}
                class="fade-ms-input input input-bordered input-xs join-item w-14 text-center"
              />
              <button
                type="button"
                class="btn btn-xs join-item"
                onclick={() => setFadeMs(playlist, playlist.fadeMs + 50)}
                title="Увеличить на 50 мс"
                aria-label="Увеличить фейд на 50 мс"
              >
                +50
              </button>
            </div>
            мс
          </label>
        </div>
      </div>

      <div class="flex items-center justify-end gap-1">
        <button
          class="btn btn-ghost btn-sm text-error"
          disabled={(playlist.items ?? []).length === 0}
          onclick={() => (playlistToClear = playlist)}
          title="Удалить все элементы плейлиста вместе с файлами, которые нигде больше не используются"
        >
          <MuiIcon name="delete_sweep" style="font-size: 1.1rem" />
          Удалить всё
        </button>
      </div>

      <!-- flex-row: список + один комплект кнопок перестановки справа
      (правка 2, второй раунд) — приём из CoupletsList.svelte:69-144
      (список + `<div class="flex h-full">` с колонкой кнопок сбоку). -->
      <div class="flex min-h-0 flex-1 flex-row gap-2">
        <!-- Зона приёма файлов из системы (правка 2 первого раунда):
        data-file-drop-target порождает событие FilesDropped (main.go),
        Wails сам подсвечивает этот элемент классом .file-drop-target-active,
        пока оператор тащит файлы над ним — своей JS-логики наведения не
        нужно (см. CSS ниже). relative — кнопка импорта (ниже) плавает
        поверх списка абсолютным позиционированием и должна отсчитываться от
        этого контейнера, а не от всей панели; min-h-0/overflow-y-auto
        самого контейнера от этого не меняются, так что прокрутка списка и
        ловушка min-height:auto во вложенном flex остаются как были. -->
        <div
          class="drop-zone relative min-h-0 flex-1 overflow-y-auto rounded-lg border border-base-300"
          data-file-drop-target
        >
          {#if (playlist.items ?? []).length === 0}
            <div class="flex h-full flex-col items-center justify-center gap-1 opacity-60">
              <span>Плейлист пуст — нажмите «Импорт» или перетащите файлы сюда</span>
            </div>
          {:else}
            <!-- pb-16 — запас под плавающую кнопку импорта в правом нижнем
            углу (ниже), чтобы она не перекрывала последнюю строку списка. -->
            <ul class="flex flex-col gap-1 pb-16">
              {#each playlist.items ?? [] as item (item.ID)}
                {@const track = trackFor(item)}
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <li
                  class={[
                    "group/item flex items-center gap-1 rounded border-2 p-2",
                    itemHighlight(item),
                  ]}
                  onclick={() => (selectedItemId = item.ID)}
                  ondblclick={() => playItem(playlist, item)}
                >
                  <span class="w-6 text-right font-mono text-xs opacity-60"
                    >{item.position}</span
                  >
                  <!-- Кнопка play строки (правка 4, третий раунд): паддинги
                  уменьшены (px-3 → px-1, был перебор в прошлый раунд) и
                  добавлен внешний mx-1, чтобы не липнуть к соседям.
                  leading-none на иконке — убирает высоту строки шрифта
                  material-icons сверх самого глифа, из-за которой кнопка
                  внутри items-center строки казалась сдвинутой по
                  вертикали. -->
                  <button
                    class="btn btn-ghost btn-xs mx-1 px-1"
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
                      classes="leading-none"
                    />
                  </button>
                  <span
                    class="flex-1 overflow-hidden text-ellipsis whitespace-nowrap"
                    >{track?.title || track?.fileName}</span
                  >
                  <span class="badge badge-neutral badge-sm font-mono"
                    >{formatDuration(track?.durationMs ?? 0)}</span
                  >
                  <button
                    class="btn btn-ghost btn-xs hidden px-1 group-hover/item:block"
                    onclick={(e) => {
                      e.stopPropagation();
                      if (track) editingTrack = track;
                    }}
                    title="Редактировать фонограмму"
                    ><MuiIcon name="edit" style="font-size: 0.9rem" /></button
                  >
                  <button
                    class="btn btn-ghost btn-xs hidden px-1 text-error group-hover/item:block"
                    onclick={(e) => {
                      e.stopPropagation();
                      requestRemoveItem(item);
                    }}
                    title="Убрать из плейлиста (файл удалится, если нигде больше не используется)"
                    ><MuiIcon name="close" style="font-size: 0.9rem" /></button
                  >
                </li>
              {/each}
            </ul>
          {/if}

          <!-- Импорт (правка 3 первого раунда): круглая кнопка с плюсом без
          подписи, плавает в правом нижнем углу списка поверх содержимого —
          раньше это была текстовая кнопка «Импорт» в панели сверху. title
          оставлен: без него назначение кнопки без подписи не считать. На
          время импорта кнопка уступает место прогрессу/отмене (та же
          позиция), чтобы не плодить второе место в разметке под них. -->
          <div class="absolute bottom-3 right-3">
            {#if audioStore.importing}
              <div
                class="flex items-center gap-2 rounded-full border border-base-300 bg-base-100 px-3 py-1.5 shadow-lg"
              >
                <progress
                  class="progress progress-primary w-20"
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
              <!-- Аутлайн вместо сплошной заливки (правка 3, третий раунд):
              btn-primary тянул на себя внимание сильнее, чем нужно для
              вспомогательного плавающего действия. -->
              <button
                class="btn btn-circle btn-outline btn-primary shadow-lg"
                onclick={doImport}
                title="Импортировать фонограммы с диска"
                aria-label="Импортировать фонограммы с диска"
              >
                <MuiIcon name="add" style="font-size: 1.4rem" />
              </button>
            {/if}
          </div>
        </div>

        <!-- Перестановка (правка 2, второй раунд) — один комплект кнопок
        справа от списка вместо кнопок у каждой строки, двигает
        selectedItemId. disabled, когда ничего не выбрано или выбранный
        элемент уже на краю списка. gap-1 вместо gap-2 (правка «заодно»,
        третий раунд) — визуально привязывает кнопки вплотную к рамке
        списка (появилась в правке 5), а не вешает их в пустоте посреди
        панели; my-auto по-прежнему центрирует блок по высоте самой рамки
        (h-full = высота списка, не всей вкладки). -->
        <div class="flex h-full gap-1">
          <div class="my-auto flex flex-col gap-2">
            <button
              class="btn btn-neutral btn-sm btn-square"
              disabled={selectedIndex <= 0}
              onclick={() => moveSelectedItem(playlist, -1)}
              title="Переместить выбранное вверх"
            >
              <MuiIcon name="arrow_upward" />
            </button>
            <button
              class="btn btn-neutral btn-sm btn-square"
              disabled={selectedIndex === -1 ||
                selectedIndex === (playlist.items ?? []).length - 1}
              onclick={() => moveSelectedItem(playlist, 1)}
              title="Переместить выбранное вниз"
            >
              <MuiIcon name="arrow_downward" />
            </button>
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>

<EditTrackModal bind:track={editingTrack} />

{#if playlistToClear}
  <ConfirmDeleteModal
    title="Удалить все элементы плейлиста?"
    onConfirm={confirmClearPlaylist}
    onCancel={() => (playlistToClear = null)}
  >
    Список плейлиста <span class="font-semibold">«{playlistToClear.name}»</span>
    будет очищен. Фонограммы, которые не используются ни в одном другом
    плейлисте, будут удалены вместе с файлами с диска.
    {#if audioStore.isPlaylistOnAir(playlistToClear.ID)}
      <br /><span class="font-semibold text-error"
        >Этот плейлист сейчас в эфире — воспроизведение остановится.</span
      >
    {/if}
  </ConfirmDeleteModal>
{/if}

{#if itemToRemove}
  {@const track = trackFor(itemToRemove)}
  {@const onAir = isPlayingItem(itemToRemove)}
  {@const lastRef = referenceCount(itemToRemove.trackId) <= 1}
  <ConfirmDeleteModal
    title="Убрать из плейлиста?"
    onConfirm={confirmRemoveItem}
    onCancel={() => (itemToRemove = null)}
  >
    <span class="font-semibold">«{track?.title || track?.fileName}»</span> будет
    убран из плейлиста.
    {#if lastRef}
      Это последняя ссылка на файл — он будет удалён с диска.
    {/if}
    {#if onAir}
      <br /><span class="font-semibold text-error"
        >Трек сейчас в эфире{lastRef
          ? " — воспроизведение остановится"
          : " — доиграет до конца, но без автоперехода"}.</span
      >
    {/if}
  </ConfirmDeleteModal>
{/if}

<style>
  /* Wails добавляет этот класс элементу с data-file-drop-target, пока
  оператор тащит файлы над окном (правка 2) — своей JS-логики
  dragenter/dragleave не нужно, только явная подсветка цели. */
  .drop-zone {
    outline: 2px dashed transparent;
    outline-offset: -2px;
    transition: outline-color 0.15s ease;
  }
  :global(.drop-zone.file-drop-target-active) {
    /* daisyUI v4 хранит цвета темы как HSL-триплет в --p (см.
    tailwind.config.js) — не raw hex и не Tailwind v4 --color-*. */
    outline-color: hsl(var(--p));
    background-color: hsl(var(--p) / 0.08);
  }

  /* Поле «Фейд» (правка 6): свои −50/+50 кнопки рядом, нативные спиннеры
  убраны — они были мелкими и дублировали смысл кнопок. */
  .fade-ms-input {
    appearance: none;
    -moz-appearance: textfield;
  }
  .fade-ms-input::-webkit-outer-spin-button,
  .fade-ms-input::-webkit-inner-spin-button {
    -webkit-appearance: none;
    margin: 0;
  }
</style>
