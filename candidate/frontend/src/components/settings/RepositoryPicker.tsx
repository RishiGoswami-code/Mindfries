"use client";

import { useState } from "react";
import { ExternalLink, Search, Star } from "lucide-react";
import { setProjectHidden } from "@/lib/settings/actions";
import { SavedSwitch } from "./Switch";

export interface PickerProject {
  url: string;
  name: string;
  description: string;
  language: string | null;
  stars: number;
  source: "github" | "gitlab";
  handle: string;
}

/**
 * The projects read from the candidate's linked GitHub and GitLab accounts,
 * each with a switch for whether hiring teams see it. Only projects that
 * were read are here: private ones are counted on the account and never
 * named, so there's nothing to list for them.
 */
export function RepositoryPicker({
  projects,
  hidden,
  privateCount,
  disabled,
}: {
  projects: PickerProject[];
  hidden: string[];
  privateCount: number;
  disabled?: boolean;
}) {
  const [query, setQuery] = useState("");
  const hiddenSet = new Set(hidden);
  const needle = query.trim().toLowerCase();
  const list = needle ? projects.filter((p) => [p.name, p.description, p.language ?? ""].some((f) => f.toLowerCase().includes(needle))) : projects;
  const sources = new Set(projects.map((p) => p.source));

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-[13.5px] text-[#5E5A82]">
          {projects.length} public {projects.length === 1 ? "project" : "projects"}
          {privateCount > 0 && ` · ${privateCount} private, counted but never named`}
        </p>
        <label className="relative block w-full sm:w-64">
          <Search size={16} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-[#8A86A8]" />
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search projects…"
            aria-label="Search projects"
            className="h-10 w-full rounded-xl border border-[#E4E0F2] bg-white pr-3 pl-9 text-[14px] text-[#1B1840] outline-none placeholder:text-[#A9A5C2] focus:border-[#B9A8F5]"
          />
        </label>
      </div>

      <div className="mt-3 overflow-hidden rounded-2xl border border-[#ECE9F6] bg-white/80">
        <div className="hidden grid-cols-[minmax(0,1.2fr)_110px_minmax(0,1.6fr)_120px] gap-4 border-b border-[#ECE9F6] bg-[#FAF9FE] px-4 py-2.5 text-[12.5px] font-medium text-[#6E6A8F] md:grid">
          <span>Project</span>
          <span>Language</span>
          <span>Description</span>
          <span className="text-right">Show to hiring teams</span>
        </div>
        {list.length === 0 ? (
          <p className="px-4 py-8 text-center text-[13.5px] text-[#6E6A8F]">No project matches “{query}”.</p>
        ) : (
          <ul className="divide-y divide-[#F0EEF7]">
            {list.map((p) => (
              <li key={p.url} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-1 px-4 py-3 md:grid-cols-[minmax(0,1.2fr)_110px_minmax(0,1.6fr)_120px]">
                <div className="min-w-0">
                  <a href={p.url} target="_blank" rel="noreferrer" className="inline-flex max-w-full items-center gap-1.5 text-[14.5px] font-medium text-[#100E2E] hover:text-[#5B3BE0]">
                    <span className="truncate">{p.name}</span>
                    <ExternalLink size={13} className="shrink-0 text-[#8A86A8]" />
                  </a>
                  <p className="text-[12px] text-[#8A86A8]">
                    {sources.size > 1 && `${p.source === "github" ? "GitHub" : "GitLab"} · `}
                    {p.stars > 0 && (
                      <span className="inline-flex items-center gap-0.5">
                        <Star size={11} /> {p.stars}
                      </span>
                    )}
                  </p>
                </div>
                <span className="hidden text-[13px] text-[#4C4874] md:block">{p.language ?? "—"}</span>
                <span className="col-span-2 row-start-2 truncate text-[13px] text-[#6E6A8F] md:col-span-1 md:row-start-auto">{p.description || "No description"}</span>
                <span className="col-start-2 row-start-1 flex justify-end md:col-start-auto md:row-start-auto">
                  <SavedSwitch
                    initial={!hiddenSet.has(p.url)}
                    save={(show) => setProjectHidden(p.url, !show)}
                    label={`Show ${p.name} to hiring teams`}
                    disabled={disabled}
                  />
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
