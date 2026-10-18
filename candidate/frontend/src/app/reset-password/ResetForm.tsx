"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";
import clsx from "clsx";
import { authInput, authSubmit } from "@/components/login/AuthShell";
import { display } from "@/lib/dashboard/fonts";
import { choosePassword, type ResetState } from "./actions";

const inputCls = authInput;

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button type="submit" className={authSubmit} disabled={pending}>
      {pending ? "Saving…" : "Set new password"}
    </button>
  );
}

export function ResetForm({ token, minLength }: { token: string; minLength: number }) {
  const [state, action] = useActionState<ResetState, FormData>(choosePassword, { error: null });

  return (
    <div className="w-full">
      <h1 className={clsx(display.className, "mb-6 text-center text-[30px] leading-tight font-semibold tracking-tight text-[#100E2E]")}>Choose a new password</h1>
      <form action={action} className="space-y-4">
        <input type="hidden" name="token" value={token} />
        <input name="password" type="password" required minLength={minLength} autoComplete="new-password" autoFocus placeholder={`New password (${minLength}+ characters)`} className={inputCls} />
        <input name="confirm" type="password" required minLength={minLength} autoComplete="new-password" placeholder="Confirm new password" className={inputCls} />
        {state.error && (
          <p role="alert" className="text-center text-[13.5px] text-[#a6203c]">
            {state.error}
          </p>
        )}
        <Submit />
      </form>
    </div>
  );
}
