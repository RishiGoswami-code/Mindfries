import type { Metadata } from "next";
import Link from "next/link";
import { fetchV2Reviews, v2Ready, type V2ReviewItem } from "@/lib/v2";
import { PageHeader, Pill } from "@/components/ui";

export const metadata: Metadata = { title: "V2 Reviews" };
export const dynamic = "force-dynamic";

function outcome(r: V2ReviewItem) {
  if (!r.submitted) return <Pill tone="gray">in progress</Pill>;
  if (!r.graded) return <Pill tone="amber">awaiting grade</Pill>;
  return r.passed ? <Pill tone="green">passed</Pill> : <Pill tone="coral">failed</Pill>;
}

export default async function V2ReviewsPage() {
  const ready = v2Ready();
  const reviews = ready ? await fetchV2Reviews() : [];

  return (
    <div className="space-y-6">
      <PageHeader eyebrow="V2 Engine" title="Candidate reviews" />

      {!ready && (
        <div className="rounded-lg border border-hair bg-surface-2 px-4 py-3 text-sm text-dim">
          V2_ENGINE_URL isn&apos;t set, so reviews can&apos;t be loaded from the engine.
        </div>
      )}

      {ready && reviews.length === 0 ? (
        <div className="text-sm text-faint">No candidate sessions yet.</div>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-hair bg-surface">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-hair text-left text-faint">
                <th className="px-4 py-2.5 font-medium">Task</th>
                <th className="px-4 py-2.5 font-medium">Role</th>
                <th className="px-4 py-2.5 font-medium">Session</th>
                <th className="px-4 py-2.5 font-medium">Outcome</th>
                <th className="px-4 py-2.5 font-medium">Started</th>
              </tr>
            </thead>
            <tbody>
              {reviews.map((r) => (
                <tr key={r.session_id} className="border-b border-hair/50 last:border-0 hover:bg-surface-2">
                  <td className="px-4 py-2.5">
                    <Link href={`/admin/v2/reviews/${r.session_id}`} className="font-medium text-accent hover:underline">
                      {r.assessment_title || r.bank_task_id}
                    </Link>
                    <div className="text-xs text-faint">{r.product_slug} · {r.bank_task_id}</div>
                  </td>
                  <td className="px-4 py-2.5">{r.role}</td>
                  <td className="px-4 py-2.5">{r.submitted ? (r.submission_status ?? "submitted") : r.status}</td>
                  <td className="px-4 py-2.5">{outcome(r)}</td>
                  <td className="px-4 py-2.5 text-faint">{new Date(r.started_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
