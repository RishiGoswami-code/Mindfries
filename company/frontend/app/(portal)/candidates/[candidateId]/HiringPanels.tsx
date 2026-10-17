"use client";

import { useActionState, useState } from "react";
import { useFormStatus } from "react-dom";
import { Button, Field, Input, Modal, Pill, Textarea } from "@/components/ui";
import type { Tone } from "@/lib/format";
import type { ApplicationEmail, ApplicationInterview, InterviewStatus } from "@/lib/types";
import { inviteApplicantToAssessment, type InviteApplicantState } from "../../roles/[roleId]/actions";
import { closeInterviewAction, rescheduleInterviewAction, scheduleInterviewAction, sendEmailAction, type HiringState } from "./hiring-actions";

const STATUS: Record<InterviewStatus, { label: string; tone: Tone }> = {
  scheduled: { label: "Waiting for the candidate", tone: "amber" },
  confirmed: { label: "Confirmed", tone: "green" },
  reschedule_requested: { label: "Asked for another time", tone: "coral" },
  cancelled_by_candidate: { label: "Cancelled by the candidate", tone: "gray" },
  cancelled: { label: "Cancelled", tone: "gray" },
  completed: { label: "Done", tone: "violet" },
};

function Submit({ children, pendingText }: { children: React.ReactNode; pendingText: string }) {
  const { pending } = useFormStatus();
  return (
    <Button type="submit" size="sm" disabled={pending}>
      {pending ? pendingText : children}
    </Button>
  );
}

function Result({ state }: { state: { error: string | null; notice: string | null } }) {
  if (state.error)
    return (
      <p role="alert" className="rounded-xl bg-[#fdecef] px-3 py-2 text-[13px] text-[#a6203c]">
        {state.error}
      </p>
    );
  if (state.notice) return <p className="text-[13px] text-[color:var(--color-success)]">{state.notice}</p>;
  return null;
}

