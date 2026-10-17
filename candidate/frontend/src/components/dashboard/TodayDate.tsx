"use client";

import { useSyncExternalStore } from "react";

const noSubscription = () => () => {};

/** "Fri, 17 Oct 2026" — assembled from parts, since locales disagree about the comma. */
function today(): string {
  const parts = Object.fromEntries(
    new Intl.DateTimeFormat("en-GB", { weekday: "short", day: "numeric", month: "short", year: "numeric" })
      .formatToParts(new Date())
      .map((p) => [p.type, p.value])
  );
  return `${parts.weekday}, ${parts.day} ${parts.month} ${parts.year}`;
}

/**
 * Today's date in the candidate's own timezone.
 *
 * Rendered by the browser, not the server: the server's "today" is UTC, so
 * for a candidate in India it would show yesterday's date for the first
 * five and a half hours of every day. Until hydration it renders nothing
 * rather than a date that might be wrong.
 */
export function TodayDate({ className }: { className?: string }) {
  const label = useSyncExternalStore(noSubscription, today, () => "");
  return <p className={className}>{label}</p>;
}
