import "server-only";

// Server-side client for the V2 engine (the deployed v2/api service). Set
// V2_ENGINE_URL to its base and V2_ADMIN_TOKEN to the engine's admin token so
// the candidate's "Set up workspace" can mint a one-time setup code.

export function v2EngineUrl(): string | null {
  return process.env.V2_ENGINE_URL?.replace(/\/$/, "") || null;
}

export async function mintV2SetupCode(input: {
  role: string;
  jd: string;
  title?: string;
  durationMinutes?: number;
  assessmentId?: string;
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
        assessment_id: input.assessmentId ?? "",
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
