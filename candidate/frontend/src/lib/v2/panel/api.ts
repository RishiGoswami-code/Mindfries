/**
 * The panel's client for the V2 API, run in the candidate's browser. It holds
 * a short-lived panel token (never the portal secret), and asks the portal's
 * own server for a fresh one before the old one runs out.
 *
 * Plain functions over fetch, with the server's problem answers turned into
 * errors a page can show. No React in here so it can be checked in Node.
 */

import type { ActivityItem, Diagram } from "./types";
import type { ApiConfig } from "./adapt";

export type ArtifactKind = "diagram" | "notes" | "review" | "hypotheses";

export class PanelApiError extends Error {
  status: number;
  code?: string;
  extra?: Record<string, unknown>;
  constructor(message: string, status: number, code?: string, extra?: Record<string, unknown>) {
    super(message);
    this.status = status;
    this.code = code;
    this.extra = extra;
  }
}

export interface PanelApiOptions {
  /** The V2 API's base address. */
  base: string;
  /** The current panel token; called on every request, so a refresh takes effect at once. */
  token: () => string;
  fetchImpl?: typeof fetch;
}

export interface StatusReply {
  session_status: string;
  ends_at: string;
  requires: { camera: boolean; screen: boolean; audio: boolean };
  missing: string[];
}

/** Holds the current panel token, so a refresh takes effect on the very next request. */
export function createTokenStore(initial: string): { get: () => string; set: (t: string) => void } {
  let current = initial;
  return { get: () => current, set: (t) => void (current = t) };
}

export function createPanelApi(o: PanelApiOptions) {
  const f = o.fetchImpl ?? fetch;

  async function call<T>(method: string, path: string, init?: { json?: unknown; body?: BodyInit; contentType?: string }): Promise<T> {
    const headers: Record<string, string> = { Authorization: `Bearer ${o.token()}` };
    let body: BodyInit | undefined = init?.body;
    if (init?.json !== undefined) {
      headers["content-type"] = "application/json";
      body = JSON.stringify(init.json);
    } else if (init?.contentType) headers["content-type"] = init.contentType;
    let res: Response;
    try {
      res = await f(`${o.base}${path}`, { method, headers, body, cache: "no-store" });
    } catch {
      throw new PanelApiError("Couldn't reach Mindfries. Check your connection.", 0, "unreachable");
    }
    if (res.status === 204) return undefined as T;
    const data = (await res.json().catch(() => ({}))) as { detail?: string; title?: string; code?: string };
    if (!res.ok) throw new PanelApiError(data.detail || data.title || `Mindfries answered ${res.status}.`, res.status, data.code, data as Record<string, unknown>);
    return data as T;
  }

  return {
    /** Everything the panel draws for this candidate, as the API sends it (see adapt.ts). */
    config: () => call<ApiConfig>("GET", "/v2/panel/config"),

    /** What the candidate's browser is sharing, every few seconds. */
    status: (s: { camera: boolean; screen: boolean; audio: boolean }) => call<StatusReply>("POST", "/v2/panel/status", { json: s }),

    /** A camera still, a screenshot or a microphone clip. */
    upload: (what: "camera" | "screen" | "audio", blob: Blob) =>
      call<{ id: string }>("POST", `/v2/panel/upload/${what}`, { body: blob, contentType: blob.type || "application/octet-stream" }),

    /** Saves a new version; a conflict (another window saved first) carries the current version. */
    async save(kind: ArtifactKind, body: unknown, version: number): Promise<number> {
      const r = await call<{ version: number }>("PUT", `/v2/panel/artifacts/${kind}`, { json: { version, body } });
      return r.version;
    },

    activity: (after: number) => call<{ items: unknown[] }>("GET", `/v2/panel/activity?after=${after}`),

    /** The candidate's own words about a flag or notice. */
    respond: (id: string, text: string) => call<void>("POST", `/v2/panel/activity/${encodeURIComponent(id)}/response`, { json: { text } }),

    avatarToken: () => call<{ name: string; token: string; model: string; ws_url: string; expires_at: string }>("POST", "/v2/panel/avatar/token", { json: {} }),

    avatarTranscript: (utterances: { seq: number; role: "candidate" | "avatar"; text: string; at: string }[]) =>
      call<{ stored: number }>("POST", "/v2/panel/avatar/transcript", { json: { utterances } }),

    /** The address for the live feed (EventSource can't set headers, so the token rides in the query). */
    feedUrl: (after: number) => `${o.base}/v2/panel/feed?token=${encodeURIComponent(o.token())}${after ? `&after=${after}` : ""}`,
  };
}

export type PanelApi = ReturnType<typeof createPanelApi>;

/** A tracker item as the API sends it, as the panel holds it. */
export function toActivity(raw: unknown): ActivityItem | null {
  const r = raw as Record<string, unknown> | null;
  if (!r || typeof r.id !== "number" || typeof r.text !== "string") return null;
  const severity = r.severity === "flag" || r.severity === "notice" ? r.severity : "info";
  const source = r.source === "watcher" || r.source === "panel" ? r.source : "server";
  const category = r.category === "observed" || r.category === "verified" || r.category === "signal" ? r.category : undefined;
  return { id: String(r.id), at: String(r.at), severity, source, category, text: r.text };
}

/** Adds items to a tracker list: new ones in order, ones already held ignored. */
export function mergeActivity(held: ActivityItem[], incoming: ActivityItem[]): ActivityItem[] {
  const seen = new Set(held.map((i) => i.id));
  const add = incoming.filter((i) => !seen.has(i.id));
  if (!add.length) return held;
  return [...held, ...add].sort((a, b) => Number(a.id) - Number(b.id));
}

export type { Diagram };
