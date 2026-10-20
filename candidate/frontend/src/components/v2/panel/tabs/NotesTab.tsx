"use client";

import { FileText } from "lucide-react";

/**
 * Notes: the candidate's decision log. Plain text (Markdown is fine);
 * timestamped and saved as evidence from step 6.
 */
export function NotesTab({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const words = value.trim() ? value.trim().split(/\s+/).length : 0;
  return (
    <div className="flex h-full flex-col">
      <TabHeader icon={<FileText className="h-4 w-4" />} title="Notes" hint="Your decisions, trade-offs and open questions. Reviewers read these." />
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        spellCheck
        className="flex-1 resize-none bg-white px-5 py-4 font-mono text-[14px] leading-6 text-[#10183A] outline-none"
        aria-label="Notes"
        placeholder="Why this design? What would you do with more time?"
      />
      <div className="border-t border-[#EEF1F7] px-5 py-2 text-xs text-[#8A91AB]">{words} words · kept with your session</div>
    </div>
  );
}

export function TabHeader({ icon, title, hint }: { icon: React.ReactNode; title: string; hint?: string }) {
  return (
    <div className="flex items-center gap-2 border-b border-[#EEF1F7] px-5 py-3">
      <span className="text-[#2F5BD3]">{icon}</span>
      <h2 className="text-[15px] font-medium text-[#10183A]">{title}</h2>
      {hint && <p className="ml-2 truncate text-sm text-[#6B7393]">{hint}</p>}
    </div>
  );
}
