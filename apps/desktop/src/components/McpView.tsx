import { useState } from "react";
import type { Health } from "../lib/types";
import { AlertCircleIcon, CheckIcon, CopyIcon, TerminalIcon } from "./Icons";

type Props = { health: Health | null; endpoint: string | null };

type ClientItem = {
  id: string;
  name: string;
  badge?: string;
  configPath?: string;
  command: string;
  note: string;
  commandAlt?: string;
  noteAlt?: string;
};

const clients: ClientItem[] = [
  {
    id: "antigravity",
    name: "Google Antigravity",
    badge: "Recommended",
    configPath: "~/.gemini/config/mcp_config.json",
    command: JSON.stringify(
      {
        mcpServers: {
          mnemosyne: {
            command: "mnemosyne",
            args: ["serve"],
          },
        },
      },
      null,
      2
    ),
    note: "Add to ~/.gemini/config/mcp_config.json or workspace .gemini/mcp_config.json",
    commandAlt: JSON.stringify(
      {
        mcpServers: {
          mnemosyne: {
            command: "C:\\Users\\ROOT\\AppData\\Local\\Programs\\Mnemosyne\\mnemosyne.exe",
            args: ["serve"],
          },
        },
      },
      null,
      2
    ),
    noteAlt: "Direct binary path fallback (resolves 'program not found' instantly without restart)",
  },
  {
    id: "claude",
    name: "Claude Code",
    badge: "CLI",
    command: "claude mcp add mnemosyne -- mnemosyne serve",
    note: "Adds to current project. Use --scope user to configure globally across all projects.",
  },
  {
    id: "cursor",
    name: "Cursor",
    configPath: "~/.cursor/mcp.json",
    command: JSON.stringify(
      {
        mcpServers: {
          mnemosyne: {
            command: "mnemosyne",
            args: ["serve"],
          },
        },
      },
      null,
      2
    ),
    note: "Add to Cursor MCP configuration (~/.cursor/mcp.json or Features > MCP in settings).",
  },
  {
    id: "codex",
    name: "Codex",
    command: "codex mcp add mnemosyne -- mnemosyne serve",
    note: "Registers mnemosyne as an MCP tool for Codex sessions.",
  },
  {
    id: "json",
    name: "Any MCP Client (Generic JSON)",
    command: JSON.stringify(
      {
        mcpServers: {
          mnemosyne: {
            command: "mnemosyne",
            args: ["serve"],
          },
        },
      },
      null,
      2
    ),
    note: "Universal stdio specification compatible with any standard Model Context Protocol client.",
  },
];

