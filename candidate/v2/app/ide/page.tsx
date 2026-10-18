import type { Metadata } from "next";
import { notFound, redirect } from "next/navigation";
import { Workspace } from "../../components/workspace/Workspace";
import { currentCandidate } from "@/lib/auth/users";
import { getSavedWorkspaceOrUndefined, getSessionAssessmentOrUndefined, getSessionOrMissing, secondsRemaining } from "@/lib/backend/client";
import { PRACTICE_BRIEF, PRACTICE_FILES, PRACTICE_NAME } from "@/lib/ide/practice-task";

export const metadata: Metadata = {
  title: "Mindfries Workspace",
  description: "The coding workspace for an assessment or a practice run.",
};

/**
 * The v2 workspace. Same entry rules as /ide: a practice run
 * (?practice=1), or a session the backend says is this candidate's
 * (?session=<id>) — everything it needs fetched here, before the first paint.
 */
export default async function V2IdePage({ searchParams }: { searchParams: Promise<{ session?: string; practice?: string }> }) {
  const [{ session, practice }, candidate] = await Promise.all([searchParams, currentCandidate()]);
  if (!session && practice) {
    return <Workspace practice candidateName={candidate?.name} assessmentName={PRACTICE_NAME} taskBrief={PRACTICE_BRIEF} starterFiles={PRACTICE_FILES} />;
  }
  if (!session) redirect("/dashboard");

  const [assessment, found, saved] = await Promise.all([getSessionAssessmentOrUndefined(session), getSessionOrMissing(session), getSavedWorkspaceOrUndefined(session)]);
  // Not this candidate's session (or no such session): no workspace.
  if (found === "missing") notFound();
  // A submitted session has nothing left to work on: its report.
  if (found && found.status !== "live") redirect(`/assessments/${encodeURIComponent(session)}/report`);

  return (
    <Workspace
      sessionId={session}
      candidateName={candidate?.name}
      assessmentName={assessment?.name}
      remainingSeconds={found ? secondsRemaining(found) : undefined}
      sandbox={!!found?.sandbox}
      sandboxNetwork={found?.sandboxNetwork}
      taskBrief={assessment?.taskBrief}
      starterFiles={assessment?.starterFiles}
      serverWorkspace={saved ? { files: saved.files, savedAt: Date.parse(saved.savedAt), frozen: saved.frozen } : undefined}
    />
  );
}
