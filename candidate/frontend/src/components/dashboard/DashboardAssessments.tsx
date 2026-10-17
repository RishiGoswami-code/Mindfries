import Link from "next/link";
import clsx from "clsx";
import { FileText } from "lucide-react";
import { AssessmentRow } from "@/components/assessments/AssessmentItem";
import { sortByAttention, UNAVAILABLE, type Assessment } from "@/lib/dashboard/data";
import { display } from "@/lib/dashboard/fonts";

const PREVIEW_COUNT = 3;

/**
 * The dashboard's preview of the candidate's assessments: the three most
 * worth acting on (sortByAttention), with "View all" when there are more.
 *
 * `items` undefined means the list couldn't be read, which is said plainly
 * (UNAVAILABLE) rather than shown as an empty list. Rows are the same
 * AssessmentRow the Assessments page lists, with the same actions.
 */
export function DashboardAssessments({ items: loaded }: { items?: Assessment[] }) {
  const items = loaded ?? [];
  const preview = sortByAttention(items).slice(0, PREVIEW_COUNT);

  return (
    <section className="rounded-[22px] border border-white/90 bg-white/55 p-6 shadow-[0_10px_30px_-20px_rgba(76,58,160,0.3)] backdrop-blur">
      <div className="flex items-center gap-5">
        <h2 className={clsx(display.className, "text-[25px] font-medium tracking-tight text-[#100E2E]")}>Your assessments</h2>
        <span className="flex h-[30px] min-w-[40px] items-center justify-center rounded-full bg-[#EEEBFA] px-3 text-[14px] font-medium tabular-nums text-[#1B1840]">
          {items.length}
        </span>
        {items.length > PREVIEW_COUNT && (
          <Link href="/assessments" className="ml-auto text-[13.5px] font-medium text-[#5B3BE0] hover:underline">
            View all →
          </Link>
        )}
      </div>

      {preview.length === 0 ? (
        <div className="mt-5 flex flex-col items-center rounded-2xl border border-dashed border-[#DCD7EE] px-6 pt-6 pb-8 text-center">
          <span className="flex h-[78px] w-[78px] items-center justify-center rounded-full bg-[#EFEBFB]">
            <FileText size={32} strokeWidth={1.6} className="text-[#6A47E6]" />
          </span>
          <p className={clsx(display.className, "mt-4 text-[23px] font-medium text-[#100E2E]")}>
            {loaded ? "Nothing here yet" : UNAVAILABLE.title}
          </p>
          <p className="mt-2 max-w-md text-[15px] text-[#7A6CB5]">
            {loaded ? "Assessments you're invited to will show up here." : UNAVAILABLE.body}
          </p>
        </div>
      ) : (
        <ul className="mt-5 space-y-3">
          {preview.map((assessment) => (
            <AssessmentRow key={assessment.id} assessment={assessment} />
          ))}
        </ul>
      )}
    </section>
  );
}
