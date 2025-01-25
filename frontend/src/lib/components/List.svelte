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
  }: {
    items: T[];
    getName: (v: T) => string;
    onClick: (v: T) => void;
    onDoubleClick?: (v: T) => void;
    activeItem: T | null;
    leftMark?: Snippet<[T]>;
    rightMark?: Snippet<[T]>;
  } = $props();

  let mainDiv = $state<HTMLDivElement | null>(null);
  let scrollTop = $state(0);
  let shown = $derived.by(() => {
    const from = Math.floor(scrollTop / 44);

    const to = Math.min(
      items.length - 1,
      Math.floor(
        (scrollTop + (mainDiv?.getBoundingClientRect().height || 0)) / 44,
      ) + 1,
    );

    return {
      from,
      to,
    };
  });

  $effect(() => {
    window.addEventListener("resize", () => {
      if (!mainDiv) return;
      scrollTop = mainDiv.scrollTop + 1;
    });
  });
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="group/list h-full select-none overflow-y-scroll border-2 border-zinc-100"
  onkeydown={(e) => {
    if (
      ["Space", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].indexOf(
        e.code,
      ) > -1
    ) {
      e.preventDefault();
    }
  }}
  bind:this={mainDiv}
  onscroll={(e) => {
    if (!mainDiv) return;
    scrollTop = mainDiv.scrollTop;
  }}
>
  <div class="relative w-full" style:height={`${(items.length - 1) * 44}px`}>
    {#each items.slice(shown.from, shown.to) as item, ind}
      <ListItem
        isActive={(activeItem?.ID || 0) === item.ID}
        onclick={() => {
          onClick(item);
        }}
        ondblclick={() => {
          onDoubleClick?.(item);
        }}
        top={(shown.from + ind) * 44}
        {getName}
        {item}
        {leftMark}
        {rightMark}
      />
    {/each}
  </div>
</div>
