import Link from "next/link";
import clsx from "clsx";
import { CircleCheck, CircleDashed, LayoutGrid, List, Mail, Send, type LucideIcon } from "lucide-react";
import { ASSESSMENT_STATUSES, statusLabels, type AssessmentStatus } from "@/lib/dashboard/data";

export type AssessmentView = "list" | "grid";

const ICONS: Record<AssessmentStatus, LucideIcon> = {
  invited: Mail,
  "in-progress": CircleDashed,
  submitted: Send,
  closed: CircleCheck,
};

/** /assessments with the given filter, search and view — omitting whatever is at its default. */
export function assessmentsHref({ status, q, view }: { status?: AssessmentStatus; q?: string; view?: AssessmentView }) {
  const params = new URLSearchParams();
  if (status) params.set("status", status);
  if (q) params.set("q", q);
  if (view && view !== "list") params.set("view", view);
  const query = params.toString();
  return query ? `/assessments?${query}` : "/assessments";
}

/**
 * The status filter and the list/grid switch on /assessments. Plain links
 * with a query string, not client state — a filtered view is a real URL you
 * can bookmark or share, and the page does the filtering server-side. Each
 * link keeps the search and the view that are already applied.
 */
export function StatusTabs({
  active,
  counts,
  total,
  q,
  view,
}: {
  active?: AssessmentStatus;
  counts: Record<AssessmentStatus, number>;
  total: number;
  q?: string;
  view: AssessmentView;
}) {
  return (
    <div className="flex items-center gap-3 rounded-[20px] border border-white/90 bg-white/55 p-2 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.4)] backdrop-blur">
      <nav className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto" aria-label="Filter by status">
        <Tab href={assessmentsHref({ q, view })} label="All" icon={LayoutGrid} count={total} active={!active} />
        {ASSESSMENT_STATUSES.map((status) => (
          <Tab
            key={status}
            href={assessmentsHref({ status, q, view })}
            label={statusLabels[status]}
            icon={ICONS[status]}
            count={counts[status]}
            active={active === status}
          />
        ))}
      </nav>

      <div className="flex shrink-0 items-center gap-1" role="group" aria-label="Layout">
        <ViewLink href={assessmentsHref({ status: active, q, view: "list" })} active={view === "list"} label="List view" icon={List} />
        <ViewLink href={assessmentsHref({ status: active, q, view: "grid" })} active={view === "grid"} label="Grid view" icon={LayoutGrid} />
      </div>
    </div>
  );
}

function Tab({ href, label, icon: Icon, count, active }: { href: string; label: string; icon: LucideIcon; count: number; active: boolean }) {
  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className={clsx(
        "inline-flex shrink-0 items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-[13px] font-medium transition-colors",
        active ? "bg-[#EDE7FD] text-[#1B1840]" : "text-[#1B1840] hover:bg-white/70"
      )}
    >
      <Icon size={17} strokeWidth={1.8} className={active ? "text-[#5B3BE0]" : "text-[#2A2654]"} />
      {label}
      <span
        className={clsx(
          "min-w-[22px] rounded-full px-1.5 py-px text-center text-[11.5px] tabular-nums",
          active ? "bg-white text-[#5B3BE0]" : "bg-[#F1EFF8] text-[#6E6A8F]"
        )}
      >
        {count}
      </span>
    </Link>
  );
}

function ViewLink({ href, active, label, icon: Icon }: { href: string; active: boolean; label: string; icon: LucideIcon }) {
  return (
    <Link
      href={href}
      aria-label={label}
      aria-current={active ? "page" : undefined}
      className={clsx(
        "flex h-[42px] w-[42px] items-center justify-center rounded-xl transition-colors",
        active ? "bg-white text-[#5B3BE0] shadow-[0_4px_14px_-6px_rgba(27,24,64,0.25)]" : "text-[#2A2654] hover:bg-white/70"
      )}
    >
      <Icon size={18} strokeWidth={1.8} />
    </Link>
  );
}
