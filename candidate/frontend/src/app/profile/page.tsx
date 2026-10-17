import type { Metadata } from "next";
import { BarChart3, Code2, Eye, FileText, Link2, ListChecks } from "lucide-react";
import { CandidateShell } from "@/components/dashboard/CandidateShell";
import { PreviewModeProvider } from "@/components/profile/PreviewMode";
import { IdentityCard } from "@/components/profile/IdentityCard";
import { ProfileStrength } from "@/components/profile/ProfileStrength";
import { ProfileTile } from "@/components/profile/ProfileTile";
import { LinkedAccounts } from "@/components/profile/LinkedAccounts";
import { EvidenceSummary } from "@/components/profile/EvidenceSummary";
import { ResumeUpload } from "@/components/profile/ResumeUpload";
import { NextSteps } from "@/components/profile/NextSteps";
import { KnowledgeBase } from "@/components/profile/KnowledgeBase";
import { currentCandidate } from "@/lib/auth/users";
import { getProfile, getResumeUrl } from "@/lib/profile/actions";
import { listKnowledge } from "@/lib/profile/knowledge";
import { nextSteps } from "@/lib/profile/next-steps";
import { PLATFORM_ORDER } from "@/lib/profile/links";
import { connectablePlatforms } from "@/lib/composio";
import { listAssessmentsOrUndefined } from "@/lib/backend/client";
import type { ResumeFields } from "@/lib/profile/resume-fields";

export const metadata: Metadata = {
  title: "Profile · Mindfries",
  description: "What hiring teams see alongside the evidence from your sessions.",
};

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" });
}

const plural = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

/**
 * The candidate's profile: who they are, what they bring, and what their
 * sessions have produced so far.
 *
 * One edit surface for who they are: name, role, location, availability and
 * bio all live in IdentityCard and change through its "Edit profile" button.
 * Everything else is a tile (ProfileTile) — a summary line that opens in
 * place to the working section: the resume upload, linked accounts, what was
 * read from them, the evidence so far, and what's left to do. Every summary
 * is counted from the same data the section shows.
 *
 * "View as others see" (PreviewModeProvider) is a real toggle, not a label:
 * it hides every edit affordance, every not-yet-connected platform, and the
 * tiles a hiring team wouldn't see at all.
 */
export default async function ProfilePage({ searchParams }: { searchParams: Promise<{ connect?: string; result?: string }> }) {
  const query = await searchParams;
  const [session, profile, assessments] = await Promise.all([
    currentCandidate(),
    getProfile(),
    listAssessmentsOrUndefined(),
  ]);
  const [resumeUrl, knowledge, verifiable] = await Promise.all([
    getResumeUrl(profile?.resume_path || null),
    session ? listKnowledge(session.id) : [],
    connectablePlatforms(),
  ]);
  const parsed = (profile?.resume_parsed ?? null) as ResumeFields | null;
  // What the resume could fill in that the candidate hasn't written themselves.
  const emptyFields = parsed
    ? [
        !profile?.role && parsed.role ? "headline" : "",
        !profile?.location && parsed.location ? "location" : "",
        !profile?.bio && parsed.bio ? "about you" : "",
      ].filter(Boolean)
    : [];

  const links = profile?.links || {};
  const hasResume = !!profile?.resume_path;
  const connected = PLATFORM_ORDER.filter((id) => links[id]).length;
  const completed = (assessments ?? []).filter((a) => a.status === "submitted" || a.status === "closed").length;
  const remaining = nextSteps(hasResume, links).length;
  // Coming back from a Composio sign-in: open Linked accounts so its result is in view.
  const returningFromConnect = !!(query.connect && query.result);

  return (
    <CandidateShell sessionName={session?.name} profile={profile}>
      <PreviewModeProvider>
        <div className="mt-6 space-y-5">
          <ProfileStrength profile={profile} hasResume={hasResume} />
          <IdentityCard sessionName={session?.name} profile={profile} />

          <div className="grid gap-5 lg:grid-cols-2">
            <ProfileTile
              icon={<FileText size={26} strokeWidth={1.7} />}
              title="Resume"
              subtitle={
                hasResume
                  ? `1 file uploaded${profile?.resume_parsed_at ? ` · Last updated ${formatDate(profile.resume_parsed_at)}` : ""}`
                  : "Not uploaded yet"
              }
              action={
                resumeUrl ? (
                  <a
                    href={resumeUrl}
                    target="_blank"
                    rel="noreferrer"
                    aria-label="Open your resume"
                    className="flex h-10 w-11 shrink-0 items-center justify-center rounded-xl border border-[#ECE9F6] bg-white text-[#1F1B4D] shadow-[0_4px_12px_-6px_rgba(27,24,64,0.25)] hover:bg-[#F6F4FC]"
                  >
                    <Eye size={19} strokeWidth={1.8} />
                  </a>
                ) : undefined
              }
              hideInPreview={!hasResume}
            >
              <ResumeUpload resumePath={profile?.resume_path} resumeUrl={resumeUrl} parsed={parsed} emptyFields={emptyFields} />
            </ProfileTile>

            <ProfileTile
              icon={<Link2 size={26} strokeWidth={1.7} />}
              title="Linked accounts"
              subtitle="Connect GitHub, GitLab, LinkedIn"
              previewSubtitle="GitHub, GitLab, LinkedIn and a portfolio"
              status={{ label: `${connected} connected`, tone: connected > 0 ? "green" : "grey" }}
              defaultOpen={returningFromConnect}
            >
              <LinkedAccounts
                links={links}
                verifiable={verifiable}
                connectResult={returningFromConnect ? { platform: query.connect!, result: query.result! } : undefined}
              />
            </ProfileTile>

            <ProfileTile
              icon={<Code2 size={26} strokeWidth={1.7} />}
              title="Projects & activity"
              subtitle="Your work, languages and recent activity"
              status={
                knowledge.length > 0
                  ? { label: `${plural(knowledge.length, "account")} read`, tone: "green" }
                  : { label: "Not added", tone: "grey" }
              }
              hideInPreview={knowledge.length === 0}
            >
              <KnowledgeBase items={knowledge} />
            </ProfileTile>

            <ProfileTile
              icon={<BarChart3 size={26} strokeWidth={1.7} />}
              title="Evidence"
              subtitle="Your assessments and how they went"
              status={{ label: `${completed} completed`, tone: completed > 0 ? "green" : "grey" }}
            >
              <EvidenceSummary items={assessments} />
            </ProfileTile>
          </div>

          {remaining > 0 && (
            <ProfileTile
              icon={<ListChecks size={26} strokeWidth={1.7} />}
              title="Next steps"
              subtitle="Finish setting up to get the most out of Mindfries."
              status={{ label: `${remaining} remaining`, tone: "badge" }}
              hideInPreview
              className="mt-2"
            >
              <NextSteps hasResume={hasResume} links={links} />
            </ProfileTile>
          )}
        </div>
      </PreviewModeProvider>
    </CandidateShell>
  );
}
