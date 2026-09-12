import type { OpenTab } from "../lib/tabs";
import { CloseIcon, GraphIcon, PaletteIcon, PlugIcon, SettingsIcon } from "./Icons";

export type MainMode = "editor" | "graph" | "mcp" | "theme" | "settings";

type Props = {
  tabs: OpenTab[];
  activeKey: string | null;
  onSelect: (key: string) => void;
  onClose: (key: string) => void;
  mainMode: MainMode;
  onCloseSpecial: () => void;
  onSelectMode: (mode: MainMode) => void;
};

export default function Tabs({
  tabs,
  activeKey,
  onSelect,
  onClose,
  mainMode,
  onCloseSpecial,
  onSelectMode,
}: Props) {
  const isSpecialActive = mainMode !== "editor";
  if (tabs.length === 0 && !isSpecialActive) return null;

  return (
    <div role="tablist" className="flex shrink-0 items-center overflow-x-auto border-b border-line bg-shell select-none">
      {/* Open memory tabs */}
      {tabs.map((tab) => {
        const active = !isSpecialActive && tab.key === activeKey;
        return (
          <div
            key={tab.key}
            role="tab"
            aria-selected={active}
            className={`group flex max-w-56 shrink-0 items-center gap-2 border-r border-line px-3 py-1.5 transition-colors ${
              active ? "bg-editor text-ink font-medium" : "text-ink-dim hover:text-ink hover:bg-hover/50"
            }`}
          >
            <button
              type="button"
              onClick={() => onSelect(tab.key)}
              className="flex min-w-0 items-center gap-1.5"
              title={`${tab.project}/${tab.slug ?? "new"}`}
            >
              <span className={`text-[10px] ${tab.dirty ? "text-warn font-bold" : "text-ink-faint"}`}>
                {tab.dirty ? "●" : "◇"}
              </span>
              <span className="truncate text-xs">{tab.title || tab.slug || "Untitled"}</span>
            </button>
            <button
              type="button"
              aria-label={`Close ${tab.title || tab.slug || "Untitled"}`}
              onClick={(e) => {
                e.stopPropagation();
                onClose(tab.key);
              }}
              className={`rounded p-0.5 hover:bg-hover ${
                active ? "opacity-70" : "opacity-0 group-hover:opacity-70"
              }`}
            >
              <CloseIcon className="h-3 w-3" />
            </button>
          </div>
        );
      })}

      {/* Special Content Pane Tabs */}
      {mainMode === "mcp" && (
        <div
          role="tab"
          aria-selected={true}
          className="group flex max-w-56 shrink-0 items-center gap-2 border-r border-line bg-editor px-3 py-1.5 text-ink font-medium shadow-xs"
        >
          <div className="flex min-w-0 items-center gap-1.5 text-xs text-ink">
            <PlugIcon className="h-3.5 w-3.5 text-accent" />
            <span className="truncate">MCP Clients</span>
          </div>
          <button
            type="button"
            aria-label="Close MCP Clients tab"
            onClick={onCloseSpecial}
            className="rounded p-0.5 text-ink-dim hover:bg-hover hover:text-ink transition-colors"
          >
            <CloseIcon className="h-3 w-3" />
          </button>
        </div>
      )}

      {mainMode === "theme" && (
        <div
          role="tab"
          aria-selected={true}
          className="group flex max-w-56 shrink-0 items-center gap-2 border-r border-line bg-editor px-3 py-1.5 text-ink font-medium shadow-xs"
        >
          <div className="flex min-w-0 items-center gap-1.5 text-xs text-ink">
            <PaletteIcon className="h-3.5 w-3.5 text-accent" />
            <span className="truncate">Theme Studio</span>
          </div>
          <button
            type="button"
            aria-label="Close Theme Studio tab"
            onClick={onCloseSpecial}
            className="rounded p-0.5 text-ink-dim hover:bg-hover hover:text-ink transition-colors"
          >
            <CloseIcon className="h-3 w-3" />
          </button>
        </div>
      )}

      {mainMode === "settings" && (
        <div
          role="tab"
          aria-selected={true}
          className="group flex max-w-56 shrink-0 items-center gap-2 border-r border-line bg-editor px-3 py-1.5 text-ink font-medium shadow-xs"
        >
          <div className="flex min-w-0 items-center gap-1.5 text-xs text-ink">
            <SettingsIcon className="h-3.5 w-3.5 text-accent" />
            <span className="truncate">Settings</span>
          </div>
          <button
            type="button"
            aria-label="Close Settings tab"
            onClick={onCloseSpecial}
            className="rounded p-0.5 text-ink-dim hover:bg-hover hover:text-ink transition-colors"
          >
            <CloseIcon className="h-3 w-3" />
          </button>
        </div>
      )}

      {mainMode === "graph" && (
        <div
          role="tab"
          aria-selected={true}
          className="group flex max-w-56 shrink-0 items-center gap-2 border-r border-line bg-editor px-3 py-1.5 text-ink font-medium shadow-xs"
        >
          <div className="flex min-w-0 items-center gap-1.5 text-xs text-ink">
            <GraphIcon className="h-3.5 w-3.5 text-tag" />
            <span className="truncate">Knowledge Graph</span>
          </div>
          <button
            type="button"
            aria-label="Close Knowledge Graph tab"
            onClick={onCloseSpecial}
            className="rounded p-0.5 text-ink-dim hover:bg-hover hover:text-ink transition-colors"
          >
            <CloseIcon className="h-3 w-3" />
          </button>
        </div>
      )}

      {/* Right side quick view shortcuts */}
      <div className="ml-auto flex shrink-0 items-center gap-1 px-2 text-xs">
        <button
          type="button"
          title="Toggle Knowledge Graph View"
          onClick={() => onSelectMode(mainMode === "graph" ? "editor" : "graph")}
          className={`flex items-center gap-1 rounded px-2 py-1 text-[11px] transition-colors ${
            mainMode === "graph" ? "bg-hover text-ink font-semibold" : "text-ink-faint hover:text-ink hover:bg-hover/50"
          }`}
        >
          <GraphIcon className="h-3 w-3 text-tag" />
          <span>Graph</span>
        </button>

        <button
          type="button"
          title="Open MCP Clients View"
          onClick={() => onSelectMode(mainMode === "mcp" ? "editor" : "mcp")}
          className={`flex items-center gap-1 rounded px-2 py-1 text-[11px] transition-colors ${
            mainMode === "mcp" ? "bg-hover text-ink font-semibold" : "text-ink-faint hover:text-ink hover:bg-hover/50"
          }`}
        >
          <PlugIcon className="h-3 w-3 text-accent" />
          <span>MCP</span>
        </button>

        <button
          type="button"
          title="Open Theme Studio"
          onClick={() => onSelectMode(mainMode === "theme" ? "editor" : "theme")}
          className={`flex items-center gap-1 rounded px-2 py-1 text-[11px] transition-colors ${
            mainMode === "theme" ? "bg-hover text-ink font-semibold" : "text-ink-faint hover:text-ink hover:bg-hover/50"
          }`}
        >
          <PaletteIcon className="h-3 w-3 text-accent" />
          <span>Theme</span>
        </button>

        <button
          type="button"
          title="Open Settings"
          onClick={() => onSelectMode(mainMode === "settings" ? "editor" : "settings")}
          className={`flex items-center gap-1 rounded px-2 py-1 text-[11px] transition-colors ${
            mainMode === "settings" ? "bg-hover text-ink font-semibold" : "text-ink-faint hover:text-ink hover:bg-hover/50"
          }`}
        >
          <SettingsIcon className="h-3 w-3" />
        </button>
      </div>
    </div>
  );
}
