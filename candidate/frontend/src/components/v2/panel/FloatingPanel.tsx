"use client";

import { useMemo, useState } from "react";
import clsx from "clsx";
import type { Diagram, PanelConfig, PanelTabId } from "@/lib/v2/panel/types";
import { firstTab } from "@/lib/v2/panel/tabs";
import { PanelHeader } from "./PanelHeader";
import { PanelRail } from "./PanelRail";
import { CameraTile } from "./CameraTile";
import { MonitorCard } from "./MonitorCard";
import { AvatarCard } from "./AvatarCard";
import { MinimisedCard } from "./MinimisedCard";
import { SystemDesignTab } from "./tabs/SystemDesignTab";
import { NotesTab } from "./tabs/NotesTab";
import { ResourcesTab } from "./tabs/ResourcesTab";
import { ReviewTab } from "./tabs/ReviewTab";
import { DebugTab } from "./tabs/DebugTab";
import { TestTab } from "./tabs/TestTab";
import { ActivityTab } from "./tabs/ActivityTab";
import { useMockAvatar } from "./useMockAvatar";
import type { AvatarController } from "./avatar";
import type { Monitoring } from "./useMonitoring";
import type { PanelData } from "./usePanelData";

/** What makes the panel live: the API's data, the browser's sharing, the avatar's voice, the pop-out. */
export interface LiveControls {
  data: PanelData;
  monitoring: Monitoring;
  avatar: AvatarController;
  popOut?: { open: () => void; close: () => void; isOpen: boolean; resize: (minimised: boolean) => void };
}

const SAVE_WORDS = { saved: "All saved", saving: "Saving...", error: "Couldn't save: will retry", conflict: "Changed in another window" } as const;

/**
 * The V2 floating panel (v2/PANEL.md): the candidate's cockpit for the whole
 * session. Expanded, it's the tablet-sized card: workspace tabs on the left,
 * the active tab in the middle, what's shared and the AI avatar on the right.
 * Minimised, everything folds into the avatar card.
 *
 * With `live` it runs on the API's data, with sharing, saving and the avatar
 * wired up. Without it (the design-review page) it works on what it is given.
 */
