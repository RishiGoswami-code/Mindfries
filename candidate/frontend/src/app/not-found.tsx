import type { Metadata } from "next";
import Link from "next/link";
import clsx from "clsx";
import { Compass } from "lucide-react";
import { AuthShell, authSubmit } from "@/components/login/AuthShell";
import { display } from "@/lib/dashboard/fonts";

export const metadata: Metadata = { title: "Not found · Mindfries" };

/**
 * What a candidate sees for an address that isn't theirs or isn't anything:
 * a mistyped link, a session or report id that belongs to someone else (the
 * pages call notFound() rather than say which), a page that was moved.
 *
 * It says the same thing in every one of those cases on purpose — telling a
 * wrong id from someone else's id would say which ids exist.
 */
export default function NotFound() {
  return (
    <AuthShell>
      <div className="text-center">
        <span className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0]">
          <Compass size={26} strokeWidth={1.8} />
        </span>
        <h1 className={clsx(display.className, "mt-5 text-[30px] leading-tight font-semibold tracking-tight text-[#100E2E]")}>We couldn&apos;t find that page</h1>
        <p className="mt-2 text-[14.5px] leading-relaxed text-[#5E5A82]">
          The link may be mistyped, or it points at something that isn&apos;t part of your account. Your assessments and reports are all on your
          dashboard.
        </p>
        <Link href="/dashboard" className={clsx(authSubmit, "mt-6")}>
          Go to your dashboard
        </Link>
      </div>
    </AuthShell>
  );
}
