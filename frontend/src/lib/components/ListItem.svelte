<script lang="ts" generics="T extends {ID: number}">
  import type { Snippet } from "svelte";
  import type { MouseEventHandler } from "svelte/elements";

  let {
    isActive,
    onclick,
    ondblclick,
    item,
    leftMark,
    rightMark,
    dividerBefore,
    getName,
    multiline,
  }: {
    isActive: boolean;
    onclick?: MouseEventHandler<HTMLDivElement> | null;
    ondblclick?: MouseEventHandler<HTMLDivElement> | null;
    item: T;
    leftMark?: Snippet<[T]>;
    rightMark?: Snippet<[T]>;
    dividerBefore?: Snippet<[T]>;
    getName: (i: T) => string;
    multiline: boolean;
  } = $props();

  let outerDiv = $state<HTMLDivElement | null>(null);

  $effect(() => {
    if (isActive && outerDiv) {
      outerDiv.scrollIntoView({ behavior: "smooth", block: "nearest" });
    }
  });
</script>

{@render dividerBefore?.(item)}

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  bind:this={outerDiv}
  class={[
    "group/item flex cursor-pointer flex-row items-center justify-between gap-2 rounded border-2 p-2 hover:bg-zinc-100",
    !isActive && "border-transparent",
    isActive && "border-primary",
  ]}
  {onclick}
  {ondblclick}
>
  <div
    class="flex"
    class:flex-col={multiline}
    class:gap-1={multiline}
    class:gap-2={!multiline}
    class:items-center={!multiline}
  >
    {#if leftMark}
      {@render leftMark(item)}
    {/if}

    <span class:whitespace-pre={multiline}>
      {getName(item)}
    </span>
  </div>

  {#if rightMark}
    {@render rightMark(item)}
  {/if}
</div>
