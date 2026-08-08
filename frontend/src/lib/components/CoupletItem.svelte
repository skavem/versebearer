<script lang="ts" generics="T">
  import type { Snippet } from "svelte";
  import type { MouseEventHandler } from "svelte/elements";

  let {
    isActive,
    isShown,
    onclick,
    ondblclick,
    item,
    leftMark,
    rightMark,
    getName,
    multiline,
    oncontextmenu,
  }: {
    isActive: boolean;
    isShown: boolean;
    onclick: MouseEventHandler<HTMLDivElement> | null;
    ondblclick: MouseEventHandler<HTMLDivElement> | null;
    item: T;
    leftMark: Snippet<[T]>;
    rightMark: Snippet<[T]>;
    getName: (i: T) => string;
    multiline: boolean;
    oncontextmenu?: MouseEventHandler<HTMLDivElement>;
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
    "group/item flex cursor-pointer flex-row items-center justify-between gap-2 rounded border-2 p-2 transition-colors",
    // primary, а не neutral: в тёмной теме neutral почти сливается с фоном.
    isActive ? "border-primary" : "border-transparent",
    // Заливка одна, «в эфире» перебивает «выбрано»: у двух bg-* одного веса
    // побеждает порядок в CSS, а не порядок в этой строке.
    isShown ? "bg-secondary/20" : isActive && "bg-primary/10",
    // Подсветка при наведении непрозрачная и в CSS идёт последней, так что
    // затёрла бы обе заливки — а для показанной, но не выбранной строки
    // заливка это единственная метка эфира.
    !isActive && !isShown && "hover:bg-base-200",
  ]}
  {onclick}
  {ondblclick}
  {oncontextmenu}
>
  <div
    class="flex"
    class:flex-col={multiline}
    class:gap-1={multiline}
    class:gap-2={!multiline}
    class:items-center={!multiline}
  >
    {#if leftMark}
      {@render leftMark(item)}
    {/if}

    <span class:whitespace-pre={multiline}>
      {getName(item)}
    </span>
  </div>

  {#if rightMark}
    {@render rightMark(item)}
  {/if}
</div>
