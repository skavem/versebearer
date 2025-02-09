<script lang="ts">
  import QrCode from "@castlenine/svelte-qrcode";
  import { fade, fly } from "svelte/transition";
  import type { IShownCouplet } from "../../types";

  let { couplet, qr }: { couplet: IShownCouplet | null; qr: boolean } =
    $props();

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
</script>

{#if couplet}
  <div class="outer" transition:fade={{ duration: 600 }}>
    <div class="inner" bind:this={innerDiv}>
      {#key couplet.text}
        <div bind:this={coupletDiv} class="text" in:fly={{ y: 20 }}>
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
    padding: 4rem;
    box-sizing: border-box;

    height: 100%;
    width: 100%;

    background-color: rgb(0 0 0 / 95%);
  }

  .text {
    white-space: pre;
    text-wrap: wrap;
    text-align: center;
    font-weight: 700;
    color: white;
    font-family: "Century Gothic";
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
