"use client";

import { useState, useTransition } from "react";
import clsx from "clsx";
import type { SettingResult } from "@/lib/settings/actions";

/** An on/off switch. `locked` is for a setting that can't change — shown on, and said so. */
export function Switch({
  checked,
  onChange,
  label,
  disabled,
  locked,
}: {
  checked: boolean;
  onChange?: (next: boolean) => void;
  label: string;
  disabled?: boolean;
  locked?: boolean;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled || locked}
      onClick={() => onChange?.(!checked)}
      className={clsx(
        "relative inline-flex h-[26px] w-[46px] shrink-0 items-center rounded-full transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#5B3BE0]",
        checked ? "bg-[#5B3BE0]" : "bg-[#D9D6E6]",
        locked ? "cursor-not-allowed opacity-60" : "disabled:opacity-60"
      )}
    >
      <span className={clsx("inline-block h-5 w-5 rounded-full bg-white shadow-[0_1px_3px_rgba(27,24,64,0.35)] transition-transform", checked ? "translate-x-[23px]" : "translate-x-[3px]")} />
    </button>
  );
}

/**
 * A switch that saves itself: it flips at once, asks the server, and flips
 * back with the server's reason if the save didn't happen — so what it shows
 * is never a setting that wasn't stored.
 */
export function SavedSwitch({
  initial,
  save,
  label,
  disabled,
}: {
  initial: boolean;
  save: (next: boolean) => Promise<SettingResult>;
  label: string;
  disabled?: boolean;
}) {
  const [on, setOn] = useState(initial);
  const [error, setError] = useState<string | null>(null);
  const [pending, start] = useTransition();

  function flip(next: boolean) {
    setOn(next);
    setError(null);
    start(async () => {
      const result = await save(next).catch(() => ({ ok: false as const, error: "Couldn't reach the server — try again." }));
      if (!result.ok) {
        setOn(!next);
        setError(result.error);
      }
    });
  }

  return (
    <span className="flex flex-col items-end gap-1">
      <Switch checked={on} onChange={flip} label={label} disabled={disabled || pending} />
      {error && (
        <span role="alert" className="max-w-[220px] text-right text-[12px] leading-snug text-[#a6203c]">
          {error}
        </span>
      )}
    </span>
  );
}
