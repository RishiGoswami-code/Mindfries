import clsx from "clsx";
import { CandidateSidebar } from "./CandidateSidebar";
import { DashboardTopBar } from "./DashboardTopBar";
import type { CandidateProfile } from "@/lib/profile/types";
import { ui } from "@/lib/dashboard/fonts";

/**
 * The frame every signed-in candidate page shares: the soft lilac backdrop,
 * the left rail, and the top bar (search, notifications, account).
 *
 * `search={false}` is for a page that carries its own search in its header
 * (Assessments), so there aren't two boxes doing the same thing; the bell
 * and the account menu stay either way.
 */
export function CandidateShell({
  sessionName,
  profile,
  search = true,
  children,
}: {
  sessionName?: string;
  profile?: CandidateProfile | null;
  search?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className={clsx(ui.className, "relative isolate flex min-h-dvh flex-1 bg-[#F3F1FA] text-[#1B1840] max-lg:flex-col")}>
      <div
        aria-hidden
        className="pointer-events-none fixed inset-0 -z-10"
        style={{
          background:
            "radial-gradient(40% 35% at 8% 70%, rgba(214,204,250,0.55), transparent 70%), radial-gradient(35% 30% at 92% 18%, rgba(226,214,252,0.7), transparent 70%), radial-gradient(45% 40% at 55% 0%, rgba(255,255,255,0.9), transparent 70%)",
        }}
      />

      <CandidateSidebar />

      <main className="min-w-0 flex-1 px-4 pt-5 pb-10 sm:px-8 lg:pt-6">
        <DashboardTopBar sessionName={sessionName} profile={profile} search={search} />
        {children}
      </main>
    </div>
  );
}
