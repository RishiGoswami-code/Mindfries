"use client";

import { useEffect, useMemo, useRef, useState, useTransition } from "react";
import dynamic from "next/dynamic";
import clsx from "clsx";
import { Bot, GripVertical, SquareTerminal, Video, type LucideIcon } from "lucide-react";
import { CameraPanel } from "@/components/ide/CameraPanel";
import { FileExplorer } from "@/components/ide/FileExplorer";
import { Breadcrumbs } from "@/components/ide/Breadcrumbs";
import { EditorPanel } from "@/components/ide/EditorPanel";
import { BottomPanel } from "@/components/ide/BottomPanel";
import { StatusBar } from "@/components/ide/StatusBar";
import { PackageCleanupGuard } from "@/components/ide/PackageCleanupGuard";
import { ChatPanel } from "@/components/ide/ChatPanel";
import { ProctorGate } from "@/components/ide/ProctorGate";
import { EndSessionDialog, SessionEnded } from "@/components/ide/EndSession";
import { SubmitConfirmDialog } from "@/components/ide/HeaderPanel";
import { SCRATCH_MARKDOWN, NO_BRIEF_MARKDOWN } from "@/components/ide/TaskDescriptionPanel";
import { useIdeTheme, type IdeTheme } from "@/lib/ide/theme";
import { idePalette } from "@/lib/ide/palette";
import { SandboxContext } from "@/lib/ide/sandbox/context";
import { useSandboxSync } from "@/lib/ide/sandbox/use-sandbox-sync";
import { useResizable } from "@/lib/ide/use-resizable";
import { useDiagnostics } from "@/lib/ide/diagnostics";
import { exitFullscreen } from "@/lib/ide/fullscreen";
import { loadManifest, saveManifest, type InstalledPackage } from "@/lib/ide/packages";
import { DIFF_PREFIX, isDiffTab, PREVIEW_TAB, TASK_TAB } from "@/lib/ide/changes";
import type { FileContents } from "@/lib/ide/types";
import { submitAssessment } from "@/app/ide/actions";
import { useWorkspaceFiles } from "./useWorkspaceFiles";
import { useWorkspaceTelemetry } from "./useWorkspaceTelemetry";
import { useProctoring } from "./useProctoring";
import { usePreview } from "./usePreview";
import { useSessionClock } from "./useSessionClock";
import { useCheckpoints } from "./useCheckpoints";
import { WorkspaceHeader } from "./WorkspaceHeader";
import { ConfirmDialog, FinishedScreen, TimeUpScreen, Toast } from "./Overlays";

/**
 * The follow-up interview is switched off for the pilot (the dominant AI
 * cost). It's a real flag here, set per deployment, and its code is only
 * loaded when it's on.
 */
const INTERVIEW_ENABLED = process.env.NEXT_PUBLIC_INTERVIEW_ENABLED === "true";
const InterviewDialog = dynamic(() => import("@/components/ide/InterviewDialog").then((m) => m.InterviewDialog), { ssr: false });

export interface WorkspaceProps {
  /** The session this workspace is for. Absent for a practice run (and a scratch workspace). */
  sessionId?: string;
  candidateName?: string;
  assessmentName?: string;
  taskBrief?: string;
  starterFiles?: FileContents;
  /** The server's latest copy of the work; `savedAt` is ms since the epoch. */
  serverWorkspace?: { files: FileContents; savedAt: number; frozen: boolean };
  /** Seconds left on the session's clock when the page loaded. */
  remainingSeconds?: number;
  practice?: boolean;
  /** The session runs on a real machine (a Daytona sandbox), not in this browser. */
  sandbox?: boolean;
  sandboxNetwork?: "open" | "essentials" | "none";
}

/**
 * The v2 workspace (/v2/ide). The same panels as /ide, put together from
 * small hooks — files, telemetry, proctoring, preview, the session clock,
 * checkpoints — instead of one component holding all of it.
 *
 * What's different for the candidate:
 * - Time running out locks the workspace, saves a last copy and hands the
 *   work in, then opens the report. Before, the editor stayed editable and
 *   silently stopped saving.
 * - The clock counts from a deadline, so it can't show more time than is left.
 * - A copy is sent as the tab closes, not just once a minute.
 * - A practice run ends with a practice screen, not "the hiring team will
 *   review your work"; there's no End session button in an assessment.
 * - Errors and confirmations are in the workspace, not browser pop-ups that
 *   would take a fullscreen session out of fullscreen.
 */
