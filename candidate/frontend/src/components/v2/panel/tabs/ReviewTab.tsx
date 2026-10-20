"use client";

import { Fragment, useState } from "react";
import clsx from "clsx";
import { GitPullRequest, MessageSquarePlus } from "lucide-react";
import type { PanelConfig, ReviewComment } from "@/lib/v2/panel/types";
import { TabHeader } from "./NotesTab";

/**
 * Code Review: a diff to read, with comments on any line (SPEC §10 E). The
 * comments are the evidence; step 6 saves them.
 */
export function ReviewTab({ review, onComments }: { review: PanelConfig["review"]; onComments?: (c: ReviewComment[]) => void }) {
  // A comment is kept by its file and its place in the diff as shown (1, 2, 3...),
  // which is unambiguous where the file's own line numbers repeat across - and + lines.
  const [local, setLocal] = useState<ReviewComment[]>(review.comments);
  const comments = onComments ? review.comments : local;
  const byKey = new Map(comments.map((c) => [`${c.file}:${c.line}`, c.text]));
  const [open, setOpen] = useState<string | null>(null);
  const setComment = (file: string, line: number, text: string) => {
    const rest = comments.filter((c) => !(c.file === file && c.line === line));
    const next = text.trim() ? [...rest, { file, line, text }] : rest;
    if (onComments) onComments(next);
    else setLocal(next);
  };
  return (
    <div className="flex h-full flex-col">
      <TabHeader icon={<GitPullRequest className="h-4 w-4" />} title={review.title} hint="Click a line to comment." />
      <div className="flex-1 overflow-auto p-4">
        {review.files.map((f) => (
          <div key={f.path} className="overflow-hidden rounded-xl border border-[#E8EDF7]">
            <div className="border-b border-[#E8EDF7] bg-[#F7F9FD] px-4 py-2 font-mono text-[13px] text-[#3B4568]">{f.path}</div>
            <table className="w-full border-collapse font-mono text-[13px]">
              <tbody>
                {f.lines.map((l, i) => {
                  const key = `${f.path}:${i + 1}`;
                  return (
                    <Fragment key={key}>
                      <tr
                        onClick={() => setOpen(open === key ? null : key)}
                        className={clsx(
                          "group cursor-pointer",
                          l.kind === "+" && "bg-[#EFFAF2]",
                          l.kind === "-" && "bg-[#FFF0F0]",
                        )}
                      >
                        <td className="w-10 select-none px-2 text-right text-[#A3A9BF]">{l.n}</td>
                        <td className={clsx("w-5 select-none text-center", l.kind === "+" ? "text-[#1F8A4C]" : l.kind === "-" ? "text-[#D23B3B]" : "text-[#A3A9BF]")}>{l.kind.trim()}</td>
                        <td className="whitespace-pre px-2 py-0.5 text-[#10183A]">{l.text}</td>
                        <td className="w-8 text-[#2F5BD3] opacity-0 group-hover:opacity-100">
                          <MessageSquarePlus className="h-4 w-4" aria-hidden />
                        </td>
                      </tr>
                      {(open === key || byKey.get(key)) && (
                        <tr>
                          <td colSpan={4} className="bg-[#F7F9FD] px-4 py-2">
                            <textarea
                              autoFocus={open === key}
                              value={byKey.get(key) ?? ""}
                              onChange={(e) => setComment(f.path, i + 1, e.target.value)}
                              rows={2}
                              placeholder="What would you change here, and why?"
                              className="w-full resize-none rounded-lg border border-[#DCE3F1] bg-white px-3 py-2 font-sans text-sm outline-none focus:border-[#2F5BD3]"
                              aria-label={`Comment on line ${l.n}`}
                            />
                          </td>
                        </tr>
                      )}
                    </Fragment>
                  );
                })}
              </tbody>
            </table>
          </div>
        ))}
        <p className="mt-3 text-xs text-[#8A91AB]">{comments.filter((c) => c.text.trim()).length} comments</p>
      </div>
    </div>
  );
}
