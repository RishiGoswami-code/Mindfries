"use server";

import { revalidatePath } from "next/cache";
import { requireAdminRole } from "@/lib/auth/admins";
import { companyContact, declineCompanyAccess, grantCompanyCredits } from "@/lib/company-access";
import { mailerReady, sendMail } from "@/lib/mailer";
import type { Plan } from "@/lib/types";

type Result = { ok: true; message: string } | { ok: false; error: string };

const PLANS: Plan[] = ["trial", "starter", "growth", "enterprise"];
const MAX_GRANT = 10_000;

const errMsg = (e: unknown): string => {
  if (e instanceof Error) return e.message;
  if (e && typeof e === "object" && "message" in e && typeof e.message === "string") return e.message;
  return String(e);
};

/**
 * Switches hiring on for a self-signed-up company and adds credits (1
 * credit = 1 candidate invited). Also tops up a company that already has
 * access. Logged with the admin's email.
 */
export async function grantAccess(input: { companyId: string; credits: number; plan: string; note: string; notify: boolean }): Promise<Result> {
  try {
    const admin = await requireAdminRole();
    const credits = Math.floor(Number(input.credits));
    if (!Number.isFinite(credits) || credits < 1) return { ok: false, error: "Add at least 1 credit." };
    if (credits > MAX_GRANT) return { ok: false, error: `That's more than ${MAX_GRANT.toLocaleString()} credits at once — check the number.` };
    const plan = (PLANS as string[]).includes(input.plan) ? (input.plan as Plan) : "";
    const note = input.note.slice(0, 1000).trim();

    const balance = await grantCompanyCredits({ companyId: input.companyId, amount: credits, plan, note, by: admin.email });

    let mail = "";
    if (input.notify) {
      const contact = await companyContact(input.companyId);
      const portal = process.env.COMPANY_PORTAL_URL?.replace(/\/+$/, "");
      if (!contact) mail = " No email sent: the company has no admin to send it to.";
      else if (!mailerReady()) mail = " No email sent: email isn't set up here.";
      else {
        try {
          await sendMail({
            to: contact.email,
            subject: `${contact.company} can start hiring on Mindfries`,
            text: `Hi ${contact.name.split(/\s+/)[0] || "there"},\n\nThe Mindfries team has switched on hiring for ${contact.company} and added ${credits} credit${credits === 1 ? "" : "s"} (${balance} available). Each candidate you invite uses 1 credit.\n\n${portal ? `Sign in: ${portal}/login\n\n` : ""}— The Mindfries team`,
          });
          mail = ` Emailed ${contact.email}.`;
        } catch (e) {
          mail = ` The email to ${contact.email} couldn't be sent (${errMsg(e)}).`;
        }
      }
    }

    revalidatePath("/admin/access");
    revalidatePath("/admin/companies");
    return { ok: true, message: `Added ${credits} credit${credits === 1 ? "" : "s"} — ${balance} available.${mail}` };
  } catch (e) {
    return { ok: false, error: errMsg(e) };
  }
}

/** Leaves a self-signed-up company signed-in-able but without hiring. Reversible: granting access later switches it on. */
export async function declineAccess(companyId: string): Promise<Result> {
  try {
    await requireAdminRole();
    await declineCompanyAccess(companyId);
    revalidatePath("/admin/access");
    return { ok: true, message: "Declined. They can still sign in, but can't hire." };
  } catch (e) {
    return { ok: false, error: errMsg(e) };
  }
}
