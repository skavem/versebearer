<script lang="ts">
  import type { Verse } from "$lib/bindings/changeme/backend/models";
  import type { MouseEventHandler } from "svelte/elements";
  import MuiIcon from "./MuiIcon.svelte";

  let {
    isActive,
    isShown,
    onclick,
    ondblclick,
    verse,
  }: {
    isActive: boolean;
    isShown: boolean;
    onclick?: MouseEventHandler<HTMLDivElement> | null;
    ondblclick?: MouseEventHandler<HTMLDivElement> | null;
    verse: Verse;
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
    "group/item flex w-full cursor-pointer flex-row items-center justify-between gap-2 rounded border-2 p-2 transition-colors hover:bg-base-200",
    !isActive && "border-transparent",
    // primary, а не neutral: в тёмной теме neutral — это почти цвет фона,
    // рамка выделения пропадала. primary читается в обеих темах.
    isActive && "border-primary",
    // Заливки взаимоисключающие: у двух bg-* одного веса порядок решает CSS,
    // а не эта строка, поэтому янтарь «в эфире» задаётся явным приоритетом.
    isActive && !isShown && "bg-primary/10",
    isShown && "bg-secondary/20",
  ]}
  {onclick}
  {ondblclick}
>
  <div class="flex w-full min-w-0 items-center justify-between gap-2">
    <span>
      <div class="badge badge-neutral badge-md">
        {verse.number.toString()}
      </div>

      {verse.text}</span
    >

    {#if isShown}
      <MuiIcon name="visibility" style="color: var(--fallback-s,oklch(var(--s)))" />
    {/if}
  </div>
</div>
