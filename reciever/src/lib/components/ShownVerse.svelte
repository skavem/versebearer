<script lang="ts">
  import { fade, fly } from "svelte/transition";
  import type { IShownVerse, IVisualStyle } from "../../types";

  let {
    verse,
    style,
    fonts,
  }: {
    verse: IShownVerse | null;
    style: IVisualStyle;
    fonts: { ID: number; name: string; mimeType: string }[];
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

  function hexToRgba(hex: string, opacity: number): string {
    const r = parseInt(hex.slice(1, 3), 16);
    const g = parseInt(hex.slice(3, 5), 16);
    const b = parseInt(hex.slice(5, 7), 16);
    return `rgba(${r}, ${g}, ${b}, ${opacity})`;
  }

  function fontFamily(style: IVisualStyle, fonts: { ID: number; name: string }[]): string {
    if (style.fontId == null) return '"Century Gothic"';
    const f = fonts.find((x) => x.ID === style.fontId);
    return f ? `"${f.name}", "Century Gothic"` : '"Century Gothic"';
  }
</script>

{#if verse}
  <div class="outer" transition:fade style:padding="calc(2rem + {style.margin}px)">
    <div
      class="inner"
      bind:this={outerDiv}
      style:background-color={hexToRgba(style.bgColor, style.bgOpacity)}
      style:border-color={style.borderColor}
      style:border-width="{style.borderWidth}px"
      style:border-radius="{style.borderRadius}px"
      style:border-style={style.borderStyle}
      style:padding="{style.padding}px"
    >
      {#key verse}
        <div
          bind:this={verseDiv}
          class="text"
          in:fly={{ y: 20 }}
          style:color={style.textColor}
          style:font-family={fontFamily(style, fonts)}
          style:text-shadow={style.textShadow || "none"}
        >
          {verse.text}
        </div>

        <span
          class="link"
          in:fly={{ y: 20 }}
          style:color={style.textColor}
          style:font-family={fontFamily(style, fonts)}
          style:text-shadow={style.textShadow || "none"}
          >{verse.Book.shortName} {verse.Chapter.number}:{verse.number}</span
        >
      {/key}
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

    max-height: 100%;
    width: 100%;
  }

  .text {
    white-space: pre;
    text-wrap: wrap;
    text-align: center;
  }

  .link {
    width: 100%;

    text-align: right;
    font-size: 4rem;
  }
</style>
