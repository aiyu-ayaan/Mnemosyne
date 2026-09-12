import { useCallback, useEffect, useMemo, useState } from "react";

import ActivityBar, { type View } from "./components/ActivityBar";
import Editor from "./components/Editor";
import Explorer from "./components/Explorer";
import GraphView, { GraphSidebar } from "./components/GraphView";
import { GraphIcon, PaletteIcon, PlugIcon } from "./components/Icons";
import McpView from "./components/McpView";
import Panel, { type PanelTab } from "./components/Panel";
import Prompt, { type Ask } from "./components/Prompt";
import QuickOpen, { type Command } from "./components/QuickOpen";
import SearchView, { type SearchMode } from "./components/SearchView";
import SettingsView from "./components/SettingsView";
import StatusBar from "./components/StatusBar";
import Tabs from "./components/Tabs";
import ThemeView from "./components/ThemeView";
import { api, bridge } from "./lib/bridge";
import {
  asMemory,
  draftTab,
  tabFromMemory,
  tabKey,
  viewKey,
  viewTab,
  type OpenTab,
  type ViewId,
} from "./lib/tabs";
import type { ChangeEvent, ChannelInfo, Hit, Memory, Meta, ThemeId } from "./lib/types";
import { useLibrary } from "./lib/useLibrary";

type LogEntry = ChangeEvent | { kind: "error"; message: string; at: string };

/** How many events the Output panel keeps. It is a tail, not a history. */
const logLimit = 200;

