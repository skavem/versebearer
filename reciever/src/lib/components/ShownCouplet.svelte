<script lang="ts">
  import type { IShownCouplet } from "../../types";

  let { couplet }: { couplet: IShownCouplet | null } = $props();

  let innerDiv = $state<null | HTMLDivElement>(null);
  let coupletDiv = $state<null | HTMLDivElement>(null);
  const getElHeight = (el: HTMLElement) => {
    var cs = getComputedStyle(el);
    var paddingY = parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom);
    var borderY =
      parseFloat(cs.borderTopWidth) + parseFloat(cs.borderBottomWidth);
    return el.offsetHeight - paddingY - borderY;
  };

  const isOverflown = () => {
    if (!coupletDiv || !innerDiv) return false;
    return coupletDiv.clientHeight > getElHeight(innerDiv);
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
    console.log(coupletDiv.clientHeight, getElHeight(innerDiv), isOverflown());
  });
</script>

{#if couplet}
  <div class="outer">
    <div class="inner" bind:this={innerDiv}>
      <div bind:this={coupletDiv} class="text">
        {couplet.text}
      </div>
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
</style>
