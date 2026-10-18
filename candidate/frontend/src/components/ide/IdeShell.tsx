"use client";

import { useCallback, useEffect, useMemo, useRef, useState, useTransition } from "react";
import clsx from "clsx";
import { Bot, GripVertical, SquareTerminal, Video, type LucideIcon } from "lucide-react";
import { CameraPanel } from "./CameraPanel";
import { FileExplorer } from "./FileExplorer";
import { Breadcrumbs } from "./Breadcrumbs";
import { EditorPanel } from "./EditorPanel";
import { BottomPanel } from "./BottomPanel";
import { StatusBar } from "./StatusBar";
import { PackageCleanupGuard } from "./PackageCleanupGuard";
import { ChatPanel } from "./ChatPanel";
import { ProctorGate } from "./ProctorGate";
import { EndSessionDialog, SessionEnded } from "./EndSession";
import { InterviewDialog } from "./InterviewDialog";
import { HeaderPanel, SubmitConfirmDialog } from "./HeaderPanel";
import { SCRATCH_MARKDOWN, NO_BRIEF_MARKDOWN } from "./TaskDescriptionPanel";
import { useIdeTheme, type IdeTheme } from "@/lib/ide/theme";
import { idePalette } from "@/lib/ide/palette";
import { initialTree, initialFiles } from "@/lib/ide/mock-project";
import { buildInitialWorkspace } from "@/lib/ide/seed-workspace";
import { SandboxContext } from "@/lib/ide/sandbox/context";
import { useSandboxSync } from "@/lib/ide/sandbox/use-sandbox-sync";
import { addNode, collectFilePaths, findNode, moveNode, removeNode } from "@/lib/ide/tree";
import type { FileContents, TreeNode } from "@/lib/ide/types";
import type { VfsBridge } from "@/lib/ide/vfs-bridge";
import { emptyNotebookJson } from "@/lib/ide/notebook";
import { loadPersistedWorkspace, pickWorkspace, savePersistedWorkspace } from "@/lib/ide/fs-persist";
import { buildPreview, releaseBuild, type PreviewBuild } from "@/lib/ide/preview/build-preview";
import { useResizable } from "@/lib/ide/use-resizable";
import { useProctorCamera } from "@/lib/ide/proctor-camera";
import { startScreenKeyframes, useProctorScreen, type ScreenKeyframes } from "@/lib/ide/screen-capture";
import { keepStill, stillTaker, type StillSink, type StillTaker } from "@/lib/ide/stills";
import { FLAG_NOTICE, watchActivity, type Flag } from "@/lib/ide/activity-watch";
import { watchFaces } from "@/lib/ide/face-watch";
import { watchAudio } from "@/lib/ide/audio-watch";
import { useDiagnostics } from "@/lib/ide/diagnostics";
import { CHANNELS, output } from "@/lib/ide/output";
import { exitFullscreen } from "@/lib/ide/fullscreen";
import { loadManifest, saveManifest, type InstalledPackage } from "@/lib/ide/packages";
import { checkpointWorkspace, submitAssessment } from "@/app/ide/actions";
import { TelemetryBuffer } from "@/lib/ide/telemetry";
import { snapshotFiles } from "@/lib/ide/snapshot";
import { watchProvenance } from "@/lib/ide/provenance";
import { terminalLog } from "@/lib/ide/terminal-log";
import { isTestCommand, parseTestRun, testResults } from "@/lib/ide/test-results";
import { lineChange } from "@/lib/ide/line-change";
import { DIFF_PREFIX, fileOfTab, isDiffTab, isVirtualTab, PREVIEW_TAB, TASK_TAB, workspaceChanges } from "@/lib/ide/changes";
import { startCameraSnapshots } from "@/lib/ide/camera-snapshots";
import { setGitScope } from "@/lib/ide/git/idb";
import { startSnapshotUpload } from "@/app/ide/interview-actions";

interface IdeShellProps {
  /**
   * The real session id this workspace was opened for (onboarding's
   * enterWorkspace passes it as `?session=<id>` on success). Undefined for
   * a workspace opened without a tracked session — directly at /ide — in
   * which case it's a scratch workspace and Submit stays local-only.
   */
  sessionId?: string;
  /** The signed-in candidate's real name from the active session. */
  candidateName?: string;
  /** The human-readable title of the active assessment. */
  assessmentName?: string;
  /**
   * The real assessment content behind `sessionId` — its task brief and
   * starting files, from `game_templates.task_brief`/`starter_files` via
   * GET /sessions/{id}/assessment (see app/ide/page.tsx, which fetches this
   * server-side before the IDE ever renders). Undefined whenever `sessionId`
   * is, when the backend isn't configured, or when the fetch itself failed
   * — every one of those degrades to the same honest fallback content
   * (SCRATCH_MARKDOWN / NO_BRIEF_MARKDOWN, an empty workspace) rather
   * than a special case for each.
   */
  taskBrief?: string;
  starterFiles?: FileContents;
  /**
   * The server's latest copy of this session's work (app/ide/page.tsx), or
   * undefined when it has none. `savedAt` is ms since the epoch.
   */
  serverWorkspace?: { files: FileContents; savedAt: number; frozen: boolean };
  /** Seconds left on the real session's clock. Undefined when there's no session to time. */
  remainingSeconds?: number;
  /**
   * A practice run: no session, so nothing is recorded — but it has a task
   * of its own (taskBrief/starterFiles) and its own saved copy in this
   * browser, kept apart from the scratch workspace and from any session.
   */
  practice?: boolean;
  /**
   * This session's workspace is a real machine (a Daytona sandbox made when
   * the session started): the terminal is a shell on it and the files here
   * are kept in step with the files on its disk. False or absent, the
   * workspace runs in this browser, as it always has.
   */
  sandbox?: boolean;
  /** What that machine may reach on the network; the Ports panel says so. */
  sandboxNetwork?: "open" | "essentials" | "none";
}

/** How often the workspace is saved to the backend while the candidate works. */
const CHECKPOINT_INTERVAL_MS = 60 * 1000;

