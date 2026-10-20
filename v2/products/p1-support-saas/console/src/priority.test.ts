import { describe, expect, it } from "vitest";
import { sortTickets } from "./priority";
import type { Ticket } from "./types";

function ticket(id: string, priority: Ticket["priority"], slaDueAt: string): Ticket {
  return { id, subject: id, requester: "a@b.com", status: "open", priority, slaDueAt, updatedAt: slaDueAt };
}

describe("sortTickets", () => {
  it("orders by priority, most urgent first", () => {
    const out = sortTickets([
      ticket("low", "low", "2026-01-01T12:00:00Z"),
      ticket("urgent", "urgent", "2026-01-01T12:00:00Z"),
      ticket("normal", "normal", "2026-01-01T12:00:00Z"),
      ticket("high", "high", "2026-01-01T12:00:00Z"),
    ]);
    expect(out.map((t) => t.id)).toEqual(["urgent", "high", "normal", "low"]);
  });

  it("breaks ties by soonest SLA", () => {
    const out = sortTickets([
      ticket("later", "high", "2026-01-01T15:00:00Z"),
      ticket("sooner", "high", "2026-01-01T13:00:00Z"),
    ]);
    expect(out.map((t) => t.id)).toEqual(["sooner", "later"]);
  });

  it("does not mutate its input", () => {
    const input = [ticket("a", "low", "2026-01-01T12:00:00Z"), ticket("b", "urgent", "2026-01-01T12:00:00Z")];
    const before = input.map((t) => t.id);
    sortTickets(input);
    expect(input.map((t) => t.id)).toEqual(before);
  });
});
