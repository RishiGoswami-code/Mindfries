"use client";

import { CircleCheck, CircleX, FlaskConical } from "lucide-react";
import type { PanelConfig } from "@/lib/v2/panel/types";
import { TabHeader } from "./NotesTab";

/**
 * Testing: the latest run of the task's check, failing first (SPEC §10 F).
 * Step 3 fills it live from the CLI watcher's test runs.
 */
export function TestTab({ test }: { test: PanelConfig["test"] }) {
  const total = test.failing.length + test.passing.length;
  return (
    <div className="flex h-full flex-col">
      <TabHeader icon={<FlaskConical className="h-4 w-4" />} title={test.title} hint={`${test.passing.length} of ${total} passing in the last run`} />
      <ul className="flex-1 space-y-1.5 overflow-auto p-4 font-mono text-[13px]">
        {test.failing.map((t) => (
          <li key={t} className="flex items-center gap-2 rounded-lg bg-[#FFF0F0] px-3 py-2 text-[#B42318]">
            <CircleX className="h-4 w-4 shrink-0" aria-label="failing" />
            {t}
          </li>
        ))}
        {test.passing.map((t) => (
          <li key={t} className="flex items-center gap-2 rounded-lg px-3 py-2 text-[#1F7A3F]">
            <CircleCheck className="h-4 w-4 shrink-0" aria-label="passing" />
            {t}
          </li>
        ))}
      </ul>
    </div>
  );
}
