"use client";

import { useId, useState } from "react";
import clsx from "clsx";
import { Info, Loader2, RotateCcw, X } from "lucide-react";
import { OPEN_TO_OPTIONS } from "@/lib/profile/data";
import {
  CURRENCIES,
  EMPLOYMENT_TYPES,
  EMPTY_PREFERENCES,
  LOCATION_SUGGESTIONS,
  MAX_ITEMS,
  ROLE_SUGGESTIONS,
  cleanJobPreferences,
  formatSalaryRange,
  type Currency,
  type EmploymentType,
  type JobPreferences,
} from "@/lib/profile/job-preferences";
import { saveJobPreferences } from "@/lib/profile/extras-actions";
import { useProfileSections } from "./ProfileCard";
import { usePreviewMode } from "./PreviewMode";

/**
 * The Job preferences window: the roles the candidate wants, how and where
 * they'd work, the employment types they'd take, and the salary range they
 * expect. Saved to their profile and shown to hiring teams beside it.
 *
 * Work mode is the profile's own "open to" field — the same one Edit
 * profile sets — so changing it here changes it there.
 *
 * There are no job alerts: assessments come by invitation, and there's no
 * job feed to alert about.
 */
export function JobPreferencesSection({
  value,
  workModes,
  available,
}: {
  value: JobPreferences | null;
  workModes: string[];
  /** False when the database doesn't have job preferences yet (migration 0024). */
  available: boolean;
}) {
  const preview = usePreviewMode();
  const { setOpen } = useProfileSections();
  const start = value ?? { ...EMPTY_PREFERENCES, updatedAt: "" };

  const [roles, setRoles] = useState(start.roles);
  const [modes, setModes] = useState(workModes);
  const [employment, setEmployment] = useState<EmploymentType[]>(start.employment);
  const [locations, setLocations] = useState(start.locations);
  const [min, setMin] = useState(start.salary ? String(start.salary.min) : "");
  const [max, setMax] = useState(start.salary ? String(start.salary.max) : "");
  const [currency, setCurrency] = useState<Currency>(start.salary?.currency ?? "USD");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!available) {
    return <p className="rounded-[18px] bg-[#F4F1FD] p-5 text-[14px] text-[#4C4874]">Job preferences aren&apos;t set up on the server yet, so they can&apos;t be saved.</p>;
  }

  if (preview) {
    const salary = value?.salary ? formatSalaryRange(value.salary) : null;
    const rows: Array<[string, string]> = [
      ["Roles", value?.roles.join(", ") ?? ""],
      ["Work mode", workModes.join(", ")],
      ["Employment", value?.employment.join(", ") ?? ""],
      ["Locations", value?.locations.join(", ") ?? ""],
      ["Salary", salary ? `${salary} a year (${value!.salary!.currency})` : ""],
    ];
    return (
      <dl className="divide-y divide-[#F0EEF7] rounded-[18px] border border-[#ECE9F6] bg-white px-5">
        {rows.map(([k, v]) => (
          <div key={k} className="flex gap-4 py-3.5 text-[14.5px]">
            <dt className="w-28 shrink-0 text-[#6E6A8F]">{k}</dt>
            <dd className="text-[#100E2E]">{v || "—"}</dd>
          </div>
        ))}
      </dl>
    );
  }

  const toggle = <T,>(list: T[], item: T) => (list.includes(item) ? list.filter((x) => x !== item) : [...list, item]);
  const salaryEntered = min.trim() !== "" || max.trim() !== "";

  async function save() {
    setError(null);
    const input = {
      roles,
      employment,
      locations,
      salary: salaryEntered ? { min: Number(min.replace(/[,\s]/g, "")), max: Number(max.replace(/[,\s]/g, "")), currency } : null,
    };
    const checked = cleanJobPreferences(input);
    if (!checked.ok) return setError(checked.error);
    setSaving(true);
    try {
      const res = await saveJobPreferences(checked.value, modes);
      if (res.ok) setOpen(null);
      else setError(res.error);
    } catch {
      setError("Couldn't reach the server — check your connection and try again.");
    } finally {
      setSaving(false);
    }
  }

  function reset() {
    setRoles([]);
    setModes([]);
    setEmployment([]);
    setLocations([]);
    setMin("");
    setMax("");
    setCurrency("USD");
    setError(null);
  }

  return (
    <div>
      <div className="space-y-6 rounded-[18px] border border-[#ECE9F6] bg-white p-5 sm:p-6">
        <Field label="Preferred roles" hint={`Up to ${MAX_ITEMS}. Pick one or type your own and press Enter.`}>
          <TagInput values={roles} onChange={setRoles} suggestions={ROLE_SUGGESTIONS} placeholder="Add a role" />
        </Field>

        <div className="grid gap-6 lg:grid-cols-2">
          <Field label="Work mode" hint="The same as “Open to” on your profile.">
            <Segmented options={OPEN_TO_OPTIONS} isOn={(o) => modes.includes(o)} onToggle={(o) => setModes((m) => toggle(m, o))} label={(o) => (o === "Onsite" ? "On-site" : o)} />
          </Field>
          <Field label="Employment type" hint="Any you'd take.">
            <Segmented options={EMPLOYMENT_TYPES} isOn={(o) => employment.includes(o)} onToggle={(o) => setEmployment((e) => toggle(e, o))} />
          </Field>
        </div>

        <Field label="Locations" hint={`Up to ${MAX_ITEMS} — countries, regions, cities, or Worldwide.`}>
          <TagInput values={locations} onChange={setLocations} suggestions={LOCATION_SUGGESTIONS} placeholder="Add a location" />
        </Field>

        <Field label="Salary expectations" hint="A yearly range. Leave both empty if you'd rather not say.">
          <div className="flex flex-wrap items-center gap-3">
            <MoneyInput value={min} onChange={setMin} label="Lowest" />
            <span className="text-[#6E6A8F]">–</span>
            <MoneyInput value={max} onChange={setMax} label="Highest" />
            <select
              value={currency}
              onChange={(e) => setCurrency(e.target.value as Currency)}
              aria-label="Currency"
              className="h-12 rounded-xl border border-[#DCD7EE] bg-white px-3 text-[15px] text-[#100E2E] outline-none focus:border-[#7C5CFC]"
            >
              {CURRENCIES.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          </div>
        </Field>

        <p className="text-[13px] text-[#6E6A8F]">Shown to hiring teams beside your profile.</p>
      </div>

      {error && <p className="mt-3 text-[13.5px] text-[#a6203c]">{error}</p>}

      <div className="mt-5 flex flex-wrap items-center gap-3">
        <button type="button" onClick={reset} className="inline-flex items-center gap-2 text-[14px] text-[#4C4874] hover:text-[#100E2E]">
          <RotateCcw size={16} /> Clear all
        </button>
        <div className="ml-auto flex gap-3">
          <button type="button" onClick={() => setOpen(null)} className="h-11 rounded-xl bg-[#EEEBFA] px-5 text-[14.5px] font-semibold text-[#1F1B4D] hover:bg-[#E4DEF9]">
            Cancel
          </button>
          <button
            type="button"
            onClick={() => void save()}
            disabled={saving}
            className="inline-flex h-11 items-center gap-2 rounded-xl bg-[#5B3BE0] px-6 text-[14.5px] font-semibold text-white hover:bg-[#4B2BB8] disabled:opacity-60"
          >
            {saving && <Loader2 size={16} className="animate-spin" />}
            Save preferences
          </button>
        </div>
      </div>
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-2.5 flex items-center gap-2 text-[16px] font-semibold text-[#100E2E]">
        {label}
        <span title={hint} className="text-[#8A86A8]">
          <Info size={16} aria-label={hint} />
        </span>
      </p>
      {children}
    </div>
  );
}

function Segmented<T extends string>({
  options,
  isOn,
  onToggle,
  label = (o) => o,
}: {
  options: readonly T[];
  isOn: (o: T) => boolean;
  onToggle: (o: T) => void;
  label?: (o: T) => string;
}) {
  return (
    // One joined control, as on a real segmented picker — but each option toggles on its own: several can be on.
    <div className="inline-flex max-w-full overflow-x-auto rounded-xl border border-[#E4E0F2] bg-white p-1">
      {options.map((o) => (
        <button
          key={o}
          type="button"
          aria-pressed={isOn(o)}
          onClick={() => onToggle(o)}
          className={clsx(
            "h-10 shrink-0 rounded-lg px-3.5 text-[14px] whitespace-nowrap transition-colors",
            isOn(o) ? "bg-[#F1EDFC] font-medium text-[#4B2BB8] shadow-[inset_0_0_0_1.5px_#7C5CFC]" : "text-[#4C4874] hover:bg-[#F6F4FC]"
          )}
        >
          {label(o)}
        </button>
      ))}
    </div>
  );
}

function TagInput({ values, onChange, suggestions, placeholder }: { values: string[]; onChange: (v: string[]) => void; suggestions: string[]; placeholder: string }) {
  const [text, setText] = useState("");
  const [focused, setFocused] = useState(false);
  const listId = useId();
  const full = values.length >= MAX_ITEMS;
  const lower = values.map((v) => v.toLowerCase());
  const matches = suggestions.filter((s) => !lower.includes(s.toLowerCase()) && s.toLowerCase().includes(text.trim().toLowerCase())).slice(0, 6);

  function add(raw: string) {
    const v = raw.replace(/\s+/g, " ").trim().slice(0, 60);
    if (!v || full || lower.includes(v.toLowerCase())) return setText("");
    onChange([...values, v]);
    setText("");
  }

  return (
    <div className="relative">
      <div className="flex min-h-12 flex-wrap items-center gap-2 rounded-xl border border-[#DCD7EE] bg-white px-2 py-1.5 focus-within:border-[#7C5CFC]">
        {values.map((v) => (
          <span key={v} className="inline-flex items-center gap-1.5 rounded-lg bg-[#F1EFF8] py-1 pr-1.5 pl-3 text-[14px] text-[#2A2654]">
            {v}
            <button type="button" onClick={() => onChange(values.filter((x) => x !== v))} aria-label={`Remove ${v}`} className="rounded p-0.5 text-[#6E6A8F] hover:bg-white hover:text-[#100E2E]">
              <X size={14} />
            </button>
          </span>
        ))}
        {!full && (
          <input
            value={text}
            onChange={(e) => setText(e.target.value)}
            onFocus={() => setFocused(true)}
            onBlur={() => setTimeout(() => setFocused(false), 120)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === ",") {
                e.preventDefault();
                add(text);
              } else if (e.key === "Backspace" && !text && values.length) onChange(values.slice(0, -1));
            }}
            placeholder={values.length ? "" : placeholder}
            aria-label={placeholder}
            aria-autocomplete="list"
            aria-controls={listId}
            className="h-9 min-w-[140px] flex-1 bg-transparent px-1.5 text-[14.5px] text-[#100E2E] outline-none placeholder:text-[#8A86A8]"
          />
        )}
      </div>
      {focused && !full && matches.length > 0 && (
        <ul id={listId} role="listbox" className="absolute inset-x-0 top-full z-10 mt-1.5 overflow-hidden rounded-xl border border-[#E4E0F2] bg-white py-1 shadow-[0_18px_40px_-20px_rgba(27,24,64,0.45)]">
          {matches.map((m) => (
            <li key={m} role="option" aria-selected={false}>
              <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => add(m)} className="w-full px-3.5 py-2 text-left text-[14px] text-[#2A2654] hover:bg-[#F6F4FC]">
                {m}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function MoneyInput({ value, onChange, label }: { value: string; onChange: (v: string) => void; label: string }) {
  return (
    <input
      inputMode="numeric"
      value={value}
      onChange={(e) => onChange(e.target.value.replace(/[^\d,\s]/g, ""))}
      placeholder={label}
      aria-label={`${label} yearly salary`}
      className="h-12 w-full min-w-0 rounded-xl border border-[#DCD7EE] bg-white px-4 text-[15px] text-[#100E2E] tabular-nums outline-none focus:border-[#7C5CFC] sm:w-[180px]"
    />
  );
}
