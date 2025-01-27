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
  GetTranslations,
  GetVerses,
} from "$lib/bindings/changeme/dbhandler";
import { Events } from "@wailsio/runtime";

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
  Events.On("show_verse", ({ data }: { data: ShownVerse[] }) => {
    showVerse(data[0]);
  });
  Events.On("hide_verse", () => {
    hideVerse();
  });

  const translations = {
    get loading() {
      return translationsLoading;
    },
    get list() {
      return translationsList;
    },
    set list(val) {
      translationsList = val;
      translationsLoading = false;
      activeTranslation = translationsList.at(0) || null;

      booksList = translationsList.at(0)?.books || [];
      activeBook = booksList.at(0) || null;

      chaptersList = booksList.at(0)?.chapters || [];
      activeChapter = chaptersList.at(0) || null;

      versesList = chaptersList.at(0)?.verses || [];
      activeVerse = versesList.at(0) || null;
    },
    get active() {
      return activeTranslation;
    },
    set active(val) {
      activeTranslation = val;

      booksLoading = true;
      chaptersLoading = true;
      versesLoading = true;
      GetBooks(activeTranslation!.ID).then((newBooks) => {
        booksList = newBooks;
        activeBook = booksList.at(0) || null;
        booksLoading = false;

        chaptersList = booksList.at(0)?.chapters || [];
        activeChapter = chaptersList.at(0) || null;
        chaptersLoading = false;

        versesList = chaptersList.at(0)?.verses || [];
        activeVerse = versesList.at(0) || null;
        versesLoading = false;
      });
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
      activeBook = val;

      chaptersLoading = true;
      versesLoading = true;
      GetChapters(activeBook!.ID).then((newChapters) => {
        chaptersList = newChapters;
        activeChapter = chaptersList.at(0) || null;
        chaptersLoading = false;

        versesList = chaptersList.at(0)?.verses || [];
        activeVerse = versesList.at(0) || null;
        versesLoading = false;
      });
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
      activeChapter = val;

      versesLoading = true;
      GetVerses(activeChapter!.ID).then((newVerses) => {
        versesList = newVerses;
        activeVerse = versesList.at(0) || null;
        versesLoading = false;
      });
    },

    next() {
      const ind = chaptersList.findIndex((c) => c.ID === activeChapter?.ID);
      if (ind === -1 || ind === chaptersList.length - 1) return;
      chapters.active = chaptersList[ind + 1];
    },
    prev() {
      const ind = chaptersList.findIndex((c) => c.ID === activeChapter?.ID);
      if (ind === -1 || ind === 0) return;
      chapters.active = chaptersList[ind - 1];
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
      const ind = versesList.findIndex((v) => v.ID === activeVerse?.ID);
      if (ind === -1 || ind === versesList.length - 1) return;
      this.active = versesList[ind + 1];
    },
    prev() {
      const ind = versesList.findIndex((v) => v.ID === activeVerse?.ID);
      if (ind === -1 || ind === 0) return;
      this.active = versesList[ind - 1];
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
      activeBook = v.Book;
      chaptersList = await GetChapters(activeBook.ID);
      activeChapter = v.Chapter;
      versesList = await GetVerses(activeChapter.ID);
      activeVerse = v;
    },
  };

  return {
    translations,

    books,

    chapters,

    verses,

    history,
  };
};

GetTranslations()
  .then((tr) => (BibleStore.translations.list = tr))
  .catch(console.error);

export const BibleStore = createBibleStore();
