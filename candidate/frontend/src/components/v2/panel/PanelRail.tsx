"use client";

import clsx from "clsx";
import { Activity, BookOpen, Bug, FileText, FlaskConical, GitPullRequest, Workflow, type LucideIcon } from "lucide-react";
import type { PanelTab, PanelTabId } from "@/lib/v2/panel/types";

export const TAB_ICONS: Record<PanelTabId, LucideIcon> = {
  design: Workflow,
  notes: FileText,
  resources: BookOpen,
  review: GitPullRequest,
  debug: Bug,
  test: FlaskConical,
  activity: Activity,
};

/** The workspace tabs down the left, as in the mock-up. */
export function PanelRail({ tabs, active, onSelect }: { tabs: PanelTab[]; active: PanelTabId; onSelect: (id: PanelTabId) => void }) {
  return (
    <nav className="flex w-[180px] shrink-0 flex-col gap-1.5 pt-1" aria-label="Workspace">
      {tabs.map((t) => {
        const Icon = TAB_ICONS[t.id];
        const on = t.id === active;
        return (
          <button
            key={t.id}
            type="button"
            onClick={() => onSelect(t.id)}
            aria-current={on ? "page" : undefined}
            className={clsx(
              "flex items-center gap-3 rounded-xl px-4 py-3 text-left text-[15px] transition-colors",
              on ? "bg-[#EAF0FE] font-medium text-[#2F5BD3]" : "text-[#3B4568] hover:bg-[#F4F6FB]",
            )}
          >
            <Icon className="h-5 w-5 shrink-0" aria-hidden />
            <span className="flex-1">{t.label}</span>
            {t.badge ? (
              <span className="rounded-full bg-[#E5484D] px-1.5 text-xs font-semibold text-white" aria-label={`${t.badge} new`}>
                {t.badge}
              </span>
            ) : null}
          </button>
        );
      })}
    </nav>
  );
}
