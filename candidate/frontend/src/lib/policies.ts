/**
 * What gets recorded, how it's used, what may be used, and for how long — the real consent
 * copy a candidate agrees to in onboarding (ConsentStep), pulled out here so
 * the "Privacy & recording policy" item in the account menu shows the exact
 * same text rather than a second, hand-written version that could quietly
 * drift from what was actually agreed to.
 */

/**
 * Each signal carries a `key` the policy window maps to an icon, and an
 * emoji the consent step shows inline.
 */
export const RECORDED_SIGNALS = [
  { key: "navigation", icon: "🗂️", label: "Repository navigation", detail: "Which files you explore and in what order" },
  { key: "code", icon: "✏️", label: "Code changes", detail: "Every edit, not just the final state" },
  { key: "terminal", icon: "⌨️", label: "Terminal commands", detail: "Commands run and their output" },
  { key: "tests", icon: "🧪", label: "Test execution", detail: "Which tests you run and when" },
  { key: "assistant", icon: "💬", label: "AI assistant usage", detail: "How you interact with the AI — what you ask and why" },
  { key: "time", icon: "⏱️", label: "Time patterns", detail: "Where you spend time — reading, coding, debugging" },
  {
    key: "paste",
    icon: "📋",
    label: "Pasting and focus",
    detail: "The size of text you paste in (never its content), and how long the workspace tab is out of view",
  },
  {
    key: "media",
    icon: "🎥",
    label: "Screen, camera and microphone",
    detail:
      "Your screen and camera are kept as stills — the camera every half minute, the screen when it changes and at flagged moments; the microphone keeps a short clip only when voices or a loud noise last a few seconds",
  },
  {
    key: "interview",
    icon: "🎙️",
    label: "Follow-up interview",
    detail: "Your answers to the AI interviewer — as text, and as a recording of your voice and camera",
  },
];

export const WHAT_IS_RECORDED = {
  title: "What is recorded during a session",
  body: "Your camera, your screen, your microphone, code activity, and workspace behaviour are part of a session. None of it is continuous video or audio: the camera is kept as a still every half minute; your entire screen is shared and kept as a still when it changes, every few minutes, and at flagged moments; the microphone is listened to in your browser, and only when voices or a loud noise last a few seconds is a short clip kept. Moments such as leaving the workspace, leaving fullscreen, a large paste from elsewhere, a second display or window, nobody or more than one person on camera, or a developer-tools shortcut are flagged as they happen, with the camera, screen and sound from that moment, for the hiring team to look at — a flag is not a decision. Nothing is recorded until you click Enter Workspace on the final onboarding step.",
};

/** The short form of HOW_EVIDENCE_IS_USED, leading into EVIDENCE_POINTS. */
export const EVIDENCE_LEAD = "Mindfries doesn't score your code. It writes an evidence report for the hiring team that shows:";

/** The four things the evidence report describes, from HOW_EVIDENCE_IS_USED. */
export const EVIDENCE_POINTS = [
  { key: "approach", label: "Your approach", detail: "What you explored and how you thought about the problem" },
  { key: "debugging", label: "Your debugging", detail: "Where you got stuck and how you worked it out" },
  { key: "testing", label: "Your testing", detail: "Which tests you ran and how you checked your solution" },
  { key: "communication", label: "Your communication", detail: "How you explained your decisions" },
];

export const HOW_EVIDENCE_IS_USED = {
  title: "How evidence is used",
  body: "Mindfries doesn't score your final code. Instead, it produces an evidence report for the hiring team that describes how you approached the problem — what you explored, where you debugged, how you used tests, and how you explained your decisions. The hiring team makes the final call.",
};

export const RETENTION = {
  title: "Retention",
  body: "Session recordings and activity data are retained for 90 days and then permanently deleted, unless you request earlier deletion.",
};

/**
 * What a candidate may and may not use during a session. Part of what they
 * agree to in onboarding (ConsentStep). Worded to match what the workspace
 * actually does: nothing records which outside sites or tools are used —
 * leaving the workspace, large pastes and second windows are what's flagged.
 */
export const TOOL_RULES = {
  title: "Allowed tools",
  body: "Work the way you normally would, with the resources an engineer reaches for — but the thinking has to be yours.",
  allowed: [
    { key: "docs", label: "Documentation and references" },
    { key: "cloud", label: "Cloud platforms (e.g. Supabase, Firebase)" },
    { key: "apis", label: "APIs and external services" },
    { key: "devtools", label: "Non-AI developer tools" },
    { key: "packages", label: "Package managers (e.g. npm, pip)" },
    { key: "assistant", label: "The workspace's own AI assistant, when the company has turned it on" },
  ],
  notAllowed: [
    { key: "ai", label: "Other AI tools (e.g. ChatGPT, Claude, Copilot)" },
    { key: "people", label: "Help from other people" },
  ],
  note: "Leaving the workspace, large pastes from elsewhere and second windows or displays are flagged for the hiring team to look at. A flag is not a decision.",
};

/** Where to ask for a candidate's data to be deleted before the 90 days are up. */
export const PRIVACY_CONTACT = "officemindfries@gmail.com";

/** Said beside RETENTION, followed by PRIVACY_CONTACT as a mail link. */
export const DELETION = {
  before: "You can ask for your data to be deleted sooner, at any time: email",
  after: "from your account's address.",
};
