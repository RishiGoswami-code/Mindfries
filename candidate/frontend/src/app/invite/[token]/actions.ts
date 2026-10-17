"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { db } from "@/lib/supabase";
import { currentCandidate } from "@/lib/auth/users";
import { SESSION_COOKIE } from "@/lib/auth/session";
import { findInviteByToken } from "@/lib/invite-link";

// The two things the invitation page can do. Each looks the invitation up
// again from the token and the signed-in account — nothing the page sent is
// trusted about whose it is.

/**
 * Begin: the invitation is accepted (opening the link and choosing to start
 * is the candidate's answer), tied to this account, and the candidate goes
 * into the usual consent → device check → lobby steps.
 */
export async function beginFromInvite(token: string): Promise<{ error: string } | never> {
  const invite = await findInviteByToken(token);
  const candidate = await currentCandidate();
  if (!invite) return { error: "This invitation link doesn't work any more." };
  if (!candidate) redirect(`/login?next=${encodeURIComponent(`/invite/${token}`)}`);
  if (candidate.email.toLowerCase() !== invite.candidateEmail) return { error: "This invitation is for a different email address." };
  if (invite.declined || invite.status !== "invited") return { error: "This invitation is no longer open." };
  if (invite.taskStatus !== "ready") return { error: "Your assessment is still being prepared. This page will update when it's ready." };

  const c = db();
  if (c) {
    await Promise.all([
      c.from("assessments").update({ accepted_at: new Date().toISOString() }).eq("id", invite.id).is("accepted_at", null),
      c.from("assessments").update({ candidate_id: candidate.id }).eq("id", invite.id).is("candidate_id", null),
    ]);
  }
  redirect(`/onboarding?template=${encodeURIComponent(invite.id)}`);
}

/** Signed in as someone else: sign out here and come back to sign in as the invited address. */
export async function switchAccount(token: string): Promise<never> {
  const jar = await cookies();
  jar.delete(SESSION_COOKIE);
  redirect(`/login?next=${encodeURIComponent(`/invite/${token}`)}`);
}
