"use client";

import { useState, useSyncExternalStore } from "react";
import clsx from "clsx";
import { CircleCheck, CircleMinus, CircleX, FlaskConical, Play, X } from "lucide-react";
import { idePalette, PRIMARY_BUTTON } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";
import type { VfsBridge } from "@/lib/ide/vfs-bridge";
import type { PreviewController } from "@/lib/ide/shell/types";
import { createSession } from "@/lib/ide/shell/types";
import { executeCommandLine } from "@/lib/ide/shell/execute";
import { terminalLog } from "@/lib/ide/terminal-log";
import { testCommandFor, testResults, type TestCase } from "@/lib/ide/test-results";
import { useSandbox } from "@/lib/ide/sandbox/context";
import { forSandbox } from "@/lib/ide/sandbox/sync";
import { runInSandbox } from "@/app/ide/sandbox-actions";

/**
 * The Tests panel: the last test run, as results rather than as a wall of
 * terminal output — which tests there were, which failed, and what each
 * failure said.
 *
 * It shows whatever the last test run was, wherever it was started: typing
 * `python -m unittest` or `node --test` in the terminal fills this in just
 * as the Run button here does. The button runs the project's test command
 * through the same shell engine the terminal uses, from the workspace root.
 *
 * Nothing here is computed: every line is read from what the test runner
 * printed (lib/ide/test-results.ts). A run whose output can't be read is
 * shown as its exit code and raw output, never as a pass.
 *
 * It is a card of its own beside the terminal; its cross tucks it away
 * (IdeShell keeps a chip on the screen edge that brings it back).
 */
export function TestsPanel({ theme, vfs, preview, onClose }: { theme: IdeTheme; vfs: VfsBridge; preview: PreviewController; onClose?: () => void }) {
  const palette = idePalette(theme);
  const run = useSyncExternalStore(testResults.subscribe, testResults.getSnapshot, testResults.getSnapshot);
  const [running, setRunning] = useState(false);
  const [note, setNote] = useState<string | undefined>();
  const [open, setOpen] = useState<string | null>(null);
  const sandbox = useSandbox();

  const runTests = async () => {
    const command = testCommandFor(Object.keys(vfs.getSnapshot().files));
    if (!command) {
      setNote("No test files found. This looks for Python test_*.py files and JavaScript or TypeScript *.test.js / *.test.ts files.");
      return;
    }
    setNote(undefined);
    setRunning(true);
    if (sandbox) {
      // On the sandbox the tests run on the real machine, against the files
      // as they are on its disk. The result goes through the terminal log
      // exactly as a run typed in the terminal does.
      const sandboxCommand = forSandbox(command);
      const tracked = terminalLog.begin(sandboxCommand);
      const result = await runInSandbox(sandbox.sessionId, sandboxCommand).catch(() => null);
      if (!result || !result.ok) {
        setNote(result && !result.ok ? result.error : "The sandbox didn't answer — try again in a moment.");
        setRunning(false);
        return;
      }
      tracked.output(result.output);
      tracked.finish(result.exitCode);
      setRunning(false);
      return;
    }
    // Run the way the terminal would, so the run is recorded the same way:
    // it reaches the terminal log's listeners, which is what fills this
    // panel and what telemetry hears.
    const session = createSession();
    const tracked = terminalLog.begin(command);
    try {
      await executeCommandLine(command, session, vfs, { write: (text) => tracked.output(text), clear: () => undefined, preview });
    } catch (err) {
      tracked.output(`${err instanceof Error ? err.message : String(err)}\n`);
      session.lastExit = 1;
    } finally {
      tracked.finish(session.lastExit);
      setRunning(false);
    }
  };

  const failing = run?.tests.filter((t) => t.status === "fail" || t.status === "error") ?? [];
  const others = run?.tests.filter((t) => t.status !== "fail" && t.status !== "error") ?? [];

  const total = run ? run.passed + run.failed + run.skipped : 0;

  return (
    <div className={clsx("flex h-full min-h-0 flex-col text-[13px]", palette.text)}>
      <div className={clsx("flex h-12 shrink-0 items-center gap-2.5 border-b pr-2 pl-4", palette.border)}>
        <FlaskConical size={17} className={theme === "dark" ? "text-[#60A5FA]" : palette.accent} />
        <span className="text-[14.5px] font-semibold">Tests</span>
        {run?.parsed && !running && total > 0 && (
          <span className={clsx("text-[12.5px]", run.failed > 0 ? "text-red-400" : "text-emerald-400")}>
            {run.failed > 0 ? `${run.failed} of ${total} failing` : `${total} passed`}
          </span>
        )}
        <button
          type="button"
          onClick={() => void runTests()}
          disabled={running}
          className={clsx("ml-auto flex h-9 items-center gap-2 rounded-lg px-3.5 text-[13.5px] font-semibold disabled:opacity-60", PRIMARY_BUTTON)}
        >
          <Play size={14} className="fill-current" /> {running ? "Running…" : "Run Tests"}
        </button>
        {onClose && (
          <button
            type="button"
            onClick={onClose}
            title="Close the tests card"
            aria-label="Close the tests card"
            className={clsx("flex h-8 w-8 items-center justify-center rounded-lg", palette.textMuted, palette.hover)}
          >
            <X size={16} />
          </button>
        )}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
        {run && !running && (
          <p className={clsx("mb-2 truncate text-[12px]", palette.textMuted)}>
            {run.parsed ? `${run.passed} passed${run.skipped > 0 ? ` · ${run.skipped} skipped` : ""}` : <span className={run.exitCode === 0 ? "" : "text-red-400"}>exited {run.exitCode}</span>}
            {" · "}
            <span className="font-mono">{run.command}</span>
            {" · "}
            {new Date(run.at).toLocaleTimeString()}
          </p>
        )}
        {note && <p className={palette.textMuted}>{note}</p>}
        {!run && !note && (
          <p className={palette.textMuted}>
            No tests have been run yet. Run them here, or from the terminal — the results show up either way.
          </p>
        )}
        {run && !run.parsed && (
          <>
            <p className={palette.textMuted}>
              That run&apos;s output wasn&apos;t in a form this panel can read into results, so here it is as it was printed.
            </p>
            <pre className="mt-2 font-mono whitespace-pre-wrap">{run.tail}</pre>
          </>
        )}
        {run?.parsed && run.tests.length === 0 && run.failed + run.passed > 0 && (
          <p className={palette.textMuted}>
            The run didn&apos;t name its tests individually. Run with <span className="font-mono">-v</span> to see each one.
          </p>
        )}
        {run?.parsed && run.tests.length === 0 && run.failed + run.passed === 0 && (
          <pre className="font-mono whitespace-pre-wrap">{run.tail}</pre>
        )}
        <ul className="space-y-1">
          {[...failing, ...others].map((test, index) => (
            <TestRow
              key={`${test.name}-${index}`}
              test={test}
              open={open === test.name}
              onToggle={() => setOpen((current) => (current === test.name ? null : test.name))}
              theme={theme}
            />
          ))}
        </ul>
      </div>
    </div>
  );
}

