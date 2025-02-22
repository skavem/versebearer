<script lang="ts">
  import type { Verse } from "$lib/bindings/changeme/backend/models";
  import type { Snippet } from "svelte";
  import VerseItem from "./VerseItem.svelte";
  import { BibleStore } from "$lib/stores/BibleStore.svelte";

  let {
    onClick,
    onDoubleClick,
    leftMark,
  }: {
    onClick: (v: Verse) => void;
    onDoubleClick?: (v: Verse) => void;
    leftMark?: Snippet<[Verse, number]>;
  } = $props();

  let verses = $derived(BibleStore.verses.list);
  let active = $derived(BibleStore.verses.active);
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
>
  <div class="w-full">
    {#each verses as verse, ind (verse.ID)}
      <VerseItem
        isActive={(active?.ID || 0) === verse.ID}
        onclick={() => {
          onClick(verse);
        }}
        ondblclick={() => {
          onDoubleClick?.(verse);
        }}
        {ind}
        {verse}
        {leftMark}
      />
    {/each}
  </div>
</div>
