"use client";

import { useCallback, useEffect, useRef } from "react";
import { TelemetryBuffer } from "@/lib/ide/telemetry";
import { watchProvenance } from "@/lib/ide/provenance";
import { terminalLog } from "@/lib/ide/terminal-log";
import { isTestCommand, parseTestRun, testResults } from "@/lib/ide/test-results";
import { output } from "@/lib/ide/output";
import { isDiffTab, TASK_TAB } from "@/lib/ide/changes";

/**
 * What the workspace records about how the candidate works (PRD §1.7):
 * pastes and time away (provenance.ts), each terminal command and test run,
 * which file they're reading, and the git/npm/pip output channels. Only
 * with a real session — a practice run or scratch workspace records nothing.
 *
 * `record` and `flush` are stable for the life of the workspace; anything
 * can call them, and they do nothing when there's no session.
 */
export function useWorkspaceTelemetry({
  sessionId,
  sandboxed,
  activePath,
  activeFile,
  onSandboxCommand,
}: {
  sessionId?: string;
  sandboxed: boolean;
  activePath: string | null;
  activeFile: string | null;
  /** A sandbox's own shell ran a command — it may have changed files there. */
  onSandboxCommand: () => void;
}) {
  const bufferRef = useRef<TelemetryBuffer | null>(null);
  useEffect(() => {
    if (!sessionId) return;
    const buffer = new TelemetryBuffer(sessionId);
    bufferRef.current = buffer;
    return () => {
      buffer.destroy();
      bufferRef.current = null;
    };
  }, [sessionId]);

  const record = useCallback((type: string, payload: unknown) => bufferRef.current?.record(type, payload), []);
  const flush = useCallback(async () => {
    await bufferRef.current?.flush();
  }, []);
  /** Sends what's left and stops — for when the workspace is closing but stays mounted. */
  const stop = useCallback(() => {
    bufferRef.current?.destroy();
    bufferRef.current = null;
  }, []);

  // Pastes and time away — sizes and durations only.
  const activeFileRef = useRef(activeFile);
  useEffect(() => {
    activeFileRef.current = activeFile;
  }, [activeFile]);
  useEffect(() => {
    if (!sessionId) return;
    return watchProvenance(record, () => activeFileRef.current);
  }, [sessionId, record]);

  // Each command that finishes in the terminal, and each test run read from one.
  const onSandboxCommandRef = useRef(onSandboxCommand);
  useEffect(() => {
    onSandboxCommandRef.current = onSandboxCommand;
  });
  useEffect(
    () =>
      terminalLog.onCommand((done) => {
        if (sandboxed) {
          // The sandbox's shell reported this and the backend recorded it;
          // sending it again would count it twice.
          onSandboxCommandRef.current();
        } else {
          record("terminal_command", { command: done.command.slice(0, 300), exitCode: done.exitCode, ms: done.ms });
        }
        if (!isTestCommand(done.command)) return;
        const run = parseTestRun(done.command, done.output, done.exitCode);
        testResults.record(run);
        // On a sandbox the backend reads the run from the output it relayed.
        if (sandboxed) return;
        record("test_run", {
          command: run.command.slice(0, 300),
          exitCode: run.exitCode,
          parsed: run.parsed,
          passed: run.passed,
          failed: run.failed,
          skipped: run.skipped,
          failing: run.tests.filter((t) => t.status === "fail" || t.status === "error").map((t) => t.name).slice(0, 20),
        });
      }),
    [sandboxed, record]
  );

  // Which file they're reading — a file flicked past on the way to another isn't one that was read.
  useEffect(() => {
    if (!sessionId) return;
    if (activePath === TASK_TAB) {
      const timeout = setTimeout(() => record("file_open", { path: "TASK.md", view: "task" }), 1500);
      return () => clearTimeout(timeout);
    }
    if (!activeFile) return;
    const view = isDiffTab(activePath) ? { view: "diff" } : {};
    const timeout = setTimeout(() => record("file_open", { path: activeFile, ...view }), 1500);
    return () => clearTimeout(timeout);
  }, [sessionId, activeFile, activePath, record]);

  // The Output panel's channels (git/npm/pip/preview), what's new each time.
  useEffect(() => {
    if (!sessionId) return;
    const sent: Record<string, number> = {};
    const relay = () => {
      for (const [channel, lines] of Object.entries(output.getSnapshot())) {
        const fresh = lines.slice(sent[channel] ?? 0);
        if (fresh.length === 0) continue;
        sent[channel] = lines.length;
        record("workspace_output", { channel, lines: fresh });
      }
    };
    relay();
    return output.subscribe(relay);
  }, [sessionId, record]);

  return { record, flush, stop };
}
