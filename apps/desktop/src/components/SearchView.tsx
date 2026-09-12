import { useEffect, useRef, useState } from "react";

import type { Project } from "../lib/types";

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

  // The view is only mounted when Search is selected, so focusing on mount is
  // what makes Ctrl+Shift+F land in the box.
  useEffect(() => {
    input.current?.focus();
    input.current?.select();
  }, []);

  return (
    <form
      className="flex h-full flex-col gap-2 p-2"
      onSubmit={(e) => {
        e.preventDefault();
        setTouched(true);
        onSubmit();
      }}
    >
      <h2 className="px-1 text-[11px] font-semibold uppercase tracking-wide text-ink-dim">Search</h2>

      <input
        ref={input}
        value={query}
        onChange={(e) => onChange({ query: e.target.value, project, mode })}
        placeholder="Search memories"
        aria-label="Search memories"
        className="w-full rounded border border-line bg-editor px-2 py-1 text-ink placeholder:text-ink-faint focus:border-accent focus:outline-none"
      />

      <label className="flex flex-col gap-1 px-1 text-ink-dim">
        Project
        <select
          value={project}
          onChange={(e) => onChange({ query, project: e.target.value, mode })}
          className="rounded border border-line bg-editor px-1 py-1 text-ink focus:border-accent focus:outline-none"
        >
          <option value="">All projects</option>
          {projects.map((p) => (
            <option key={p.slug} value={p.slug}>
              {p.name}
            </option>
          ))}
        </select>
      </label>

      <fieldset className="flex flex-col gap-1 px-1">
        <legend className="text-ink-dim">Ranking</legend>
        {(["text", "hybrid", "semantic"] as SearchMode[]).map((value) => {
          // Semantic and hybrid need an embedding provider. Disabling them with
          // the reason beats letting the request fail and reporting it late.
          const disabled = value !== "text" && !semanticAvailable;
          return (
            <label
              key={value}
              title={disabled ? "No embedding provider is reachable — see Settings" : undefined}
              className={`flex items-center gap-2 ${disabled ? "text-ink-faint" : "text-ink"}`}
            >
              <input
                type="radio"
                name="mode"
                value={value}
                checked={mode === value}
                disabled={disabled}
                onChange={() => onChange({ query, project, mode: value })}
              />
              {value === "text" ? "Keyword (FTS5)" : value === "hybrid" ? "Hybrid" : "Semantic"}
            </label>
          );
        })}
      </fieldset>

      <button
        type="submit"
        disabled={busy || query.trim() === ""}
        className="rounded bg-accent px-2 py-1 font-medium text-accent-ink disabled:opacity-40"
      >
        {busy ? "Searching…" : "Search"}
      </button>

      <p className="px-1 text-ink-faint">
        {touched ? "Results are in the panel below." : "Results appear in the panel below."}
      </p>
    </form>
  );
}
