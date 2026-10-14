import "server-only";
import { createHmac } from "node:crypto";
import { headers } from "next/headers";
import { db } from "../supabase";
import { sessionSecret } from "./session";

// Slowing down guessing at the admin sign-in (issue #66).
//
// The per-account lock (lib/auth/admins.ts) stops guessing at one account.
// This adds the two things it can't do:
//
//   * a limit per network — one machine trying a few passwords against each
//     of many accounts is refused after ADMIN_LIMIT failures in a window,
//     whichever account it tries next;
//   * attention on the root admin, which is never locked (it is how the
//     operator gets back in). Every failed root sign-in is counted from
//     every network and costs the caller a pause; past ROOT_ALERT_AT in a
//     window the team inbox is told, once per window.
//
// Counts live in the database (candidate_auth_attempts, migrations 0020 and
// 0023) so every instance of this app sees the same numbers. The address
// itself isn't stored — a keyed hash of it is. If the count can't be read
// the sign-in goes ahead: a database hiccup shouldn't lock the operator out.

const ADMIN_LIMIT = 10;
const WINDOW_MINUTES = 15;
const ROOT_ALERT_AT = 5;
/** What each failed root sign-in costs the caller, in ms. */
export const ROOT_FAILURE_PAUSE_MS = 2000;

async function callerKey(): Promise<string | null> {
  const secret = sessionSecret();
  if (!secret) return null;
  const h = await headers();
  // Vercel sets x-forwarded-for itself; the first address is the one it saw.
  const address = (h.get("x-forwarded-for")?.split(",")[0] ?? h.get("x-real-ip") ?? "").trim();
  if (!address) return null;
  return createHmac("sha256", secret).update(`admin-attempt:${address}`).digest("hex").slice(0, 40);
}

const since = () => new Date(Date.now() - WINDOW_MINUTES * 60_000).toISOString();

/** Minutes this network must wait before another admin sign-in, or 0. */
export async function adminAttemptsExceeded(): Promise<number> {
  const c = db();
  const key = await callerKey();
  if (!c || !key) return 0;
  const { data, error } = await c
    .from("candidate_auth_attempts")
    .select("at")
    .eq("ip_hash", key)
    .eq("kind", "admin_login")
    .gte("at", since())
    .order("at", { ascending: false })
    .limit(ADMIN_LIMIT);
  if (error || !data || data.length < ADMIN_LIMIT) return 0;
  const oldest = Date.parse(data[data.length - 1].at as string);
  return Math.max(1, Math.ceil((oldest + WINDOW_MINUTES * 60_000 - Date.now()) / 60_000));
}

/** Counts a failed admin sign-in against this network. */
export async function noteAdminFailure(): Promise<void> {
  const c = db();
  const key = await callerKey();
  if (!c || !key) return;
  await c.from("candidate_auth_attempts").insert({ ip_hash: key, kind: "admin_login" });
}

/**
 * Counts a failed root sign-in, from any network, and reports whether this
 * one crossed the alert threshold — true exactly once per window, so the
 * inbox gets one message, not one per guess.
 */
export async function noteRootFailure(): Promise<{ failures: number; alert: boolean }> {
  const c = db();
  if (!c) return { failures: 0, alert: false };
  const key = (await callerKey()) ?? "unknown";
  await c.from("candidate_auth_attempts").insert({ ip_hash: key, kind: "root_login" });
  const { count } = await c
    .from("candidate_auth_attempts")
    .select("id", { count: "exact", head: true })
    .eq("kind", "root_login")
    .gte("at", since());
  const failures = count ?? 0;
  return { failures, alert: failures === ROOT_ALERT_AT };
}

export const adminWaitMessage = (minutes: number) =>
  `Too many sign-in attempts from this network. Try again in ${minutes} minute${minutes === 1 ? "" : "s"}.`;
