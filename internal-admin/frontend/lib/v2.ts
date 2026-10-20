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

// mintV2SetupCode asks the engine for a one-time setup code for an assessment.
// Returns null if the engine URL/token aren't configured or the call fails.
export async function mintV2SetupCode(input: {
  role: string;
  jd: string;
  title?: string;
  durationMinutes?: number;
}): Promise<{ code: string } | null> {
  const base = v2EngineUrl();
  const token = process.env.V2_ADMIN_TOKEN;
  if (!base || !token) return null;
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
    if (!res.ok) return null;
    const d = (await res.json()) as { setup_code?: string };
    return d.setup_code ? { code: d.setup_code } : null;
  } catch {
    return null;
  }
}
