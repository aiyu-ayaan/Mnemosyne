import type { ChangeEvent, Hit, Meta } from "../lib/types";
import { CloseIcon } from "./Icons";

export type PanelTab = "search" | "backlinks" | "mcp" | "output";

type Props = {
  tab: PanelTab;
  onTab: (tab: PanelTab) => void;
  onClose: () => void;

  query: string;
  results: Hit[];
  searchError: string | null;

  backlinks: Meta[];
  backlinksFor: string | null;

  endpoint: string | null;
  daemonRoot: string | null;

  log: (ChangeEvent | { kind: "error"; message: string; at: string })[];

  onOpenHit: (project: string, slug: string) => void;
};

const tabs: { id: PanelTab; label: string }[] = [
  { id: "search", label: "Search" },
  { id: "backlinks", label: "Backlinks" },
  { id: "mcp", label: "MCP" },
  { id: "output", label: "Output" },
];

export default function Panel({
  tab,
  onTab,
  onClose,
  query,
  results,
  searchError,
  backlinks,
  backlinksFor,
  endpoint,
  daemonRoot,
  log,
  onOpenHit,
}: Props) {
  return (
    <section className="flex h-56 shrink-0 flex-col border-t border-line bg-shell">
      <div role="tablist" className="flex shrink-0 items-center gap-1 border-b border-line px-2">
        {tabs.map(({ id, label }) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={tab === id}
            onClick={() => onTab(id)}
            className={`border-b-2 px-2 py-1.5 text-[11px] font-semibold uppercase tracking-wide ${
              tab === id
                ? "border-ink text-ink"
                : "border-transparent text-ink-faint hover:text-ink-dim"
            }`}
          >
            {label}
          </button>
        ))}
        <button
          type="button"
          aria-label="Close panel (Ctrl+J)"
          title="Ctrl+J"
          onClick={onClose}
          className="ml-auto rounded p-1 text-ink-faint hover:bg-hover hover:text-ink"
        >
          <CloseIcon />
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-2 py-1">
        {tab === "search" && (
          <>
            {searchError && <p className="px-1 py-1 text-danger">{searchError}</p>}
            {!searchError && query === "" && (
              <p className="px-1 py-1 text-ink-faint">Search from the sidebar to see results here.</p>
            )}
            {!searchError && query !== "" && (
              <p className="px-1 py-1 text-ink-dim">
                {results.length} {results.length === 1 ? "result" : "results"} for “{query}”
              </p>
            )}
            {results.map((hit) => (
              <button
                key={`${hit.project}/${hit.slug}`}
                type="button"
                onClick={() => onOpenHit(hit.project, hit.slug)}
                className="block w-full rounded px-1 py-1 text-left hover:bg-hover"
              >
                <span className="text-ink">{hit.title || hit.slug}</span>
                <span className="ml-2 font-mono text-[11px] text-ink-faint">
                  {hit.project}/{hit.slug}
                </span>
                {hit.snippet && (
                  <span className="mt-0.5 block truncate text-ink-dim">{hit.snippet}</span>
                )}
              </button>
            ))}
          </>
        )}

        {tab === "backlinks" && (
          <>
            {!backlinksFor && (
              <p className="px-1 py-1 text-ink-faint">
                Open a memory to see what links to it.
              </p>
            )}
            {backlinksFor && backlinks.length === 0 && (
              <p className="px-1 py-1 text-ink-faint">Nothing links to {backlinksFor} yet.</p>
            )}
            {backlinks.map((meta) => (
              <button
                key={`${meta.project}/${meta.slug}`}
                type="button"
                onClick={() => onOpenHit(meta.project, meta.slug)}
                className="block w-full rounded px-1 py-1 text-left hover:bg-hover"
              >
                <span className="text-ink">{meta.title || meta.slug}</span>
                <span className="ml-2 font-mono text-[11px] text-ink-faint">
                  {meta.project}/{meta.slug}
                </span>
              </button>
            ))}
          </>
        )}

        {tab === "mcp" && (
          <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 px-1 py-1 text-ink-dim">
            <dt>daemon</dt>
            <dd className="selectable break-all font-mono text-[11px]">
              {endpoint ?? "not connected"}
            </dd>
            <dt>root</dt>
            <dd className="selectable break-all font-mono text-[11px]">{daemonRoot ?? "—"}</dd>
            <dt>agents</dt>
            <dd>
              <code className="font-mono text-[11px]">mnemosyne serve</code> over stdio, per session
            </dd>
          </dl>
        )}

        {tab === "output" && (
          <>
            {log.length === 0 && <p className="px-1 py-1 text-ink-faint">Nothing yet.</p>}
            {/* Newest first: the reason someone opens this panel is the thing
                that just happened. */}
            {[...log].reverse().map((entry, i) => (
              <p key={i} className="selectable px-1 font-mono text-[11px] text-ink-dim">
                <span className="text-ink-faint">
                  {new Date(entry.at).toLocaleTimeString()}{" "}
                </span>
                <span className={entry.kind === "error" ? "text-danger" : "text-ink"}>
                  {entry.kind}
                </span>{" "}
                {"message" in entry
                  ? entry.message
                  : [entry.project, entry.memory].filter(Boolean).join("/")}
              </p>
            ))}
          </>
        )}
      </div>
    </section>
  );
}
