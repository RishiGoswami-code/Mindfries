// The ticket shape the console renders. Mirrors the Go API's support.Ticket
// (internal/support) closely enough for the client — the API is the source of
// truth; this is the view model.

export type Status = "open" | "pending" | "resolved" | "closed";
export type Priority = "urgent" | "high" | "normal" | "low";

export interface Ticket {
  id: string;
  subject: string;
  requester: string;
  status: Status;
  priority: Priority;
  /** When the SLA is due, as an ISO-8601 timestamp. */
  slaDueAt: string;
  updatedAt: string;
}
