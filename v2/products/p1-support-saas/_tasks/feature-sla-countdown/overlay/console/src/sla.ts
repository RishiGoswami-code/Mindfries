// SLA state for a ticket. Pure and time-injected (never reads the clock
// itself) so the console renders deterministically and tests don't flake.

export type SlaState = "ok" | "at-risk" | "breached";

export interface Sla {
  state: SlaState;
  /** Whole minutes until the SLA is due; negative once it's past due. */
  minutesLeft: number;
}

/** Minutes-left threshold under which a not-yet-breached SLA is "at-risk". */
export const AT_RISK_MINUTES = 60;

export function slaFor(_dueAtISO: string, _now: Date): Sla {
  // TODO(candidate): compute minutesLeft and the state. See TASK.md.
  return { state: "ok", minutesLeft: 0 };
}
