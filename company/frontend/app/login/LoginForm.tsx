"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";
import { authField, authSubmit } from "@/components/login/AuthSplit";
import { signIn, type LoginState } from "./actions";

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button type="submit" className={authSubmit} disabled={pending}>
      {pending ? "Signing in…" : "Sign in"}
    </button>
  );
}

const label = "flex flex-col gap-1.5 text-[13px] font-medium text-[#2A2654]";

export function LoginForm({ next }: { next: string }) {
  const [state, action] = useActionState<LoginState, FormData>(signIn, { error: null });

  return (
    <form action={action} className="space-y-4">
      {next && <input type="hidden" name="next" value={next} />}

      <label className={label}>
        Work email
        <input name="email" type="email" required autoComplete="username" placeholder="you@company.com" autoFocus className={authField} />
      </label>
      <label className={label}>
        Password
        <input name="password" type="password" required autoComplete="current-password" placeholder="Password" className={authField} />
      </label>

      {state.error && (
        <p role="alert" className="rounded-xl bg-[#fdecef] px-3 py-2.5 text-center text-[13px] leading-relaxed text-[#a6203c]">
          {state.error}
        </p>
      )}

      <Submit />
    </form>
  );
}
