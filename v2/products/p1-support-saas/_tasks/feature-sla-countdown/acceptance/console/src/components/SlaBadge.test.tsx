import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SlaBadge } from "./SlaBadge";

const now = new Date("2026-01-01T12:00:00Z");

describe("SlaBadge (acceptance)", () => {
  it("renders the breached state and overdue time", () => {
    render(<SlaBadge dueAt="2026-01-01T11:30:00Z" now={now} />);
    const el = screen.getByRole("status");
    expect(el).toHaveAttribute("data-state", "breached");
    expect(el).toHaveTextContent(/breached/i);
    expect(el).toHaveTextContent(/30m overdue/i);
  });

  it("renders at-risk when the SLA is due soon", () => {
    render(<SlaBadge dueAt="2026-01-01T12:45:00Z" now={now} />);
    const el = screen.getByRole("status");
    expect(el).toHaveAttribute("data-state", "at-risk");
    expect(el).toHaveTextContent(/45m left/i);
  });

  it("renders on-track when well ahead", () => {
    render(<SlaBadge dueAt="2026-01-01T18:00:00Z" now={now} />);
    const el = screen.getByRole("status");
    expect(el).toHaveAttribute("data-state", "ok");
    expect(el).toHaveTextContent(/on track/i);
  });
});
