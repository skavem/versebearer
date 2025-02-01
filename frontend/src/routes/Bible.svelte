<script lang="ts">
  import { Book } from "$lib/bindings/changeme/backend/models";
  import { HideVerse, ShowVerse } from "$lib/bindings/changeme/dbhandler";
  import List from "$lib/components/List.svelte";
  import MuiIcon from "$lib/components/MuiIcon.svelte";
  import Select from "$lib/components/Select.svelte";
  import { BibleStore } from "$lib/stores/BibleStore.svelte";

  let translations = $derived(BibleStore.translations);
  let books = $derived(BibleStore.books);
  let chapters = $derived(BibleStore.chapters);
  let verses = $derived(BibleStore.verses);
  let history = $derived(BibleStore.history);
  let shown = $derived(verses.shown);

  const showVerse = () => {
    const activeId = verses.active?.ID;
    if (activeId) {
      ShowVerse(activeId);
    }
  };

  $effect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      switch (e.code) {
        case "Escape":
          HideVerse();
          e.preventDefault();
          return;
        case "Enter":
          showVerse();
          e.preventDefault();
          return;
        case "ArrowDown":
          verses.next();
          e.preventDefault();
          return;
        case "ArrowUp":
          verses.prev();
          e.preventDefault();
          return;
        case "ArrowLeft":
          chapters.prev();
          e.preventDefault;
          return;
        case "ArrowRight":
          chapters.next();
          e.preventDefault();
          return;
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  });
</script>

<div class="flex h-[calc(100vh-4rem)] flex-row gap-2 p-4">
  <div class="flex flex-col gap-2">
    <Select
      bind:items={translations.list}
      getName={(i) => i.name}
      activeItem={translations.active}
      setActiveItem={(i) => (translations.active = i)}
    />
    <Select
      bind:items={books.list}
      getName={(i) => i.title}
      activeItem={books.active}
      setActiveItem={(i) => (books.active = i)}
    />
    <List
      items={books.list.flatMap((b) => {
        const bs = [b];
        if (b.dividerBefore) {
          return [
            {
              ID: -1,
              title: b.dividerBefore,
            } as Book,
            b,
          ];
        }
        return bs;
      })}
      getName={(i) => i.title}
      onClick={(i) => (books.active = i)}
      activeItem={books.active}
    ></List>
  </div>

  <div class="w-20">
    <List
      items={chapters.list}
      getName={(i) => i.number.toString()}
      onClick={(i) => (chapters.active = i)}
      activeItem={chapters.active}
    />
  </div>

  <div class="flex flex-grow flex-col gap-2">
    <List
      items={verses.list}
      getName={(i) => i.text}
      onClick={(i) => (verses.active = i)}
      onDoubleClick={(i) => {
        if (i.ID !== shown?.ID) {
          showVerse();
        } else {
          HideVerse();
        }
      }}
      activeItem={verses.active}
    >
      {#snippet rightMark(i)}
        {#if i.ID === shown?.ID}
          <MuiIcon name={"remove_red_eye"} />
        {/if}
      {/snippet}
      {#snippet leftMark(i)}
        <div class="badge badge-neutral badge-md">
          {i.number.toString()}
        </div>
      {/snippet}
    </List>

    <div class="flex justify-center gap-2">
      <button
        class="btn btn-neutral btn-sm"
        onclick={() => {
          if (shown) {
            HideVerse();
          } else {
            showVerse();
          }
        }}
      >
        <MuiIcon name={shown ? "visibility_off" : "visibility"} />
        {shown ? "СКРЫТЬ" : "ПОКАЗАТЬ"}
      </button>
    </div>

    <List
      items={history.list}
      getName={(i) => i.text}
      onClick={history.restore}
      activeItem={history.active}
      getKey={(_, i) => i.toString()}
    >
      {#snippet leftMark(i)}
        <span class="badge badge-neutral badge-md text-nowrap">
          {`${i.Book.shortName} ${i.Chapter.number}:${i.number}`}
        </span>
      {/snippet}
    </List>
  </div>
</div>
