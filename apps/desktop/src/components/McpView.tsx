import { useState } from "react";
import { bridge } from "../lib/bridge";
import type { Health } from "../lib/types";
import { AlertCircleIcon, CheckIcon, CopyIcon, TerminalIcon } from "./Icons";

type Props = {
  health: Health | null;
  endpoint: string | null;
  binaryPath?: string | null;
};

type ClientDef = {
  id: string;
  name: string;
  badge?: string;
  configPath?: string;
  getCommand: (exe: string) => string;
  note: string;
};

const CLIENT_DEFS: ClientDef[] = [
  {
    id: "antigravity",
    name: "Google Antigravity",
    badge: "Recommended",
    configPath: "~/.gemini/config/mcp_config.json",
    getCommand: (exe) =>
      JSON.stringify(
        {
          mcpServers: {
            mnemosyne: {
              command: exe,
              args: ["serve"],
            },
          },
        },
        null,
        2,
      ),
    note: "Add to ~/.gemini/config/mcp_config.json or workspace .gemini/mcp_config.json",
  },
  {
    id: "codex",
    name: "OpenAI Codex",
    badge: "CLI",
    getCommand: (exe) =>
      exe.includes(" ") || exe.includes("\\") || exe.includes("/")
        ? `codex mcp add mnemosyne -- "${exe}" serve`
        : `codex mcp add mnemosyne -- ${exe} serve`,
    note: "Registers mnemosyne as an MCP tool for Codex sessions.",
  },
  {
    id: "claude",
    name: "Claude Code",
    badge: "CLI",
    getCommand: (exe) =>
      exe.includes(" ") || exe.includes("\\") || exe.includes("/")
        ? `claude mcp add mnemosyne -- "${exe}" serve`
        : `claude mcp add mnemosyne -- ${exe} serve`,
    note: "Adds to current project. Use --scope user to configure globally across all projects.",
  },
  {
    id: "cursor",
    name: "Cursor",
    configPath: "~/.cursor/mcp.json",
    getCommand: (exe) =>
      JSON.stringify(
        {
          mcpServers: {
            mnemosyne: {
              command: exe,
              args: ["serve"],
            },
          },
        },
        null,
        2,
      ),
    note: "Add to Cursor MCP configuration (~/.cursor/mcp.json or Features > MCP in settings).",
  },
  {
    id: "json",
    name: "Any MCP Client (Generic JSON)",
    getCommand: (exe) =>
      JSON.stringify(
        {
          mcpServers: {
            mnemosyne: {
              command: exe,
              args: ["serve"],
            },
          },
        },
        null,
        2,
      ),
    note: "Universal stdio specification compatible with any standard Model Context Protocol client.",
  },
];

