<script lang="ts">
  import MuiIcon from "$lib/components/MuiIcon.svelte";
  import ScreenMiniMap from "$lib/components/ScreenMiniMap.svelte";
  import ScreenToggler from "$lib/components/ScreenToggler.svelte";
  import { screenStore } from "$lib/stores/screenStore.svelte";

  const screens = $derived(screenStore.list);
  const projectingCount = $derived(screenStore.activeScreens.length);
  const pending = $derived(screenStore.pendingScreen);
</script>

<div class="flex flex-col gap-4 p-4">
  <div class="flex items-end justify-between">
    <div>
      <h2 class="text-xl font-bold">Экраны</h2>
      <p class="text-sm text-base-content/60">
        Выбери монитор для трансляции стихов и куплетов
      </p>
    </div>
    <div class="stats stats-horizontal bg-base-200/40">
      <div class="stat px-4 py-2">
        <div class="stat-title text-[10px]">Всего</div>
        <div class="stat-value text-xl">{screens.length}</div>
      </div>
      <div class="stat px-4 py-2">
        <div class="stat-title text-[10px]">Транслируется</div>
        <div class="stat-value text-xl text-neutral">{projectingCount}</div>
      </div>
    </div>
  </div>

  <div class="flex items-center gap-2 rounded-lg border border-base-300 bg-base-200/60 px-3 py-2 text-sm text-base-content/80">
    <MuiIcon name="layers" style="font-size: 1.2rem" />
    <span>
      Окно трансляции отображается <b>поверх</b> всех других программ на выбранном мониторе.
    </span>
  </div>

  {#if screens.length === 0}
    <div class="rounded-2xl border border-dashed border-base-300 p-8 text-center text-base-content/60">
      Мониторы не обнаружены
    </div>
  {:else}
    <ScreenMiniMap />

    <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
      {#each screens as scr, i (scr.ID)}
        <ScreenToggler {scr} index={i} />
      {/each}
    </div>
  {/if}
</div>

{#if pending}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="flex items-start gap-3">
        <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-neutral text-neutral-content">
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
        <button class="btn btn-ghost" onclick={() => screenStore.cancelPending()}>
          Отмена
        </button>
        <button class="btn btn-neutral" onclick={() => screenStore.confirmPending()}>
          Всё равно транслировать
        </button>
      </div>
    </div>
    <div
      class="modal-backdrop"
      onclick={() => screenStore.cancelPending()}
      onkeydown={(e) => e.key === "Escape" && screenStore.cancelPending()}
      role="button"
      tabindex="-1"
      aria-label="Закрыть"
    ></div>
  </div>
{/if}
