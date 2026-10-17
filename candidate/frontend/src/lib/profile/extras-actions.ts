"use server";

import { revalidatePath } from "next/cache";
import { db } from "@/lib/supabase";
import { currentCandidate } from "@/lib/auth/users";
import { OPEN_TO_OPTIONS } from "./data";
import { cleanJobPreferences } from "./job-preferences";
import { DOCUMENTS_BUCKET, isMissingSchema } from "./extras";
import {
  DOCUMENT_EXT,
  DOCUMENT_KINDS,
  DOCUMENT_MIME,
  MAX_DOCUMENT_BYTES,
  MAX_DOCUMENTS,
  MAX_TITLE,
  type DocumentKind,
} from "./documents-shape";

type Result = { ok: true } | { ok: false; error: string };

const NOT_SET_UP = "This part of the profile isn't set up on the server yet.";

/**
 * Saves the job preferences form. Work mode goes to candidate_users.open_to
 * — the same field Edit profile sets — and the rest to job_preferences.
 * Everything is checked here (cleanJobPreferences): the form's limits are a
 * courtesy, this is what holds.
 */
export async function saveJobPreferences(input: unknown, workModes: unknown): Promise<Result> {
  const session = await currentCandidate();
  if (!session) return { ok: false, error: "Not signed in." };
  const c = db();
  if (!c) return { ok: false, error: "The database isn't connected." };

  const cleaned = cleanJobPreferences(input);
  if (!cleaned.ok) return cleaned;
  const openTo = (Array.isArray(workModes) ? workModes : []).filter((m): m is string => OPEN_TO_OPTIONS.includes(m as string));

  const { error } = await c
    .from("candidate_users")
    .update({ job_preferences: { ...cleaned.value, updatedAt: new Date().toISOString() }, open_to: [...new Set(openTo)] })
    .eq("id", session.id);
  if (error) return { ok: false, error: isMissingSchema(error) ? NOT_SET_UP : "Couldn't save your preferences — try again." };

  revalidatePath("/profile");
  return { ok: true };
}

/**
 * Adds a supporting document: the file to the private bucket under the
 * candidate's own folder, then the row that lists it. If the row can't be
 * written the file is removed again, so nothing is left in storage that no
 * list points at.
 */
export async function uploadDocument(formData: FormData): Promise<Result> {
  const session = await currentCandidate();
  if (!session) return { ok: false, error: "Not signed in." };
  const c = db();
  if (!c) return { ok: false, error: "The database isn't connected." };

  const file = formData.get("file");
  if (!(file instanceof File) || file.size === 0) return { ok: false, error: "Choose a file first." };
  const ext = `.${file.name.split(".").pop()?.toLowerCase()}`;
  if (!(DOCUMENT_MIME as readonly string[]).includes(file.type) || !DOCUMENT_EXT.includes(ext)) {
    return { ok: false, error: "PDF, PNG or JPEG only." };
  }
  if (file.size > MAX_DOCUMENT_BYTES) return { ok: false, error: "Keep it under 4.5 MB." };

  const kindRaw = formData.get("kind");
  const kind: DocumentKind = (DOCUMENT_KINDS as readonly string[]).includes(kindRaw as string) ? (kindRaw as DocumentKind) : "other";
  const titleRaw = formData.get("title");
  const title =
    (typeof titleRaw === "string" ? titleRaw.replace(/\s+/g, " ").trim().slice(0, MAX_TITLE) : "") ||
    file.name.replace(/\.[^.]+$/, "").slice(0, MAX_TITLE);

  const { count, error: countError } = await c
    .from("candidate_documents")
    .select("id", { count: "exact", head: true })
    .eq("candidate_id", session.id);
  if (countError) return { ok: false, error: isMissingSchema(countError) ? NOT_SET_UP : "Couldn't add that — try again." };
  if ((count ?? 0) >= MAX_DOCUMENTS) return { ok: false, error: `You can keep up to ${MAX_DOCUMENTS} documents. Remove one first.` };

  const path = `${session.id}/${Date.now()}-${file.name.replace(/[^a-zA-Z0-9.-]/g, "_")}`;
  const { error: uploadError } = await c.storage.from(DOCUMENTS_BUCKET).upload(path, file, { contentType: file.type });
  if (uploadError) return { ok: false, error: "Couldn't upload that file — try again." };

  const { error } = await c.from("candidate_documents").insert({
    candidate_id: session.id,
    kind,
    title,
    path,
    mime_type: file.type,
    size_bytes: file.size,
  });
  if (error) {
    await c.storage.from(DOCUMENTS_BUCKET).remove([path]);
    return { ok: false, error: isMissingSchema(error) ? NOT_SET_UP : "Couldn't add that — try again." };
  }

  revalidatePath("/profile");
  return { ok: true };
}

/** Removes one of the candidate's own documents, row and file. */
export async function removeDocument(id: string): Promise<Result> {
  const session = await currentCandidate();
  if (!session) return { ok: false, error: "Not signed in." };
  const c = db();
  if (!c) return { ok: false, error: "The database isn't connected." };

  const { data, error } = await c
    .from("candidate_documents")
    .delete()
    .eq("id", id)
    .eq("candidate_id", session.id)
    .select("path")
    .maybeSingle();
  if (error) return { ok: false, error: "Couldn't remove that — try again." };
  if (data?.path) await c.storage.from(DOCUMENTS_BUCKET).remove([data.path as string]);

  revalidatePath("/profile");
  return { ok: true };
}

/** A link to one of the candidate's own documents that works for an hour. */
export async function documentLink(id: string): Promise<{ ok: true; url: string } | { ok: false; error: string }> {
  const session = await currentCandidate();
  if (!session) return { ok: false, error: "Not signed in." };
  const c = db();
  if (!c) return { ok: false, error: "The database isn't connected." };

  const { data } = await c.from("candidate_documents").select("path").eq("id", id).eq("candidate_id", session.id).maybeSingle();
  if (!data?.path) return { ok: false, error: "That document isn't there any more." };
  const { data: signed } = await c.storage.from(DOCUMENTS_BUCKET).createSignedUrl(data.path as string, 3600);
  return signed?.signedUrl ? { ok: true, url: signed.signedUrl } : { ok: false, error: "Couldn't open that — try again." };
}
