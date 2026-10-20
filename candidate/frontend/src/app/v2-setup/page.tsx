import { listAssessmentsOrUndefined } from "@/lib/backend/client";
import { V2SetupWizard } from "@/components/v2-setup/V2SetupWizard";

/** Looked up per request from the candidate's real records. */
export const dynamic = "force-dynamic";

/**
 * Entry point for setting up a V2 (local-workspace) assessment. Resolves
 * `?assessment=<id>` — set by the "Set up workspace" link on a V2 assessment
 * (components/assessments/AssessmentItem.tsx) — into its display details, then
 * hands them to the client wizard. A missing or non-V2 id renders the wizard's
 * own "no assessment" state rather than inventing one.
 */
export default async function V2SetupPage({
  searchParams,
}: {
  searchParams: Promise<{ assessment?: string }>;
}) {
  const { assessment } = await searchParams;
  const id = assessment?.trim() || null;

  let info: { id: string; role: string; company: string } | null = null;
  if (id) {
    const items = (await listAssessmentsOrUndefined()) ?? [];
    const match = items.find((a) => a.id === id && a.engine === "v2");
    if (match) info = { id, role: match.role, company: match.company };
  }

  return <V2SetupWizard assessment={info} />;
}
