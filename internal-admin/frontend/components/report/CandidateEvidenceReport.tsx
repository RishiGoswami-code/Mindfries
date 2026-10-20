"use client";
// Copied from company/frontend/components/report/CandidateEvidenceReport.tsx so the Mindfries admin sees the same
// evidence report a company does. Keep the two in step.

import React, { useState } from "react";
import {
  Clock,
  Code2,
  FileText,
  Flag,
  Folder,
  Lightbulb,
  ListChecks,
  MessageSquare,
  Play,
  SquareTerminal,
  UserRound,
  Workflow,
  ChevronRight,
  Video,
  ShieldAlert,
  Camera,
  Monitor,
  Mic,
  ExternalLink,
  type LucideIcon,
} from "lucide-react";
import type { AssessmentReport, CameraRecord, FlaggedMoment, InterviewExchange, SessionSummary } from "@/lib/report/types";
import type { ReportActivity } from "@/lib/report/data";
import { clock, shortTimeline, type TimelineTone } from "@/lib/report/trail";
import { DiffView, FlagList, Observations, Panel, Pill, TestResults, Timeline } from "./Parts";

type Section = "summary" | "code" | "workflow" | "reasoning" | "interview" | "artifacts" | "proctoring";

const SECTIONS: Array<{ id: Section; label: string; icon: LucideIcon; blurb: string }> = [
  { id: "summary", label: "Summary", icon: FileText, blurb: "Overview & key metrics" },
  { id: "code", label: "Code evaluation", icon: Code2, blurb: "Changes, diffs and test runs" },
  { id: "workflow", label: "Workflow", icon: Workflow, blurb: "Timeline and activity trail" },
  { id: "reasoning", label: "Reasoning", icon: Lightbulb, blurb: "Understanding and AI chat" },
  { id: "interview", label: "Interview & Recordings", icon: MessageSquare, blurb: "Q&A with video/audio replay" },
  { id: "artifacts", label: "Artifacts", icon: Folder, blurb: "Files as left by candidate" },
  { id: "proctoring", label: "Proctoring & Snapshots", icon: Camera, blurb: "Webcam, screen & flagged clips" },
];

const CATEGORIES: Record<"code" | "workflow" | "reasoning" | "interview", string[]> = {
  code: ["code_evaluation", "provenance", "rubric"],
  workflow: ["workflow", "activity_flags", "integrity"],
  reasoning: ["reasoning", "ai_usage"],
  interview: ["interview", "interview_status"],
};

const RECOMMENDATION: Record<string, { label: string; tone: "good" | "warn" | "bad" }> = {
  strong_hire: { label: "Strong hire", tone: "good" },
  hire: { label: "Hire", tone: "good" },
  lean_no: { label: "Leaning no", tone: "warn" },
  no_hire: { label: "No hire", tone: "bad" },
};

