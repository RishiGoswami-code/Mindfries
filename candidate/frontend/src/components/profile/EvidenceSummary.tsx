import type { Assessment } from "@/lib/dashboard/data";

/**
 * What this candidate's evidence adds up to so far — the profile page's
 * answer to "why should this be here at all", given the product's whole
 * differentiator is evidence over a bare score (PRD's framing, and the same
 * one RecordedPanel ("What gets recorded") leads with on the dashboard).
 *
 * Every number is counted from the candidate's own real assessments — the
 * same list the dashboard renders — so a brand-new account reads 0 across
 * the board instead of someone else's history.
 */
export function EvidenceSummary({ items = [] }: { items?: Assessment[] }) {
  const count = (...statuses: Assessment["status"][]) => items.filter((a) => statuses.includes(a.status)).length;

  const rows = [
    { label: "Assessments completed", value: count("submitted", "closed") },
    { label: "In progress", value: count("in-progress") },
    { label: "Open to start", value: count("invited") },
  ];

  return (
    <div>
      <p className="text-[13px] leading-relaxed text-[#5E5A82]">
        Companies don&apos;t see a score — they see how you worked. This is what that adds up to.
      </p>

      <dl className="mt-4 grid grid-cols-3 gap-3 border-t border-[#ECE9F6] pt-4">
        {rows.map((row) => (
          <div key={row.label}>
            <dd className="text-2xl leading-none font-semibold tabular-nums text-[#100E2E]">{row.value}</dd>
            <dt className="mt-1.5 text-[11.5px] leading-snug text-[#6E6A8F]">{row.label}</dt>
          </div>
        ))}
      </dl>
    </div>
  );
}
