"use client";

import { useState, useTransition } from "react";
import { grantAccess, declineAccess } from "@/app/admin/access/actions";
import { Button } from "@/components/ui";
import { toast } from "@/components/admin/toast";
import type { Plan } from "@/lib/types";

const PLANS: { value: Plan; label: string }[] = [
  { value: "trial", label: "Trial" },
  { value: "starter", label: "Starter" },
  { value: "growth", label: "Growth" },
  { value: "enterprise", label: "Enterprise" },
];

const field = "h-10 w-full rounded-xl border border-hair bg-surface px-3 text-sm outline-none transition focus:border-accent";

/**
 * Grant access and credits to one self-signed-up company (or top one up).
 * 1 credit = 1 candidate invited. Pending companies also get Decline.
 */
export function GrantAccessForm({ companyId, plan, pending }: { companyId: string; plan: Plan; pending: boolean }) {
  const [open, setOpen] = useState(false);
  const [credits, setCredits] = useState("");
  const [selectedPlan, setPlan] = useState<Plan>(plan);
  const [note, setNote] = useState("");
  const [notify, setNotify] = useState(true);
  const [busy, start] = useTransition();

  if (!open) {
    return (
      <Button size="sm" variant={pending ? "primary" : "soft"} onClick={() => setOpen(true)}>
        {pending ? "Grant access" : "Add credits"}
      </Button>
    );
  }

  const grant = () =>
    start(async () => {
      const r = await grantAccess({ companyId, credits: Number(credits), plan: selectedPlan, note, notify });
      if (r.ok) {
        toast.success(pending ? "Access granted" : "Credits added", r.message);
        setOpen(false);
        setCredits("");
        setNote("");
      } else toast.error("Couldn't grant that", r.error);
    });

  const decline = () =>
    start(async () => {
      const r = await declineAccess(companyId);
      if (r.ok) toast.success("Declined", r.message);
      else toast.error("Couldn't decline", r.error);
    });

  return (
    <div className="w-[320px] max-w-full space-y-3 rounded-2xl border border-hair bg-[#fafafc] p-4 text-left">
      <label className="block text-[13px] font-semibold">
        Credits to add
        <input type="number" min={1} step={1} inputMode="numeric" value={credits} onChange={(e) => setCredits(e.target.value)} placeholder="1 credit = 1 candidate" className={`mt-1 ${field}`} />
      </label>
      <label className="block text-[13px] font-semibold">
        Plan
        <select value={selectedPlan} onChange={(e) => setPlan(e.target.value as Plan)} className={`mt-1 ${field}`}>
          {PLANS.map((p) => (
            <option key={p.value} value={p.value}>
              {p.label}
            </option>
          ))}
        </select>
      </label>
      <label className="block text-[13px] font-semibold">
        Note <span className="font-normal text-dim">(only Mindfries sees this)</span>
        <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} maxLength={1000} className="mt-1 w-full resize-none rounded-xl border border-hair bg-surface px-3 py-2 text-sm outline-none transition focus:border-accent" />
      </label>
      <label className="flex items-center gap-2 text-[13px]">
        <input type="checkbox" checked={notify} onChange={(e) => setNotify(e.target.checked)} className="h-4 w-4 accent-[var(--color-accent)]" />
        Email the company when it&apos;s done
      </label>
      <div className="flex flex-wrap items-center gap-2 pt-1">
        <Button size="sm" onClick={grant} disabled={busy || !credits}>
          {busy ? "Saving…" : pending ? "Grant access" : "Add credits"}
        </Button>
        {pending && (
          <Button size="sm" variant="ghost" onClick={decline} disabled={busy}>
            Decline
          </Button>
        )}
        <button type="button" onClick={() => setOpen(false)} disabled={busy} className="ml-auto text-[13px] text-dim hover:text-ink">
          Cancel
        </button>
      </div>
    </div>
  );
}
