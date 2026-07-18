<script lang="ts">
  import MuiIcon from "$lib/components/MuiIcon.svelte";
  import OutputCard from "$lib/components/OutputCard.svelte";
  import ScreenMiniMap from "$lib/components/ScreenMiniMap.svelte";
  import { outputStore } from "$lib/stores/outputStore.svelte";

  const outputs = $derived(outputStore.outputs);
  const projectingCount = $derived(outputStore.activeOutputIds.length);
  const pending = $derived(outputStore.pendingOutput);
</script>

<div class="flex flex-col gap-4 p-4">
  <div class="flex items-end justify-between">
    <div>
      <h2 class="text-xl font-bold">Экраны</h2>
      <p class="text-sm text-base-content/60">
        Настрой выходы и назначь им мониторы для трансляции стихов и куплетов
      </p>
    </div>
    <div class="flex items-center gap-2">
      <div class="stats stats-horizontal bg-base-200/40">
        <div class="stat px-4 py-2">
          <div class="stat-title text-[10px]">Выходов</div>
          <div class="stat-value text-xl">{outputs.length}</div>
        </div>
        <div class="stat px-4 py-2">
          <div class="stat-title text-[10px]">В эфире</div>
          <div class="stat-value text-xl text-secondary">{projectingCount}</div>
        </div>
      </div>
      <button class="btn btn-sm btn-neutral" onclick={() => outputStore.add()}>
        <MuiIcon name="add" style="font-size: 1.1rem" />
        Добавить выход
      </button>
    </div>
  </div>

  <div class="flex items-center gap-2 rounded-lg border border-base-300 bg-base-100 px-3 py-2 text-sm text-base-content/70 shadow-sm">
    <MuiIcon name="layers" style="font-size: 1.2rem" classes="text-base-content/40" />
    <span>
      Окно трансляции отображается <b>поверх</b> всех других программ на выбранном мониторе.
    </span>
  </div>

  {#if !outputStore.loaded}
    <div class="flex justify-center py-8">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
  {:else}
    {#if outputStore.monitors.length > 0}
      <ScreenMiniMap />
    {/if}

    {#if outputs.length === 0}
      <div class="flex flex-col items-center gap-3 rounded-xl border border-dashed border-base-300 bg-base-100/60 p-10 text-center">
        <div class="flex h-12 w-12 items-center justify-center rounded-full bg-base-200 text-base-content/40">
          <MuiIcon name="output" style="font-size: 1.75rem" />
        </div>
        <div>
          <p class="font-semibold text-base-content/80">Выходов пока нет</p>
          <p class="text-sm text-base-content/50">
            Добавь выход и назначь ему монитор, чтобы начать трансляцию
          </p>
        </div>
        <button class="btn btn-sm btn-neutral mt-1" onclick={() => outputStore.add()}>
          <MuiIcon name="add" style="font-size: 1.1rem" />
          Добавить выход
        </button>
      </div>
    {:else}
      <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
        {#each outputs as output, i (output.ID)}
          <OutputCard {output} index={i} />
        {/each}
      </div>
    {/if}
  {/if}
</div>

<svelte:window
  onkeydown={(e) => pending && e.key === "Escape" && outputStore.cancelPending()}
/>

{#if pending}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="flex items-start gap-3">
        <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-warning/10 text-warning">
          <MuiIcon name="warning" style="font-size: 1.5rem" />
        </div>
        <div class="flex-1">
          <h3 class="text-lg font-bold">Перекрыть текущий монитор?</h3>
          <p class="mt-1 text-sm text-base-content/70">
            Этот монитор используется VerseBearer. Окно трансляции перекроет
            интерфейс программы и все другие окна на этом экране.
          </p>
        </div>
      </div>
      <div class="modal-action">
        <button class="btn btn-ghost" onclick={() => outputStore.cancelPending()}>
          Отмена
        </button>
        <button class="btn btn-neutral" onclick={() => outputStore.confirmPending()}>
          <MuiIcon name="present_to_all" style="font-size: 1.1rem" />
          Всё равно транслировать
        </button>
      </div>
    </div>
    <div
      class="modal-backdrop"
      onclick={() => outputStore.cancelPending()}
      onkeydown={(e) => e.key === "Escape" && outputStore.cancelPending()}
      role="button"
      tabindex="-1"
      aria-label="Закрыть"
    ></div>
  </div>
{/if}
