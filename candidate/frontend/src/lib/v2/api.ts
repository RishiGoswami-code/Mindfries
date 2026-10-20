import "server-only";

/**
 * The candidate portal's server-side client for the V2 API (v2/api, deployed
 * on Render). Only this app's server calls it with the portal secret: a
 * browser never holds it. See v2/api/README.md for every route.
 *
 *   V2_ENGINE_URL     the API's base address
 *   V2_PORTAL_SECRET  the shared secret the API expects as X-Portal-Secret
 *
 * Either one missing means V2 isn't set up here, and every caller says so
 * rather than failing.
 */

export function engineUrl(): string | null {
  return process.env.V2_ENGINE_URL?.replace(/\/$/, "") || null;
}

export function portalReady(): boolean {
  return !!engineUrl() && !!process.env.V2_PORTAL_SECRET;
}

/** An error from the V2 API, with its stable `code` and a message fit to show. */
export class V2Error extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string,
  ) {
    super(message);
  }
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const base = engineUrl();
  const secret = process.env.V2_PORTAL_SECRET;
  if (!base || !secret) throw new V2Error("The workspace service isn't set up here yet.", 503, "not_configured");
  let res: Response;
  try {
    res = await fetch(`${base}${path}`, {
      method,
      headers: { "content-type": "application/json", "X-Portal-Secret": secret },
      body: body === undefined ? undefined : JSON.stringify(body),
      cache: "no-store",
    });
  } catch {
    throw new V2Error("Couldn't reach the workspace service. Try again in a moment.", 502, "unreachable");
  }
  if (res.status === 204) return undefined as T;
  const data = (await res.json().catch(() => ({}))) as { detail?: string; title?: string; code?: string };
  if (!res.ok) throw new V2Error(data.detail || data.title || `The workspace service answered ${res.status}.`, res.status, data.code);
  return data as T;
}

// ---- the Start assessment button -------------------------------------------

export type Grant = {
  candidateId: string;
  companyId?: string | null;
  assessmentId: string;
  invitationId?: string | null;
  role: string;
  jd: string;
  title?: string;
  durationMinutes?: number;
};

const grantBody = (g: Grant) => ({
  candidate_id: g.candidateId,
  company_id: g.companyId || "",
  assessment_id: g.assessmentId,
  invitation_id: g.invitationId || "",
  role: g.role,
  jd: g.jd,
  assessment_title: g.title ?? "",
  duration_minutes: g.durationMinutes ?? 0,
});

export type SetupCommand = {
  setup_code: string;
  expires_at: string;
  windows_command: string;
  unix_command: string;
  cli_command: string;
  task_title?: string;
};

export const mintSetupCode = (g: Grant) => call<SetupCommand>("POST", "/v2/portal/setup-codes", grantBody(g));

// ---- notice and consent -----------------------------------------------------

export type NoticeItem = { id: string; label: string; description: string; collected: boolean };
export type Notice = { version: string; items: NoticeItem[] };

export const getNotice = (assessmentId: string) =>
  call<Notice>("GET", `/v2/portal/assessments/${encodeURIComponent(assessmentId)}/notice`);

export const saveConsent = (candidateId: string, assessmentId: string, version: string, items: string[]) =>
  call<void>("POST", "/v2/portal/consents", { candidate_id: candidateId, assessment_id: assessmentId, version, items });

export const getConsent = (candidateId: string, assessmentId: string) =>
  call<{ accepted: boolean; current_version: string }>(
    "GET",
    `/v2/portal/consents?candidate_id=${encodeURIComponent(candidateId)}&assessment_id=${encodeURIComponent(assessmentId)}`,
  );

// ---- the browser sign-in ----------------------------------------------------

export type DeviceInfo = {
  user_code: string;
  status: "pending" | "approved" | "denied" | "used" | "expired";
  expires_at: string;
  device: { os: string; os_version?: string; arch: string };
};

export const deviceInfo = (userCode: string) => call<DeviceInfo>("GET", `/v2/portal/device/${encodeURIComponent(userCode)}`);

export const approveDevice = (userCode: string, g: Grant) =>
  call<void>("POST", `/v2/portal/device/${encodeURIComponent(userCode)}/approve`, grantBody(g));

export const denyDevice = (userCode: string) => call<void>("POST", `/v2/portal/device/${encodeURIComponent(userCode)}/deny`, {});

// ---- GitHub -----------------------------------------------------------------

export type GitHubStatus = { connected: boolean; login?: string; token_waiting?: boolean };

/** Null when this server has no GitHub (so the page doesn't ask for it). */
export async function githubStatus(candidateId: string): Promise<GitHubStatus | null> {
  try {
    return await call<GitHubStatus>("GET", `/v2/portal/github/status?candidate_id=${encodeURIComponent(candidateId)}`);
  } catch (e) {
    if (e instanceof V2Error && e.code === "not_configured") return null;
    throw e;
  }
}

export const githubConnectUrl = (candidateId: string, returnTo: string) =>
  call<{ authorize_url: string }>("POST", "/v2/portal/github/connect", { candidate_id: candidateId, return_to: returnTo });

// ---- the panel --------------------------------------------------------------

export type PanelToken = { token: string; expires_at: string; session_id: string; status: string; ends_at: string };

export const panelToken = (sessionId: string, candidateId: string) =>
  call<PanelToken>("POST", `/v2/portal/sessions/${encodeURIComponent(sessionId)}/panel-token`, { candidate_id: candidateId });

/** The panel's configuration, read on the server so the page arrives drawn. */
export async function panelConfigFor(token: string): Promise<import("./panel/adapt").ApiConfig> {
  const base = engineUrl();
  if (!base) throw new V2Error("The workspace service isn't set up here yet.", 503, "not_configured");
  let res: Response;
  try {
    res = await fetch(`${base}/v2/panel/config`, { headers: { Authorization: `Bearer ${token}` }, cache: "no-store" });
  } catch {
    throw new V2Error("Couldn't reach the workspace service. Try again in a moment.", 502, "unreachable");
  }
  if (!res.ok) throw new V2Error(`The workspace service answered ${res.status}.`, res.status);
  return (await res.json()) as import("./panel/adapt").ApiConfig;
}
