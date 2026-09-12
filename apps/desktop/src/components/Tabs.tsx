import type { OpenTab, ViewId } from "../lib/tabs";
import {
  CloseIcon,
  DocIcon,
  GraphIcon,
  PaletteIcon,
  PlugIcon,
  SettingsIcon,
} from "./Icons";

type Props = {
  tabs: OpenTab[];
  activeKey: string | null;
  onSelect: (key: string) => void;
  onClose: (key: string) => void;
};

const VIEW_ICONS: Record<ViewId, (p: { className?: string }) => React.JSX.Element> = {
  graph: GraphIcon,
  mcp: PlugIcon,
  theme: PaletteIcon,
  settings: SettingsIcon,
};

/** Accent per view, so a tab is recognisable before its label is read. */
const VIEW_TINTS: Record<ViewId, string> = {
  graph: "text-tag",
  mcp: "text-accent",
  theme: "text-accent",
  settings: "text-ink-dim",
};

/**
 * The four convention slugs get their own mark. A memory library is mostly
 * these four per project, so tinting them is what makes a row of tabs scannable
 * rather than a row of identical document icons.
 */
const SLUG_MARKS: Record<string, { glyph: string; tint: string }> = {
  todo: { glyph: "☑", tint: "text-warn" },
  decisions: { glyph: "◆", tint: "text-accent" },
  conventions: { glyph: "§", tint: "text-tag" },
  "dev-log": { glyph: "❯", tint: "text-ink-dim" },
};

export default function Tabs({ tabs, activeKey, onSelect, onClose }: Props) {
  if (tabs.length === 0) return null;

  return (
    <div
      role="tablist"
      className="flex shrink-0 items-stretch overflow-x-auto border-b border-line bg-shell select-none"
    >
      {tabs.map((tab) => {
        const active = tab.key === activeKey;
        const label = tab.kind === "memory" ? tab.title || tab.slug || "Untitled" : tab.title;

        return (
          <div
            key={tab.key}
            role="tab"
            aria-selected={active}
            title={tab.kind === "memory" ? `${tab.project}/${tab.slug ?? "new"}` : tab.title}
            className={`group relative flex max-w-56 shrink-0 items-center gap-2 border-r border-line pl-3 pr-2 py-1.5 transition-colors ${
              active ? "bg-editor text-ink" : "text-ink-dim hover:bg-hover/50 hover:text-ink"
            }`}
          >
            {/* Top rule on the active tab rather than VS Code's, so the strip
                reads as Mnemosyne's without losing the "this one" signal. */}
            {active && <span className="absolute inset-x-0 top-0 h-0.5 bg-accent" />}

            <button
              type="button"
              onClick={() => onSelect(tab.key)}
              className="flex min-w-0 items-center gap-1.5"
            >
              <TabMark tab={tab} />
              <span className="truncate text-xs">{label}</span>
            </button>

            <button
              type="button"
              aria-label={`Close ${label}`}
              onClick={(e) => {
                e.stopPropagation();
                onClose(tab.key);
              }}
              className={`rounded p-0.5 hover:bg-hover ${
                active || (tab.kind === "memory" && tab.dirty)
                  ? "opacity-70"
                  : "opacity-0 group-hover:opacity-70"
              }`}
            >
              {tab.kind === "memory" && tab.dirty ? (
                <span className="block h-3 w-3 text-center text-[10px] leading-3 text-warn group-hover:hidden">
                  ●
                </span>
              ) : null}
              <CloseIcon
                className={`h-3 w-3 ${
                  tab.kind === "memory" && tab.dirty ? "hidden group-hover:block" : ""
                }`}
              />
            </button>
          </div>
        );
      })}
    </div>
  );
}

/** The glyph at the head of a tab: a view's icon, or a memory's kind. */
function TabMark({ tab }: { tab: OpenTab }) {
  if (tab.kind === "view") {
    const Icon = VIEW_ICONS[tab.view];
    return <Icon className={`h-3.5 w-3.5 shrink-0 ${VIEW_TINTS[tab.view]}`} />;
  }

  const mark = tab.slug ? SLUG_MARKS[tab.slug] : undefined;
  if (mark) {
    return (
      <span className={`shrink-0 text-[11px] leading-none ${mark.tint}`} aria-hidden>
        {mark.glyph}
      </span>
    );
  }
  return <DocIcon className="h-3.5 w-3.5 shrink-0 text-ink-faint" />;
}
