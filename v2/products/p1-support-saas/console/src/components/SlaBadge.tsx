// A small status pill showing how a ticket stands against its SLA. The colour
// and label come straight from the SLA state; the minutes are shown so an agent
// can triage at a glance.

import { slaFor, type SlaState } from "../sla";

const LABEL: Record<SlaState, string> = {
  ok: "On track",
  "at-risk": "Due soon",
  breached: "Breached",
};

export function SlaBadge({ dueAt, now = new Date() }: { dueAt: string; now?: Date }) {
  const { state, minutesLeft } = slaFor(dueAt, now);
  const mins = Math.abs(minutesLeft);
  const when = state === "breached" ? `${mins}m overdue` : `${mins}m left`;
  return (
    <span className={`sla-badge sla-${state}`} data-state={state} role="status">
      {LABEL[state]} · {when}
    </span>
  );
}