export default function McpView({ health, endpoint }: Props) {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<string>("all");
  const [useAbsolutePath, setUseAbsolutePath] = useState<boolean>(false);

  const copy = async (key: string, value: string) => {
    await navigator.clipboard.writeText(value);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey((c) => (c === key ? null : c)), 1500);
  };

  const filteredClients =
    selectedCategory === "all"
      ? clients
      : clients.filter((c) => c.id === selectedCategory);

  return (
    <div className="flex h-full flex-col gap-4 overflow-y-auto p-4 max-w-4xl mx-auto w-full">
      {/* Top Banner / Header */}
      <div>
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-base font-semibold tracking-tight text-ink">
              Connect AI Agents & MCP Clients
            </h2>
            <p className="mt-0.5 text-xs text-ink-dim leading-relaxed">
              Each AI assistant runs <code className="font-mono text-accent bg-shell px-1 py-0.2 rounded border border-line">mnemosyne serve</code> over stdio to read and query project memories in real-time.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <span className="flex items-center gap-1.5 rounded-full border border-tag/30 bg-tag/10 px-2.5 py-1 text-[11px] font-medium text-tag">
              <span className="h-2 w-2 rounded-full bg-tag animate-pulse" />
              Daemon Ready
            </span>
          </div>
        </div>

        {/* Category Filters */}
        <div className="mt-3 flex items-center gap-1.5 border-b border-line pb-2">
          {["all", "antigravity", "claude", "cursor", "codex"].map((cat) => (
            <button
              key={cat}
              type="button"
              onClick={() => setSelectedCategory(cat)}
              className={`rounded-md px-2.5 py-1 text-xs font-medium capitalize transition-colors ${
                selectedCategory === cat
                  ? "bg-accent/15 text-accent border border-accent/30"
                  : "text-ink-faint hover:bg-hover hover:text-ink border border-transparent"
              }`}
            >
              {cat === "all"
                ? "All Clients"
                : cat === "antigravity"
                  ? "Google Antigravity"
                  : cat === "claude"
                    ? "Claude Code"
                    : cat}
            </button>
          ))}
        </div>
      </div>

      {/* Troubleshooting Banner for 'program not found' */}
      <div className="rounded-lg border border-warn/40 bg-warn/10 p-3.5 text-xs">
        <div className="flex items-start gap-2.5">
          <div className="rounded-md bg-warn/20 p-1 text-warn shrink-0 mt-0.5">
            <AlertCircleIcon className="h-4 w-4" />
          </div>
          <div className="flex-1">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold text-warn text-[12px]">
                Troubleshooting: &ldquo;program not found&rdquo; or MCP Startup Failure
              </h3>
              <span className="text-[10px] font-mono text-warn/80 bg-warn/20 px-1.5 py-0.5 rounded">
                Path & Environment
              </span>
            </div>
            <p className="mt-1 text-ink-dim leading-relaxed">
              If Antigravity or your MCP client shows{" "}
              <code className="rounded bg-shell/80 px-1 py-0.5 font-mono text-[11px] text-danger border border-line">
                ⚠ MCP client failed to start: program not found
              </code>
              , the client process cannot locate <code className="font-mono text-ink">mnemosyne</code> on its current <code className="font-mono text-ink">%PATH%</code>.
            </p>

            <div className="mt-2.5 grid grid-cols-1 md:grid-cols-2 gap-2">
              {/* Solution 1 */}
              <div className="rounded-md border border-line bg-shell/60 p-2.5 flex flex-col justify-between">
                <div>
                  <div className="flex items-center gap-1 font-semibold text-ink text-[11px]">
                    <TerminalIcon className="h-3.5 w-3.5 text-accent" />
                    <span>Option 1: Register on System PATH</span>
                  </div>
                  <p className="mt-1 text-[11px] text-ink-faint">
                    Run the installer once in your terminal, then restart your IDE / Antigravity:
                  </p>
                </div>
                <div className="mt-2 flex items-center justify-between rounded bg-editor px-2 py-1 border border-line">
                  <code className="font-mono text-[11px] text-ink">mnemosyne install</code>
                  <button
                    type="button"
                    onClick={() => copy("install-cmd", "mnemosyne install")}
                    className="flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium text-ink-dim hover:bg-hover hover:text-ink transition-colors"
                  >
                    {copiedKey === "install-cmd" ? (
                      <>
                        <CheckIcon className="h-3 w-3 text-tag" />
                        <span className="text-tag">Copied</span>
                      </>
                    ) : (
                      <>
                        <CopyIcon className="h-3 w-3" />
                        <span>Copy</span>
                      </>
                    )}
                  </button>
                </div>
              </div>

              {/* Solution 2 */}
              <div className="rounded-md border border-line bg-shell/60 p-2.5 flex flex-col justify-between">
                <div>
                  <div className="flex items-center gap-1 font-semibold text-ink text-[11px]">
                    <span className="h-1.5 w-1.5 rounded-full bg-tag" />
                    <span>Option 2: Direct Binary Path (No Restart)</span>
                  </div>
                  <p className="mt-1 text-[11px] text-ink-faint">
                    Use the direct absolute executable path in your MCP config:
                  </p>
                </div>
                <div className="mt-2 flex items-center justify-between rounded bg-editor px-2 py-1 border border-line">
                  <code className="font-mono text-[10.5px] text-ink truncate mr-2" title="C:\Users\ROOT\AppData\Local\Programs\Mnemosyne\mnemosyne.exe">
                    C:\Users\...\Mnemosyne\mnemosyne.exe
                  </code>
                  <button
                    type="button"
                    onClick={() =>
                      copy(
                        "bin-path",
                        "C:\\Users\\ROOT\\AppData\\Local\\Programs\\Mnemosyne\\mnemosyne.exe"
                      )
                    }
                    className="flex shrink-0 items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium text-ink-dim hover:bg-hover hover:text-ink transition-colors"
                  >
                    {copiedKey === "bin-path" ? (
                      <>
                        <CheckIcon className="h-3 w-3 text-tag" />
                        <span className="text-tag">Copied</span>
                      </>
                    ) : (
                      <>
                        <CopyIcon className="h-3 w-3" />
                        <span>Copy Path</span>
                      </>
                    )}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Client Configurations */}
      <div className="flex flex-col gap-3">
        {filteredClients.map((client) => {
          const isAntigravity = client.id === "antigravity";
          const activeCommand =
            isAntigravity && useAbsolutePath && client.commandAlt
              ? client.commandAlt
              : client.command;
          const activeNote =
            isAntigravity && useAbsolutePath && client.noteAlt
              ? client.noteAlt
              : client.note;

          return (
            <section
              key={client.id}
              className="rounded-lg border border-line bg-raised/30 p-3.5 transition-colors hover:border-line/80"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <h3 className="font-semibold text-ink text-sm">{client.name}</h3>
                  {client.badge && (
                    <span className="rounded bg-accent/20 px-1.5 py-0.2 font-mono text-[10px] font-medium text-accent">
                      {client.badge}
                    </span>
                  )}
                  {client.configPath && (
                    <span className="rounded bg-shell px-1.5 py-0.2 font-mono text-[10px] text-ink-faint border border-line">
                      {client.configPath}
                    </span>
                  )}
                </div>

                {isAntigravity && (
                  <div className="flex items-center gap-2 text-xs">
                    <button
                      type="button"
                      onClick={() => setUseAbsolutePath((v) => !v)}
                      className={`rounded px-2 py-0.5 text-[10.5px] border font-medium transition-colors ${
                        useAbsolutePath
                          ? "bg-accent/20 border-accent/40 text-accent"
                          : "bg-shell border-line text-ink-dim hover:text-ink"
                      }`}
                    >
                      {useAbsolutePath ? "Using Absolute Path" : "Use Absolute Path"}
                    </button>
                  </div>
                )}
              </div>

              <div className="relative mt-2.5">
                <pre className="selectable overflow-x-auto rounded-md border border-line bg-editor p-3 font-mono text-[11.5px] text-ink leading-relaxed">
                  {activeCommand}
                </pre>
                <button
                  type="button"
                  onClick={() => copy(client.id, activeCommand)}
                  className="absolute top-2.5 right-2.5 flex items-center gap-1.5 rounded border border-line bg-raised px-2 py-1 text-xs font-medium text-ink-dim hover:bg-hover hover:text-ink transition-colors shadow-sm"
                >
                  {copiedKey === client.id ? (
                    <>
                      <CheckIcon className="h-3.5 w-3.5 text-tag" />
                      <span className="text-tag">Copied</span>
                    </>
                  ) : (
                    <>
                      <CopyIcon className="h-3.5 w-3.5" />
                      <span>Copy</span>
                    </>
                  )}
                </button>
              </div>

              <p className="mt-2 text-xs text-ink-faint leading-relaxed">
                {activeNote}
              </p>
            </section>
          );
        })}
      </div>

      {/* Host / Daemon Info */}
      <section className="rounded-lg border border-line bg-shell p-3.5 text-ink-dim">
        <h3 className="mb-2 font-semibold text-ink text-xs uppercase tracking-wider">
          Local Daemon Details
        </h3>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-xs">
          <dt className="text-ink-faint">IPC / Stdio Endpoint:</dt>
          <dd className="selectable font-mono text-[11px] text-ink break-all">
            {endpoint ?? "not connected"}
          </dd>
          <dt className="text-ink-faint">Backend Version:</dt>
          <dd className="font-mono text-[11px] text-ink">{health?.version ?? "—"}</dd>
          <dt className="text-ink-faint">Indexed Memories:</dt>
          <dd className="font-mono text-[11px] text-tag font-semibold">
            {health?.memories ?? "—"}
          </dd>
        </dl>
        <p className="mt-2.5 text-[11px] text-ink-faint">
          The daemon runs locally. Each AI agent starts its own lightweight stdio server accessing the same shared database without conflict.
        </p>
      </section>
    </div>
  );
}
