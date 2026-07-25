<script lang="ts">
  import {
    ImportTranslationFromFile,
    ListTranslationSummaries,
    PickTranslationFile,
    RemoveTranslation,
  } from "$lib/bindings/changeme/dbhandler";
  import type {
    FilePick,
    TranslationSummary,
  } from "$lib/bindings/changeme/models";
  import { Events } from "@wailsio/runtime";
  import MuiIcon from "./MuiIcon.svelte";

  let list = $state<TranslationSummary[]>([]);
  let pick = $state<FilePick | null>(null);
  let nameInput = $state("");
  let shortInput = $state("");
  let importing = $state(false);
  let progress = $state<{ done: number; total: number; book: string } | null>(
    null,
  );
  let error = $state<string | null>(null);
  let done = $state<string | null>(null);
  let toDelete = $state<TranslationSummary | null>(null);

  const reload = async () => {
    list = (await ListTranslationSummaries()) ?? [];
  };
  reload();

  Events.On(
    "import_progress",
    ({ data }: { data: { done: number; total: number; book: string } }) => {
      progress = data;
    },
  );

  const preview = $derived(pick?.preview ?? null);
  const canImport = $derived(
    !!preview && !preview.error && nameInput.trim().length > 0,
  );

  const choose = async () => {
    error = null;
    done = null;
    const picked = await PickTranslationFile();
    if (!picked?.path) {
      pick = null; // диалог закрыли — молча выходим
      return;
    }
    pick = picked;
    nameInput = picked.preview?.name ?? "";
    shortInput = picked.preview?.shortName ?? "";
  };

  const submit = async () => {
    if (!pick?.path || !canImport) return;
    importing = true;
    error = null;
    progress = null;

    const result = await ImportTranslationFromFile(
      pick.path,
      nameInput.trim(),
      shortInput.trim(),
    );

    importing = false;
    progress = null;

    if (!result || result.error) {
      error = result?.error ?? "не удалось импортировать файл";
      return;
    }

    done = `«${result.name}» — книг ${result.books}, глав ${result.chapters}, стихов ${result.verses}`;
    pick = null;
    await reload();
  };

  const confirmDelete = async () => {
    if (!toDelete) return;
    const message = await RemoveTranslation(toDelete.id);
    toDelete = null;
    if (message) {
      error = message;
      return;
    }
    await reload();
  };

  const cancelPick = () => {
    pick = null;
    error = null;
  };
</script>

<div class="form-control">
  <div class="label py-1">
    <span class="label-text font-medium">Переводы Библии</span>
    <span class="label-text-alt opacity-70">{list.length} шт.</span>
  </div>

  <div class="mb-2 flex flex-col gap-1">
    {#each list as t (t.id)}
      <div class="flex items-center gap-2 rounded-lg bg-base-200 px-3 py-2">
        {#if t.shortName}
          <span class="badge badge-neutral badge-sm shrink-0">{t.shortName}</span>
        {/if}
        <div class="min-w-0 flex-1">
          <div class="truncate font-medium">{t.name}</div>
          <div class="text-xs opacity-60">
            книг {t.books} · стихов {t.verses.toLocaleString("ru")}
            {#if t.inUse}· <span class="text-warning">сейчас на экране</span>{/if}
          </div>
        </div>
        <button
          class="btn btn-ghost btn-sm btn-square text-error"
          onclick={() => (toDelete = t)}
          aria-label="Удалить перевод"
          title="Удалить перевод"
          disabled={list.length <= 1}
        >
          <MuiIcon name="delete" />
        </button>
      </div>
    {/each}
  </div>

  {#if !pick}
    <button class="btn btn-outline btn-sm gap-1" onclick={choose}>
      <MuiIcon name="add" style="font-size: 1.15rem" />
      Добавить перевод из файла
    </button>
  {:else}
    <div class="rounded-lg border border-base-300 p-3">
      <div class="mb-2 flex items-center gap-2">
        <MuiIcon name="description" />
        <span class="truncate text-sm opacity-70">{pick.fileName}</span>
      </div>

      {#if preview?.error}
        <div class="alert alert-error py-2 text-sm">{preview.error}</div>
      {:else if preview}
        <div class="mb-2 text-sm opacity-70">
          книг {preview.books} · глав {preview.chapters} · стихов {preview.verses.toLocaleString(
            "ru",
          )}
        </div>

        {#if preview.duplicate}
          <div class="alert alert-warning mb-2 py-2 text-sm">
            Перевод с таким названием уже установлен — измените название или
            удалите старый.
          </div>
        {/if}

        <div class="flex gap-2">
          <label class="form-control flex-1">
            <span class="label-text text-xs">Название</span>
            <input
              class="input input-sm input-bordered w-full"
              bind:value={nameInput}
              placeholder="Например: Новый русский перевод"
            />
          </label>
          <label class="form-control w-24">
            <span class="label-text text-xs">Метка</span>
            <input
              class="input input-sm input-bordered w-full"
              bind:value={shortInput}
              placeholder="НРП"
            />
          </label>
        </div>
      {/if}

      {#if importing}
        <div class="mt-3">
          <progress
            class="progress progress-primary w-full"
            value={progress?.done ?? 0}
            max={progress?.total ?? 1}
          ></progress>
          <div class="mt-1 truncate text-xs opacity-60">
            {progress
              ? `${progress.book} — ${progress.done} из ${progress.total}`
              : "Подготовка…"}
          </div>
        </div>
      {/if}

      <div class="mt-3 flex justify-end gap-2">
        <button class="btn btn-ghost btn-sm" onclick={cancelPick} disabled={importing}>
          Отмена
        </button>
        <button
          class="btn btn-primary btn-sm"
          onclick={submit}
          disabled={!canImport || importing}
        >
          {importing ? "Импорт…" : "Импортировать"}
        </button>
      </div>
    </div>
  {/if}

  {#if error}
    <div class="alert alert-error mt-2 py-2 text-sm">{error}</div>
  {/if}
  {#if done}
    <div class="alert alert-success mt-2 py-2 text-sm">Добавлен {done}</div>
  {/if}
</div>

{#if toDelete}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-2 flex items-center gap-3">
        <div
          class="flex h-10 w-10 items-center justify-center rounded-full bg-error/10 text-error"
        >
          <MuiIcon name="delete" />
        </div>
        <h3 class="text-lg font-bold">Удалить перевод?</h3>
      </div>

      <p class="py-2">
        Перевод <span class="font-semibold">«{toDelete.name}»</span>
        и все его {toDelete.verses.toLocaleString("ru")} стихов будут удалены безвозвратно.
      </p>

      <div class="modal-action">
        <button class="btn btn-ghost" onclick={() => (toDelete = null)}>
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
      onclick={() => (toDelete = null)}
      aria-label="Закрыть"
    ></button>
  </div>
{/if}