/** The line a failure ends on, which is usually the one that says what was wrong. */
function failureLine(detail: string | undefined): string | null {
  const lines = (detail ?? "").split("\n").map((l) => l.trim()).filter(Boolean);
  return lines.at(-1) ?? null;
}

function TestRow({ test, open, onToggle, theme }: { test: TestCase; open: boolean; onToggle: () => void; theme: IdeTheme }) {
  const palette = idePalette(theme);
  const failed = test.status === "fail" || test.status === "error";
  const skipped = test.status === "skip";
  const Icon = failed ? CircleX : skipped ? CircleMinus : CircleCheck;
  const reason = failed ? failureLine(test.detail) : null;
  return (
    <li className="rounded-lg">
      <button
        type="button"
        onClick={onToggle}
        disabled={!test.detail}
        title={test.detail ? (open ? "Hide the failure" : "Show the whole failure") : undefined}
        className={clsx("flex w-full items-start gap-3 rounded-lg px-2 py-2 text-left", test.detail && palette.hover)}
      >
        <Icon size={20} className={clsx("mt-px shrink-0", failed ? "fill-[#F0566A] text-[#0F172A]" : skipped ? palette.textMuted : "fill-[#22C55E] text-[#0F172A]")} />
        <span className="min-w-0 flex-1">
          <span className="block truncate font-mono text-[13.5px]">{test.name}</span>
          {reason && !open && <span className={clsx("mt-1 block truncate font-mono text-[12px]", palette.textMuted)}>{reason}</span>}
        </span>
        <span
          className={clsx(
            "shrink-0 rounded-md px-2 py-0.5 text-[11px] font-semibold tracking-wide",
            failed ? "bg-[#F0566A]/15 text-[#F87182]" : skipped ? "bg-white/5 text-[#94A3B8]" : "bg-[#22C55E]/15 text-[#4ADE80]"
          )}
        >
          {test.status === "error" ? "ERROR" : failed ? "FAILED" : skipped ? "SKIPPED" : "PASSED"}
        </span>
      </button>
      {open && test.detail && (
        <pre className={clsx("mx-2 mb-2 overflow-x-auto rounded-md border px-2.5 py-2 font-mono text-xs whitespace-pre-wrap", palette.border)}>
          {test.detail}
        </pre>
      )}
    </li>
  );
}
