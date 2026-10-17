import type { Metadata } from "next";
import { Award, Briefcase, Eye, FileText, Layers, Link2, ListChecks } from "lucide-react";
import { CandidateShell } from "@/components/dashboard/CandidateShell";
import { PreviewModeProvider } from "@/components/profile/PreviewMode";
import { IdentityCard } from "@/components/profile/IdentityCard";
import { Chips, ProfileCard, ProfileSections, StatePill } from "@/components/profile/ProfileCard";
import { CompletionSection } from "@/components/profile/CompletionSection";
import { ResumeSection } from "@/components/profile/ResumeSection";
import { LinkedAccounts } from "@/components/profile/LinkedAccounts";
import { ProjectsSection } from "@/components/profile/ProjectsSection";
import { EvidenceSection } from "@/components/profile/EvidenceSection";
import { JobPreferencesSection } from "@/components/profile/JobPreferencesSection";
import { currentCandidate } from "@/lib/auth/users";
import { getProfile, getResumeUrl } from "@/lib/profile/actions";
import { getJobPreferences, getResumeFile, listDocuments } from "@/lib/profile/extras";
import { listKnowledge } from "@/lib/profile/knowledge";
import { activeLabel, summarizeKnowledge } from "@/lib/profile/knowledge-summary";
import { profileCompletion } from "@/lib/profile/completion";
import { formatSalaryRange, hasPreferences } from "@/lib/profile/job-preferences";
import { PLATFORM_ORDER, PLATFORMS } from "@/lib/profile/links";
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
 * Who they are lives in IdentityCard and changes through "Edit profile".
 * Everything else is a card (ProfileCard) whose summary is worked out from
 * the section's own records, and which opens that section's window — the
 * resume and its preview, linked accounts, projects and activity read from
 * them, evidence with supporting documents, job preferences — plus a
 * completion checklist that opens each of the others.
 *
 * Job preferences and documents need migration 0024; read apart from the
 * profile (lib/profile/extras.ts), a database without it still loads this
 * page, and those two say they aren't set up.
 *
 * "View as others see" (PreviewModeProvider) is a real toggle: it hides
 * every edit affordance, and the cards a hiring team wouldn't see at all.
 */
