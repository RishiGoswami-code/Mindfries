"use client";

import { useState, useTransition } from "react";
import { Button, Field, Input, PageHeader, Pill, Select, Textarea } from "@/components/ui";
import { toast } from "@/components/admin/toast";
import { assignV2Batch } from "@/app/admin/actions";
import { V2_ROLES, type V2Product } from "@/lib/v2-roles";

type Sent = { email: string; setupCode?: string; error?: string; role: string; at: string };

export function V2AssignView({ products, engineReady }: { products: V2Product[]; engineReady: boolean }) {
  const [pending, start] = useTransition();
  // Which roles the registry covers; uncovered roles are disabled in the picker.
  const covered = new Set(products.flatMap((p) => p.Roles ?? []));
  const knowCoverage = engineReady && covered.size > 0;

  const [emails, setEmails] = useState("");
  const [role, setRole] = useState(
    (knowCoverage ? V2_ROLES.find((r) => covered.has(r.value)) : undefined)?.value ?? V2_ROLES[0].value,
  );
  const [jd, setJd] = useState("");
  const [title, setTitle] = useState("");
  const [sent, setSent] = useState<Sent[]>([]);

  const roleLabel = (v: string) => V2_ROLES.find((r) => r.value === v)?.label ?? v;

  function assign() {
    const list = emails.split(/[\s,;]+/).map((e) => e.trim()).filter(Boolean);
    if (list.length === 0) {
      toast.error("Add at least one candidate email");
      return;
    }
    start(async () => {
      const res = await assignV2Batch({ emails: list, role, jd, title });
      if (!res.ok) {
        toast.error("Couldn't send invitations", res.error);
        return;
      }
      // Reuse the same role + JD: keep them, clear only the candidate list, and
      // keep a running record of what was sent (so the form never goes blank).
      const stamp = new Date().toLocaleTimeString();
      setSent((prev) => [...res.results.map((r) => ({ ...r, role, at: stamp })), ...prev]);
      setEmails("");
      const ok = res.results.filter((r) => r.setupCode).length;
      const bad = res.results.length - ok;
      toast.success(`${ok} invitation${ok === 1 ? "" : "s"} sent`, bad ? `${bad} failed` : "Same role + JD kept for more candidates");
    });
  }

  return (
    <>
      <PageHeader title="Assign a V2 task" eyebrow="V2 Engine" />

      {!engineReady && (
        <div className="rounded-lg border border-hair bg-surface-2 px-4 py-3 text-sm text-dim">
          V2_ENGINE_URL isn&apos;t set, so products can&apos;t be listed and setup codes can&apos;t be minted here.
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-[1.3fr_1fr]">
        <div className="space-y-4 rounded-xl border border-hair bg-surface p-5">
          <Field label="Candidate emails" hint="One or more — separated by new lines, commas or spaces. All get the same role + JD.">
            <Textarea
              rows={3}
              value={emails}
              onChange={(e) => setEmails(e.target.value)}
              placeholder={"alice@acme.test\nbob@acme.test"}
            />
          </Field>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Role" hint="The engine picks a product for this role">
              <Select value={role} onChange={(e) => setRole(e.target.value)}>
                {V2_ROLES.map((r) => {
                  const uncovered = knowCoverage && !covered.has(r.value);
                  return (
                    <option key={r.value} value={r.value} disabled={uncovered}>
                      {r.label}{uncovered ? " — no product yet" : ""}
                    </option>
                  );
                })}
              </Select>
            </Field>
            <Field label="Assessment title" hint="Optional; shown to the candidate">
              <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Frontend take-home" />
            </Field>
          </div>
          <Field label="Job description" hint="Reused for every candidate above. Same JD → same task (no regeneration); change it only to generate a new task.">
            <Textarea rows={6} value={jd} onChange={(e) => setJd(e.target.value)} placeholder="Paste the role's job description — the more specific, the better the product/task match." />
          </Field>
          <div className="flex justify-end">
            <Button onClick={assign} disabled={pending || !emails.trim() || !jd.trim()}>
              {pending ? "Sending…" : "Send invitations"}
            </Button>
          </div>
        </div>

        <div className="rounded-xl border border-hair bg-surface p-5">
          <div className="eyebrow mb-3">Sent invitations</div>
          {sent.length === 0 ? (
            <div className="text-sm text-faint">Nothing sent yet. Codes appear here as you invite candidates.</div>
          ) : (
            <ul className="space-y-2.5">
              {sent.map((s, i) => (
                <li key={`${s.email}-${i}`} className="rounded-lg border border-hair bg-surface-2 p-3 text-sm">
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate font-medium">{s.email}</span>
                    {s.setupCode ? <Pill tone="green">sent</Pill> : <Pill tone="coral">failed</Pill>}
                  </div>
                  <div className="mt-1 text-xs text-faint">{roleLabel(s.role)} · {s.at}</div>
                  {s.setupCode ? (
                    <div className="mt-1">Code: <code className="text-accent">{s.setupCode}</code></div>
                  ) : (
                    <div className="mt-1 text-[#9f2b33]">{s.error}</div>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </>
  );
}
