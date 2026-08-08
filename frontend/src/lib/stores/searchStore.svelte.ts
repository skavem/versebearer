import type { ReferenceHit } from "$lib/bindings/changeme";
import {
  ParseReference,
  SearchCouplets,
  SearchVerses,
} from "$lib/bindings/changeme/dbhandler";
import { createDebouncer } from "$lib/debounce";
import { Events } from "@wailsio/runtime";
import { BibleStore } from "./BibleStore.svelte";

/**
 * Состояние построения индекса. При первом запуске (и после импорта перевода)
 * индекс собирается несколько секунд, и до готовности поиск честно ничего не
 * находит — без этого признака оператор решил бы, что поиск сломан.
 */
function createIndexState() {
  let building = $state(false);
  let done = $state(0);
  let total = $state(0);

  Events.On(
    "search_index_progress",
    ({ data }: { data: { done: number; total: number } }) => {
      building = data.done < data.total;
      done = data.done;
      total = data.total;
    },
  );
  Events.On("search_index_ready", () => {
    building = false;
  });

  return {
    get building() {
      return building;
    },
    get done() {
      return done;
    },
    get total() {
      return total;
    },
    /** Текст для пустой выдачи, пока индекс ещё строится. */
    get label() {
      return total > 0
        ? `Идёт первичная индексация — ${done} из ${total}…`
        : "Идёт первичная индексация…";
    },
  };
}

export const searchIndex = createIndexState();

/** Сколько результатов запрашиваем у бэкенда. Дальше первых полусотни
 * оператор всё равно не смотрит: не нашлось — проще уточнить запрос. */
const LIMIT = 50;

/** Пауза перед запросом, пока оператор печатает. Поиск занимает доли
 * миллисекунды, так что задержка нужна только чтобы не дёргать бэкенд на
 * каждую букву. */
const TYPING_DELAY = 150;

/**
 * Общая механика строки поиска: текст запроса, дебаунс, гонка ответов,
 * выбранная строка выдачи.
 *
 * `run` возвращает результаты для текущего текста. Ответы нумеруются: пока
 * летит запрос, оператор успевает дописать букву, и ответ на старый текст
 * приходит последним — без счётчика он затёр бы актуальную выдачу.
 */
function createSearch<T>(run: (query: string) => Promise<T[]>) {
  const debouncer = createDebouncer();

  let query = $state("");
  let results = $state<T[]>([]);
  let loading = $state(false);
  let active = $state<T | null>(null);
  let seq = 0;

  const execute = async (text: string) => {
    const mine = ++seq;
    if (!text.trim()) {
      results = [];
      active = null;
      loading = false;
      return;
    }
    loading = true;
    try {
      const found = await run(text);
      if (mine !== seq) return; // ответ устарел, пришёл уже более свежий
      results = found;
      active = found.at(0) ?? null;
    } finally {
      if (mine === seq) loading = false;
    }
  };

  return {
    get query() {
      return query;
    },
    set query(value: string) {
      query = value;
      debouncer.debounce("search", () => execute(value), TYPING_DELAY);
    },
    get results() {
      return results;
    },
    get loading() {
      return loading;
    },
    get active() {
      return active;
    },
    set active(value: T | null) {
      active = value;
    },
    /** Пустой ли запрос — по этому признаку интерфейс решает, показывать
     * обычный список или выдачу поиска. */
    get isEmpty() {
      return query.trim().length === 0;
    },
    clear() {
      query = "";
      results = [];
      active = null;
      seq++;
    },
    /** Перебор выдачи стрелками. */
    step(delta: number) {
      if (!results.length) return;
      const i = active ? results.indexOf(active) : -1;
      const next = Math.min(Math.max(i + delta, 0), results.length - 1);
      active = results[next];
    },
  };
}

/**
 * Поиск по стихам. Кроме полнотекстовой выдачи держит разобранную ссылку:
 * «Ин 3:16» — это не поиск по словам, а прямой переход, и показывается он
 * отдельной строкой над результатами.
 */
function createVerseSearch() {
  const base = createSearch((q) =>
    SearchVerses(q, BibleStore.translations.active?.ID ?? 0, LIMIT),
  );

  let reference = $state<ReferenceHit | null>(null);
  const debouncer = createDebouncer();

  // Свойства перечисляются вручную, а не спредом: спред вычислил бы геттеры
  // один раз при создании объекта и превратил реактивное состояние в
  // застывшие значения.
  return {
    get results() {
      return base.results;
    },
    get loading() {
      return base.loading;
    },
    get active() {
      return base.active;
    },
    set active(value) {
      base.active = value;
    },
    get isEmpty() {
      return base.isEmpty;
    },
    step(delta: number) {
      base.step(delta);
    },
    get query() {
      return base.query;
    },
    set query(value: string) {
      base.query = value;
      debouncer.debounce(
        "ref",
        async () => {
          const translationId = BibleStore.translations.active?.ID;
          if (!value.trim() || !translationId) {
            reference = null;
            return;
          }
          reference = await ParseReference(value, translationId);
        },
        TYPING_DELAY,
      );
    },
    get reference() {
      return reference;
    },
    clear() {
      base.clear();
      reference = null;
    },
  };
}

export const verseSearch = createVerseSearch();
export const coupletSearch = createSearch((q) => SearchCouplets(q, LIMIT));
