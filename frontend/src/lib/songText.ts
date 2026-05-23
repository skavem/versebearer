import type { Couplet } from "$lib/bindings/changeme/backend/models";

export function serializeSongText(couplets: Couplet[]): string {
  return couplets.map((c) => `${c.label}\n${c.text}`).join("\n\n");
}

export function parseSongText(
  raw: string,
): { label: string; text: string }[] {
  const trimmed = raw.split(/\r?\n/).map((l) => l.trim());

  const collapsed: string[] = [];
  let emptyRun = 0;
  for (const l of trimmed) {
    if (l === "") {
      emptyRun++;
      if (emptyRun <= 1) collapsed.push(l);
    } else {
      emptyRun = 0;
      collapsed.push(l);
    }
  }

  while (collapsed.length && collapsed[collapsed.length - 1] === "") {
    collapsed.pop();
  }

  const blocks: string[][] = [[]];
  for (const l of collapsed) {
    if (l === "") {
      if (blocks[blocks.length - 1].length > 0) blocks.push([]);
    } else {
      blocks[blocks.length - 1].push(l);
    }
  }

  return blocks
    .filter((b) => b.length > 0)
    .map((b) => ({
      label: b[0],
      text: b.slice(1).join("\n"),
    }));
}
