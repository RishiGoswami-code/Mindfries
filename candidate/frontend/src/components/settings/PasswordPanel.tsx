"use client";

import { useActionState, useState, useTransition } from "react";
import { KeyRound } from "lucide-react";
import { changePassword, sendPasswordLink, type PasswordState } from "@/lib/settings/actions";
import { SettingsRow, buttonClass } from "./Card";

const field =
  "h-11 w-full rounded-xl border border-[#E4E0F2] bg-white px-3.5 text-[14.5px] text-[#1B1840] outline-none placeholder:text-[#A9A5C2] focus:border-[#B9A8F5]";

/**
 * The password row. With a password: change it, given the current one. An
 * account made through Google, GitHub, GitLab or LinkedIn has none, and can
 * only get one by email — a link to the address on the account — since the
 * person at this keyboard hasn't otherwise shown they own that address.
 */
export function PasswordPanel({ hasPassword, email }: { hasPassword: boolean; email: string }) {
  const [open, setOpen] = useState(false);
  const [state, action, pending] = useActionState<PasswordState, FormData>(changePassword, { error: null, notice: null });
  const [linkState, setLinkState] = useState<{ ok: boolean; text: string } | null>(null);
  const [sending, start] = useTransition();

  if (!hasPassword) {
    return (
      <div>
        <SettingsRow icon={<KeyRound size={19} />} label="Password" detail="Not set — you sign in with a linked account. A password lets you sign in with your email too.">
          <button
            type="button"
            className={buttonClass.quiet}
            disabled={sending}
            onClick={() =>
              start(async () => {
                const r = await sendPasswordLink();
                setLinkState(r.ok ? { ok: true, text: `We've emailed ${email} a link to set one. It works for an hour.` } : { ok: false, text: r.error });
              })
            }
          >
            {sending ? "Sending…" : "Email me a link"}
          </button>
        </SettingsRow>
        {linkState && <p className={linkState.ok ? "pb-2 pl-[54px] text-[13px] text-[#14693a]" : "pb-2 pl-[54px] text-[13px] text-[#a6203c]"}>{linkState.text}</p>}
      </div>
    );
  }

  return (
    <div>
      <SettingsRow icon={<KeyRound size={19} />} label="Password" detail="Changing it signs you out everywhere else.">
        <button type="button" className={buttonClass.quiet} onClick={() => setOpen((v) => !v)} aria-expanded={open}>
          {open ? "Cancel" : "Change password"}
        </button>
      </SettingsRow>
      {state.notice && !open && <p className="pb-2 pl-[54px] text-[13px] text-[#14693a]">{state.notice}</p>}
      {open && (
        <form
          action={action}
          className="mb-3 ml-0 grid gap-3 rounded-2xl bg-[#FAF9FE] p-4 sm:ml-[54px] sm:grid-cols-3"
        >
          <input name="current" type="password" autoComplete="current-password" required placeholder="Current password" aria-label="Current password" className={field} />
          <input name="next" type="password" autoComplete="new-password" required minLength={8} placeholder="New password (8+ characters)" aria-label="New password" className={field} />
          <input name="confirm" type="password" autoComplete="new-password" required minLength={8} placeholder="Confirm new password" aria-label="Confirm new password" className={field} />
          <div className="flex flex-wrap items-center gap-3 sm:col-span-3">
            <button type="submit" disabled={pending} className={buttonClass.primary}>
              {pending ? "Saving…" : "Save new password"}
            </button>
            {state.error && <p role="alert" className="text-[13px] text-[#a6203c]">{state.error}</p>}
            {state.notice && <p className="text-[13px] text-[#14693a]">{state.notice}</p>}
            <a href="/forgot-password" className="ml-auto text-[13px] text-[#5B3BE0] hover:underline">
              Forgot it?
            </a>
          </div>
        </form>
      )}
    </div>
  );
}