export default function McpView({ health, endpoint, binaryPath }: Props) {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<string>("all");
  const [useDevExe, setUseDevExe] = useState<boolean>(true);
  const [installing, setInstalling] = useState<boolean>(false);
  const [installStatus, setInstallStatus] = useState<{ ok: boolean; msg: string } | null>(null);

  const devExe = binaryPath || "D:\\VS-Code\\AI Expermients\\Mnemosyne\\bin\\mnemosyne.exe";
  const installedExe = "C:\\Users\\ROOT\\AppData\\Local\\Programs\\Mnemosyne\\mnemosyne.exe";
  const activeExe = useDevExe ? devExe : "mnemosyne";

  const copy = async (key: string, value: string) => {
    await navigator.clipboard.writeText(value);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey((c) => (c === key ? null : c)), 1500);
  };

  const handleInstallNow = async () => {
    setInstalling(true);
    setInstallStatus(null);
    try {
      const msg = await bridge.installBinary();
      setInstallStatus({ ok: true, msg: msg || "Installed successfully to Programs and added to user PATH." });
    } catch (err) {
      setInstallStatus({
        ok: false,
        msg: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setInstalling(false);
    }
  };

  const filteredClients =
    selectedCategory === "all"
      ? CLIENT_DEFS
      : CLIENT_DEFS.filter((c) => c.id === selectedCategory);

  return (
    <div className="flex h-full flex-col gap-5 overflow-y-auto p-6 max-w-5xl mx-auto w-full select-none">
      {/* Top Banner / Header */}
      <div className="border-b border-line pb-4">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <div>
            <h2 className="text-xl font-bold tracking-tight text-ink">
              Connect AI Agents &amp; MCP Clients
            </h2>
            <p className="mt-1 text-xs text-ink-dim leading-relaxed">
              Mnemosyne serves memories over standard I/O via <code className="font-mono text-accent bg-shell px-1.5 py-0.5 rounded border border-line">mnemosyne serve</code>.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <span className="flex items-center gap-1.5 rounded-full border border-tag/30 bg-tag/10 px-3 py-1 text-xs font-medium text-tag">
              <span className="h-2 w-2 rounded-full bg-tag animate-pulse" />
              Daemon Ready
            </span>
          </div>
        </div>

        {/* Binary Mode Switcher Banner */}
        <div className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-line bg-shell/80 p-3">
          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold text-ink">Active MCP Binary Mode:</span>
            <span className="font-mono text-[11px] text-ink-dim bg-editor px-2 py-0.5 rounded border border-line">
              {useDevExe ? "Direct Workspace Exe (Recommended for Dev)" : "System PATH ('mnemosyne')"}
            </span>
          </div>
          <div className="flex items-center gap-1.5 bg-editor p-0.5 rounded-md border border-line">
            <button
              type="button"
              onClick={() => setUseDevExe(true)}
              className={`rounded px-2.5 py-1 text-xs font-medium transition-colors ${
                useDevExe
                  ? "bg-accent text-accent-ink font-semibold shadow-xs"
                  : "text-ink-dim hover:text-ink"
              }`}
            >
              Direct Exe (Dev)
            </button>
            <button
              type="button"
              onClick={() => setUseDevExe(false)}
              className={`rounded px-2.5 py-1 text-xs font-medium transition-colors ${
                !useDevExe
                  ? "bg-accent text-accent-ink font-semibold shadow-xs"
                  : "text-ink-dim hover:text-ink"
              }`}
            >
              System PATH
            </button>
          </div>
        </div>

        {/* Category Filters */}
        <div className="mt-3 flex items-center gap-1.5">
          {["all", "antigravity", "codex", "claude", "cursor"].map((cat) => (
            <button
              key={cat}
              type="button"
              onClick={() => setSelectedCategory(cat)}
              className={`rounded-md px-3 py-1 text-xs font-medium capitalize transition-colors ${
                selectedCategory === cat
                  ? "bg-accent/15 text-accent border border-accent/30 font-semibold"
                  : "text-ink-faint hover:bg-hover hover:text-ink border border-transparent"
              }`}
            >
              {cat === "all"
                ? "All Assistants"
                : cat === "antigravity"
                  ? "Google Antigravity"
                  : cat === "claude"
                    ? "Claude Code"
                    : cat === "codex"
                      ? "OpenAI Codex"
                      : cat}
            </button>
          ))}
        </div>
      </div>

      {/* Troubleshooting & Development Setup Banner */}
      <div className="rounded-xl border border-warn/40 bg-warn/10 p-4 text-xs">
        <div className="flex items-start gap-3">
          <div className="rounded-md bg-warn/20 p-1.5 text-warn shrink-0 mt-0.5">
            <AlertCircleIcon className="h-4 w-4" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold text-warn text-sm">
                How to Resolve &ldquo;program not found&rdquo; or MCP Startup Failure
              </h3>
              <span className="text-[10px] font-mono text-warn/90 bg-warn/20 px-2 py-0.5 rounded font-semibold">
                Windows Dev Guide
              </span>
            </div>
            <p className="mt-1 text-xs text-ink-dim leading-relaxed">
              If Codex or Antigravity fails with <code className="rounded bg-shell px-1 py-0.5 font-mono text-[11px] text-danger border border-line">program not found</code>, it cannot find <code className="font-mono text-ink">mnemosyne</code> on the current shell&apos;s <code className="font-mono text-ink">%PATH%</code>. Use either solution below:
            </p>

            <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3">
              {/* Option 1 */}
              <div className="rounded-lg border border-line bg-shell/80 p-3.5 flex flex-col justify-between">
                <div>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-1.5 font-semibold text-ink text-xs">
                      <TerminalIcon className="h-3.5 w-3.5 text-accent" />
                      <span>Option 1: One-Click System PATH Registration</span>
                    </div>
                  </div>
                  <p className="mt-1 text-[11px] text-ink-faint leading-relaxed">
                    Install <code className="font-mono text-ink">mnemosyne.exe</code> to your user profile and append it to the Windows Registry PATH:
                  </p>
                </div>

                <div className="mt-3 space-y-2">
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={handleInstallNow}
                      disabled={installing}
                      className="rounded bg-accent px-3 py-1.5 text-xs font-semibold text-accent-ink hover:opacity-95 transition-opacity disabled:opacity-50 shadow-xs"
                    >
                      {installing ? "Installing…" : "⚡ Run \"mnemosyne install\" Now"}
                    </button>
                    <span className="text-[11px] text-ink-faint">or in terminal:</span>
                  </div>

                  <div className="flex items-center justify-between rounded bg-editor px-2.5 py-1.5 border border-line">
                    <code className="font-mono text-[11px] text-ink">.\bin\mnemosyne.exe install</code>
                    <button
                      type="button"
                      onClick={() => copy("install-cmd", ".\\bin\\mnemosyne.exe install")}
                      className="flex items-center gap-1 rounded px-2 py-0.5 text-[10.5px] font-medium text-ink-dim hover:bg-hover hover:text-ink transition-colors"
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

                  {installStatus && (
                    <p className={`text-[11px] ${installStatus.ok ? "text-tag font-medium" : "text-danger"}`}>
                      {installStatus.ok ? "✓ " : "✗ "}{installStatus.msg}
                    </p>
                  )}
                </div>
              </div>

              {/* Option 2 */}
              <div className="rounded-lg border border-line bg-shell/80 p-3.5 flex flex-col justify-between">
                <div>
                  <div className="flex items-center gap-1.5 font-semibold text-ink text-xs">
                    <span className="h-2 w-2 rounded-full bg-tag" />
                    <span>Option 2: Direct Workspace Exe (Zero Restart)</span>
                  </div>
                  <p className="mt-1 text-[11px] text-ink-faint leading-relaxed">
                    Point your tools directly to this repo&apos;s dev binary. Works immediately without restarting terminals:
                  </p>
                </div>

                <div className="mt-3 space-y-1.5">
                  <span className="text-[10px] font-mono text-ink-faint block uppercase">Workspace Executable Path:</span>
                  <div className="flex items-center justify-between rounded bg-editor px-2.5 py-1.5 border border-line">
                    <code className="font-mono text-[11px] text-ink truncate mr-2 selectable" title={devExe}>
                      {devExe}
                    </code>
                    <button
                      type="button"
                      onClick={() => copy("dev-exe", devExe)}
                      className="flex shrink-0 items-center gap-1 rounded px-2 py-0.5 text-[10.5px] font-medium text-ink-dim hover:bg-hover hover:text-ink transition-colors"
                    >
                      {copiedKey === "dev-exe" ? (
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

                  <div className="flex items-center justify-between text-[11px] text-ink-faint pt-1">
                    <span>Installed Exe:</span>
                    <button
                      type="button"
                      onClick={() => copy("installed-exe", installedExe)}
                      className="text-accent hover:underline text-[10.5px] font-mono"
                    >
                      Copy Installed Path
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Client Configuration Cards */}
      <div className="flex flex-col gap-3.5">
        {filteredClients.map((client) => {
          const commandText = client.getCommand(activeExe);

          return (
            <section
              key={client.id}
              className="rounded-xl border border-line bg-raised/30 p-4 transition-colors hover:border-line/80 shadow-xs"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <h3 className="font-semibold text-ink text-sm">{client.name}</h3>
                  {client.badge && (
                    <span className="rounded bg-accent/20 px-2 py-0.5 font-mono text-[10px] font-semibold text-accent">
                      {client.badge}
                    </span>
                  )}
                  {client.configPath && (
                    <span className="rounded bg-shell px-2 py-0.5 font-mono text-[10px] text-ink-faint border border-line">
                      {client.configPath}
                    </span>
                  )}
                </div>

                <span className="font-mono text-[10.5px] text-ink-faint">
                  {useDevExe ? "Using dev binary" : "Using system PATH"}
                </span>
              </div>

              <div className="relative mt-3">
                <pre className="selectable overflow-x-auto rounded-lg border border-line bg-editor p-3.5 font-mono text-[11.5px] text-ink leading-relaxed">
                  {commandText}
                </pre>
                <button
                  type="button"
                  onClick={() => copy(client.id, commandText)}
                  className="absolute top-2.5 right-2.5 flex items-center gap-1.5 rounded border border-line bg-raised px-2.5 py-1 text-xs font-semibold text-ink-dim hover:bg-hover hover:text-ink transition-colors shadow-xs"
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

              <p className="mt-2.5 text-xs text-ink-faint leading-relaxed">
                {client.note}
              </p>
            </section>
          );
        })}
      </div>

      {/* Host / Daemon Info */}
      <section className="rounded-xl border border-line bg-shell p-4 text-ink-dim">
        <h3 className="mb-2 font-semibold text-ink text-xs uppercase tracking-wider">
          Local Daemon Details &amp; Runtime
        </h3>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-xs">
          <dt className="text-ink-faint">IPC / Pipe Endpoint:</dt>
          <dd className="selectable font-mono text-[11px] text-ink break-all">
            {endpoint ?? "not connected"}
          </dd>
          <dt className="text-ink-faint">Active Binary Path:</dt>
          <dd className="selectable font-mono text-[11px] text-accent break-all">
            {binaryPath || devExe}
          </dd>
          <dt className="text-ink-faint">Backend Version:</dt>
          <dd className="font-mono text-[11px] text-ink">{health?.version ?? "0.1.0"}</dd>
          <dt className="text-ink-faint">Indexed Memories:</dt>
          <dd className="font-mono text-[11px] text-tag font-semibold">
            {health?.memories ?? "0"}
          </dd>
        </dl>
      </section>
    </div>
  );
}
