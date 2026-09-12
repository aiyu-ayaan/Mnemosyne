import { useState } from "react";
import type { ChangeEvent, Hit, Meta } from "../lib/types";
import { CloseIcon, DocIcon, LinkIcon, SearchIcon } from "./Icons";

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
  const [expanded, setExpanded] = useState(false);

  const tabs: { id: PanelTab; label: string; count?: number }[] = [
    { id: "search", label: "Search Results", count: query ? results.length : undefined },
    { id: "backlinks", label: "Backlinks", count: backlinksFor ? backlinks.length : undefined },
    { id: "mcp", label: "MCP Daemon" },
    { id: "output", label: "Daemon Output", count: log.length ? log.length : undefined },
  ];

  return (
    <section
      className={`flex shrink-0 flex-col border-t border-line bg-shell text-ink-dim select-none transition-all duration-150 ${
        expanded ? "h-96" : "h-60"
      }`}
    >
      {/* Panel Header Tabs */}
      <div
        role="tablist"
        className="flex shrink-0 items-center gap-1 border-b border-line px-2 bg-shell/80"
      >
        {tabs.map(({ id, label, count }) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={tab === id}
            onClick={() => onTab(id)}
            className={`flex items-center gap-1.5 border-b-2 px-2.5 py-1.5 text-[11px] font-semibold uppercase tracking-wider transition-colors ${
              tab === id
                ? "border-accent text-ink"
                : "border-transparent text-ink-faint hover:text-ink-dim"
            }`}
          >
            <span>{label}</span>
            {count !== undefined && (
              <span
                className={`rounded-full px-1.5 py-0.2 font-mono text-[9.5px] ${
                  tab === id ? "bg-accent/20 text-accent" : "bg-raised text-ink-faint"
                }`}
              >
                {count}
              </span>
            )}
          </button>
        ))}

        {/* Right controls: expand / minimize / close */}
        <div className="ml-auto flex items-center gap-1">
          <button
            type="button"
            title={expanded ? "Restore panel height" : "Expand panel height"}
            onClick={() => setExpanded((v) => !v)}
            className="rounded p-1 text-ink-faint hover:bg-hover hover:text-ink transition-colors font-mono text-xs"
          >
            {expanded ? "▾" : "▴"}
          </button>
          <button
            type="button"
            aria-label="Close panel (Ctrl+J)"
            title="Close panel (Ctrl+J)"
            onClick={onClose}
            className="rounded p-1 text-ink-faint hover:bg-hover hover:text-ink transition-colors"
          >
            <CloseIcon />
          </button>
        </div>
      </div>

      {/* Panel Body Content */}
      <div className="min-h-0 flex-1 overflow-y-auto px-3 py-2">
        {/* Search Tab */}
        {tab === "search" && (
          <div className="flex flex-col gap-1.5">
            {searchError && (
              <div className="rounded border border-danger/30 bg-danger/10 px-2 py-1.5 text-xs text-danger">
                {searchError}
              </div>
            )}
            {!searchError && query === "" && (
              <div className="py-6 text-center text-xs text-ink-faint">
                <SearchIcon className="mx-auto h-6 w-6 text-ink-faint/50 mb-1" />
                <p>Run a search from the sidebar to inspect matching memories here.</p>
              </div>
            )}
            {!searchError && query !== "" && results.length === 0 && (
              <p className="py-4 text-center text-xs text-ink-faint">
                No memories matched “{query}”.
              </p>
            )}
            {results.map((hit) => (
              <button
                key={`${hit.project}/${hit.slug}`}
                type="button"
                onClick={() => onOpenHit(hit.project, hit.slug)}
                className="group flex flex-col rounded-md border border-line/60 bg-raised/40 p-2 text-left hover:border-accent hover:bg-raised transition-all"
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-1.5 font-medium text-ink">
                    <DocIcon className="h-3.5 w-3.5 text-accent" />
                    <span className="group-hover:text-accent transition-colors">
                      {hit.title || hit.slug}
                    </span>
                  </div>
                  <span className="font-mono text-[10px] text-ink-faint bg-shell px-1.5 py-0.5 rounded border border-line">
                    {hit.project}/{hit.slug}
                  </span>
                </div>
                {hit.snippet && (
                  <p className="mt-1 line-clamp-2 text-xs text-ink-dim leading-relaxed font-sans">
                    {hit.snippet}
                  </p>
                )}
                {hit.tags && hit.tags.length > 0 && (
                  <div className="mt-1.5 flex flex-wrap gap-1">
                    {hit.tags.map((t) => (
                      <span key={t} className="rounded bg-shell px-1 py-0.2 font-mono text-[9px] text-tag">
                        #{t}
                      </span>
                    ))}
                  </div>
                )}
              </button>
            ))}
          </div>
        )}

        {/* Backlinks Tab */}
        {tab === "backlinks" && (
          <div className="flex flex-col gap-1.5">
            {!backlinksFor && (
              <div className="py-6 text-center text-xs text-ink-faint">
                <LinkIcon className="mx-auto h-6 w-6 text-ink-faint/50 mb-1" />
                <p>Open a memory to see other memories linking to it via [[wikilinks]].</p>
              </div>
            )}
            {backlinksFor && backlinks.length === 0 && (
              <p className="py-4 text-center text-xs text-ink-faint">
                No memories link to <span className="font-mono text-ink">{backlinksFor}</span> yet.
              </p>
            )}
            {backlinks.map((meta) => (
              <button
                key={`${meta.project}/${meta.slug}`}
                type="button"
                onClick={() => onOpenHit(meta.project, meta.slug)}
                className="group flex items-center justify-between rounded-md border border-line/60 bg-raised/40 p-2 text-left hover:border-accent hover:bg-raised transition-all"
              >
                <div className="flex items-center gap-1.5 font-medium text-ink">
                  <LinkIcon className="h-3.5 w-3.5 text-accent" />
                  <span className="group-hover:text-accent transition-colors">
                    {meta.title || meta.slug}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  {meta.tags && meta.tags.length > 0 && (
                    <span className="font-mono text-[10px] text-tag">#{meta.tags[0]}</span>
                  )}
                  <span className="font-mono text-[10px] text-ink-faint bg-shell px-1.5 py-0.5 rounded border border-line">
                    {meta.project}/{meta.slug}
                  </span>
                </div>
              </button>
            ))}
          </div>
        )}

        {/* MCP Tab */}
        {tab === "mcp" && (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
            <div className="rounded-md border border-line bg-raised/40 p-3">
              <h4 className="font-semibold text-ink mb-2">Daemon Connection</h4>
              <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5 text-ink-dim">
                <dt className="text-ink-faint">Status</dt>
                <dd className="font-semibold text-tag">Connected & Listening</dd>
                <dt className="text-ink-faint">Endpoint</dt>
                <dd className="selectable break-all font-mono text-[11px] text-ink">
                  {endpoint ?? "not connected"}
                </dd>
                <dt className="text-ink-faint">Root</dt>
                <dd className="selectable break-all font-mono text-[11px] text-ink">
                  {daemonRoot ?? "—"}
                </dd>
              </dl>
            </div>

            <div className="rounded-md border border-line bg-raised/40 p-3">
              <h4 className="font-semibold text-ink mb-2">AI Agent Integration</h4>
              <p className="text-ink-dim mb-2">
                Connect AI agents via stdio by running:
              </p>
              <pre className="selectable rounded border border-line bg-shell p-2 font-mono text-[11px] text-ink overflow-x-auto">
                mnemosyne serve
              </pre>
            </div>
          </div>
        )}

        {/* Output Log Tab */}
        {tab === "output" && (
          <div className="flex flex-col gap-1 font-mono text-[11px]">
            {log.length === 0 && (
              <p className="py-4 text-center text-xs text-ink-faint">No daemon events logged yet.</p>
            )}
            {[...log].reverse().map((entry, i) => {
              const isError = entry.kind === "error";
              const isWrite = entry.kind === "memory.written";
              const isDelete = entry.kind === "memory.deleted";
              return (
                <div
                  key={i}
                  className="flex items-center gap-2 rounded px-2 py-0.5 hover:bg-hover/60"
                >
                  <span className="text-ink-faint text-[10px]">
                    {new Date(entry.at).toLocaleTimeString()}
                  </span>
                  <span
                    className={`rounded px-1 text-[10px] font-semibold ${
                      isError
                        ? "bg-danger/20 text-danger"
                        : isWrite
                          ? "bg-tag/20 text-tag"
                          : isDelete
                            ? "bg-warn/20 text-warn"
                            : "bg-raised text-ink"
                    }`}
                  >
                    {entry.kind}
                  </span>
                  <span className="text-ink truncate">
                    {"message" in entry
                      ? entry.message
                      : [entry.project, entry.memory].filter(Boolean).join("/")}
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </section>
  );
}
