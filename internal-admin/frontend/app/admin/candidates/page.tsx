import { listCandidateCards } from "@/lib/candidates";
import { supabaseReady } from "@/lib/supabase";
import { PageHeader } from "@/components/ui";
import { CandidateCard } from "@/components/admin/CandidateCard";

export const dynamic = "force-dynamic";

/**
 * Every candidate account, as cards: their latest assessment's outcome and
 * what their linked GitHub, GitLab and LinkedIn accounts show. The search is
 * a plain GET form, filtered here — candidate numbers are small enough.
 */
export default async function CandidatesPage({ searchParams }: { searchParams: Promise<{ q?: string; linked?: string }> }) {
  const { q = "", linked = "" } = await searchParams;
  const all = await listCandidateCards();
  const query = q.trim().toLowerCase();
  const shown = all.filter((c) => {
    if (linked === "1" && !c.github && !c.gitlab && !c.linkedin) return false;
    if (!query) return true;
    return [c.name, c.email, c.headline, c.github?.handle, c.gitlab?.handle, c.linkedin?.name, ...c.languages.map((l) => l.name)]
      .filter(Boolean)
      .some((v) => String(v).toLowerCase().includes(query));
  });
  const connected = all.filter((c) => c.github?.verified || c.gitlab?.verified || c.linkedin).length;

  return (
    <div className="space-y-6">
      <PageHeader eyebrow="Candidates" title="Candidates">
        {supabaseReady()
          ? `${all.length} candidate accounts · ${connected} signed in to at least one of GitHub, GitLab or LinkedIn.`
          : "Connect Supabase to see candidate accounts."}
      </PageHeader>

      <form className="flex flex-wrap items-center gap-3" action="/admin/candidates">
        <input
          name="q"
          defaultValue={q}
          placeholder="Search name, email, username or language…"
          className="w-full max-w-md rounded-lg border border-hair bg-white px-3 py-2 text-sm outline-none focus:border-[#6d3fe0]"
        />
        <label className="flex items-center gap-2 text-sm text-dim">
          <input type="checkbox" name="linked" value="1" defaultChecked={linked === "1"} /> With linked accounts
        </label>
        <button type="submit" className="rounded-lg border border-hair px-3 py-2 text-sm font-semibold hover:bg-surface-2">
          Filter
        </button>
      </form>

      {shown.length === 0 ? (
        <div className="hair-card p-8 text-center text-sm text-dim">{all.length === 0 ? "No candidate accounts yet." : "No candidates match."}</div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {shown.map((c) => (
            <CandidateCard key={c.id} c={c} />
          ))}
        </div>
      )}
    </div>
  );
}
