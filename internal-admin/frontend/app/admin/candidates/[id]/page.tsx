import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ExternalLink } from "lucide-react";
import { listCandidateCards, type CodeAccount } from "@/lib/candidates";
import { PageHeader, Pill } from "@/components/ui";
import { Avatar, RECOMMENDATION } from "@/components/admin/CandidateCard";
import { GithubMark, GitlabMark, LinkedinMark } from "@/components/BrandMarks";
import { fmtDate, titleCase } from "@/lib/format";

export const metadata: Metadata = { title: "Candidate" };

export const dynamic = "force-dynamic";

const n = (count: number, one: string, many = `${one}s`) => `${count} ${count === 1 ? one : many}`;

/** One candidate in full: their accounts' knowledge base and every assessment they were invited to. */
export default async function CandidatePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[0-9a-f-]{36}$/i.test(id)) notFound();
  const [c] = await listCandidateCards(id);
  if (!c) notFound();

  return (
    <div className="space-y-6">
      <Link href="/admin/candidates" className="text-sm text-dim hover:underline">
        ← Candidates
      </Link>
      <div className="flex items-center gap-4">
        <Avatar c={c} size={64} live={c.assessments.some((a) => a.sessionStatus === "live")} />
        <div className="min-w-0 flex-1">
          <PageHeader eyebrow={c.email} title={c.name || c.email}>
            {[c.headline, c.location, `joined ${fmtDate(c.joinedAt)}`, c.emailConfirmed ? "email confirmed" : "email not confirmed"].filter(Boolean).join(" · ")}
          </PageHeader>
        </div>
      </div>

      <section className="hair-card divide-y divide-hair">
        <div className="p-5">
          <div className="eyebrow">Assessments</div>
          {c.assessments.length === 0 ? (
            <p className="mt-2 text-sm text-dim">Not invited to any assessment yet.</p>
          ) : (
            <ul className="mt-3 space-y-3">
              {c.assessments.map((a) => {
                const rec = a.recommendation ? RECOMMENDATION[a.recommendation] : null;
                return (
                  <li key={a.id} className="flex flex-wrap items-center gap-3 text-sm">
                    <span className="font-semibold">{[a.company, a.role].filter(Boolean).join(" — ") || "Assessment"}</span>
                    <span className="text-xs text-faint">invited {fmtDate(a.invitedAt)}</span>
                    <Pill tone={a.sessionStatus === "live" ? "violet" : a.sessionStatus ? "gray" : "amber"}>{a.sessionStatus ?? "not started"}</Pill>
                    {rec && (
                      <Pill tone={rec.tone}>
                        {rec.label}
                        {a.recommendationBy === "reviewer" ? " · reviewer" : " · AI"}
                      </Pill>
                    )}
                    {a.score != null && <span className="mono text-dim">{a.score}%</span>}
                    {Object.entries(a.sectionScores).map(([k, v]) => (
                      <span key={k} className="text-xs text-dim">
                        {titleCase(k)} {v}
                      </span>
                    ))}
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        {c.linkedin && (
          <div className="flex items-start gap-3 p-5">
            <LinkedinMark size={18} />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-semibold">{c.linkedin.name}</span>
                <Pill tone="green">Signed in by the candidate</Pill>
                {c.linkedin.profileUrl && (
                  <a href={c.linkedin.profileUrl} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-xs underline">
                    Profile <ExternalLink size={11} />
                  </a>
                )}
              </div>
              {c.linkedin.headline && <p className="mt-1 text-sm text-dim">{c.linkedin.headline}</p>}
              <p className="mt-1 text-xs text-faint">LinkedIn gives apps a name, headline and picture — not work history. Read {fmtDate(c.linkedin.fetchedAt)}.</p>
            </div>
          </div>
        )}

        {[c.github, c.gitlab]
          .filter((a): a is CodeAccount => !!a)
          .map((a) => (
            <CodeAccountSection key={a.source} a={a} />
          ))}

        {!c.github && !c.gitlab && !c.linkedin && <p className="p-5 text-sm text-dim">No GitHub, GitLab or LinkedIn account linked.</p>}
      </section>
    </div>
  );
}

function CodeAccountSection({ a }: { a: CodeAccount }) {
  const Mark = a.source === "github" ? GithubMark : GitlabMark;
  return (
    <div className="p-5">
      <div className="flex flex-wrap items-center gap-2">
        <Mark size={18} />
        <a href={a.profileUrl} target="_blank" rel="noreferrer" className="font-semibold underline underline-offset-2">
          {a.handle}
        </a>
        <Pill tone={a.verified ? "green" : "amber"}>{a.verified ? "Signed in by the candidate" : "Username entered, not confirmed"}</Pill>
        <span className="text-xs text-faint">
          {a.via === "connection" ? "read through their read-only sign-in" : "public data"} · read {fmtDate(a.fetchedAt)}
        </span>
      </div>
      {a.bio && <p className="mt-2 text-sm text-dim">{a.bio}</p>}
      <p className="mt-2 text-sm">
        {[
          n(a.projectCount, "own public project"),
          a.privateProjects > 0 ? `${a.privateProjects} private (counted, never named)` : "",
          a.totalStars > 0 ? `${a.totalStars} stars` : "",
          a.followers != null ? n(a.followers, "follower") : "",
          a.memberSince ? `since ${fmtDate(a.memberSince)}` : "",
        ]
          .filter(Boolean)
          .join(" · ")}
      </p>
      {a.activity && (
        <p className="mt-1 text-sm text-dim">
          {a.activity.lastActiveAt
            ? `Last active ${fmtDate(a.activity.lastActiveAt)}. Since ${a.activity.since ? fmtDate(a.activity.since) : "—"}: ${n(a.activity.pushes, "push", "pushes")}, ${n(a.activity.pullRequests, "pull request")}, ${n(a.activity.reviews, "review")}, ${n(a.activity.issues, "issue")}.`
            : "No recent public activity."}
        </p>
      )}
      {a.languages.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {a.languages.slice(0, 12).map((l) => (
            <span key={l.name} className="rounded-full border border-hair bg-surface-2 px-2 py-0.5 text-xs">
              {l.name} · {l.projects}
            </span>
          ))}
        </div>
      )}
      {a.projects.length > 0 && (
        <ul className="mt-3 grid gap-2 md:grid-cols-2">
          {a.projects.map((p) => (
            <li key={p.url} className="rounded-lg border border-hair p-3 text-sm">
              <a href={p.url} target="_blank" rel="noreferrer" className="font-semibold underline underline-offset-2">
                {p.name}
              </a>
              <span className="text-xs text-faint">
                {p.language ? ` · ${p.language}` : ""}
                {p.stars > 0 ? ` · ${p.stars}★` : ""}
                {p.updatedAt ? ` · ${fmtDate(p.updatedAt)}` : ""}
              </span>
              {p.description && <div className="mt-1 text-xs text-dim">{p.description}</div>}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
