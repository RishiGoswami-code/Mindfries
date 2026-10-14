"use client";

import { useState, useSyncExternalStore } from "react";
import clsx from "clsx";
import { idePalette } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";
import type { VfsBridge } from "@/lib/ide/vfs-bridge";
import type { PreviewController } from "@/lib/ide/shell/types";
import { TerminalGroup } from "./TerminalGroup";
import { ChevronUp } from "lucide-react";
import { ProblemsPanel } from "./ProblemsPanel";
import { OutputPanel } from "./OutputPanel";
import { DebugConsolePanel } from "./DebugConsolePanel";
import { PortsPanel } from "./PortsPanel";
import { TestsPanel } from "./TestsPanel";
import { ChangesPanel } from "./ChangesPanel";
import type { FileChange } from "@/lib/ide/changes";
import { testResults } from "@/lib/ide/test-results";
import type { Diagnostic } from "@/lib/ide/diagnostics";

type PanelTab = "problems" | "output" | "debug" | "terminal" | "tests" | "changes" | "ports";

const TABS: { id: PanelTab; label: string }[] = [
  { id: "problems", label: "Problems" },
  { id: "output", label: "Output" },
  { id: "debug", label: "Debug Console" },
  { id: "terminal", label: "Terminal" },
  { id: "tests", label: "Tests" },
  { id: "changes", label: "Changes" },
  { id: "ports", label: "Ports" },
];

/**
 * The task drawer under the editor: Problems, Output, Debug Console,
 * Terminal, Tests, Changes and Ports.
 *
 * It starts collapsed to its tab bar, so the editor has the room. Choosing a
 * tab opens it upward; choosing the open tab again, or the chevron, closes
 * it; the strip along its top edge drags it to any height (IdeShell owns the
 * height and the open state). Every panel stays mounted while it is closed —
 * terminals and the debug console keep their sessions and scrollback.
 */
export function BottomPanel({
  theme,
  vfs,
  preview,
  open,
  onOpenChange,
  diagnostics,
  onOpenLocation,
  changes,
  onOpenDiff,
  previewState,
  onClosePreview,
  onStopPreview,
}: {
  theme: IdeTheme;
  vfs: VfsBridge;
  preview: PreviewController;
  /** Whether the drawer is open (showing a panel) or collapsed to its tab bar. */
  open: boolean;
  onOpenChange: (open: boolean) => void;
  diagnostics: Diagnostic[];
  onOpenLocation: (path: string, line: number) => void;
  /** Files that differ from the task as given, and how to open one as a diff. */
  changes: FileChange[];
  onOpenDiff: (path: string) => void;
  previewState: { html: string; title: string; root: string; watching: boolean } | null;
  onClosePreview: () => void;
  onStopPreview: () => void;
}) {
  const palette = idePalette(theme);
  const [active, setActive] = useState<PanelTab>("terminal");
  const lastTestRun = useSyncExternalStore(testResults.subscribe, testResults.getSnapshot, testResults.getSnapshot);
  const dark = theme === "dark";

  const choose = (tab: PanelTab) => {
    if (open && tab === active) {
      onOpenChange(false);
      return;
    }
    setActive(tab);
    onOpenChange(true);
  };

  return (
    <div className={clsx("flex h-full flex-col", palette.panelBg)}>
      <div className={clsx("flex h-11 shrink-0 items-stretch gap-1 overflow-x-auto px-2 [scrollbar-width:none]", open && "border-b", palette.border)}>
        {TABS.map((tab) => {
          const selected = open && active === tab.id;
          return (
            <button
              key={tab.id}
              type="button"
              onClick={() => choose(tab.id)}
              aria-expanded={selected}
              className={clsx(
                "relative flex shrink-0 items-center gap-1.5 px-3 text-[13px] font-medium transition-colors",
                selected ? palette.text : clsx(palette.textMuted, dark ? "hover:text-[#E7EAF6]" : "hover:text-[#0A1931]")
              )}
            >
              {tab.label}
              {tab.id === "tests" && lastTestRun?.parsed && (
                <span
                  className={clsx(
                    "rounded-full px-1.5 text-[10.5px] font-semibold",
                    lastTestRun.failed > 0 ? "bg-red-500/25 text-red-400" : "bg-emerald-500/20 text-emerald-400",
                  )}
                >
                  {lastTestRun.failed > 0 ? lastTestRun.failed : "✓"}
                </span>
              )}
              {tab.id === "changes" && changes.length > 0 && (
                <span className={clsx("rounded-full px-1.5 text-[10.5px]", dark ? "bg-[#7C5CFF]/25 text-[#C4B5FD]" : "bg-[#4A7FA7]/30")}>{changes.length}</span>
              )}
              {tab.id === "problems" && diagnostics.length > 0 && (
                <span className={clsx("rounded-full px-1.5 text-[10.5px]", dark ? "bg-[#7C5CFF]/25 text-[#C4B5FD]" : "bg-[#4A7FA7]/30")}>{diagnostics.length}</span>
              )}
              {/* The open tab's violet underline. */}
              <span
                className={clsx(
                  "absolute inset-x-2 bottom-0 h-[2px] rounded-full transition-opacity",
                  dark ? "bg-[#8B6CFF]" : "bg-[#4A7FA7]",
                  selected ? "opacity-100" : "opacity-0"
                )}
              />
            </button>
          );
        })}
        <button
          type="button"
          onClick={() => onOpenChange(!open)}
          title={open ? "Close the panel" : "Open the panel"}
          aria-label={open ? "Close the panel" : "Open the panel"}
          className={clsx("ml-auto flex w-8 shrink-0 items-center justify-center rounded-lg", palette.textMuted, palette.hover)}
        >
          <ChevronUp size={15} className={clsx("transition-transform duration-200", open && "rotate-180")} />
        </button>
      </div>

      {/* Hidden, not unmounted, while the drawer is closed. */}
      <div className="flex min-h-0 flex-1" style={{ display: open ? "flex" : "none" }}>
        <div className="min-h-0 min-w-0 flex-1">
          {/* Kept mounted (just hidden) rather than unmounted while another tab is
              active, so terminal sessions and their scrollback survive tab switches. */}
          <div className="h-full" style={{ display: active === "terminal" ? "block" : "none" }}>
            <TerminalGroup
              theme={theme}
              vfs={vfs}
              preview={preview}
              previewState={previewState}
              onClosePreview={onClosePreview}
            />
          </div>
          {active === "problems" && (
            <ProblemsPanel theme={theme} diagnostics={diagnostics} onOpen={onOpenLocation} />
          )}
          {active === "output" && <OutputPanel theme={theme} />}
          {/* Kept mounted like the terminal: the REPL's scrollback and its
              declarations are a session, and switching tabs shouldn't end it. */}
          <div className="h-full" style={{ display: active === "debug" ? "block" : "none" }}>
            <DebugConsolePanel theme={theme} />
          </div>
          {active === "tests" && <TestsPanel theme={theme} vfs={vfs} preview={preview} />}
          {active === "changes" && <ChangesPanel theme={theme} changes={changes} onOpenDiff={onOpenDiff} />}
          {active === "ports" && (
            <PortsPanel
              theme={theme}
              preview={previewState}
              // The preview lives beside the terminals, so "focus" means
              // switching to the tab it's rendered on.
              onFocusPreview={() => setActive("terminal")}
              onStopPreview={onStopPreview}
            />
          )}
        </div>

      </div>
    </div>
  );
}