const when = (iso: string) =>
  new Intl.DateTimeFormat("en-GB", { weekday: "short", day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(new Date(iso));

/** A `datetime-local` value for an ISO time, in this browser's timezone. */
function toLocalInput(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/**
 * The fields of an interview. A `datetime-local` value has no timezone, so
 * on submit the moment is worked out here, in the scheduler's own browser,
 * and sent as UTC with the timezone's name (for the email); the candidate
 * then sees it in their own local time.
 */
function InterviewFields({ interview }: { interview?: ApplicationInterview }) {
  const [local, setLocal] = useState(interview ? toLocalInput(interview.startsAt) : "");
  const iso = local ? new Date(local).toISOString() : "";
  return (
    <div className="space-y-3">
      <Field label="Interview">
        <Input name="title" required maxLength={120} defaultValue={interview?.title ?? ""} placeholder="Technical interview" />
      </Field>
      <div className="grid gap-3 sm:grid-cols-[1fr_120px]">
        <Field label="When" hint={`Your time (${Intl.DateTimeFormat().resolvedOptions().timeZone})`}>
          <Input type="datetime-local" required value={local} onChange={(e) => setLocal(e.target.value)} />
        </Field>
        <Field label="Minutes">
          <Input type="number" name="durationMin" min={10} max={480} step={5} defaultValue={interview?.durationMin ?? 45} />
        </Field>
      </div>
      <input type="hidden" name="startsAtIso" value={iso} />
      <input type="hidden" name="timeZone" value={Intl.DateTimeFormat().resolvedOptions().timeZone} />
      <Field label="Where" hint="A meeting link or an address — shown to the candidate as written">
        <Input name="location" maxLength={500} defaultValue={interview?.location ?? ""} placeholder="https://meet.google.com/…" />
      </Field>
      <Field label="Notes for the candidate">
        <Textarea name="notes" rows={3} maxLength={2000} defaultValue={interview?.notes ?? ""} placeholder="Who they'll meet, what to prepare." />
      </Field>
    </div>
  );
}

/** Someone who applied from the job board: their note, and the way to invite them to the assessment. */
export function ApplicantCard({
  roleId,
  applicationId,
  coverNote,
  selfAssessment,
  canInvite,
}: {
  roleId: string;
  applicationId: string;
  coverNote: string | null;
  selfAssessment: boolean;
  canInvite: boolean;
}) {
  const [state, action] = useActionState<InviteApplicantState, FormData>(inviteApplicantToAssessment.bind(null, roleId, applicationId), {
    error: null,
    notice: null,
  });
  return (
    <div className="hair-card space-y-3 p-5">
      <div className="eyebrow">Applied from the job board</div>
      {coverNote ? (
        <p className="text-sm whitespace-pre-line">&ldquo;{coverNote}&rdquo;</p>
      ) : (
        <p className="text-sm text-dim">They didn&apos;t add a note.</p>
      )}
      {selfAssessment ? (
        <p className="text-xs text-dim">This role&apos;s assessment is open to every applicant, so it opened for them when they applied.</p>
      ) : (
        canInvite && (
          <form action={action} className="flex flex-wrap items-end gap-3 border-t border-hair pt-3">
            <div className="w-44">
              <Field label="Due by (optional)">
                <Input type="date" name="dueDate" />
              </Field>
            </div>
            <Submit pendingText="Inviting…">Invite to the assessment</Submit>
            <p className="w-full text-xs text-dim">They get their own task, written for them from the role&apos;s brief. To turn them down, use Reject on the role&apos;s page.</p>
          </form>
        )
      )}
      <Result state={state} />
    </div>
  );
}

export function InterviewsPanel({ applicationId, interviews, canEdit }: { applicationId: string; interviews: ApplicationInterview[]; canEdit: boolean }) {
  const [scheduling, setScheduling] = useState(false);
  const [state, action] = useActionState<HiringState, FormData>(scheduleInterviewAction.bind(null, applicationId), { error: null, notice: null });

  return (
    <div className="hair-card p-5">
      <div className="flex items-center justify-between gap-3">
        <div className="eyebrow">Interviews</div>
        {canEdit && !scheduling && (
          <Button type="button" size="sm" variant="soft" onClick={() => setScheduling(true)}>
            Schedule an interview
          </Button>
        )}
      </div>

      {interviews.length === 0 && !scheduling && <p className="mt-2 text-sm text-dim">None scheduled.</p>}

      {interviews.length > 0 && (
        <ul className="mt-3 divide-y divide-hair">
          {interviews.map((iv) => (
            <InterviewRow key={iv.id} applicationId={applicationId} interview={iv} canEdit={canEdit} />
          ))}
        </ul>
      )}

      {scheduling && (
        <form
          action={async (fd) => {
            await action(fd);
          }}
          className="mt-4 space-y-3 border-t border-hair pt-4"
        >
          <InterviewFields />
          <div className="flex gap-2">
            <Submit pendingText="Scheduling…">Schedule and email them</Submit>
            <Button type="button" size="sm" variant="ghost" onClick={() => setScheduling(false)}>
              Cancel
            </Button>
          </div>
        </form>
      )}
      <div className="mt-3">
        <Result state={state} />
      </div>
    </div>
  );
}

function InterviewRow({ applicationId, interview: iv, canEdit }: { applicationId: string; interview: ApplicationInterview; canEdit: boolean }) {
  const [moving, setMoving] = useState(false);
  const [state, action] = useActionState<HiringState, FormData>(rescheduleInterviewAction.bind(null, applicationId, iv.id), { error: null, notice: null });
  const open = !["cancelled", "cancelled_by_candidate", "completed"].includes(iv.status);

  return (
    <li className="py-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-sm font-bold">{iv.title}</span>
        <Pill tone={STATUS[iv.status].tone}>{STATUS[iv.status].label}</Pill>
      </div>
      <p className="mt-1 text-sm text-dim">
        {when(iv.startsAt)} · {iv.durationMin} min{iv.location ? ` · ${iv.location}` : ""}
      </p>
      {iv.candidateMessage && <p className="mt-1.5 text-sm whitespace-pre-line">Candidate: &ldquo;{iv.candidateMessage}&rdquo;</p>}
      {canEdit && open && (
        <form className="mt-2 flex flex-wrap gap-1.5">
          <Button type="button" size="sm" variant="soft" className="!px-3 !py-1.5 !text-[12px]" onClick={() => setMoving(true)}>
            Move
          </Button>
          <Button type="submit" size="sm" variant="ghost" className="!px-3 !py-1.5 !text-[12px]" formAction={closeInterviewAction.bind(null, applicationId, iv.id, "completed")}>
            Mark done
          </Button>
          <Button type="submit" size="sm" variant="danger" className="!px-3 !py-1.5 !text-[12px]" formAction={closeInterviewAction.bind(null, applicationId, iv.id, "cancelled")}>
            Cancel interview
          </Button>
        </form>
      )}
      <Modal open={moving} onClose={() => setMoving(false)} title="Move the interview">
        <form
          action={async (fd) => {
            await action(fd);
          }}
          className="space-y-3"
        >
          <InterviewFields interview={iv} />
          <Submit pendingText="Saving…">Move and email them</Submit>
          <Result state={state} />
        </form>
      </Modal>
    </li>
  );
}

export function EmailPanel({ applicationId, emails, canSend, candidateEmail }: { applicationId: string; emails: ApplicationEmail[]; canSend: boolean; candidateEmail: string }) {
  const [state, action] = useActionState<HiringState, FormData>(sendEmailAction.bind(null, applicationId), { error: null, notice: null });
  const [key, setKey] = useState(0);

  return (
    <div className="hair-card p-5">
      <div className="eyebrow">Email</div>
      <p className="mt-1 text-xs text-dim">
        Sent to {candidateEmail} from Mindfries, with your address as reply-to — their answer comes to your inbox. They also see it in their
        Applications.
      </p>

      {canSend && (
        <form
          key={key}
          action={async (fd) => {
            await action(fd);
            setKey((k) => k + 1);
          }}
          className="mt-3 space-y-3"
        >
          <Field label="Subject">
            <Input name="subject" required maxLength={200} />
          </Field>
          <Field label="Message">
            <Textarea name="body" required rows={5} maxLength={10000} />
          </Field>
          <Submit pendingText="Sending…">Send email</Submit>
        </form>
      )}
      <div className="mt-2">
        <Result state={state} />
      </div>

      {emails.length > 0 && (
        <ul className="mt-4 divide-y divide-hair border-t border-hair">
          {emails.map((m) => (
            <li key={m.id} className="py-3">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-bold">{m.subject}</span>
                {!m.delivered && <Pill tone="coral">Not delivered</Pill>}
              </div>
              <p className="mt-0.5 text-xs text-faint">
                {when(m.sentAt)}
                {m.sentByName ? ` · ${m.sentByName}` : ""}
                {m.error ? ` · ${m.error}` : ""}
              </p>
              <p className="mt-1.5 line-clamp-4 text-sm whitespace-pre-line text-dim">{m.body}</p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
