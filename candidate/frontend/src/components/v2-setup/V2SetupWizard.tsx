"use client";

import Link from "next/link";
import { useEffect, useState, useTransition } from "react";
import { Button } from "@/components/ui/Button";
import { acceptNotice, loadNotice, startV2Session } from "@/app/v2-setup/actions";
import { roleLabel } from "@/lib/v2/roles";

type NoticeItem = { id: string; label: string; description: string; collected: boolean };
type Notice = { version: string; items: NoticeItem[] };
type Command = { setup_code: string; windows_command: string; unix_command: string; cli_command: string; expires_at: string };

function Copyable({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex items-stretch gap-2">
      <code className="flex-1 overflow-x-auto whitespace-nowrap rounded-lg border border-[#D6E3F0] bg-[#F6FAFD] px-3 py-2 text-[13px] text-[#0A1931]">
        {text}
      </code>
      <button
        type="button"
        onClick={() => {
          navigator.clipboard?.writeText(text);
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        }}
        className="shrink-0 rounded-lg border border-[#D6E3F0] px-3 text-[13px] font-semibold text-[#1A3D63] hover:bg-[#E3EDF7]"
      >
        {copied ? "Copied" : "Copy"}
      </button>
    </div>
  );
}

/**
 * Setting up a V2 (local-workspace) assessment: first what is collected, which
 * the candidate reads and accepts (the clock can't start without it), then the
 * one command to run. Running it again later resumes the same session.
 */
