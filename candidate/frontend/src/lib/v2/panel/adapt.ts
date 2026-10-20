/**
 * From what GET /v2/panel/config sends to what the panel holds. Kept apart so
 * it can be checked in Node, and so a change to the API's shape has one place
 * to land.
 */

import type { ActivityItem, Diagram, Hypothesis, PanelConfig, PanelTab, ReviewComment, ReviewFile, Resource } from "./types";

export interface ApiConfig {
  sessionId: string;
  assessmentTitle?: string;
  taskTitle?: string;
  role: string;
  product: string;
  status: string;
  endsAt: string;
  tabs: { id: string; label: string; badge?: number }[];
  monitoring: { camera: boolean; screen: boolean; audio: boolean };
  avatar: { name: string; enabled: boolean };
  diagram: Diagram;
  notes: string;
  resources: Resource[];
  review: { title: string; files: ReviewFile[] | null; comments: ReviewComment[] | null };
  debug: { title: string; log: string[] | null; hypotheses: Hypothesis[] | null };
  test: { title: string; failing: string[] | null; passing: string[] | null; passed?: boolean };
  activity: ActivityApi[] | null;
  versions: Partial<Record<"diagram" | "notes" | "review" | "hypotheses", number>>;
}

export interface ActivityApi {
  id: number;
  at: string;
  severity: string;
  source: string;
  category?: string;
  text: string;
}

const TAB_IDS = new Set(["design", "notes", "resources", "review", "debug", "test", "activity"]);

export function fromApi(c: ApiConfig): { config: PanelConfig; versions: Record<"diagram" | "notes" | "review" | "hypotheses", number> } {
  const tabs = (c.tabs ?? []).filter((t) => TAB_IDS.has(t.id)) as PanelTab[];
  const activity: ActivityItem[] = (c.activity ?? []).map((a) => ({
    id: String(a.id),
    at: a.at,
    severity: a.severity === "flag" || a.severity === "notice" ? a.severity : "info",
    source: a.source === "watcher" || a.source === "panel" ? a.source : "server",
    category: a.category === "observed" || a.category === "verified" || a.category === "signal" ? a.category : undefined,
    text: a.text,
  }));
  const config: PanelConfig = {
    sessionId: c.sessionId,
    assessmentTitle: c.taskTitle || c.assessmentTitle || "Assessment",
    status: c.status,
    role: c.role,
    product: c.product,
    endsAt: c.endsAt,
    tabs,
    monitoring: c.monitoring,
    avatar: c.avatar,
    diagram: { title: c.diagram?.title ?? "Untitled Diagram", nodes: c.diagram?.nodes ?? [], edges: c.diagram?.edges ?? [], groups: c.diagram?.groups },
    notes: c.notes ?? "",
    resources: c.resources ?? [],
    review: { title: c.review.title, files: c.review.files ?? [], comments: c.review.comments ?? [] },
    debug: { title: c.debug.title, log: c.debug.log ?? [], hypotheses: c.debug.hypotheses ?? [] },
    test: { title: c.test.title, failing: c.test.failing ?? [], passing: c.test.passing ?? [], passed: c.test.passed },
    activity,
  };
  const v = c.versions ?? {};
  return { config, versions: { diagram: v.diagram ?? 0, notes: v.notes ?? 0, review: v.review ?? 0, hypotheses: v.hypotheses ?? 0 } };
}
