"use client";

import { useSyncExternalStore } from "react";

const noSubscription = () => () => {};

const FORMATS = {
  dateTime: { weekday: "short", day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" },
  date: { day: "numeric", month: "short", year: "numeric" },
} satisfies Record<string, Intl.DateTimeFormatOptions>;

/**
 * A time in the candidate's own timezone and locale. Formatted only in the
 * browser: the server's timezone and locale aren't the candidate's, and
 * formatting there would show a different time — and make React's
 * hydration disagree with what the browser renders. Until then it shows
 * nothing, rather than a time that might be wrong.
 */
export function LocalTime({ iso, format = "dateTime" }: { iso: string; format?: keyof typeof FORMATS }) {
  const text = useSyncExternalStore(
    noSubscription,
    () => new Intl.DateTimeFormat(undefined, FORMATS[format]).format(new Date(iso)),
    () => ""
  );
  return (
    <time dateTime={iso} suppressHydrationWarning>
      {text}
    </time>
  );
}
