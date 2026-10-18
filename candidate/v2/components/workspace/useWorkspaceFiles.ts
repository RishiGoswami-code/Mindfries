"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { initialTree, initialFiles } from "@/lib/ide/mock-project";
import { buildInitialWorkspace } from "@/lib/ide/seed-workspace";
import { addNode, collectFilePaths, findNode, moveNode, removeNode } from "@/lib/ide/tree";
import type { FileContents, TreeNode } from "@/lib/ide/types";
import type { VfsBridge } from "@/lib/ide/vfs-bridge";
import { emptyNotebookJson } from "@/lib/ide/notebook";
import { loadPersistedWorkspace, pickWorkspace, savePersistedWorkspace } from "@/lib/ide/fs-persist";
import { snapshotFiles } from "@/lib/ide/snapshot";
import { lineChange } from "@/lib/ide/line-change";
import { fileOfTab, isVirtualTab, PREVIEW_TAB, TASK_TAB, workspaceChanges } from "@/lib/ide/changes";
import { setGitScope } from "@/lib/ide/git/idb";

/**
 * The workspace's files: the tree, the contents, what's saved, which tabs
 * are open — and the bridge the terminal, the preview and the explorer all
 * write through. One copy of the files, shared by every view of them.
 *
 * The terminal runs from xterm callbacks, outside React's render, so writes
 * update a synchronous snapshot (`vfs.getSnapshot()`) before the React state:
 * a command line like `echo hi > a.txt && cat a.txt` must see its own file.
 */

export interface WorkspaceFilesOptions {
  starterFiles?: FileContents;
  /** Which saved copy in this browser: the session id, "practice", or none (scratch). */
  storageId?: string;
  /** A sandbox workspace's files are the sandbox's; nothing is restored or kept here. */
  sandboxed: boolean;
  serverWorkspace?: { files: FileContents; savedAt: number; frozen: boolean };
  /** Called with each autosave: which files, and how much each changed. */
  onSaved: (paths: string[], changes: Array<{ path: string; added: number; removed: number }>) => void;
}

/** What a create, rename or delete went wrong with, in words for the candidate — or null when it worked. */
export type EntryResult = string | null;

