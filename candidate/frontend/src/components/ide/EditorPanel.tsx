"use client";

import dynamic from "next/dynamic";
import type { ReactNode } from "react";
import clsx from "clsx";
import { Globe, X } from "lucide-react";
import { loader, type BeforeMount, type OnMount } from "@monaco-editor/react";
import { languageForPath } from "@/lib/ide/language";
import { idePalette, MONACO_DARK } from "@/lib/ide/palette";
import { FileTypeIcon } from "./FileTypeIcon";
import type { IdeTheme } from "@/lib/ide/theme";
import { NotebookEditor } from "./NotebookEditor";
import { DIFF_PREFIX, fileOfTab, isDiffTab, PREVIEW_TAB, TASK_TAB } from "@/lib/ide/changes";
import { TinyMarkdown } from "./tiny-markdown";
import { PreviewPanel } from "./PreviewPanel";

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
  /** The task brief, rendered in the TASK.md tab. */
  taskMarkdown: string;
  /** The live preview, rendered in its own tab beside the code. Null when nothing is being previewed. */
  previewState: { html: string; title: string; watching: boolean } | null;
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
  taskMarkdown,
  previewState,
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
      <div className={clsx("flex h-11 shrink-0 items-stretch gap-1 overflow-x-auto border-b px-2 pt-1.5", palette.border, dark ? "bg-[#0F172A]" : "")}>
        {openPaths.map((path) => {
          const isActive = path === activePath;
          const isDirty = dirtyPaths.has(path);
          const diff = isDiffTab(path);
          const name =
            path === TASK_TAB ? "TASK.md" : path === PREVIEW_TAB ? (previewState?.title || "Preview") : `${(fileOfTab(path) ?? path).split("/").pop() ?? path}${diff ? " (changes)" : ""}`;
          return (
            <div
              key={path}
              className={clsx(
                "group relative flex shrink-0 cursor-pointer items-center gap-2 rounded-t-lg px-3 text-[13px] transition-colors",
                isActive
                  ? clsx(palette.text, dark ? "bg-[#2F6BFF]/15" : palette.tabActiveBg)
                  : clsx(palette.textMuted, palette.hover)
              )}
              onClick={() => onSelectTab(path)}
              title={
                path === TASK_TAB ? "The task: read-only, and not part of your project" : path === PREVIEW_TAB ? "The live preview" : diff ? `Changes to ${path.slice(DIFF_PREFIX.length)} since the task started` : path
              }
            >
              {path === PREVIEW_TAB ? <Globe size={14} className="text-[#60A5FA]" /> : <FileTypeIcon name={path === TASK_TAB ? "TASK.md" : ((fileOfTab(path) ?? path).split("/").pop() ?? path)} size={14} />}
              <span className="max-w-[160px] truncate">{name}</span>
              {isActive && <span className={clsx("absolute inset-x-2 bottom-0 h-[2px] rounded-full", dark ? "bg-[#3B82F6]" : "bg-[#4A7FA7]")} />}
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

      {activePath !== TASK_TAB && activePath !== PREVIEW_TAB && breadcrumbs}

      <div className="min-h-0 flex-1">
        {activePath === TASK_TAB ? (
          // The brief, as a page to read. It isn't a file in the project, so
          // there is nothing to edit or save.
          <div className="h-full overflow-y-auto">
            <article className={clsx("mx-auto max-w-3xl px-8 py-8 text-[14.5px] leading-relaxed", palette.text, "[&_code]:rounded [&_code]:px-1 [&_code]:py-0.5 [&_h1]:mb-3 [&_h1]:text-[26px] [&_h1]:font-bold [&_h2]:mt-6 [&_h2]:mb-2 [&_h2]:text-[19px] [&_h2]:font-semibold [&_h3]:mt-4 [&_h3]:font-semibold [&_li]:my-1 [&_p]:my-2", dark ? "[&_code]:bg-[#16213A]" : "[&_code]:bg-[#B3CFE5]/40")}>
              <p className={clsx("mb-5 text-[12px] tracking-wide uppercase", palette.textMuted)}>TASK.md · read-only</p>
              <TinyMarkdown text={taskMarkdown} />
            </article>
          </div>
        ) : activePath === PREVIEW_TAB ? (
          previewState ? (
            <PreviewPanel theme={theme} html={previewState.html} title={previewState.title} watching={previewState.watching} onClose={() => onCloseTab(PREVIEW_TAB)} />
          ) : (
            <div className={clsx("flex h-full items-center justify-center text-sm", palette.textMuted)}>Nothing is being previewed.</div>
          )
        ) : isDiffTab(activePath) ? (
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
