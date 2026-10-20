// Queue ordering for the agent console: most-urgent first, and within the same
// priority the ticket whose SLA is due soonest comes first. Pure (returns a new
// array) so it's trivially testable and safe to call during render.

import type { Priority, Ticket } from "./types";

/** Lower rank = handled first. */
const RANK: Record<Priority, number> = {
  urgent: 0,
  high: 1,
  normal: 2,
  low: 3,
};

export function sortTickets(tickets: Ticket[]): Ticket[] {
  return [...tickets].sort((a, b) => {
    if (RANK[a.priority] !== RANK[b.priority]) {
      return RANK[b.priority] - RANK[a.priority];
    }
    return new Date(a.slaDueAt).getTime() - new Date(b.slaDueAt).getTime();
  });
}
