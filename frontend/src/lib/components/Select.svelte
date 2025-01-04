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
    items.filter((i) =>
      getSearchLabel(i).toLocaleLowerCase().includes(value.toLocaleLowerCase()),
    ),
  );
</script>

<div class="dropdown">
  <input
    type="text"
    class="input input-bordered w-full"
    placeholder={activeItem ? getName(activeItem) : ""}
    bind:value
  />
  <ul
    class="dropdown-content menu bg-base-100 rounded-box z-10 w-52 p-2 shadow"
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
