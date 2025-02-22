<script lang="ts">
  import type { Verse } from "$lib/bindings/changeme/backend/models";
  import type { Snippet } from "svelte";
  import type { MouseEventHandler } from "svelte/elements";

  let {
    isActive,
    onclick,
    ondblclick,
    verse,
    ind,
    leftMark,
  }: {
    isActive: boolean;
    onclick?: MouseEventHandler<HTMLDivElement> | null;
    ondblclick?: MouseEventHandler<HTMLDivElement> | null;
    verse: Verse;
    ind: number;
    leftMark?: Snippet<[Verse, number]>;
  } = $props();
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class={[
    "group/item flex w-full cursor-pointer flex-row items-center justify-between gap-2 rounded border-2 p-2 hover:bg-zinc-100",
    !isActive && "border-transparent",
    isActive && "border-black/40",
  ]}
  {onclick}
  {ondblclick}
>
  <div class="flex min-w-0 items-center gap-2">
    {#if leftMark}
      {@render leftMark(verse, ind)}
    {/if}

    <span>{verse.text}</span>
  </div>
</div>