export function FloatingPanel({ config: given, live, fill }: { config: PanelConfig; live?: LiveControls; fill?: boolean }) {
  const config = live ? live.data.config : given;
  const mock = useMockAvatar();
  const avatar = live?.avatar ?? mock;

  const [minimised, setMinimised] = useState(false);
  const [tab, setTab] = useState<PanelTabId>(() => firstTab(config.tabs, config.tabs.find((t) => t.id === "debug") ? "debug" : "design"));
  const [localDiagram, setLocalDiagram] = useState<Diagram>(given.diagram);
  const [localNotes, setLocalNotes] = useState(given.notes);
  const [seenActivity, setSeenActivity] = useState(false);

  const flags = useMemo(() => config.activity.filter((a) => a.severity !== "info").length, [config.activity]);
  // Unread means flags and notices that arrived since the Activity tab was last open.
  const [seenCount, setSeenCount] = useState(0);
  const unread = live ? Math.max(0, flags - seenCount) : seenActivity ? 0 : flags;
  const tabs = config.tabs.map((t) => (t.id === "activity" ? { ...t, badge: unread || undefined } : t));
  const active = tabs.find((t) => t.id === tab) ?? tabs[0];

  const open = (id: PanelTabId) => {
    setTab(id);
    if (id === "activity") {
      setSeenActivity(true);
      setSeenCount(flags);
    }
  };
  const setMin = (m: boolean) => {
    setMinimised(m);
    live?.popOut?.resize(m);
  };

  if (minimised) {
    return (
      <MinimisedCard
        config={config}
        avatar={avatar}
        unread={unread}
        onExpand={() => setMin(false)}
        onOpenActivity={() => {
          setMin(false);
          open("activity");
        }}
      />
    );
  }

  const diagram = live ? config.diagram : localDiagram;
  const notes = live ? config.notes : localNotes;
  const m = live?.monitoring;

  return (
    <div
      className={clsx(
        "flex flex-col overflow-hidden border border-white/70 bg-white/95",
        fill ? "h-screen w-screen" : "h-[680px] w-[1180px] max-w-full rounded-[28px] shadow-[0_30px_80px_-20px_rgba(37,73,160,0.35)] ring-1 ring-[#DCE6FA] backdrop-blur",
      )}
      data-testid="v2-panel"
      data-state="expanded"
    >
      <PanelHeader
        activeId={active.id}
        activeLabel={active.label}
        endsAt={config.endsAt}
        unread={unread}
        onFlags={() => open("activity")}
        onMinimise={() => setMin(true)}
        saveNote={live ? SAVE_WORDS[live.data.saveState] : undefined}
        onPopOut={live?.popOut ? (live.popOut.isOpen ? live.popOut.close : live.popOut.open) : undefined}
        poppedOut={live?.popOut?.isOpen}
      />
      {live?.data.saveState === "conflict" && (
        <p role="alert" className="mx-4 mb-2 flex items-center gap-3 rounded-xl bg-[#FFF6E5] px-4 py-2 text-sm text-[#7A5200]">
          This was changed in another window.
          <button type="button" onClick={() => void live.data.reload()} className="font-medium underline">
            Load the latest
          </button>
        </p>
      )}
      {m && <StartBanner m={m} status={config.status ?? m.sessionStatus} />}
      <div className="flex min-h-0 flex-1 gap-4 px-4 pb-4">
        <PanelRail tabs={tabs} active={active.id} onSelect={open} />
        <main className="min-w-0 flex-1 overflow-hidden rounded-2xl border border-[#E8EDF7] bg-white" aria-label={active.label}>
          {active.id === "design" && <SystemDesignTab diagram={diagram} onChange={live ? live.data.saveDiagram : setLocalDiagram} />}
          {active.id === "notes" && <NotesTab value={notes} onChange={live ? live.data.saveNotes : setLocalNotes} />}
          {active.id === "resources" && <ResourcesTab resources={config.resources} />}
          {active.id === "review" && <ReviewTab review={config.review} onComments={live?.data.saveReview} />}
          {active.id === "debug" && <DebugTab debug={config.debug} onHypotheses={live?.data.saveHypotheses} />}
          {active.id === "test" && <TestTab test={config.test} />}
          {active.id === "activity" && <ActivityTab items={config.activity} onRespond={live?.data.respond} />}
        </main>
        <aside className="flex w-[260px] shrink-0 flex-col gap-4">
          {m ? <MonitorCard m={m} requires={config.monitoring} /> : <CameraTile required={config.monitoring.camera} />}
          <AvatarCard name={config.avatar.name} controller={avatar} className="flex-1" />
        </aside>
      </div>
    </div>
  );
}

/**
 * Above the tabs, until the clock can start: what still has to be on. And
 * once it runs, if a share stops: said plainly, as a gap in the record and not
 * a verdict, with a way back.
 */
function StartBanner({ m, status }: { m: Monitoring; status: string }) {
  const running = status === "active";
  const waitingOn = m.missing.length ? m.missing : m.todo;
  if (running && m.todo.length === 0) return null;
  if (!running && status !== "ready") return null;
  return (
    <div
      role="status"
      className={clsx("mx-4 mb-2 rounded-xl px-4 py-2.5 text-sm", running ? "bg-[#FFF6E5] text-[#7A5200]" : "bg-[#EEF3FE] text-[#274DB5]")}
    >
      {running ? (
        <>
          Sharing stopped: <strong>{m.todo.join(", ").toLowerCase()}</strong>. The gap is recorded; turn it back on in the Monitoring card.
        </>
      ) : waitingOn.length > 0 ? (
        <>
          Your clock starts when everything is on. Still needed: <strong>{waitingOn.join(", ")}</strong>.
        </>
      ) : (
        <>Everything is on. Go back to your terminal: the clock is starting.</>
      )}
    </div>
  );
}
