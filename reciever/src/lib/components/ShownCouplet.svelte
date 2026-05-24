<script lang="ts">
  import QrCode from "@castlenine/svelte-qrcode";
  import { fade, fly } from "svelte/transition";
  import type { IShownCouplet, IVisualStyle } from "../../types";

  let {
    couplet,
    qr,
    style,
    fonts,
  }: {
    couplet: IShownCouplet | null;
    qr: boolean;
    style: IVisualStyle;
    fonts: { ID: number; name: string; mimeType: string }[];
  } = $props();

  let innerDiv = $state<null | HTMLDivElement>(null);
  let coupletDiv = $state<null | HTMLDivElement>(null);
  let qrDiv = $state<null | HTMLDivElement>(null);

  const getElHeight = (el: HTMLElement) => {
    var cs = getComputedStyle(el);
    var paddingY = parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom);
    var borderY =
      parseFloat(cs.borderTopWidth) + parseFloat(cs.borderBottomWidth);
    return el.offsetHeight - paddingY - borderY;
  };

  const isOverflown = () => {
    if (!coupletDiv || !innerDiv) return false;
    return (
      coupletDiv.clientHeight + (qrDiv ? getElHeight(qrDiv) : 0) >
      getElHeight(innerDiv)
    );
  };
  $effect(() => {
    if (!coupletDiv || !innerDiv || !couplet) return;

    let size = 50;
    coupletDiv.style.fontSize = `${size}px`;
    coupletDiv.style.lineHeight = `${size}px`;
    while (!isOverflown()) {
      coupletDiv.style.fontSize = `${size}px`;
      coupletDiv.style.lineHeight = `${size}px`;
      size++;
      if (size > 120) break;
    }
    while (isOverflown()) {
      coupletDiv.style.fontSize = `${size}px`;
      coupletDiv.style.lineHeight = `${size}px`;
      size--;
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

{#if couplet}
  <div class="outer" transition:fade={{ duration: 600 }} style:padding="{style.margin}px">
    <div
      class="inner"
      bind:this={innerDiv}
      style:background-color={hexToRgba(style.bgColor, style.bgOpacity)}
      style:border-color={style.borderColor}
      style:border-width="{style.borderWidth}px"
      style:border-radius="{style.borderRadius}px"
      style:border-style={style.borderStyle}
      style:padding="{style.padding}px"
    >
      {#key couplet.text}
        <div
          bind:this={coupletDiv}
          class="text"
          in:fly={{ y: 20 }}
          style:color={style.textColor}
          style:font-family={fontFamily(style, fonts)}
          style:text-shadow={style.textShadow || "none"}
        >
          {couplet.text}
        </div>
      {/key}

      {#if qr}
        <div bind:this={qrDiv} class="qr">
          <QrCode data={import.meta.env.VITE_QR_URL} />
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .outer {
    height: 100vh;
    width: 100vw;
    box-sizing: border-box;

    font-weight: 700;
    color: white;

    position: absolute;
    top: 0;
    left: 0;

    display: flex;
    justify-content: center;
    align-items: center;
  }

  .inner {
    display: flex;
    align-items: center;
    justify-content: center;
    flex-direction: column;
    box-sizing: border-box;

    height: 100%;
    width: 100%;
  }

  .text {
    white-space: pre;
    text-wrap: wrap;
    text-align: center;
    font-weight: 700;
    box-sizing: border-box;
  }

  .qr {
    margin: 1rem;
    padding: 1rem;
    display: block;
    font-size: 0;

    background-color: white;
    border-radius: 0.5rem;
  }
</style>
