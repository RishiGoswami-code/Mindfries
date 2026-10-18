"use client";

import { useState, type MouseEvent } from "react";
import {
  ChevronDown,
  ChevronRight,
  Folder,
  FolderOpen,
  FilePlus,
  Files,
  FolderPlus,
  Pin,
  LogOut,
  Pencil,
  Trash2,
} from "lucide-react";
import { FileTypeIcon } from "./FileTypeIcon";
import clsx from "clsx";
import type { TreeNode } from "@/lib/ide/types";
import { siblingNames } from "@/lib/ide/tree";
import { idePalette } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";

interface NewEntryDraft {
  parentPath: string | null;
  kind: "file" | "folder";
}

interface FileExplorerProps {
  tree: TreeNode[];
  activePath: string | null;
  theme: IdeTheme;
  candidateName?: string;
  onOpenFile: (path: string) => void;
  onCreate: (parentPath: string | null, kind: "file" | "folder", name: string) => void;
  onRename: (path: string, newName: string) => void;
  onDelete: (path: string) => void;
  /** Absent: no End session button (in an assessment, Submit is the way out). */
  onEndSession?: () => void;
  /** Opens the task brief (TASK.md), pinned above the project's files. */
  onOpenTask: () => void;
  /** Whether the TASK.md tab is the one showing. */
  taskActive: boolean;
}

export function FileExplorer({
  tree,
  activePath,
  theme,
  candidateName,
  onOpenFile,
  onCreate,
  onRename,
  onDelete,
  onEndSession,
  onOpenTask,
  taskActive,
}: FileExplorerProps) {
  const palette = idePalette(theme);
  // Top-level folders start open, so the project's shape is visible at once.
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(["src", "tests", ...tree.filter((n) => n.type === "folder").map((n) => n.path)]));
  const [draft, setDraft] = useState<NewEntryDraft | null>(null);
  const [renaming, setRenaming] = useState<string | null>(null);

  const toggleExpanded = (path: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const startCreate = (parentPath: string | null, kind: "file" | "folder") => {
    if (parentPath) setExpanded((prev) => new Set(prev).add(parentPath));
    setDraft({ parentPath, kind });
  };

  const commitDraft = (name: string) => {
    if (draft && name.trim()) {
      onCreate(draft.parentPath, draft.kind, name.trim());
    }
    setDraft(null);
  };

  const commitRename = (path: string, name: string) => {
    if (name.trim()) onRename(path, name.trim());
    setRenaming(null);
  };

  return (
    <div className={clsx("flex h-full min-h-0 flex-col text-sm select-none", palette.panelBg, palette.text)}>
      <div
        className={clsx(
          "flex h-12 items-center justify-between border-b px-3",
          palette.border,
          palette.textMuted
        )}
      >
        <span className={clsx("flex items-center gap-2 text-[14.5px] font-semibold", palette.text)}>
          <Files size={17} className={theme === "dark" ? "text-[#60A5FA]" : palette.accent} />
          Files
        </span>
        <div className="flex items-center gap-1">
          <button
            type="button"
            title="New File"
            className={clsx("rounded p-1", palette.hover)}
            onClick={() => startCreate(null, "file")}
          >
            <FilePlus size={14} />
          </button>
          <button
            type="button"
            title="New Folder"
            className={clsx("rounded p-1", palette.hover)}
            onClick={() => startCreate(null, "folder")}
          >
            <FolderPlus size={14} />
          </button>
        </div>
      </div>
      <div className="flex-1 overflow-auto py-1">
        {/* The task brief, pinned: it reads like a file in the project but
            isn't one — it's never saved, sent to the sandbox or seen by git. */}
        <button
          type="button"
          onClick={onOpenTask}
          title="The task, read-only (not part of your project)"
          className={clsx(
            "mx-1.5 mb-1 flex w-[calc(100%-12px)] items-center gap-2 rounded-lg px-2 py-1.5 text-left text-[13.5px]",
            taskActive ? palette.active : palette.hover
          )}
        >
          <FileTypeIcon name="TASK.md" size={15} />
          <span className="min-w-0 flex-1 truncate font-medium">TASK.md</span>
          <Pin size={12} className={palette.textMuted} />
        </button>
        <TreeLevel
          nodes={tree}
          depth={0}
          expanded={expanded}
          activePath={activePath}
          theme={theme}
          draft={draft}
          renaming={renaming}
          onToggleExpanded={toggleExpanded}
          onOpenFile={onOpenFile}
          onStartCreate={startCreate}
          onCommitDraft={commitDraft}
          onCancelDraft={() => setDraft(null)}
          onStartRename={setRenaming}
          onCommitRename={commitRename}
          onCancelRename={() => setRenaming(null)}
          onDelete={onDelete}
          existingSiblingNames={(parentPath) => siblingNames(tree, parentPath)}
        />
        {draft?.parentPath === null && (
          <InlineInput
            depth={0}
            theme={theme}
            placeholder={draft.kind === "file" ? "file-name.ts" : "folder-name"}
            onCommit={commitDraft}
            onCancel={() => setDraft(null)}
          />
        )}
      </div>

      <SidebarFooter theme={theme} candidateName={candidateName} onEndSession={onEndSession} />
    </div>
  );
}

