<script lang="ts">
  import type { IShownCouplet } from "../../types";
  import QrCode from "@castlenine/svelte-qrcode";

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
    console.log(
      coupletDiv.clientHeight + (qrDiv ? getElHeight(qrDiv) : 0),
      getElHeight(innerDiv)
    );
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
    // console.log(coupletDiv.clientHeight, getElHeight(innerDiv), isOverflown());
  });
</script>

{#if couplet}
  <div class="outer">
    <div class="inner" bind:this={innerDiv}>
      <div bind:this={coupletDiv} class="text">
        {couplet.text}
      </div>
      {#if qr}
        <div bind:this={qrDiv} class="qr">
          <QrCode data="https://platiqr.ru/?uuid=1000093621" />
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .outer {
    height: 100vh;
    width: 100vw;
    padding: 2rem;
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

    border-radius: 0.75rem;
    background-color: rgb(0 0 0 / 85%);
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
