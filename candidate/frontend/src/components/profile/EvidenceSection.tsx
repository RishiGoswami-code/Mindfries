"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import clsx from "clsx";
import { Award, CheckCircle2, ChevronRight, Clock3, FileText, Loader2, Mail, Trash2, Upload } from "lucide-react";
import { statusLabels, type Assessment } from "@/lib/dashboard/data";
import {
  DOCUMENT_EXT,
  DOCUMENT_KIND_LABELS,
  DOCUMENT_KINDS,
  DOCUMENT_MIME,
  MAX_DOCUMENT_BYTES,
  MAX_DOCUMENTS,
  MAX_TITLE,
  fileTypeLabel,
  type CandidateDocument,
  type DocumentKind,
  type DocumentList,
} from "@/lib/profile/documents-shape";
import { documentLink, removeDocument, uploadDocument } from "@/lib/profile/extras-actions";
import { formatBytes, formatDate } from "@/lib/profile/useResumeUploader";
import { StatePill } from "./ProfileCard";
import { usePreviewMode } from "./PreviewMode";

type Tab = "overview" | "assessments" | "documents";

const STATE: Record<Assessment["status"], "done" | "in-progress" | "todo" | "info"> = {
  submitted: "done",
  "in-progress": "in-progress",
  invited: "todo",
  closed: "info",
};

/**
 * The Evidence window: the candidate's assessments, and documents they add
 * to back up their profile — certificates, project reports.
 *
 * Assessments are the same list the dashboard shows; each opens its report
 * or its workspace. There are no scores here: an assessment's evidence is
 * its report, which says how the candidate worked, not a number.
 */
