<script lang="ts">
  import { themesStore } from "$lib/stores/themesStore.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  const themes = $derived(themesStore.themes);
  const active = $derived(themesStore.active);

  type Mode = "create" | "rename" | "delete" | null;
  let mode = $state<Mode>(null);
  let nameInput = $state("");

  function pluralThemes(n: number): string {
    const mod10 = n % 10;
    const mod100 = n % 100;
    if (mod100 >= 11 && mod100 <= 14) return "тем";
    if (mod10 === 1) return "тема";
    if (mod10 >= 2 && mod10 <= 4) return "темы";
    return "тем";
  }

  function openCreate() {
    nameInput = "Новая тема";
    mode = "create";
  }

  function openRename() {
    if (!active) return;
    nameInput = active.name;
    mode = "rename";
  }

  function close() {
    mode = null;
  }

  async function confirm() {
    const name = nameInput.trim();
    if (mode === "create") {
      if (!name) return;
      await themesStore.create(name);
    } else if (mode === "rename") {
      if (!name || !active) return;
      await themesStore.rename(active.ID, name);
    } else if (mode === "delete") {
      if (active) await themesStore.remove(active.ID);
    }
    close();
  }
</script>

<div class="flex flex-col gap-3 rounded-xl border border-base-300 bg-base-100 p-4 shadow-sm">
  <!-- Header: title + actions -->
  <div class="flex flex-wrap items-center justify-between gap-3 border-b border-base-300 pb-3">
    <div class="flex items-center gap-2.5">
      <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-base-200">
        <MuiIcon name="palette" style="font-size: 1.25rem" />
      </div>
      <div class="leading-tight">
        <div class="text-sm font-semibold">Тема вывода</div>
        <div class="text-xs text-base-content/50">
          {themes.length} {pluralThemes(themes.length)}
        </div>
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <button class="btn btn-sm btn-neutral" onclick={openCreate}>
        <MuiIcon name="add" style="font-size: 1.1rem" />
        Создать
      </button>
      <div class="join">
        <button
          class="btn btn-sm btn-outline join-item"
          onclick={() => active && themesStore.duplicate(active.ID)}
          disabled={!active}
          title="Дублировать тему"
        >
          <MuiIcon name="content_copy" style="font-size: 1.1rem" />
          <span class="hidden sm:inline">Дублировать</span>
        </button>
        <button
          class="btn btn-sm btn-outline join-item"
          onclick={openRename}
          disabled={!active}
          title="Переименовать тему"
        >
          <MuiIcon name="edit" style="font-size: 1.1rem" />
          <span class="hidden sm:inline">Переименовать</span>
        </button>
      </div>
      <button
        class="btn btn-sm btn-outline btn-error"
        onclick={() => (mode = "delete")}
        disabled={!active || active.isDefault}
        title={active?.isDefault ? "Базовую тему удалить нельзя" : "Удалить тему"}
      >
        <MuiIcon name="delete" style="font-size: 1.1rem" />
        <span class="hidden sm:inline">Удалить</span>
      </button>
    </div>
  </div>

  <!-- Theme picker -->
  <div class="flex flex-wrap gap-2" role="group" aria-label="Выбор темы вывода">
    {#each themes as t (t.ID)}
      <button
        type="button"
        class={["theme-chip", t.ID === themesStore.activeId && "theme-chip--active"]}
        aria-pressed={t.ID === themesStore.activeId}
        onclick={() => themesStore.apply(t.ID)}
        title={t.name}
      >
        {#if t.ID === themesStore.activeId}
          <MuiIcon name="check" style="font-size: 1rem" />
        {/if}
        <span class="theme-chip__name">{t.name}</span>
        {#if t.isDefault}
          <span class="theme-chip__badge">база</span>
        {/if}
      </button>
    {/each}
  </div>
</div>

<svelte:window
  onkeydown={(e) => mode && e.key === "Escape" && close()}
/>

{#if mode === "create" || mode === "rename"}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-4 flex items-center gap-3">
        <div
          class="flex h-10 w-10 items-center justify-center rounded-full bg-neutral/10 text-neutral"
        >
          <MuiIcon name={mode === "create" ? "add" : "edit"} />
        </div>
        <h3 class="text-lg font-bold">
          {mode === "create" ? "Новая тема" : "Переименовать тему"}
        </h3>
      </div>
      <label class="form-control w-full">
        <div class="label py-1">
          <span class="label-text">Название темы</span>
        </div>
        <input
          type="text"
          bind:value={nameInput}
          class="input input-bordered w-full"
          placeholder="Например, Рождество"
          onkeydown={(e) => e.key === "Enter" && confirm()}
        />
      </label>
      <div class="modal-action">
        <button class="btn btn-ghost" onclick={close}>Отмена</button>
        <button
          class="btn btn-neutral"
          disabled={!nameInput.trim()}
          onclick={confirm}
        >
          <MuiIcon name={mode === "create" ? "add" : "check"} style="font-size: 1.1rem" />
          {mode === "create" ? "Создать" : "Сохранить"}
        </button>
      </div>
    </div>
    <button class="modal-backdrop" onclick={close} aria-label="Закрыть"></button>
  </div>
{/if}

{#if mode === "delete"}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-2 flex items-center gap-3">
        <div
          class="flex h-10 w-10 items-center justify-center rounded-full bg-error/10 text-error"
        >
          <MuiIcon name="delete" />
        </div>
        <h3 class="text-lg font-bold">Удалить тему?</h3>
      </div>
      <p class="py-2">
        Тема <span class="font-semibold">«{active?.name}»</span> будет удалена безвозвратно.
      </p>
      <div class="modal-action">
        <button class="btn btn-ghost" onclick={close}>Отмена</button>
        <button class="btn btn-error" onclick={confirm}>
          <MuiIcon name="delete" />
          Удалить
        </button>
      </div>
    </div>
    <button class="modal-backdrop" onclick={close} aria-label="Закрыть"></button>
  </div>
{/if}

<style>
  .theme-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    max-width: 16rem;
    padding: 0.375rem 0.75rem;
    border-radius: 9999px;
    border: 1px solid oklch(var(--b3));
    background-color: oklch(var(--b1));
    font-size: 0.8125rem;
    font-weight: 500;
    color: oklch(var(--bc) / 0.85);
    white-space: nowrap;
    cursor: pointer;
    transition:
      background-color 0.12s,
      border-color 0.12s,
      color 0.12s,
      transform 0.08s;
  }
  .theme-chip:hover {
    border-color: oklch(var(--bc) / 0.3);
    background-color: oklch(var(--b2));
  }
  .theme-chip:active {
    transform: scale(0.97);
  }
  .theme-chip--active {
    border-color: oklch(var(--n));
    background-color: oklch(var(--n));
    color: oklch(var(--nc));
  }
  .theme-chip--active:hover {
    background-color: oklch(var(--n));
  }
  .theme-chip__name {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .theme-chip__badge {
    font-size: 0.625rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    opacity: 0.65;
  }
</style>
