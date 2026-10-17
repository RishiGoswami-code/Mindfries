/**
 * Supporting documents on a candidate's profile (candidate_documents,
 * migration 0024): the shapes and limits, shared by the upload form, the
 * server action and the list. No server or browser imports, so both sides
 * can use it.
 */

export const DOCUMENT_KINDS = ["certificate", "report", "other"] as const;
export type DocumentKind = (typeof DOCUMENT_KINDS)[number];

export const DOCUMENT_KIND_LABELS: Record<DocumentKind, string> = {
  certificate: "Certificate",
  report: "Project report",
  other: "Other",
};

export const DOCUMENT_MIME = ["application/pdf", "image/png", "image/jpeg"] as const;
export const DOCUMENT_EXT = [".pdf", ".png", ".jpg", ".jpeg"];
/** The bucket allows 10MB; server actions are capped at 5MB (next.config.ts), so that's the real limit. */
export const MAX_DOCUMENT_BYTES = 4.5 * 1024 * 1024;
export const MAX_DOCUMENTS = 12;
export const MAX_TITLE = 120;

export interface CandidateDocument {
  id: string;
  kind: DocumentKind;
  title: string;
  mimeType: string;
  sizeBytes: number;
  uploadedAt: string;
}

/**
 * `available: false` means the documents table isn't in this database yet
 * (migration 0024 not applied) — said as such, rather than shown as an
 * empty list the candidate can't add to.
 */
export type DocumentList = { available: true; items: CandidateDocument[] } | { available: false; items: [] };

/** "PDF", "PNG", "JPEG" — what to call a file by its type. */
export function fileTypeLabel(mime: string): string {
  if (mime === "application/pdf") return "PDF";
  if (mime === "image/png") return "PNG";
  if (mime === "image/jpeg") return "JPEG";
  if (mime.includes("wordprocessingml")) return "DOCX";
  if (mime === "application/msword") return "DOC";
  return "File";
}
