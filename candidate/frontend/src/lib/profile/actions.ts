"use server";

import { db } from "@/lib/supabase";
import { currentCandidate } from "@/lib/auth/users";
import { revalidatePath } from "next/cache";
import { MAX_RESUME_TEXT, parseResumeText, type ResumeFields } from "./resume-fields";
import { dropKnowledge, dropLinkedIn, isKnowledgeSource, storeKnowledge } from "./knowledge";
import { statsOf } from "./knowledge-shape";
import { readThroughConnection } from "./knowledge-connection";
import { endConnection } from "../composio";
import { PLATFORMS } from "./links";
import { NOTICE_PERIODS, OPEN_TO_OPTIONS } from "./data";
import type { CandidateProfile, StoredLinks } from "./types";

// The edit form limits what can be typed, but a server action is a public
// endpoint: these are the limits that actually hold. The bio's matches the
// form's counter; the others are generous for a real headline or city.
const MAX_NAME = 200;
const MAX_HEADLINE = 120;
const MAX_LOCATION = 120;
const MAX_BIO = 500;
const line = (v: unknown, max: number) => (typeof v === "string" ? v.replace(/\s+/g, " ").trim().slice(0, max) : "");

/** A linked account's knowledge base can be re-read this often, per account. */
const REFRESH_EVERY_MS = 10 * 60_000;

export async function getProfile(): Promise<CandidateProfile | null> {
  const session = await currentCandidate();
  if (!session) return null;

  const c = db();
  if (!c) return null;

  const { data } = await c
    .from("candidate_users")
    .select("name, role, location, bio, notice_period, open_to, resume_path, resume_parsed, resume_parsed_at, links, profile_updated_at, email_verified_at")
    .eq("id", session.id)
    .single();
  if (!data) return data;

  // What this returns is handed to client components, so it carries only
  // *that* an account is connected. The connection's id never leaves the
  // server; code that needs it reads the row itself.
  const links = Object.fromEntries(
    Object.entries((data.links ?? {}) as StoredLinks).map(([platform, link]) => [
      platform,
      link?.connectionId ? { ...link, connectionId: "connected" } : link,
    ]),
  );
  return { ...data, links };
}

export async function getResumeUrl(path: string | null) {
  if (!path) return null;
  const c = db();
  if (!c) return null;

  const { data } = await c.storage.from("resumes").createSignedUrl(path, 3600);
  return data?.signedUrl || null;
}

export async function saveIdentity(form: { name: string; role: string; location: string; openTo: string[]; noticePeriod: string; bio: string }) {
  const session = await currentCandidate();
  if (!session) return { error: "Not signed in" };

  const c = db();
  if (!c) return { error: "Database not connected" };

  const name = line(form?.name, MAX_NAME);
  const role = line(form?.role, MAX_HEADLINE);
  if (!name || !role) return { error: "Name and role can't be empty." };
  const bio = typeof form?.bio === "string" ? form.bio.trim().slice(0, MAX_BIO) : "";
  // Only the choices the form offers; anything else is dropped.
  const openTo = (Array.isArray(form?.openTo) ? form.openTo : []).filter((o) => OPEN_TO_OPTIONS.includes(o));
  const noticePeriod = NOTICE_PERIODS.includes(form?.noticePeriod) ? form.noticePeriod : "";

  const { error } = await c
    .from("candidate_users")
    .update({
      name,
      role,
      location: line(form?.location, MAX_LOCATION),
      open_to: openTo,
      notice_period: noticePeriod,
      bio,
      profile_updated_at: new Date().toISOString(),
    })
    .eq("id", session.id);

  if (error) return { error: error.message };

  revalidatePath("/profile");
  revalidatePath("/dashboard");
  return { ok: true };
}

/**
 * Saves the candidate's portfolio link — the one link that is typed in.
 * GitHub, GitLab and LinkedIn are only ever linked by signing in to them
 * (app/api/connect, lib/composio.ts), so a typed username can't stand in
 * for an account the candidate hasn't shown is theirs. The value is
 * normalised here as well as on the page: a server action is a public
 * endpoint.
 */
