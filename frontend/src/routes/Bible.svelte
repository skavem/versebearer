<script lang="ts">
  import type { VerseSearchHit } from "$lib/bindings/changeme";
  import { Book } from "$lib/bindings/changeme/backend/models/models";
  import { HideVerse, ShowVerse } from "$lib/bindings/changeme/dbhandler";
  import List from "$lib/components/List.svelte";
  import MuiIcon from "$lib/components/MuiIcon.svelte";
  import SearchSelect from "$lib/components/SearchSelect.svelte";
  import Select from "$lib/components/Select.svelte";
  import VerseList from "$lib/components/VerseList.svelte";
  import { isFromModal, isSearchShortcut, isTypingTarget } from "$lib/keyboard";
  import { BibleStore } from "$lib/stores/BibleStore.svelte";
  import { verseSearch } from "$lib/stores/searchStore.svelte";

  let translations = $derived(BibleStore.translations);
  let books = $derived(BibleStore.books);
  let chapters = $derived(BibleStore.chapters);
  let verses = $derived(BibleStore.verses);
  let history = $derived(BibleStore.history);
  let shown = $derived(verses.shown);

  let searchField = $state<ReturnType<typeof SearchSelect> | null>(null);

  const showVerse = () => {
    const activeId = verses.active?.ID;
    if (activeId) {
      ShowVerse(activeId);
    }
  };

  /** Переход к найденному стиху: выбираем его в основном каскаде, чтобы
   * дальше работали привычные стрелки и «Показать стих».
   *
   * Поиск после выбора сбрасывается — выдача своё дело сделала, а список
   * стихов главы уже стоит на нужном месте. */
  const selectHit = async (hit: VerseSearchHit) => {
    verseSearch.clear();
    await BibleStore.navigate.goTo(hit.Book, hit.Chapter, hit.ID);
  };

  const goToReference = async () => {
    const reference = verseSearch.reference;
    if (!reference) return;
    verseSearch.clear();
    await BibleStore.navigate.goTo(
      reference.book,
      reference.chapter,
      reference.hasVerse ? reference.verse.ID : undefined,
    );
  };

  /** Enter в строке поиска: перейти и сразу вывести на экран. Разобранная
   * ссылка приоритетнее выдачи — набравший «Ин 3:16» хочет туда, а не в
   * список совпадений. */
  const submitSearch = async () => {
    if (verseSearch.reference) {
      await goToReference();
      showVerse();
      return;
    }
    const hit = verseSearch.active;
    if (hit) {
      await selectHit(hit);
      showVerse();
    }
  };

  $effect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (isFromModal(e)) return;

      if (isSearchShortcut(e)) {
        searchField?.focus();
        e.preventDefault();
        return;
      }

      if (isTypingTarget(e)) return;

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
          e.preventDefault();
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

  <div class="flex w-0 flex-grow flex-col gap-2">
    <div class="flex flex-col h-2/3 w-full min-h-0 gap-2">
      <SearchSelect
        bind:this={searchField}
        bind:query={verseSearch.query}
        loading={verseSearch.loading}
        placeholder="Поиск по тексту или ссылка — «Ин 3:16»"
        items={verseSearch.results}
        activeItem={verseSearch.active}
        getText={(i) => i.text}
        getMatches={(i) => i.matches}
        getBadge={(i) => `${i.Book.shortName} ${i.Chapter.number}:${i.number}`}
        onSelect={selectHit}
        onNavigate={(d) => verseSearch.step(d)}
        onSubmit={submitSearch}
        referenceLabel={verseSearch.reference?.ref}
        onReference={goToReference}
      />

      <VerseList
        onClick={(i) => (verses.active = i)}
        onDoubleClick={(i) => {
          if (i.ID !== shown?.ID) {
            showVerse();
          } else {
            HideVerse();
          }
        }}
      ></VerseList>

      <div class="flex justify-center">
        <button
          class={[
            "btn btn-wide",
            // Янтарь = «в эфире»; красный зарезервирован за разрушительными.
            shown ? "btn-outline btn-secondary" : "btn-neutral",
          ]}
          onclick={() => {
            if (shown) {
              HideVerse();
            } else {
              showVerse();
            }
          }}
        >
          <MuiIcon name={shown ? "visibility_off" : "visibility"} />
          {shown ? "Скрыть стих" : "Показать стих"}
        </button>
      </div>
    </div>

    <div class="h-1/3 w-full">
      <List
        items={history.list}
        getName={(i) => i.text}
        onClick={history.restore}
        onDoubleClick={(v) => ShowVerse(v.ID)}
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
</div>
