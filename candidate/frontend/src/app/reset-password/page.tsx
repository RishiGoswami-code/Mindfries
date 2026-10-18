import type { Metadata } from "next";
import Link from "next/link";
import clsx from "clsx";
import { Link2Off as LinkIcon } from "lucide-react";
import { AuthShell, authSubmit } from "@/components/login/AuthShell";
import { display } from "@/lib/dashboard/fonts";
import { MIN_PASSWORD_LENGTH, resetLinkIsLive } from "@/lib/auth/email-links";
import { ResetForm } from "./ResetForm";

export const metadata: Metadata = {
  title: "Choose a new password · Mindfries",
  // The address carries a working token; it shouldn't travel in a Referer.
  referrer: "no-referrer",
  robots: { index: false },
};

export const dynamic = "force-dynamic";

/**
 * Where a reset link lands. Opening it only looks the token up — it is
 * spent when the form is submitted, so a mail scanner that fetches the link
 * ahead of the person doesn't use it up.
 */
export default async function ResetPasswordPage({ searchParams }: { searchParams: Promise<{ token?: string }> }) {
  const { token } = await searchParams;
  const live = typeof token === "string" && (await resetLinkIsLive(token));

  return (
    <AuthShell>
      {live ? (
        <ResetForm token={token as string} minLength={MIN_PASSWORD_LENGTH} />
      ) : (
        <div className="w-full text-center">
          <span className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-[#FDE8EC] text-[#DC2643]">
            <LinkIcon size={24} strokeWidth={1.8} />
          </span>
          <h1 className={clsx(display.className, "mt-5 text-[30px] leading-tight font-semibold tracking-tight text-[#100E2E]")}>This link no longer works</h1>
          <p className="mt-2 text-[14.5px] leading-relaxed text-[#5E5A82]">
            A reset link works once, for an hour. Ask for a new one and use it straight away.
          </p>
          <Link href="/forgot-password" className={clsx(authSubmit, "mt-6")}>
            Send a new link
          </Link>
        </div>
      )}
    </AuthShell>
  );
}
