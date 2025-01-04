<script lang="ts" generics="T extends { ID: number }">
  import MuiIcon from "./MuiIcon.svelte";

  let {
    items = $bindable(),
    getName,
    activeItem,
    setActiveItem,
    getSearchLabel,
  }: {
    items: T[];
    activeItem: T | null;
    setActiveItem: (item: T) => void;
    getName: (item: T) => string;
    getSearchLabel?: (item: T) => string;
  } = $props();

  getSearchLabel ??= getName;

  let inputValue = $state("");
  let shownItems = $derived.by(() =>
    items.filter((i) =>
      getSearchLabel(i)
        .toLocaleLowerCase()
        .includes(inputValue.toLocaleLowerCase()),
    ),
  );
  let inputEl = $state<HTMLInputElement | null>(null);

  let selectEl = $state<HTMLDivElement | null>(null);
  let selectElHeight = $state(0);
  let selectElWidth = $state(0);

  let isActive = $state(false);
  $effect(() => {
    const onclick = () => {
      isActive = false;
    };
    if (isActive) {
      document.addEventListener("click", onclick);
    }
    return () => document.removeEventListener("click", onclick);
  });

  let windowHeight = $state(0);
  let selectElRect = $derived<DOMRect>(
    selectEl?.getBoundingClientRect() ?? {
      x: 0,
      y: 0,
      bottom: 0,
      height: 0,
      left: 0,
      right: 0,
      top: 0,
      width: 0,
      toJSON: () => "",
    },
  );
</script>

<svelte:window bind:innerHeight={windowHeight} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="relative flex h-12 items-center justify-start gap-2 rounded bg-zinc-100"
  bind:this={selectEl}
  bind:clientHeight={selectElHeight}
  bind:clientWidth={selectElWidth}
  onclick={(e) => {
    if (isActive) return;
    isActive = true;
    e.stopPropagation();
  }}
>
  <input
    class="h-full w-full min-w-0 bg-transparent p-2 font-normal outline-none"
    bind:value={inputValue}
    bind:this={inputEl}
    onkeydown={(e) => {
      if (e.code === "Escape") {
        inputEl?.blur();
        inputValue = "";
        isActive = false;
        e.stopPropagation();
      }
    }}
    placeholder={activeItem ? getName(activeItem) : ""}
  />
  <MuiIcon name="expand_more" classes={{ "rotate-180": isActive }} />

  <div
    class={[
      "absolute left-0 z-10 select-none overflow-y-scroll rounded bg-white",
      {
        invisible: !isActive,
        visible: isActive,
      },
    ]}
    style={`top: ${selectElHeight}px; width: ${selectElWidth}px; max-height: calc(${windowHeight - selectElRect.top - selectElHeight}px - 1rem)`}
  >
    {#each shownItems as item}
      <div
        class="p-2 hover:bg-gray-200"
        onclick={(e) => {
          setActiveItem(item);
          isActive = false;
          e.stopPropagation();
        }}
      >
        {getName(item)}
      </div>
    {/each}
  </div>
</div>
