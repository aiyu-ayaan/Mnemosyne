import { useEffect, useMemo, useRef, useState } from "react";

import type { Meta } from "../lib/types";

/** One entry in the palette's command mode. */
export type Command = {
  id: string;
  label: string;
  /** Grouping shown before the label, as VS Code does: "View: Toggle Panel". */
  group: string;
  hint?: string;
  run: () => void;
  /** Commands that need an open memory grey out rather than disappear. */
  disabled?: boolean;
};

type Props = {
  memories: Meta[];
  commands: Command[];
  /** True when opened with Ctrl+Shift+P rather than Ctrl+P. */
  commandMode: boolean;
  onPick: (meta: Meta) => void;
  onClose: () => void;
};

/** How many matches to show. Beyond this the list stops being a shortcut. */
const limit = 40;

/**
 * Quick-open and the command palette are one overlay, because in an editor they
 * are one overlay: `>` switches between them, and the same arrow keys, the same
 * filter, and the same Enter apply to both. Two components would be the same
 * list twice.
 */
export default function QuickOpen({ memories, commands, commandMode, onPick, onClose }: Props) {
  const [query, setQuery] = useState(commandMode ? ">" : "");
  const [cursor, setCursor] = useState(0);
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => {
    input.current?.focus();
    // Put the caret after the ">" rather than selecting it, so typing filters
    // instead of replacing the prefix that chose this mode.
    input.current?.setSelectionRange(query.length, query.length);
    // Only on mount: this is the initial caret placement, not a caret policy.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const isCommands = query.startsWith(">");
  const needle = (isCommands ? query.slice(1) : query).trim().toLowerCase();

  const commandMatches = useMemo(() => {
    if (!isCommands) return [];
    const pool = needle
      ? commands.filter((c) => `${c.group} ${c.label}`.toLowerCase().includes(needle))
      : commands;
    return pool.slice(0, limit);
  }, [commands, isCommands, needle]);

  const memoryMatches = useMemo(() => {
    if (isCommands) return [];
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
  }, [memories, isCommands, needle]);

  const count = isCommands ? commandMatches.length : memoryMatches.length;
  // A filtered list can be shorter than the cursor's last position.
  const index = Math.min(cursor, Math.max(count - 1, 0));

  const choose = (i: number) => {
    if (isCommands) {
      const command = commandMatches[i];
      if (!command || command.disabled) return;
      onClose();
      command.run();
      return;
    }
    const picked = memoryMatches[i];
    if (picked) onPick(picked);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setCursor((c) => Math.min(c + 1, count - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setCursor((c) => Math.max(c - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      choose(index);
    } else if (e.key === "Escape") {
      e.preventDefault();
      onClose();
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex justify-center bg-black/40 pt-16"
      onMouseDown={onClose}
      role="presentation"
    >
      <div
        className="h-fit w-[36rem] max-w-[90vw] overflow-hidden rounded-lg border border-line bg-raised shadow-2xl"
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
          placeholder="Go to memory, or type > for commands"
          aria-label={isCommands ? "Run command" : "Go to memory"}
          className="w-full border-b border-line bg-editor px-3 py-2.5 text-ink placeholder:text-ink-faint focus:outline-none"
        />

        <ul className="max-h-80 overflow-y-auto">
          {count === 0 && (
            <li className="px-3 py-2 text-ink-faint">
              {isCommands ? "No command matches" : "No memory matches"}
              {needle && ` “${needle}”`}.
            </li>
          )}

          {isCommands
            ? commandMatches.map((command, i) => (
                <li key={command.id}>
                  <button
                    type="button"
                    disabled={command.disabled}
                    onMouseEnter={() => setCursor(i)}
                    onClick={() => choose(i)}
                    className={`flex w-full items-baseline gap-2 px-3 py-1.5 text-left disabled:opacity-40 ${
                      i === index ? "bg-selected text-ink" : "hover:bg-hover"
                    }`}
                  >
                    <span className="shrink-0 text-ink-faint">{command.group}:</span>
                    <span className="truncate">{command.label}</span>
                    {command.hint && (
                      <kbd className="ml-auto shrink-0 rounded border border-line px-1.5 py-0.5 font-mono text-[10px] text-ink-faint">
                        {command.hint}
                      </kbd>
                    )}
                  </button>
                </li>
              ))
            : memoryMatches.map((meta, i) => (
                <li key={`${meta.project}/${meta.slug}`}>
                  <button
                    type="button"
                    onMouseEnter={() => setCursor(i)}
                    onClick={() => choose(i)}
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
