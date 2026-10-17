/**
 * The work a candidate is looking for (candidate_users.job_preferences,
 * migration 0024). Pure: the profile form and the server action both use
 * `cleanJobPreferences`, so what the form allows and what is stored can't
 * drift apart.
 *
 * Work mode isn't here — it's candidate_users.open_to, the field Edit
 * profile already sets (OPEN_TO_OPTIONS in ./data.ts). The preferences form
 * edits that same field rather than keeping a second copy.
 */

export const EMPLOYMENT_TYPES = ["Full-time", "Part-time", "Contract", "Internship"] as const;
export type EmploymentType = (typeof EMPLOYMENT_TYPES)[number];

export const CURRENCIES = ["USD", "EUR", "GBP", "INR"] as const;
export type Currency = (typeof CURRENCIES)[number];

/** Suggestions for the roles field — a candidate can type any role; these are only offered. */
export const ROLE_SUGGESTIONS = [
  "Software Engineer",
  "Frontend Engineer",
  "Backend Engineer",
  "Full-stack Engineer",
  "Mobile Engineer",
  "DevOps Engineer",
  "Site Reliability Engineer",
  "Data Engineer",
  "Data Scientist",
  "AI/ML Engineer",
  "LLM Engineer",
  "Applied Scientist",
  "Security Engineer",
  "QA Engineer",
];

/** Suggestions for the locations field, likewise only offered. */
export const LOCATION_SUGGESTIONS = ["Worldwide", "India", "United States", "Europe", "United Kingdom", "Canada", "Singapore", "Middle East"];

export const MAX_ITEMS = 8;
const MAX_LABEL = 60;
const MAX_SALARY = 100_000_000;

export interface JobPreferences {
  roles: string[];
  employment: EmploymentType[];
  locations: string[];
  /** A yearly range. Null when the candidate would rather not say. */
  salary: { min: number; max: number; currency: Currency } | null;
  updatedAt: string;
}

export type JobPreferencesInput = Omit<JobPreferences, "updatedAt">;

export const EMPTY_PREFERENCES: JobPreferencesInput = { roles: [], employment: [], locations: [], salary: null };

function labels(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  const seen = new Set<string>();
  const out: string[] = [];
  for (const item of value) {
    if (typeof item !== "string") continue;
    const label = item.replace(/\s+/g, " ").trim().slice(0, MAX_LABEL);
    const key = label.toLowerCase();
    if (!label || seen.has(key)) continue;
    seen.add(key);
    out.push(label);
    if (out.length === MAX_ITEMS) break;
  }
  return out;
}

/**
 * What can be stored, or the reason it can't. Unknown employment types and
 * currencies are dropped; a salary range has to be two whole numbers, low
 * to high.
 */
export function cleanJobPreferences(input: unknown): { ok: true; value: JobPreferencesInput } | { ok: false; error: string } {
  const raw = (input ?? {}) as Record<string, unknown>;
  const employment = (Array.isArray(raw.employment) ? raw.employment : []).filter((e): e is EmploymentType =>
    (EMPLOYMENT_TYPES as readonly string[]).includes(e as string)
  );

  let salary: JobPreferencesInput["salary"] = null;
  const s = raw.salary as Record<string, unknown> | null | undefined;
  if (s && (s.min !== undefined || s.max !== undefined)) {
    const min = Number(s.min);
    const max = Number(s.max);
    if (!Number.isInteger(min) || !Number.isInteger(max) || min < 0 || max < 0) return { ok: false, error: "Salary needs to be two whole numbers." };
    if (min > max) return { ok: false, error: "The lowest salary is above the highest." };
    if (max > MAX_SALARY) return { ok: false, error: "That salary is higher than this form takes." };
    const currency = (CURRENCIES as readonly string[]).includes(s.currency as string) ? (s.currency as Currency) : "USD";
    salary = { min, max, currency };
  }

  return { ok: true, value: { roles: labels(raw.roles), employment: [...new Set(employment)], locations: labels(raw.locations), salary } };
}

/** Reads what's stored, tolerating rows written before a field existed. */
export function readJobPreferences(stored: unknown): JobPreferences | null {
  if (!stored || typeof stored !== "object") return null;
  const cleaned = cleanJobPreferences(stored);
  if (!cleaned.ok) return null;
  const updatedAt = (stored as { updatedAt?: unknown }).updatedAt;
  return { ...cleaned.value, updatedAt: typeof updatedAt === "string" ? updatedAt : "" };
}

/** True once there is anything a hiring team would see. */
export function hasPreferences(p: JobPreferencesInput | null, workModes: string[]): boolean {
  return !!p && (p.roles.length > 0 || p.employment.length > 0 || p.locations.length > 0 || !!p.salary || workModes.length > 0);
}

const SYMBOL: Record<Currency, string> = { USD: "$", EUR: "€", GBP: "£", INR: "₹" };

/** "$80K – $120K", "₹12L – ₹18L" for INR. */
export function formatSalaryRange(salary: NonNullable<JobPreferences["salary"]>): string {
  const sym = SYMBOL[salary.currency];
  const short = (n: number) => {
    if (salary.currency === "INR" && n >= 100_000) return `${sym}${trim(n / 100_000)}L`;
    if (n >= 1_000_000) return `${sym}${trim(n / 1_000_000)}M`;
    if (n >= 1_000) return `${sym}${trim(n / 1_000)}K`;
    return `${sym}${n}`;
  };
  return `${short(salary.min)} – ${short(salary.max)}`;
}

function trim(n: number): string {
  return Number.isInteger(n) ? String(n) : n.toFixed(1).replace(/\.0$/, "");
}