export function IdeShell({ sessionId, candidateName, taskBrief, starterFiles, assessmentName, remainingSeconds, serverWorkspace, practice, sandbox, sandboxNetwork = "open" }: IdeShellProps) {
  // Set once, from props resolved before this component existed.
  const sandboxed = !!sessionId && !!sandbox;
  // The sandbox sync's two actions, reachable from effects declared above
  // the hook that provides them (it needs state that comes later). Both do
  // nothing until that hook has run, and for an in-browser workspace always.
  const sandboxRefreshRef = useRef<() => void>(() => undefined);
  const sandboxFlushRef = useRef<() => Promise<void>>(() => Promise.resolve());
  // Which saved copy (files and git) this workspace reads and writes in this
  // browser. It is only a storage key — everything that records or submits
  // asks for `sessionId` itself, which a practice run doesn't have.
  const storageId = sessionId ?? (practice ? "practice" : undefined);
  const { theme, toggleTheme } = useIdeTheme();
  const palette = idePalette(theme);
  // The cards' soft shadow, in the dark theme only — on the light canvas it reads as dirt.
  const cardShadow = theme === "dark" ? "shadow-[0_10px_30px_-14px_rgba(0,0,0,0.75)]" : "";

  // A real starting codebase when one was authored for this assessment;
  // the same empty workspace as always otherwise. Computed once, from
  // props already resolved server-side before this component ever mounted
  // — not re-derived on every render — since useState only reads its
  // initializer on the very first render anyway.
  // Git's storage is per session too — set before anything can run git.
  useState(() => setGitScope(storageId));
  const [seeded] = useState(() =>
    starterFiles && Object.keys(starterFiles).length > 0
      ? buildInitialWorkspace(starterFiles)
      : { tree: initialTree, files: initialFiles },
  );
  const [tree, setTree] = useState<TreeNode[]>(seeded.tree);
  const [files, setFiles] = useState<FileContents>(seeded.files);
  const [savedFiles, setSavedFiles] = useState<FileContents>(seeded.files);
  // The task brief (TASK.md) is the first tab: it's what a candidate reads first.
  const [openPaths, setOpenPaths] = useState<string[]>([TASK_TAB]);
  const [activePath, setActivePath] = useState<string | null>(TASK_TAB);

  // The file the active tab is about. A diff tab is "about" the file it
  // compares, so everything that asks what the candidate is looking at —
  // the breadcrumbs, the assistant, telemetry — gets a real path.
  const activeFile = fileOfTab(activePath);
  // What has changed since the task was handed over — see lib/ide/changes.ts.
  const changes = useMemo(() => workspaceChanges(seeded.files, files), [seeded.files, files]);

  const dirtyPaths = useMemo(() => {
    const dirty = new Set<string>();
    for (const path of openPaths) {
      if (files[path] !== savedFiles[path]) dirty.add(path);
    }
    return dirty;
  }, [openPaths, files, savedFiles]);

  // Telemetry (PRD §1.7) — real, but deliberately scoped: git/npm/pip
  // activity and preview rebuilds (via the existing Output-channel store)
  // plus file saves. See lib/ide/telemetry.ts's own doc comment for exactly
  // what this does and doesn't cover yet, and why. Only created when this
  // workspace has a real session to attribute events to.
  const telemetryRef = useRef<TelemetryBuffer | null>(null);
  useEffect(() => {
    if (!sessionId) return;
    const buffer = new TelemetryBuffer(sessionId);
    telemetryRef.current = buffer;
    return () => {
      buffer.destroy();
      telemetryRef.current = null;
    };
  }, [sessionId]);

  // Pastes and time away from the tab — sizes and durations only, see
  // lib/ide/provenance.ts.
  const activePathRef = useRef<string | null>(null);
  useEffect(() => {
    activePathRef.current = activeFile;
  }, [activeFile]);
  useEffect(() => {
    if (!sessionId) return;
    return watchProvenance(
      (type, payload) => telemetryRef.current?.record(type, payload),
      () => activePathRef.current,
    );
  }, [sessionId]);

  // Every command that finishes in the terminal: a test run is read into
  // results, and — in a real session — the command, how it exited and how long it took are recorded.
  // Its output isn't sent, except a test run's counts: what a candidate ran
  // and whether it worked is the evidence; megabytes of logs aren't.
  useEffect(
    () =>
      terminalLog.onCommand((done) => {
        if (sandboxed) {
          // The sandbox's own shell reported this command and the backend
          // has already recorded it — sending it again from here would
          // count it twice. What it may have done is change files.
          sandboxRefreshRef.current();
        } else {
          telemetryRef.current?.record("terminal_command", { command: done.command.slice(0, 300), exitCode: done.exitCode, ms: done.ms });
        }
        if (!isTestCommand(done.command)) return;
        const run = parseTestRun(done.command, done.output, done.exitCode);
        testResults.record(run);
        // In a sandbox the backend read this run's result itself, from the
        // output it relayed; it doesn't take one from the page.
        if (sandboxed) return;
        telemetryRef.current?.record("test_run", {
          command: run.command.slice(0, 300),
          exitCode: run.exitCode,
          parsed: run.parsed,
          passed: run.passed,
          failed: run.failed,
          skipped: run.skipped,
          failing: run.tests.filter((t) => t.status === "fail" || t.status === "error").map((t) => t.name).slice(0, 20),
        });
      }),
    [sandboxed],
  );

  // Which file the candidate is looking at, as it changes — the order they
  // read a codebase in says a lot about how they went looking.
  useEffect(() => {
    if (sessionId && activePath === TASK_TAB) {
      const timeout = setTimeout(() => telemetryRef.current?.record("file_open", { path: "TASK.md", view: "task" }), 1500);
      return () => clearTimeout(timeout);
    }
    if (!sessionId || !activeFile) return;
    const diff = isDiffTab(activePath);
    // A file flicked past on the way to another isn't one that was read.
    const timeout = setTimeout(() => telemetryRef.current?.record("file_open", diff ? { path: activeFile, view: "diff" } : { path: activeFile }), 1500);
    return () => clearTimeout(timeout);
  }, [sessionId, activeFile, activePath]);

  // Relays the Output panel's own channels (git/npm/pip/preview — see
  // lib/ide/output.ts) into telemetry. Tracks how many lines of each
  // channel have already been sent, since the store only exposes full
  // snapshots, not "what's new since last time."
  useEffect(() => {
    if (!sessionId) return;
    const sent: Record<string, number> = {};
    const relay = () => {
      const snapshot = output.getSnapshot();
      for (const [channel, lines] of Object.entries(snapshot)) {
        const from = sent[channel] ?? 0;
        const newLines = lines.slice(from);
        if (newLines.length === 0) continue;
        sent[channel] = lines.length;
        telemetryRef.current?.record("workspace_output", { channel, lines: newLines });
      }
    };
    relay(); // catch anything already buffered before this effect ran
    return output.subscribe(relay);
  }, [sessionId]);

  // Restore whatever was here last time — localStorage only exists client-side,
  // so this has to happen post-mount (matches the theme provider's same pattern).
  const [restored, setRestored] = useState(false);
  useEffect(() => {
    // localStorage is only reachable client-side, so this restore can only
    // happen post-mount — same unavoidable pattern as theme.tsx's read.
    /* eslint-disable react-hooks/set-state-in-effect */
    // A sandbox workspace's files are the ones on the sandbox's disk; they
    // are read from there (useSandboxSync below), and `restored` is set
    // when they arrive.
    if (sandboxed) return;
    // This session's own copy — never another session's — or, when the
    // server's is newer (the candidate last worked somewhere else) or this
    // browser has none, the server's.
    const saved = loadPersistedWorkspace(storageId);
    const source = pickWorkspace(saved, serverWorkspace ?? null);
    if (source === "local" && saved) {
      setTree(saved.tree);
      setFiles(saved.files);
      setSavedFiles(saved.savedFiles);
      // A preview doesn't survive a reload; with no tabs at all, the task opens.
      const tabs = saved.openPaths.filter((p) => p !== PREVIEW_TAB);
      setOpenPaths(tabs.length > 0 ? tabs : [TASK_TAB]);
      setActivePath(tabs.length === 0 || saved.activePath === PREVIEW_TAB || !saved.activePath ? (tabs[0] ?? TASK_TAB) : saved.activePath);
    } else if (source === "server" && serverWorkspace) {
      const restoredFromServer = buildInitialWorkspace(serverWorkspace.files);
      setTree(restoredFromServer.tree);
      setFiles(restoredFromServer.files);
      setSavedFiles(restoredFromServer.files);
      // Keep the tabs the candidate had open here, where they still exist.
      const stillThere = (saved?.openPaths ?? [TASK_TAB]).filter((p) => p === TASK_TAB || p in restoredFromServer.files);
      setOpenPaths(stillThere);
      setActivePath(saved?.activePath && (saved.activePath === TASK_TAB || saved.activePath in restoredFromServer.files) ? saved.activePath : (stillThere[0] ?? null));
    } else if (storageId) {
      // A session (or practice run) opening for the first time starts from its task's own
      // files, with nothing carried over — the mock project's default tab
      // doesn't exist in it.
      setOpenPaths((paths) => paths.filter((p) => isVirtualTab(p) || p in seeded.files));
      setActivePath((path) => (path && (isVirtualTab(path) || path in seeded.files) ? path : null));
    }
    setRestored(true);
    /* eslint-enable react-hooks/set-state-in-effect */
    // Runs once, on mount: the props it reads were fixed server-side before
    // this component existed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Persist on every change, debounced so fast typing doesn't hit
  // localStorage on every keystroke. Skipped until the restore above has
  // run, so a fresh mount doesn't briefly overwrite saved data with the
  // empty initial state.
  useEffect(() => {
    // Nothing to keep in this browser for a sandbox workspace: the sandbox
    // is the copy, and a stale local one must never be mistaken for it.
    if (!restored || sandboxed) return;
    const timeout = setTimeout(() => {
      savePersistedWorkspace({ tree, files, savedFiles, openPaths, activePath }, storageId);
    }, 300);
    return () => clearTimeout(timeout);
  }, [restored, sandboxed, storageId, tree, files, savedFiles, openPaths, activePath]);

  // Auto Save, on by default (no setting to flip it off yet — ask if you
  // want that toggle). A short delay after you stop typing, whatever's
  // dirty gets marked saved — Ctrl/Cmd+S still works too, for an
  // immediate save without waiting out the debounce.
  useEffect(() => {
    if (dirtyPaths.size === 0) return;
    const timeout = setTimeout(() => {
      const paths = [...dirtyPaths];
      // How much each file changed since it was last saved — the size of the
      // edit, not its text (the text is in the workspace checkpoints).
      const changes = paths.map((path) => ({ path, ...lineChange(savedFiles[path] ?? "", files[path] ?? "") }));
      setSavedFiles((prev) => {
        const next = { ...prev };
        for (const path of paths) next[path] = files[path];
        return next;
      });
      telemetryRef.current?.record("file_edit", { paths, changes });
    }, 800);
    return () => clearTimeout(timeout);
    // savedFiles is read for the size of the edit only; depending on it
    // would re-run this the moment it saves.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dirtyPaths, files]);

  const sidebar = useResizable({ initial: 240, min: 160, max: 480, axis: "horizontal" });
  // The height of the bottom row — terminal, tests, and the camera beside them.
  const terminal = useResizable({ initial: 300, min: 160, max: 620, axis: "vertical", invert: true });
  const chatPane = useResizable({ initial: 380, min: 280, max: 600, axis: "horizontal", invert: true });
  // The camera's own height: it never follows the terminal's.
  const cameraPane = useResizable({ initial: 300, min: 180, max: 560, axis: "vertical", invert: true });
  // The terminal, the assistant and the camera can each be tucked away. A
  // closed one leaves a chip that opens it again — on the camera picture,
  // or in the bottom-right corner when the camera itself is closed. A
  // closed terminal keeps its sessions running; closing the camera hides
  // only its picture: it keeps recording.
  const [terminalOpen, setTerminalOpen] = useState(true);
  const [chatOpen, setChatOpen] = useState(true);
  const [cameraOpen, setCameraOpen] = useState(true);
  // The session is camera-proctored: the gate below blocks the workspace
  // until this is live, and re-blocks if the stream ever stops.
  const camera = useProctorCamera(!!sessionId && !practice);
  // A still of the camera every half minute, kept with the session — see
  // lib/ide/camera-snapshots.ts. Only in a real session: a scratch workspace
  // records nothing.
  useEffect(() => {
    if (!sessionId || !camera.stream) return;
    return startCameraSnapshots(camera.stream, {
      uploadTarget: async () => {
        const target = await startSnapshotUpload(sessionId, "camera");
        return "error" in target ? null : target;
      },
      stored: (snapshot) => telemetryRef.current?.record("camera_snapshot", snapshot),
    });
  }, [sessionId, camera.stream]);

  // ── The screen, and live activity flags (issue #120) ──────────────────
  // A real session also needs the entire screen shared (a practice run and
  // the scratch workspace don't record anything, so they don't ask). Its
  // keyframes, and the flags raised during the session, each with a camera
  // still and a screen still of that moment — see lib/ide/screen-capture.ts
  // and lib/ide/activity-watch.ts.
  const screen = useProctorScreen();
  const proctored = !!sessionId && !practice;
  const keyframesRef = useRef<ScreenKeyframes | null>(null);
  const cameraStillsRef = useRef<StillTaker | null>(null);
  const [flagNotice, setFlagNotice] = useState<string | null>(null);
  const stillSink = useMemo<StillSink | null>(
    () =>
      sessionId
        ? {
            uploadTarget: async (kind) => {
              const target = await startSnapshotUpload(sessionId, kind);
              return "error" in target ? null : target;
            },
          }
        : null,
    [sessionId],
  );
  useEffect(() => {
    if (!proctored || !screen.stream || !stillSink) return;
    const keyframes = startScreenKeyframes(screen.stream, stillSink, (still) => telemetryRef.current?.record("screen_snapshot", still));
    keyframesRef.current = keyframes;
    return () => {
      keyframes.stop();
      keyframesRef.current = null;
    };
  }, [proctored, screen.stream, stillSink]);
  useEffect(() => {
    if (!proctored || !camera.stream) return;
    const taker = stillTaker(camera.stream, { width: 640, quality: 0.6 });
    cameraStillsRef.current = taker;
    return () => {
      taker.stop();
      cameraStillsRef.current = null;
    };
  }, [proctored, camera.stream]);

  // One flagged moment: the camera and the screen as they are right now,
  // kept, and the flag recorded with both. The candidate is told.
  const raiseFlag = useCallback(
    async (flag: Flag, audioClip?: Promise<Blob | null>) => {
      const at = new Date().toISOString();
      const notice = FLAG_NOTICE[flag.kind];
      if (notice) setFlagNotice(notice);
      const cameraStill = async () => {
        const blob = await cameraStillsRef.current?.take();
        if (!blob || !stillSink) return null;
        const path = await keepStill(blob, "camera", stillSink);
        if (path) telemetryRef.current?.record("camera_snapshot", { path, bytes: blob.size, reason: "flag" });
        return path;
      };
      // A sound's clip finishes a few seconds after the flag; it is kept and
      // named in the same record.
      const audio = async () => {
        const blob = audioClip ? await audioClip : null;
        if (!blob || !stillSink) return null;
        const path = await keepStill(blob, "audio", stillSink);
        if (path) telemetryRef.current?.record("audio_clip", { path, bytes: blob.size });
        return path;
      };
      const [cameraPath, screenPath, audioPath] = await Promise.all([cameraStill(), keyframesRef.current?.captureNow() ?? Promise.resolve(null), audio()]);
      telemetryRef.current?.record("activity_flag", {
        kind: flag.kind,
        at,
        ...(flag.detail ? { detail: flag.detail } : {}),
        cameraPath,
        screenPath,
        ...(audioClip ? { audioPath } : {}),
      });
    },
    [stillSink],
  );
  useEffect(() => {
    if (!flagNotice) return;
    const timer = setTimeout(() => setFlagNotice(null), 8000);
    return () => clearTimeout(timer);
  }, [flagNotice]);
  // Watching starts once the workspace is in use — both shares live — so
  // the permission prompts themselves aren't flagged as leaving.
  const watching = proctored && camera.status === "live" && screen.status === "live";
  useEffect(() => {
    if (!watching || !sessionId) return;
    return watchActivity((flag) => void raiseFlag(flag), sessionId);
  }, [watching, sessionId, raiseFlag]);
  useEffect(() => {
    if (!watching || !camera.stream) return;
    return watchAudio(camera.stream, (event) => void raiseFlag({ kind: event.kind }, event.clip));
  }, [watching, camera.stream, raiseFlag]);
  useEffect(() => {
    if (!watching || !cameraStillsRef.current) return;
    return watchFaces(cameraStillsRef.current.video, (event) =>
      void raiseFlag({ kind: event.kind, ...(event.kind === "multiple_faces" ? { detail: { faces: event.faces } } : {}) }),
    );
  }, [watching, raiseFlag]);
  // The share stopping is a moment too; the gate below locks the workspace.
  const screenStatus = screen.status;
  useEffect(() => {
    if (!proctored || screenStatus !== "ended") return;
    // After this render, not during it: raising a flag updates state.
    const timer = setTimeout(() => void raiseFlag({ kind: "screen_share_stopped" }), 0);
    return () => clearTimeout(timer);
  }, [proctored, screenStatus, raiseFlag]);

  // Real markers from Monaco's TypeScript service — see lib/ide/diagnostics.ts.
  const diagnostics = useDiagnostics(tree, files);

  // Ending the session, on the candidate's own terms rather than the
  // browser's. `ending` holds the packages the confirmation is asking about;
  // `ended` swaps the workspace for the closing screen.
  const [ending, setEnding] = useState<InstalledPackage[] | null>(null);
  const [ended, setEnded] = useState<{ deleted: boolean } | null>(null);

  // Submit flow — the header's Submit button opens a confirmation dialog.
  // With a real sessionId, confirming calls the backend for real
  // (submitAssessment) and leaves for the report page rather than staying
  // on a local "submitted" screen — there's somewhere real to send the
  // candidate now. Without one (no tracked session for this workspace),
  // it falls back to the original local-only confirmation screen.
  const [submitting, setSubmitting] = useState(false);
  // ponytail: interview DISABLED for the pilot — candidates are judged on
  // their dev-steps/telemetry + chat assistant, not the interview. This skips
  // the whole interviewer_live/voice path (the dominant Gemini cost); the
  // backend interview endpoints are simply never called, so nothing runs or
  // bills in the background. Flip to true to restore the interview.
  const INTERVIEW_ENABLED = false;
  // The follow-up interview (PRD §1.6: workspace → interview → submit) comes
  // after the candidate confirms they're finished, and only for a real
  // session — there's nothing to ask about a scratch workspace. Confirming
  // is the point of no return: the interviewer talks about their code in
  // specifics, so there is no going back to the editor once it starts.
  // A session resumed after its interview began opens on the interview:
  // the work is frozen (the backend refuses further changes), so showing the
  // editor again would only invite edits that go nowhere.
  const [interviewing, setInterviewing] = useState(() => INTERVIEW_ENABLED && !!sessionId && !!serverWorkspace?.frozen);
  // The interview has been held (or couldn't be) — all that remains is the submit itself.
  const [interviewed, setInterviewed] = useState(false);
  // The session's clock has run out. Working time is over: the interview
  // opens by itself with no way back to the editor, and once it's done the
  // work is submitted without asking — there is nothing left to confirm.
  const [timeUp, setTimeUp] = useState(false);
  // Read through a ref so the interview and submit always send the files as
  // they are at that moment, without re-creating callbacks on every keystroke.
  const filesRef = useRef(files);
  useEffect(() => {
    filesRef.current = files;
  }, [files]);
  const currentSnapshot = useCallback(() => snapshotFiles(filesRef.current), []);
  // A copy of the work goes to the backend every couple of minutes while the
  // candidate is still working, so that a session whose tab is closed before
  // Submit still has code on the server (the backend submits it once the
  // time has run out). Stops when the interview starts: the work is frozen
  // from then on, and the backend would refuse it anyway.
  const working = !!sessionId && restored && !interviewing && !interviewed && !timeUp;
  useEffect(() => {
    if (!working || !sessionId) return;
    let lastSent = "";
    const save = () => {
      if (sandboxed) {
        // The backend takes this checkpoint from the sandbox's own disk; the
        // call is only the prompt to do it, once what's saved here is there.
        void sandboxFlushRef.current().then(() => checkpointWorkspace(sessionId, {}));
        return;
      }
      const snapshot = currentSnapshot();
      const serialized = JSON.stringify(snapshot);
      if (serialized === lastSent) return;
      void checkpointWorkspace(sessionId, snapshot).then((result) => {
        if (result.ok) lastSent = serialized;
      });
    };
    const interval = setInterval(save, CHECKPOINT_INTERVAL_MS);
    // Leaving the tab is the moment a candidate is most likely not to come
    // back to this browser, so the server gets a copy then too.
    const onHidden = () => {
      if (document.hidden) save();
    };
    document.addEventListener("visibilitychange", onHidden);
    return () => {
      clearInterval(interval);
      document.removeEventListener("visibilitychange", onHidden);
    };
  }, [working, sessionId, currentSnapshot, sandboxed]);

  // ── The sandbox workspace ────────────────────────────────────────────────
  // Files read from, and saved to, the session's sandbox. Off (and inert)
  // for an in-browser workspace.
  const sandboxSync = useSandboxSync({
    sessionId: sandboxed ? sessionId : undefined,
    files,
    savedFiles,
    paused: interviewing || interviewed || timeUp,
    onFiles: ({ files: nextFiles, saved, initial }) => {
      // The tree is rebuilt from the files themselves: what the explorer
      // shows is what is on the sandbox's disk.
      const rebuilt = buildInitialWorkspace(nextFiles);
      syncSnapshot({ tree: rebuilt.tree, files: nextFiles });
      setTree(rebuilt.tree);
      setFiles(nextFiles);
      setSavedFiles(saved);
      // A tab on a file that is gone from the disk closes.
      setOpenPaths((paths) => paths.filter((p) => isVirtualTab(p) || (fileOfTab(p) !== null && (fileOfTab(p) as string) in nextFiles)));
      setActivePath((path) => (path && (isVirtualTab(path) || (fileOfTab(path) ?? path) in nextFiles) ? path : null));
      if (initial) setRestored(true);
    },
  });
  const { refresh: sandboxRefresh, flush: sandboxFlush } = sandboxSync;
  useEffect(() => {
    sandboxRefreshRef.current = sandboxRefresh;
    sandboxFlushRef.current = sandboxFlush;
  }, [sandboxRefresh, sandboxFlush]);
  const sandboxContext = useMemo(() => (sandboxed && sessionId ? { sessionId, refresh: sandboxRefresh, network: sandboxNetwork } : null), [sandboxed, sessionId, sandboxRefresh, sandboxNetwork]);
  const [submitted, setSubmitted] = useState(false);
  const [submitError, setSubmitError] = useState<string | undefined>();
  const [isSubmittingReal, startSubmitTransition] = useTransition();
  // Time ran out while the candidate was still working (or on the confirm
  // step): go to the interview. If it's already open it simply loses its
  // "Back to my work" button. A scratch workspace has no clock to run out.
  const expire = () => {
    if (!sessionId || interviewed) return;
    void sandboxFlushRef.current();
    setTimeUp(true);
    setSubmitting(false);
    if (INTERVIEW_ENABLED) setInterviewing(true);
    // ponytail: interview off → the backend auto-submits on timeout; no
    // interview is opened and nothing is billed.
  };
  const submitNow = () => {
    if (!sessionId) return;
    setSubmitError(undefined);
    startSubmitTransition(async () => {
      await sandboxFlushRef.current();
      // Make sure everything buffered — the closing reflection most of all —
      // reaches the backend before it reads the session to evaluate it.
      await telemetryRef.current?.flush();
      const result = await submitAssessment(sessionId, currentSnapshot());
      // A successful call redirects and never returns here.
      if (result?.error) setSubmitError(result.error);
    });
  };
  const openFile = (path: string) => {
    setOpenPaths((prev) => (prev.includes(path) ? prev : [...prev, path]));
    setActivePath(path);
  };

  const closeTab = (path: string) => {
    if (path === PREVIEW_TAB) closePreviewRef.current();
    const isDirty = !isVirtualTab(path) && files[path] !== savedFiles[path];
    if (isDirty && !window.confirm(`${path} has unsaved changes. Close anyway?`)) {
      return;
    }
    setOpenPaths((prev) => {
      const next = prev.filter((p) => p !== path);
      if (activePath === path) {
        setActivePath(next.length > 0 ? next[next.length - 1] : null);
      }
      return next;
    });
  };

  const changeFile = (path: string, value: string) => {
    setFiles((prev) => ({ ...prev, [path]: value }));
  };

  const saveFile = (path: string) => {
    setSavedFiles((prev) => ({ ...prev, [path]: files[path] }));
  };

  // --- Virtual filesystem: the single source of truth shared by the
  // Explorer, the Editor, AND the terminal (see vfs-shell.ts). The
  // terminal runs commands from an xterm callback, outside React's render
  // cycle, so it needs a *synchronous* read of the current tree/files —
  // hence the ref, kept in sync every render. Mutators use functional
  // setState, so they're always correct regardless of when they run.
  const vfsSnapshotRef = useRef({ tree, files });
  useEffect(() => {
    vfsSnapshotRef.current = { tree, files };
  });

  /**
   * Writes the mutation into the snapshot ref *synchronously* before queuing
   * the React update. This matters as soon as one command line can run more
   * than one command (`echo hi > a.txt && cat a.txt`): every command in that
   * line executes within a single tick, long before React re-renders, so a
   * ref that only refreshed on render would hand the second command a
   * filesystem that doesn't have the first command's file in it yet.
   */
  const syncSnapshot = (next: { tree?: TreeNode[]; files?: FileContents }) => {
    vfsSnapshotRef.current = {
      tree: next.tree ?? vfsSnapshotRef.current.tree,
      files: next.files ?? vfsSnapshotRef.current.files,
    };
  };

  const vfsCreateFile = (path: string, content = ""): string | null => {
    const { tree: t, files: f } = vfsSnapshotRef.current;
    if (findNode(t, path)) return "File exists";
    const parentPath = path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : null;
    if (parentPath && !findNode(t, parentPath)) return "No such file or directory";
    const name = path.split("/").pop() as string;

    const nextTree = addNode(t, parentPath, { type: "file", path, name });
    const nextFiles = { ...f, [path]: content };
    syncSnapshot({ tree: nextTree, files: nextFiles });
    setTree(nextTree);
    setFiles(nextFiles);
    setSavedFiles((prev) => ({ ...prev, [path]: content }));
    return null;
  };

  const vfsCreateFolder = (path: string): string | null => {
    const { tree: t } = vfsSnapshotRef.current;
    if (findNode(t, path)) return "File exists";
    const parentPath = path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : null;
    if (parentPath && !findNode(t, parentPath)) return "No such file or directory";
    const name = path.split("/").pop() as string;

    const nextTree = addNode(t, parentPath, { type: "folder", path, name, children: [] });
    syncSnapshot({ tree: nextTree });
    setTree(nextTree);
    return null;
  };

  const vfsRemove = (path: string, recursive: boolean): string | null => {
    const { tree: t, files: f } = vfsSnapshotRef.current;
    const node = findNode(t, path);
    if (!node) return "No such file or directory";
    if (node.type === "folder" && !recursive) return "Is a directory";
    const removedFiles = collectFilePaths(t, path);

    const nextTree = removeNode(t, path);
    const nextFiles = omit(f, removedFiles);
    syncSnapshot({ tree: nextTree, files: nextFiles });
    setTree(nextTree);
    setFiles(nextFiles);
    setSavedFiles((prev) => omit(prev, removedFiles));
    setOpenPaths((prev) => {
      const next = prev.filter((p) => !removedFiles.includes(p));
      if (activePath && removedFiles.includes(activePath)) {
        setActivePath(next.length > 0 ? next[next.length - 1] : null);
      }
      return next;
    });
    return null;
  };

  const vfsWrite = (path: string, content: string, append = false): string | null => {
    const { tree: t, files: f } = vfsSnapshotRef.current;
    const existing = findNode(t, path);
    if (existing?.type === "folder") return "Is a directory";

    let nextTree = t;
    if (!existing) {
      const parentPath = path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : null;
      if (parentPath && !findNode(t, parentPath)) return "No such file or directory";
      const name = path.split("/").pop() as string;
      nextTree = addNode(t, parentPath, { type: "file", path, name });
    }

    const newContent = append ? (f[path] ?? "") + content : content;
    const nextFiles = { ...f, [path]: newContent };
    syncSnapshot({ tree: nextTree, files: nextFiles });
    setTree(nextTree);
    setFiles(nextFiles);
    setSavedFiles((prev) => ({ ...prev, [path]: newContent }));
    return null;
  };

  const vfsMove = (srcPath: string, destParentPath: string | null, destName: string): string | null => {
    const { tree: t } = vfsSnapshotRef.current;
    if (!findNode(t, srcPath)) return "No such file or directory";
    if (destParentPath && !findNode(t, destParentPath)) return "No such file or directory";
    const destPath = destParentPath ? `${destParentPath}/${destName}` : destName;
    if (destPath !== srcPath && findNode(t, destPath)) return "File exists";

    const oldFilePaths = collectFilePaths(t, srcPath);
    const nextTree = moveNode(t, srcPath, destParentPath, destName);
    if (!nextTree) return "No such file or directory";
    const newFilePaths = collectFilePaths(nextTree, destPath);

    syncSnapshot({ tree: nextTree });
    setTree(nextTree);
    remapFilePaths(oldFilePaths, newFilePaths);
    return null;
  };

  const remapFilePaths = (oldPaths: string[], newPaths: string[]) => {
    if (oldPaths.length !== newPaths.length) return;
    const mapping = new Map(oldPaths.map((old, i) => [old, newPaths[i]]));

    const nextFiles = remapKeys(vfsSnapshotRef.current.files, mapping);
    syncSnapshot({ files: nextFiles });
    setFiles(nextFiles);
    setSavedFiles((prev) => remapKeys(prev, mapping));
    setOpenPaths((prev) => prev.map((p) => mapping.get(p) ?? p));
    setActivePath((prev) => (prev ? mapping.get(prev) ?? prev : prev));
  };

  const vfs: VfsBridge = {
    getSnapshot: () => vfsSnapshotRef.current,
    createFile: vfsCreateFile,
    createFolder: vfsCreateFolder,
    remove: vfsRemove,
    write: vfsWrite,
    move: vfsMove,
  };

  // The live preview: `root` is the project being watched, so edits can
  // rebuild it. Kept separate from `html` so the rebuild effect can depend
  // on the root without re-running on its own output.
  const [preview, setPreview] = useState<
    { html: string; title: string; root: string; watching: boolean } | null
  >(null);
  const previewBuildRef = useRef<PreviewBuild | null>(null);
  // Opens the preview's tab. Reached through a ref so the preview controller
  // below can stay one stable object for the life of the workspace.
  const showPreviewTabRef = useRef<() => void>(() => undefined);
  useEffect(() => {
    showPreviewTabRef.current = () => {
      setOpenPaths((prev) => (prev.includes(PREVIEW_TAB) ? prev : [...prev, PREVIEW_TAB]));
      setActivePath(PREVIEW_TAB);
    };
  }, []);

  // Subscribers are the running dev server: it logs each rebuild into the
  // terminal, the way a real one does.
  const rebuildListenersRef = useRef(new Set<(line: string) => void>());
  const emitRebuild = (line: string) => {
    // Rebuilds are driven by the file watcher rather than a command, so they
    // bypass the executor's channel routing and are logged here instead.
    output.append(CHANNELS.preview, line);
    for (const listener of rebuildListenersRef.current) listener(line);
  };

  const previewController = useMemo(
    () => ({
      open: (build: { html: string; title: string; root: string; objectUrls: string[] }) => {
        releaseBuild(previewBuildRef.current);
        previewBuildRef.current = { objectUrls: build.objectUrls } as PreviewBuild;
        setPreview({ html: build.html, title: build.title, root: build.root, watching: true });
        // The preview opens where the code does, as a tab of its own.
        showPreviewTabRef.current();
      },
      // Stopping leaves the panel up — the last build still renders, it just
      // no longer follows edits, which is what killing a dev server does.
      stop: () => setPreview((current) => (current ? { ...current, watching: false } : current)),
      onRebuild: (listener: (line: string) => void) => {
        rebuildListenersRef.current.add(listener);
        return () => {
          rebuildListenersRef.current.delete(listener);
        };
      },
    }),
    []
  );

  // Closing the preview (its tab, or the panel's own button) ends it.
  // Reached through a ref from closeTab, which is declared before the
  // preview's state exists.
  const closePreviewRef = useRef<() => void>(() => undefined);
  useEffect(() => {
    closePreviewRef.current = () => {
      releaseBuild(previewBuildRef.current);
      previewBuildRef.current = null;
      setPreview(null);
      setOpenPaths((prev) => prev.filter((p) => p !== PREVIEW_TAB));
      setActivePath((current) => (current === PREVIEW_TAB ? TASK_TAB : current));
    };
  }, []);

  // Keep the preview live: any edit rebuilds the watched project, debounced
  // so a burst of typing produces one rebuild. Depends on the ROOT (not the
  // built html), so writing the result can't retrigger it.
  const previewRoot = preview?.watching ? preview.root : null;
  useEffect(() => {
    if (previewRoot === null) return;
    const timeout = setTimeout(() => {
      const started = performance.now();
      buildPreview(vfs, previewRoot)
        .then((build) => {
          releaseBuild(previewBuildRef.current);
          previewBuildRef.current = build;
          setPreview((current) =>
            current && current.root === previewRoot ? { ...current, html: build.html } : current
          );
          emitRebuild(`rebuilt in ${Math.round(performance.now() - started)}ms`);
        })
        .catch((err) => {
          // A broken intermediate edit shouldn't blank the preview — keep the
          // last build that worked, and report it the way a dev server would.
          emitRebuild(`build failed: ${err instanceof Error ? err.message : String(err)}`);
        });
    }, 400);
    return () => clearTimeout(timeout);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- vfs reads live state via a ref; rebuilding is driven by tree/files changing
  }, [previewRoot, tree, files]);

  /**
   * Ends the session for real: stop the preview, release the camera, and
   * optionally drop the downloaded packages. The camera goes last so the
   * capture light goes out at the same moment the closing screen appears.
   *
   * Files are deliberately left alone — the candidate's work is theirs, and
   * nothing here has been submitted anywhere yet.
   */
  const endSession = (deletePackages: boolean) => {
    previewController.stop();
    if (deletePackages) {
      saveManifest({});
      vfs.remove("node_modules", true);
    }
    // Flush and stop telemetry before the closing screen replaces the
    // workspace. The component doesn't unmount on end-session, so the
    // keyed effect's cleanup never reruns — destroy() must be called
    // explicitly here, otherwise the 5-second interval keeps firing.
    telemetryRef.current?.destroy();
    telemetryRef.current = null;
    camera.stop();
    screen.stop();
    // Hand the screen back: staying fullscreen on a closing screen leaves
    // someone stuck in a chrome-less tab with nothing left to do in it.
    void exitFullscreen();
    setEnding(null);
    setEnded({ deleted: deletePackages });
  };

  // Explorer-driven create/rename/delete are the same virtual filesystem
  // operations the terminal uses, just with UI-appropriate confirmation
  // dialogs and auto-opening a newly created file in the editor.
  const createEntry = (parentPath: string | null, kind: "file" | "folder", name: string) => {
    const path = parentPath ? `${parentPath}/${name}` : name;
    const err =
      kind === "file"
        ? vfsCreateFile(path, name.toLowerCase().endsWith(".ipynb") ? emptyNotebookJson() : "")
        : vfsCreateFolder(path);
    if (err) {
      window.alert(err === "File exists" ? `"${name}" already exists here.` : err);
      return;
    }
    if (kind === "file") openFile(path);
  };

  const renameEntry = (path: string, newName: string) => {
    const parentPath = path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : null;
    const err = vfsMove(path, parentPath, newName);
    if (err) window.alert(err === "File exists" ? `"${newName}" already exists here.` : err);
  };

  const deleteEntry = (path: string) => {
    if (!window.confirm(`Delete "${path}"? This can't be undone.`)) return;
    vfsRemove(path, true);
  };

  // What the TASK.md tab shows.
  const taskMarkdown = sessionId ? (taskBrief ?? NO_BRIEF_MARKDOWN) : practice && taskBrief ? taskBrief : SCRATCH_MARKDOWN;
  // The chips that reopen closed cards.
  const closedChips = (
    <ClosedChips
      theme={theme}
      chips={[
        !terminalOpen && { id: "terminal", label: "Terminal", icon: SquareTerminal, onOpen: () => setTerminalOpen(true) },
        !chatOpen && { id: "chat", label: "Mindfries AI", icon: Bot, onOpen: () => setChatOpen(true) },
        !cameraOpen && { id: "camera", label: "Camera", icon: Video, onOpen: () => setCameraOpen(true) },
      ]}
    />
  );

  // Submission replaces the workspace the same way ending does.
  if (submitted) return <SubmittedScreen theme={theme} />;

  // Replaces the workspace outright rather than overlaying it: the session is
  // over, so there's nothing behind the screen worth showing. It also means
  // the proctoring gate can't reappear over the top once the camera is
  // released — releasing it deliberately looks the same to the gate as never
  // having started.
  if (ended) return <SessionEnded theme={theme} deleted={ended.deleted} />;

  // The chrome is a set of independent rounded "cards" (Explorer, Editor,
  // Terminal, status bar) floating on a recessed canvas, rather than VS
  // Code's flush edge-to-edge panels — every panel gets `overflow-hidden` so
  // its own (sharp-cornered) internal content is clipped to the card's
  // rounded shape instead of poking out past the corners.
  return (
    <div className={clsx("flex h-dvh w-full flex-col gap-2 p-2", palette.canvas, palette.text)}>
      <HeaderPanel
        theme={theme}
        assessmentName={assessmentName ?? (sessionId ? "Assessment" : "Scratch workspace")}
        durationSeconds={remainingSeconds}
        onSubmit={() => setSubmitting(true)}
        onExpire={expire}
        cameraLive={camera.status === "live"}
      />
      <div className="flex min-h-0 flex-1 gap-2">
        <div
          style={{ width: sidebar.size }}
          className={clsx("flex shrink-0 flex-col overflow-hidden rounded-2xl border", palette.border, palette.panelBg, cardShadow)}
        >
          <FileExplorer
            tree={tree}
            activePath={activePath}
            theme={theme}
            candidateName={candidateName}
            onOpenFile={openFile}
            onCreate={createEntry}
            onRename={renameEntry}
            onDelete={deleteEntry}
            onEndSession={() => setEnding(Object.values(loadManifest()))}
            onOpenTask={() => openFile(TASK_TAB)}
            taskActive={activePath === TASK_TAB}
          />
        </div>

        <div
          onMouseDown={sidebar.startDrag}
          className={clsx("-mx-1.5 flex w-1 shrink-0 cursor-col-resize items-center justify-center rounded-full", palette.hover)}
        >
          <GripVertical size={10} className={palette.textMuted} />
        </div>

        <div className="flex min-w-0 flex-1 flex-col">
          <div className={clsx("relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border", palette.border, cardShadow)}>
            <div className="min-h-0 flex-1">
              <EditorPanel
                theme={theme}
                openPaths={openPaths}
                activePath={activePath}
                dirtyPaths={dirtyPaths}
                content={activeFile ? files[activeFile] ?? "" : ""}
                original={isDiffTab(activePath) && activeFile ? seeded.files[activeFile] : undefined}
                onSelectTab={setActivePath}
                onCloseTab={closeTab}
                onChange={changeFile}
                onSave={saveFile}
                breadcrumbs={<Breadcrumbs path={activeFile} theme={theme} />}
                taskMarkdown={taskMarkdown}
                previewState={preview}
              />
            </div>
          </div>

          {/* The terminal's top edge: drag it to size the terminal. The
              camera has a height of its own and doesn't move with it. */}
          {/* Hidden, never unmounted, while closed: the terminals keep their
              sessions and scrollback. */}
          <div
            onMouseDown={terminal.startDrag}
            title="Drag to resize"
            style={{ display: terminalOpen ? undefined : "none" }}
            className="group flex h-2 shrink-0 cursor-row-resize items-center justify-center"
          >
            <span className={clsx("h-[3px] w-10 rounded-full opacity-0 transition-opacity group-hover:opacity-100", theme === "dark" ? "bg-[#3B82F6]/60" : "bg-[#4A7FA7]/60")} />
          </div>
          <div
            style={{ height: terminal.size, display: terminalOpen ? undefined : "none" }}
            className={clsx("min-w-0 shrink-0 overflow-hidden rounded-2xl border", palette.border, cardShadow, !terminal.dragging && "transition-[height] duration-200 ease-out")}
          >
            {/* The terminal acts differently on a sandbox; the context is
                how it knows. */}
            <SandboxContext.Provider value={sandboxContext}>
              <BottomPanel
                theme={theme}
                vfs={vfs}
                preview={previewController}
                onClose={() => setTerminalOpen(false)}
                diagnostics={diagnostics}
                onOpenLocation={(path) => openFile(path)}
                changes={changes}
                onOpenDiff={(path) => openFile(`${DIFF_PREFIX}${path}`)}
                previewState={preview}
                onFocusPreview={() => openFile(PREVIEW_TAB)}
                // Same path Ctrl+C takes: stop watching, keep the last build on
                // screen. Closing the preview's tab is a separate, more destructive act.
                onStopPreview={() => previewController.stop()}
              />
            </SandboxContext.Provider>
          </div>
        </div>

        {(chatOpen || cameraOpen) && (
          <>
            <div
              onMouseDown={chatPane.startDrag}
              className={clsx("flex w-1 shrink-0 cursor-col-resize items-center justify-center rounded-full", palette.hover)}
            >
              <GripVertical size={10} className={palette.textMuted} />
            </div>
            <div style={{ width: chatPane.size }} className="flex shrink-0 flex-col">
              {chatOpen ? (
                <div className={clsx("min-h-0 flex-1 overflow-hidden rounded-2xl border", palette.border, cardShadow)}>
                  <ChatPanel
                    theme={theme}
                    sessionId={sessionId}
                    activePath={activeFile}
                    activeContent={activeFile ? files[activeFile] : undefined}
                    getFiles={currentSnapshot}
                    onCopy={(chars) => telemetryRef.current?.record("assistant_copy", { chars })}
                    onClose={() => setChatOpen(false)}
                  />
                </div>
              ) : (
                <div className="min-h-0 flex-1" />
              )}
              {cameraOpen && (
                <>
                  {/* The camera's own top edge: drag it to size the camera. */}
                  <div onMouseDown={cameraPane.startDrag} title="Drag to resize the camera" className="group flex h-2 shrink-0 cursor-row-resize items-center justify-center">
                    <span className={clsx("h-[3px] w-10 rounded-full opacity-0 transition-opacity group-hover:opacity-100", theme === "dark" ? "bg-[#3B82F6]/60" : "bg-[#4A7FA7]/60")} />
                  </div>
                  <div style={{ height: cameraPane.size }} className={clsx("shrink-0 overflow-hidden rounded-2xl border", palette.border, palette.panelBg, cardShadow)}>
                    <CameraPanel theme={theme} stream={camera.stream} overlay={closedChips} onClose={() => setCameraOpen(false)} />
                  </div>
                </>
              )}
            </div>
          </>
        )}
      </div>


      {!cameraOpen && (
        <div className="fixed right-4 bottom-14 z-30 flex flex-col items-end gap-2" aria-label="Closed panels">
          {closedChips}
        </div>
      )}

      {/* Where the workspace runs, and whether it is reachable — said once,
          plainly, rather than left for the candidate to infer from a
          terminal that won't connect. */}
      {sandboxed && (sandboxSync.status !== "ready" || sandboxSync.problem) && (
        <div
          role="status"
          className={clsx(
            "shrink-0 rounded-xl border px-3 py-1.5 text-xs",
            palette.border,
            sandboxSync.status === "error" || sandboxSync.problem ? "text-amber-500" : palette.textMuted,
          )}
        >
          {sandboxSync.status === "loading" && "Loading your files from the sandbox…"}
          {sandboxSync.status === "error" &&
            `Your sandbox couldn't be reached${sandboxSync.problem ? ` (${sandboxSync.problem})` : ""}. Reload the page to try again — your work on it is safe.`}
          {sandboxSync.status === "ready" && sandboxSync.problem && `The sandbox isn't answering (${sandboxSync.problem}). Your edits are kept here and sent as soon as it does.`}
        </div>
      )}

      {/* A real session that didn't get a machine — the pool was full, or
          it didn't come up — runs here in the browser. Said once, so a
          candidate isn't surprised that some packages won't install. */}
      {sessionId && !sandboxed && !practice && (
        <div role="status" className={clsx("shrink-0 rounded-xl border px-3 py-1.5 text-xs", palette.border, palette.textMuted)}>
          This session runs in your browser: Python and JavaScript run for real, but some packages can&apos;t be installed and servers can&apos;t be
          started. The hiring team can see which workspace you had.
        </div>
      )}

      {/* Files the editor leaves out are still in the project. Without this
          line a candidate who generated a large data file from the terminal
          would see nothing appear and reasonably think it failed. */}
      {sandboxed && sandboxSync.unshown.length > 0 && (
        <div role="status" className={clsx("shrink-0 rounded-xl border px-3 py-1.5 text-xs", palette.border, palette.textMuted)}>
          {sandboxSync.unshown.length === 1 ? "1 file in your project isn't" : `${sandboxSync.unshown.length} files in your project aren't`} shown in the
          editor (too large or not text):{" "}
          <span className="font-mono">
            {sandboxSync.unshown
              .slice(0, 4)
              .map((f) => f.path)
              .join(", ")}
          </span>
          {sandboxSync.unshown.length > 4 && ` and ${sandboxSync.unshown.length - 4} more`}. They are on your sandbox — use the terminal to open or
          change them. The report and the interviewer read text files only.
        </div>
      )}

      {/* Edge to edge along the bottom, outside the canvas's padding. */}
      <div className={clsx("-mx-2 -mb-2 shrink-0 overflow-hidden border-t", palette.border)}>
        <StatusBar
          activePath={activePath}
          dirtyCount={dirtyPaths.size}
          diagnostics={diagnostics}
          theme={theme}
          onToggleTheme={toggleTheme}
        />
      </div>

      {/* Held back until the camera gate has been satisfied: both are modal,
          and asking about cached packages behind a "you cannot start yet"
          dialog reads as a stack of broken overlays. The proctoring gate is
          the one that has to be answered first. */}
      {camera.status === "live" && <PackageCleanupGuard theme={theme} vfs={vfs} />}
      {ending && (
        <EndSessionDialog
          theme={theme}
          packages={ending}
          onCancel={() => setEnding(null)}
          onEnd={endSession}
        />
      )}
      {interviewing && sessionId && (
        <InterviewDialog
          theme={theme}
          sessionId={sessionId}
          getFiles={currentSnapshot}
          camera={camera.stream}
          onDone={() => {
            // The interview is the last step: the candidate confirmed before
            // it began, so the work goes in as soon as it ends. The dialog
            // below shows that happening, and the error if it fails.
            setInterviewing(false);
            setInterviewed(true);
            setSubmitting(true);
            submitNow();
          }}
        />
      )}
      {submitting && (
        <SubmitConfirmDialog
          theme={theme}
          pending={isSubmittingReal}
          error={submitError}
          onCancel={() => {
            setSubmitting(false);
            setSubmitError(undefined);
          }}
          onConfirm={(reflection) => {
            if (!sessionId) {
              // No tracked session for this workspace (backend wasn't
              // configured when it was entered, or /ide was opened
              // directly) — nothing real to submit to, so this stays the
              // original local-only confirmation.
              setSubmitting(false);
              camera.stop();
              screen.stop();
              exitFullscreen();
              setSubmitted(true);
              return;
            }
            // The candidate's own closing words, if any — recorded before the
            // submit below flushes telemetry and triggers evaluation.
            if (reflection) telemetryRef.current?.record("reflection", { text: reflection });
            if (interviewed || !INTERVIEW_ENABLED) {
              // Already interviewed, or the interview is disabled for this
              // pilot — submit directly instead of opening the interviewer.
              submitNow();
              return;
            }
            // Confirmed: the interview comes next, and there's no way back
            // from it to the editor. In a sandbox workspace the work the
            // interviewer reads is the sandbox's, so anything still on its
            // way there is sent first.
            void sandboxFlushRef.current().then(() => {
              setSubmitting(false);
              setInterviewing(true);
            });
          }}
          timeUp={timeUp || interviewed}
          beforeInterview={INTERVIEW_ENABLED && !interviewed}
        />
      )}
      <ProctorGate theme={theme} camera={camera} screen={proctored ? screen : null} />
      {flagNotice && (
        <div role="status" className="fixed bottom-4 left-1/2 z-40 max-w-md -translate-x-1/2 rounded-xl bg-[#0A1931] px-4 py-2.5 text-center text-xs text-white shadow-lg">
          {flagNotice}
        </div>
      )}
    </div>
  );
}

function omit(obj: FileContents, keys: string[]): FileContents {
  const keySet = new Set(keys);
  return Object.fromEntries(Object.entries(obj).filter(([k]) => !keySet.has(k)));
}

function remapKeys(obj: FileContents, mapping: Map<string, string>): FileContents {
  return Object.fromEntries(
    Object.entries(obj).map(([k, v]) => [mapping.get(k) ?? k, v])
  );
}

/**
 * Full-screen closing state after the candidate confirms submission.
 * Same pattern as SessionEnded but with a submission-specific message.
 */
function SubmittedScreen({ theme }: { theme: IdeTheme }) {
  const palette = idePalette(theme);

  return (
    <div className={clsx("flex h-screen w-full items-center justify-center p-6", palette.appBg)}>
      <div className={clsx("max-w-sm text-center", palette.text)}>
        {/* eslint-disable-next-line @next/next/no-img-element -- tiny static local SVG */}
        <img src="/mindfries-logo.svg" alt="" width={36} height={36} className="mx-auto" />
        <h1 className="mt-4 text-base font-semibold">Work submitted</h1>
        <p className={clsx("mt-2 text-sm leading-relaxed", palette.textMuted)}>
          Your code and all session activity have been submitted for evaluation. The hiring team
          will review your work and get back to you.
        </p>
        <p className={clsx("mt-3 text-xs", palette.textMuted)}>You can close this tab now.</p>
      </div>
    </div>
  );
}

interface EdgeChip {
  id: string;
  label: string;
  icon: LucideIcon;
  onOpen: () => void;
  badge?: { text: string; bad: boolean };
}

/**
 * The cards a candidate has closed, as chips floating over the camera
 * picture — always in view, never over the code. Clicking one opens its
 * card again, as it was.
 */
function ClosedChips({ theme, chips }: { theme: IdeTheme; chips: Array<EdgeChip | false> }) {
  const shown = chips.filter((c): c is EdgeChip => !!c);
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
          {chip.badge && (
            <span className={clsx("rounded-full px-1.5 text-[10.5px] font-semibold", chip.badge.bad ? "bg-red-500/30 text-red-300" : "bg-emerald-500/25 text-emerald-300")}>
              {chip.badge.text}
            </span>
          )}
        </button>
      ))}
    </>
  );
}
