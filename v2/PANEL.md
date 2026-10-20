# V2 floating panel — design

The candidate's cockpit for a V2 assessment: a tablet-sized card that floats
**above every window** (their editor included), holds the workspace tabs, the
camera, and the AI avatar, and is where every notification and flag lands.
It's the spec's floating tabbed panel ([SPEC §10](SPEC.md#10-the-unified-candidate-workspace))
and AI avatar ([§12](SPEC.md#12-ai-avatar-important-but-not-the-main-judge)),
shaped by the owner's mock-up (2026-10-20). **Status: proposal, for the
owner's approval before any V1 code changes.**

Decisions (owner, 2026-10-20):
- **Always on top** with the browser's **Document Picture-in-Picture**, as
  Meet and Zoom do. Chrome and Edge; elsewhere it's a normal tab.
- **AI avatar** = **Gemini Live** (voice, plus camera and screen frames) and a
  **2D animated avatar**. Keys stay on the server.
- **Where:** in the **candidate portal** (`candidate/frontend`), flag-gated,
  per [`INTEGRATION.md`](INTEGRATION.md). Backend in `v2/api`.
- **Tabs (first version):** System Design canvas, Notes, Resources,
  Activity & flags, and Review / Debug / Test. Each test shows the ones its
  role needs.

## 1. What the candidate sees

**Expanded** (about 1024 × 680, resizable): the mock-up.

```
┌ Mindfries · <active tab> ─────────────────── ⏱ 1:12:40 · ⚑ 2 ·  ⚙  ⤢  ─ ┐
│ ┌ rail ───────┐ ┌ workspace ───────────────────────┐ ┌ right ─────────┐ │
│ │ System      │ │  (the active tab)                │ │ Camera ● Rec   │ │
│ │ Design      │ │  canvas · notes · resources ·    │ │ [video]        │ │
│ │ Notes       │ │  review/debug/test · activity    │ │ 🎤 📷 ⏹        │ │
│ │ Resources   │ │                                  │ ├────────────────┤ │
│ │ Activity ⚑2 │ │                                  │ │ AI Avatar      │ │
│ │ Review      │ │                                  │ │ [2D avatar]    │ │
│ └─────────────┘ └──────────────────────────────────┘ │ Listening…  🎤 │ │
│                                                       └────────────────┘ │
└──────────────────────────────────────────────────────────────────────────┘
```

**Minimised** (about 300 × 380): only the **AI avatar card**. Everything else
collapses into it: the timer, a flag/notification badge, a "monitoring on"
dot, mic mute, and Expand. New flags pulse the badge; the avatar can still
talk.

Header: the active tab, the **timer**, the **flag count**, settings
(devices, avatar voice on/off), **pop out / pop in** (PiP), and **minimise**.
It never closes while the session runs; closing the tab is a monitoring gap.

## 2. How "on top of everything" works

- `mindfries start` opens `/v2/session/<id>` in the browser. Document PiP
  needs a click, so the page shows one big **"Open the floating panel"**
  button. It calls `documentPictureInPicture.requestWindow({ width, height })`
  and renders the panel into that window with a React portal (styles copied
  across). The window stays above all apps until the session ends.
- **Minimise / expand** resizes the PiP window (`resizeTo`) and switches the
  layout. If the candidate closes the PiP window, it returns to the tab, the
  page offers "Pop out again", and it's noted as an event.
- Capture (camera, mic, screen) and the Gemini Live connection live in the
  **opener page**, not the PiP window, so they survive pop-in and pop-out;
  the PiP window only displays.
- **Firefox and Safari:** no Document PiP, so the same panel runs in the tab
  and the candidate keeps the tab beside their editor.

## 3. The tabs, per candidate

Each session tells the panel which tabs to show, from the product's role
manifest and the task (`v2/products/*/manifest.json` `roles.<role>.modes`,
`_tasks/*/task.json` `kind`):

| Tab | Shown when | Evidence it records |
|---|---|---|
| **System Design** canvas (boxes, arrows, text, as in the mock-up) | mode `design` / `architecture` roles | the diagram JSON, saved with history |
| **Notes** (decision log) | always | the notes, timestamped |
| **Resources** (the task's docs and links; research is allowed) | always; plus `research` mode | which resources were opened |
| **Review / Debug / Test** (a diff to review, logs to investigate, tests to write) | modes `review`, `debugging`, `testing` | review comments, findings |
| **Activity & flags** (the tracker) | always | (it shows the record; it doesn't add to it) |

So a backend candidate on P1 sees Notes, Resources, Debug, Test, Activity; an
architect sees System Design first; a quant on P3 sees Notes, Resources,
Review, Activity. Adding a role or product is a manifest change, not code.

## 4. The tracker: notifications, flags, activity

Everything already recorded about the session streams to the panel live:

- **From the CLI watcher** (through `v2/api`): commits, test runs, file
  changes, `TASK.md` edits, coding agents detected, offline / back online,
  submissions.
- **From the panel itself:** camera, screen or mic stopped, tab hidden, PiP
  closed.
- **From the server:** time warnings (30, 10, 5 minutes), session ending,
  reviewer messages (later).

Each item is `{at, kind, severity (info | notice | flag), text, source}`.
Flags show the spec's three categories ([§16](SPEC.md#use-three-distinct-categories)):
observed events, verified violations, review signals. They're stated as
facts, never accusations, and never block the candidate.

Delivery: **Server-Sent Events** `GET /v2/panel/session/<id>/stream`, proxied
by the portal (the cookie is httpOnly), resuming from the last seen id.

## 5. Monitoring (camera, screen, microphone)

What the fake panel does today moves here, reusing the portal's existing
hooks (`useProctorCamera`, `useProctorScreen`, `camera-snapshots`,
`interview-recorder`):

- The right column's **Camera** tile is the live camera (Recording dot; mic,
  camera and stop controls).
- The screen share starts from the panel (`getDisplayMedia`).
- A camera still and a screenshot every 30 seconds, and mic audio in
  30-second clips, go to `v2/api` (then Supabase Storage). Reports to
  `/session/monitoring` let `mindfries start` begin (contract
  `POST /session/begin`).

## 6. The AI avatar

- **Voice:** the opener page streams mic PCM to `v2/api` over a WebSocket,
  and the server relays it to **Gemini Live**, as `candidate/backend`'s
  `live.go` already does for V1's interview (to be copied into `v2/api` and
  owned there). Replies come back as audio and are played.
- **Seeing:** camera frames (about 1 per 2 seconds) and screen frames (about
  1 per 2 seconds, downscaled) go to the same Live session, so the avatar
  can talk about what's on screen.
- **Face:** a 2D animated avatar (SVG/Canvas): idle, listening, thinking and
  speaking states, with mouth movement driven by the reply's audio level. It
  shows "Listening…" as in the mock-up.
- **Behaviour** ([§12](SPEC.md#12-ai-avatar-important-but-not-the-main-judge)):
  the system prompt comes from the task brief, the role's competencies and
  the session's recent activity. It asks short follow-ups ("why a cache in
  front of Postgres?"), never gives the solution, never judges, and is quiet
  unless spoken to or at checkpoints. Turns and transcripts are evidence
  ([§15](SPEC.md#15-evidence-model-and-timeline)); its weight in the report
  stays small.
- **Consent:** before the session, the candidate is told the avatar sees
  their camera and screen and hears their mic, and that it's sent to
  Google's Gemini under Mindfries' data terms.

## 7. Backend (`v2/api`) additions

| Endpoint | For |
|---|---|
| `GET /v2/panel/session/{id}` | the panel's config: tabs, timer, monitoring requirements, avatar on/off |
| `GET /v2/panel/session/{id}/stream` (SSE) | the tracker feed |
| `POST /v2/panel/session/{id}/monitoring` (status, stills, audio) | monitoring, as the fake API's panel does |
| `PUT /v2/panel/session/{id}/artifacts/{tab}` | canvas JSON, notes, review comments (versioned) |
| `GET /v2/panel/session/{id}/avatar` (WebSocket) | the Gemini Live relay |

The portal calls these from its own route handlers with the candidate's
session (the browser never reaches `v2/api` directly). Storage: new
`v2.panel_events`, `v2.panel_artifacts`, `v2.captures` tables plus a Storage
bucket. The CLI contract gets the matching changes, so the session's state
and flags are the same on both sides.

## 8. The V1 seam (what changes in `candidate/frontend`)

All behind `V2_ENABLED` (off by default), in owner-approved `v1-seam` PRs:

1. The dashboard: an assessment with `engine = 'v2'` shows **Start
   assessment**, which mints the setup code and shows the one command (what
   the fake dashboard does today).
2. A new route `/v2/session/[id]`: the panel, with its own components under
   `src/components/v2/panel/`, and route handlers under `src/app/api/v2/`.
3. The middleware matcher gets `/v2`.

Nothing else in V1 changes. V1 assessments look and work exactly as today.

## 9. Build order

1. **Panel UI**, with mock data: the expanded and minimised layouts, the
   tabs, the camera tile, the avatar card. In `candidate/frontend` behind
   the flag. *Done when* it matches the mock-up in both states.
2. **Pop-out:** Document PiP with minimise and expand, and the in-tab
   fallback.
3. **Live tracker:** `v2/api` events + SSE, wired to the CLI watcher's
   events.
4. **Monitoring** moved into the panel, and `begin` waits for it.
5. **AI avatar:** the Gemini Live relay in `v2/api`, the 2D avatar, frames,
   transcripts.
6. **Tabs by role:** the manifest-driven tab set, and the canvas, notes and
   review artifacts saved as evidence.
7. **The CLI and fake API stay in step** (the fake serves the same panel API,
   so every CLI test covers it).
