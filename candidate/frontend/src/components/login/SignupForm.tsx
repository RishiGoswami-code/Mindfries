"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";
import Link from "next/link";
import { authInput, authLink, authSubmit } from "./AuthShell";
import { signUp, type SignUpState } from "@/app/signup/actions";

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button type="submit" className={authSubmit} disabled={pending}>
      {pending ? "Creating account…" : "Create account"}
    </button>
  );
}

/** Real — createAccount hashes the password and inserts a row in candidate_users, then signs the new account straight in. See lib/auth/users.ts. */
export function SignupForm({ next = "" }: { next?: string }) {
  const [state, action] = useActionState<SignUpState, FormData>(signUp, { error: null });

  return (
    <div className="w-full">
      <form action={action} className="space-y-4">
        {/* Where to go once the account exists — an invitation's own page,
            usually. The action only follows a path on this site. */}
        {next && <input type="hidden" name="next" value={next} />}
        <input
          name="name"
          type="text"
          required
          autoComplete="name"
          autoFocus
          placeholder="Full name"
          className={authInput}
        />
        <input
          name="email"
          type="email"
          required
          autoComplete="username"
          placeholder="you@company.com"
          className={authInput}
        />
        <input
          name="password"
          type="password"
          required
          minLength={8}
          autoComplete="new-password"
          placeholder="Password (at least 8 characters)"
          className={authInput}
        />
        <input
          name="confirm"
          type="password"
          required
          minLength={8}
          autoComplete="new-password"
          placeholder="Confirm password"
          className={authInput}
        />

        {state.error && (
          <p role="alert" className="text-center text-[13.5px] text-[#a6203c]">
            {state.error}
          </p>
        )}

        <Submit />

        <p className="text-center text-[13.5px] text-[#5E5A82]">
          Already have an account?{" "}
          <Link href={next ? `/login?next=${encodeURIComponent(next)}` : "/login"} className={authLink}>
            Sign in
          </Link>
        </p>
      </form>
    </div>
  );
}
