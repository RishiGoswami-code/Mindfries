"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";
import { Button, Field, Input, Select, Textarea } from "@/components/ui";
import { createRole, type CreateRoleState } from "./actions";

function Submit() {
  const { pending } = useFormStatus();
  return (
    <Button type="submit" disabled={pending}>
      {pending ? "Creating…" : "Create role"}
    </Button>
  );
}

export function NewRoleForm() {
  const [state, action] = useActionState<CreateRoleState, FormData>(createRole, { error: null });

  return (
    <form action={action} className="space-y-4">
      <Field label="Role title">
        <Input name="title" required placeholder="Senior Backend Engineer" autoFocus />
      </Field>
      <Field label="Tech stack" hint="Comma-separated, optional">
        <Input name="techStack" placeholder="Python, PostgreSQL, AWS" />
      </Field>
      <div className="grid grid-cols-2 gap-4">
        <Field label="Duration (minutes)" hint="Leave blank to decide later">
          <Input name="durationMin" type="number" min={1} placeholder="60" />
        </Field>
        <Field label="Kind of task">
          <Select name="taskVariant" defaultValue="bug_fix">
            <option value="bug_fix">Find and fix a bug</option>
            <option value="debug">Debug a failure</option>
            <option value="feature">Build a feature</option>
            <option value="refactor">Refactor</option>
          </Select>
        </Field>
      </div>
      <Field label="What the assessment tests" hint="Each candidate you invite gets their own task, generated from this">
        <Textarea
          name="assessmentBrief"
          rows={4}
          maxLength={4000}
          placeholder="Mid-level backend engineer for our payments team. Python services, REST APIs, careful handling of money and retries. Look for debugging discipline and clear tests."
        />
      </Field>

      {state.error && (
        <p role="alert" className="rounded-xl bg-[#fdecef] px-3 py-2.5 text-[13px] leading-relaxed text-[#a6203c]">
          {state.error}
        </p>
      )}

      <Submit />
    </form>
  );
}