export default async function ProfilePage({ searchParams }: { searchParams: Promise<{ connect?: string; result?: string }> }) {
  const query = await searchParams;
  const [session, profile, assessments] = await Promise.all([currentCandidate(), getProfile(), listAssessmentsOrUndefined()]);
  const [resumeUrl, resumeFile, knowledge, verifiable, preferences, documents] = await Promise.all([
    getResumeUrl(profile?.resume_path || null),
    getResumeFile(profile?.resume_path),
    session ? listKnowledge(session.id) : [],
    connectablePlatforms(),
    session ? getJobPreferences(session.id) : Promise.resolve({ available: false as const, value: null }),
    session ? listDocuments(session.id) : Promise.resolve({ available: false as const, items: [] as [] }),
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
  const workModes = profile?.open_to ?? [];
  const connected = PLATFORM_ORDER.filter((id) => links[id]);
  const items = assessments ?? [];
  const completed = items.filter((a) => a.status === "submitted" || a.status === "closed").length;
  const inProgress = items.filter((a) => a.status === "in-progress").length;
  const summary = summarizeKnowledge(knowledge);
  const prefs = preferences.value;
  const prefsSet = preferences.available ? hasPreferences(prefs, workModes) : null;

  const completion = profileCompletion({
    hasResume,
    detailsSaved: !!profile?.profile_updated_at && !!profile.role,
    links,
    knowledgeCount: knowledge.length,
    completedAssessments: completed,
    assessmentsInProgress: inProgress,
    documents: documents.items.length,
    preferencesSet: prefsSet,
  });
  const nextUp = completion.sections.filter((s) => s.state !== "done").slice(0, 2);

  // Coming back from a Composio sign-in: open Linked accounts so its result is in view.
  const returningFromConnect = !!(query.connect && query.result);
  const totalProjects = summary.publicProjects + summary.privateProjects;
  const active = activeLabel(summary.lastActiveAt);

  return (
    <CandidateShell sessionName={session?.name} profile={profile}>
      <PreviewModeProvider>
        <ProfileSections initial={returningFromConnect ? "linked" : null}>
          <div className="mt-6 space-y-5">
            <ProfileCard
              id="completion"
              icon={<ListChecks size={28} strokeWidth={1.7} />}
              title="Profile completion"
              hideInPreview
              dialog={{ subtitle: "Finish each section so hiring teams see the whole of what you bring.", size: "lg" }}
              summary={
                <>
                  <p>
                    {completion.percent}% complete · {completion.done} of {completion.total} sections finished
                  </p>
                  <div className="mt-3 h-2 max-w-[560px] overflow-hidden rounded-full bg-[#E9E6F6]">
                    <div className="h-full rounded-full bg-gradient-to-r from-[#5B3BE0] to-[#7C5CFC]" style={{ width: `${completion.percent}%` }} />
                  </div>
                  {nextUp.length > 0 && <p className="mt-2.5 text-[14px] text-[#6E6A8F]">Next: {nextUp.map((s) => s.next).join(" · ")}</p>}
                </>
              }
            >
              <CompletionSection completion={completion} />
            </ProfileCard>

            <IdentityCard sessionName={session?.name} profile={profile} />

            <div className="grid gap-5 lg:grid-cols-2">
              <ProfileCard
                id="resume"
                icon={<FileText size={28} strokeWidth={1.7} />}
                title="Resume"
                hideInPreview={!hasResume}
                dialog={{ subtitle: "The resume hiring teams see beside your assessments.", size: "xl" }}
                summary={
                  hasResume ? (
                    <>
                      <p className="truncate">{resumeFile?.fileName ?? "Resume"}</p>
                      {resumeFile?.uploadedAt && <p className="text-[14px] text-[#6E6A8F]">Uploaded {formatDate(resumeFile.uploadedAt)}</p>}
                    </>
                  ) : (
                    <p>Not uploaded yet</p>
                  )
                }
                action={
                  resumeUrl ? (
                    <a
                      href={resumeUrl}
                      target="_blank"
                      rel="noreferrer"
                      aria-label="Open your resume in a new tab"
                      className="flex h-12 w-12 items-center justify-center rounded-2xl border border-[#ECE9F6] bg-white text-[#1F1B4D] shadow-[0_4px_12px_-6px_rgba(27,24,64,0.25)] hover:bg-[#F6F4FC]"
                    >
                      <Eye size={21} strokeWidth={1.8} />
                    </a>
                  ) : undefined
                }
              >
                <ResumeSection resumePath={profile?.resume_path} resumeUrl={resumeUrl} file={resumeFile} parsed={parsed} emptyFields={emptyFields} />
              </ProfileCard>

              <ProfileCard
                id="linked"
                icon={<Link2 size={28} strokeWidth={1.7} />}
                title="Linked accounts"
                dialog={{ subtitle: "Connect your professional accounts so hiring teams can see your work.", size: "md" }}
                summary={
                  <div className="flex flex-wrap items-center gap-3">
                    <p>
                      {connected.length} of {PLATFORM_ORDER.length} connected
                    </p>
                    {connected.length > 0 && (
                      <span className="flex -space-x-1.5">
                        {connected.map((id) => {
                          const Icon = PLATFORMS[id].icon;
                          return (
                            <span
                              key={id}
                              title={PLATFORMS[id].label}
                              className="flex h-8 w-8 items-center justify-center rounded-full text-white ring-2 ring-white"
                              style={{ backgroundColor: PLATFORMS[id].tint }}
                            >
                              <Icon size={15} />
                            </span>
                          );
                        })}
                      </span>
                    )}
                  </div>
                }
              >
                <LinkedAccounts
                  links={links}
                  verifiable={verifiable}
                  connectResult={returningFromConnect ? { platform: query.connect!, result: query.result! } : undefined}
                />
              </ProfileCard>

              <ProfileCard
                id="projects"
                icon={<Layers size={28} strokeWidth={1.7} />}
                title="Projects & activity"
                hideInPreview={knowledge.length === 0}
                dialog={{ subtitle: "Your projects, languages and recent activity, read from your linked accounts.", size: "xl" }}
                summary={
                  knowledge.length === 0 ? (
                    <p>Connect GitHub or GitLab to show your work</p>
                  ) : (
                    <>
                      <p>{[plural(totalProjects, "project"), plural(summary.languages.length, "language"), active].filter(Boolean).join(" · ")}</p>
                      <Chips items={summary.projects.slice(0, 3).map((p) => p.name)} more={Math.max(0, totalProjects - 3)} />
                    </>
                  )
                }
              >
                <ProjectsSection items={knowledge} />
              </ProfileCard>

              <ProfileCard
                id="evidence"
                icon={<Award size={28} strokeWidth={1.7} />}
                title="Evidence"
                dialog={{ subtitle: "Your assessments, and the documents that back up your profile.", size: "xl" }}
                summary={
                  <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                    <p>
                      {plural(items.length, "assessment")}
                      {documents.available && ` · ${plural(documents.items.length, "document")}`}
                    </p>
                    <span className="flex flex-wrap gap-2">
                      {completed > 0 && <StatePill state="done">{completed} completed</StatePill>}
                      {inProgress > 0 && <StatePill state="in-progress">{inProgress} in progress</StatePill>}
                    </span>
                  </div>
                }
              >
                <EvidenceSection assessments={assessments} documents={documents} />
              </ProfileCard>

              {preferences.available && (
                <ProfileCard
                  id="preferences"
                  icon={<Briefcase size={28} strokeWidth={1.7} />}
                  title="Job preferences"
                  hideInPreview={!prefsSet}
                  className="lg:col-span-2"
                  dialog={{ subtitle: "The work you're looking for — shown to hiring teams beside your profile.", size: "lg" }}
                  summary={
                    prefsSet ? (
                      <>
                        <p>Roles, work mode, locations and salary you&apos;re looking for.</p>
                        <Chips
                          items={[
                            ...(prefs?.roles.slice(0, 2) ?? []),
                            ...(prefs?.employment.slice(0, 1) ?? []),
                            ...workModes.slice(0, 1).map((m) => (m === "Onsite" ? "On-site" : m)),
                            ...(prefs?.salary ? [formatSalaryRange(prefs.salary)] : []),
                          ]}
                        />
                      </>
                    ) : (
                      <p>Preferred roles, work mode, locations and salary expectations.</p>
                    )
                  }
                >
                  <JobPreferencesSection value={prefs} workModes={workModes} available={preferences.available} />
                </ProfileCard>
              )}
            </div>
          </div>
        </ProfileSections>
      </PreviewModeProvider>
    </CandidateShell>
  );
}