export function EvidenceSection({ assessments, documents }: { assessments?: Assessment[]; documents: DocumentList }) {
  const [tab, setTab] = useState<Tab>("overview");
  const items = assessments ?? [];
  const completed = items.filter((a) => a.status === "submitted" || a.status === "closed").length;
  const inProgress = items.filter((a) => a.status === "in-progress").length;
  const open = items.filter((a) => a.status === "invited").length;

  const tabs: { id: Tab; label: string }[] = [
    { id: "overview", label: "Overview" },
    { id: "assessments", label: `Assessments (${items.length})` },
    { id: "documents", label: documents.available ? `Documents (${documents.items.length})` : "Documents" },
  ];

  return (
    <div>
      <div role="tablist" className="-mx-1 flex gap-1 overflow-x-auto overflow-y-hidden border-b border-[#ECE9F6]">
        {tabs.map((t) => (
          <button
            key={t.id}
            role="tab"
            type="button"
            aria-selected={tab === t.id}
            onClick={() => setTab(t.id)}
            className={clsx("relative shrink-0 px-4 py-3 text-[15px] transition-colors", tab === t.id ? "font-medium text-[#5B3BE0]" : "text-[#4C4874] hover:text-[#100E2E]")}
          >
            {t.label}
            {tab === t.id && <span className="absolute inset-x-2 -bottom-px h-[3px] rounded-full bg-[#5B3BE0]" />}
          </button>
        ))}
      </div>

      <div className="mt-5 space-y-5">
        {tab === "overview" && (
          <>
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <Stat icon={<FileText size={22} strokeWidth={1.7} />} tint="bg-[#F1EDFC] text-[#5B3BE0]" value={items.length} label="Assessments" note="All you've been given" />
              <Stat icon={<CheckCircle2 size={22} strokeWidth={1.7} />} tint="bg-[#E3F6EC] text-[#14693a]" value={completed} label="Completed" note="Submitted or closed" />
              <Stat icon={<Clock3 size={22} strokeWidth={1.7} />} tint="bg-[#FFF3D6] text-[#8A5A00]" value={inProgress} label="In progress" note={open > 0 ? `${open} more waiting` : "None waiting"} />
              <Stat
                icon={<Award size={22} strokeWidth={1.7} />}
                tint="bg-[#EAF1FD] text-[#1E5AA8]"
                value={documents.available ? documents.items.length : "—"}
                label="Documents"
                note="Certificates and reports"
              />
            </div>

            <Panel title="Recent assessments" onViewAll={items.length > 3 ? () => setTab("assessments") : undefined}>
              <AssessmentList items={items.slice(0, 3)} unavailable={!assessments} />
            </Panel>
            <Panel title="Supporting documents" onViewAll={() => setTab("documents")} viewAllLabel={documents.items.length > 2 ? "View all" : "Manage"}>
              <DocumentRows documents={documents} limit={2} />
            </Panel>
          </>
        )}

        {tab === "assessments" && (
          <div className="rounded-[18px] border border-[#ECE9F6] bg-white p-5">
            <AssessmentList items={items} unavailable={!assessments} />
          </div>
        )}

        {tab === "documents" && (
          <>
            <UploadDocument documents={documents} />
            <div className="rounded-[18px] border border-[#ECE9F6] bg-white p-5">
              <DocumentRows documents={documents} />
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function Stat({ icon, tint, value, label, note }: { icon: React.ReactNode; tint: string; value: number | string; label: string; note: string }) {
  return (
    <div className="flex items-center gap-4 rounded-[18px] border border-[#ECE9F6] bg-white p-4">
      <span className={clsx("flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl", tint)}>{icon}</span>
      <div className="min-w-0">
        <p className="text-[26px] leading-none font-semibold tabular-nums text-[#100E2E]">{value}</p>
        <p className="mt-1 text-[14.5px] text-[#2A2654]">{label}</p>
        <p className="truncate text-[12.5px] text-[#6E6A8F]">{note}</p>
      </div>
    </div>
  );
}

function Panel({ title, onViewAll, viewAllLabel = "View all", children }: { title: string; onViewAll?: () => void; viewAllLabel?: string; children: React.ReactNode }) {
  return (
    <section className="rounded-[18px] border border-[#ECE9F6] bg-white p-5">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-[16.5px] font-semibold text-[#100E2E]">{title}</h3>
        {onViewAll && (
          <button type="button" onClick={onViewAll} className="text-[13.5px] font-medium text-[#5B3BE0] hover:underline">
            {viewAllLabel} →
          </button>
        )}
      </div>
      {children}
    </section>
  );
}

function AssessmentList({ items, unavailable }: { items: Assessment[]; unavailable: boolean }) {
  if (unavailable) return <p className="text-[14px] text-[#6E6A8F]">Your assessments couldn&apos;t be loaded just now.</p>;
  if (items.length === 0) return <p className="text-[14px] text-[#6E6A8F]">No assessments yet. They appear here once a company invites you.</p>;
  return (
    <ul className="divide-y divide-[#F0EEF7]">
      {items.map((a) => {
        const href =
          a.status === "submitted" && a.sessionId
            ? `/assessments/${encodeURIComponent(a.sessionId)}/report`
            : a.status === "in-progress" && a.sessionId
              ? `/ide?session=${encodeURIComponent(a.sessionId)}`
              : "/assessments";
        return (
          <li key={a.id}>
            <Link href={href} className="group flex items-center gap-4 py-3.5">
              <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-[#F1EDFC] text-[#5B3BE0]">
                {a.status === "invited" ? <Mail size={19} strokeWidth={1.7} /> : <FileText size={19} strokeWidth={1.7} />}
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-[15px] font-semibold text-[#100E2E] group-hover:underline">{a.role}</p>
                <p className="truncate text-[13px] text-[#6E6A8F]">{[a.company, ...a.tags.slice(0, 2)].filter(Boolean).join(" · ")}</p>
              </div>
              <StatePill state={STATE[a.status]}>{a.invitation?.declined ? "Declined" : statusLabels[a.status]}</StatePill>
              <span className="hidden w-[150px] shrink-0 text-right text-[13px] text-[#4C4874] sm:block">{a.due}</span>
              <ChevronRight size={18} className="shrink-0 text-[#1B1840]" />
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

function DocumentRows({ documents, limit }: { documents: DocumentList; limit?: number }) {
  const preview = usePreviewMode();
  if (!documents.available) {
    return <p className="text-[14px] text-[#6E6A8F]">Documents aren&apos;t set up on the server yet, so none can be added.</p>;
  }
  const shown = limit ? documents.items.slice(0, limit) : documents.items;
  if (shown.length === 0) {
    return <p className="text-[14px] text-[#6E6A8F]">No documents yet. Add a certificate or a project report to back up your profile.</p>;
  }
  return (
    <ul className="divide-y divide-[#F0EEF7]">
      {shown.map((d) => (
        <DocumentRow key={d.id} doc={d} canRemove={!preview && !limit} />
      ))}
    </ul>
  );
}

function DocumentRow({ doc, canRemove }: { doc: CandidateDocument; canRemove: boolean }) {
  const [busy, setBusy] = useState<"open" | "remove" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirm, setConfirm] = useState(false);

  async function open() {
    // Opened before the request so the browser treats it as the click's own window, not a pop-up.
    const win = window.open("", "_blank");
    setBusy("open");
    setError(null);
    try {
      const res = await documentLink(doc.id);
      if (res.ok && win) win.location.href = res.url;
      else {
        win?.close();
        setError(res.ok ? "Your browser blocked the new tab." : res.error);
      }
    } catch {
      win?.close();
      setError("Couldn't reach the server — try again.");
    } finally {
      setBusy(null);
    }
  }

  async function remove() {
    setBusy("remove");
    setError(null);
    try {
      const res = await removeDocument(doc.id);
      if (!res.ok) setError(res.error);
    } catch {
      setError("Couldn't reach the server — try again.");
    } finally {
      setBusy(null);
    }
  }

  return (
    <li className="py-3.5">
      <div className="flex items-center gap-4">
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-[#F1EDFC] text-[#5B3BE0]">
          {doc.kind === "certificate" ? <Award size={19} strokeWidth={1.7} /> : <FileText size={19} strokeWidth={1.7} />}
        </span>
        <button type="button" onClick={() => void open()} disabled={busy !== null} className="min-w-0 flex-1 text-left">
          <p className="truncate text-[15px] font-semibold text-[#100E2E] hover:underline">{doc.title}</p>
          <p className="truncate text-[13px] text-[#6E6A8F]">
            {DOCUMENT_KIND_LABELS[doc.kind]} · {fileTypeLabel(doc.mimeType)} · {formatBytes(doc.sizeBytes)} · Uploaded {formatDate(doc.uploadedAt)}
          </p>
        </button>
        {busy === "open" && <Loader2 size={16} className="animate-spin text-[#6E6A8F]" />}
        {canRemove &&
          (confirm ? (
            <span className="flex shrink-0 items-center gap-2 text-[13px]">
              <button type="button" onClick={() => setConfirm(false)} className="text-[#4C4874] hover:underline">
                Keep
              </button>
              <button type="button" onClick={() => void remove()} disabled={busy !== null} className="font-semibold text-[#a6203c] hover:underline disabled:opacity-50">
                {busy === "remove" ? "Removing…" : "Remove"}
              </button>
            </span>
          ) : (
            <button type="button" onClick={() => setConfirm(true)} aria-label={`Remove ${doc.title}`} className="shrink-0 rounded-lg p-2 text-[#6E6A8F] hover:bg-[#FDF1F3] hover:text-[#a6203c]">
              <Trash2 size={16} />
            </button>
          ))}
        {!canRemove && <ChevronRight size={18} className="shrink-0 text-[#1B1840]" />}
      </div>
      {error && <p className="mt-1.5 pl-[60px] text-[13px] text-[#a6203c]">{error}</p>}
    </li>
  );
}

function UploadDocument({ documents }: { documents: DocumentList }) {
  const preview = usePreviewMode();
  const input = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [kind, setKind] = useState<DocumentKind>("certificate");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (preview || !documents.available) return null;
  if (documents.items.length >= MAX_DOCUMENTS) {
    return <p className="rounded-[18px] bg-[#F4F1FD] p-4 text-[13.5px] text-[#4C4874]">You have {MAX_DOCUMENTS} documents, the most a profile keeps. Remove one to add another.</p>;
  }

  function pick(f: File) {
    setError(null);
    const ext = `.${f.name.split(".").pop()?.toLowerCase()}`;
    if (!(DOCUMENT_MIME as readonly string[]).includes(f.type) || !DOCUMENT_EXT.includes(ext)) return setError("PDF, PNG or JPEG only.");
    if (f.size > MAX_DOCUMENT_BYTES) return setError(`That's ${formatBytes(f.size)} — keep it under ${formatBytes(MAX_DOCUMENT_BYTES)}.`);
    setFile(f);
    if (!title) setTitle(f.name.replace(/\.[^.]+$/, "").replace(/[_-]+/g, " ").slice(0, MAX_TITLE));
  }

  async function submit() {
    if (!file) return;
    setBusy(true);
    setError(null);
    try {
      const form = new FormData();
      form.append("file", file);
      form.append("title", title);
      form.append("kind", kind);
      const res = await uploadDocument(form);
      if (res.ok) {
        setFile(null);
        setTitle("");
      } else setError(res.error);
    } catch {
      setError("Couldn't upload that — check your connection and try again.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="rounded-[18px] border border-[#ECE9F6] bg-white p-5">
      <h3 className="text-[16.5px] font-semibold text-[#100E2E]">Add a document</h3>
      <p className="mt-0.5 text-[13.5px] text-[#6E6A8F]">
        PDF, PNG or JPEG, up to {formatBytes(MAX_DOCUMENT_BYTES)}. Hiring teams you have an assessment with can open it.
      </p>

      {!file ? (
        <button
          type="button"
          onClick={() => input.current?.click()}
          className="mt-4 flex w-full flex-col items-center gap-2 rounded-2xl border-2 border-dashed border-[#DCD5F3] bg-[#FBFAFE] px-4 py-7 text-[14px] text-[#4C4874] hover:border-[#B9A7F6]"
        >
          <Upload size={22} className="text-[#5B3BE0]" />
          Choose a file
        </button>
      ) : (
        <div className="mt-4 space-y-3">
          <p className="flex items-center gap-2 rounded-xl bg-[#F6F4FC] px-3 py-2 text-[13.5px] text-[#2A2654]">
            <FileText size={16} className="shrink-0 text-[#5B3BE0]" />
            <span className="min-w-0 flex-1 truncate">{file.name}</span>
            <span className="shrink-0 text-[#6E6A8F]">{formatBytes(file.size)}</span>
          </p>
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_200px]">
            <label className="block text-[13px] font-medium text-[#4C4874]">
              Title
              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                maxLength={MAX_TITLE}
                placeholder="e.g. Machine Learning certificate (Coursera)"
                className="mt-1 h-11 w-full rounded-xl border border-[#DCD7EE] bg-white px-3 text-[14px] font-normal text-[#100E2E] outline-none focus:border-[#7C5CFC]"
              />
            </label>
            <label className="block text-[13px] font-medium text-[#4C4874]">
              Kind
              <select
                value={kind}
                onChange={(e) => setKind(e.target.value as DocumentKind)}
                className="mt-1 h-11 w-full rounded-xl border border-[#DCD7EE] bg-white px-3 text-[14px] font-normal text-[#100E2E] outline-none focus:border-[#7C5CFC]"
              >
                {DOCUMENT_KINDS.map((k) => (
                  <option key={k} value={k}>
                    {DOCUMENT_KIND_LABELS[k]}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <div className="flex items-center justify-end gap-2">
            <button type="button" onClick={() => setFile(null)} disabled={busy} className="h-10 rounded-xl px-4 text-[14px] font-medium text-[#4C4874] hover:bg-[#F1EFF8]">
              Cancel
            </button>
            <button
              type="button"
              onClick={() => void submit()}
              disabled={busy || !title.trim()}
              className="inline-flex h-10 items-center gap-2 rounded-xl bg-[#5B3BE0] px-5 text-[14px] font-semibold text-white hover:bg-[#4B2BB8] disabled:opacity-60"
            >
              {busy && <Loader2 size={16} className="animate-spin" />}
              {busy ? "Uploading…" : "Add document"}
            </button>
          </div>
        </div>
      )}
      {error && <p className="mt-2 text-[13px] text-[#a6203c]">{error}</p>}
      <input
        ref={input}
        type="file"
        accept={[...DOCUMENT_EXT, ...DOCUMENT_MIME].join(",")}
        className="sr-only"
        onChange={(e) => {
          const f = e.target.files?.[0];
          e.target.value = "";
          if (f) pick(f);
        }}
      />
    </div>
  );
}
