import { useEffect, useMemo, useRef, useState } from "react";

import type { Meta } from "../lib/types";

type Props = {
  memories: Meta[];
  onPick: (meta: Meta) => void;
  onClose: () => void;
};

/** How many matches to show. Beyond this the list stops being a shortcut. */
const limit = 40;

export default function QuickOpen({ memories, onPick, onClose }: Props) {
  const [query, setQuery] = useState("");
  const [cursor, setCursor] = useState(0);
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => input.current?.focus(), []);

  const matches = useMemo(() => {
    const needle = query.trim().toLowerCase();
    const pool = needle
      ? memories.filter((meta) =>
          `${meta.project}/${meta.slug} ${meta.title} ${(meta.tags ?? []).join(" ")}`
            .toLowerCase()
            .includes(needle),
        )
      : // With no query, the most recently touched memories are the useful
        // default — the same thing Ctrl+P does in an editor.
        [...memories].sort((a, b) => b.updated.localeCompare(a.updated));
    return pool.slice(0, limit);
  }, [memories, query]);

  // A filtered list can be shorter than the cursor's last position.
  const index = Math.min(cursor, Math.max(matches.length - 1, 0));

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setCursor((c) => Math.min(c + 1, matches.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setCursor((c) => Math.max(c - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const picked = matches[index];
      if (picked) onPick(picked);
    } else if (e.key === "Escape") {
      e.preventDefault();
      onClose();
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex justify-center bg-black/30 pt-16"
      onMouseDown={onClose}
      role="presentation"
    >
      <div
        className="h-fit w-[36rem] max-w-[90vw] overflow-hidden rounded border border-line bg-raised shadow-2xl"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <input
          ref={input}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setCursor(0);
          }}
          onKeyDown={onKeyDown}
          placeholder="Go to memory"
          aria-label="Go to memory"
          className="w-full border-b border-line bg-editor px-3 py-2 text-ink placeholder:text-ink-faint focus:outline-none"
        />

        <ul className="max-h-80 overflow-y-auto">
          {matches.length === 0 && (
            <li className="px-3 py-2 text-ink-faint">No memory matches “{query}”.</li>
          )}
          {matches.map((meta, i) => (
            <li key={`${meta.project}/${meta.slug}`}>
              <button
                type="button"
                onMouseEnter={() => setCursor(i)}
                onClick={() => onPick(meta)}
                className={`flex w-full items-baseline gap-2 px-3 py-1.5 text-left ${
                  i === index ? "bg-selected text-ink" : "hover:bg-hover"
                }`}
              >
                <span className="truncate">{meta.title || meta.slug}</span>
                <span className="ml-auto shrink-0 font-mono text-[11px] text-ink-faint">
                  {meta.project}/{meta.slug}
                </span>
              </button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