export async function saveLink(platform: string, rawValue: string) {
  const session = await currentCandidate();
  if (!session) return { error: "Not signed in" };
  if (platform !== "portfolio") return { error: "Connect this account by signing in to it." };

  const normalised = PLATFORMS.portfolio.normalize(String(rawValue).slice(0, 500));
  if (!normalised.ok) return { error: normalised.error };

  const c = db();
  if (!c) return { error: "Database not connected" };

  const { data } = await c.from("candidate_users").select("links").eq("id", session.id).single();
  const { error } = await c
    .from("candidate_users")
    .update({ links: { ...(data?.links || {}), portfolio: { value: normalised.value, savedAt: new Date().toISOString() } } })
    .eq("id", session.id);
  if (error) return { error: error.message };

  revalidatePath("/profile");
  return { ok: true };
}

/** Reads a linked GitHub or GitLab account again, so the knowledge base reflects it as it is now. */
export async function refreshKnowledge(platform: string) {
  const session = await currentCandidate();
  if (!session) return { error: "Not signed in" };
  if (!isKnowledgeSource(platform)) return { error: "Only GitHub and GitLab can be refreshed." };

  const c = db();
  if (!c) return { error: "Database not connected" };

  const { data } = await c.from("candidate_users").select("links").eq("id", session.id).single();
  const links = data?.links || {};
  const value = links[platform]?.value;
  if (typeof value !== "string" || !value) return { error: "That account isn't linked." };

  const { data: existing } = await c
    .from("candidate_knowledge")
    .select("fetched_at")
    .eq("candidate_id", session.id)
    .eq("source", platform)
    .maybeSingle();
  const age = existing?.fetched_at ? Date.now() - Date.parse(existing.fetched_at as string) : Infinity;
  if (age < REFRESH_EVERY_MS) {
    return { error: `That was read ${Math.max(1, Math.round(age / 60_000))} minutes ago — try again a little later.` };
  }

  try {
    // A refresh re-reads the same account, so it stays verified if it was —
    // and a connected account is re-read through its connection.
    // Only an account the candidate signed in to is read, and only through
    // that connection. One linked by username before sign-in was the only
    // way has to be connected first.
    const verifiedAs = links[platform]?.verified?.as;
    const connectionId = links[platform]?.connectionId;
    if (typeof verifiedAs !== "string" || verifiedAs.toLowerCase() !== value.toLowerCase() || typeof connectionId !== "string") {
      return { error: "Connect this account by signing in to it, then it can be refreshed." };
    }
    const knowledge = await readThroughConnection(platform, session.id, connectionId, value);
    await storeKnowledge(session.id, knowledge, true);
    await c
      .from("candidate_users")
      .update({ links: { ...links, [platform]: { ...links[platform], stats: statsOf(knowledge) } } })
      .eq("id", session.id);
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't read that account." };
  }

  revalidatePath("/profile");
  return { ok: true };
}

export async function removeLink(platform: string) {
  const session = await currentCandidate();
  if (!session) return { error: "Not signed in" };

  const c = db();
  if (!c) return { error: "Database not connected" };

  const { data } = await c.from("candidate_users").select("links").eq("id", session.id).single();
  const currentLinks = data?.links || {};
  // Removing a connected account ends the connection too: the access the
  // candidate gave goes when the account does.
  const connectionId = currentLinks[platform]?.connectionId;
  if (typeof connectionId === "string" && connectionId) await endConnection(connectionId);
  delete currentLinks[platform];

  const { error } = await c
    .from("candidate_users")
    .update({ links: currentLinks })
    .eq("id", session.id);

  if (error) return { error: error.message };
  if (isKnowledgeSource(platform)) await dropKnowledge(session.id, platform);
  if (platform === "linkedin") await dropLinkedIn(session.id);

  revalidatePath("/profile");
  return { ok: true };
}

