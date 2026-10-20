import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TicketList } from "./TicketList";
import type { Ticket } from "../types";

const now = new Date("2026-01-01T12:00:00Z");

function ticket(id: string, priority: Ticket["priority"]): Ticket {
  return { id, subject: `Subject ${id}`, requester: "a@b.com", status: "open", priority, slaDueAt: "2026-01-01T13:00:00Z", updatedAt: "2026-01-01T11:00:00Z" };
}

describe("TicketList", () => {
  it("renders tickets in triage order", () => {
    render(<TicketList tickets={[ticket("low", "low"), ticket("urgent", "urgent")]} now={now} />);
    const rows = screen.getAllByRole("listitem");
    expect(rows.map((r) => r.getAttribute("data-ticket-id"))).toEqual(["urgent", "low"]);
  });

  it("shows an empty state when there are no tickets", () => {
    render(<TicketList tickets={[]} now={now} />);
    expect(screen.getByText(/no open tickets/i)).toBeInTheDocument();
  });
});
