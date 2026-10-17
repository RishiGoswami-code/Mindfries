"use client";

import { useState } from "react";
import clsx from "clsx";
import { idePalette } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";
import type { VfsBridge } from "@/lib/ide/vfs-bridge";
import type { PreviewController } from "@/lib/ide/shell/types";
import { TerminalGroup } from "./TerminalGroup";
import { Bug, CircleAlert, FileDiff, Network, ScrollText, SquareTerminal, X, type LucideIcon } from "lucide-react";
import { ProblemsPanel } from "./ProblemsPanel";
import { OutputPanel } from "./OutputPanel";
import { DebugConsolePanel } from "./DebugConsolePanel";
import { PortsPanel } from "./PortsPanel";
import { ChangesPanel } from "./ChangesPanel";
import type { FileChange } from "@/lib/ide/changes";
import type { Diagnostic } from "@/lib/ide/diagnostics";

type PanelTab = "terminal" | "output" | "debug" | "problems" | "changes" | "ports";

const TABS: { id: PanelTab; label: string; icon: LucideIcon }[] = [
  { id: "terminal", label: "Terminal", icon: SquareTerminal },
  { id: "output", label: "Output", icon: ScrollText },
  { id: "debug", label: "Debug Console", icon: Bug },
  { id: "problems", label: "Problems", icon: CircleAlert },
  { id: "changes", label: "Changes", icon: FileDiff },
  { id: "ports", label: "Ports", icon: Network },
];

/**
 * The Terminal card under the editor: Terminal, Output and Debug Console,
 * then Problems, Changes and Ports.
 *
 * Its cross tucks the card away (IdeShell floats a chip over the camera
 * picture that brings it back). Every panel stays mounted while another tab is
 * shown, and while the card is tucked away: terminals and the debug
 * console keep their sessions and scrollback.
 */
export function BottomPanel({
  theme,
  vfs,
  preview,
  onClose,
  diagnostics,
  onOpenLocation,
  changes,
  onOpenDiff,
  previewState,
  onFocusPreview,
  onStopPreview,
}: {
  theme: IdeTheme;
  vfs: VfsBridge;
  preview: PreviewController;
  /** Tucks the card away. */
  onClose: () => void;
  diagnostics: Diagnostic[];
  onOpenLocation: (path: string, line: number) => void;
  /** Files that differ from the task as given, and how to open one as a diff. */
  changes: FileChange[];
  onOpenDiff: (path: string) => void;
  previewState: { html: string; title: string; root: string; watching: boolean } | null;
  /** Shows the preview, which opens in the editor beside the code. */
  onFocusPreview: () => void;
  onStopPreview: () => void;
}) {
  const palette = idePalette(theme);
  const [active, setActive] = useState<PanelTab>("terminal");
  const dark = theme === "dark";

  return (
    <div className={clsx("flex h-full flex-col", palette.panelBg)}>
      <div className={clsx("flex h-12 shrink-0 items-stretch gap-1 overflow-x-auto border-b px-2 [scrollbar-width:none]", palette.border)}>
        {TABS.map((tab) => {
          const selected = active === tab.id;
          return (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActive(tab.id)}
              aria-pressed={selected}
              className={clsx(
                "relative flex shrink-0 items-center gap-2 px-3 text-[13.5px] font-medium transition-colors",
                selected ? clsx(palette.text, dark ? "bg-[#2F6BFF]/10" : "") : clsx(palette.textMuted, dark ? "hover:text-[#E2E8F0]" : "hover:text-[#0A1931]")
              )}
            >
              <tab.icon size={15} />
              {tab.label}
              {tab.id === "changes" && changes.length > 0 && (
                <span className={clsx("rounded-full px-1.5 text-[10.5px]", dark ? "bg-[#2F6BFF]/25 text-[#93C5FD]" : "bg-[#4A7FA7]/30")}>{changes.length}</span>
              )}
              {tab.id === "problems" && diagnostics.length > 0 && (
                <span className={clsx("rounded-full px-1.5 text-[10.5px]", dark ? "bg-[#2F6BFF]/25 text-[#93C5FD]" : "bg-[#4A7FA7]/30")}>{diagnostics.length}</span>
              )}
              <span className={clsx("absolute inset-x-2 bottom-0 h-[2px] rounded-full transition-opacity", dark ? "bg-[#3B82F6]" : "bg-[#4A7FA7]", selected ? "opacity-100" : "opacity-0")} />
            </button>
          );
        })}
        <button
          type="button"
          onClick={onClose}
          title="Close the terminal card (it keeps running)"
          aria-label="Close the terminal card"
          className={clsx("my-auto ml-auto flex h-8 w-8 shrink-0 items-center justify-center rounded-lg", palette.textMuted, palette.hover)}
        >
          <X size={16} />
        </button>
      </div>

      <div className="flex min-h-0 flex-1">
        <div className="min-h-0 min-w-0 flex-1">
          {/* Kept mounted (just hidden) rather than unmounted while another tab is
              active, so terminal sessions and their scrollback survive tab switches. */}
          <div className="h-full" style={{ display: active === "terminal" ? "block" : "none" }}>
            <TerminalGroup theme={theme} vfs={vfs} preview={preview} />
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
          {active === "changes" && <ChangesPanel theme={theme} changes={changes} onOpenDiff={onOpenDiff} />}
          {active === "ports" && (
            <PortsPanel
              theme={theme}
              preview={previewState}
              onFocusPreview={onFocusPreview}
              onStopPreview={onStopPreview}
            />
          )}
        </div>

      </div>
    </div>
  );
}
