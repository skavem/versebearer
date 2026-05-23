export function cycleIndex<T extends { ID: number }>(
  list: T[],
  active: T | null,
  delta: 1 | -1,
): T | undefined {
  const ind = list.findIndex((v) => v.ID === active?.ID);
  if (ind === -1) return undefined;
  const next = ind + delta;
  if (next < 0 || next >= list.length) return undefined;
  return list[next];
}
