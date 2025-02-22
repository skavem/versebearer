<script lang="ts" generics="T extends {ID: number}">
  import type { Snippet } from "svelte";
  import ListItem from "./ListItem.svelte";

  let {
    items,
    getName,
    onClick,
    onDoubleClick,
    activeItem,
    getKey = (v) => v.ID.toString(),
    getActiveKey = (v) => v.ID,
    leftMark,
    rightMark,
  }: {
    items: T[];
    getName: (v: T) => string;
    onClick: (v: T) => void;
    onDoubleClick?: (v: T) => void;
    activeItem: T | null;
    getKey?: (v: T, i: number) => string;
    getActiveKey?: (v: T) => number;
    leftMark?: Snippet<[T, number]>;
    rightMark?: Snippet<[T, number]>;
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

  let scrolled = $state(false);
  $effect(() => {
    activeItem;
    scrolled = false;
  });
  $effect(() => {
    if (activeItem && mainDiv && !scrolled) {
      const i = items.findIndex((i) => getActiveKey(i) === getActiveKey(activeItem));
      if (i > shown.from && i < shown.to) return;
      scrolled = true;
      mainDiv.scrollTo({ top: i * 44 });
    }
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
    {#each items.slice(shown.from, shown.to + 1) as item, ind (getKey(item, ind))}
      <ListItem
        isActive={(activeItem ? getActiveKey(activeItem) : 0) ===
          getActiveKey(item)}
        onclick={() => {
          onClick(item);
        }}
        ondblclick={() => {
          onDoubleClick?.(item);
        }}
        {ind}
        top={(shown.from + ind) * 44}
        {getName}
        {item}
        {leftMark}
        {rightMark}
      />
    {/each}
  </div>
</div>
