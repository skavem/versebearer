<script lang="ts">
  import { RebuildSearchIndex } from "$lib/bindings/changeme/dbhandler";
  import { searchIndex } from "$lib/stores/searchStore.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  let error = $state("");
  let pending = $state(false);

  // Два источника: свой вызов (pending) и пересборка, запущенная не отсюда —
  // первым запуском или импортом перевода (searchIndex.building). Локальный
  // флаг нужен потому, что первое событие прогресса приходит не мгновенно, и
  // без него кнопку успевают нажать дважды.
  const busy = $derived(pending || searchIndex.building);

  const rebuild = async () => {
    error = "";
    pending = true;
    try {
      await RebuildSearchIndex();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      pending = false;
    }
  };
</script>

<div class="form-control">
  <div class="label py-1">
    <span class="label-text font-medium">Поисковый индекс</span>
  </div>

  <p class="label-text-alt mb-2 opacity-70">
    Индекс строится сам — при первом запуске и после импорта перевода.
    Пересоберите вручную, если поиск находит не то, что есть в базе.
  </p>

  <button class="btn btn-outline btn-sm gap-2" onclick={rebuild} disabled={busy}>
    {#if busy}
      <span class="loading loading-spinner loading-xs"></span>
      {searchIndex.total
        ? `Пересборка — ${searchIndex.done} из ${searchIndex.total}`
        : "Пересборка…"}
    {:else}
      <MuiIcon name="refresh" style="font-size: 1.15rem" />
      Пересобрать индекс
    {/if}
  </button>

  {#if error}
    <div class="label py-1">
      <span class="label-text-alt text-error">{error}</span>
    </div>
  {/if}
</div>
