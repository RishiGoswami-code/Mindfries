"use client";

import { useState, useTransition } from "react";
import { Button, Field, Input, Select } from "@/components/ui";
import { Panel } from "@/components/admin/cards";
import { planLabel } from "@/lib/format";
import type { Plan } from "@/lib/types";
import { onboardCompany } from "@/app/admin/actions";
import { toast } from "@/components/admin/toast";

// `initial` comes from a target's "Onboard →" link: the company and the person
// most likely to be its admin, plus the target to mark won once this succeeds.
export function OnboardForm({ initial }: { initial?: { company?: string; adminEmail?: string; targetId?: string } }) {
  const [company, setCompany] = useState(initial?.company ?? "");
  const [adminEmail, setAdminEmail] = useState(initial?.adminEmail ?? "");
  const [targetId, setTargetId] = useState(initial?.targetId);
  const [password, setPassword] = useState("");
  const [plan, setPlan] = useState<Plan>("starter");
  const [monthlyCost, setMonthlyCost] = useState(0);
  const [pending, start] = useTransition();

  function submit() {
    start(async () => {
      const res = await onboardCompany({ company, adminEmail, plan, monthlyCost, targetId, password: password.trim() || undefined });
      if (res.ok) {
        toast.success("Workspace created", `Account ready for ${adminEmail}.`);
        setCompany(""); setAdminEmail(""); setPassword(""); setPlan("starter"); setMonthlyCost(0); setTargetId(undefined);
      } else {
        toast.error("Couldn't onboard the company", res.error);
      }
    });
  }

  return (
    <Panel title="Onboard a company" subtitle="Creates their workspace and enables immediate sign-in.">
      <div className="px-6 py-5">
      {targetId && (
        <div className="mb-4 rounded-lg bg-accent-soft px-3 py-2 text-sm text-accent">
          From a target — onboarding it marks the target as won.
        </div>
      )}
      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <Field label="Company"><Input value={company} onChange={(e) => setCompany(e.target.value)} placeholder="Acme Robotics" /></Field>
          <Field label="Admin email" hint="Account login identifier."><Input type="email" value={adminEmail} onChange={(e) => setAdminEmail(e.target.value)} placeholder="admin@acme.io" /></Field>
        </div>
        <div className="grid grid-cols-3 gap-4">
          <Field label="Initial Password" hint="Optional (min 8 chars). If blank, can set on first login.">
            <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Min 8 characters" />
          </Field>
          <Field label="Plan">
            <Select value={plan} onChange={(e) => setPlan(e.target.value as Plan)}>
              {(Object.keys(planLabel) as Plan[]).map((p) => <option key={p} value={p}>{planLabel[p]}</option>)}
            </Select>
          </Field>
          <Field label="Monthly cost to us ($)" hint="Compute allocation.">
            <Input type="number" min={0} value={monthlyCost} onChange={(e) => setMonthlyCost(Math.max(0, Number(e.target.value) || 0))} />
          </Field>
        </div>
        <Button onClick={submit} disabled={pending || !company.trim() || !adminEmail.trim()}>
          {pending ? "Creating…" : "Create & activate account"}
        </Button>
      </div>
      </div>
    </Panel>
  );
}
