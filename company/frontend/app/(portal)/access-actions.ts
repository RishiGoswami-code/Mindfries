"use server";

import { revalidatePath } from "next/cache";
import { currentCompanyUser } from "@/lib/auth/company-users";
import { getCompanyAccess, markAccessRequested } from "@/lib/access";
import { mailerReady, NOTIFY_EMAIL, sendMail } from "@/lib/mailer";

export type RemindState = { sent: boolean; error: string | null };

/** At most one reminder an hour per company, so the button can't flood the team's inbox. */
const REMIND_EVERY_MS = 60 * 60 * 1000;

/**
 * "Remind the Mindfries team" — from the access banner or the credits
 * dialog. Records the nudge (internal-admin shows it on the request) and
 * emails the team when email is set up.
 */
export async function remindTeam(): Promise<RemindState> {
  const user = await currentCompanyUser();
  if (!user) return { sent: false, error: "Not signed in." };

  const access = await getCompanyAccess(user.companyId);
  if (access.requestedAt && Date.now() - Date.parse(access.requestedAt) < REMIND_EVERY_MS) {
    return { sent: true, error: null };
  }

  await markAccessRequested(user.companyId);
  if (mailerReady()) {
    const outOfCredits = access.status === "active";
    try {
      await sendMail({
        to: NOTIFY_EMAIL,
        subject: outOfCredits ? `${user.companyName} is asking for more credits` : `${user.companyName} is waiting for access`,
        text: `${user.name} <${user.email}> at ${user.companyName} ${outOfCredits ? "is out of credits and asked for more" : "is waiting for access and sent a reminder"}.\n\nGrant credits in internal-admin → Access requests.`,
        replyTo: user.email,
      });
    } catch {
      // Recorded either way; it shows in internal-admin.
    }
  }

  revalidatePath("/", "layout");
  return { sent: true, error: null };
}
