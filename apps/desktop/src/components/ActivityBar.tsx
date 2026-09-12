import { ExplorerIcon, GraphIcon, PaletteIcon, PlugIcon, SearchIcon, SettingsIcon } from "./Icons";

export type View = "explorer" | "search" | "graph" | "mcp" | "theme" | "settings";

const views: { id: View; label: string; hint: string; Icon: (props: { className?: string }) => React.JSX.Element }[] = [
  { id: "explorer", label: "Explorer", hint: "Ctrl+Shift+E", Icon: ExplorerIcon },
  { id: "search", label: "Search", hint: "Ctrl+Shift+F", Icon: SearchIcon },
  { id: "graph", label: "Knowledge Graph", hint: "", Icon: GraphIcon },
  { id: "mcp", label: "AI & MCP Clients", hint: "", Icon: PlugIcon },
  { id: "theme", label: "Appearance & Themes", hint: "", Icon: (p) => <PaletteIcon className={p.className ?? "h-6 w-6"} /> },
  { id: "settings", label: "Settings", hint: "", Icon: SettingsIcon },
];

type Props = {
  active: View;
  sidebarOpen: boolean;
  onSelect: (view: View) => void;
};

export default function ActivityBar({ active, sidebarOpen, onSelect }: Props) {
  return (
    <nav
      aria-label="Activity bar"
      className="flex w-12 shrink-0 flex-col items-center border-r border-line bg-shell py-1"
    >
      {views.map(({ id, label, hint, Icon }) => {
        const isContentMode = id === "mcp" || id === "theme" || id === "settings";
        const current = isContentMode ? active === id : active === id && sidebarOpen;
        return (
          <button
            key={id}
            type="button"
            aria-label={hint ? `${label} (${hint})` : label}
            aria-current={current}
            title={hint ? `${label}  ${hint}` : label}
            onClick={() => onSelect(id)}
            className={`relative flex h-12 w-12 items-center justify-center transition-colors ${
              current ? "text-ink" : "text-ink-faint hover:text-ink"
            }`}
          >
            {current && <span className="absolute left-0 top-0 h-full w-0.5 bg-ink" />}
            <Icon className="h-6 w-6" />
          </button>
        );
      })}
    </nav>
  );
}