export default function App() {
  const library = useLibrary();

  const [theme, setTheme] = useState<ThemeId>(
    () => (localStorage.getItem("mnemosyne.theme") as ThemeId | null) ?? "dark",
  );
  const [view, setView] = useState<View>("explorer");
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [panelOpen, setPanelOpen] = useState(true);
  const [panelTab, setPanelTab] = useState<PanelTab>("search");

  const [tabs, setTabs] = useState<OpenTab[]>([]);
  const [activeKey, setActiveKey] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const [search, setSearch] = useState<{ query: string; project: string; mode: SearchMode }>({
    query: "",
    project: "",
    mode: "text",
  });
  const [submitted, setSubmitted] = useState("");
  const [results, setResults] = useState<Hit[]>([]);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [searching, setSearching] = useState(false);
  const [semanticAvailable, setSemanticAvailable] = useState(false);

  const [tagFilter, setTagFilter] = useState<string | null>(null);
  // null when closed; otherwise which mode the palette opened in.
  const [palette, setPalette] = useState<"memories" | "commands" | null>(null);
  const [ask, setAsk] = useState<Ask | null>(null);
  const [log, setLog] = useState<LogEntry[]>([]);
  const [channel, setChannel] = useState<ChannelInfo | null>(null);
  const [fatal, setFatal] = useState<string | null>(null);
  const [backlinks, setBacklinks] = useState<Meta[]>([]);

  const activeTab = tabs.find((tab) => tab.key === activeKey) ?? null;
  // Most of the app only means something for a memory. Narrowing once here
  // beats a `kind === "memory"` check at every use.
  const activeMemory = asMemory(activeTab);

  const note = useCallback((entry: LogEntry) => {
    setLog((prev) => [...prev, entry].slice(-logLimit));
  }, []);

  const reportError = useCallback(
    (err: unknown) => {
      const message = err instanceof Error ? err.message : String(err);
      note({ kind: "error", message, at: new Date().toISOString() });
      return message;
    },
    [note],
  );

  // --- theme ---

  useEffect(() => {
    const root = document.documentElement;
    root.classList.remove(
      "light",
      "theme-midnight",
      "theme-tokyo",
      "theme-nord",
      "theme-catppuccin",
      "theme-monokai",
    );
    if (theme === "light") {
      root.classList.add("light");
    } else if (theme !== "dark") {
      root.classList.add(`theme-${theme}`);
    }
    localStorage.setItem("mnemosyne.theme", theme);
  }, [theme]);

  // --- daemon connection and events ---

  useEffect(() => {
    bridge.info().then(setChannel).catch(reportError);
    const stopFatal = bridge.onFatal(setFatal);
    const stopEvents = bridge.onEvent(note);
    return () => {
      stopFatal();
      stopEvents();
    };
  }, [note, reportError]);

  // Semantic ranking needs an embedding provider, which the daemon reports on.
  // Asking once at startup keeps the radio buttons honest instead of letting a
  // search fail to find out.
  useEffect(() => {
    api<{ available: boolean }>("GET", "/v1/embeddings")
      .then((r) => setSemanticAvailable(r.available))
      .catch(() => setSemanticAvailable(false));
  }, []);

  // --- tabs ---

  const openMemory = useCallback(
    async (project: string, slug: string) => {
      const key = tabKey(project, slug);
      const existing = tabs.find((tab) => tab.key === key);
      // An open tab may hold unsaved edits, so it is selected rather than
      // refetched over the top of them.
      if (existing) {
        setActiveKey(key);
        return;
      }
      try {
        const memory = await api<Memory>(
          "GET",
          `/v1/projects/${encodeURIComponent(project)}/memories/${encodeURIComponent(slug)}`,
        );
        setTabs((prev) => [...prev, tabFromMemory(memory)]);
        setActiveKey(key);
        setSaveError(null);
      } catch (err) {
        reportError(err);
        setPanelTab("output");
        setPanelOpen(true);
      }
    },
    [tabs, reportError],
  );

  const closeTab = useCallback(
    (key: string) => {
      const tab = tabs.find((t) => t.key === key);
      const drop = () => {
        setTabs((prev) => {
          const next = prev.filter((t) => t.key !== key);
          setActiveKey((current) =>
            current === key ? (next[next.length - 1]?.key ?? null) : current,
          );
          return next;
        });
        setAsk(null);
      };

      const memory = asMemory(tab);
      if (memory?.dirty) {
        setAsk({
          title: `Discard changes to ${memory.title || memory.slug || "this memory"}?`,
          message: "It has unsaved edits. They cannot be recovered.",
          confirmLabel: "Discard",
          danger: true,
          onConfirm: drop,
        });
        return;
      }
      drop();
    },
    [tabs],
  );

  const saveTab = useCallback(async () => {
    if (!activeMemory || !activeMemory.dirty || activeMemory.title.trim() === "") return;
    const current = activeMemory;

    setSaving(true);
    setSaveError(null);
    try {
      const base = `/v1/projects/${encodeURIComponent(current.project)}/memories`;
      const body = {
        title: current.title,
        body: current.body,
        tags: current.tags,
        links: current.links,
      };

      // POST creates and lets the backend derive the slug from the title; PUT
      // updates in place. The renderer never invents a slug.
      const saved =
        current.slug === null
          ? await api<Memory>("POST", base, body)
          : await api<Memory>("PUT", `${base}/${encodeURIComponent(current.slug)}`, body);

      const fresh = tabFromMemory(saved);
      setTabs((prev) => prev.map((tab) => (tab.key === current.key ? fresh : tab)));
      setActiveKey(fresh.key);
      library.reload();
    } catch (err) {
      setSaveError(reportError(err));
    } finally {
      setSaving(false);
    }
  }, [activeMemory, library, reportError]);

  const deleteActive = useCallback(() => {
    if (!activeMemory?.slug) return;
    const { project, slug, title, key } = activeMemory;

    setAsk({
      title: `Delete ${title || slug}?`,
      message: "The Markdown file is removed. There is no undo and no backup yet.",
      confirmLabel: "Delete",
      danger: true,
      onConfirm: async () => {
        setAsk(null);
        try {
          await api<void>(
            "DELETE",
            `/v1/projects/${encodeURIComponent(project)}/memories/${encodeURIComponent(slug)}`,
          );
          setTabs((prev) => {
            const next = prev.filter((tab) => tab.key !== key);
            setActiveKey(next[next.length - 1]?.key ?? null);
            return next;
          });
          library.reload();
        } catch (err) {
          setSaveError(reportError(err));
        }
      },
    });
  }, [activeMemory, library, reportError]);

  const newMemory = useCallback((project: string) => {
    const tab = draftTab(project);
    setTabs((prev) => [...prev, tab]);
    setActiveKey(tab.key);
    setSaveError(null);
  }, []);

  const newProject = useCallback(() => {
    setAsk({
      title: "New project",
      message: "Lowercase letters, digits, and hyphens — it becomes the directory name.",
      initial: "",
      placeholder: "my-project",
      confirmLabel: "Create",
      onConfirm: async (value) => {
        setAsk(null);
        try {
          await api<unknown>("POST", "/v1/projects", { project: value });
          library.reload();
        } catch (err) {
          reportError(err);
          setPanelTab("output");
          setPanelOpen(true);
        }
      },
    });
  }, [library, reportError]);

  const deleteProject = useCallback(
    (project: string) => {
      const count = library.projects.find((p) => p.slug === project)?.memoryCount ?? 0;
      setAsk({
        title: `Delete project ${project}?`,
        message: `Its directory and all ${count} ${count === 1 ? "memory" : "memories"} are removed. There is no undo.`,
        confirmLabel: "Delete",
        danger: true,
        onConfirm: async () => {
          setAsk(null);
          try {
            await api<void>("DELETE", `/v1/projects/${encodeURIComponent(project)}`);
            setTabs((prev) => {
              const next = prev.filter((tab) => tab.kind !== "memory" || tab.project !== project);
              setActiveKey(next[next.length - 1]?.key ?? null);
              return next;
            });
            library.reload();
          } catch (err) {
            reportError(err);
            setPanelTab("output");
            setPanelOpen(true);
          }
        },
      });
    },
    [library, reportError],
  );

  // --- search ---

  const runSearch = useCallback(async () => {
    const query = search.query.trim();
    if (!query) return;

    setSearching(true);
    setSearchError(null);
    setSubmitted(query);
    setPanelTab("search");
    setPanelOpen(true);
    try {
      const params = new URLSearchParams({ q: query, mode: search.mode });
      if (search.project) params.set("project", search.project);
      const { results: hits } = await api<{ results: Hit[] }>("GET", `/v1/search?${params}`);
      setResults(hits);
    } catch (err) {
      setResults([]);
      setSearchError(reportError(err));
    } finally {
      setSearching(false);
    }
  }, [search, reportError]);

  // --- backlinks for the open memory ---

  useEffect(() => {
    if (!activeMemory?.slug) {
      setBacklinks([]);
      return;
    }
    let cancelled = false;
    const { project, slug } = activeMemory;

    api<{ backlinks: Meta[] }>(
      "GET",
      `/v1/projects/${encodeURIComponent(project)}/memories/${encodeURIComponent(slug)}/backlinks`,
    )
      .then(({ backlinks: found }) => {
        if (!cancelled) setBacklinks(found);
      })
      // Backlinks land in Phase 4. Until the route exists, an empty list is the
      // right answer and not worth reporting as a failure.
      .catch(() => {
        if (!cancelled) setBacklinks([]);
      });

    return () => {
      cancelled = true;
    };
  }, [activeMemory?.project, activeMemory?.slug, activeMemory?.updated]);

  // --- keyboard ---

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const mod = e.ctrlKey || e.metaKey;
      if (!mod) return;

      const key = e.key.toLowerCase();
      if (key === "p") {
        e.preventDefault();
        setPalette(e.shiftKey ? "commands" : "memories");
      } else if (key === "f" && e.shiftKey) {
        e.preventDefault();
        setView("search");
        setSidebarOpen(true);
      } else if (key === "e" && e.shiftKey) {
        e.preventDefault();
        setView("explorer");
        setSidebarOpen(true);
      } else if (key === "b") {
        e.preventDefault();
        setSidebarOpen((v) => !v);
      } else if (key === "j") {
        e.preventDefault();
        setPanelOpen((v) => !v);
      } else if (key === "s") {
        e.preventDefault();
        saveTab();
      } else if (key === "w") {
        e.preventDefault();
        if (activeKey) closeTab(activeKey);
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [saveTab, closeTab, activeKey]);

  // --- derived ---

  const allMemories = useMemo(
    () => Object.values(library.memories).flat(),
    [library.memories],
  );
  const dirtyCount = tabs.filter((tab) => tab.kind === "memory" && tab.dirty).length;

  // Open the tab for a view, or focus it if it is already open. This is the
  // whole of what used to be a five-branch mainMode state machine: a view is
  // just a tab, so "show me settings" is the same action as "show me a memory".
  const openView = useCallback((view: ViewId) => {
    const key = viewKey(view);
    setTabs((prev) => (prev.some((tab) => tab.key === key) ? prev : [...prev, viewTab(view)]));
    setActiveKey(key);
  }, []);

  // The palette's command list. Everything here is reachable another way too —
  // the palette exists so none of it has to be found first.
  const commands: Command[] = useMemo(() => {
    const project = activeMemory?.project ?? library.projects[0]?.slug ?? null;
    return [
      { id: "memory.new", group: "Memory", label: "New Memory", disabled: project === null,
        run: () => project && newMemory(project) },
      { id: "memory.save", group: "Memory", label: "Save", hint: "Ctrl+S",
        disabled: !activeMemory?.dirty, run: saveTab },
      { id: "memory.delete", group: "Memory", label: "Delete Memory",
        disabled: !activeMemory?.slug, run: deleteActive },
      { id: "memory.close", group: "Memory", label: "Close Tab", hint: "Ctrl+W",
        disabled: activeKey === null, run: () => activeKey && closeTab(activeKey) },
      { id: "project.new", group: "Project", label: "New Project", run: newProject },
      { id: "view.graph", group: "View", label: "Knowledge Graph", run: () => openView("graph") },
      { id: "view.mcp", group: "View", label: "MCP Clients", run: () => openView("mcp") },
      { id: "view.theme", group: "View", label: "Theme Studio", run: () => openView("theme") },
      { id: "view.settings", group: "View", label: "Settings", run: () => openView("settings") },
      { id: "view.explorer", group: "View", label: "Show Explorer", hint: "Ctrl+Shift+E",
        run: () => { setView("explorer"); setSidebarOpen(true); } },
      { id: "view.search", group: "View", label: "Show Search", hint: "Ctrl+Shift+F",
        run: () => { setView("search"); setSidebarOpen(true); } },
      { id: "view.sidebar", group: "View", label: "Toggle Sidebar", hint: "Ctrl+B",
        run: () => setSidebarOpen((v) => !v) },
      { id: "view.panel", group: "View", label: "Toggle Panel", hint: "Ctrl+J",
        run: () => setPanelOpen((v) => !v) },
    ];
  }, [activeMemory, activeKey, library.projects, newMemory, newProject, saveTab, deleteActive, closeTab, openView]);

  const onActivity = (next: View) => {
    if (next === "explorer" || next === "search") {
      // Clicking the active navigator collapses it, as in VS Code.
      if (view === next && sidebarOpen) {
        setSidebarOpen(false);
        return;
      }
      setView(next);
      setSidebarOpen(true);
      return;
    }
    if (next === "graph") {
      setView("graph");
      setSidebarOpen(true);
    }
    openView(next);
  };

  // The activity bar highlights the navigator on the left, except that an open
  // view tab is the more specific answer to "where am I".
  const activeActivityView: View =
    activeTab?.kind === "view" && activeTab.view !== "graph" ? activeTab.view : view;

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {fatal && (
        <div className="shrink-0 bg-danger px-3 py-2 text-accent-ink">
          <strong>Could not reach the Mnemosyne daemon.</strong> {fatal}
        </div>
      )}

      <div className="flex min-h-0 flex-1">
        <ActivityBar active={activeActivityView} sidebarOpen={sidebarOpen} onSelect={onActivity} />

        {sidebarOpen && (
          <aside className="w-64 shrink-0 overflow-hidden border-r border-line bg-shell">
            {view === "explorer" && (
              <Explorer
                library={library}
                activeKey={activeKey}
                tagFilter={tagFilter}
                onOpen={(meta) => openMemory(meta.project, meta.slug)}
                onTagFilter={setTagFilter}
                onNewMemory={newMemory}
                onNewProject={newProject}
                onDeleteProject={deleteProject}
              />
            )}
            {view === "search" && (
              <SearchView
                projects={library.projects}
                query={search.query}
                project={search.project}
                mode={search.mode}
                semanticAvailable={semanticAvailable}
                busy={searching}
                onChange={setSearch}
                onSubmit={runSearch}
              />
            )}
            {view === "graph" && (
              <GraphSidebar
                projects={library.projects}
                activeProject={activeMemory?.project ?? null}
                onOpenMemory={openMemory}
              />
            )}
          </aside>
        )}

        <main className="flex min-w-0 flex-1 flex-col">
          <Tabs tabs={tabs} activeKey={activeKey} onSelect={setActiveKey} onClose={closeTab} />

          <div className="min-h-0 flex-1 overflow-hidden">
            {activeTab === null ? (
              <Welcome error={library.error} onOpenView={openView} />
            ) : activeTab.kind === "memory" ? (
              <Editor
                tab={activeTab}
                saving={saving}
                error={saveError}
                onChange={(next) =>
                  setTabs((prev) => prev.map((tab) => (tab.key === next.key ? next : tab)))
                }
                onSave={saveTab}
                onDelete={deleteActive}
                onOpenWikilink={(targetSlug) => openMemory(activeTab.project, targetSlug)}
              />
            ) : activeTab.view === "graph" ? (
              <GraphView
                projects={library.projects}
                activeProject={activeMemory?.project ?? null}
                onOpenMemory={openMemory}
              />
            ) : activeTab.view === "mcp" ? (
              <McpView
                health={library.health}
                endpoint={channel?.endpoint ?? null}
                binaryPath={channel?.binaryPath ?? null}
              />
            ) : activeTab.view === "theme" ? (
              <ThemeView theme={theme} onTheme={setTheme} />
            ) : (
              <SettingsView health={library.health} onChanged={library.reload} />
            )}
          </div>

          {panelOpen && (
            <Panel
              tab={panelTab}
              onTab={setPanelTab}
              onClose={() => setPanelOpen(false)}
              query={submitted}
              results={results}
              searchError={searchError}
              backlinks={backlinks}
              backlinksFor={activeMemory?.slug ?? null}
              endpoint={channel?.endpoint ?? null}
              daemonRoot={library.health?.root ?? null}
              log={log}
              onOpenHit={openMemory}
            />
          )}
        </main>
      </div>

      <StatusBar
        health={library.health}
        connected={channel !== null && !fatal}
        activeProject={activeMemory?.project ?? null}
        dirtyCount={dirtyCount}
        theme={theme}
        onTheme={setTheme}
        onTogglePanel={() => setPanelOpen((v) => !v)}
      />

      {palette && (
        <QuickOpen
          memories={allMemories}
          commands={commands}
          commandMode={palette === "commands"}
          onPick={(meta) => {
            setPalette(null);
            openMemory(meta.project, meta.slug);
          }}
          onClose={() => setPalette(null)}
        />
      )}

      {ask && <Prompt ask={ask} onCancel={() => setAsk(null)} />}
    </div>
  );
}

