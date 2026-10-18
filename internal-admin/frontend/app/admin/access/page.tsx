import type { Metadata } from "next";
import { PageHeader } from "@/components/ui";
import { Panel, SoftPill } from "@/components/admin/cards";
import { GrantAccessForm } from "@/components/admin/GrantAccessForm";
import { listSelfSignupCompanies, type AccessCompany } from "@/lib/company-access";
import { supabaseReady } from "@/lib/supabase";
import { fmtDate } from "@/lib/format";

export const metadata: Metadata = { title: "Access requests" };

export const dynamic = "force-dynamic";

/**
 * Companies that signed themselves up on the company portal. They start
 * with hiring locked and 0 credits; granting access here switches it on and
 * adds credits (1 credit = 1 candidate invited). Companies Mindfries
 * onboarded itself aren't metered and don't appear here.
 */
export default async function AccessRequestsPage() {
  const companies = supabaseReady() ? await listSelfSignupCompanies() : [];
  const pending = companies.filter((c) => c.status === "pending");
  const active = companies.filter((c) => c.status === "active");
  const declined = companies.filter((c) => c.status === "declined");

  return (
    <div className="space-y-6">
      <PageHeader eyebrow="Product" title="Access requests">
        Companies that signed up on their own. Nothing they can spend until you grant it — 1 credit is 1 candidate invited.
      </PageHeader>

      <Panel title="Waiting for access" count={pending.length} countTone={pending.length ? "amber" : "gray"} subtitle="Signed up and can look around, but can't create roles or invite until you grant access.">
        <CompanyTable rows={pending} empty="No companies waiting." />
      </Panel>

      <Panel title="With access" count={active.length} countTone="green" subtitle="Self-serve companies that can hire. Top up their credits here.">
        <CompanyTable rows={active} empty="None yet." />
      </Panel>

      {declined.length > 0 && (
        <Panel title="Declined" count={declined.length} countTone="red" subtitle="They can still sign in, but can't hire. Granting access switches it on.">
          <CompanyTable rows={declined} empty="" />
        </Panel>
      )}
    </div>
  );
}

function CompanyTable({ rows, empty }: { rows: AccessCompany[]; empty: string }) {
  if (!rows.length) return <p className="px-5 py-6 text-sm text-dim">{empty}</p>;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-hair bg-[#fafafc] text-left text-[13px] font-semibold text-ink">
            <th className="px-5 py-3 font-semibold">Company</th>
            <th className="px-5 py-3 font-semibold">Contact</th>
            <th className="px-5 py-3 font-semibold">Signed up</th>
            <th className="px-5 py-3 font-semibold">Credits</th>
            <th className="px-5 py-3 font-semibold" aria-label="Actions" />
          </tr>
        </thead>
        <tbody>
          {rows.map((c) => (
            <tr key={c.id} className="border-b border-hair align-top last:border-0">
              <td className="px-5 py-4">
                <div className="font-semibold text-ink">{c.name}</div>
                {c.website && <div className="text-[13px] text-dim">{c.website}</div>}
                {c.requestedAt && c.status !== "declined" && (
                  <div className="mt-1.5">
                    <SoftPill tone="violet">Reminded {fmtDate(c.requestedAt)}</SoftPill>
                  </div>
                )}
              </td>
              <td className="px-5 py-4">
                {c.contact ? (
                  <>
                    <div className="text-ink">{c.contact.name}</div>
                    <a href={`mailto:${c.contact.email}`} className="text-[13px] text-accent hover:underline">
                      {c.contact.email}
                    </a>
                  </>
                ) : (
                  <span className="text-dim">—</span>
                )}
              </td>
              <td className="px-5 py-4 whitespace-nowrap text-dim">{fmtDate(c.createdAt)}</td>
              <td className="px-5 py-4">
                <div className="font-extrabold tabular-nums text-ink">{c.credits ?? "—"}</div>
                {c.grants[0] && (
                  <div className="mt-1 text-[12px] leading-snug text-dim">
                    Last: +{c.grants[0].amount} by {c.grants[0].by}, {fmtDate(c.grants[0].at)}
                  </div>
                )}
              </td>
              <td className="px-5 py-4 text-right">
                <GrantAccessForm companyId={c.id} plan={c.plan} pending={c.status !== "active"} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
