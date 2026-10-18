import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import clsx from "clsx";
import { AuthShell, authLink, authSecondary, authSubmit } from "@/components/login/AuthShell";
import { display } from "@/lib/dashboard/fonts";
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
    <AuthShell width="md">
      <p className="flex items-center gap-3 text-[12.5px] font-medium tracking-[0.24em] text-[#6E6A8F] uppercase">
        Assessment invitation <span className="h-px w-8 bg-[#C9BCF7]" />
      </p>
      <h1 className={clsx(display.className, "mt-3 text-[32px] leading-tight font-semibold tracking-tight text-[#100E2E] [overflow-wrap:anywhere]")}>{invite.role}</h1>
      <p className="mt-2 text-[15px] leading-relaxed text-[#2A2654]">
        {invite.company} invited {here && invite.candidateName ? invite.candidateName.split(/\s+/)[0] : "you"} to a technical assessment
        {invite.dueDate ? `, to be done by ${invite.dueDate}` : ""}.
      </p>

      {!candidate && (
        <section className="mt-6 space-y-4">
          <p className="text-[14.5px] leading-relaxed text-[#5E5A82]">
            This invitation is for <span className="font-semibold text-[#100E2E]">{maskEmail(invite.candidateEmail)}</span>. Sign in, or create an
            account, with that address to see it.
          </p>
          <div className="flex flex-col gap-2.5 sm:flex-row">
            <Link href={`/login?next=${next}`} className={authSubmit}>
              Sign in
            </Link>
            <Link href={`/signup?next=${next}`} className={authSecondary}>
              Create account
            </Link>
          </div>
        </section>
      )}

      {candidate && !here && (
        <section className="mt-6 space-y-4">
          <p className="text-[14.5px] leading-relaxed text-[#5E5A82]">
            You&apos;re signed in as <span className="font-semibold text-[#100E2E] [overflow-wrap:anywhere]">{candidate.email}</span>, but this invitation is for{" "}
            <span className="font-semibold text-[#100E2E]">{maskEmail(invite.candidateEmail)}</span>. Only that address can take it.
          </p>
          <form action={switchAccount.bind(null, token)}>
            <button type="submit" className={authSubmit}>
              Sign out and use the invited address
            </button>
          </form>
        </section>
      )}

      {here && (
        <section className="mt-6">
          {invite.declined || invite.status === "declined" ? (
            <p className="rounded-2xl bg-[#F6F4FC] px-4 py-3 text-[14.5px] text-[#2A2654]">You declined this invitation, so it can&apos;t be started.</p>
          ) : invite.status === "in_progress" && sessionId ? (
            <Link href={`/ide?session=${encodeURIComponent(sessionId)}`} className={authSubmit}>
              Resume your assessment
            </Link>
          ) : invite.status !== "invited" ? (
            <p className="rounded-2xl bg-[#EAFAF6] px-4 py-3 text-[14.5px] text-[#1E7A5A]">
              You&apos;ve already submitted this assessment.{" "}
              {sessionId && (
                <Link href={`/assessments/${encodeURIComponent(sessionId)}/report`} className="font-semibold underline underline-offset-4">
                  View submission status
                </Link>
              )}
            </p>
          ) : (
            <InviteActions token={token} taskStatus={invite.taskStatus} />
          )}
          <p className="mt-5 text-[13px] leading-relaxed text-[#6E6A8F]">
            Nothing starts, and no clock runs, until you choose to begin. During the assessment your camera is on, and the hiring team sees how you
            worked — commands, changes, tests and questions to the AI assistant.
          </p>
        </section>
      )}

      <p className="mt-6 border-t border-[#ECE9F6] pt-5 text-[13px] leading-relaxed text-[#6E6A8F]">
        Assessments on Mindfries are by invitation only.{" "}
        <Link href="/dashboard" className={authLink}>
          Your dashboard
        </Link>{" "}
        lists every invitation sent to you.
      </p>
    </AuthShell>
  );
}
