<script lang="ts" generics="T extends {ID: number}">
  import type { Match } from "$lib/bindings/changeme/backend/search/models";
  import { splitHighlights } from "$lib/highlight";
  import { searchIndex } from "$lib/stores/searchStore.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  let {
    query = $bindable(),
    placeholder,
    loading = false,
    items,
    activeItem,
    getText,
    getMatches,
    getBadge,
    onSelect,
    onNavigate,
    onSubmit,
    referenceLabel,
    onReference,
  }: {
    query: string;
    placeholder: string;
    loading?: boolean;
    items: T[];
    activeItem: T | null;
    getText: (i: T) => string;
    getMatches: (i: T) => Match[];
    getBadge: (i: T) => string;
    /** Выбор строки: переход к найденному. Поле и выпадашка после этого
     * очищаются — вызывающий код сбрасывает состояние поиска. */
    onSelect: (i: T) => void;
    onNavigate?: (delta: number) => void;
    onSubmit?: () => void;
    /** Разобранная ссылка («Ин 3:16») — отдельной строкой над результатами. */
    referenceLabel?: string;
    onReference?: () => void;
  } = $props();

  let inputEl = $state<HTMLInputElement | null>(null);
  let dropdownEl = $state<HTMLUListElement | null>(null);
  let windowHeight = $state(0);

  const isOpen = $derived(query.trim().length > 0);

  export function focus() {
    inputEl?.focus();
    inputEl?.select();
  }

  /** Закрыть выпадашку. Через blur, а не флагом: выпадашка daisyUI держится
   * на :focus-within, и своё состояние открытости разошлось бы с реальным
   * фокусом.
   *
   * Блюрится именно активный элемент, а не поле ввода: при выборе мышью фокус
   * уже перешёл на кнопку строки, и blur() по инпуту не закрыл бы ничего. */
  function close() {
    (document.activeElement as HTMLElement | null)?.blur();
  }

  function choose(item: T) {
    onSelect(item);
    close();
  }

  // Подгонка под ширину поля и высоту окна — тот же приём, что в Select.
  $effect(() => {
    items;
    const el = dropdownEl;
    if (!el) return;
    if (inputEl) {
      el.style.width = `${inputEl.getBoundingClientRect().width}px`;
    }
    if (windowHeight) {
      const maxHeight = windowHeight - el.getBoundingClientRect().top - 10;
      el.style.maxHeight = `${maxHeight}px`;
    }
  });

  // Активная строка меняется стрелками, пока курсор в поле, — подтягиваем её
  // в зону видимости.
  $effect(() => {
    if (!activeItem || !dropdownEl) return;
    dropdownEl
      .querySelector<HTMLElement>(`[data-id="${activeItem.ID}"]`)
      ?.scrollIntoView({ block: "nearest" });
  });
</script>

<svelte:window bind:innerHeight={windowHeight} />

<div class="dropdown w-full">
  <label class="input input-bordered flex w-full items-center gap-2">
    <MuiIcon name="search" style="font-size: 1.25rem; opacity: .6" />

    <input
      bind:this={inputEl}
      bind:value={query}
      type="text"
      class="grow"
      {placeholder}
      spellcheck="false"
      autocomplete="off"
      onkeydown={(e) => {
        switch (e.key) {
          case "ArrowDown":
            onNavigate?.(1);
            e.preventDefault();
            return;
          case "ArrowUp":
            onNavigate?.(-1);
            e.preventDefault();
            return;
          case "Enter":
            onSubmit?.();
            close();
            e.preventDefault();
            return;
          case "Escape":
            query = "";
            close();
            e.preventDefault();
            return;
        }
      }}
    />

    <!-- Индикатор построения индекса живёт прямо в поле: это единственное
         место, куда оператор смотрит, когда поиск ничего не находит. -->
    {#if searchIndex.building}
      <span class="loading loading-spinner loading-xs text-primary"></span>
      <span class="whitespace-nowrap text-xs opacity-60">
        индексация {searchIndex.done}/{searchIndex.total}
      </span>
    {:else if loading}
      <span class="loading loading-spinner loading-xs opacity-60"></span>
    {:else if query}
      <button
        class="btn btn-circle btn-ghost btn-xs"
        onclick={() => {
          query = "";
          inputEl?.focus();
        }}
        title="Очистить"
        aria-label="Очистить"
      >
        <MuiIcon name="close" style="font-size: 1rem" />
      </button>
    {/if}
  </label>

  {#if isOpen}
    <ul
      bind:this={dropdownEl}
      class="dropdown-content menu z-10 mt-1 flex-nowrap overflow-y-auto rounded-box bg-base-100 p-2 shadow-lg"
    >
      {#if referenceLabel}
        <li>
          <button
            class="flex flex-row items-center gap-2 text-primary"
            onclick={(e) => {
              e.preventDefault();
              onReference?.();
              close();
            }}
          >
            <MuiIcon name="arrow_forward" style="font-size: 1.1rem" />
            Перейти к {referenceLabel}
          </button>
        </li>
      {/if}

      {#each items as item (item.ID)}
        <li data-id={item.ID}>
          <button
            class={[
              "flex flex-row items-start gap-2 text-left",
              activeItem?.ID === item.ID && "active",
            ]}
            onclick={(e) => {
              e.preventDefault();
              choose(item);
            }}
          >
            <span class="badge badge-neutral badge-sm shrink-0 text-nowrap">
              {getBadge(item)}
            </span>
            <span class="min-w-0 whitespace-pre-line">
              {#each splitHighlights(getText(item), getMatches(item)) as part}
                {#if part.hit}<mark
                    class="rounded bg-primary/25 px-0.5 font-semibold text-inherit"
                    >{part.text}</mark
                  >{:else}{part.text}{/if}
              {/each}
            </span>
          </button>
        </li>
      {/each}

      {#if items.length === 0 && !referenceLabel}
        <li class="p-2 text-center opacity-60">
          {#if searchIndex.building}
            {searchIndex.label}
          {:else if loading}
            Ищем…
          {:else}
            Ничего не найдено
          {/if}
        </li>
      {/if}
    </ul>
  {/if}
</div>
