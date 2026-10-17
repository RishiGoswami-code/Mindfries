import { NotificationBell } from "./NotificationBell";
import { AccountMenu } from "./AccountMenu";
import { SearchBox } from "./SearchBox";
import type { CandidateProfile } from "@/lib/profile/types";

/**
 * Search, notifications and the account menu, above every candidate page
 * (CandidateShell). The search filters the candidate's own assessments —
 * see SearchBox. A page with its own search in its header passes
 * `search={false}`.
 */
export function DashboardTopBar({
  sessionName,
  profile,
  search = true,
}: {
  sessionName?: string;
  profile?: CandidateProfile | null;
  search?: boolean;
}) {
  return (
    <div className="flex items-center gap-4 print:hidden">
      {search && <SearchBox className="flex-1 lg:max-w-[834px]" />}

      <div className="ml-auto flex shrink-0 items-center gap-5">
        <NotificationBell appearance="large" />
        <AccountMenu sessionName={sessionName} profile={profile} appearance="large" />
      </div>
    </div>
  );
}