function Welcome({
  error,
  onOpenView,
}: {
  error: string | null;
  onOpenView: (view: ViewId) => void;
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-4 p-8 text-center text-ink-dim select-none">
      <div className="flex flex-col items-center gap-1">
        <h1 className="text-2xl font-bold tracking-tight text-ink">Mnemosyne</h1>
        <p className="text-xs text-ink-faint">Local memory server &amp; knowledge graph for AI pair programmers</p>
      </div>

      {error ? (
        <p className="max-w-md text-danger">{error}</p>
      ) : (
        <p className="max-w-md text-xs text-ink-dim">
          Pick a memory from the Explorer on the left, press{" "}
          <kbd className="rounded border border-line bg-shell px-1.5 py-0.5 font-mono text-[11px]">Ctrl+P</kbd>{" "}
          to jump to one, or{" "}
          <kbd className="rounded border border-line bg-shell px-1.5 py-0.5 font-mono text-[11px]">Ctrl+Shift+P</kbd>{" "}
          for commands.
        </p>
      )}

      <div className="mt-1 flex flex-wrap items-center justify-center gap-2">
        {(
          [
            ["graph", GraphIcon, "text-tag", "Knowledge Graph"],
            ["mcp", PlugIcon, "text-accent", "AI & MCP Integration"],
            ["theme", PaletteIcon, "text-accent", "Theme Studio"],
          ] as const
        ).map(([view, Icon, tint, label]) => (
          <button
            key={view}
            type="button"
            onClick={() => onOpenView(view)}
            className="flex items-center gap-1.5 rounded-lg border border-line bg-raised px-3 py-1.5 text-xs text-ink shadow-xs transition-all hover:border-accent hover:bg-hover"
          >
            <Icon className={`h-3.5 w-3.5 ${tint}`} />
            <span>{label}</span>
          </button>
        ))}
      </div>

      <dl className="mt-4 grid grid-cols-[auto_1fr] gap-x-5 gap-y-1.5 text-left font-mono text-[11px] text-ink-faint border-t border-line/60 pt-4">
        <dt>Ctrl+P</dt>
        <dd>go to memory</dd>
        <dt>Ctrl+Shift+P</dt>
        <dd>run a command</dd>
        <dt>Ctrl+Shift+F</dt>
        <dd>search memories</dd>
        <dt>Ctrl+B</dt>
        <dd>toggle sidebar</dd>
        <dt>Ctrl+J</dt>
        <dd>toggle bottom panel</dd>
        <dt>Ctrl+S</dt>
        <dd>save changes</dd>
        <dt>Ctrl+W</dt>
        <dd>close active tab</dd>
      </dl>
    </div>
  );
}
