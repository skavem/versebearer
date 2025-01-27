<script lang="ts" generics="T extends {ID: number}">
  import type { Snippet } from "svelte";
  import type { MouseEventHandler } from "svelte/elements";

  let {
    isActive,
    onclick,
    ondblclick,
    item,
    top,
    getName,
    leftMark,
    rightMark,
  }: {
    isActive: boolean;
    onclick?: MouseEventHandler<HTMLDivElement> | null;
    ondblclick?: MouseEventHandler<HTMLDivElement> | null;
    item: T;
    top: number;
    getName: (i: T) => string;
    leftMark?: Snippet<[T]>;
    rightMark?: Snippet<[T]>;
  } = $props();

  const isDivider = $derived(item.ID === -1);
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
{#if isDivider}
  <div
    class="divider absolute m-0 flex h-[44px] w-full cursor-default"
    style:top={`${top}px`}
  >
    {getName(item)}
  </div>
{:else}
  <div
    style:top={`${top}px`}
    class={[
      "group/item absolute flex w-full cursor-pointer flex-row items-center justify-between gap-2 rounded border-2 p-2 hover:bg-zinc-100",
      !isActive && "border-transparent",
      isActive && "border-primary",
    ]}
    {onclick}
    {ondblclick}
  >
    <div class="flex min-w-[calc(100%-34px)] items-center gap-2">
      {#if leftMark}
        {@render leftMark(item)}
      {/if}

      <span class="overflow-hidden text-ellipsis whitespace-nowrap"
        >{getName(item)}</span
      >
    </div>

    {#if rightMark}
      {@render rightMark(item)}
    {/if}
  </div>
{/if}
