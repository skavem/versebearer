<script lang="ts" generics="T">
  import type { Snippet } from "svelte";
  import type { MouseEventHandler } from "svelte/elements";

  let {
    isActive,
    onclick,
    ondblclick,
    item,
    leftMark,
    rightMark,
    getName,
    multiline,
    oncontextmenu,
  }: {
    isActive: boolean;
    onclick: MouseEventHandler<HTMLDivElement> | null;
    ondblclick: MouseEventHandler<HTMLDivElement> | null;
    item: T;
    leftMark: Snippet<[T]>;
    rightMark: Snippet<[T]>;
    getName: (i: T) => string;
    multiline: boolean;
    oncontextmenu: MouseEventHandler<HTMLDivElement>;
  } = $props();

  let outerDiv = $state<HTMLDivElement | null>(null);

  $effect(() => {
    if (isActive && outerDiv) {
      outerDiv.scrollIntoView({ behavior: "smooth", block: "nearest" });
    }
  });
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  bind:this={outerDiv}
  class={[
    "flex flex-row items-center justify-between p-2 hover:bg-zinc-100 gap-2 border-2 rounded cursor-pointer group/item",
    {
      "border-transparent": !isActive,
      "border-primary": isActive,
    },
  ]}
  {onclick}
  {ondblclick}
  {oncontextmenu}
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
