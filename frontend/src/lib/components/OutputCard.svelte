<script lang="ts">
  import { outputStore } from "$lib/stores/outputStore.svelte";
  import { themesStore } from "$lib/stores/themesStore.svelte";
  import type { Output } from "$lib/bindings/changeme/backend/models";
  import MuiIcon from "./MuiIcon.svelte";
  import PopoverSelect from "./PopoverSelect.svelte";

  const { output, index }: { output: Output; index: number } = $props();

  const projecting = $derived(outputStore.isActive(output));
  const monitor = $derived(outputStore.monitorFor(output));
  const monitorMissing = $derived(output.screenId !== "" && !monitor);
  const isCurrent = $derived(!!monitor && output.screenId === outputStore.currentScreenID);
  const scalePct = $derived(monitor ? Math.round(monitor.ScaleFactor * 100) : 0);

  const monitorOptions = $derived([
    { value: "", label: "Не выбран" },
    ...outputStore.monitors.map((m, i) => ({
      value: m.ID,
      label: `Монитор ${i + 1}`,
      hint: m.IsPrimary ? "основной" : undefined,
    })),
  ]);

  const defaultThemeId = $derived(themesStore.themes.find((t) => t.isDefault)?.ID ?? 0);
  const themeValue = $derived(output.themeId ?? defaultThemeId);
  const themeOptions = $derived(
    themesStore.themes.map((t) => ({
      value: t.ID,
      label: t.name,
      hint: t.isDefault ? "база" : undefined,
    })),
  );

  let editingName = $state(false);
  let nameDraft = $state(output.name);
  let nameInput = $state<HTMLInputElement | null>(null);
  let confirmDelete = $state(false);

  const monitorState = $derived.by(() => {
    if (monitorMissing) return "missing" as const;
    if (!monitor) return "unassigned" as const;
    return "ready" as const;
  });

  $effect(() => {
    if (!editingName) nameDraft = output.name;
  });

  function startEdit() {
    nameDraft = output.name;
    editingName = true;
    queueMicrotask(() => nameInput?.focus());
  }

  async function commitName() {
    editingName = false;
    const trimmed = nameDraft.trim();
    if (trimmed && trimmed !== output.name) {
      await outputStore.rename(output.ID, trimmed);
    } else {
      nameDraft = output.name;
    }
  }

  function cancelEdit() {
    nameDraft = output.name;
    editingName = false;
  }
</script>

<div
  class={[
    "group/card card border bg-base-100 transition-all",
    projecting ? "border-neutral shadow-md shadow-neutral/20" : "border-base-300",
    isCurrent && "ring-2 ring-secondary ring-offset-2 ring-offset-base-100",
  ]}