/** The signed-in candidate, pinned to the bottom of the Explorer, with End session when there is one. */
function SidebarFooter({
  theme,
  candidateName,
  onEndSession,
}: {
  theme: IdeTheme;
  candidateName?: string;
  onEndSession?: () => void;
}) {
  const palette = idePalette(theme);
  const displayName = candidateName?.trim() || "Candidate";
  const initial = displayName.charAt(0).toUpperCase() || "C";

  return (
    <div className={clsx("shrink-0 border-t", palette.border)}>
      <div className={clsx("flex items-center gap-2 px-3 py-2", palette.border)}>
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[#4A7FA7] text-[11px] font-semibold text-[#F6FAFD]">
          {initial}
        </span>
        <span className="min-w-0 flex-1">
          <span className={clsx("block truncate text-xs", palette.text)}>{displayName}</span>
          <span className={clsx("block truncate text-[10px]", palette.textMuted)}>Candidate</span>
        </span>
        {/* The workspace's own exit. It's here rather than somewhere more
            prominent on purpose: ending is deliberate, not something to hit
            while reaching for a tab. */}
        {onEndSession && <button
          type="button"
          title="End session"
          aria-label="End session"
          onClick={onEndSession}
          // Red, because it's the one control here that ends something. The
          // hue is the same #E06C75 the Debug Console uses for errors, rather
          // than a second red introduced alongside it.
          className="shrink-0 rounded-md p-1.5 text-[#E06C75] hover:bg-[#E06C75]/15"
        >
          <LogOut size={14} />
        </button>}
      </div>
    </div>
  );
}

interface TreeLevelProps {
  nodes: TreeNode[];
  depth: number;
  expanded: Set<string>;
  activePath: string | null;
  theme: IdeTheme;
  draft: NewEntryDraft | null;
  renaming: string | null;
  onToggleExpanded: (path: string) => void;
  onOpenFile: (path: string) => void;
  onStartCreate: (parentPath: string | null, kind: "file" | "folder") => void;
  onCommitDraft: (name: string) => void;
  onCancelDraft: () => void;
  onStartRename: (path: string) => void;
  onCommitRename: (path: string, name: string) => void;
  onCancelRename: () => void;
  onDelete: (path: string) => void;
  existingSiblingNames: (parentPath: string | null) => string[];
}

function TreeLevel({
  nodes,
  depth,
  expanded,
  activePath,
  theme,
  draft,
  renaming,
  onToggleExpanded,
  onOpenFile,
  onStartCreate,
  onCommitDraft,
  onCancelDraft,
  onStartRename,
  onCommitRename,
  onCancelRename,
  onDelete,
  existingSiblingNames,
}: TreeLevelProps) {
  const palette = idePalette(theme);
  const sorted = [...nodes].sort((a, b) => {
    if (a.type !== b.type) return a.type === "folder" ? -1 : 1;
    return a.name.localeCompare(b.name);
  });

  return (
    <>
      {sorted.map((node) => {
        const isActive = node.path === activePath;
        const isRenaming = renaming === node.path;
        const paddingLeft = 12 + depth * 14;

        if (node.type === "file") {
          return (
            <div key={node.path} className="group">
              {isRenaming ? (
                <InlineInput
                  depth={depth}
                  theme={theme}
                  initialValue={node.name}
                  onCommit={(name) => onCommitRename(node.path, name)}
                  onCancel={onCancelRename}
                />
              ) : (
                <div
                  className={clsx(
                    "mx-1.5 flex cursor-pointer items-center justify-between rounded-lg py-[5px] pr-2",
                    palette.hover,
                    isActive && clsx(palette.active, theme === "dark" && "text-white")
                  )}
                  style={{ paddingLeft }}
                  onClick={() => onOpenFile(node.path)}
                  onDoubleClick={() => onStartRename(node.path)}
                >
                  <span className="flex min-w-0 items-center gap-1.5">
                    <FileTypeIcon name={node.name} />
                    <span className="truncate">{node.name}</span>
                  </span>
                  <RowActions
                    theme={theme}
                    onRename={() => onStartRename(node.path)}
                    onDelete={() => onDelete(node.path)}
                  />
                </div>
              )}
            </div>
          );
        }

        const isOpen = expanded.has(node.path);
        return (
          <div key={node.path}>
            {isRenaming ? (
              <InlineInput
                depth={depth}
                theme={theme}
                initialValue={node.name}
                onCommit={(name) => onCommitRename(node.path, name)}
                onCancel={onCancelRename}
              />
            ) : (
              <div
                className={clsx(
                  "group mx-1 flex cursor-pointer items-center justify-between rounded-[4px] py-[3px] pr-2",
                  palette.hover
                )}
                style={{ paddingLeft }}
                onClick={() => onToggleExpanded(node.path)}
                onDoubleClick={() => onStartRename(node.path)}
              >
                <span className="flex min-w-0 items-center gap-1">
                  {isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                  {isOpen ? (
                    <FolderOpen size={14} className={palette.accent} />
                  ) : (
                    <Folder size={14} className={palette.accent} />
                  )}
                  <span className="truncate">{node.name}</span>
                </span>
                <RowActions
                  theme={theme}
                  onNewFile={(e) => {
                    e.stopPropagation();
                    onStartCreate(node.path, "file");
                  }}
                  onNewFolder={(e) => {
                    e.stopPropagation();
                    onStartCreate(node.path, "folder");
                  }}
                  onRename={() => onStartRename(node.path)}
                  onDelete={() => onDelete(node.path)}
                />
              </div>
            )}
            {isOpen && (
              <TreeLevel
                nodes={node.children}
                depth={depth + 1}
                expanded={expanded}
                activePath={activePath}
                theme={theme}
                draft={draft}
                renaming={renaming}
                onToggleExpanded={onToggleExpanded}
                onOpenFile={onOpenFile}
                onStartCreate={onStartCreate}
                onCommitDraft={onCommitDraft}
                onCancelDraft={onCancelDraft}
                onStartRename={onStartRename}
                onCommitRename={onCommitRename}
                onCancelRename={onCancelRename}
                onDelete={onDelete}
                existingSiblingNames={existingSiblingNames}
              />
            )}
            {isOpen && draft?.parentPath === node.path && (
              <InlineInput
                depth={depth + 1}
                theme={theme}
                placeholder={draft.kind === "file" ? "file-name.ts" : "folder-name"}
                onCommit={onCommitDraft}
                onCancel={onCancelDraft}
              />
            )}
          </div>
        );
      })}
    </>
  );
}

function RowActions({
  theme,
  onNewFile,
  onNewFolder,
  onRename,
  onDelete,
}: {
  theme: IdeTheme;
  onNewFile?: (e: MouseEvent) => void;
  onNewFolder?: (e: MouseEvent) => void;
  onRename: () => void;
  onDelete: () => void;
}) {
  const palette = idePalette(theme);
  return (
    <span className="hidden items-center gap-0.5 group-hover:flex">
      {onNewFile && (
        <button type="button" title="New File" className={clsx("rounded p-0.5", palette.hover)} onClick={onNewFile}>
          <FilePlus size={12} />
        </button>
      )}
      {onNewFolder && (
        <button type="button" title="New Folder" className={clsx("rounded p-0.5", palette.hover)} onClick={onNewFolder}>
          <FolderPlus size={12} />
        </button>
      )}
      <button
        type="button"
        title="Rename"
        className={clsx("rounded p-0.5", palette.hover)}
        onClick={(e) => {
          e.stopPropagation();
          onRename();
        }}
      >
        <Pencil size={12} />
      </button>
      <button
        type="button"
        title="Delete"
        className={clsx("rounded p-0.5", palette.hover)}
        onClick={(e) => {
          e.stopPropagation();
          onDelete();
        }}
      >
        <Trash2 size={12} />
      </button>
    </span>
  );
}

function InlineInput({
  depth,
  theme,
  initialValue = "",
  placeholder,
  onCommit,
  onCancel,
}: {
  depth: number;
  theme: IdeTheme;
  initialValue?: string;
  placeholder?: string;
  onCommit: (value: string) => void;
  onCancel: () => void;
}) {
  const palette = idePalette(theme);
  const [value, setValue] = useState(initialValue);

  return (
    <div style={{ paddingLeft: 12 + depth * 14 }} className="py-[2px] pr-2">
      <input
        autoFocus
        value={value}
        placeholder={placeholder}
        onChange={(e) => setValue(e.target.value)}
        onFocus={(e) => e.target.select()}
        onBlur={() => (value.trim() ? onCommit(value) : onCancel())}
        onKeyDown={(e) => {
          if (e.key === "Enter") onCommit(value);
          if (e.key === "Escape") onCancel();
        }}
        className={clsx(
          "w-full rounded border px-1 py-0.5 text-sm outline-none",
          palette.border,
          palette.appBg,
          palette.text
        )}
      />
    </div>
  );
}
