import { useState } from "react";
import { THEMES, type Health, type ThemeId } from "../lib/types";
import { PaletteIcon } from "./Icons";

type Props = {
  health: Health | null;
  connected: boolean;
  activeProject: string | null;
  dirtyCount: number;
  theme: ThemeId;
  onTheme: (theme: ThemeId) => void;
  onTogglePanel: () => void;
};

export default function StatusBar({
  health,
  connected,
  activeProject,
  dirtyCount,
  theme,
  onTheme,
  onTogglePanel,
}: Props) {
  const [showThemeMenu, setShowThemeMenu] = useState(false);
  const currentTheme = THEMES.find((t) => t.id === theme) || THEMES[0]!;

  return (
    <footer className="relative flex h-6 shrink-0 items-center gap-3 border-t border-line bg-shell px-3 text-[11px] text-ink-dim select-none">
      {/* Live connection dot */}
      <div className="flex items-center gap-1.5" title={connected ? "Daemon connected" : "Daemon offline"}>
        <span
          className={`inline-block h-2 w-2 rounded-full ${
            connected ? "bg-emerald-500 shadow-xs shadow-emerald-500/50 animate-pulse" : "bg-warn"
          }`}
        />
        <span className={connected ? "text-ink-dim" : "text-warn"}>
          {connected ? "daemon online" : "daemon offline"}
        </span>
      </div>

      {activeProject && (
        <>
          <span className="text-line">|</span>
          <span className="truncate text-ink font-medium">{activeProject}</span>
        </>
      )}

      {health && (
        <>
          <span className="text-line">|</span>
          <span className="font-mono text-[10.5px]">
            {health.memories} {health.memories === 1 ? "memory" : "memories"}
          </span>
          <span className="text-line">|</span>
          <span className="font-mono text-[10.5px] text-ink-faint">
            ~{Math.round((health.memories * 350))} tokens
          </span>
        </>
      )}

      {dirtyCount > 0 && (
        <span className="rounded bg-warn/20 px-1.5 py-0.2 font-mono text-[10px] text-warn font-semibold">
          ● {dirtyCount} unsaved
        </span>
      )}

      {/* Right aligned tools */}
      <div className="ml-auto flex items-center gap-3">
        {/* Quick Theme Switcher */}
        <div className="relative">
          <button
            type="button"
            onClick={() => setShowThemeMenu((v) => !v)}
            title="Change Theme"
            className="flex items-center gap-1 hover:text-ink transition-colors"
          >
            <PaletteIcon className="h-3 w-3 text-accent" />
            <span className="text-ink font-medium">{currentTheme.name}</span>
          </button>

          {showThemeMenu && (
            <>
              <div
                className="fixed inset-0 z-40"
                onClick={() => setShowThemeMenu(false)}
              />
              <div className="absolute bottom-6 right-0 z-50 w-44 rounded-md border border-line bg-raised/98 p-1.5 shadow-xl backdrop-blur-md">
                <div className="px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-ink-faint border-b border-line mb-1">
                  Color Theme
                </div>
                {THEMES.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => {
                      onTheme(item.id);
                      setShowThemeMenu(false);
                    }}
                    className={`flex w-full items-center justify-between rounded px-2 py-1 text-left text-xs transition-colors ${
                      theme === item.id ? "bg-hover text-ink font-semibold" : "text-ink-dim hover:bg-hover hover:text-ink"
                    }`}
                  >
                    <span className="truncate">{item.name}</span>
                    <span
                      className="h-2.5 w-2.5 rounded-full border border-line"
                      style={{ backgroundColor: item.accent }}
                    />
                  </button>
                ))}
              </div>
            </>
          )}
        </div>

        <span className="text-line">|</span>

        {/* Panel toggle */}
        <button
          type="button"
          onClick={onTogglePanel}
          title="Toggle Bottom Panel (Ctrl+J)"
          className="hover:text-ink transition-colors"
        >
          Panel
        </button>

        <span className="text-line">|</span>
        <span className="text-ink-faint font-mono text-[10px]">UTF-8</span>
      </div>
    </footer>
  );
}
