/**
 * Время во вкладке «Звук» показывается ровно в двух видах, и оба живут здесь,
 * чтобы «м:сс» не разъезжался между списком, мини-плеером и правкой обрезки.
 */

/**
 * Момент внутри трека — позиция, остаток, границы обрезки. Ноль здесь
 * осмыслен ("0:00" — начало), отрицательное время показывается как ноль.
 */
export function formatMinSec(ms: number): string {
  const totalSec = Math.max(0, Math.round(ms / 1000));
  const min = Math.floor(totalSec / 60);
  const sec = totalSec % 60;
  return `${min}:${sec.toString().padStart(2, "0")}`;
}

/**
 * Длительность трека — "—", если она неизвестна (durationMs<=0): прочерк
 * честнее, чем "0:00", который читается как «пустой файл».
 */
export function formatDuration(ms: number): string {
  return !ms || ms <= 0 ? "—" : formatMinSec(ms);
}
