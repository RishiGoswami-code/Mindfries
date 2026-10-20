"use server";

import { headers } from "next/headers";
import { currentCandidate } from "@/lib/auth/users";
import { approveDevice, denyDevice, getConsent, githubConnectUrl, githubStatus, V2Error } from "@/lib/v2/api";
import { ownedAssessment } from "@/lib/v2/owned";
import { v2Enabled } from "@/lib/v2/flag";

type Failure = { ok: false; error: string };

const message = (e: unknown) => (e instanceof V2Error ? e.message : "Something went wrong. Please try again.");

/**
 * The candidate confirms the terminal that is asking: it is connected to THIS
 * assessment, which must be theirs and must have its notice accepted. The code
 * on the page and in the terminal were compared by the candidate; this is the
 * server's side of the same check.
 */
export async function approveSignIn(userCode: string, assessmentId: string): Promise<{ ok: true } | Failure> {
  try {
    const owned = await ownedAssessment(assessmentId);
    if (!owned.ok) return owned;
    const a = owned.a;
    const consent = await getConsent(a.candidateId, assessmentId);
    if (!consent.accepted) return { ok: false, error: "Please read and accept what is collected for this assessment first." };
    const gh = await githubStatus(a.candidateId);
    if (gh && !gh.connected) return { ok: false, error: "Connect your GitHub first." };
    await approveDevice(userCode, {
      candidateId: a.candidateId,
      companyId: a.companyId,
      assessmentId,
      role: a.role,
      jd: a.jd,
      title: a.title,
    });
    return { ok: true };
  } catch (e) {
    return { ok: false, error: message(e) };
  }
}

/** "That isn't me": the terminal gets nothing. */
export async function denySignIn(userCode: string): Promise<{ ok: true } | Failure> {
  try {
    if (!v2Enabled()) return { ok: false, error: "This isn't available yet." };
    if (!(await currentCandidate())) return { ok: false, error: "Please sign in again." };
    await denyDevice(userCode);
    return { ok: true };
  } catch (e) {
    return { ok: false, error: message(e) };
  }
}

/** Where to send the browser to connect GitHub; it comes back to this page. */
export async function startGitHub(userCode: string): Promise<{ ok: true; url: string } | Failure> {
  try {
    if (!v2Enabled()) return { ok: false, error: "This isn't available yet." };
    const candidate = await currentCandidate();
    if (!candidate) return { ok: false, error: "Please sign in again." };
    const h = await headers();
    const host = h.get("x-forwarded-host") ?? h.get("host");
    const proto = h.get("x-forwarded-proto") ?? (host?.startsWith("localhost") ? "http" : "https");
    if (!host) return { ok: false, error: "Couldn't work out this page's address." };
    const returnTo = `${proto}://${host}/v2/device?code=${encodeURIComponent(userCode)}`;
    const { authorize_url } = await githubConnectUrl(candidate.id, returnTo);
    return { ok: true, url: authorize_url };
  } catch (e) {
    return { ok: false, error: message(e) };
  }
}
