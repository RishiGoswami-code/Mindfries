"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";
import Link from "next/link";
import { authField, authLink, authSubmit } from "@/components/login/AuthSplit";
import { signUp, type SignUpState } from "./actions";

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button type="submit" className={authSubmit} disabled={pending}>
      {pending ? "Creating account…" : "Create account"}
    </button>
  );
}

const label = "flex flex-col gap-1.5 text-[13px] font-medium text-[#2A2654]";

export function SignupForm({ minPassword }: { minPassword: number }) {
  const [state, action] = useActionState<SignUpState, FormData>(signUp, { error: null });

  return (
    <form action={action} className="space-y-4">
      <label className={label}>
        Company name
        <input name="companyName" required maxLength={200} autoComplete="organization" placeholder="Your company" className={authField} />
      </label>
      <div className="grid gap-4 sm:grid-cols-2">
        <label className={label}>
          Your name
          <input name="name" required maxLength={200} autoComplete="name" placeholder="Full name" className={authField} />
        </label>
        <label className={label}>
          Company website <span className="sr-only">(optional)</span>
          <input name="website" maxLength={300} autoComplete="url" placeholder="company.com" className={authField} />
        </label>
      </div>
      <label className={label}>
        Work email
        <input name="email" type="email" required autoComplete="username" placeholder="you@company.com" className={authField} />
      </label>
      <label className={label}>
        Password
        <input
          name="password"
          type="password"
          required
          minLength={minPassword}
          autoComplete="new-password"
          placeholder={`At least ${minPassword} characters`}
          className={authField}
        />
      </label>

      <p className="rounded-2xl bg-[#F6F4FC] px-4 py-3 text-[13.5px] leading-relaxed text-[#4C4874]">
        New accounts start with <strong className="text-ink">0 credits</strong>. You can sign in and look around straight away; creating roles and
        inviting candidates unlocks once the Mindfries team grants access.
      </p>

      {state.error && (
        <p role="alert" className="rounded-xl bg-[#fdecef] px-3 py-2.5 text-center text-[13px] leading-relaxed text-[#a6203c]">
          {state.error}
        </p>
      )}

      <Submit />

      <p className="text-center text-[14px] text-dim">
        Already have an account?{" "}
        <Link href="/login" className={authLink}>
          Sign in
        </Link>
      </p>
    </form>
  );
}
