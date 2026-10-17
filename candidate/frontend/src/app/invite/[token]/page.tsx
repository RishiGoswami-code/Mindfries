import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { LoginBlobs } from "@/components/login/LoginBlobs";
import { currentCandidate } from "@/lib/auth/users";
import { findInviteByToken, maskEmail, sessionForInvite } from "@/lib/invite-link";
import { InviteActions } from "./InviteActions";
import { switchAccount } from "./actions";

export const metadata: Metadata = { title: "Your invitation · Mindfries" };

/**
 * An invitation's own page — the link a company or Mindfries sends.
 *
 * It is public (the middleware doesn't guard it), because the person
 * opening it may not have an account yet. What it shows depends on who is
 * looking:
 *
 *   not signed in            who invited them and for what, and the two ways
 *                            in: sign in, or create an account, as the
 *                            invited address (shown masked)
 *   signed in as someone     that the invitation is for another address, and
 *   else                     a way to switch accounts
 *   signed in as the         the assessment: start it, or follow it while
 *   invited address          its task is still being made, or resume it
 *
 * An unknown or malformed token is a plain not-found, the same as any other
 * address that isn't anything.
 */
export default async function InvitePage({ params }: { params: Promise<{ token: string }> }) {
  const { token } = await params;
  const invite = await findInviteByToken(token);
  if (!invite) notFound();

  const candidate = await currentCandidate();
  const next = encodeURIComponent(`/invite/${token}`);
  const here = candidate && candidate.email.toLowerCase() === invite.candidateEmail;
  const sessionId = here && invite.status !== "invited" ? await sessionForInvite(invite.id, candidate.id) : null;

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#F6FAFD] px-4 py-16">
      <LoginBlobs />
      <main className="relative z-10 w-full max-w-md rounded-3xl border border-[#B3CFE5] bg-white/90 p-7 shadow-[0_20px_45px_-16px_rgba(10,25,49,0.25)]">
        <div className="mb-6 flex items-center gap-2.5">
          {/* eslint-disable-next-line @next/next/no-img-element -- tiny static local SVG */}
          <img src="/mindfries-logo.svg" alt="" width={28} height={28} />
          <span className="text-[17px] font-semibold tracking-tight text-[#0A1931]">Mindfries</span>
        </div>

        <p className="text-[12px] font-semibold tracking-[0.12em] text-[#4A7FA7] uppercase">Assessment invitation</p>
        <h1 className="mt-2 text-[24px] leading-tight font-semibold tracking-tight text-[#0A1931] [overflow-wrap:anywhere]">{invite.role}</h1>
        <p className="mt-1 text-[14px] text-[#1A3D63]">
          {invite.company} invited {here && invite.candidateName ? invite.candidateName.split(/\s+/)[0] : "you"} to a technical assessment
          {invite.dueDate ? `, to be done by ${invite.dueDate}` : ""}.
        </p>

        {!candidate && (
          <section className="mt-6 space-y-3">
            <p className="text-[13.5px] leading-relaxed text-[#1A3D63]">
              This invitation is for <span className="font-semibold">{maskEmail(invite.candidateEmail)}</span>. Sign in, or create an account, with that
              address to see it.
            </p>
            <div className="flex gap-2">
              <Link
                href={`/login?next=${next}`}
                className="flex-1 rounded-xl bg-[#0A1931] px-4 py-2.5 text-center text-[13px] font-semibold text-white hover:opacity-90"
              >
                Sign in
              </Link>
              <Link
                href={`/signup?next=${next}`}
                className="flex-1 rounded-xl border border-[#B3CFE5] px-4 py-2.5 text-center text-[13px] font-semibold text-[#1A3D63] hover:bg-[#B3CFE5]/20"
              >
                Create account
              </Link>
            </div>
          </section>
        )}

        {candidate && !here && (
          <section className="mt-6 space-y-3">
            <p className="text-[13.5px] leading-relaxed text-[#1A3D63]">
              You&apos;re signed in as <span className="font-semibold [overflow-wrap:anywhere]">{candidate.email}</span>, but this invitation is for{" "}
              <span className="font-semibold">{maskEmail(invite.candidateEmail)}</span>. Only that address can take it.
            </p>
            <form action={switchAccount.bind(null, token)}>
              <button type="submit" className="w-full rounded-xl bg-[#0A1931] px-4 py-2.5 text-[13px] font-semibold text-white hover:opacity-90">
                Sign out and use the invited address
              </button>
            </form>
          </section>
        )}

        {here && (
          <section className="mt-6">
            {invite.declined || invite.status === "declined" ? (
              <p className="text-[13.5px] text-[#1A3D63]">You declined this invitation, so it can&apos;t be started.</p>
            ) : invite.status === "in_progress" && sessionId ? (
              <Link
                href={`/ide?session=${encodeURIComponent(sessionId)}`}
                className="block rounded-xl bg-[#0A1931] px-4 py-2.5 text-center text-[13px] font-semibold text-white hover:opacity-90"
              >
                Resume your assessment
              </Link>
            ) : invite.status !== "invited" ? (
              <p className="text-[13.5px] text-[#1A3D63]">
                You&apos;ve already submitted this assessment.{" "}
                {sessionId && (
                  <Link href={`/assessments/${encodeURIComponent(sessionId)}/report`} className="font-semibold underline">
                    See your report
                  </Link>
                )}
              </p>
            ) : (
              <InviteActions token={token} taskStatus={invite.taskStatus} />
            )}
            <p className="mt-4 text-[12px] leading-relaxed text-[#4A7FA7]">
              Nothing starts, and no clock runs, until you choose to begin. During the assessment your camera is on, and the hiring team sees how you
              worked — commands, changes, tests and questions to the AI assistant.
            </p>
          </section>
        )}

        <p className="mt-6 border-t border-[#B3CFE5]/60 pt-4 text-[12px] text-[#4A7FA7]">
          Assessments on Mindfries are by invitation only. <Link href="/dashboard" className="font-medium underline">Your dashboard</Link> lists every
          invitation sent to you.
        </p>
      </main>
    </div>
  );
}
