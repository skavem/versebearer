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
  let windowHeight = $state(0);
  $effect(() => {
    shownItems;
    if (!dropdownContentEl || !windowHeight) return;
    const maxHeight =
      windowHeight - dropdownContentEl.getBoundingClientRect().top - 10;
    if (dropdownContentEl.scrollHeight <= maxHeight) return;
    dropdownContentEl.style.height = `${maxHeight}px`;
  });

  let inputEl = $state<HTMLInputElement | null>(null);
  $effect(() => {
    shownItems;
    if (!inputEl || !dropdownContentEl) return;
    dropdownContentEl.style.width = `${inputEl.getBoundingClientRect().width}px`;
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
      <li onfocus={console.log}>
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
