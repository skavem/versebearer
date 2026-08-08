/**
 * Разбор клавиатуры, общий для вкладок «Библия» и «Песни». Обработчик каждая
 * вкладка ставит свой — здесь только предикаты, которые обязаны совпадать.
 */

/**
 * Ctrl+F / Ctrl+L — встать в строку поиска, не трогая мышь.
 *
 * Сравниваем по `code`, а не по `key`: `key` отдаёт символ текущей раскладки, и
 * на русской Ctrl+F приходит как «а» — сочетание не срабатывало.
 */
export function isSearchShortcut(e: KeyboardEvent): boolean {
  return (e.ctrlKey || e.metaKey) && (e.code === "KeyF" || e.code === "KeyL");
}

/**
 * Курсор в поле ввода — навигация принадлежит полю: иначе стрелки листали бы
 * главы или куплеты прямо во время набора запроса.
 */
export function isTypingTarget(e: KeyboardEvent): boolean {
  return (
    e.target instanceof HTMLInputElement ||
    e.target instanceof HTMLTextAreaElement
  );
}

/**
 * Событие пришло из модального окна. Обработчики вкладок висят на документе и
 * продолжают работать, пока открыт диалог, — а перехватывать в нём клавиши
 * нельзя: Ctrl+F увёл бы фокус в строку поиска за спиной диалога.
 */
export function isFromModal(e: KeyboardEvent): boolean {
  return !!(e.target as HTMLElement | null)?.closest?.(".modal");
}
