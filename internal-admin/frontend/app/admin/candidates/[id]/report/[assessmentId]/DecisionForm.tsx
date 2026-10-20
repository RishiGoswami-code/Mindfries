"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";
import { Button, Field, Select, Textarea } from "@/components/ui";
import { RECOMMENDATION } from "@/components/admin/CandidateCard";
import { saveAdminDecision, type DecisionState } from "./actions";

function Submit() {
  const { pending } = useFormStatus();
  return (
    <Button type="submit" size="sm" disabled={pending}>
      {pending ? "Saving…" : "Save decision"}
    </Button>
  );
}

/** A Mindfries admin's hiring decision — the same one a company's reviewer records. */
export function DecisionForm({ candidateId, assessmentId, decision, note }: { candidateId: string; assessmentId: string; decision: string | null; note: string | null }) {
  const save = saveAdminDecision.bind(null, candidateId, assessmentId);
  const [state, action] = useActionState<DecisionState, FormData>(save, { error: null, success: false });
  return (
    <form action={action} className="mt-4 space-y-3 border-t border-hair pt-4">
      <div className="grid gap-3 sm:grid-cols-[14rem_1fr]">
        <Field label="Decision">
          <Select name="recommendation" defaultValue={decision ?? ""}>
            <option value="">Not decided</option>
            {Object.entries(RECOMMENDATION).map(([value, r]) => (
              <option key={value} value={value}>
                {r.label}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Why" hint="Seen by the company and Mindfries. The candidate sees the decision, not this note.">
          <Textarea name="note" rows={2} maxLength={4000} defaultValue={note ?? ""} />
        </Field>
      </div>
      <div className="flex items-center gap-3">
        <Submit />
        {state.success && !state.error && <p className="text-[13px] text-dim">Saved.</p>}
      </div>
      {state.error && (
        <p role="alert" className="rounded-xl bg-[#fdecef] px-3 py-2 text-[13px] text-[#a6203c]">
          {state.error}
        </p>
      )}
    </form>
  );
}
