import { useEffect, useRef, useState } from "react";

import type { Project } from "../lib/types";
import { FilterIcon, FolderIcon, SearchIcon, SparklesIcon } from "./Icons";

export type SearchMode = "text" | "semantic" | "hybrid";

type Props = {
  projects: Project[];
  query: string;
  project: string;
  mode: SearchMode;
  semanticAvailable: boolean;
  busy: boolean;
  onChange: (next: { query: string; project: string; mode: SearchMode }) => void;
  onSubmit: () => void;
};

export default function SearchView({
  projects,
  query,
  project,
  mode,
  semanticAvailable,
  busy,
  onChange,
  onSubmit,
}: Props) {
  const input = useRef<HTMLInputElement>(null);
  const [touched, setTouched] = useState(false);

  useEffect(() => {
    input.current?.focus();
    input.current?.select();
  }, []);

  return (
    <form
      className="flex h-full flex-col gap-3 p-3 text-ink-dim select-none"
      onSubmit={(e) => {
        e.preventDefault();
        setTouched(true);
        onSubmit();
      }}
    >
      <div className="flex items-center justify-between border-b border-line pb-2">
        <h2 className="text-[11px] font-semibold uppercase tracking-wider text-ink">Search</h2>
      </div>

      {/* Search Input Box */}
      <div className="relative flex items-center">
        <input
          ref={input}
          value={query}
          onChange={(e) => onChange({ query: e.target.value, project, mode })}
          placeholder="Search memories (Ctrl+Shift+F)..."
          aria-label="Search memories"
          className="w-full rounded-md border border-line bg-editor px-3 py-1.5 text-xs text-ink placeholder:text-ink-faint outline-none focus:border-accent"
        />
        {query && (
          <button
            type="button"
            onClick={() => onChange({ query: "", project, mode })}
            className="absolute right-2 text-xs text-ink-faint hover:text-ink"
          >
            ✕
          </button>
        )}
      </div>

      {/* Project Scope Filter */}
      <div className="flex flex-col gap-1">
        <label className="flex items-center gap-1.5 text-xs font-medium text-ink">
          <FolderIcon className="h-3.5 w-3.5 text-accent" />
          <span>Project Scope</span>
        </label>
        <select
          value={project}
          onChange={(e) => onChange({ query, project: e.target.value, mode })}
          className="w-full rounded-md border border-line bg-editor px-2.5 py-1.5 text-xs text-ink outline-none focus:border-accent"
        >
          <option value="">All Projects</option>
          {projects.map((p) => (
            <option key={p.slug} value={p.slug}>
              {p.name || p.slug} ({p.memoryCount})
            </option>
          ))}
        </select>
      </div>

      {/* Search Mode Segmented Pills */}
      <div className="flex flex-col gap-1.5">
        <label className="flex items-center justify-between text-xs font-medium text-ink">
          <div className="flex items-center gap-1.5">
            <FilterIcon className="h-3.5 w-3.5 text-tag" />
            <span>Ranking Mode</span>
          </div>
          {semanticAvailable && (
            <span className="flex items-center gap-1 text-[10px] text-tag font-mono">
              <SparklesIcon className="h-2.5 w-2.5" />
              AI Embeddings Ready
            </span>
          )}
        </label>

        <div className="grid grid-cols-3 gap-1 rounded-md border border-line bg-raised p-1 text-[11px]">
          {(
            [
              { id: "text", label: "Keyword", desc: "BM25 text match" },
              { id: "hybrid", label: "Hybrid", desc: "Text + Semantic RRF" },
              { id: "semantic", label: "Semantic", desc: "Vector similarity" },
            ] as const
          ).map(({ id, label, desc }) => {
            const disabled = id !== "text" && !semanticAvailable;
            const active = mode === id;
            return (
              <button
                key={id}
                type="button"
                disabled={disabled}
                onClick={() => onChange({ query, project, mode: id })}
                title={
                  disabled
                    ? "Embedding provider not reachable. Check settings."
                    : `${label} search: ${desc}`
                }
                className={`flex flex-col items-center rounded py-1 px-1 text-center transition-colors ${
                  active
                    ? "bg-selected text-ink font-semibold shadow-xs"
                    : disabled
                      ? "opacity-30 cursor-not-allowed text-ink-faint"
                      : "text-ink-dim hover:bg-hover hover:text-ink"
                }`}
              >
                <span>{label}</span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Submit Button */}
      <button
        type="submit"
        disabled={busy || query.trim() === ""}
        className="mt-1 flex items-center justify-center gap-1.5 rounded-md bg-accent px-3 py-1.5 text-xs font-semibold text-accent-ink hover:opacity-95 transition-opacity disabled:opacity-40"
      >
        <SearchIcon className="h-3.5 w-3.5" />
        <span>{busy ? "Searching…" : "Search"}</span>
      </button>

      {/* Search Result Hint */}
      <div className="mt-auto rounded-md border border-line/60 bg-raised/30 p-2 text-[11px] text-ink-faint">
        <p className="font-medium text-ink-dim mb-1">Tip</p>
        <p>
          {touched
            ? "Results appear in the bottom Search panel."
            : "Press Enter or click Search to view ranked results in the panel."}
        </p>
      </div>
    </form>
  );
}
