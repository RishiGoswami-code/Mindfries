import "server-only";
import type { V2Product } from "./v2-roles";

// Client for the V2 engine (the deployed v2/api service). Set V2_ENGINE_URL to
// its base (e.g. https://mindfries-cli.onrender.com) and V2_ADMIN_TOKEN to the
// engine's admin token to mint setup codes. Client-safe types/constants live in
// lib/v2-roles.ts.

export type { V2Product };

export function v2EngineUrl(): string | null {
  return process.env.V2_ENGINE_URL?.replace(/\/$/, "") || null;
}

export function v2Ready(): boolean {
  return !!v2EngineUrl();
}

// fetchV2Products lists the verified products in the engine's registry.
export async function fetchV2Products(): Promise<V2Product[]> {
  const base = v2EngineUrl();
  if (!base) return [];
  try {
    const res = await fetch(`${base}/v2/products`, { cache: "no-store" });
    if (!res.ok) return [];
    return (await res.json()) as V2Product[];
  } catch {
    return [];
  }
}

// mintV2SetupCode asks the engine to resolve-at-invite: it validates that a
// product covers this role+JD and, only then, mints a one-time setup code.
// Returns the code (plus the chosen product/task) on success, or an { error }
// the caller surfaces to the admin — so we never record an assessment the
// candidate couldn't start.
export type MintResult =
  | { code: string; productSlug?: string; taskTitle?: string; taskKind?: string }
  | { error: string };

export async function mintV2SetupCode(input: {
  role: string;
  jd: string;
  title?: string;
  durationMinutes?: number;
}): Promise<MintResult> {
  const base = v2EngineUrl();
  const token = process.env.V2_ADMIN_TOKEN;
  if (!base || !token) {
    return { error: "V2 engine isn't configured (set V2_ENGINE_URL and V2_ADMIN_TOKEN)." };
  }
  try {
    const res = await fetch(`${base}/v2/admin/setup-codes`, {
      method: "POST",
      headers: { "content-type": "application/json", "X-Admin-Token": token },
      body: JSON.stringify({
        role: input.role,
        jd: input.jd,
        assessment_title: input.title ?? "",
        duration_minutes: input.durationMinutes ?? 90,
      }),
      cache: "no-store",
    });
    const d = (await res.json().catch(() => ({}))) as {
      setup_code?: string;
      product_slug?: string;
      task_title?: string;
      task_kind?: string;
      detail?: string;
    };
    if (!res.ok) {
      return { error: d.detail || `Engine returned ${res.status}.` };
    }
    if (!d.setup_code) return { error: "Engine didn't return a setup code." };
    return { code: d.setup_code, productSlug: d.product_slug, taskTitle: d.task_title, taskKind: d.task_kind };
  } catch {
    return { error: "Couldn't reach the V2 engine." };
  }
}

// --- GRADE / review surface (reads the engine's admin review endpoints) ---

export type V2ReviewItem = {
  session_id: string;
  assessment_title: string;
  role: string;
  product_slug: string;
  bank_task_id: string;
  status: string;
  started_at: string;
  submitted: boolean;
  graded: boolean;
  passed?: boolean;
  submission_status?: string;
};

export type V2Report = {
  outcome: { graded: boolean; passed: boolean; check?: string[] };
  effort: {
    window_minutes: number; active_minutes: number; files_changed: number;
    lines_added: number; lines_removed: number; commits: number; task_files_intact: boolean;
  };
  ai: {
    agents_detected: string[]; agents_left_running: boolean; ai_extensions: string[];
    candidate_attributed_lines: number; agent_attributed_lines: number;
  };
  changes: { path: string; lines_added: number; lines_removed: number; origin?: string }[] | null;
  coverage: { competencies: string[]; cannot_assess: string[] };
  flags: string[];
};

export type V2Review = {
  session: {
    id: string; assessment_title: string; role: string; product_slug: string;
    bank_task_id: string; task_version: string; status: string;
    started_at: string; ends_at: string; duration_minutes: number;
  };
  submission?: {
    id: string; status: string; head_commit: string; base_commit: string;
    commit_message: string; bundle_bytes: number; task_files_intact: boolean;
    created_at: string; verified_at?: string | null;
  } | null;
  report: V2Report;
  events: { seq: number; occurred_at: string; type: string; data: unknown }[];
};

async function v2AdminGet<T>(path: string): Promise<T | null> {
  const base = v2EngineUrl();
  const token = process.env.V2_ADMIN_TOKEN;
  if (!base || !token) return null;
  try {
    const res = await fetch(`${base}${path}`, { headers: { "X-Admin-Token": token }, cache: "no-store" });
    if (!res.ok) return null;
    return (await res.json()) as T;
  } catch {
    return null;
  }
}

export async function fetchV2Reviews(): Promise<V2ReviewItem[]> {
  return (await v2AdminGet<V2ReviewItem[]>("/v2/admin/reviews")) ?? [];
}

export async function fetchV2Review(id: string): Promise<V2Review | null> {
  return v2AdminGet<V2Review>(`/v2/admin/reviews/${encodeURIComponent(id)}`);
}
