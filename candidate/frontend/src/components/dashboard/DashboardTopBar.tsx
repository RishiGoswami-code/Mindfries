import { Search } from "lucide-react";
import { NotificationBell } from "./NotificationBell";
import { AccountMenu } from "./AccountMenu";
import type { CandidateProfile } from "@/lib/profile/types";

/**
 * Search, notifications and the account menu, above the dashboard.
 *
 * The search is a plain GET form to /assessments?q=…, which filters the
 * candidate's own assessments by role and company — a real search over the
 * only thing there is to search, and one that works without JavaScript.
 */
export function DashboardTopBar({ sessionName, profile }: { sessionName?: string; profile?: CandidateProfile | null }) {
  return (
    <div className="flex items-center gap-4">
      <form action="/assessments" method="get" role="search" className="min-w-0 flex-1 lg:max-w-[634px]">
        <label className="flex h-[46px] items-center gap-4 rounded-2xl border border-white/80 bg-white/55 px-5 shadow-[0_1px_2px_rgba(27,24,64,0.04)] backdrop-blur focus-within:border-[#C9BCF7] focus-within:bg-white/80">
          <Search size={20} strokeWidth={1.9} className="shrink-0 text-[#1B1840]" />
          <span className="sr-only">Search your assessments</span>
          <input
            type="search"
            name="q"
            maxLength={100}
            placeholder="Search assessments…"
            className="min-w-0 flex-1 bg-transparent text-[13px] text-[#1B1840] outline-none placeholder:text-[#8A86A8]"
          />
        </label>
      </form>

      <div className="ml-auto flex shrink-0 items-center gap-5">
        <NotificationBell appearance="large" />
        <AccountMenu sessionName={sessionName} profile={profile} appearance="large" />
      </div>
    </div>
  );
}