export async function uploadResume(formData: FormData) {
  const session = await currentCandidate();
  if (!session) return { error: "Not signed in" };

  const file = formData.get("file") as File;
  if (!file) return { error: "No file provided" };

  const c = db();
  if (!c) return { error: "Database not connected" };

  // Remove old resume if it exists
  const { data: userData } = await c.from("candidate_users").select("resume_path").eq("id", session.id).single();
  if (userData?.resume_path) {
    await c.storage.from("resumes").remove([userData.resume_path]);
  }

  const path = `${session.id}/${Date.now()}-${file.name.replace(/[^a-zA-Z0-9.-]/g, "_")}`;
  
  const { error: uploadError } = await c.storage
    .from("resumes")
    .upload(path, file, { contentType: file.type });
    
  if (uploadError) return { error: uploadError.message };

  // The page extracts the text (lib/profile/resumeParse.ts) and sends it
  // with the file; the fields are read out of it here, so what is stored is
  // this code's reading of the text and not something the page asserted. No
  // text — a .doc, a scanned PDF, an extraction that failed — clears what an
  // earlier resume left behind rather than leaving it to describe this one.
  const textRaw = formData.get("text");
  const text = typeof textRaw === "string" ? textRaw.replace(/\u0000/g, "").slice(0, MAX_RESUME_TEXT).trim() : "";
  const parsed: ResumeFields | null = text ? parseResumeText(text) : null;

  const { error: dbError } = await c
    .from("candidate_users")
    .update({
      resume_path: path,
      resume_text: text || null,
      resume_parsed: parsed,
      resume_parsed_at: parsed ? new Date().toISOString() : null,
    })
    .eq("id", session.id);

  if (dbError) return { error: dbError.message };

  revalidatePath("/profile");
  return { ok: true, path, parsed };
}

/**
 * Copies what was read from the resume into the profile — only into fields
 * the candidate has left empty, and only when they ask. Nothing they wrote
 * is replaced.
 */
export async function fillProfileFromResume() {
  const session = await currentCandidate();
  if (!session) return { error: "Not signed in" };

  const c = db();
  if (!c) return { error: "Database not connected" };

  const { data } = await c
    .from("candidate_users")
    .select("role, location, bio, links, resume_parsed")
    .eq("id", session.id)
    .single();
  const parsed = (data?.resume_parsed ?? null) as ResumeFields | null;
  if (!parsed) return { error: "Nothing has been read from a resume yet." };

  const update: Record<string, unknown> = {};
  const filled: string[] = [];
  if (!data?.role && parsed.role) { update.role = parsed.role; filled.push("headline"); }
  if (!data?.location && parsed.location) { update.location = parsed.location; filled.push("location"); }
  if (!data?.bio && parsed.bio) { update.bio = parsed.bio; filled.push("about you"); }
  if (filled.length === 0) return { ok: true, filled };

  const { error } = await c
    .from("candidate_users")
    .update({ ...update, profile_updated_at: new Date().toISOString() })
    .eq("id", session.id);
  if (error) return { error: error.message };

  revalidatePath("/profile");
  revalidatePath("/dashboard");
  return { ok: true, filled };
}

export async function removeResume() {
  const session = await currentCandidate();
  if (!session) return { error: "Not signed in" };

  const c = db();
  if (!c) return { error: "Database not connected" };

  const { data: userData } = await c.from("candidate_users").select("resume_path").eq("id", session.id).single();
  if (userData?.resume_path) {
    await c.storage.from("resumes").remove([userData.resume_path]);
  }

  const { error } = await c
    .from("candidate_users")
    .update({ resume_path: null, resume_text: null, resume_parsed: null, resume_parsed_at: null })
    .eq("id", session.id);

  if (error) return { error: error.message };

  revalidatePath("/profile");
  return { ok: true };
}
