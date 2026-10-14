import { BadgeCheck, Code2, Star } from "lucide-react";
import { GithubMark, GitlabMark, LinkedinMark } from "@/components/BrandMarks";
import { Pill } from "@/components/ui";
import { fmtDate, recommendationLabel, titleCase, type Tone } from "@/lib/format";
import type { CandidateProfile } from "@/lib/candidate-profile";

const n = (count: number, one: string, many = `${one}s`) => `${count} ${count === 1 ? one : many}`;

const RECOMMENDATION_TONE: Record<string, Tone> = { strong_hire: "violet", hire: "green", lean_no: "amber", no_hire: "coral" };

/**
 * The candidate at a glance, at the top of their page: who they are, how
 * the assessment came out, and what their linked accounts show.
 *
 * The scores are the assessment's (candidate_applications.section_scores);
 * the recommendation is the reviewer's when one was recorded, else the AI
 * report's, and the card says which. Everything under "Linked accounts" was
 * read from GitHub, GitLab or LinkedIn — through the candidate's own
 * read-only sign-in where the account is marked as theirs. The detail
 * behind each part is further down the page.
 */
export function CandidateSnapshot({
  name,
  email,
  profile,
  score,
  sectionScores,
  recommendation,
  sessionStatus,
}: {
  name: string;
  email: string;
  profile: CandidateProfile | null;
  score: number | null;
  sectionScores: Record<string, number>;
  recommendation: { value: string; by: "reviewer" | "ai" } | null;
  /** The assessment session's status, or null when it hasn't started. */
  sessionStatus: string | null;
}) {
  const scores = Object.entries(sectionScores).slice(0, 4);
  const github = profile?.codeAccounts.find((a) => a.source === "github");
  const gitlab = profile?.codeAccounts.find((a) => a.source === "gitlab");
  const linkedin = profile?.linkedin ?? null;
  const languages = new Map<string, number>();
  for (const a of profile?.codeAccounts ?? []) for (const l of a.languages) languages.set(l.name, (languages.get(l.name) ?? 0) + l.projects);
  const topLanguages = [...languages.entries()].sort((a, b) => b[1] - a[1]).slice(0, 6);
  const lastActive = (profile?.codeAccounts ?? [])
    .map((a) => a.activity?.lastActiveAt)
    .filter((t): t is string => !!t)
    .sort()
    .at(-1);
  const initial = (name || email).trim().charAt(0).toUpperCase();

  return (
    <div className="hair-card p-5">
      <div className="flex flex-wrap items-start gap-4">
        <div className="relative shrink-0">
          {profile?.avatarUrl ? (
            // eslint-disable-next-line @next/next/no-img-element -- a remote avatar from the platform itself; not worth an optimisation pass at 56px
            <img src={profile.avatarUrl} alt="" width={56} height={56} className="h-14 w-14 rounded-full border border-hair object-cover" />
          ) : (
            <span className="flex h-14 w-14 items-center justify-center rounded-full bg-surface-2 text-xl font-bold text-dim">{initial}</span>
          )}
          {sessionStatus === "live" && <span title="In a live session now" className="absolute bottom-0.5 right-0.5 h-3 w-3 rounded-full border-2 border-white bg-[#1b8f4e]" />}
        </div>
        <div className="min-w-0 flex-1">
          <div className="truncate text-lg font-extrabold tracking-tight">{name || email}</div>
          <div className="truncate text-sm text-dim">{profile?.headline || linkedin?.headline || email}</div>
          {profile?.location && <div className="text-xs text-faint">{profile.location}</div>}
        </div>
        {recommendation && (
          <div className="text-right">
            <Pill tone={RECOMMENDATION_TONE[recommendation.value] ?? "gray"}>{recommendationLabel[recommendation.value] ?? titleCase(recommendation.value)}</Pill>
            <div className="mt-1 text-[11px] text-faint">{recommendation.by === "reviewer" ? "Reviewer's decision" : "AI recommendation"}</div>
          </div>
        )}
      </div>

      {scores.length > 0 ? (
        <div className="mt-5 grid grid-cols-2 gap-x-6 gap-y-4 border-t border-hair pt-4 sm:grid-cols-4">
          {score != null && <Metric label="Overall" value={score} />}
          {scores.slice(0, score != null ? 3 : 4).map(([section, value]) => (
            <Metric key={section} label={titleCase(section)} value={value} />
          ))}
        </div>
      ) : (
        <p className="mt-4 border-t border-hair pt-4 text-sm text-dim">
          {sessionStatus ? "Scores appear once the assessment has been evaluated." : "Scores appear once the candidate has taken the assessment."}
        </p>
      )}

      <div className="mt-4 border-t border-hair pt-4">
        <div className="eyebrow">Evidence</div>
        <div className="mt-2 flex flex-wrap gap-x-5 gap-y-2 text-sm">
          {github && (
            <Account href={github.profileUrl} icon={<GithubMark />} verified={github.verified}>
              {n(github.projectCount + github.privateProjects, "repo")}
              {github.totalStars > 0 ? ` · ${github.totalStars}★` : ""}
            </Account>
          )}
          {gitlab && (
            <Account href={gitlab.profileUrl} icon={<GitlabMark />} verified={gitlab.verified}>
              {n(gitlab.projectCount + gitlab.privateProjects, "project")}
            </Account>
          )}
          {linkedin && (
            <Account href={linkedin.profileUrl} icon={<LinkedinMark />} verified>
              {linkedin.name}
            </Account>
          )}
          <span className="inline-flex items-center gap-1.5">
            <Code2 size={14} className="text-dim" />
            {sessionStatus ? `Live assessment · ${sessionStatus}` : "Assessment not started"}
          </span>
        </div>
        {!github && !gitlab && !linkedin && <p className="mt-2 text-xs text-faint">The candidate hasn&apos;t linked GitHub, GitLab or LinkedIn.</p>}

        {(topLanguages.length > 0 || lastActive || github?.followers != null) && (
          <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs text-dim">
            {topLanguages.map(([lang, count]) => (
              <span key={lang} className="rounded-full border border-hair bg-surface-2 px-2 py-0.5">
                {lang} · {count}
              </span>
            ))}
            {lastActive && <span className="ml-1">Last active {fmtDate(lastActive)}</span>}
            {github?.followers != null && (
              <span className="ml-1 inline-flex items-center gap-1">
                <Star size={11} /> {n(github.followers, "follower")} on GitHub
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  const pct = Math.max(0, Math.min(100, value));
  return (
    <div>
      <div className="text-2xl font-extrabold tracking-tight">{value}</div>
      <div className="text-xs text-dim">{label}</div>
      <div className="mt-1.5 h-1.5 rounded-full bg-surface-2">
        <div className="h-1.5 rounded-full bg-[#6d3fe0]" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

function Account({ href, icon, verified, children }: { href: string | null; icon: React.ReactNode; verified: boolean; children: React.ReactNode }) {
  const body = (
    <>
      {icon}
      <span>{children}</span>
      {verified && <BadgeCheck size={13} className="text-[#1b8f4e]" aria-label="Signed in by the candidate — theirs" />}
    </>
  );
  return href ? (
    <a href={href} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 underline-offset-2 hover:underline">
      {body}
    </a>
  ) : (
    <span className="inline-flex items-center gap-1.5">{body}</span>
  );
}
