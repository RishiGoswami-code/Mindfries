"use client";

import { useState } from "react";
import clsx from "clsx";
import { Bug } from "lucide-react";
import type { Hypothesis, PanelConfig } from "@/lib/v2/panel/types";
import { TabHeader } from "./NotesTab";

/**
 * Debugging: the logs to investigate, and the candidate's findings (the
 * hypothesis they test, what they ruled out). SPEC §10 D.
 */
export function DebugTab({ debug, onHypotheses }: { debug: PanelConfig["debug"]; onHypotheses?: (h: Hypothesis[]) => void }) {
  const [local, setLocal] = useState<Hypothesis[]>(debug.hypotheses);
  const items = onHypotheses ? debug.hypotheses : local;
  const set = (next: Hypothesis[]) => (onHypotheses ? onHypotheses(next) : setLocal(next));
  return (
    <div className="flex h-full flex-col">
      <TabHeader icon={<Bug className="h-4 w-4" />} title={debug.title} hint="What do the logs tell you? Write down your hypotheses." />
      {debug.log.length > 0 ? (
        <pre className="m-4 mb-0 max-h-[45%] overflow-auto rounded-xl bg-[#111827] p-4 font-mono text-[12.5px] leading-6 text-[#D1D5DB]">
          {debug.log.map((l, i) => (
            <div key={i} className={clsx(l.includes(" WARN ") && "text-[#FBBF24]", l.includes(" ERROR ") && "text-[#F87171]")}>
              {l}
            </div>
          ))}
        </pre>
      ) : (
        <p className="m-4 mb-0 rounded-xl bg-[#F7F9FD] p-4 text-sm text-[#6B7393]">
          This task ships no logs. Investigate in your editor and your terminal, and keep your findings here.
        </p>
      )}
      <div className="flex-1 overflow-auto p-4">
        <div className="mb-2 flex items-center justify-between">
          <span className="text-sm font-medium text-[#10183A]">Hypotheses</span>
          <button
            type="button"
            onClick={() => set([...items, { text: "", status: "open" }])}
            className="rounded-lg border border-[#DCE3F1] px-3 py-1 text-sm text-[#2F5BD3] hover:bg-[#F4F7FD]"
          >
            Add a hypothesis
          </button>
        </div>
        <ul className="space-y-2">
          {items.map((h, i) => (
            <li key={i} className="flex gap-2">
              <textarea
                value={h.text}
                onChange={(e) => set(items.map((x, j) => (j === i ? { ...x, text: e.target.value } : x)))}
                rows={2}
                placeholder="What do you think is wrong, and how will you check?"
                className="flex-1 resize-none rounded-xl border border-[#E3E8F2] p-3 text-sm outline-none focus:border-[#2F5BD3]"
                aria-label={`Hypothesis ${i + 1}`}
              />
              <select
                value={h.status}
                onChange={(e) => set(items.map((x, j) => (j === i ? { ...x, status: e.target.value as Hypothesis["status"] } : x)))}
                className="h-10 rounded-lg border border-[#E3E8F2] bg-white px-2 text-sm"
                aria-label={`Status of hypothesis ${i + 1}`}
              >
                <option value="open">Open</option>
                <option value="confirmed">Confirmed</option>
                <option value="ruled_out">Ruled out</option>
              </select>
              <button
                type="button"
                onClick={() => set(items.filter((_, j) => j !== i))}
                className="h-10 rounded-lg px-2 text-sm text-[#8A91AB] hover:text-[#D23B3B]"
                aria-label={`Remove hypothesis ${i + 1}`}
              >
                Remove
              </button>
            </li>
          ))}
          {items.length === 0 && <li className="py-6 text-center text-sm text-[#8A91AB]">No hypotheses yet.</li>}
        </ul>
      </div>
    </div>
  );
}
