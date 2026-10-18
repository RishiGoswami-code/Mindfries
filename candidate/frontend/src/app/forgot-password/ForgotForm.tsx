"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";
import Link from "next/link";
import clsx from "clsx";
import { MailCheck } from "lucide-react";
import { authInput, authLink, authSubmit } from "@/components/login/AuthShell";
import { display } from "@/lib/dashboard/fonts";
import { forgotPassword, type ForgotState } from "./actions";

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button type="submit" className={authSubmit} disabled={pending}>
      {pending ? "Sending…" : "Send reset link"}
    </button>
  );
}

export function ForgotForm() {
  const [state, action] = useActionState<ForgotState, FormData>(forgotPassword, { done: false, error: null });

  if (state.done) {
    return (
      <div className="w-full text-center">
        <span className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-[#D7F3EC] text-[#1E7A5A]">
          <MailCheck size={26} strokeWidth={1.8} />
        </span>
        <h1 className={clsx(display.className, "mt-5 text-[30px] leading-tight font-semibold tracking-tight text-[#100E2E]")}>Check your inbox</h1>
        <p className="mt-2 text-[14.5px] leading-relaxed text-[#5E5A82]">
          If that address has a Mindfries account, a link to choose a new password is on its way. It works once, for an hour.
        </p>
        <Link href="/login" className={clsx(authLink, "mt-6 inline-block text-[14px]")}>
          Back to sign in
        </Link>
      </div>
    );
  }

  return (
    <div className="w-full">
      <h1 className={clsx(display.className, "text-center text-[30px] leading-tight font-semibold tracking-tight text-[#100E2E]")}>Forgot your password?</h1>
      <p className="mt-2 mb-6 text-center text-[14.5px] leading-relaxed text-[#5E5A82]">Enter your email and we&apos;ll send a link to choose a new one.</p>
      <form action={action} className="space-y-4">
        <input
          name="email"
          type="email"
          required
          autoComplete="username"
          autoFocus
          placeholder="you@company.com"
          className={authInput}
        />
        {state.error && (
          <p role="alert" className="text-center text-[13.5px] text-[#a6203c]">
            {state.error}
          </p>
        )}
        <Submit />
        <p className="text-center text-[13.5px]">
          <Link href="/login" className={authLink}>
            Back to sign in
          </Link>
        </p>
      </form>
    </div>
  );
}
