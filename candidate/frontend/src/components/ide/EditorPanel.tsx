"use client";

import dynamic from "next/dynamic";
import type { ReactNode } from "react";
import clsx from "clsx";
import { X } from "lucide-react";
import { loader, type BeforeMount, type OnMount } from "@monaco-editor/react";
import { languageForPath } from "@/lib/ide/language";
import { idePalette, MONACO_DARK } from "@/lib/ide/palette";
import { FileTypeIcon } from "./FileTypeIcon";
import type { IdeTheme } from "@/lib/ide/theme";
import { NotebookEditor } from "./NotebookEditor";
import { DIFF_PREFIX, fileOfTab, isDiffTab } from "@/lib/ide/changes";

// Self-hosted instead of the library's CDN default (jsdelivr): assets are
// copied to public/monaco-editor/vs by scripts/copy-monaco.js, so the
// editor works offline and doesn't depend on a third party at runtime.
loader.config({ paths: { vs: "/monaco-editor/vs" } });

// Monaco touches `window`/`navigator` at import time, so it can only load
// on the client — never during SSR.
const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });
const MonacoDiffEditor = dynamic(() => import("@monaco-editor/react").then((m) => m.DiffEditor), { ssr: false });

interface EditorPanelProps {
  theme: IdeTheme;
  openPaths: string[];
  activePath: string | null;
  dirtyPaths: Set<string>;
  content: string;
  /**
   * For a diff tab (a path under DIFF_PREFIX): the file as the task gave it.
   * `content` is then the file as it stands now. Undefined when the file
   * didn't exist at the start.
   */
  original?: string;
  onSelectTab: (path: string) => void;
  onCloseTab: (path: string) => void;
  onChange: (path: string, value: string) => void;
  onSave: (path: string) => void;
  /** Shown under the tabs: where the open file is (Breadcrumbs). */
  breadcrumbs?: ReactNode;
}

/** Registers the editor's dark theme, matched to the workspace chrome. */
const defineThemes: BeforeMount = (monaco) => {
  monaco.editor.defineTheme("mindfries-dark", MONACO_DARK);
};

export function EditorPanel({
  theme,
  openPaths,
  activePath,
  dirtyPaths,
  content,
  original,
  onSelectTab,
  onCloseTab,
  onChange,
  onSave,
  breadcrumbs,
}: EditorPanelProps) {
  const handleMount: OnMount = (editor, monaco) => {
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
      if (activePath) onSave(activePath);
    });
  };

  const palette = idePalette(theme);
  const dark = theme === "dark";
  const monacoTheme = dark ? "mindfries-dark" : "light";

  return (
    <div className={clsx("flex h-full min-w-0 flex-1 flex-col", palette.appBg)}>
      <div className={clsx("flex h-11 shrink-0 items-stretch gap-1 overflow-x-auto border-b px-2 pt-1.5", palette.border, dark ? "bg-[#0E1431]" : "")}>
        {openPaths.map((path) => {
          const isActive = path === activePath;
          const isDirty = dirtyPaths.has(path);
          const diff = isDiffTab(path);
          const name = `${(fileOfTab(path) ?? path).split("/").pop() ?? path}${diff ? " (changes)" : ""}`;
          return (
            <div
              key={path}
              className={clsx(
                "group relative flex shrink-0 cursor-pointer items-center gap-2 rounded-t-lg px-3 text-[13px] transition-colors",
                isActive
                  ? clsx(palette.text, dark ? "bg-[#7C5CFF]/15" : palette.tabActiveBg)
                  : clsx(palette.textMuted, palette.hover)
              )}
              onClick={() => onSelectTab(path)}
              title={diff ? `Changes to ${path.slice(DIFF_PREFIX.length)} since the task started` : path}
            >
              <FileTypeIcon name={(fileOfTab(path) ?? path).split("/").pop() ?? path} size={14} />
              <span className="max-w-[160px] truncate">{name}</span>
              {isActive && <span className={clsx("absolute inset-x-2 bottom-0 h-[2px] rounded-full", dark ? "bg-[#8B6CFF]" : "bg-[#4A7FA7]")} />}
              <button
                type="button"
                className={clsx(
                  "flex h-4 w-4 items-center justify-center rounded-[3px]",
                  palette.hover
                )}
                onClick={(e) => {
                  e.stopPropagation();
                  onCloseTab(path);
                }}
              >
                {isDirty ? (
                  <span className="h-2 w-2 rounded-full bg-current group-hover:hidden" />
                ) : null}
                <X size={12} className={isDirty ? "hidden group-hover:block" : ""} />
              </button>
            </div>
          );
        })}
      </div>

      {breadcrumbs}

      <div className="min-h-0 flex-1">
        {isDiffTab(activePath) ? (
          // Read-only: this is a view of what changed. The file itself is
          // edited in its own tab.
          <MonacoDiffEditor
            key={activePath}
            original={original ?? ""}
            modified={content}
            language={languageForPath(activePath.slice(DIFF_PREFIX.length))}
            theme={monacoTheme}
            beforeMount={defineThemes}
            options={{ readOnly: true, renderSideBySide: true, fontSize: 13, automaticLayout: true, scrollBeyondLastLine: false, minimap: { enabled: false } }}
          />
        ) : activePath && languageForPath(activePath) === "ipynb" ? (
          <NotebookEditor
            key={activePath}
            theme={theme}
            content={content}
            onChange={(value) => onChange(activePath, value)}
            onSave={() => onSave(activePath)}
          />
        ) : activePath ? (
          <MonacoEditor
            key={activePath}
            path={activePath}
            language={languageForPath(activePath)}
            value={content}
            theme={monacoTheme}
            beforeMount={defineThemes}
            onChange={(value) => onChange(activePath, value ?? "")}
            onMount={handleMount}
            options={{
              fontSize: 14,
              lineHeight: 22,
              fontFamily: "'JetBrains Mono', 'Fira Code', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
              padding: { top: 12 },
              // A subtle minimap: blocks rather than characters, shown on hover of its slider.
              minimap: { enabled: true, renderCharacters: false, scale: 1, showSlider: "mouseover" },
              renderLineHighlight: "line",
              smoothScrolling: true,
              cursorBlinking: "smooth",
              automaticLayout: true,
              scrollBeyondLastLine: false,
              tabSize: 2,
            }}
          />
        ) : (
          <div className={clsx("flex h-full items-center justify-center text-sm", palette.textMuted)}>
            Select a file to start editing
          </div>
        )}
      </div>
    </div>
  );
}
