import Link from "next/link";
import { BadgeCheck, Code2, Globe, Star } from "lucide-react";
import { GithubMark, GitlabMark, LinkedinMark } from "@/components/BrandMarks";
import { Pill } from "@/components/ui";
import { fmtDate, titleCase, type Tone } from "@/lib/format";
import type { CandidateCard as Card } from "@/lib/candidates";

const n = (count: number, one: string, many = `${one}s`) => `${count} ${count === 1 ? one : many}`;

export const RECOMMENDATION: Record<string, { label: string; tone: Tone }> = {
  strong_hire: { label: "Strong hire", tone: "violet" },
  hire: { label: "Hire", tone: "green" },
  lean_no: { label: "Lean no", tone: "amber" },
  no_hire: { label: "No hire", tone: "coral" },
};

/**
 * One candidate at a glance: who they are, how their latest assessment came
 * out, and what their linked accounts show. A tick beside an account means
 * the candidate signed in to it (through Composio, read-only), so it is
 * theirs; without one, it is a username they typed.
 */
export function CandidateCard({ c }: { c: Card }) {
  const latest = c.assessments.find((a) => a.score != null || a.decision || a.aiSuggestion) ?? c.assessments[0] ?? null;
  // Only a person's decision is shown as the hiring status, never the AI's.
  const rec = latest?.decision ? RECOMMENDATION[latest.decision] : null;
  const scores = latest ? Object.entries(latest.sectionScores).slice(0, 3) : [];
  const live = c.assessments.some((a) => a.sessionStatus === "live");
  const finished = c.assessments.filter((a) => a.sessionStatus && a.sessionStatus !== "live").length;

  return (
    <Link href={`/admin/candidates/${c.id}`} className="hair-card block p-5 transition hover:shadow-md">
      <div className="flex items-start gap-3">
        <Avatar c={c} live={live} size={48} />
        <div className="min-w-0 flex-1">
          <div className="truncate text-[15px] font-bold">{c.name || c.email}</div>
          <div className="truncate text-xs text-dim">{c.headline || c.email}</div>
        </div>
        {rec && <Pill tone={rec.tone}>{rec.label}</Pill>}
      </div>

      {scores.length > 0 || latest?.score != null ? (
        <div className="mt-4 grid grid-cols-3 gap-3 border-t border-hair pt-3">
          {(latest?.score != null ? ([["Overall", latest.score], ...scores.slice(0, 2)] as Array<[string, number]>) : scores).map(([label, value]) => (
            <div key={label}>
              <div className="text-xl font-extrabold tracking-tight">{value}</div>
              <div className="truncate text-[11px] text-dim">{titleCase(label)}</div>
              <div className="mt-1 h-1 rounded-full bg-surface-2">
                <div className="h-1 rounded-full bg-[#6d3fe0]" style={{ width: `${Math.max(0, Math.min(100, value))}%` }} />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <p className="mt-4 border-t border-hair pt-3 text-xs text-faint">
          {c.assessments.length === 0 ? "Not invited to an assessment yet." : "No scores yet."}
        </p>
      )}

      <div className="mt-3 border-t border-hair pt-3">
        <div className="text-[11px] font-semibold uppercase tracking-wide text-faint">Evidence</div>
        <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-1.5 text-xs">
          {c.github && (
            <Account icon={<GithubMark size={13} />} verified={c.github.verified}>
              {n(c.github.projectCount + c.github.privateProjects, "repo")}
              {c.github.totalStars > 0 && (
                <span className="ml-1 inline-flex items-center gap-0.5 text-dim">
                  <Star size={10} /> {c.github.totalStars}
                </span>
              )}
            </Account>
          )}
          {c.gitlab && (
            <Account icon={<GitlabMark size={13} />} verified={c.gitlab.verified}>
              {n(c.gitlab.projectCount + c.gitlab.privateProjects, "project")}
            </Account>
          )}
          {c.linkedin && (
            <Account icon={<LinkedinMark size={13} />} verified>
              LinkedIn
            </Account>
          )}
          {c.portfolio && (
            <Account icon={<Globe size={13} />} verified={false}>
              Portfolio
            </Account>
          )}
          <span className="inline-flex items-center gap-1">
            <Code2 size={13} className="text-dim" />
            {live ? "Live now" : c.assessments.length === 0 ? "No assessments" : `${finished}/${n(c.assessments.length, "assessment")}`}
          </span>
        </div>
        {(c.languages.length > 0 || c.lastActiveAt) && (
          <div className="mt-2 truncate text-[11px] text-dim">
            {c.languages
              .slice(0, 4)
              .map((l) => l.name)
              .join(" · ")}
            {c.lastActiveAt ? `${c.languages.length > 0 ? " — " : ""}active ${fmtDate(c.lastActiveAt)}` : ""}
          </div>
        )}
      </div>
    </Link>
  );
}

export function Avatar({ c, live = false, size }: { c: Card; live?: boolean; size: number }) {
  return (
    <span className="relative shrink-0" style={{ width: size, height: size }}>
      {c.avatarUrl ? (
        // eslint-disable-next-line @next/next/no-img-element -- a remote avatar from the candidate's linked account
        <img src={c.avatarUrl} alt="" width={size} height={size} className="rounded-full border border-hair object-cover" style={{ width: size, height: size }} />
      ) : (
        <span className="flex items-center justify-center rounded-full bg-surface-2 font-bold text-dim" style={{ width: size, height: size, fontSize: size / 2.6 }}>
          {(c.name || c.email).charAt(0).toUpperCase()}
        </span>
      )}
      {live && <span title="In a live session now" className="absolute bottom-0 right-0 h-3 w-3 rounded-full border-2 border-white bg-[#1b8f4e]" />}
    </span>
  );
}

function Account({ icon, verified, children }: { icon: React.ReactNode; verified: boolean; children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1" title={verified ? "Signed in by the candidate — theirs" : "Entered by the candidate, not confirmed"}>
      {icon}
      {children}
      {verified && <BadgeCheck size={12} className="text-[#1b8f4e]" />}
    </span>
  );
}
