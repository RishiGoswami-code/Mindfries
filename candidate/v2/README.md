# Candidate panel — v2 (parked, not built)

> **V2 is now specified in [`v2/SPEC.md`](../../v2/SPEC.md); start at
> [`v2/README.md`](../../v2/README.md).** This folder predates that spec. It
> refactors V1's *browser* workspace, while V2 moves coding into the
> candidate's own IDE. Whether any of it carries into V2, as a browser
> fallback, is still to be decided.

The v2 candidate panel, kept apart from the live one until it's complete.
**Nothing here is built, run or deployed.** The live candidate portal is still
`candidate/frontend` (v1), unchanged. What goes into v2 is still to be decided;
this folder holds the first piece, the workspace.

The files use the same `@/…` import paths as `candidate/frontend/src`, so they
can be moved into an app as they are. Paths inside are relative to that `src`.

## What's here

### `components/workspace/` — the v2 workspace

The v1 workspace (`candidate/frontend/src/components/ide/IdeShell.tsx`) is
one 1,300-line component. Here it's split into hooks, with a short component
that lays out the existing panels:

| File | What it owns |
|---|---|
| `Workspace.tsx` | Layout, and handing the work in |
| `useWorkspaceFiles.ts` | Files, tree, tabs, restore, autosave, the terminal's filesystem bridge |
| `useWorkspaceTelemetry.ts` | Pastes, time away, commands, test runs, files read, tool output |
| `useProctoring.ts` | Camera, screen, stills, flagged moments |
| `usePreview.ts` | The live web preview |
| `useSessionClock.ts` | The session clock |
| `useCheckpoints.ts` | Copies of the work sent to the server |
| `WorkspaceHeader.tsx` | Top bar |
| `Overlays.tsx` | Time's up, finished, confirm, toast |

It reuses v1's panels (`components/ide/*`: editor, terminal, explorer, chat,
camera, status bar) as they are.

What it changes for the candidate, compared with v1:

- **Time's up:** when time runs out, the workspace locks, saves a last copy and hands the work in, then opens the report. In v1 the editor stays editable and silently stops saving.
- **Clock:** it counts from a deadline, not tick by tick, so a throttled tab or a sleeping laptop can't show more time than is left. A screen reader hears "five minutes left" and "one minute left".
- **Saving on close:** a copy is sent as the tab closes, with a `keepalive` request, not only once a minute. That needs `app/api/workspace/checkpoint`.
- **Practice:** a practice run ends with "Finish" and a practice screen. It doesn't claim the hiring team will review it.
- **End session:** the button is gone in a real assessment; Submit is the way out.
- **Pop-ups:** delete confirmations and explorer errors are in the workspace, not browser pop-ups. Those take a fullscreen session out of fullscreen, which is itself a flagged moment.
- **Storage full:** if the browser can't keep a local copy, the candidate is told the work is still being saved to the server.
- **Interview:** a real flag (`NEXT_PUBLIC_INTERVIEW_ENABLED`), and its code only loads when it's on.

### `app/ide/` — the route

`page.tsx` and `layout.tsx` for the workspace page. It takes the same entry
rules as v1's `/ide`: `?practice=1`, or `?session=<id>` for the candidate's
own live session.

### `app/api/workspace/checkpoint/route.ts`

The endpoint the workspace sends its last copy to as the tab closes.

### Replacements for v1 files

These are v1 files with changes v2 depends on. They replace the file at the
same path when v2 is built; v1's own copies are untouched.

| File | Change |
|---|---|
| `lib/ide/telemetry.ts` (+ `telemetry.test.ts`) | Sends at most 25 events per request, so a backlog after an outage can still be delivered. In v1 the server refuses a queue over 100 forever. Also: drops a batch the server refuses as malformed, caps the queue at 2,000, and sends what's left on close in beacon-sized pieces. The tests cover each. |
| `lib/ide/fs-persist.ts` | `savePersistedWorkspace` returns whether the browser kept the copy. |
| `components/ide/FileExplorer.tsx` | `onEndSession` is optional; without it there's no End session button. |

## Not done yet

- **Barely checked:** typechecked once inside `candidate/frontend` before being parked. The one error that showed was fixed afterwards, without typechecking again. Never linted, never run in a browser.
- **Not tested:** no test covers the hooks themselves.
- **Rest of the panel:** what else goes into v2 is to be decided.
