import type { ShownVerse } from "$lib/bindings/changeme";
import {
  Chapter,
  Translation,
  Verse,
  type Book,
} from "$lib/bindings/changeme/backend/models";
import {
  GetBooks,
  GetChapters,
  GetShownVerse,
  GetTranslations,
  GetVerses,
} from "$lib/bindings/changeme/dbhandler";
import { Events } from "@wailsio/runtime";
import { cycleIndex } from "./cycle";

const createBibleStore = () => {
  let translationsLoading = $state(true);
  let translationsList = $state<Translation[]>([]);
  let activeTranslation = $state<Translation | null>(null);

  let booksLoading = $state(true);
  let booksList = $state<Book[]>([]);
  let activeBook = $state<Book | null>(null);

  let chaptersLoading = $state(true);
  let chaptersList = $state<Chapter[]>([]);
  let activeChapter = $state<Chapter | null>(null);

  let versesLoading = $state(true);
  let versesList = $state<Verse[]>([]);
  let activeVerse = $state<Verse | null>(null);

  const historyVerses = $state<ShownVerse[]>([]);
  let activeHistoryVerse = $state<ShownVerse | null>(null);

  let shownVerse = $state<ShownVerse | null>(null);
  const showVerse = (v: ShownVerse) => {
    shownVerse = v;
    historyVerses.push(v);
  };
  const hideVerse = () => {
    shownVerse = null;
  };
  Events.On("show_verse", ({ data }: { data: ShownVerse }) => {
    showVerse(data);
  });
  Events.On("hide_verse", () => {
    hideVerse();
  });
  // Импорт/удаление перевода в настройках — подхватываем новый список.
  Events.On("translations_update", () => {
    BibleStore.translations.reload().catch(console.error);
  });

  // Каскад книга→глава→стих запускают сразу несколько источников: смена
  // перевода, выбор книги, поиск, история. Токен поколения не даёт
  // запоздавшему ответу затереть более свежий переход.
  let cascadeGeneration = 0;
  const startCascade = () => ++cascadeGeneration;
  const isStale = (generation: number) => generation !== cascadeGeneration;

  type Picker<T> = (items: T[]) => T | undefined;
  // Чем выбрать главу и стих, если первый элемент списка не подходит.
  type Picks = { chapter?: Picker<Chapter>; verse?: Picker<Verse> };

  // Ближайший элемент с номером не больше запрошенного: в другом переводе глав
  // или стихов бывает меньше. Списки приходят отсортированными по номеру.
  const nearestByNumber = <T extends { number: number }>(
    items: T[],
    max?: number,
  ) => (max === undefined ? undefined : items.findLast((i) => i.number <= max));

  // Бэкенд отдаёт детей первого элемента вместе со списком — не тянем повторно.
  const chaptersOf = async (book: Book) =>
    book.chapters?.length ? book.chapters : await GetChapters(book.ID);
  const versesOf = async (chapter: Chapter) =>
    chapter.verses?.length ? chapter.verses : await GetVerses(chapter.ID);

  const openChapter = async (
    chapter: Chapter | null,
    generation: number,
    picks: Picks = {},
  ) => {
    if (isStale(generation)) return;
    activeChapter = chapter;
    versesLoading = true;
    try {
      const newVerses = chapter ? await versesOf(chapter) : [];
      if (isStale(generation)) return;

      versesList = newVerses;
      activeVerse = picks.verse?.(newVerses) ?? newVerses.at(0) ?? null;
    } catch (err) {
      console.error(err);
    } finally {
      if (!isStale(generation)) versesLoading = false;
    }
  };

  const openBook = async (
    book: Book | null,
    generation: number,
    picks: Picks = {},
  ) => {
    if (isStale(generation)) return;
    activeBook = book;
    chaptersLoading = true;
    versesLoading = true;
    try {
      const newChapters = book ? await chaptersOf(book) : [];
      if (isStale(generation)) return;

      chaptersList = newChapters;
      chaptersLoading = false;
      const chapter = picks.chapter?.(newChapters) ?? newChapters.at(0) ?? null;
      await openChapter(chapter, generation, picks);
    } catch (err) {
      console.error(err);
    } finally {
      if (!isStale(generation)) {
        chaptersLoading = false;
        versesLoading = false;
      }
    }
  };

  // «Быт» и «Быт.» — одна книга: точки, регистр и пробелы между файлами не
  // согласованы.
  const sameShortName = (a: string, b: string) =>
    a.replaceAll(".", "").trim().toLowerCase() ===
    b.replaceAll(".", "").trim().toLowerCase();

  // Та же книга в другом переводе: сначала по сокращению, потом по номеру —
  // сокращения расходятся между языками («Быт» / «Gen»), а номер это позиция
  // книги в файле, и она совпадает у переводов одинакового состава.
  const sameBook = (books: Book[], kept: Book) =>
    books.find((b) => sameShortName(b.shortName, kept.shortName)) ??
    books.find((b) => b.number === kept.number);

  const openTranslation = async (translation: Translation | null) => {
    // Позицию снимаем до загрузки: в новом переводе встаём на то же место.
    const keptBook = activeBook;
    const keptChapterNumber = activeChapter?.number;
    const keptVerseNumber = activeVerse?.number;

    activeTranslation = translation;
    const generation = startCascade();

    if (!translation) {
      booksList = [];
      booksLoading = false;
      await openBook(null, generation);
      return;
    }

    booksLoading = true;
    try {
      const newBooks = await GetBooks(translation.ID);
      if (isStale(generation)) return;

      booksList = newBooks;
      booksLoading = false;

      const book = keptBook ? sameBook(newBooks, keptBook) : undefined;
      // Главу и стих восстанавливаем только внутри найденной книги — иначе их
      // номера увели бы в случайное место чужой книги.
      const picks: Picks = book
        ? {
            chapter: (cs) => nearestByNumber(cs, keptChapterNumber),
            verse: (vs) => nearestByNumber(vs, keptVerseNumber),
          }
        : {};

      await openBook(book ?? newBooks.at(0) ?? null, generation, picks);
    } catch (err) {
      console.error(err);
    } finally {
      if (!isStale(generation)) booksLoading = false;
    }
  };

  const translations = {
    get loading() {
      return translationsLoading;
    },
    /** Перечитывает список после импорта/удаления перевода в настройках. */
    async reload() {
      const fresh = await GetTranslations();
      const keep = fresh.find((t) => t.ID === activeTranslation?.ID);
      translationsList = fresh;
      translationsLoading = false;
      if (keep) {
        activeTranslation = keep;
        return;
      }
      translations.active = fresh.at(0) ?? null;
    },
    get list() {
      return translationsList;
    },
    set list(val) {
      translationsList = val;
      translationsLoading = false;
      activeTranslation = val.at(0) ?? null;

      booksList = activeTranslation?.books ?? [];
      booksLoading = false;
      openBook(booksList.at(0) ?? null, startCascade());
    },
    get active() {
      return activeTranslation;
    },
    set active(val) {
      openTranslation(val);
    },
  };

  const books = {
    get loading() {
      return booksLoading;
    },
    get list() {
      return booksList;
    },
    get active() {
      return activeBook;
    },
    set active(val) {
      openBook(val, startCascade());
    },
  };

  const chapters = {
    get loading() {
      return chaptersLoading;
    },
    get list() {
      return chaptersList;
    },
    get active() {
      return activeChapter;
    },
    set active(val) {
      openChapter(val, startCascade());
    },

    next() {
      const n = cycleIndex(chaptersList, activeChapter, 1);
      if (n) chapters.active = n;
    },
    prev() {
      const n = cycleIndex(chaptersList, activeChapter, -1);
      if (n) chapters.active = n;
    },
  };

  const verses = {
    get loading() {
      return versesLoading;
    },
    get list() {
      return versesList;
    },
    get active() {
      return activeVerse;
    },
    set active(val) {
      activeVerse = val;
    },

    next() {
      const n = cycleIndex(versesList, activeVerse, 1);
      if (n) this.active = n;
    },
    prev() {
      const n = cycleIndex(versesList, activeVerse, -1);
      if (n) this.active = n;
    },

    get shown() {
      return shownVerse;
    },
    set shown(v) {
      if (v) {
        showVerse(v);
      } else {
        hideVerse();
      }
    },
  };

  /**
   * Прямой переход к месту — по разобранной ссылке («Ин 3:16»), по строке
   * выдачи поиска или из истории показов.
   */
  const navigate = {
    async goTo(book: Book, chapter: Chapter, verseId?: number) {
      await openBook(book, startCascade(), {
        chapter: (cs) => cs.find((c) => c.ID === chapter.ID),
        verse: (vs) => vs.find((v) => v.ID === verseId),
      });
    },
  };

  const history = {
    get list() {
      return historyVerses.toReversed();
    },
    get active() {
      return activeHistoryVerse;
    },
    set active(v) {
      activeHistoryVerse = v;
    },
    async restore(v: ShownVerse) {
      activeHistoryVerse = v;
      await navigate.goTo(v.Book, v.Chapter, v.ID);
    },
  };

  return { translations, books, chapters, verses, history, navigate };
};

GetTranslations()
  .then((tr) => (BibleStore.translations.list = tr))
  .catch(console.error);
GetShownVerse().then((v) => (BibleStore.verses.shown = v));

export const BibleStore = createBibleStore();
