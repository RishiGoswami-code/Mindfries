"use client";

import { useMemo, useState } from "react";
import clsx from "clsx";
import type { Diagram, PanelConfig, PanelTabId } from "@/lib/v2/panel/types";
import { firstTab } from "@/lib/v2/panel/tabs";
import { PanelHeader } from "./PanelHeader";
import { PanelRail } from "./PanelRail";
import { CameraTile } from "./CameraTile";
import { AvatarCard } from "./AvatarCard";
import { MinimisedCard } from "./MinimisedCard";
import { SystemDesignTab } from "./tabs/SystemDesignTab";
import { NotesTab } from "./tabs/NotesTab";
import { ResourcesTab } from "./tabs/ResourcesTab";
import { ReviewTab } from "./tabs/ReviewTab";
import { DebugTab } from "./tabs/DebugTab";
import { TestTab } from "./tabs/TestTab";
import { ActivityTab } from "./tabs/ActivityTab";

/**
 * The V2 floating panel (v2/PANEL.md): the candidate's cockpit for the
 * whole session. Expanded, it's the tablet-sized card: workspace tabs on the
 * left, the active tab in the middle, the camera and the AI avatar on the
 * right. Minimised, everything folds into the avatar card.
 *
 * Step 1: the layouts and tabs, with mock data. Popping out above other
 * windows (Document Picture-in-Picture) is step 2; live data is step 3.
 */
export function FloatingPanel({ config }: { config: PanelConfig }) {
  const [minimised, setMinimised] = useState(false);
  const [tab, setTab] = useState<PanelTabId>(() => firstTab(config.tabs, "design"));
  const [diagram, setDiagram] = useState<Diagram>(config.diagram);
  const [notes, setNotes] = useState(config.notes);
  const [seenActivity, setSeenActivity] = useState(false);

  const flags = useMemo(() => config.activity.filter((a) => a.severity !== "info").length, [config.activity]);
  const unread = seenActivity ? 0 : flags;
  const tabs = config.tabs.map((t) => (t.id === "activity" ? { ...t, badge: unread || undefined } : t));
  const active = tabs.find((t) => t.id === tab) ?? tabs[0];

  const open = (id: PanelTabId) => {
    setTab(id);
    if (id === "activity") setSeenActivity(true);
  };

  if (minimised) {
    return (
      <MinimisedCard
        config={config}
        unread={unread}
        onExpand={() => setMinimised(false)}
        onOpenActivity={() => {
          setMinimised(false);
          open("activity");
        }}
      />
    );
  }

  return (
    <div
      className={clsx(
        "flex h-[680px] w-[1180px] max-w-full flex-col overflow-hidden rounded-[28px] border border-white/70 bg-white/95",
        "shadow-[0_30px_80px_-20px_rgba(37,73,160,0.35)] ring-1 ring-[#DCE6FA] backdrop-blur",
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
        onMinimise={() => setMinimised(true)}
      />
      <div className="flex min-h-0 flex-1 gap-4 px-4 pb-4">
        <PanelRail tabs={tabs} active={active.id} onSelect={open} />
        <main className="min-w-0 flex-1 overflow-hidden rounded-2xl border border-[#E8EDF7] bg-white" aria-label={active.label}>
          {active.id === "design" && <SystemDesignTab diagram={diagram} onChange={setDiagram} />}
          {active.id === "notes" && <NotesTab value={notes} onChange={setNotes} />}
          {active.id === "resources" && <ResourcesTab resources={config.resources} />}
          {active.id === "review" && <ReviewTab review={config.review} />}
          {active.id === "debug" && <DebugTab debug={config.debug} />}
          {active.id === "test" && <TestTab test={config.test} />}
          {active.id === "activity" && <ActivityTab items={config.activity} />}
        </main>
        <aside className="flex w-[260px] shrink-0 flex-col gap-4">
          <CameraTile required={config.monitoring.camera} />
          <AvatarCard name={config.avatar.name} className="flex-1" />
        </aside>
      </div>
    </div>
  );
}
