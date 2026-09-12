import type { Health } from "../lib/types";

type Props = {
  health: Health | null;
  connected: boolean;
  activeProject: string | null;
  dirtyCount: number;
  onTogglePanel: () => void;
};

export default function StatusBar({
  health,
  connected,
  activeProject,
  dirtyCount,
  onTogglePanel,
}: Props) {
  return (
    <footer className="flex h-6 shrink-0 items-center gap-4 border-t border-line bg-shell px-3 text-[11px] text-ink-dim">
      <span
        className={connected ? "text-ink-dim" : "text-warn"}
        title={connected ? "Connected to the daemon" : "Not connected to the daemon"}
      >
        {connected ? "⎇ connected" : "⚠ offline"}
      </span>

      {activeProject && <span className="truncate">{activeProject}</span>}

      {health && (
        <>
          <span>
            {health.memories} {health.memories === 1 ? "memory" : "memories"} in {health.projects}{" "}
            {health.projects === 1 ? "project" : "projects"}
          </span>
          {/* Phase 6 replaces this with the token counters the design calls for.
              Index size is the honest thing to show until then. */}
          <span title="Derived index — safe to delete">
            index {Math.round(health.indexBytes / 1024)} KB
          </span>
        </>
      )}

      {dirtyCount > 0 && <span className="text-warn">{dirtyCount} unsaved</span>}

      <button
        type="button"
        onClick={onTogglePanel}
        title="Ctrl+J"
        className="ml-auto hover:text-ink"
      >
        Panel
      </button>
      <span>UTF-8</span>
    </footer>
  );
}
