"use client";

import { BookOpen, ExternalLink, FileText, Link as LinkIcon } from "lucide-react";
import type { Resource } from "@/lib/v2/panel/types";
import { TabHeader } from "./NotesTab";

/** Resources: the task's own docs and links. Looking things up is allowed. */
export function ResourcesTab({ resources }: { resources: Resource[] }) {
  return (
    <div className="flex h-full flex-col">
      <TabHeader icon={<BookOpen className="h-4 w-4" />} title="Resources" hint="Docs for this task. Searching the web is fine too." />
      <ul className="flex-1 space-y-2 overflow-auto p-4">
        {resources.map((r) => {
          const Icon = r.kind === "link" ? LinkIcon : FileText;
          const body = (
            <>
              <Icon className="h-5 w-5 shrink-0 text-[#2F5BD3]" aria-hidden />
              <div className="min-w-0 flex-1">
                <div className="truncate text-[14px] font-medium text-[#10183A]">{r.title}</div>
                <div className="truncate text-xs text-[#6B7393]">{r.detail}</div>
              </div>
              {r.href && <ExternalLink className="h-4 w-4 shrink-0 text-[#8A91AB]" aria-hidden />}
            </>
          );
          return (
            <li key={r.id}>
              {r.href ? (
                <a href={r.href} target="_blank" rel="noopener noreferrer" className="flex items-center gap-3 rounded-xl border border-[#E8EDF7] px-4 py-3 hover:bg-[#F7F9FD]">
                  {body}
                </a>
              ) : (
                <div className="flex items-center gap-3 rounded-xl border border-[#E8EDF7] px-4 py-3">{body}</div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