export function CandidateEvidenceReport({
  session,
  report,
  activity,
  interview,
  camera,
  screen,
  flags,
}: {
  session: SessionSummary | null;
  report: AssessmentReport | null;
  activity: ReportActivity | null;
  interview: InterviewExchange[];
  camera: CameraRecord;
  screen: CameraRecord;
  flags: FlaggedMoment[];
}) {
  const [activeSection, setActiveSection] = useState<Section>("summary");

  const evidence = report?.status === "ready" ? (report.evidence ?? []) : [];
  const ready = report?.status === "ready";

  const pick = (section: "code" | "workflow" | "reasoning" | "interview") => {
    return evidence.filter((e) => CATEGORIES[section].includes(e.category));
  };

  const trail = activity?.trail;
  const topDiff = activity?.diffs[0];
  const pasteInto = topDiff ? trail?.pastes.filter((p) => p.external && p.target.replace(/^\/+/, "") === topDiff.path && p.chars >= 200) : [];
  const pastedLines = (pasteInto ?? []).reduce((n, p) => n + p.lines, 0);
  const lastTests = trail?.lastTests ?? null;
  const incomplete = report?.status === "ready" ? (report.evidence ?? []).filter((e) => e.category === "incomplete") : [];

  const tiles = [
    {
      icon: Clock,
      label: "Time worked",
      value: `${trail?.workMinutes ?? session?.elapsedMin ?? 0} min`,
      note: `of ${session?.durationMin ?? 60} min allowed`,
      tint: "bg-[#eaf1fe] text-[#2f6bde]",
      onClick: () => setActiveSection("workflow"),
    },
    {
      icon: FileText,
      label: "Files modified",
      value: activity?.diffs.length ?? trail?.editedPaths.length ?? 0,
      note: (activity?.diffs.map((d) => d.path) ?? trail?.editedPaths ?? []).slice(0, 2).join(", ") || undefined,
      tint: "bg-[#f4f0ff] text-[#7c3aed]",
      onClick: () => setActiveSection("code"),
    },
    {
      icon: Play,
      label: "Tests",
      value: lastTests && lastTests.passed !== null ? `${lastTests.passed} / ${lastTests.passed + (lastTests.failed ?? 0)}` : "—",
      note: lastTests ? (
        <span className={lastTests.failed ? "text-[#b45309] font-medium" : "text-[#15803d] font-medium"}>
          {lastTests.failed ? `${lastTests.failed} failing` : "all passed"}
        </span>
      ) : (
        "none run"
      ),
      tint: "bg-[#e8f8ee] text-[#15a34a]",
      onClick: () => setActiveSection("code"),
    },
    {
      icon: SquareTerminal,
      label: "Terminal commands",
      value: trail?.terminalCommands ?? 0,
      tint: "bg-[#f4f0ff] text-[#7c3aed]",
      onClick: () => setActiveSection("workflow"),
    },
    {
      icon: Flag,
      label: "Workspace flags",
      value: (
        <span className={`inline-flex items-center gap-1.5 ${trail?.flags.length || flags.length ? "text-[#e11d48]" : "text-[#15803d]"}`}>
          {trail?.flags.length || flags.length ? <Flag size={16} className="fill-current" /> : null}
          {trail?.flags.length || flags.length ? `${Math.max(trail?.flags.length ?? 0, flags.length)} flagged` : "Clean"}
        </span>
      ),
      tint: "bg-[#feeef0] text-[#e11d48]",
      onClick: () => setActiveSection("workflow"),
    },
  ];

  return (
    <div className="space-y-6">
      {/* Section navigation tabs */}
      <nav
        className="flex gap-1.5 overflow-x-auto rounded-2xl border border-hair bg-surface p-1.5 shadow-sm [scrollbar-width:none] print:hidden"
        aria-label="Report sections"
      >
        {SECTIONS.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            type="button"
            onClick={() => setActiveSection(id)}
            className={`relative flex shrink-0 items-center gap-2 rounded-xl px-4 py-2.5 text-[13.5px] font-medium transition-all ${
              activeSection === id
                ? "bg-[#7c3aed] text-white shadow-sm font-semibold"
                : "text-dim hover:text-ink hover:bg-surface-2"
            }`}
          >
            <Icon size={17} strokeWidth={2} />
            <span>{label}</span>
          </button>
        ))}
      </nav>

      {/* ── Summary Tab ── */}
      {activeSection === "summary" && (
        <div className="space-y-6">
          <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
            <RecommendationCard report={report} />
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
              {tiles.map((t) => (
                <button
                  key={t.label}
                  type="button"
                  onClick={t.onClick}
                  className="relative flex flex-col text-left rounded-2xl border border-hair bg-surface p-4 shadow-sm transition-all hover:border-hair-bright hover:bg-surface-2 cursor-pointer"
                >
                  <span className={`flex h-9 w-9 items-center justify-center rounded-xl ${t.tint}`}>
                    <t.icon size={18} />
                  </span>
                  <p className="mt-3 text-[12.5px] font-medium text-dim">{t.label}</p>
                  <p className="mt-0.5 text-[20px] font-bold text-ink tabular-nums">{t.value}</p>
                  {t.note && <p className="mt-0.5 truncate text-[11.5px] text-dim">{t.note}</p>}
                </button>
              ))}
            </div>
          </div>

          <div className="grid gap-6 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)] xl:items-start">
            <div className="space-y-6">
              <Panel
                icon={<Code2 size={20} />}
                title="Code changes preview"
                action={
                  topDiff ? (
                    <button
                      type="button"
                      onClick={() => setActiveSection("code")}
                      className="inline-flex items-center gap-1 text-[13.5px] font-semibold text-[#7c3aed] hover:underline"
                    >
                      View full diff ({activity?.diffs.length ?? 0} files) <ChevronRight size={15} />
                    </button>
                  ) : undefined
                }
              >
                {!topDiff ? (
                  <p className="text-[13.5px] text-dim">
                    {activity?.finalFiles
                      ? "The files submitted match the task starting files (no changes made)."
                      : "No workspace snapshot was recorded for this session."}
                  </p>
                ) : (
                  <>
                    <div className="mb-3 flex flex-wrap items-center gap-2">
                      <span className="font-mono text-[13.5px] font-bold text-ink flex items-center gap-1.5">
                        <FileText size={15} /> {topDiff.path}
                      </span>
                      {pastedLines > 0 && <Pill tone="red">External paste: {pastedLines} lines</Pill>}
                      <Pill tone="green">+{topDiff.added} lines</Pill>
                      <Pill tone="red">-{topDiff.removed} lines</Pill>
                      {(activity?.diffs.length ?? 0) > 1 && (
                        <Pill tone="gray">+{activity!.diffs.length - 1} more files</Pill>
                      )}
                    </div>
                    <DiffView
                      diff={topDiff}
                      window={12}
                      pasteBadge={pastedLines > 0 ? <Pill tone="green">+{pastedLines} lines pasted</Pill> : undefined}
                    />
                  </>
                )}
              </Panel>

              <Panel icon={<ListChecks size={20} />} title="Test results summary">
                <TestResults run={lastTests} />
              </Panel>
            </div>

            <div className="space-y-6">
              <Panel
                icon={<Workflow size={20} />}
                title="Session activity timeline"
                action={
                  trail && trail.timeline.length > 0 ? (
                    <button
                      type="button"
                      onClick={() => setActiveSection("workflow")}
                      className="text-[13px] font-semibold text-[#7c3aed] hover:underline"
                    >
                      Full timeline
                    </button>
                  ) : undefined
                }
              >
                {trail && trail.timeline.length > 0 ? (
                  <Timeline items={shortTimeline(trail.timeline).items} />
                ) : (
                  <p className="text-[13.5px] text-dim">No activity events recorded yet.</p>
                )}
              </Panel>

              <Panel
                icon={<Flag size={18} />}
                title={
                  <span className="inline-flex items-center gap-2">
                    Flags
                    {trail && trail.flags.length > 0 && (
                      <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-[#e11d48] px-1.5 text-[11px] font-bold text-white">
                        {trail.flags.length}
                      </span>
                    )}
                  </span>
                }
                action={
                  (trail?.flags.length ?? 0) > 0 ? (
                    <button
                      type="button"
                      onClick={() => setActiveSection("workflow")}
                      className="text-[13px] font-semibold text-[#7c3aed] hover:underline"
                    >
                      View all
                    </button>
                  ) : undefined
                }
              >
                <FlagList flags={trail?.flags.slice(0, 4) ?? []} onSelect={() => setActiveSection("workflow")} />
              </Panel>
            </div>
          </div>

          {incomplete.length > 0 && (
            <Panel title="Missing / Incomplete in this report">
              <Observations items={incomplete} empty="" />
            </Panel>
          )}


        </div>
      )}

      {/* ── Code Evaluation Tab ── */}
      {activeSection === "code" && (
        <div className="space-y-6">
          <Panel icon={<Code2 size={20} />} title="What code evaluation found">
            <Observations
              items={pick("code")}
              empty={ready ? "The code evaluation recorded no observations." : "Evaluation in progress."}
            />
          </Panel>

          <Panel icon={<FileText size={20} />} title={`Files changed (${activity?.diffs.length ?? 0})`}>
            {!activity?.diffs.length ? (
              <p className="text-[13.5px] text-dim">
                {activity?.finalFiles
                  ? "The submitted files match the starter files without changes."
                  : "No final workspace files recorded."}
              </p>
            ) : (
              <div className="space-y-6">
                {activity.diffs.map((d) => (
                  <div key={d.path} className="space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-mono text-[14px] font-bold text-ink">{d.path}</span>
                      {!d.beforeExists && <Pill tone="violet">New file</Pill>}
                      {!d.afterExists && <Pill tone="gray">Deleted</Pill>}
                      <Pill tone="green">+{d.added} lines</Pill>
                      <Pill tone="red">-{d.removed} lines</Pill>
                    </div>
                    <div className="max-h-[580px] overflow-auto rounded-xl">
                      <DiffView diff={d} />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </Panel>

          <Panel icon={<Play size={20} />} title={`Test runs (${activity?.trail.testRuns.length ?? 0})`}>
            {!activity?.trail.testRuns.length ? (
              <p className="text-[13.5px] text-dim">No test runs were executed during this session.</p>
            ) : (
              <ul className="divide-y divide-hair">
                {activity.trail.testRuns.map((r) => (
                  <li key={r.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 py-3">
                    <span className="w-14 font-mono text-[12.5px] text-dim">{clock(r.offset)}</span>
                    <code className="min-w-0 flex-1 truncate font-mono text-[13px] text-ink">{r.command}</code>
                    {r.passed === null ? (
                      <Pill tone="gray">exit {r.exitCode}</Pill>
                    ) : (
                      <Pill tone={r.failed ? "amber" : "green"}>
                        {r.passed} passed · {r.failed} failed
                      </Pill>
                    )}
                    {r.failing.length > 0 && (
                      <p className="w-full pl-[72px] text-[12px] font-medium text-[#b45309]">
                        Failing: {r.failing.join(", ")}
                      </p>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Panel>
        </div>
      )}

      {/* ── Workflow Tab ── */}
      {activeSection === "workflow" && (
        <div className="grid gap-6 xl:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
          <div className="space-y-6">
            <Panel icon={<Workflow size={20} />} title="Workflow observations">
              <Observations
                items={pick("workflow")}
                empty={ready ? "The workflow evaluation recorded no observations." : "Evaluation in progress."}
              />
            </Panel>

            <Panel icon={<Clock size={20} />} title="Complete activity timeline">
              {trail && trail.timeline.length > 0 ? (
                <Timeline items={trail.timeline} />
              ) : (
                <p className="text-[13.5px] text-dim">No activity events recorded.</p>
              )}
            </Panel>
          </div>

          <div className="space-y-6">
            <Panel icon={<Flag size={18} />} title={`Flags (${trail?.flags.length ?? 0})`}>
              <FlagList flags={trail?.flags ?? []} />
              <p className="mt-3 text-[12px] text-dim">
                Each flagged moment was recorded with camera and screen telemetry. A flag is a prompt to inspect, not a negative verdict.
              </p>
            </Panel>

            <Panel icon={<SquareTerminal size={18} />} title="Session telemetry at a glance">
              <dl className="grid grid-cols-2 gap-3 text-[13.5px]">
                {[
                  ["Terminal commands", trail?.terminalCommands ?? 0],
                  ["Test runs", trail?.testRuns.length ?? 0],
                  ["Pastes", trail?.pastes.length ?? 0],
                  ["Time away", `${trail?.awayMinutes ?? 0} min`],
                  ["AI Assistant questions", trail?.assistant.filter((t) => t.role === "candidate").length ?? 0],
                  ["Files modified", trail?.editedPaths.length ?? 0],
                ].map(([k, v]) => (
                  <div key={String(k)} className="rounded-xl border border-hair bg-surface-2 px-3 py-2.5">
                    <dt className="text-[12px] text-dim">{k}</dt>
                    <dd className="mt-0.5 text-[17px] font-bold text-ink tabular-nums">{v}</dd>
                  </div>
                ))}
              </dl>
            </Panel>
          </div>
        </div>
      )}

      {/* ── Reasoning Tab ── */}
      {activeSection === "reasoning" && (
        <div className="space-y-6">
          <Panel icon={<Lightbulb size={20} />} title="Understanding and approach">
            <Observations
              items={pick("reasoning")}
              empty={ready ? "The reasoning evaluation recorded no observations." : "Evaluation in progress."}
            />
          </Panel>

          <Panel
            icon={<UserRound size={20} />}
            title={`Conversation with workspace AI assistant (${
              activity?.trail.assistant.filter((t) => t.role === "candidate").length ?? 0
            } queries)`}
          >
            {!activity?.trail.assistant.length ? (
              <p className="text-[13.5px] text-dim">The candidate did not use the AI assistant in this session.</p>
            ) : (
              <Conversation turns={activity.trail.assistant} you="Candidate" them="AI Assistant" />
            )}
          </Panel>
        </div>
      )}

      {/* ── Interview & Recordings Tab ── */}
      {activeSection === "interview" && (
        <div className="space-y-6">
          <Panel icon={<MessageSquare size={20} />} title="What the interview showed">
            <Observations
              items={pick("interview")}
              empty={ready ? "The interview evaluation recorded no observations." : "Evaluation in progress."}
            />
          </Panel>

          <Panel
            icon={<Video size={20} />}
            title={`Interview Q&A with Recordings (${interview.length} questions)`}
          >
            {interview.length === 0 ? (
              <p className="text-[13.5px] text-dim">No interview transcript or recordings recorded for this session.</p>
            ) : (
              <div className="divide-y divide-hair">
                {interview.map((x) => (
                  <div key={x.number} className="py-4 space-y-3 first:pt-0 last:pb-0">
                    <div className="flex items-start justify-between gap-3">
                      <p className="text-[14.5px] font-bold text-ink">
                        <span className="font-mono text-xs text-[#7c3aed] mr-2 px-1.5 py-0.5 rounded bg-[#f4f0ff]">
                          Q{x.number}
                        </span>
                        {x.question}
                      </p>
                      {x.recording && (
                        <span className="inline-flex items-center gap-1 text-[11.5px] font-semibold text-[#15a34a] bg-[#e8f8ee] px-2 py-0.5 rounded-full shrink-0">
                          <Video size={12} /> Recording available
                        </span>
                      )}
                    </div>

                    <div className="rounded-xl border border-hair bg-surface-2 p-3.5">
                      <div className="text-[11.5px] font-semibold text-dim uppercase tracking-wider mb-1">
                        Candidate Answer
                      </div>
                      {x.answer === null ? (
                        <p className="text-[13.5px] text-dim italic">
                          Not answered — session concluded before this question was completed.
                        </p>
                      ) : (
                        <p className={`text-[14px] leading-relaxed whitespace-pre-line ${x.unanswered ? "text-dim italic" : "text-ink"}`}>
                          {x.answer}
                        </p>
                      )}
                    </div>

                    {/* Telemetry info */}
                    <div className="flex flex-wrap items-center gap-3 text-[12px] text-dim">
                      {x.seconds !== null && !x.unanswered && <span>Response time: {x.seconds}s</span>}
                      {x.timedOut && !x.unanswered && <span className="font-semibold text-[#e11d48]">Over time limit</span>}
                      {x.unanswered && <span className="font-semibold text-[#e11d48]">Timed out</span>}
                      {x.recordingExpired && <span className="text-dim">Recording expired (retained for 90 days)</span>}
                    </div>

                    {/* Media playback */}
                    {x.recording && (
                      <div className="mt-2 rounded-xl border border-hair bg-black/5 p-3">
                        <div className="flex items-center gap-2 mb-2 text-xs font-semibold text-ink">
                          {x.recording.kind === "video" ? <Video size={14} className="text-[#7c3aed]" /> : <Mic size={14} className="text-[#7c3aed]" />}
                          <span>Recorded Answer Playback ({x.recording.kind})</span>
                        </div>
                        {x.recording.kind === "video" ? (
                          <video
                            controls
                            preload="none"
                            src={x.recording.url}
                            className="w-full max-w-lg rounded-lg bg-black aspect-video shadow"
                          />
                        ) : (
                          <audio controls preload="none" src={x.recording.url} className="w-full max-w-md" />
                        )}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </Panel>
        </div>
      )}

      {/* ── Artifacts Tab ── */}
      {activeSection === "artifacts" && (
        <div className="grid gap-6 xl:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
          <Panel
            icon={<Folder size={20} />}
            title={`Files submitted by candidate (${activity?.finalFiles ? Object.keys(activity.finalFiles).length : 0})`}
          >
            {!activity?.finalFiles || Object.keys(activity.finalFiles).length === 0 ? (
              <p className="text-[13.5px] text-dim">No final workspace files recorded.</p>
            ) : (
              <div className="space-y-3">
                {activity.finalAt && (
                  <p className="text-[12.5px] text-dim">Snapshot captured at {activity.finalAt}.</p>
                )}
                <div className="space-y-2">
                  {Object.entries(activity.finalFiles)
                    .sort(([a], [b]) => a.localeCompare(b))
                    .map(([path, text]) => (
                      <details key={path} className="group rounded-xl border border-hair bg-surface">
                        <summary className="flex cursor-pointer list-none items-center gap-2.5 px-4 py-3 text-[13.5px] font-medium text-ink hover:bg-surface-2 transition-colors rounded-xl">
                          <ChevronRight size={16} className="text-dim transition-transform group-open:rotate-90" />
                          <span className="min-w-0 flex-1 truncate font-mono text-sm">{path.replace(/^\/+/, "")}</span>
                          <span className="text-[12px] text-dim">{String(text).split("\n").length} lines</span>
                        </summary>
                        <pre className="max-h-[460px] overflow-auto border-t border-hair bg-surface-2 px-4 py-3 font-mono text-[12px] leading-[1.6] text-ink">
                          {String(text)}
                        </pre>
                      </details>
                    ))}
                </div>
              </div>
            )}
          </Panel>

          <Panel icon={<Video size={20} />} title="Recordings & Telemetry kept">
            <ul className="space-y-3 text-[13.5px] text-ink">
              <li className="flex items-center justify-between border-b border-hair pb-2">
                <span className="flex items-center gap-2">
                  <Camera size={16} className="text-[#7c3aed]" /> Camera stills
                </span>
                <span className="font-bold tabular-nums">{camera.total || activity?.trail.media.cameraStills || 0}</span>
              </li>
              <li className="flex items-center justify-between border-b border-hair pb-2">
                <span className="flex items-center gap-2">
                  <Monitor size={16} className="text-[#7c3aed]" /> Screen stills
                </span>
                <span className="font-bold tabular-nums">{screen.total || activity?.trail.media.screenStills || 0}</span>
              </li>
              <li className="flex items-center justify-between border-b border-hair pb-2">
                <span className="flex items-center gap-2">
                  <Mic size={16} className="text-[#7c3aed]" /> Audio flagged clips
                </span>
                <span className="font-bold tabular-nums">{activity?.trail.media.audioClips || flags.filter((f) => f.audioUrl).length || 0}</span>
              </li>
              <li className="flex items-center justify-between border-b border-hair pb-2">
                <span className="flex items-center gap-2">
                  <Video size={16} className="text-[#7c3aed]" /> Interview recordings
                </span>
                <span className="font-bold tabular-nums">
                  {interview.filter((x) => x.recording).length || activity?.trail.media.interviewRecordings || 0}
                </span>
              </li>
            </ul>
            <p className="mt-4 text-[12px] text-dim">
              All telemetry files are cryptographically signed and stored in compliance with the candidate consent policy.
            </p>
          </Panel>
        </div>
      )}

      {/* ── Proctoring & Snapshots Tab ── */}
      {activeSection === "proctoring" && (
        <div className="space-y-6">
          {/* Flagged Moments with Camera, Screen & Audio */}
          <Panel
            icon={<ShieldAlert size={20} className="text-[#e11d48]" />}
            title={`Flagged moments with evidence (${flags.length})`}
          >
            {flags.length === 0 ? (
              <p className="text-[13.5px] text-[#15803d]">No moments were flagged by proctoring checks.</p>
            ) : (
              <div className="space-y-4">
                <p className="text-[12.5px] text-dim">
                  Live proctoring captures camera stills, screen captures, and microphone audio whenever an anomalous event occurs.
                </p>
                <div className="space-y-3">
                  {flags.map((f, i) => (
                    <div key={i} className="rounded-xl border border-hair bg-surface p-4 space-y-3 shadow-sm">
                      <div className="flex flex-wrap items-baseline gap-2 text-sm">
                        <span className="font-mono text-xs text-dim bg-surface-2 px-2 py-0.5 rounded">{clock(f.offsetSeconds)}</span>
                        <span className="font-bold text-ink">{f.label}</span>
                        {f.detail && <span className="text-xs text-dim">({f.detail})</span>}
                        {f.lowConfidence && <span className="text-xs font-semibold text-[#e11d48]">low confidence signal</span>}
                      </div>
                      <div className="flex flex-wrap items-start gap-4">
                        {f.cameraUrl && (
                          <div className="space-y-1">
                            <span className="block text-[11px] font-semibold text-dim">Webcam Still</span>
                            {/* eslint-disable-next-line @next/next/no-img-element */}
                            <img
                              src={f.cameraUrl}
                              alt={`Camera at ${clock(f.offsetSeconds)}`}
                              className="h-28 rounded-lg border border-hair object-cover shadow-sm bg-black"
                            />
                          </div>
                        )}
                        {f.screenUrl && (
                          <div className="space-y-1">
                            <span className="block text-[11px] font-semibold text-dim">Screen Capture</span>
                            <a href={f.screenUrl} target="_blank" rel="noreferrer" title="Open full size">
                              {/* eslint-disable-next-line @next/next/no-img-element */}
                              <img
                                src={f.screenUrl}
                                alt={`Screen at ${clock(f.offsetSeconds)}`}
                                className="h-28 rounded-lg border border-hair object-cover shadow-sm bg-black hover:opacity-90 transition-opacity"
                              />
                            </a>
                          </div>
                        )}
                        {f.audioUrl && (
                          <div className="space-y-1">
                            <span className="block text-[11px] font-semibold text-dim">Audio Clip</span>
                            <audio controls preload="none" src={f.audioUrl} className="h-10 mt-2" />
                          </div>
                        )}
                        {!f.cameraUrl && !f.screenUrl && !f.audioUrl && (
                          <span className="text-xs text-dim">No clip retained or telemetry deleted after 90 days.</span>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </Panel>

          {/* Camera Timeline */}
          {camera.stills.length > 0 && (
            <Panel icon={<Camera size={20} />} title={`Candidate webcam timeline (${camera.total} stills)`}>
              <div className="flex gap-2.5 overflow-x-auto pb-2 [scrollbar-width:thin]">
                {camera.stills.map((s, i) => (
                  <div key={i} className="shrink-0 text-center space-y-1">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={s.url}
                      alt={`Camera at ${clock(s.offsetSeconds)}`}
                      className="h-24 w-32 rounded-lg border border-hair object-cover bg-black"
                    />
                    <span className="block font-mono text-[11px] text-dim">{clock(s.offsetSeconds)}</span>
                  </div>
                ))}
              </div>
            </Panel>
          )}

          {/* Screen Timeline */}
          {screen.stills.length > 0 && (
            <Panel icon={<Monitor size={20} />} title={`Screen capture timeline (${screen.total} stills)`}>
              <div className="flex gap-2.5 overflow-x-auto pb-2 [scrollbar-width:thin]">
                {screen.stills.map((s, i) => (
                  <a key={i} href={s.url} target="_blank" rel="noreferrer" className="shrink-0 text-center space-y-1 hover:opacity-90">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={s.url}
                      alt={`Screen at ${clock(s.offsetSeconds)}`}
                      className="h-24 w-36 rounded-lg border border-hair object-cover bg-black"
                    />
                    <span className="block font-mono text-[11px] text-dim">{clock(s.offsetSeconds)}</span>
                  </a>
                ))}
              </div>
            </Panel>
          )}
        </div>
      )}
    </div>
  );
}

function RecommendationCard({ report }: { report: AssessmentReport | null }) {
  if (!report || report.status !== "ready") {
    return (
      <div className="rounded-2xl border border-hair bg-surface p-5 shadow-sm">
        <div className="flex items-center gap-2">
          {report?.status === "generating" && <span className="h-2.5 w-2.5 animate-pulse rounded-full bg-[#7c3aed]" />}
          <p className="text-[15px] font-bold text-ink">
            {report?.status === "generating" ? "Generating AI report…" : "Report not completed"}
          </p>
        </div>
        <p className="mt-2 text-[13.5px] text-dim">
          {report?.status === "failed" ? report.error ?? "Evaluation did not complete." : "Evaluation agents are processing session telemetry."}
        </p>
      </div>
    );
  }

  const rec = report.recommendation ? RECOMMENDATION[report.recommendation] : undefined;
  const tone = rec?.tone ?? "none";

  return (
    <div
      className={`rounded-2xl border p-5 shadow-sm ${
        tone === "bad"
          ? "border-[#fecdd3] bg-[#fff1f2]"
          : tone === "warn"
          ? "border-[#fde68a] bg-[#fff8eb]"
          : tone === "good"
          ? "border-[#bbf0cc] bg-[#f0fbf4]"
          : "border-hair bg-surface"
      }`}
    >
      <div className="flex items-start gap-3.5">
        <span
          className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-white shadow-sm ${
            tone === "bad" ? "text-[#e11d48]" : tone === "warn" ? "text-[#d97706]" : tone === "good" ? "text-[#15a34a]" : "text-[#7c3aed]"
          }`}
        >
          <UserRound size={22} />
        </span>
        <div className="min-w-0">
          <p className="text-[13px] font-bold uppercase tracking-wider text-dim">AI Recommendation</p>
          <p
            className={`text-[24px] leading-tight font-extrabold ${
              tone === "bad" ? "text-[#e11d48]" : tone === "warn" ? "text-[#d97706]" : tone === "good" ? "text-[#15803d]" : "text-ink"
            }`}
          >
            {rec?.label ?? report.recommendation ?? "Not specified"}
          </p>
        </div>
      </div>
      {report.summary && (
        <div className="mt-3.5 rounded-xl bg-white/90 p-3.5 border border-hair">
          <p className="text-[12.5px] font-bold text-ink uppercase tracking-wide">Key Reasoning</p>
          <p className="mt-1 text-[13.5px] leading-relaxed text-dim">{report.summary.replace(/ ?\[E\d+\]/g, "")}</p>
        </div>
      )}
    </div>
  );
}

function Conversation({
  turns,
  you,
  them,
}: {
  turns: Array<{ id: number; offset: number; role: "candidate" | "other"; text: string }>;
  you: string;
  them: string;
}) {
  return (
    <ol className="space-y-3">
      {turns.map((t) => (
        <li
          key={t.id}
          className={`max-w-[85%] rounded-2xl px-4 py-3 text-sm ${
            t.role === "candidate" ? "ml-auto bg-[#f4f0ff] border border-[#ddd6fe]" : "bg-surface-2 border border-hair"
          }`}
        >
          <p className="text-[11.5px] font-semibold text-dim">
            {t.role === "candidate" ? you : them} · {clock(t.offset)}
          </p>
          <p className="mt-1 leading-relaxed whitespace-pre-line text-ink">{t.text}</p>
        </li>
      ))}
    </ol>
  );
}
