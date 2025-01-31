<script lang="ts">
  import type { IShownVerse } from "../../types";

  let {
    verse,
  }: {
    verse: IShownVerse | null;
  } = $props();

  const isOverflown = ({
    clientWidth,
    clientHeight,
    scrollWidth,
    scrollHeight,
  }: HTMLDivElement) => {
    return scrollHeight > clientHeight || scrollWidth > clientWidth;
  };

  let verseDiv = $state<null | HTMLDivElement>(null);
  let outerDiv = $state<null | HTMLDivElement>(null);
  $effect(() => {
    if (!verseDiv || !outerDiv || !verse) return;

    let size = 4;
    outerDiv.style.fontSize = `${size}em`;
    while (!isOverflown(outerDiv)) {
      outerDiv.style.fontSize = `${size}em`;
      size += 0.1;
      if (size > 8) break;
    }
    while (isOverflown(outerDiv)) {
      outerDiv.style.fontSize = `${size}em`;
      size -= 0.1;
    }
  });
</script>

{#if verse}
  <div class="outer">
    <div class="inner" bind:this={outerDiv}>
      <div bind:this={verseDiv} class="text">
        {verse.text}
      </div>

      <span class="link"
        >{verse.Book.shortName} {verse.Chapter.number}:{verse.number}</span
      >
    </div>
  </div>
{/if}

<style>
  .outer {
    height: 50vh;
    width: 100vw;
    padding: 2rem;
    box-sizing: border-box;

    font-weight: 700;
    color: white;

    position: absolute;
    top: 50%;
    left: 0;

    display: flex;
    justify-content: center;
    align-items: end;
  }

  .inner {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    box-sizing: border-box;
    padding: 2rem;

    max-height: 100%;
    width: 100%;

    border-radius: 1rem;

    background-color: rgb(0 0 0 / 85%);
  }

  .text {
    white-space: pre;
    text-wrap: wrap;
    text-align: center;
    color: white;
    font-family: "Century Gothic";
  }

  .link {
    width: 100%;

    text-align: right;
    font-size: 4rem;
    color: white;
    font-family: "Century Gothic";
  }
</style>
