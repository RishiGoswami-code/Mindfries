# `v2/web` — the new V2 screens

Only the screens V2 adds. The candidate arrives from the V1 dashboard
through a signed, single-use handoff link; there is no sign-in, dashboard or
settings here. See [PLAN, Stage 3 and 5](../PLAN.md#stage-3-onward--the-rest-of-v2)
and [SPEC §10](../SPEC.md#10-the-unified-candidate-workspace).

**Status: empty.** Stage 3 builds it.

## What goes here

- A Next.js app, deployed as its own Vercel project.
- Consent: what the CLI checks, and the camera.
- "Set up my workspace", which shows the one setup command.
- Live session status, the stage view, submission review.
- The floating tabbed panel: Camera, Stage & Task, System Design, Debugging &
  Reliability, Notes & Evidence, Conversation, Submission review. It has no
  AI chat; that is `mindfries agent`.

## Rules

- Talks only to `v2/api`, never to V1's backends or tables.
- Imports nothing from V1's Next apps.
- Like V1's apps, check `node_modules/next/dist/docs/` before writing
  framework code: this Next.js is newer than you remember.
