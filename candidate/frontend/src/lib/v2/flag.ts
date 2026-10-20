import "server-only";

/**
 * The V2 switch (v2/PLAN.md Part 1, INTEGRATION.md). Off unless the
 * deployment sets V2_ENABLED=1, so every V2 surface in this app is invisible
 * to V1 users until the owner turns it on.
 */
export function v2Enabled(): boolean {
  const v = (process.env.V2_ENABLED ?? "").trim().toLowerCase();
  return v === "1" || v === "true";
}
