"use client";

import { useState } from "react";
import clsx from "clsx";
import { Activity, Flag, Info, TriangleAlert } from "lucide-react";
import type { ActivityItem, ActivitySeverity } from "@/lib/v2/panel/types";
import { TabHeader } from "./NotesTab";

const CATEGORY: Record<string, string> = {
  observed: "Observed",
  verified: "Verified",
  signal: "Review signal",
};

const SOURCE: Record<ActivityItem["source"], string> = { watcher: "CLI", panel: "Panel", server: "Mindfries" };

/**
 * The tracker: every notification and flag, newest first. Flags are stated
 * as facts in the spec's three categories (§16); none of them is a verdict,
 * and none blocks the candidate.
 */
export function ActivityTab({
  items,
  onRespond,
}: {
  items: ActivityItem[];
  /** Saves what the candidate says about a flag or notice; resolves to an error message, or null when saved. */
  onRespond?: (id: string, text: string) => Promise<string | null>;
}) {
  const [only, setOnly] = useState<"all" | "flags">("all");
  const [answering, setAnswering] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  async function send(id: string) {
    if (!onRespond || !draft.trim()) return;
    setSaving(true);
    setProblem(null);
    const err = await onRespond(id, draft.trim());
    setSaving(false);
    if (err) return setProblem(err);
    setAnswering(null);
    setDraft("");
  }
  const shown = [...items].reverse().filter((i) => only === "all" || i.severity !== "info");
  return (
    <div className="flex h-full flex-col">
      <TabHeader icon={<Activity className="h-4 w-4" />} title="Activity" hint="Everything recorded about your session." />
      <div className="flex gap-2 px-4 pt-3" role="tablist" aria-label="Filter">
        {(["all", "flags"] as const).map((f) => (
          <button
            key={f}
            type="button"
            role="tab"
            aria-selected={only === f}
            onClick={() => setOnly(f)}
            className={clsx("rounded-full px-3 py-1 text-sm", only === f ? "bg-[#EAF0FE] font-medium text-[#2F5BD3]" : "text-[#3B4568] hover:bg-[#F4F6FB]")}
          >
            {f === "all" ? "Everything" : "Flags and notices"}
          </button>
        ))}
      </div>
      <ol className="flex-1 space-y-2 overflow-auto p-4" aria-live="polite">
        {shown.map((i) => (
          <li key={i.id} className={clsx("flex gap-3 rounded-xl border px-4 py-3", tone(i.severity))}>
            <SeverityIcon s={i.severity} />
            <div className="min-w-0 flex-1">
              <p className="text-[14px] text-[#10183A]">{i.text}</p>
              <p className="mt-0.5 text-xs text-[#6B7393]">
                {new Date(i.at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} · {SOURCE[i.source]}
                {i.category ? ` · ${CATEGORY[i.category]}` : ""}
              </p>
              {i.response && (
                <p className="mt-2 rounded-lg bg-white/70 px-3 py-2 text-[13px] text-[#3B4568]">
                  <span className="font-medium">Your note:</span> {i.response}
                </p>
              )}
              {onRespond && i.severity !== "info" && answering !== i.id && (
                <button
                  type="button"
                  onClick={() => {
                    setAnswering(i.id);
                    setDraft(i.response ?? "");
                    setProblem(null);
                  }}
                  className="mt-2 text-xs font-medium text-[#2F5BD3] hover:underline"
                >
                  {i.response ? "Edit your note" : "Add context"}
                </button>
              )}
              {answering === i.id && (
                <div className="mt-2">
                  <textarea
                    autoFocus
                    value={draft}
                    onChange={(e) => setDraft(e.target.value)}
                    rows={3}
                    maxLength={2000}
                    placeholder="In your own words: what was this? The hiring team will read it next to the flag."
                    className="w-full resize-none rounded-lg border border-[#DCE3F1] bg-white px-3 py-2 text-sm outline-none focus:border-[#2F5BD3]"
                    aria-label="What this was"
                  />
                  {problem && <p className="mt-1 text-xs text-[#D23B3B]">{problem}</p>}
                  <div className="mt-2 flex gap-2">
                    <button
                      type="button"
                      disabled={saving || !draft.trim()}
                      onClick={() => void send(i.id)}
                      className="rounded-lg bg-[#2F5BD3] px-3 py-1.5 text-xs font-medium text-white disabled:opacity-50"
                    >
                      {saving ? "Saving..." : "Save note"}
                    </button>
                    <button type="button" onClick={() => setAnswering(null)} className="rounded-lg px-3 py-1.5 text-xs text-[#6B7393] hover:bg-white">
                      Cancel
                    </button>
                  </div>
                </div>
              )}
            </div>
          </li>
        ))}
        {shown.length === 0 && <li className="py-8 text-center text-sm text-[#8A91AB]">Nothing flagged.</li>}
      </ol>
    </div>
  );
}

function tone(s: ActivitySeverity) {
  return s === "flag" ? "border-[#F5C2C3] bg-[#FFF6F6]" : s === "notice" ? "border-[#F6DFA6] bg-[#FFFBF0]" : "border-[#E8EDF7] bg-white";
}

function SeverityIcon({ s }: { s: ActivitySeverity }) {
  if (s === "flag") return <Flag className="mt-0.5 h-4 w-4 shrink-0 text-[#D23B3B]" aria-label="Flag" />;
  if (s === "notice") return <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-[#B7791F]" aria-label="Notice" />;
  return <Info className="mt-0.5 h-4 w-4 shrink-0 text-[#2F5BD3]" aria-label="Info" />;
}
