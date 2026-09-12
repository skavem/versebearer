<script lang="ts">
  import type { Snippet } from "svelte";
  import MuiIcon from "./MuiIcon.svelte";

  // Подтверждение удаления — одна разметка на фонограмму (Audio.svelte) и
  // плейлист (PlaylistPanel.svelte): раньше это были две почти дословные
  // копии модалки вместе с собственным обработчиком Escape, и правка в одной
  // легко расходилась с другой.
  //
  // Компонент рендерится вызывающим под {#if}, поэтому свой `svelte:window`
  // он ставит только пока открыт. Escape закрывает модалку тем же onCancel,
  // а вкладка «Звук» в это время глушит свою карту клавиш по признаку
  // «открыта хоть одна модалка» (anyModalOpen в Audio.svelte) — иначе тот же
  // Escape заодно остановил бы воспроизведение.
  let {
    title,
    onConfirm,
    onCancel,
    children,
  }: {
    title: string;
    onConfirm: () => void;
    onCancel: () => void;
    children: Snippet;
  } = $props();
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && onCancel()} />

<div class="modal modal-open">
  <div class="modal-box">
    <div class="mb-2 flex items-center gap-3">
      <div
        class="flex h-10 w-10 items-center justify-center rounded-full bg-error/10 text-error"
      >
        <MuiIcon name="delete" />
      </div>
      <h3 class="text-lg font-bold">{title}</h3>
    </div>

    <p class="py-2">{@render children()}</p>

    <div class="modal-action">
      <button class="btn btn-ghost" onclick={onCancel}>Отмена</button>
      <button class="btn btn-error" onclick={onConfirm}>
        <MuiIcon name="delete" />
        Удалить
      </button>
    </div>
  </div>
  <button class="modal-backdrop" onclick={onCancel} aria-label="Закрыть"
  ></button>
</div>
