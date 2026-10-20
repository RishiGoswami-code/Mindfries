"use client";

import Link from "next/link";
import { useState, useTransition } from "react";
import { Button } from "@/components/ui/Button";
import { approveSignIn, denySignIn, startGitHub } from "@/app/v2/device/actions";
import { roleLabel } from "@/lib/v2/roles";

export type Choice = { id: string; role: string; company: string; accepted: boolean };

const OS: Record<string, string> = { windows: "Windows", darwin: "macOS", linux: "Linux" };

/**
 * The sign-in page's one interactive part. The candidate compares the code with
 * the one in their terminal, sees which computer asked, picks the assessment,
 * connects GitHub, and confirms or refuses. It says plainly that confirming
 * connects a terminal to an assessment, so nobody can be talked into approving
 * someone else's request.
 */
export function DeviceConfirm({
  userCode,
  device,
  choices,
  github,
  githubResult,
  githubReason,
  name,
}: {
  userCode: string;
  device: { os: string; os_version?: string; arch: string };
  choices: Choice[];
  github: { connected: boolean; login?: string } | null;
  githubResult?: "connected" | "failed";
  githubReason?: string;
  name: string;
}) {
  const [pending, start] = useTransition();
  const [picked, setPicked] = useState<string>(choices.find((c) => c.accepted)?.id ?? choices[0]?.id ?? "");
  const [done, setDone] = useState<"approved" | "denied" | null>(null);
  const [error, setError] = useState<string | null>(githubResult === "failed" ? `GitHub didn't connect${githubReason ? `: ${githubReason}` : ""}.` : null);

  const choice = choices.find((c) => c.id === picked);
  const needsGitHub = !!github && !github.connected;
  const canApprove = !!choice && choice.accepted && !needsGitHub;

  function connect() {
    setError(null);
    start(async () => {
      const r = await startGitHub(userCode);
      if (!r.ok) return setError(r.error);
      window.location.href = r.url;
    });
  }
  function approve() {
    setError(null);
    start(async () => {
      const r = await approveSignIn(userCode, picked);
      if (!r.ok) return setError(r.error);
      setDone("approved");
    });
  }
  function deny() {
    setError(null);
    start(async () => {
      const r = await denySignIn(userCode);
      if (!r.ok) return setError(r.error);
      setDone("denied");
    });
  }

  if (done) {
    return (
      <main className="mx-auto max-w-md px-6 py-24 text-center">
        <h1 className="text-2xl font-extrabold text-[#0A1931]">{done === "approved" ? "Connected" : "Refused"}</h1>
        <p className="mt-3 text-[#4A7FA7]">
          {done === "approved"
            ? "Go back to your terminal. It will carry on by itself, and open this panel so you can share your camera, screen and microphone."
            : "Nothing was connected. If that wasn't you, you can ignore it; the code stops working by itself."}
        </p>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-lg px-6 py-12">
      <div className="text-xs font-semibold uppercase tracking-wide text-[#4A7FA7]">Connect your terminal</div>
      <h1 className="mt-2 text-3xl font-extrabold text-[#0A1931]">Hi {name.split(" ")[0] || "there"}</h1>
      <p className="mt-2 text-sm text-[#355A7D]">
        A terminal on a <strong>{OS[device.os] ?? device.os}</strong> computer
        {device.os_version ? ` (${device.os_version})` : ""} is asking to start one of your assessments. Only continue if you just ran{" "}
        <code>mindfries start</code> yourself.
      </p>

      <div className="mt-6 rounded-2xl border border-[#D6E3F0] bg-white p-6">
        <div className="text-xs font-semibold uppercase tracking-wide text-[#4A7FA7]">The code in your terminal</div>
        <div className="mt-1 text-3xl font-extrabold tracking-[0.2em] text-[#1A3D63]" aria-label={`Code ${userCode}`}>
          {userCode}
        </div>
        <p className="mt-1 text-xs text-[#4A7FA7]">It must match what your terminal shows. If it doesn&apos;t, press Refuse.</p>

        {choices.length === 0 ? (
          <p className="mt-6 text-sm text-[#B4232A]">You don&apos;t have a V2 assessment waiting. Check your dashboard.</p>
        ) : (
          <fieldset className="mt-6">
            <legend className="text-sm font-semibold text-[#0A1931]">Which assessment?</legend>
            <div className="mt-2 space-y-2">
              {choices.map((c) => (
                <label key={c.id} className="flex cursor-pointer items-start gap-3 rounded-xl border border-[#D6E3F0] p-3 hover:bg-[#F6FAFD]">
                  <input type="radio" name="assessment" value={c.id} checked={picked === c.id} onChange={() => setPicked(c.id)} className="mt-1" />
                  <span className="text-sm">
                    <span className="font-semibold text-[#0A1931]">{roleLabel(c.role)}</span>
                    {c.company && <span className="text-[#4A7FA7]"> · {c.company}</span>}
                    {!c.accepted && (
                      <span className="mt-0.5 block text-xs text-[#B4561A]">
                        You haven&apos;t accepted what this one collects yet.{" "}
                        <Link href={`/v2-setup?assessment=${c.id}`} className="font-semibold underline">
                          Read and accept
                        </Link>
                      </span>
                    )}
                  </span>
                </label>
              ))}
            </div>
          </fieldset>
        )}

        {github && (
          <div className="mt-6 rounded-xl border border-[#D6E3F0] bg-[#F6FAFD] p-4 text-sm">
            <div className="font-semibold text-[#0A1931]">GitHub</div>
            {github.connected ? (
              <p className="mt-1 text-[#1F7A3E]">
                Connected{github.login ? ` as ${github.login}` : ""}. Your work will go to a private repository in your account.
              </p>
            ) : (
              <>
                <p className="mt-1 text-[#355A7D]">
                  Your work goes to a private repository in your own GitHub that the hiring team can read. One click, no code to type.
                </p>
                <div className="mt-3">
                  <Button tone="soft" size="sm" onClick={connect} disabled={pending}>
                    Connect GitHub
                  </Button>
                </div>
              </>
            )}
          </div>
        )}

        <p className="mt-6 text-xs text-[#4A7FA7]">
          Confirming connects that terminal to this assessment. It doesn&apos;t give it your password or anything else.
        </p>
        {error && <p className="mt-3 text-sm text-[#B4232A]">{error}</p>}
        <div className="mt-4 flex gap-3">
          <Button onClick={approve} disabled={!canApprove || pending}>
            {pending ? "One moment…" : "Confirm"}
          </Button>
          <Button tone="ghost" onClick={deny} disabled={pending}>
            Refuse
          </Button>
        </div>
      </div>
    </main>
  );
}
