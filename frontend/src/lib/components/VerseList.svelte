<script lang="ts">
  import type { Verse } from "$lib/bindings/changeme/backend/models";
  import { BibleStore } from "$lib/stores/BibleStore.svelte";
  import VerseItem from "./VerseItem.svelte";

  let {
    onClick,
    onDoubleClick,
  }: {
    onClick: (v: Verse) => void;
    onDoubleClick?: (v: Verse) => void;
  } = $props();

  let shown = $derived(BibleStore.verses.shown);
  let verses = $derived(BibleStore.verses.list);
  let active = $derived(BibleStore.verses.active);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="group/list h-0 flex-grow select-none overflow-y-scroll border-2 border-zinc-100"
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
    {#each verses as verse (verse.ID)}
      <VerseItem
        isActive={(active?.ID || 0) === verse.ID}
        onclick={() => {
          onClick(verse);
        }}
        ondblclick={() => {
          onDoubleClick?.(verse);
        }}
        {verse}
        isShown={shown?.ID === verse.ID}
      />
    {/each}
  </div>
</div>
