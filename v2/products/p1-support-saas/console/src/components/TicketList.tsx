// The agent queue: tickets in triage order, each with its SLA badge. Ordering
// is delegated to sortTickets so the policy lives in one pure place.

import { sortTickets } from "../priority";
import type { Ticket } from "../types";
import { SlaBadge } from "./SlaBadge";

export function TicketList({ tickets, now }: { tickets: Ticket[]; now?: Date }) {
  const ordered = sortTickets(tickets);
  if (ordered.length === 0) {
    return <p className="empty">No open tickets 🎉</p>;
  }
  return (
    <ul className="ticket-list">
      {ordered.map((t) => (
        <li key={t.id} className="ticket-row" data-ticket-id={t.id} data-priority={t.priority}>
          <span className="ticket-subject">{t.subject}</span>
          <span className="ticket-requester">{t.requester}</span>
          <SlaBadge dueAt={t.slaDueAt} now={now} />
        </li>
      ))}
    </ul>
  );
}
