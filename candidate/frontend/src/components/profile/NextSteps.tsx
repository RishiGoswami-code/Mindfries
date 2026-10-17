import { nextSteps } from "@/lib/profile/next-steps";
import type { StoredLinks } from "@/lib/profile/types";

/**
 * What's left to do, as a numbered checklist that IS the next action —
 * rendered inside the profile page's Next steps tile, which supplies the
 * heading and hides itself (in preview mode, or once nothing is left).
 */
export function NextSteps({ hasResume, links }: { hasResume: boolean; links: StoredLinks }) {
  const remaining = nextSteps(hasResume, links);

  return (
    <ol className="space-y-3">
      {remaining.map((step, i) => (
        <li key={step.title} className="flex items-start gap-3">
          <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[#1F1B4D] text-[11.5px] font-semibold text-white">
            {i + 1}
          </span>
          <div className="min-w-0">
            <p className="text-[14px] font-medium text-[#100E2E]">{step.title}</p>
            <p className="text-[12.5px] leading-snug text-[#6E6A8F]">{step.body}</p>
          </div>
        </li>
      ))}
    </ol>
  );
}
