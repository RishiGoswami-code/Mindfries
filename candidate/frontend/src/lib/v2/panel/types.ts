/**
 * The V2 floating panel's data (v2/PANEL.md). Step 1 fills it with mock
 * data (./mock.ts); later steps load it from v2/api.
 */

/** The workspace tabs. Which ones a session gets comes from ./tabs.ts. */
export type PanelTabId = "design" | "notes" | "resources" | "review" | "debug" | "test" | "activity";

export interface PanelTab {
  id: PanelTabId;
  label: string;
  /** Unread flags or notifications, shown as a badge on the rail. */
  badge?: number;
}

export type ActivitySeverity = "info" | "notice" | "flag";

/** One line in the tracker: everything recorded about the session. */
export interface ActivityItem {
  id: string;
  at: string; // ISO time
  severity: ActivitySeverity;
  /** Where it came from: the CLI watcher, the panel, or the server. */
  source: "watcher" | "panel" | "server";
  /** The spec's categories for flags (§16): observed, verified, review signal. */
  category?: "observed" | "verified" | "signal";
  text: string;
  /** What the candidate said about it, in their own words (flags and notices). */
  response?: string;
}

export interface Resource {
  id: string;
  title: string;
  kind: "doc" | "link" | "file";
  detail: string;
  href?: string;
}

/** A diagram on the System Design canvas. */
export type NodeTone = "blue" | "purple" | "green" | "sky" | "amber" | "red" | "violet";

export interface DiagramNode {
  id: string;
  x: number;
  y: number;
  w: number;
  h: number;
  title: string;
  subtitle?: string;
  tone: NodeTone;
  icon: "client" | "balancer" | "lock" | "user" | "file" | "database" | "storage" | "cache" | "box";
}

export interface DiagramEdge {
  id: string;
  from: string;
  to: string;
  dashed?: boolean;
}

export interface Diagram {
  title: string;
  nodes: DiagramNode[];
  edges: DiagramEdge[];
  /** A dashed group drawn around these nodes (the mock-up's service tier). */
  groups?: { id: string; nodes: string[] }[];
}

/** A file in the review / debug / test tabs: a diff to read and comment on. */
export interface ReviewFile {
  path: string;
  lines: { kind: " " | "+" | "-"; text: string; n: number }[];
}

/** A comment on a line of the code under review. */
export interface ReviewComment {
  file: string;
  line: number;
  text: string;
}

/** A debugging hypothesis the candidate is testing. */
export interface Hypothesis {
  text: string;
  status: "open" | "confirmed" | "ruled_out";
}

export interface PanelConfig {
  sessionId: string;
  assessmentTitle: string;
  /** ready (clock not started), active, or over. */
  status?: string;
  role: string;
  product: string;
  endsAt: string; // ISO
  tabs: PanelTab[];
  monitoring: { camera: boolean; screen: boolean; audio: boolean };
  avatar: { name: string; enabled: boolean };
  diagram: Diagram;
  notes: string;
  resources: Resource[];
  review: { title: string; files: ReviewFile[]; comments: ReviewComment[] };
  debug: { title: string; log: string[]; hypotheses: Hypothesis[] };
  test: { title: string; failing: string[]; passing: string[]; passed?: boolean };
  activity: ActivityItem[];
}
