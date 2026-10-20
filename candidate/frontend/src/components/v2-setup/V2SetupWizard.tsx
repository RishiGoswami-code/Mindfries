"use client";

import Link from "next/link";
import { useState, useTransition } from "react";
import { Button } from "@/components/ui/Button";
import { startV2Session } from "@/app/v2-setup/actions";

const ROLE_LABELS: Record<string, string> = {
  backend: "Backend Engineer",
  frontend: "Frontend Engineer",
  fullstack: "Full-Stack SDE",
  fde: "Forward Deployed Engineer",
  quant: "Quantitative Engineer",
  data_analyst: "Data Analyst",
  ml_researcher: "AI/ML Researcher",
  ai_engineer: "AI Engineer",
};

function Copyable({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex items-stretch gap-2">
      <code className="flex-1 overflow-x-auto rounded-lg border border-[#D6E3F0] bg-[#F6FAFD] px-3 py-2 text-[13px] text-[#0A1931]">
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

export function V2SetupWizard({ assessment }: { assessment: { id: string; role: string; company: string } | null }) {
  const [pending, start] = useTransition();
  const [result, setResult] = useState<{ setupCode: string; engineUrl: string } | null>(null);
  const [error, setError] = useState<string | null>(null);

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

  const roleLabel = ROLE_LABELS[assessment.role] ?? assessment.role;

  function generate() {
    setError(null);
    start(async () => {
      const res = await startV2Session(assessment!.id);
      if (!res.ok) {
        setError(res.error);
        return;
      }
      setResult({ setupCode: res.setupCode, engineUrl: res.engineUrl });
    });
  }

  return (
    <main className="mx-auto max-w-2xl px-6 py-12">
      <div className="text-xs font-semibold uppercase tracking-wide text-[#4A7FA7]">Set up your workspace</div>
      <h1 className="mt-2 text-3xl font-extrabold text-[#0A1931]">{roleLabel}</h1>
      <p className="mt-1 text-[#4A7FA7]">{assessment.company}</p>

      <div className="mt-8 rounded-2xl border border-[#D6E3F0] bg-white p-6">
        <p className="text-sm text-[#0A1931]">
          This assessment runs on your own machine in your own editor. You&apos;ll install the Mindfries
          CLI once, then set up and start the assessment from your terminal. Your work is recorded as
          evidence while you code.
        </p>

        {!result ? (
          <div className="mt-6">
            <Button onClick={generate} disabled={pending}>
              {pending ? "Preparing…" : "Generate my setup code"}
            </Button>
            {error && <p className="mt-3 text-sm text-[#B4232A]">{error}</p>}
            <p className="mt-3 text-xs text-[#4A7FA7]">
              The code is one-time and expires in about an hour — generate it when you&apos;re ready to begin.
            </p>
          </div>
        ) : (
          <div className="mt-6 space-y-5">
            <div>
              <div className="text-xs font-semibold uppercase tracking-wide text-[#4A7FA7]">Your setup code</div>
              <div className="mt-1 text-2xl font-extrabold tracking-wider text-[#1A3D63]">{result.setupCode}</div>
            </div>

            <ol className="space-y-4 text-sm text-[#0A1931]">
              <li>
                <div className="font-semibold">1. Install the Mindfries CLI (one-time)</div>
                <div className="mt-1.5">
                  <Copyable text={`curl -fsSL ${result.engineUrl}/install.sh | sh`} />
                </div>
                <div className="mt-1 text-xs text-[#4A7FA7]">Windows (PowerShell): <code>irm {result.engineUrl}/install.ps1 | iex</code></div>
              </li>
              <li>
                <div className="font-semibold">2. Set up the assessment</div>
                <div className="mt-1.5">
                  <Copyable text={`mindfries setup ${result.setupCode}`} />
                </div>
              </li>
              <li>
                <div className="font-semibold">3. Start — this begins the clock</div>
                <div className="mt-1.5">
                  <Copyable text="mindfries start" />
                </div>
              </li>
            </ol>
          </div>
        )}
      </div>

      <Link href="/dashboard" className="mt-6 inline-block text-sm font-semibold text-[#1A3D63] underline">
        Back to dashboard
      </Link>
    </main>
  );
}
