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

export function slaFor(dueAtISO: string, now: Date): Sla {
  const minutesLeft = Math.floor((new Date(dueAtISO).getTime() - now.getTime()) / 60_000);
  let state: SlaState = "ok";
  if (minutesLeft < 0) state = "breached";
  else if (minutesLeft <= AT_RISK_MINUTES) state = "at-risk";
  return { state, minutesLeft };
}
