"use client";

import { useState } from "react";
import clsx from "clsx";
import { Bug } from "lucide-react";
import type { PanelConfig } from "@/lib/v2/panel/types";
import { TabHeader } from "./NotesTab";

/**
 * Debugging: the logs to investigate, and the candidate's findings (the
 * hypothesis they test, what they ruled out). SPEC §10 D.
 */
export function DebugTab({ debug }: { debug: PanelConfig["debug"] }) {
  const [findings, setFindings] = useState("");
  return (
    <div className="flex h-full flex-col">
      <TabHeader icon={<Bug className="h-4 w-4" />} title={debug.title} hint="What do the logs tell you? Write down your hypothesis." />
      <pre className="m-4 mb-0 max-h-[45%] overflow-auto rounded-xl bg-[#111827] p-4 font-mono text-[12.5px] leading-6 text-[#D1D5DB]">
        {debug.log.map((l, i) => (
          <div key={i} className={clsx(l.includes(" WARN ") && "text-[#FBBF24]", l.includes(" ERROR ") && "text-[#F87171]")}>
            {l}
          </div>
        ))}
      </pre>
      <label className="flex flex-1 flex-col p-4">
        <span className="mb-1.5 text-sm font-medium text-[#10183A]">Findings</span>
        <textarea
          value={findings}
          onChange={(e) => setFindings(e.target.value)}
          placeholder="Hypothesis, what you checked, what you ruled out, the root cause."
          className="flex-1 resize-none rounded-xl border border-[#E3E8F2] p-3 text-sm outline-none focus:border-[#2F5BD3]"
        />
      </label>
    </div>
  );
}
