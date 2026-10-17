import "server-only";
import { db } from "@/lib/supabase";
import { readJobPreferences, type JobPreferences } from "./job-preferences";
import type { CandidateDocument, DocumentKind, DocumentList } from "./documents-shape";

/**
 * Reads for the profile's newer parts — job preferences, supporting
 * documents, and the resume file's details — kept apart from getProfile()
 * on purpose: they depend on migration 0024, and a select naming a column
 * that isn't there yet fails the whole query. Read separately, a database
 * without 0024 still loads the profile, and these say "not set up yet".
 */

export const DOCUMENTS_BUCKET = "candidate-documents";

/** PostgREST's "no such table" and Postgres's "no such column". */
export function isMissingSchema(error: unknown): boolean {
  const code = (error as { code?: string } | null)?.code;
  return code === "PGRST205" || code === "42P01" || code === "42703" || code === "PGRST204";
}

export type PreferencesRead = { available: true; value: JobPreferences | null } | { available: false; value: null };

export async function getJobPreferences(candidateId: string): Promise<PreferencesRead> {
  const c = db();
  if (!c) return { available: false, value: null };
  const { data, error } = await c.from("candidate_users").select("job_preferences").eq("id", candidateId).maybeSingle();
  if (error) {
    if (!isMissingSchema(error)) console.error("profile: couldn't read job preferences:", error.message);
    return { available: false, value: null };
  }
  return { available: true, value: readJobPreferences(data?.job_preferences) };
}

export async function listDocuments(candidateId: string): Promise<DocumentList> {
  const c = db();
  if (!c) return { available: false, items: [] };
  const { data, error } = await c
    .from("candidate_documents")
    .select("id, kind, title, mime_type, size_bytes, uploaded_at")
    .eq("candidate_id", candidateId)
    .order("uploaded_at", { ascending: false });
  if (error) {
    if (!isMissingSchema(error)) console.error("profile: couldn't list documents:", error.message);
    return { available: false, items: [] };
  }
  return {
    available: true,
    items: (data ?? []).map(
      (r): CandidateDocument => ({
        id: r.id as string,
        kind: r.kind as DocumentKind,
        title: r.title as string,
        mimeType: r.mime_type as string,
        sizeBytes: r.size_bytes as number,
        uploadedAt: r.uploaded_at as string,
      })
    ),
  };
}

export interface ResumeFile {
  fileName: string;
  mimeType: string | null;
  sizeBytes: number | null;
  /** When it was uploaded — from the time stamped into its stored name. */
  uploadedAt: string | null;
}

/**
 * The stored resume's name, type, size and upload time. The path is
 * `<candidate>/<ms since epoch>-<name>` (uploadResume), so the time comes
 * from the path; the size and type come from the storage object itself.
 * Either can be null if storage won't say.
 */
export async function getResumeFile(path: string | null | undefined): Promise<ResumeFile | null> {
  if (!path) return null;
  const slash = path.lastIndexOf("/");
  const folder = path.slice(0, slash);
  const object = path.slice(slash + 1);
  const dash = object.indexOf("-");
  const stamp = Number(object.slice(0, dash));
  const fileName = (dash > 0 ? object.slice(dash + 1) : object) || "Resume";
  const uploadedAt = Number.isFinite(stamp) && stamp > 0 ? new Date(stamp).toISOString() : null;

  let sizeBytes: number | null = null;
  let mimeType: string | null = null;
  const c = db();
  if (c) {
    const { data } = await c.storage.from("resumes").list(folder, { search: object, limit: 5 });
    const hit = data?.find((f) => f.name === object);
    const meta = (hit?.metadata ?? {}) as { size?: number; mimetype?: string };
    sizeBytes = typeof meta.size === "number" ? meta.size : null;
    mimeType = typeof meta.mimetype === "string" ? meta.mimetype : null;
  }
  if (!mimeType) mimeType = fileName.toLowerCase().endsWith(".pdf") ? "application/pdf" : null;
  return { fileName, mimeType, sizeBytes, uploadedAt };
}
