import type { Match } from "$lib/bindings/changeme/backend/search/models";

export type Segment = { text: string; hit: boolean };

/**
 * Режет текст на подсвеченные и обычные куски.
 *
 * `Match.start`/`len` приходят с Go уже в единицах UTF-16 — то есть в том же
 * счёте, которым режет slice, — поэтому применяются к строке напрямую. См.
 * комментарий к типу Match на бэкенде: считать их в символах было бы неверно
 * для куплетов, куда оператор может вставить эмодзи.
 */
export function splitHighlights(
  text: string,
  matches: Match[] | undefined | null,
): Segment[] {
  if (!matches?.length) return [{ text, hit: false }];

  // Совпадения приходят по порядку, но перекрытия и дубли теоретически
  // возможны (одно слово попало под два слова запроса) — сливаем, иначе куски
  // текста продублируются в выдаче.
  const merged: Match[] = [];
  for (const m of [...matches].sort((a, b) => a.start - b.start)) {
    const last = merged.at(-1);
    if (last && m.start <= last.start + last.len) {
      last.len = Math.max(last.len, m.start + m.len - last.start);
      continue;
    }
    merged.push({ start: m.start, len: m.len });
  }

  const out: Segment[] = [];
  let pos = 0;
  for (const m of merged) {
    if (m.start > pos) out.push({ text: text.slice(pos, m.start), hit: false });
    out.push({ text: text.slice(m.start, m.start + m.len), hit: true });
    pos = m.start + m.len;
  }
  if (pos < text.length) out.push({ text: text.slice(pos), hit: false });
  return out;
}
