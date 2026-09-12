import { useState } from "react";

import type { Health } from "../lib/types";

type Props = { health: Health | null; endpoint: string | null };

// One entry per client the project supports wiring today. The command is what
// the user copies, so it is written exactly as it must be typed.
const clients = [
  {
    name: "Claude Code",
    command: "claude mcp add mnemosyne -- mnemosyne serve",
    note: "Adds it to the current project. Use --scope user for every project.",
  },
  {
    name: "Codex",
    command: "codex mcp add mnemosyne -- mnemosyne serve",
    note: "Same server, same memories.",
  },
  {
    name: "Any MCP client (JSON)",
    command: '{ "mcpServers": { "mnemosyne": { "command": "mnemosyne", "args": ["serve"] } } }',
    note: "The stdio form every MCP client understands.",
  },
];

export default function McpView({ health, endpoint }: Props) {
  const [copied, setCopied] = useState<string | null>(null);

  const copy = async (value: string) => {
    await navigator.clipboard.writeText(value);
    setCopied(value);
    setTimeout(() => setCopied((c) => (c === value ? null : c)), 1500);
  };

  return (
    <div className="flex h-full flex-col gap-3 overflow-y-auto p-2">
      <h2 className="px-1 text-[11px] font-semibold uppercase tracking-wide text-ink-dim">
        Connect an agent
      </h2>

      <p className="px-1 text-ink-dim">
        Each agent runs its own <code className="font-mono">mnemosyne serve</code> over stdio. It
        reads and writes the same memory root this window shows, so nothing needs to be kept in
        sync.
      </p>

      {clients.map(({ name, command, note }) => (
        <section key={name} className="px-1">
          <h3 className="mb-1 font-medium text-ink">{name}</h3>
          <pre className="selectable overflow-x-auto rounded border border-line bg-editor p-2 font-mono text-[11px] text-ink">
            {command}
          </pre>
          <div className="mt-1 flex items-center gap-2">
            <button
              type="button"
              onClick={() => copy(command)}
              className="rounded border border-line px-2 py-0.5 text-ink-dim hover:bg-hover hover:text-ink"
            >
              {copied === command ? "Copied" : "Copy"}
            </button>
            <span className="text-ink-faint">{note}</span>
          </div>
        </section>
      ))}

      <section className="mt-2 border-t border-line px-1 pt-2 text-ink-dim">
        <h3 className="mb-1 font-medium text-ink">This window</h3>
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1">
          <dt>daemon</dt>
          <dd className="selectable truncate font-mono text-[11px]">{endpoint ?? "not connected"}</dd>
          <dt>version</dt>
          <dd className="font-mono text-[11px]">{health?.version ?? "—"}</dd>
          <dt>memories</dt>
          <dd className="font-mono text-[11px]">{health?.memories ?? "—"}</dd>
        </dl>
        <p className="mt-2 text-ink-faint">
          The daemon serves this window only. Agents do not go through it.
        </p>
      </section>
    </div>
  );
}
