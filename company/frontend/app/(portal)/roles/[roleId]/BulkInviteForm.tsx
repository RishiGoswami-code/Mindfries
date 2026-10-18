"use client";

import { useActionState, useEffect, useRef } from "react";
import { useFormStatus } from "react-dom";
import { Button, Field, Input } from "@/components/ui";
import { bulkInviteCandidates, type BulkInviteState } from "./actions";

function Submit() {
  const { pending } = useFormStatus();
  return (
    <Button type="submit" size="sm" disabled={pending}>
      {pending ? "Inviting…" : "Invite everyone"}
    </Button>
  );
}

/**
 * Invite a whole list of candidates to this role at once. They all share the
 * role's one task — already generated — so there's no per-candidate wait. One
 * credit per candidate actually invited.
 */
export function BulkInviteForm({ roleId }: { roleId: string }) {
  const inviteToThisRole = bulkInviteCandidates.bind(null, roleId);
  const [state, action] = useActionState<BulkInviteState, FormData>(inviteToThisRole, { error: null, success: false });
  const formRef = useRef<HTMLFormElement>(null);

  useEffect(() => {
    if (state.success) formRef.current?.reset();
  }, [state.success]);

  return (
    <form ref={formRef} action={action} className="hair-card flex flex-col gap-3 p-4">
      <Field label="Candidate emails" hint={"One per line (or comma-separated). “Name <email>” works too."}>
        <textarea
          name="candidates"
          required
          rows={6}
          placeholder={"jane@example.com\nRavi Kumar <ravi@example.com>\nsam@example.com"}
          className="w-full resize-y rounded-xl border border-[color:var(--hairline)] bg-white px-3 py-2 text-[13px] leading-relaxed outline-none focus:border-[color:var(--color-accent)]"
        />
      </Field>
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="w-40">
          <Field label="Due date" hint="Optional, applies to all">
            <Input name="dueDate" type="date" />
          </Field>
        </div>
        <Submit />
      </div>

      {state.error && (
        <p role="alert" className="rounded-xl bg-[#fdecef] px-3 py-2 text-[13px] leading-relaxed text-[#a6203c]">
          {state.error}
        </p>
      )}
      {state.success && !state.error && state.summary && (
        <p className="text-[13px] leading-relaxed text-[color:var(--color-success)]">{state.summary}</p>
      )}
    </form>
  );
}
