<script lang="ts" generics="T extends {ID: number}">
  import type { Snippet } from "svelte";
  import ListItem from "./ListItem.svelte";

  let {
    items,
    getName,
    onClick,
    onDoubleClick,
    activeItem,
    leftMark,
    rightMark,
    dividerBefore,
    multiline = false,
  }: {
    items: T[];
    getName: (v: T) => string;
    onClick: (v: T) => void;
    onDoubleClick?: (v: T) => void;
    activeItem: T | null;
    leftMark?: Snippet<[T]>;
    rightMark?: Snippet<[T]>;
    dividerBefore?: Snippet<[T]>;
    multiline?: boolean;
  } = $props();

  let isLarge = $derived(items.length > 100);
  let totalSices = $derived(Math.ceil(items.length / 100));
  let curSlice = $state(0);
  const sliceSize = 100;
  let shownItems = $derived.by(() => {
    if (!isLarge) {
      return items;
    }

    return items.slice(sliceSize * curSlice, sliceSize * (curSlice + 1));
  });
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="h-full border-zinc-100 border-2 overflow-y-scroll select-none group/list"
  onkeydown={(e) => {
    if (
      ["Space", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].indexOf(
        e.code,
      ) > -1
    ) {
      e.preventDefault();
    }
  }}
>
  {#each shownItems as item}
    <ListItem
      isActive={(activeItem?.ID || 0) === item.ID}
      onclick={() => {
        onClick(item);
      }}
      ondblclick={() => {
        onDoubleClick?.(item);
      }}
      {getName}
      {item}
      {leftMark}
      {rightMark}
      {dividerBefore}
      {multiline}
    />
  {/each}
</div>

{#if isLarge}
  <div class="w-full flex flex-row gap-1 overflow-scroll pb-2">
    {#each new Array(totalSices).fill(null) as _, ind}
      {@const selected = ind === curSlice}
      <button
        class="rounded-full border p-1 flex items-center justify-center hover:bg-zinc-200 cursor-pointer"
        class:border-zinc-100={!selected}
        class:border-seawave={selected}
        onclick={() => (curSlice = ind)}
      >
        {ind}
      </button>
    {/each}
  </div>
{/if}
