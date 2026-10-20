import { describe, expect, it } from "vitest";
import { slaFor } from "./sla";

const now = new Date("2026-01-01T12:00:00Z");

describe("slaFor (acceptance)", () => {
  it("is ok when well ahead of the SLA", () => {
    expect(slaFor("2026-01-01T15:00:00Z", now)).toEqual({ state: "ok", minutesLeft: 180 });
  });

  it("is at-risk within the hour, with minutes left", () => {
    expect(slaFor("2026-01-01T12:30:00Z", now)).toEqual({ state: "at-risk", minutesLeft: 30 });
  });

  it("is at-risk exactly at the threshold", () => {
    expect(slaFor("2026-01-01T13:00:00Z", now).state).toBe("at-risk");
  });

  it("is breached once past due, with negative minutes", () => {
    expect(slaFor("2026-01-01T11:30:00Z", now)).toEqual({ state: "breached", minutesLeft: -30 });
  });
});
