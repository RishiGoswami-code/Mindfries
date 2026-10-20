import type { PanelTab, PanelTabId } from "./types";

/**
 * Which tabs a candidate's panel shows, from the product's role manifest
 * (`v2/products/<p>/manifest.json` roles.<role>.modes) and the task's kind
 * (`_tasks/<t>/task.json` kind). Adding a role or a product is a manifest
 * change, not a code change (v2/PANEL.md §3).
 *
 * Notes, Resources and Activity are always there; the others follow the
 * modes the role is assessed in.
 */
const LABELS: Record<PanelTabId, string> = {
  design: "System Design",
  notes: "Notes",
  resources: "Resources",
  review: "Code Review",
  debug: "Debugging",
  test: "Testing",
  activity: "Activity",
};

/** Manifest modes (and task kinds) → the tab they need. */
const MODE_TAB: Record<string, PanelTabId> = {
  design: "design",
  architecture: "design",
  "system design": "design",
  review: "review",
  comprehension: "review",
  debugging: "debug",
  debug: "debug",
  reliability: "debug",
  testing: "test",
  test: "test",
};

/** The order tabs appear on the rail. */
const ORDER: PanelTabId[] = ["design", "notes", "resources", "review", "debug", "test", "activity"];

export function tabsFor(modes: string[], taskKind?: string): PanelTab[] {
  const want = new Set<PanelTabId>(["notes", "resources", "activity"]);
  for (const m of [...modes, ...(taskKind ? [taskKind] : [])]) {
    const tab = MODE_TAB[m.trim().toLowerCase()];
    if (tab) want.add(tab);
  }
  return ORDER.filter((id) => want.has(id)).map((id) => ({ id, label: LABELS[id] }));
}

/** The tab the panel opens on: the task's own mode first, else Notes. */
export function firstTab(tabs: PanelTab[], taskKind?: string): PanelTabId {
  const own = taskKind ? MODE_TAB[taskKind.toLowerCase()] : undefined;
  if (own && tabs.some((t) => t.id === own)) return own;
  for (const id of ["design", "debug", "review", "test"] as PanelTabId[]) {
    if (tabs.some((t) => t.id === id)) return id;
  }
  return "notes";
}