export function useWorkspaceFiles({ starterFiles, storageId, sandboxed, serverWorkspace, onSaved }: WorkspaceFilesOptions) {
  // Git's storage is per session too — set before anything can run git.
  useState(() => setGitScope(storageId));
  const [seeded] = useState(() =>
    starterFiles && Object.keys(starterFiles).length > 0 ? buildInitialWorkspace(starterFiles) : { tree: initialTree, files: initialFiles }
  );
  const [tree, setTree] = useState<TreeNode[]>(seeded.tree);
  const [files, setFiles] = useState<FileContents>(seeded.files);
  const [savedFiles, setSavedFiles] = useState<FileContents>(seeded.files);
  // TASK.md is the first tab: it's what a candidate reads first.
  const [openPaths, setOpenPaths] = useState<string[]>([TASK_TAB]);
  const [activePath, setActivePath] = useState<string | null>(TASK_TAB);
  const [restored, setRestored] = useState(false);

  const activeFile = fileOfTab(activePath);
  const changes = useMemo(() => workspaceChanges(seeded.files, files), [seeded.files, files]);
  const dirtyPaths = useMemo(() => {
    const dirty = new Set<string>();
    for (const path of openPaths) if (files[path] !== savedFiles[path]) dirty.add(path);
    return dirty;
  }, [openPaths, files, savedFiles]);

  // ── Restore, once, after mount (localStorage is client-only) ───────────
  useEffect(() => {
    /* eslint-disable react-hooks/set-state-in-effect -- restoring from localStorage can only happen after mount */
    if (sandboxed) return; // the sandbox sync restores, from the sandbox's disk
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
      const fromServer = buildInitialWorkspace(serverWorkspace.files);
      setTree(fromServer.tree);
      setFiles(fromServer.files);
      setSavedFiles(fromServer.files);
      const stillThere = (saved?.openPaths ?? [TASK_TAB]).filter((p) => p === TASK_TAB || p in fromServer.files);
      setOpenPaths(stillThere.length > 0 ? stillThere : [TASK_TAB]);
      setActivePath(saved?.activePath && (saved.activePath === TASK_TAB || saved.activePath in fromServer.files) ? saved.activePath : (stillThere[0] ?? TASK_TAB));
    }
    setRestored(true);
    /* eslint-enable react-hooks/set-state-in-effect */
    // Once, on mount: the props it reads were fixed server-side before this existed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ── Keep a copy in this browser, a moment after each change ────────────
  const [persistError, setPersistError] = useState(false);
  useEffect(() => {
    if (!restored || sandboxed) return;
    const timeout = setTimeout(() => {
      // When the browser refuses (storage full, private mode) the work is
      // still here and still checkpointed to the server; the candidate is told.
      setPersistError(!savePersistedWorkspace({ tree, files, savedFiles, openPaths, activePath }, storageId));
    }, 300);
    return () => clearTimeout(timeout);
  }, [restored, sandboxed, storageId, tree, files, savedFiles, openPaths, activePath]);

  // ── Autosave: a short pause after typing marks what's dirty saved ──────
  const onSavedRef = useRef(onSaved);
  useEffect(() => {
    onSavedRef.current = onSaved;
  });
  useEffect(() => {
    if (dirtyPaths.size === 0) return;
    const timeout = setTimeout(() => {
      const paths = [...dirtyPaths];
      const edits = paths.map((path) => ({ path, ...lineChange(savedFiles[path] ?? "", files[path] ?? "") }));
      setSavedFiles((prev) => {
        const next = { ...prev };
        for (const path of paths) next[path] = files[path];
        return next;
      });
      onSavedRef.current(paths, edits);
    }, 800);
    return () => clearTimeout(timeout);
    // savedFiles is read for the size of the edit only; depending on it would re-run this the moment it saves.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dirtyPaths, files]);

  // ── The synchronous snapshot the terminal reads ─────────────────────────
  const snapshotRef = useRef({ tree, files });
  useEffect(() => {
    snapshotRef.current = { tree, files };
  });
  const syncSnapshot = (next: { tree?: TreeNode[]; files?: FileContents }) => {
    snapshotRef.current = { tree: next.tree ?? snapshotRef.current.tree, files: next.files ?? snapshotRef.current.files };
  };

  const filesRef = useRef(files);
  useEffect(() => {
    filesRef.current = files;
  }, [files]);
  /** The files as they are this moment, ready to send. */
  const currentSnapshot = useCallback(() => snapshotFiles(filesRef.current), []);

  // ── Tabs ───────────────────────────────────────────────────────────────
  const openFile = (path: string) => {
    setOpenPaths((prev) => (prev.includes(path) ? prev : [...prev, path]));
    setActivePath(path);
  };
  /** Closing a tab never loses work: an unsaved edit is saved as it closes. */
  const closeTab = (path: string) => {
    if (!isVirtualTab(path) && files[path] !== savedFiles[path]) setSavedFiles((prev) => ({ ...prev, [path]: files[path] }));
    setOpenPaths((prev) => {
      const next = prev.filter((p) => p !== path);
      if (activePath === path) setActivePath(next.at(-1) ?? null);
      return next;
    });
  };
  const changeFile = (path: string, value: string) => setFiles((prev) => ({ ...prev, [path]: value }));
  const saveFile = (path: string) => setSavedFiles((prev) => ({ ...prev, [path]: files[path] }));

  // ── The filesystem the terminal, explorer and preview share ────────────
  const parentOf = (path: string) => (path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : null);

  const createFile = (path: string, content = ""): string | null => {
    const { tree: t, files: f } = snapshotRef.current;
    if (findNode(t, path)) return "File exists";
    const parent = parentOf(path);
    if (parent && !findNode(t, parent)) return "No such file or directory";
    const nextTree = addNode(t, parent, { type: "file", path, name: path.split("/").pop() as string });
    const nextFiles = { ...f, [path]: content };
    syncSnapshot({ tree: nextTree, files: nextFiles });
    setTree(nextTree);
    setFiles(nextFiles);
    setSavedFiles((prev) => ({ ...prev, [path]: content }));
    return null;
  };

  const createFolder = (path: string): string | null => {
    const { tree: t } = snapshotRef.current;
    if (findNode(t, path)) return "File exists";
    const parent = parentOf(path);
    if (parent && !findNode(t, parent)) return "No such file or directory";
    const nextTree = addNode(t, parent, { type: "folder", path, name: path.split("/").pop() as string, children: [] });
    syncSnapshot({ tree: nextTree });
    setTree(nextTree);
    return null;
  };

  const remove = (path: string, recursive: boolean): string | null => {
    const { tree: t, files: f } = snapshotRef.current;
    const node = findNode(t, path);
    if (!node) return "No such file or directory";
    if (node.type === "folder" && !recursive) return "Is a directory";
    const removed = collectFilePaths(t, path);
    const nextTree = removeNode(t, path);
    const nextFiles = omit(f, removed);
    syncSnapshot({ tree: nextTree, files: nextFiles });
    setTree(nextTree);
    setFiles(nextFiles);
    setSavedFiles((prev) => omit(prev, removed));
    setOpenPaths((prev) => {
      const next = prev.filter((p) => !removed.includes(fileOfTab(p) ?? p));
      setActivePath((current) => (current && removed.includes(fileOfTab(current) ?? current) ? (next.at(-1) ?? null) : current));
      return next;
    });
    return null;
  };

  const write = (path: string, content: string, append = false): string | null => {
    const { tree: t, files: f } = snapshotRef.current;
    const existing = findNode(t, path);
    if (existing?.type === "folder") return "Is a directory";
    let nextTree = t;
    if (!existing) {
      const parent = parentOf(path);
      if (parent && !findNode(t, parent)) return "No such file or directory";
      nextTree = addNode(t, parent, { type: "file", path, name: path.split("/").pop() as string });
    }
    const content2 = append ? (f[path] ?? "") + content : content;
    const nextFiles = { ...f, [path]: content2 };
    syncSnapshot({ tree: nextTree, files: nextFiles });
    setTree(nextTree);
    setFiles(nextFiles);
    setSavedFiles((prev) => ({ ...prev, [path]: content2 }));
    return null;
  };

  const move = (src: string, destParent: string | null, destName: string): string | null => {
    const { tree: t } = snapshotRef.current;
    if (!findNode(t, src)) return "No such file or directory";
    if (destParent && !findNode(t, destParent)) return "No such file or directory";
    const dest = destParent ? `${destParent}/${destName}` : destName;
    if (dest !== src && findNode(t, dest)) return "File exists";
    const oldPaths = collectFilePaths(t, src);
    const nextTree = moveNode(t, src, destParent, destName);
    if (!nextTree) return "No such file or directory";
    const newPaths = collectFilePaths(nextTree, dest);
    syncSnapshot({ tree: nextTree });
    setTree(nextTree);
    if (oldPaths.length !== newPaths.length) return null;
    const mapping = new Map(oldPaths.map((old, i) => [old, newPaths[i]]));
    const nextFiles = remapKeys(snapshotRef.current.files, mapping);
    syncSnapshot({ files: nextFiles });
    setFiles(nextFiles);
    setSavedFiles((prev) => remapKeys(prev, mapping));
    setOpenPaths((prev) => prev.map((p) => mapping.get(p) ?? p));
    setActivePath((prev) => (prev ? (mapping.get(prev) ?? prev) : prev));
    return null;
  };

  const vfs: VfsBridge = { getSnapshot: () => snapshotRef.current, createFile, createFolder, remove, write, move };

  // ── What the explorer does, with errors in words rather than browser pop-ups ──
  const createEntry = (parentPath: string | null, kind: "file" | "folder", name: string): EntryResult => {
    const path = parentPath ? `${parentPath}/${name}` : name;
    const err = kind === "file" ? createFile(path, name.toLowerCase().endsWith(".ipynb") ? emptyNotebookJson() : "") : createFolder(path);
    if (err) return err === "File exists" ? `"${name}" already exists here.` : err;
    if (kind === "file") openFile(path);
    return null;
  };
  const renameEntry = (path: string, newName: string): EntryResult => {
    const err = move(path, parentOf(path), newName);
    return err ? (err === "File exists" ? `"${newName}" already exists here.` : err) : null;
  };
  const deleteEntry = (path: string): EntryResult => remove(path, true);

  /** The sandbox's files arrived (or changed on its disk): they replace what's here. */
  const applySandboxFiles = (next: { files: FileContents; saved: FileContents; initial: boolean }) => {
    const rebuilt = buildInitialWorkspace(next.files);
    syncSnapshot({ tree: rebuilt.tree, files: next.files });
    setTree(rebuilt.tree);
    setFiles(next.files);
    setSavedFiles(next.saved);
    setOpenPaths((paths) => paths.filter((p) => isVirtualTab(p) || (fileOfTab(p) !== null && (fileOfTab(p) as string) in next.files)));
    setActivePath((path) => (path && (isVirtualTab(path) || (fileOfTab(path) ?? path) in next.files) ? path : null));
    if (next.initial) setRestored(true);
  };

  return {
    seeded,
    tree,
    files,
    savedFiles,
    openPaths,
    setOpenPaths,
    activePath,
    setActivePath,
    activeFile,
    dirtyPaths,
    changes,
    restored,
    persistError,
    vfs,
    currentSnapshot,
    openFile,
    closeTab,
    changeFile,
    saveFile,
    createEntry,
    renameEntry,
    deleteEntry,
    applySandboxFiles,
  };
}

function omit(obj: FileContents, keys: string[]): FileContents {
  const keySet = new Set(keys);
  return Object.fromEntries(Object.entries(obj).filter(([k]) => !keySet.has(k)));
}

function remapKeys(obj: FileContents, mapping: Map<string, string>): FileContents {
  return Object.fromEntries(Object.entries(obj).map(([k, v]) => [mapping.get(k) ?? k, v]));
}