export function V2SetupWizard({ assessment }: { assessment: { id: string; role: string; company: string } | null }) {
  const [pending, start] = useTransition();
  const [notice, setNotice] = useState<Notice | null>(null);
  const [accepted, setAccepted] = useState(false);
  const [agree, setAgree] = useState(false);
  const [command, setCommand] = useState<Command | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(!!assessment);

  const id = assessment?.id;
  useEffect(() => {
    if (!id) return;
    let live = true;
    loadNotice(id).then((r) => {
      if (!live) return;
      setLoading(false);
      if (!r.ok) return setError(r.error);
      setNotice(r.notice);
      setAccepted(r.accepted);
    });
    return () => {
      live = false;
    };
  }, [id]);

  if (!assessment) {
    return (
      <main className="mx-auto max-w-xl px-6 py-16 text-center">
        <h1 className="text-2xl font-extrabold text-[#0A1931]">No assessment selected</h1>
        <p className="mt-3 text-[#4A7FA7]">Start a workspace from an assessment on your dashboard.</p>
        <Link href="/dashboard" className="mt-6 inline-block font-semibold text-[#1A3D63] underline">
          Back to dashboard
        </Link>
      </main>
    );
  }

  function accept() {
    if (!notice) return;
    setError(null);
    start(async () => {
      const r = await acceptNotice(assessment!.id, notice.version);
      if (!r.ok) return setError(r.error);
      setAccepted(true);
    });
  }

  function generate() {
    setError(null);
    start(async () => {
      const r = await startV2Session(assessment!.id);
      if (!r.ok) return setError(r.error);
      setCommand(r.command);
    });
  }

  return (
    <main className="mx-auto max-w-2xl px-6 py-12">
      <div className="text-xs font-semibold uppercase tracking-wide text-[#4A7FA7]">Set up your workspace</div>
      <h1 className="mt-2 text-3xl font-extrabold text-[#0A1931]">{roleLabel(assessment.role)}</h1>
      <p className="mt-1 text-[#4A7FA7]">{assessment.company}</p>

      <div className="mt-8 rounded-2xl border border-[#D6E3F0] bg-white p-6">
        <p className="text-sm text-[#0A1931]">
          This assessment runs on your own computer, in your own editor. You&apos;ll install the Mindfries CLI once, set the
          assessment up from your terminal, and start it when you&apos;re ready. Your work is recorded as evidence while you
          code, so first, here is exactly what is and isn&apos;t collected.
        </p>

        {loading && <p className="mt-6 text-sm text-[#4A7FA7]">Loading…</p>}

        {notice && !accepted && (
          <div className="mt-6" aria-label="What is collected">
            <ul className="space-y-3">
              {notice.items.map((i) => (
                <li key={i.id} className="rounded-xl border border-[#D6E3F0] bg-[#F6FAFD] p-4">
                  <div className="flex items-center gap-2 text-sm font-semibold text-[#0A1931]">
                    {i.label}
                    <span
                      className={
                        i.collected
                          ? "rounded-full bg-[#E3EDF7] px-2 py-0.5 text-[11px] font-semibold text-[#1A3D63]"
                          : "rounded-full bg-[#EAF6EE] px-2 py-0.5 text-[11px] font-semibold text-[#1F7A3E]"
                      }
                    >
                      {i.collected ? "Collected" : i.id === "not" ? "Not collected" : "Off for this assessment"}
                    </span>
                  </div>
                  <p className="mt-1 text-[13px] leading-relaxed text-[#355A7D]">{i.description}</p>
                </li>
              ))}
            </ul>
            <label className="mt-5 flex items-start gap-3 text-sm text-[#0A1931]">
              <input type="checkbox" checked={agree} onChange={(e) => setAgree(e.target.checked)} className="mt-1 h-4 w-4" />
              <span>I&apos;ve read this and I accept that everything marked &ldquo;Collected&rdquo; is recorded for the hiring team.</span>
            </label>
            <div className="mt-4">
              <Button onClick={accept} disabled={!agree || pending}>
                {pending ? "Saving…" : "Accept and continue"}
              </Button>
            </div>
          </div>
        )}

        {notice && accepted && !command && (
          <div className="mt-6">
            <p className="text-sm text-[#1F7A3E]">You&apos;ve accepted what is collected.</p>
            <div className="mt-4">
              <Button onClick={generate} disabled={pending}>
                {pending ? "Preparing…" : "Show me the command"}
              </Button>
            </div>
            <p className="mt-3 text-xs text-[#4A7FA7]">
              The command works once and lasts about ten minutes: ask for it when you&apos;re ready to begin.
            </p>
          </div>
        )}

        {error && <p className="mt-4 text-sm text-[#B4232A]">{error}</p>}

        {command && (
          <div className="mt-6 space-y-5">
            <ol className="space-y-4 text-sm text-[#0A1931]">
              <li>
                <div className="font-semibold">1. Paste this in a terminal</div>
                <p className="mt-1 text-xs text-[#4A7FA7]">It installs the CLI if you don&apos;t have it, then sets up your task and codebase.</p>
                <div className="mt-2 space-y-2">
                  <div>
                    <div className="mb-1 text-xs font-semibold text-[#4A7FA7]">Windows (PowerShell)</div>
                    <Copyable text={command.windows_command} />
                  </div>
                  <div>
                    <div className="mb-1 text-xs font-semibold text-[#4A7FA7]">macOS and Linux</div>
                    <Copyable text={command.unix_command} />
                  </div>
                </div>
              </li>
              <li>
                <div className="font-semibold">2. Connect your GitHub when it asks</div>
                <p className="mt-1 text-xs text-[#4A7FA7]">
                  Your work goes to a private repository in your own GitHub that the hiring team can read. It takes one click in your browser.
                </p>
              </li>
              <li>
                <div className="font-semibold">3. Start. This begins the clock</div>
                <div className="mt-2">
                  <Copyable text="mindfries start" />
                </div>
                <p className="mt-1 text-xs text-[#4A7FA7]">
                  It checks your computer, opens this panel in your browser to share your camera, screen and microphone, then opens your editor.
                </p>
              </li>
            </ol>
            <p className="text-xs text-[#4A7FA7]">
              Already installed? <code>{command.cli_command}</code> does the setup alone. If you ever lose your terminal, come back here: running it again picks up the same session.
            </p>
          </div>
        )}
      </div>

      <Link href="/dashboard" className="mt-6 inline-block text-sm font-semibold text-[#1A3D63] underline">
        Back to dashboard
      </Link>
    </main>
  );
}
