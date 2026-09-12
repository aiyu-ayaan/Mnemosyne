import type { OpenTab } from "../lib/tabs";
import { CloseIcon, GraphIcon } from "./Icons";

type Props = {
  tabs: OpenTab[];
  activeKey: string | null;
  onSelect: (key: string) => void;
  onClose: (key: string) => void;
  showingGraph?: boolean;
  onToggleGraph?: () => void;
};

export default function Tabs({
  tabs,
  activeKey,
  onSelect,
  onClose,
  showingGraph = false,
  onToggleGraph,
}: Props) {
  if (tabs.length === 0) return null;

  return (
    <div role="tablist" className="flex shrink-0 items-center overflow-x-auto border-b border-line bg-shell">

      {tabs.map((tab) => {
        const active = tab.key === activeKey;
        return (
          <div
            key={tab.key}
            role="tab"
            aria-selected={active}
            className={`group flex max-w-56 shrink-0 items-center gap-2 border-r border-line px-3 py-1.5 ${
              active ? "bg-editor text-ink" : "text-ink-dim hover:text-ink"
            }`}
          >
            <button
              type="button"
              onClick={() => onSelect(tab.key)}
              className="flex min-w-0 items-center gap-1.5"
              title={`${tab.project}/${tab.slug ?? "new"}`}
            >
              {/* A dirty marker, not a word: the dot is what a VS Code user
                  already reads as "unsaved". */}
              <span className={`text-[10px] ${tab.dirty ? "text-warn" : "text-ink-faint"}`}>
                {tab.dirty ? "●" : "◇"}
              </span>
              <span className="truncate">{tab.title || tab.slug || "Untitled"}</span>
            </button>
            <button
              type="button"
              aria-label={`Close ${tab.title || tab.slug || "Untitled"}`}
              onClick={() => onClose(tab.key)}
              className={`rounded p-0.5 hover:bg-hover ${
                active ? "opacity-70" : "opacity-0 group-hover:opacity-70"
              }`}
            >
              <CloseIcon />
            </button>
          </div>
        );
      })}
      {onToggleGraph && (
        <div className="ml-auto flex shrink-0 items-center px-2">
          <button
            type="button"
            title="Toggle Knowledge Graph View"
            onClick={onToggleGraph}
            className={`flex items-center gap-1 rounded px-2 py-1 text-[11px] transition-colors ${
              showingGraph ? "bg-hover text-ink font-semibold" : "text-ink-faint hover:text-ink"
            }`}
          >
            <GraphIcon className="h-3.5 w-3.5" />
            <span>Graph</span>
          </button>
        </div>
      )}
    </div>
  );
}