>
  <div class="card-body gap-3 p-4">
    <div class="flex items-start justify-between gap-2">
      <div class="flex min-w-0 items-center gap-2">
        <div
          class={[
            "flex h-10 w-10 shrink-0 items-center justify-center rounded-lg",
            projecting ? "bg-neutral text-neutral-content" : "bg-base-200",
          ]}
        >
          <MuiIcon name="output" style="font-size: 1.5rem" />
        </div>
        <div class="min-w-0">
          {#if editingName}
            <input
              bind:this={nameInput}
              type="text"
              class="input input-sm input-bordered w-40"
              bind:value={nameDraft}
              onblur={commitName}
              onkeydown={(e) => {
                if (e.key === "Enter") commitName();
                if (e.key === "Escape") cancelEdit();
              }}
            />
          {:else}
            <div class="flex min-w-0 items-center gap-1">
              <h3 class="truncate text-lg font-bold leading-tight">{output.name}</h3>
              <button
                type="button"
                class="oc-inline-btn opacity-0 transition-opacity group-hover/card:opacity-100 focus-visible:opacity-100"
                onclick={startEdit}
                title="Переименовать выход"
                aria-label="Переименовать выход"
              >
                <MuiIcon name="edit" style="font-size: 0.85rem" />
              </button>
            </div>
          {/if}
          <div class="text-xs leading-tight text-base-content/60">
            Выход {index + 1}
          </div>
        </div>
      </div>
      <div class="flex shrink-0 items-start gap-1.5">
        <div class="flex flex-col items-end gap-1">
          {#if monitor?.IsPrimary}
            <span class="badge badge-sm badge-ghost">Основной</span>
          {/if}
          {#if isCurrent}
            <span class="badge badge-sm badge-secondary">Здесь</span>
          {/if}
          {#if projecting}
            <span class="badge badge-sm badge-neutral gap-1">
              <span class="oc-live-dot" aria-hidden="true"></span>
              В эфире
            </span>
          {/if}
        </div>
        <button
          type="button"
          class="oc-inline-btn oc-inline-btn--danger opacity-0 transition-opacity group-hover/card:opacity-100 focus-visible:opacity-100"
          onclick={() => (confirmDelete = true)}
          title="Удалить выход"
          aria-label="Удалить выход"
        >
          <MuiIcon name="delete" style="font-size: 0.95rem" />
        </button>
      </div>
    </div>

    <div class="flex flex-col gap-2">
      <PopoverSelect
        icon="monitor"
        value={output.screenId}
        options={monitorOptions}
        ariaLabel="Монитор выхода"
        onChange={(v) => outputStore.setScreen(output.ID, v)}
      />
      <PopoverSelect
        icon="palette"
        value={themeValue}
        options={themeOptions}
        ariaLabel="Тема выхода"
        onChange={(v) => outputStore.setTheme(output.ID, v)}
      />
    </div>

    {#if monitorState === "missing"}
      <div class="flex items-center gap-1.5 rounded-md bg-warning/10 p-2 text-xs text-warning">
        <MuiIcon name="warning" style="font-size: 1rem" />
        Монитор не найден — выберите другой
      </div>
    {:else if monitorState === "unassigned"}
      <div class="flex items-center gap-1.5 rounded-md bg-base-200/60 p-2 text-xs text-base-content/50">
        <MuiIcon name="info" style="font-size: 1rem" />
        Монитор не выбран — трансляция недоступна
      </div>
    {:else}
      <div class="grid grid-cols-2 gap-2 text-sm">
        <div class="flex flex-col rounded-md bg-base-200/60 p-2">
          <span class="text-[10px] uppercase tracking-wide text-base-content/50">
            Разрешение
          </span>
          <span class="font-mono font-semibold">
            {monitor?.Bounds.Width}×{monitor?.Bounds.Height}
          </span>
        </div>
        <div class="flex flex-col rounded-md bg-base-200/60 p-2">
          <span class="text-[10px] uppercase tracking-wide text-base-content/50">
            Масштаб
          </span>
          <span class="font-mono font-semibold">{scalePct}%</span>
        </div>
      </div>
    {/if}

    <label
      class={[
        "flex items-center justify-between gap-2 rounded-md bg-base-200/60 p-2",
        projecting ? "cursor-not-allowed opacity-60" : "cursor-pointer",
      ]}
      title={projecting
        ? "Остановите трансляцию, чтобы изменить режим"
        : "Окно проектора станет прозрачным — виден только текст поверх остального содержимого монитора"}
    >
      <span class="flex items-center gap-1 text-sm">
        <MuiIcon name="opacity" style="font-size: 1.1rem" />
        Прозрачный экран
      </span>
      <input
        type="checkbox"
        class="toggle toggle-sm toggle-neutral"
        checked={output.transparent}
        disabled={projecting}
        onchange={(e) => outputStore.setTransparent(output.ID, e.currentTarget.checked)}
      />
    </label>

    <button
      onclick={() => outputStore.requestToggle(output)}
      disabled={!projecting && !monitor}
      title={!projecting && !monitor ? "Сначала выберите монитор" : undefined}
      class={[
        "btn btn-sm w-full",
        projecting ? "btn-outline btn-error" : "btn-neutral",
      ]}
    >
      <MuiIcon
        name={projecting ? "cancel_presentation" : "present_to_all"}
        style="font-size: 1.1rem"
      />
      {projecting ? "Остановить трансляцию" : "Транслировать"}
    </button>
  </div>
</div>

<svelte:window onkeydown={(e) => confirmDelete && e.key === "Escape" && (confirmDelete = false)} />

{#if confirmDelete}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-2 flex items-center gap-3">
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-error/10 text-error">
          <MuiIcon name="delete" />
        </div>
        <h3 class="text-lg font-bold">Удалить выход?</h3>
      </div>
      <p class="py-2">
        Выход <span class="font-semibold">«{output.name}»</span> будет удалён безвозвратно.
        {#if projecting}Трансляция на нём остановится.{/if}
      </p>
      <div class="modal-action">
        <button class="btn btn-ghost" onclick={() => (confirmDelete = false)}>Отмена</button>
        <button
          class="btn btn-error"
          onclick={() => {
            confirmDelete = false;
            outputStore.remove(output.ID);
          }}
        >
          <MuiIcon name="delete" />
          Удалить
        </button>
      </div>
    </div>
    <button class="modal-backdrop" onclick={() => (confirmDelete = false)} aria-label="Закрыть"></button>
  </div>
{/if}

<style>
  .oc-inline-btn {
    display: flex;
    flex: none;
    align-items: center;
    justify-content: center;
    width: 1.5rem;
    height: 1.5rem;
    border: none;
    border-radius: 9999px;
    background: transparent;
    color: oklch(var(--bc) / 0.5);
    cursor: pointer;
    transition:
      background-color 0.12s,
      color 0.12s;
  }
  .oc-inline-btn:hover {
    background-color: oklch(var(--b2));
    color: oklch(var(--bc));
  }
  .oc-inline-btn:focus-visible {
    outline: 2px solid oklch(var(--p));
    outline-offset: 1px;
  }
  .oc-inline-btn--danger:hover {
    background-color: oklch(var(--er) / 0.15);
    color: oklch(var(--er));
  }

  .oc-live-dot {
    display: inline-block;
    width: 0.375rem;
    height: 0.375rem;
    border-radius: 9999px;
    background-color: oklch(var(--nc));
    animation: oc-pulse 1.6s ease-in-out infinite;
  }
  @keyframes oc-pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.35;
    }
  }
</style>
