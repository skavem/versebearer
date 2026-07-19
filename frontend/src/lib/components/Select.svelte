<script lang="ts" generics="T extends { ID: number }">
  let {
    items = $bindable(),
    getName,
    activeItem,
    setActiveItem,
    getSearchLabel,
  }: {
    items: T[];
    activeItem: T | null;
    setActiveItem: (item: T) => void;
    getName: (item: T) => string;
    getSearchLabel?: (item: T) => string;
  } = $props();

  getSearchLabel ??= getName;

  let value = $state("");
  let shownItems = $derived.by(() =>
    items
      .filter((i) =>
        getSearchLabel(i)
          .toLocaleLowerCase()
          .includes(value.toLocaleLowerCase()),
      )
      .slice(0, 20),
  );

  let dropdownContentEl = $state<HTMLUListElement | null>(null);
  let inputEl = $state<HTMLInputElement | null>(null);
  let windowHeight = $state(0);

  // Fit the dropdown to the input width and cap its height to the viewport.
  $effect(() => {
    shownItems; // re-measure whenever the filtered list changes
    const el = dropdownContentEl;
    if (!el) return;
    if (inputEl) {
      el.style.width = `${inputEl.getBoundingClientRect().width}px`;
    }
    if (windowHeight) {
      const maxHeight = windowHeight - el.getBoundingClientRect().top - 10;
      el.style.height = el.scrollHeight > maxHeight ? `${maxHeight}px` : "";
    }
  });
</script>

<svelte:window bind:innerHeight={windowHeight} />

<div class="dropdown">
  <input
    type="text"
    class="input input-bordered w-full"
    placeholder={activeItem ? getName(activeItem) : ""}
    bind:value
    bind:this={inputEl}
  />
  <ul
    class="dropdown-content menu bg-base-100 rounded-box z-10 flex-nowrap overflow-y-scroll p-2 shadow"
    bind:this={dropdownContentEl}
  >
    {#each shownItems as item}
      <li>
        <button
          class="dropdown-item"
          onclick={(e) => {
            e.preventDefault();
            setActiveItem(item);
            e.currentTarget?.blur();
            value = "";
          }}
        >
          {getName(item)}
        </button>
      </li>
    {/each}
  </ul>
</div>