export function Workspace({ sessionId, candidateName, assessmentName, taskBrief, starterFiles, serverWorkspace, remainingSeconds, practice = false, sandbox, sandboxNetwork = "open" }: WorkspaceProps) {
  const sandboxed = !!sessionId && !!sandbox;
  const storageId = sessionId ?? (practice ? "practice" : undefined);
  const { theme, toggleTheme } = useIdeTheme();
  const palette = idePalette(theme);
  const cardShadow = theme === "dark" ? "shadow-[0_10px_30px_-14px_rgba(0,0,0,0.75)]" : "";

  // ── Files, telemetry, sandbox ──────────────────────────────────────────
  const recordRef = useRef<(type: string, payload: unknown) => void>(() => undefined);
  const ws = useWorkspaceFiles({
    starterFiles,
    storageId,
    sandboxed,
    serverWorkspace,
    onSaved: (paths, changes) => recordRef.current("file_edit", { paths, changes }),
  });
  const sandboxRefreshRef = useRef<() => void>(() => undefined);
  const telemetry = useWorkspaceTelemetry({ sessionId, sandboxed, activePath: ws.activePath, activeFile: ws.activeFile, onSandboxCommand: () => sandboxRefreshRef.current() });
  useEffect(() => {
    recordRef.current = telemetry.record;
  }, [telemetry.record]);

  // ── Session state: working → (interview) → submitted, or time's up ─────
  const [interviewing, setInterviewing] = useState(() => INTERVIEW_ENABLED && !!sessionId && !!serverWorkspace?.frozen);
  const [interviewed, setInterviewed] = useState(false);
  const [timeUp, setTimeUp] = useState<null | "saving" | "failed">(null);
  const [timeUpError, setTimeUpError] = useState<string | undefined>();
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>();
  const [pending, startTransition] = useTransition();
  const [finished, setFinished] = useState(false);
  const [ending, setEnding] = useState<InstalledPackage[] | null>(null);
  const [ended, setEnded] = useState<{ deleted: boolean } | null>(null);

  const sandboxSync = useSandboxSync({
    sessionId: sandboxed ? sessionId : undefined,
    files: ws.files,
    savedFiles: ws.savedFiles,
    paused: interviewing || interviewed || timeUp !== null,
    onFiles: ws.applySandboxFiles,
  });
  useEffect(() => {
    sandboxRefreshRef.current = sandboxSync.refresh;
  }, [sandboxSync.refresh]);
  const sandboxContext = useMemo(
    () => (sandboxed && sessionId ? { sessionId, refresh: sandboxSync.refresh, network: sandboxNetwork } : null),
    [sandboxed, sessionId, sandboxSync.refresh, sandboxNetwork]
  );

  const working = !!sessionId && ws.restored && !interviewing && !interviewed && timeUp === null;
  const { saveNow } = useCheckpoints({ sessionId, working, sandboxed, flushSandbox: sandboxSync.flush, snapshot: ws.currentSnapshot });

  const proctoring = useProctoring({ sessionId, practice, record: telemetry.record });
  const { camera, screen } = proctoring;

  const preview = usePreview({
    vfs: ws.vfs,
    tree: ws.tree,
    files: ws.files,
    onOpened: () => ws.openFile(PREVIEW_TAB),
    onClosed: () => {
      ws.setOpenPaths((prev) => prev.filter((p) => p !== PREVIEW_TAB));
      ws.setActivePath((current) => (current === PREVIEW_TAB ? TASK_TAB : current));
    },
  });
  const closeTab = (path: string) => (path === PREVIEW_TAB ? preview.close() : ws.closeTab(path));

  const diagnostics = useDiagnostics(ws.tree, ws.files);

  // ── Handing the work in ────────────────────────────────────────────────
  const submitNow = () => {
    if (!sessionId) return;
    setSubmitError(undefined);
    startTransition(async () => {
      await sandboxSync.flush();
      await telemetry.flush();
      const result = await submitAssessment(sessionId, ws.currentSnapshot());
      // Success redirects to the report and never returns here.
      if (result?.error) setSubmitError(result.error);
    });
  };

  /** Time ran out: lock, save, hand in. Or, with the interview on, the interview. */
  const finishOnTimeUp = async () => {
    if (!sessionId || interviewed) return;
    setSubmitting(false);
    if (INTERVIEW_ENABLED && !interviewing) {
      await sandboxSync.flush();
      setInterviewing(true);
      return;
    }
    setTimeUp("saving");
    setTimeUpError(undefined);
    await saveNow();
    await telemetry.flush();
    const result = await submitAssessment(sessionId, ws.currentSnapshot());
    if (result?.error) {
      setTimeUp("failed");
      setTimeUpError(result.error);
    }
  };

  const clock = useSessionClock(sessionId ? remainingSeconds : undefined);
  const expired = clock.expired;
  useEffect(() => {
    if (!expired) return;
    // After this render, not during it: finishing updates state.
    const timer = setTimeout(() => void finishOnTimeUp(), 0);
    return () => clearTimeout(timer);
    // Once, when the clock reaches zero.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [expired]);

  // ── Ending a practice run (an assessment ends with Submit) ─────────────
  const endSession = (deletePackages: boolean) => {
    preview.controller.stop();
    if (deletePackages) {
      saveManifest({});
      ws.vfs.remove("node_modules", true);
    }
    telemetry.stop();
    camera.stop();
    screen.stop();
    void exitFullscreen();
    setEnding(null);
    setEnded({ deleted: deletePackages });
  };

  // ── Layout: three cards can be closed; each leaves a chip ──────────────
  const sidebar = useResizable({ initial: 240, min: 160, max: 480, axis: "horizontal" });
  const terminal = useResizable({ initial: 280, min: 140, max: 620, axis: "vertical", invert: true });
  const rightColumn = useResizable({ initial: 360, min: 280, max: 600, axis: "horizontal", invert: true });
  const cameraHeight = useResizable({ initial: 260, min: 160, max: 520, axis: "vertical", invert: true });
  const [terminalOpen, setTerminalOpen] = useState(true);
  const [chatOpen, setChatOpen] = useState(true);
  const [cameraOpen, setCameraOpen] = useState(true);

  // ── Explorer errors and confirmations, in the workspace ────────────────
  const [toast, setToast] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [confirmFinish, setConfirmFinish] = useState(false);
  const report = (err: string | null) => err && setToast(err);

  const taskMarkdown = sessionId ? (taskBrief ?? NO_BRIEF_MARKDOWN) : practice && taskBrief ? taskBrief : SCRATCH_MARKDOWN;
  const chips = (
    <ClosedChips
      theme={theme}
      chips={[
        !terminalOpen && { id: "terminal", label: "Terminal", icon: SquareTerminal, onOpen: () => setTerminalOpen(true) },
        !chatOpen && { id: "chat", label: "Mindfries AI", icon: Bot, onOpen: () => setChatOpen(true) },
        !cameraOpen && { id: "camera", label: "Camera", icon: Video, onOpen: () => setCameraOpen(true) },
      ]}
    />
  );

  if (finished) return <FinishedScreen theme={theme} practice={practice} />;
  if (ended) return <SessionEnded theme={theme} deleted={ended.deleted} />;

  const grip = clsx("flex shrink-0 items-center justify-center rounded-full", palette.hover);
  const rowGrip = (
    <span className={clsx("h-[3px] w-10 rounded-full opacity-0 transition-opacity group-hover:opacity-100", theme === "dark" ? "bg-[#3B82F6]/60" : "bg-[#4A7FA7]/60")} />
  );

  return (
    <div className={clsx("flex h-dvh w-full flex-col gap-2 p-2", palette.canvas, palette.text)}>
      <WorkspaceHeader
        theme={theme}
        title={assessmentName ?? (sessionId ? "Assessment" : practice ? "Practice run" : "Scratch workspace")}
        remaining={clock.remaining}
        cameraLive={camera.status === "live"}
        onSubmit={() => (sessionId ? setSubmitting(true) : setConfirmFinish(true))}
        submitLabel={sessionId ? "Submit" : "Finish"}
      />

      <div className="flex min-h-0 flex-1 gap-2">
        <div style={{ width: sidebar.size }} className={clsx("flex shrink-0 flex-col overflow-hidden rounded-2xl border", palette.border, palette.panelBg, cardShadow)}>
          <FileExplorer
            tree={ws.tree}
            activePath={ws.activePath}
            theme={theme}
            candidateName={candidateName}
            onOpenFile={ws.openFile}
            onCreate={(parent, kind, name) => report(ws.createEntry(parent, kind, name))}
            onRename={(path, name) => report(ws.renameEntry(path, name))}
            onDelete={(path) => setConfirmDelete(path)}
            // Only outside an assessment: there, Submit is the way out.
            onEndSession={sessionId ? undefined : () => setEnding(Object.values(loadManifest()))}
            onOpenTask={() => ws.openFile(TASK_TAB)}
            taskActive={ws.activePath === TASK_TAB}
          />
        </div>

        <div onMouseDown={sidebar.startDrag} role="separator" aria-orientation="vertical" aria-label="Resize the files panel" className={clsx("-mx-1.5 w-1 cursor-col-resize", grip)}>
          <GripVertical size={10} className={palette.textMuted} />
        </div>

        <div className="flex min-w-0 flex-1 flex-col">
          <div className={clsx("relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border", palette.border, cardShadow)}>
            <EditorPanel
              theme={theme}
              openPaths={ws.openPaths}
              activePath={ws.activePath}
              dirtyPaths={ws.dirtyPaths}
              content={ws.activeFile ? (ws.files[ws.activeFile] ?? "") : ""}
              original={isDiffTab(ws.activePath) && ws.activeFile ? ws.seeded.files[ws.activeFile] : undefined}
              onSelectTab={ws.setActivePath}
              onCloseTab={closeTab}
              onChange={ws.changeFile}
              onSave={ws.saveFile}
              breadcrumbs={<Breadcrumbs path={ws.activeFile} theme={theme} />}
              taskMarkdown={taskMarkdown}
              previewState={preview.preview}
            />
          </div>

          {/* Hidden, never unmounted, while closed: terminals keep their sessions. */}
          <div
            onMouseDown={terminal.startDrag}
            role="separator"
            aria-orientation="horizontal"
            aria-label="Resize the terminal"
            style={{ display: terminalOpen ? undefined : "none" }}
            className="group flex h-2 shrink-0 cursor-row-resize items-center justify-center"
          >
            {rowGrip}
          </div>
          <div
            style={{ height: terminal.size, display: terminalOpen ? undefined : "none" }}
            className={clsx("min-w-0 shrink-0 overflow-hidden rounded-2xl border", palette.border, cardShadow, !terminal.dragging && "transition-[height] duration-200 ease-out")}
          >
            <SandboxContext.Provider value={sandboxContext}>
              <BottomPanel
                theme={theme}
                vfs={ws.vfs}
                preview={preview.controller}
                onClose={() => setTerminalOpen(false)}
                diagnostics={diagnostics}
                onOpenLocation={(path) => ws.openFile(path)}
                changes={ws.changes}
                onOpenDiff={(path) => ws.openFile(`${DIFF_PREFIX}${path}`)}
                previewState={preview.preview}
                onFocusPreview={() => ws.openFile(PREVIEW_TAB)}
                onStopPreview={() => preview.controller.stop()}
              />
            </SandboxContext.Provider>
          </div>
        </div>

        {(chatOpen || cameraOpen) && (
          <>
            <div onMouseDown={rightColumn.startDrag} role="separator" aria-orientation="vertical" aria-label="Resize the assistant and camera" className={clsx("w-1 cursor-col-resize", grip)}>
              <GripVertical size={10} className={palette.textMuted} />
            </div>
            <div style={{ width: rightColumn.size }} className="flex shrink-0 flex-col">
              {chatOpen ? (
                <div className={clsx("min-h-0 flex-1 overflow-hidden rounded-2xl border", palette.border, cardShadow)}>
                  <ChatPanel
                    theme={theme}
                    sessionId={sessionId}
                    activePath={ws.activeFile}
                    activeContent={ws.activeFile ? ws.files[ws.activeFile] : undefined}
                    getFiles={ws.currentSnapshot}
                    onCopy={(chars) => telemetry.record("assistant_copy", { chars })}
                    onClose={() => setChatOpen(false)}
                  />
                </div>
              ) : (
                <div className="min-h-0 flex-1" />
              )}
              {cameraOpen && (
                <>
                  <div onMouseDown={cameraHeight.startDrag} role="separator" aria-orientation="horizontal" aria-label="Resize the camera" className="group flex h-2 shrink-0 cursor-row-resize items-center justify-center">
                    {rowGrip}
                  </div>
                  <div style={{ height: cameraHeight.size }} className={clsx("shrink-0 overflow-hidden rounded-2xl border", palette.border, palette.panelBg, cardShadow)}>
                    <CameraPanel theme={theme} stream={camera.stream} overlay={chips} onClose={() => setCameraOpen(false)} />
                  </div>
                </>
              )}
            </div>
          </>
        )}
      </div>

      {!cameraOpen && (
        <div className="fixed right-4 bottom-14 z-30 flex flex-col items-end gap-2" aria-label="Closed panels">
          {chips}
        </div>
      )}

      <Notices
        theme={theme}
        sandboxed={sandboxed}
        inBrowserSession={!!sessionId && !sandboxed && !practice}
        sandboxStatus={sandboxSync.status}
        sandboxProblem={sandboxSync.problem}
        unshown={sandboxSync.unshown.map((f) => f.path)}
        persistError={ws.persistError && !sandboxed}
      />

      <div className={clsx("-mx-2 -mb-2 shrink-0 overflow-hidden border-t", palette.border)}>
        <StatusBar activePath={ws.activePath} dirtyCount={ws.dirtyPaths.size} diagnostics={diagnostics} theme={theme} onToggleTheme={toggleTheme} />
      </div>

      {camera.status === "live" && <PackageCleanupGuard theme={theme} vfs={ws.vfs} />}
      {ending && <EndSessionDialog theme={theme} packages={ending} onCancel={() => setEnding(null)} onEnd={endSession} />}
      {INTERVIEW_ENABLED && interviewing && sessionId && (
        <InterviewDialog
          theme={theme}
          sessionId={sessionId}
          getFiles={ws.currentSnapshot}
          camera={camera.stream}
          onDone={() => {
            setInterviewing(false);
            setInterviewed(true);
            setSubmitting(true);
            submitNow();
          }}
        />
      )}
      {submitting && sessionId && (
        <SubmitConfirmDialog
          theme={theme}
          pending={pending}
          error={submitError}
          onCancel={() => {
            setSubmitting(false);
            setSubmitError(undefined);
          }}
          onConfirm={(reflection) => {
            if (reflection) telemetry.record("reflection", { text: reflection });
            if (interviewed || !INTERVIEW_ENABLED) {
              submitNow();
              return;
            }
            void sandboxSync.flush().then(() => {
              setSubmitting(false);
              setInterviewing(true);
            });
          }}
          timeUp={interviewed}
          beforeInterview={INTERVIEW_ENABLED && !interviewed}
        />
      )}
      {confirmFinish && (
        <ConfirmDialog
          theme={theme}
          title={practice ? "Finish the practice run?" : "Close this workspace?"}
          body={practice ? "Nothing is sent anywhere. Your files stay in this browser for the next practice run." : "There's no session behind this workspace, so nothing is submitted."}
          confirmLabel="Finish"
          onCancel={() => setConfirmFinish(false)}
          onConfirm={() => {
            camera.stop();
            screen.stop();
            void exitFullscreen();
            setFinished(true);
          }}
        />
      )}
      {confirmDelete && (
        <ConfirmDialog
          theme={theme}
          title={`Delete ${confirmDelete.split("/").pop()}?`}
          body={`"${confirmDelete}" will be deleted. This can't be undone.`}
          confirmLabel="Delete"
          danger
          onCancel={() => setConfirmDelete(null)}
          onConfirm={() => {
            report(ws.deleteEntry(confirmDelete));
            setConfirmDelete(null);
          }}
        />
      )}
      {timeUp && <TimeUpScreen theme={theme} state={timeUp} error={timeUpError} onRetry={() => void finishOnTimeUp()} />}
      <ProctorGate theme={theme} camera={camera} screen={proctoring.proctored ? screen : null} />
      {proctoring.notice && (
        <Toast tone="warning" onDismiss={proctoring.dismissNotice}>
          {proctoring.notice}
        </Toast>
      )}
      {toast && !proctoring.notice && <Toast onDismiss={() => setToast(null)}>{toast}</Toast>}
    </div>
  );
}

/** One line each, when there's something the candidate should know about where their work lives. */
function Notices({
  theme,
  sandboxed,
  inBrowserSession,
  sandboxStatus,
  sandboxProblem,
  unshown,
  persistError,
}: {
  theme: IdeTheme;
  sandboxed: boolean;
  inBrowserSession: boolean;
  sandboxStatus: string;
  sandboxProblem?: string | null;
  unshown: string[];
  persistError: boolean;
}) {
  const palette = idePalette(theme);
  const line = (text: React.ReactNode, warn = false) => (
    <div role="status" className={clsx("shrink-0 rounded-xl border px-3 py-1.5 text-xs", palette.border, warn ? "text-amber-500" : palette.textMuted)}>
      {text}
    </div>
  );
  return (
    <>
      {sandboxed && sandboxStatus === "loading" && line("Loading your files from the sandbox…")}
      {sandboxed && sandboxStatus === "error" && line(`Your sandbox couldn't be reached${sandboxProblem ? ` (${sandboxProblem})` : ""}. Your work on it is safe; reload the page to reconnect.`, true)}
      {sandboxed && sandboxStatus === "ready" && sandboxProblem && line(`The sandbox isn't answering (${sandboxProblem}). Your edits are kept here and sent as soon as it does.`, true)}
      {inBrowserSession && line("This session runs in your browser: Python and JavaScript run for real, but some packages can't be installed and servers can't be started.")}
      {sandboxed &&
        unshown.length > 0 &&
        line(
          <>
            {unshown.length === 1 ? "1 file isn't" : `${unshown.length} files aren't`} shown in the editor (too large or not text):{" "}
            <span className="font-mono">{unshown.slice(0, 4).join(", ")}</span>
            {unshown.length > 4 && ` and ${unshown.length - 4} more`}. Use the terminal for them.
          </>
        )}
      {persistError && line("This browser couldn't keep a copy of your work (storage is full or blocked). It's still being saved to the server.", true)}
    </>
  );
}

interface Chip {
  id: string;
  label: string;
  icon: LucideIcon;
  onOpen: () => void;
}

/** The cards a candidate has closed, as chips that open them again. */
function ClosedChips({ theme, chips }: { theme: IdeTheme; chips: Array<Chip | false> }) {
  const shown = chips.filter((c): c is Chip => !!c);
  if (shown.length === 0) return null;
  return (
    <>
      {shown.map((chip) => (
        <button
          key={chip.id}
          type="button"
          onClick={chip.onOpen}
          title={`Open ${chip.label}`}
          className={clsx(
            "flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-[12.5px] font-medium shadow-[0_8px_20px_-8px_rgba(0,0,0,0.8)] backdrop-blur transition-transform hover:-translate-y-px",
            theme === "dark" ? "border-white/15 bg-[#0F172A]/80 text-[#E2E8F0]" : "border-[#B3CFE5] bg-white/90 text-[#0A1931]"
          )}
        >
          <chip.icon size={14} className={theme === "dark" ? "text-[#60A5FA]" : "text-[#4A7FA7]"} />
          {chip.label}
        </button>
      ))}
    </>
  );
}
