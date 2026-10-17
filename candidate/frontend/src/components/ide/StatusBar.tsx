import clsx from "clsx";
import { GitBranch, Bell, Moon, Sun, AlertCircle, AlertTriangle, Maximize2, Minimize2 } from "lucide-react";
import { useFullscreen } from "@/lib/ide/fullscreen";
import type { Diagnostic } from "@/lib/ide/diagnostics";
import { STATUS_BAR_BG } from "@/lib/ide/palette";
import { languageForPath, displayLanguageName } from "@/lib/ide/language";
import type { IdeTheme } from "@/lib/ide/theme";

interface StatusBarProps {
  activePath: string | null;
  dirtyCount: number;
  diagnostics: Diagnostic[];
  theme: IdeTheme;
  onToggleTheme: () => void;
}

export function StatusBar({ activePath, dirtyCount, diagnostics, theme, onToggleTheme }: StatusBarProps) {
  // Esc leaves fullscreen and the browser never asks first, so there has to
  // be a way back in that isn't "reload the page and start over".
  const fullscreen = useFullscreen();
  const errors = diagnostics.filter((d) => d.severity === "error").length;
  const warnings = diagnostics.filter((d) => d.severity === "warning").length;
  const language = activePath ? displayLanguageName(languageForPath(activePath)) : null;

  return (
    <div
      // Dark: quiet, in the chrome's own colours, so it doesn't compete with
      // the editor. Light: the brand-blue bar it has always been.
      className={clsx(
        "flex h-10 shrink-0 items-center justify-between px-5 text-[13px]",
        theme === "dark" ? "bg-[#0B1222] text-[#94A3B8]" : "text-white"
      )}
      style={theme === "dark" ? undefined : { backgroundColor: STATUS_BAR_BG }}
    >
      <div className="flex items-center gap-4">
        <span className="flex items-center gap-1.5">
          <GitBranch size={15} />
          main
        </span>
        <span className="flex items-center gap-1" title={`${errors} errors, ${warnings} warnings`}>
          <AlertCircle size={15} />
          {errors}
          <AlertTriangle size={15} className="ml-1" />
          {warnings}
        </span>
        {dirtyCount > 0 && (
          <span>
            {dirtyCount} unsaved file{dirtyCount === 1 ? "" : "s"}
          </span>
        )}
      </div>
      <div className="flex items-center gap-5">
        <span>Spaces: 2</span>
        <span>UTF-8</span>
        <span>LF</span>
        {language && <span>{language}</span>}
        <Bell size={15} />
        <button
          type="button"
          title={fullscreen.active ? "Exit fullscreen" : "Enter fullscreen"}
          onClick={fullscreen.toggle}
          className="flex items-center hover:opacity-80"
        >
          {fullscreen.active ? <Minimize2 size={15} /> : <Maximize2 size={15} />}
        </button>
        <button
          type="button"
          title={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
          onClick={onToggleTheme}
          className="flex items-center hover:opacity-80"
        >
          {theme === "dark" ? <Moon size={15} /> : <Sun size={15} />}
        </button>
      </div>
    </div>
  );
}
