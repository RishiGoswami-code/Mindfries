"use server";

import {
  getConsent,
  getNotice,
  mintSetupCode,
  saveConsent,
  V2Error,
  type Notice,
  type SetupCommand,
} from "@/lib/v2/api";

import { ownedAssessment, type Failure } from "@/lib/v2/owned";

const message = (e: unknown) => (e instanceof V2Error ? e.message : "Something went wrong. Please try again.");

/** What this assessment collects, and whether the candidate has already accepted it. */
export async function loadNotice(assessmentId: string): Promise<{ ok: true; notice: Notice; accepted: boolean } | Failure> {
  try {
    const owned = await ownedAssessment(assessmentId);
    if (!owned.ok) return owned;
    const [notice, consent] = await Promise.all([getNotice(assessmentId), getConsent(owned.a.candidateId, assessmentId)]);
    return { ok: true, notice, accepted: consent.accepted };
  } catch (e) {
    return { ok: false, error: message(e) };
  }
}

/** The candidate read what is collected and accepted all of it. */
export async function acceptNotice(assessmentId: string, version: string): Promise<{ ok: true } | Failure> {
  try {
    const owned = await ownedAssessment(assessmentId);
    if (!owned.ok) return owned;
    // What they accept is exactly what the notice lists as collected, taken
    // from the service again here, so a page can't claim less.
    const notice = await getNotice(assessmentId);
    if (notice.version !== version) return { ok: false, error: "The notice changed while you were reading it. Please read it again." };
    await saveConsent(owned.a.candidateId, assessmentId, notice.version, notice.items.filter((i) => i.collected).map((i) => i.id));
    return { ok: true };
  } catch (e) {
    return { ok: false, error: message(e) };
  }
}

/**
 * The Start assessment button. It makes a one-time command for THIS
 * candidate's assessment, tied to them and to the assessment so the session
 * the CLI gets is theirs, and running it again resumes the same session.
 */
export async function startV2Session(assessmentId: string): Promise<{ ok: true; command: SetupCommand } | Failure> {
  try {
    const owned = await ownedAssessment(assessmentId);
    if (!owned.ok) return owned;
    const consent = await getConsent(owned.a.candidateId, assessmentId);
    if (!consent.accepted) return { ok: false, error: "Please read and accept what is collected first." };
    const a = owned.a;
    const command = await mintSetupCode({
      candidateId: a.candidateId,
      companyId: a.companyId,
      assessmentId,
      role: a.role,
      jd: a.jd,
      title: a.title,
    });
    return { ok: true, command };
  } catch (e) {
    return { ok: false, error: message(e) };
  }
}
