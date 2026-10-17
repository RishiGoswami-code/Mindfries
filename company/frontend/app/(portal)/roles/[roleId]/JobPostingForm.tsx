"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";
import { Button, Field, Input, Select, Textarea } from "@/components/ui";
import type { JobPosting } from "@/lib/types";
import { saveJobPosting, type JobPostingState } from "./actions";

function Submit() {
  const { pending } = useFormStatus();
  return (
    <Button type="submit" size="sm" disabled={pending}>
      {pending ? "Saving…" : "Save"}
    </Button>
  );
}

/**
 * The role as a job on the candidates' job board. Listing it lets anyone
 * signed in to the candidate portal find it and apply; applicants arrive in
 * this role's pipeline as "Applied", and you invite the ones you want to the
 * assessment. "Every applicant can take the assessment" skips that step:
 * applying opens it at once.
 */
export function JobPostingForm({ roleId, posting, roleOpen }: { roleId: string; posting: JobPosting; roleOpen: boolean }) {
  const [state, action] = useActionState<JobPostingState, FormData>(saveJobPosting.bind(null, roleId), { error: null, saved: false });

  return (
    <form action={action} className="hair-card space-y-3 p-4">
      <div>
        <h2 className="text-sm font-bold">Job posting</h2>
        <p className="mt-0.5 text-xs text-dim">
          List this role on the candidates&apos; job board so people can find it and apply. Applicants appear below as{" "}
          <span className="font-semibold">Applied</span>; you choose who takes the assessment.
          {posting.publishedAt && ` First posted ${new Date(posting.publishedAt).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" })}.`}
        </p>
      </div>

      <label className="flex items-start gap-2.5 text-sm">
        <input type="checkbox" name="listed" defaultChecked={posting.listed} className="mt-0.5" />
        <span>
          <span className="font-semibold">List on the job board</span>
          <span className="block text-xs text-dim">
            {roleOpen ? "Shown while the role is open." : "This role is closed, so it won't show until you reopen it."}
          </span>
        </span>
      </label>

      <label className="flex items-start gap-2.5 text-sm">
        <input type="checkbox" name="selfAssessment" defaultChecked={posting.selfAssessment} className="mt-0.5" />
        <span>
          <span className="font-semibold">Every applicant can take the assessment</span>
          <span className="block text-xs text-dim">
            Applying opens the assessment straight away, with a task written for that candidate — no invitation needed. Off, you invite the
            applicants you want.
          </span>
        </span>
      </label>

      <Field label="Description" hint="What the job is, the team, what you're looking for. Needed to list it.">
        <Textarea name="description" rows={6} maxLength={8000} defaultValue={posting.description} />
      </Field>

      <div className="grid gap-3 sm:grid-cols-3">
        <Field label="Location">
          <Input name="location" maxLength={120} defaultValue={posting.location} placeholder="Bengaluru, India" />
        </Field>
        <Field label="Work mode">
          <Select name="workMode" defaultValue={posting.workMode ?? ""}>
            <option value="">Not said</option>
            <option value="remote">Remote</option>
            <option value="hybrid">Hybrid</option>
            <option value="onsite">On-site</option>
          </Select>
        </Field>
        <Field label="Employment type">
          <Select name="employmentType" defaultValue={posting.employmentType ?? ""}>
            <option value="">Not said</option>
            <option value="full_time">Full-time</option>
            <option value="part_time">Part-time</option>
            <option value="contract">Contract</option>
            <option value="internship">Internship</option>
          </Select>
        </Field>
      </div>

      <div className="flex items-center gap-3">
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
