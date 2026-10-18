import type { Metadata } from "next";
import clsx from "clsx";
import { CandidateShell } from "@/components/dashboard/CandidateShell";
import { EnvironmentCheck } from "@/components/environment/EnvironmentCheck";
import { currentCandidate } from "@/lib/auth/users";
import { display } from "@/lib/dashboard/fonts";
import { getProfile } from "@/lib/profile/actions";
import { getSetupState } from "@/lib/setup-state";

export const metadata: Metadata = {
  title: "Environment Check · Mindfries",
  description: "Make sure your browser, camera, and microphone are ready for a live session.",
};

export default async function EnvironmentCheckPage() {
  const [session, profile] = await Promise.all([currentCandidate(), getProfile()]);
  const setup = await getSetupState(session?.id);

  return (
    <CandidateShell sessionName={session?.name} profile={profile} search={false}>
      <div className="mx-auto max-w-3xl px-1">
        <header className="mt-6">
          <h1 className={clsx(display.className, "text-[44px] leading-none font-medium tracking-[-0.02em] text-[#100E2E] sm:text-[52px]")}>
            Environment check
          </h1>
          <p className="mt-3 max-w-xl text-[16px] leading-relaxed text-[#5E5A82]">
            Two minutes: camera, microphone, and whether your browser can run the workspace. Do it once before your first real session.
          </p>
        </header>

        <EnvironmentCheck lastPassedAt={setup.environmentCheckedAt} />
      </div>
    </CandidateShell>
  );
}
