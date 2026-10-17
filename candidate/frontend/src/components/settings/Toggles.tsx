"use client";

import { setEmailPreference, setFieldHidden } from "@/lib/settings/actions";
import type { EmailKind, HideableField } from "@/lib/settings/shape";
import { SavedSwitch } from "./Switch";

/** One email kind, on or off. */
export function EmailSwitch({ kind, initial, label, disabled }: { kind: EmailKind; initial: boolean; label: string; disabled?: boolean }) {
  return <SavedSwitch initial={initial} save={(on) => setEmailPreference(kind, on)} label={label} disabled={disabled} />;
}

/** One part of the profile, shown to hiring teams or not. On = shown. */
export function VisibilitySwitch({ field, shown, label, disabled }: { field: HideableField; shown: boolean; label: string; disabled?: boolean }) {
  return <SavedSwitch initial={shown} save={(show) => setFieldHidden(field, !show)} label={label} disabled={disabled} />;
}
