<script lang="ts">
  import { themesStore } from "$lib/stores/themesStore.svelte";
  import MuiIcon from "./MuiIcon.svelte";
  import PopoverSelect from "./PopoverSelect.svelte";

  const themes = $derived(themesStore.themes);
  const active = $derived(themesStore.active);
  const themeOptions = $derived(
    themes.map((t) => ({
      value: t.ID,
      label: t.name,
      hint: t.isDefault ? "база" : undefined,
    })),
  );

  type Mode = "create" | "rename" | "delete" | null;
  let mode = $state<Mode>(null);
  let nameInput = $state("");

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

<div class="theme-bar">
  <PopoverSelect
    classes="min-w-0 flex-1"
    icon="palette"
    value={themesStore.activeId}
    options={themeOptions}
    ariaLabel="Активная тема вывода"
    onChange={(id) => themesStore.apply(id)}
  />

  <div class="theme-bar__actions" role="group" aria-label="Действия с темой">
    <button class="theme-bar__btn" onclick={openCreate} title="Создать тему" aria-label="Создать тему">
      <MuiIcon name="add" style="font-size: 1.1rem" />
    </button>
    <button
      class="theme-bar__btn"
      onclick={() => active && themesStore.duplicate(active.ID)}
      disabled={!active}
      title="Дублировать тему"
      aria-label="Дублировать тему"
    >
      <MuiIcon name="content_copy" style="font-size: 1.05rem" />
    </button>
    <button
      class="theme-bar__btn"
      onclick={openRename}
      disabled={!active}
      title="Переименовать тему"
      aria-label="Переименовать тему"
    >
      <MuiIcon name="edit" style="font-size: 1.05rem" />
    </button>
    <button
      class="theme-bar__btn theme-bar__btn--danger"
      onclick={() => (mode = "delete")}
      disabled={!active || active.isDefault}
      title={active?.isDefault ? "Базовую тему удалить нельзя" : "Удалить тему"}
      aria-label="Удалить тему"
    >
      <MuiIcon name="delete" style="font-size: 1.05rem" />
    </button>
  </div>
</div>

<svelte:window onkeydown={(e) => mode && e.key === "Escape" && close()} />

{#if mode === "create" || mode === "rename"}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-4 flex items-center gap-3">
        <div
          class="flex h-10 w-10 items-center justify-center rounded-full bg-neutral/10 text-base-content"
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
        <button class="btn btn-neutral" disabled={!nameInput.trim()} onclick={confirm}>
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
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-error/10 text-error">
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
  .theme-bar {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    padding: 0.375rem 0.625rem;
    border: 1px solid oklch(var(--b3));
    border-radius: 0.625rem;
    background-color: oklch(var(--b1));
    box-shadow: 0 1px 2px oklch(var(--bc) / 0.04);
  }

  .theme-bar__actions {
    display: flex;
    align-items: center;
    gap: 0.125rem;
    padding-left: 0.375rem;
    border-left: 1px solid oklch(var(--b3));
  }

  .theme-bar__btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2rem;
    height: 2rem;
    border: none;
    border-radius: 0.5rem;
    background: transparent;
    color: oklch(var(--bc) / 0.65);
    cursor: pointer;
    transition:
      background-color 0.12s,
      color 0.12s;
  }
  .theme-bar__btn:hover:not(:disabled) {
    background-color: oklch(var(--b2));
    color: oklch(var(--bc));
  }
  .theme-bar__btn:focus-visible {
    outline: 2px solid oklch(var(--p));
    outline-offset: 1px;
  }
  .theme-bar__btn:disabled {
    cursor: not-allowed;
    opacity: 0.3;
  }
  .theme-bar__btn--danger:hover:not(:disabled) {
    background-color: oklch(var(--er) / 0.15);
    color: oklch(var(--er));
  }
</style>
