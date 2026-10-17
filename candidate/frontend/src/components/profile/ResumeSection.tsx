"use client";

import { useRef, useState } from "react";
import { CalendarDays, ExternalLink, FileText, Loader2, PenLine, ShieldCheck, Sparkles, Trash2, Upload } from "lucide-react";
import { ACCEPTED_EXT, ACCEPTED_MIME, formatBytes, MAX_RESUME_BYTES } from "@/lib/profile/useResumeUploader";
import { fillProfileFromResume, removeResume, uploadResume } from "@/lib/profile/actions";
import { fileTypeLabel } from "@/lib/profile/documents-shape";
import type { ResumeFields } from "@/lib/profile/resume-fields";
import type { ResumeFile } from "@/lib/profile/extras";
import { PdfPreview } from "./PdfPreview";
import { usePreviewMode } from "./PreviewMode";

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString("en-GB", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

/**
 * The Resume window: the file itself on the left (PdfPreview, for a PDF),
 * and on the right what it is, when it came in, and what can be done with
 * it — replace it, fill empty profile fields from it, remove it.
 *
 * Uploading is a real file picker: the text is read in this browser first
 * (lib/profile/resumeParse.ts) and sent with the file, so the server can
 * keep what the resume says beside it. A file that can't be read (a .doc, a
 * scan) is still uploaded, and the window says nothing could be read.
 */
export function ResumeSection({
  resumePath,
  resumeUrl,
  file,
  parsed,
  emptyFields,
}: {
  resumePath?: string | null;
  resumeUrl: string | null;
  file: ResumeFile | null;
  parsed: ResumeFields | null;
  /** Profile fields left empty that the resume has something for. */
  emptyFields: string[];
}) {
  const preview = usePreviewMode();
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState<"upload" | "fill" | "remove" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [confirmRemove, setConfirmRemove] = useState(false);

  async function run(kind: "upload" | "fill" | "remove", task: () => Promise<void>) {
    setBusy(kind);
    setError(null);
    setNotice(null);
    try {
      await task();
    } catch {
      setError("Couldn't reach the server — check your connection and try again.");
    } finally {
      setBusy(null);
    }
  }

  function onFile(picked: File) {
    const ext = `.${picked.name.split(".").pop()?.toLowerCase()}`;
    if (!ACCEPTED_MIME.has(picked.type) && !ACCEPTED_EXT.includes(ext)) return setError("PDF, DOC or DOCX only.");
    if (picked.size > MAX_RESUME_BYTES) return setError(`That's ${formatBytes(picked.size)} — keep it under ${formatBytes(MAX_RESUME_BYTES)}.`);
    void run("upload", async () => {
      const form = new FormData();
      form.append("file", picked);
      // Reading the text is best-effort: a resume that can't be read is still worth keeping.
      try {
        const { extractResumeText } = await import("@/lib/profile/resumeParse");
        const text = await extractResumeText(picked);
        if (text.trim()) form.append("text", text);
      } catch {}
      const res = await uploadResume(form);
      if (!res.ok) setError(res.error || "Couldn't upload that file — try again.");
    });
  }

  const picker = (
    <input
      ref={input}
      type="file"
      accept={[...ACCEPTED_EXT, ...ACCEPTED_MIME].join(",")}
      className="sr-only"
      onChange={(e) => {
        const picked = e.target.files?.[0];
        e.target.value = ""; // so picking the same file again still fires
        if (picked) onFile(picked);
      }}
    />
  );

  if (!resumePath) {
    return (
      <div className="rounded-2xl border-2 border-dashed border-[#DCD5F3] bg-white/60 px-6 py-12 text-center">
        <span className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-[#EFEAFD] text-[#5B3BE0]">
          <FileText size={28} strokeWidth={1.7} />
        </span>
        <p className="mt-4 text-[17px] font-semibold text-[#100E2E]">No resume yet</p>
        <p className="mx-auto mt-1 max-w-sm text-[14px] text-[#5E5A82]">
          PDF, DOC or DOCX, up to {formatBytes(MAX_RESUME_BYTES)}. Only hiring teams you have an assessment with can see it.
        </p>
        {!preview && (
          <button
            type="button"
            onClick={() => input.current?.click()}
            disabled={busy !== null}
            className="mt-5 inline-flex h-11 items-center gap-2 rounded-xl bg-[#5B3BE0] px-5 text-[14.5px] font-semibold text-white hover:bg-[#4B2BB8] disabled:opacity-60"
          >
            {busy === "upload" ? <Loader2 size={17} className="animate-spin" /> : <Upload size={17} />}
            {busy === "upload" ? "Uploading…" : "Upload a resume"}
          </button>
        )}
        {error && <p className="mt-3 text-[13px] text-[#a6203c]">{error}</p>}
        {picker}
      </div>
    );
  }

  const name = file?.fileName ?? "Resume";
  const isPdf = (file?.mimeType ?? "") === "application/pdf" || name.toLowerCase().endsWith(".pdf");

  return (
    <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_300px]">
      <div className="min-w-0 rounded-[20px] border border-[#ECE9F6] bg-white p-3 sm:p-4">
        {resumeUrl && isPdf ? (
          <PdfPreview url={resumeUrl} fileName={name} />
        ) : (
          <div className="flex min-h-[320px] flex-col items-center justify-center gap-3 rounded-2xl bg-[#F1EFF8] p-6 text-center">
            <FileText size={30} strokeWidth={1.6} className="text-[#6A47E6]" />
            <p className="max-w-xs text-[14.5px] text-[#4C4874]">
              {resumeUrl ? "Word files can't be previewed in the browser." : "The file couldn't be fetched just now."}
            </p>
            {resumeUrl && (
              <a href={resumeUrl} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 text-[14px] font-semibold text-[#5B3BE0] hover:underline">
                Open {name} <ExternalLink size={14} />
              </a>
            )}
          </div>
        )}
      </div>

      <aside className="min-w-0 rounded-[20px] border border-[#ECE9F6] bg-white p-5">
        <h3 className="text-[16px] font-semibold text-[#100E2E]">File information</h3>
        <dl className="mt-4 space-y-4">
          <InfoRow icon={<FileText size={20} strokeWidth={1.7} />} title={name}>
            {[file?.mimeType ? fileTypeLabel(file.mimeType) : null, file?.sizeBytes != null ? formatBytes(file.sizeBytes) : null].filter(Boolean).join(" · ") ||
              "Type and size unknown"}
          </InfoRow>
          {file?.uploadedAt && (
            <InfoRow icon={<CalendarDays size={20} strokeWidth={1.7} />} title="Uploaded on">
              {formatDateTime(file.uploadedAt)}
            </InfoRow>
          )}
          <InfoRow icon={<ShieldCheck size={20} strokeWidth={1.7} />} title="Who sees it">
            Hiring teams you have an assessment with.
          </InfoRow>
        </dl>

        {!preview && (
          <div className="mt-5 space-y-2.5 border-t border-[#ECE9F6] pt-5">
            <button
              type="button"
              onClick={() => input.current?.click()}
              disabled={busy !== null}
              className="flex h-12 w-full items-center justify-center gap-2 rounded-xl bg-[#5B3BE0] text-[15px] font-semibold text-white hover:bg-[#4B2BB8] disabled:opacity-60"
            >
              {busy === "upload" ? <Loader2 size={18} className="animate-spin" /> : <Upload size={18} />}
              {busy === "upload" ? "Uploading…" : "Replace file"}
            </button>
            {emptyFields.length > 0 && (
              <button
                type="button"
                onClick={() =>
                  void run("fill", async () => {
                    const res = await fillProfileFromResume();
                    if (!res.ok) setError(res.error || "Couldn't fill in your profile.");
                    else setNotice(res.filled.length > 0 ? `Filled in your ${res.filled.join(", ")}. Check it reads right.` : "Nothing left to fill in.");
                  })
                }
                disabled={busy !== null}
                className="flex h-12 w-full items-center justify-center gap-2 rounded-xl border border-[#D9D1F7] bg-white text-[14.5px] font-semibold text-[#4B2BB8] hover:bg-[#F6F4FC] disabled:opacity-60"
              >
                {busy === "fill" ? <Loader2 size={17} className="animate-spin" /> : <PenLine size={17} />}
                Fill my {emptyFields.join(", ")} from it
              </button>
            )}
            {confirmRemove ? (
              <div className="rounded-xl bg-[#FDF1F3] p-3 text-[13px] text-[#7A1F33]">
                Remove your resume? Hiring teams won&apos;t see it any more.
                <div className="mt-2 flex gap-2">
                  <button type="button" onClick={() => setConfirmRemove(false)} className="rounded-lg px-3 py-1.5 font-medium hover:bg-white/70">
                    Keep it
                  </button>
                  <button
                    type="button"
                    disabled={busy !== null}
                    onClick={() =>
                      void run("remove", async () => {
                        const res = await removeResume();
                        if (!res.ok) setError(res.error || "Couldn't remove it.");
                      })
                    }
                    className="rounded-lg bg-[#a6203c] px-3 py-1.5 font-semibold text-white disabled:opacity-60"
                  >
                    {busy === "remove" ? "Removing…" : "Remove"}
                  </button>
                </div>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setConfirmRemove(true)}
                className="flex w-full items-center justify-center gap-1.5 py-1.5 text-[13px] font-medium text-[#6E6A8F] hover:text-[#a6203c]"
              >
                <Trash2 size={14} /> Remove resume
              </button>
            )}
            {error && <p className="text-[13px] text-[#a6203c]">{error}</p>}
            {notice && <p className="text-[13px] text-[#14693a]">{notice}</p>}
          </div>
        )}

        <div className="mt-5 rounded-2xl bg-[#F4F1FD] p-4">
          <p className="flex items-center gap-2 text-[14px] font-semibold text-[#100E2E]">
            <Sparkles size={16} className="text-[#5B3BE0]" /> What we read from it
          </p>
          {parsed && parsed.skills.length > 0 ? (
            <>
              <div className="mt-2.5 flex flex-wrap gap-1.5">
                {parsed.skills.map((skill) => (
                  <span key={skill} className="rounded-lg bg-white px-2 py-0.5 text-[12.5px] text-[#2A2654]">
                    {skill}
                  </span>
                ))}
              </div>
              {parsed.years && (
                <p className="mt-2.5 text-[12.5px] text-[#5E5A82]">
                  Mentions {parsed.years.from}–{parsed.years.to}.
                </p>
              )}
              <p className="mt-2 text-[12px] leading-relaxed text-[#6E6A8F]">Matched against a fixed list of technologies — words found, not skill judged.</p>
            </>
          ) : (
            <p className="mt-2 text-[13px] leading-relaxed text-[#5E5A82]">
              {parsed
                ? "It was read, but none of the technologies we look for are named in it."
                : "Nothing could be read from this file — a .doc, or a scan with no text. It's still saved; a PDF or DOCX with real text lets hiring teams see what it says."}
            </p>
          )}
        </div>
        {picker}
      </aside>
    </div>
  );
}

function InfoRow({ icon, title, children }: { icon: React.ReactNode; title: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-3">
      <span className="mt-0.5 shrink-0 text-[#4C4874]">{icon}</span>
      <div className="min-w-0">
        <dt className="truncate text-[14.5px] font-medium text-[#100E2E]">{title}</dt>
        <dd className="text-[13.5px] text-[#6E6A8F]">{children}</dd>
      </div>
    </div>
  );
}
