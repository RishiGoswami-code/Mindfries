"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";
import { Button, Field, Select, Textarea } from "@/components/ui";
import type { TaskVariant } from "@/lib/types";
import { saveRoleBrief, type RoleBriefState } from "./actions";

function Submit() {
  const { pending } = useFormStatus();
  return (
    <Button type="submit" size="sm" disabled={pending}>
      {pending ? "Saving…" : "Save"}
    </Button>
  );
}

/**
 * What this role's assessment tests. Every candidate invited to the role
 * gets their own task, written for them from this brief — the same skills
 * and level, a different problem — so nobody can pass answers along.
 */
export function RoleBriefForm({ roleId, brief, taskVariant }: { roleId: string; brief: string; taskVariant: TaskVariant }) {
  const [state, action] = useActionState<RoleBriefState, FormData>(saveRoleBrief.bind(null, roleId), { error: null, saved: false });

  return (
    <form action={action} className="hair-card space-y-3 p-4">
      <div>
        <h2 className="text-sm font-bold">What the assessment tests</h2>
        <p className="mt-0.5 text-xs text-dim">
          Each candidate you invite gets their own task, generated from this — the same skills at the same level, on a different problem. Changes
          apply to candidates invited from now on.
        </p>
      </div>
      <Field label="Brief" hint="The work, the level, what to look for">
        <Textarea
          name="assessmentBrief"
          rows={5}
          maxLength={4000}
          defaultValue={brief}
          placeholder="Mid-level backend engineer for our payments team. Python services, REST APIs, careful handling of money and retries. Look for debugging discipline and clear tests."
        />
      </Field>
      <div className="flex flex-wrap items-end gap-3">
        <div className="w-48">
          <Field label="Kind of task">
            <Select name="taskVariant" defaultValue={taskVariant}>
              <option value="bug_fix">Find and fix a bug</option>
              <option value="debug">Debug a failure</option>
              <option value="feature">Build a feature</option>
              <option value="refactor">Refactor</option>
            </Select>
          </Field>
        </div>
        <Submit />
        {state.saved && !state.error && <span className="text-[13px] text-[color:var(--color-success)]">Saved.</span>}
      </div>
      {state.error && (
        <p role="alert" className="rounded-xl bg-[#fdecef] px-3 py-2 text-[13px] text-[#a6203c]">
          {state.error}
        </p>
      )}
    </form>
  );
}
