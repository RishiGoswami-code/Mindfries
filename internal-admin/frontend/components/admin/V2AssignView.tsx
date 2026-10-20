"use client";

import { useState, useTransition } from "react";
import { Button, Field, Input, PageHeader, Select, Textarea } from "@/components/ui";
import { toast } from "@/components/admin/toast";
import { assignV2Task } from "@/app/admin/actions";
import { V2_ROLES, type V2Product } from "@/lib/v2-roles";

export function V2AssignView({ products, engineReady }: { products: V2Product[]; engineReady: boolean }) {
  const [pending, start] = useTransition();
  // Which roles the registry actually covers (union across products). When we
  // know it, uncovered roles are disabled in the picker — the authoritative
  // check still happens server-side at assign time.
  const covered = new Set(products.flatMap((p) => p.Roles ?? []));
  const knowCoverage = engineReady && covered.size > 0;
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [role, setRole] = useState(
    (knowCoverage ? V2_ROLES.find((r) => covered.has(r.value)) : undefined)?.value ?? V2_ROLES[0].value,
  );
  const [jd, setJd] = useState("");
  const [title, setTitle] = useState("");
  const [result, setResult] = useState<{ assessmentId: string; setupCode: string | null } | null>(null);

  function assign() {
    start(async () => {
      const res = await assignV2Task({ candidateEmail: email, candidateName: name, role, jd, title });
      if (!res.ok) {
        toast.error("Couldn't assign the task", res.error);
        return;
      }
      setResult({ assessmentId: res.assessmentId, setupCode: res.setupCode });
      toast.success("V2 task assigned", res.setupCode ? "Setup code minted" : "Assessment recorded");
    });
  }

  return (
    <>
      <PageHeader title="Assign a V2 task" eyebrow="V2 Engine" />

      {!engineReady && (
        <div className="rounded-lg border border-hair bg-surface-2 px-4 py-3 text-sm text-dim">
          V2_ENGINE_URL isn&apos;t set, so products can&apos;t be listed and setup codes can&apos;t be
          minted here. The assessment is still recorded; the candidate gets a code from their dashboard.
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-[1.3fr_1fr]">
        <div className="rounded-xl border border-hair bg-surface p-5 space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Candidate email">
              <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="candidate@acme.test" />
            </Field>
            <Field label="Candidate name" hint="Optional">
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Jo Candidate" />
            </Field>
          </div>
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
              <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Backend take-home" />
            </Field>
          </div>
          <Field label="Job description" hint="The engine resolves a product and task from the role + this JD.">
            <Textarea rows={6} value={jd} onChange={(e) => setJd(e.target.value)} placeholder="Paste the role's job description — the more specific, the better the product/task match." />
          </Field>
          <div className="flex justify-end">
            <Button onClick={assign} disabled={pending || !email.trim() || !jd.trim()}>
              {pending ? "Assigning…" : "Assign task"}
            </Button>
          </div>

          {result && (
            <div className="rounded-lg border border-hair bg-surface-2 p-4 text-sm space-y-2">
              <div className="font-semibold text-dim">Assigned ✓</div>
              <div className="text-faint">Assessment <code>{result.assessmentId}</code> — the candidate will see it on their dashboard.</div>
              {result.setupCode ? (
                <div className="space-y-1">
                  <div>Setup code: <code className="text-accent">{result.setupCode}</code></div>
                  <div className="text-faint">The candidate runs the installer, then <code>mindfries setup {result.setupCode}</code> and <code>mindfries start</code>.</div>
                </div>
              ) : (
                <div className="text-faint">No setup code minted (engine not configured) — the candidate gets one from their dashboard.</div>
              )}
            </div>
          )}
        </div>

        <div className="rounded-xl border border-hair bg-surface p-5">
          <div className="eyebrow mb-3">Products in the registry</div>
          {products.length === 0 ? (
            <div className="text-sm text-faint">No products listed (engine not reachable, or none seeded).</div>
          ) : (
            <ul className="space-y-3">
              {products.map((p) => (
                <li key={p.Slug} className="rounded-lg border border-hair bg-surface-2 p-3">
                  <div className="text-sm font-semibold">{p.Name}</div>
                  <div className="text-xs text-faint">{p.Domain}</div>
                  <div className="mt-1.5 flex flex-wrap gap-1">
                    {(p.Roles ?? []).map((r) => (
                      <span key={r} className="rounded bg-surface px-1.5 py-0.5 text-[11px] text-dim">{r}</span>
                    ))}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </>
  );
}
