/**
 * A line diff between two versions of a file: what the task started with
 * and what the candidate left. Longest-common-subsequence, line by line.
 */

export type DiffLine =
  | { kind: "same"; before: number; after: number; text: string }
  | { kind: "added"; after: number; text: string }
  | { kind: "removed"; before: number; text: string };

export interface FileDiff {
  path: string;
  /** The task's version; null when the candidate created the file. */
  beforeExists: boolean;
  /** Null when the candidate deleted the file. */
  afterExists: boolean;
  added: number;
  removed: number;
  lines: DiffLine[];
  tooLarge: boolean;
}

const MAX_CELLS = 4_000_000;

const split = (text: string) => (text === "" ? [] : text.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n"));

export function diffText(path: string, before: string | null, after: string | null): FileDiff {
  const a = split(before ?? "");
  const b = split(after ?? "");
  const base = { path, beforeExists: before !== null, afterExists: after !== null };

  if (a.length * b.length > MAX_CELLS) {
    const inA = new Map<string, number>();
    for (const l of a) inA.set(l, (inA.get(l) ?? 0) + 1);
    let common = 0;
    for (const l of b) {
      const n = inA.get(l) ?? 0;
      if (n > 0) {
        common++;
        inA.set(l, n - 1);
      }
    }
    return { ...base, added: b.length - common, removed: a.length - common, lines: [], tooLarge: true };
  }

  // lcs[i][j]: length of the common subsequence of a[i..] and b[j..].
  const w = b.length + 1;
  const lcs = new Uint32Array((a.length + 1) * w);
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      lcs[i * w + j] = a[i] === b[j] ? lcs[(i + 1) * w + j + 1] + 1 : Math.max(lcs[(i + 1) * w + j], lcs[i * w + j + 1]);
    }
  }

  const lines: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      lines.push({ kind: "same", before: i + 1, after: j + 1, text: a[i] });
      i++;
      j++;
    } else if (lcs[(i + 1) * w + j] >= lcs[i * w + j + 1]) {
      lines.push({ kind: "removed", before: i + 1, text: a[i] });
      i++;
    } else {
      lines.push({ kind: "added", after: j + 1, text: b[j] });
      j++;
    }
  }
  for (; i < a.length; i++) lines.push({ kind: "removed", before: i + 1, text: a[i] });
  for (; j < b.length; j++) lines.push({ kind: "added", after: j + 1, text: b[j] });

  return {
    ...base,
    added: lines.filter((l) => l.kind === "added").length,
    removed: lines.filter((l) => l.kind === "removed").length,
    lines,
    tooLarge: false,
  };
}

/** Every file that differs between the two sets, most changed first. Paths are compared without a leading slash. */
export function diffFiles(before: Record<string, string>, after: Record<string, string>): FileDiff[] {
  const norm = (files: Record<string, string>) => new Map(Object.entries(files).map(([p, t]) => [p.replace(/^\/+/, ""), typeof t === "string" ? t : ""]));
  const a = norm(before);
  const b = norm(after);
  const paths = [...new Set([...a.keys(), ...b.keys()])];
  return paths
    .filter((p) => a.get(p) !== b.get(p))
    .map((p) => diffText(p, a.has(p) ? a.get(p)! : null, b.has(p) ? b.get(p)! : null))
    .sort((x, y) => y.added + y.removed - (x.added + x.removed) || x.path.localeCompare(y.path));
}
