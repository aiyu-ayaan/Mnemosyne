import { useCallback, useEffect, useMemo, useState } from "react";

import ActivityBar, { type View } from "./components/ActivityBar";
import Editor from "./components/Editor";
import Explorer from "./components/Explorer";
import GraphView, { GraphSidebar } from "./components/GraphView";
import { GraphIcon, PaletteIcon, PlugIcon } from "./components/Icons";
import McpView from "./components/McpView";
import Panel, { type PanelTab } from "./components/Panel";
import Prompt, { type Ask } from "./components/Prompt";
import QuickOpen from "./components/QuickOpen";
import SearchView, { type SearchMode } from "./components/SearchView";
import SettingsView from "./components/SettingsView";
import StatusBar from "./components/StatusBar";
import Tabs, { type MainMode } from "./components/Tabs";
import ThemeView from "./components/ThemeView";
import { api, bridge } from "./lib/bridge";
import { draftTab, tabFromMemory, tabKey, type OpenTab } from "./lib/tabs";
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
  const [mainMode, setMainMode] = useState<MainMode>("editor");

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
  const [quickOpen, setQuickOpen] = useState(false);
  const [ask, setAsk] = useState<Ask | null>(null);
  const [log, setLog] = useState<LogEntry[]>([]);
  const [channel, setChannel] = useState<ChannelInfo | null>(null);
  const [fatal, setFatal] = useState<string | null>(null);
  const [backlinks, setBacklinks] = useState<Meta[]>([]);

  const activeTab = tabs.find((tab) => tab.key === activeKey) ?? null;

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
        setMainMode("editor");
        return;
      }
      try {
        const memory = await api<Memory>(
          "GET",
          `/v1/projects/${encodeURIComponent(project)}/memories/${encodeURIComponent(slug)}`,
        );
        setTabs((prev) => [...prev, tabFromMemory(memory)]);
        setActiveKey(key);
        setMainMode("editor");
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

      if (tab?.dirty) {
        setAsk({
          title: `Discard changes to ${tab.title || tab.slug || "this memory"}?`,
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
    if (!activeTab || !activeTab.dirty || activeTab.title.trim() === "") return;

    setSaving(true);
    setSaveError(null);
    try {
      const base = `/v1/projects/${encodeURIComponent(activeTab.project)}/memories`;
      const body = {
        title: activeTab.title,
        body: activeTab.body,
        tags: activeTab.tags,
        links: activeTab.links,
      };

      // POST creates and lets the backend derive the slug from the title; PUT
      // updates in place. The renderer never invents a slug.
      const saved =
        activeTab.slug === null
          ? await api<Memory>("POST", base, body)
          : await api<Memory>("PUT", `${base}/${encodeURIComponent(activeTab.slug)}`, body);

      const fresh = tabFromMemory(saved);
      setTabs((prev) => prev.map((tab) => (tab.key === activeTab.key ? fresh : tab)));
      setActiveKey(fresh.key);
      library.reload();
    } catch (err) {
      setSaveError(reportError(err));
    } finally {
      setSaving(false);
    }
  }, [activeTab, library, reportError]);

  const deleteActive = useCallback(() => {
    if (!activeTab?.slug) return;
    const { project, slug, title, key } = activeTab;

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
  }, [activeTab, library, reportError]);

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
              const next = prev.filter((tab) => tab.project !== project);
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
    if (!activeTab?.slug) {
      setBacklinks([]);
      return;
    }
    let cancelled = false;
    const { project, slug } = activeTab;

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
  }, [activeTab?.project, activeTab?.slug, activeTab?.updated]);

  // --- keyboard ---

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const mod = e.ctrlKey || e.metaKey;
      if (!mod) return;

      const key = e.key.toLowerCase();
      if (key === "p" && !e.shiftKey) {
        e.preventDefault();
        setQuickOpen(true);
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
  const dirtyCount = tabs.filter((tab) => tab.dirty).length;

  const onActivity = (next: View) => {
    if (next === "mcp") {
      setMainMode((current) => (current === "mcp" ? "editor" : "mcp"));
      return;
    }
    if (next === "theme") {
      setMainMode((current) => (current === "theme" ? "editor" : "theme"));
      return;
    }
    if (next === "settings") {
      setMainMode((current) => (current === "settings" ? "editor" : "settings"));
      return;
    }
    if (next === "graph") {
      setView("graph");
      setSidebarOpen(true);
      setMainMode("graph");
      return;
    }
    if (next === "explorer") {
      if (view === "explorer" && sidebarOpen && mainMode === "editor") {
        setSidebarOpen(false);
      } else {
        setView("explorer");
        setSidebarOpen(true);
        if (mainMode !== "editor" && mainMode !== "graph") {
          setMainMode("editor");
        }
      }
      return;
    }
    if (next === "search") {
      if (view === "search" && sidebarOpen) {
        setSidebarOpen(false);
      } else {
        setView("search");
        setSidebarOpen(true);
      }
      return;
    }
  };

  const activeActivityView: View =
    mainMode === "mcp"
      ? "mcp"
      : mainMode === "theme"
        ? "theme"
        : mainMode === "settings"
          ? "settings"
          : mainMode === "graph"
            ? "graph"
            : view;

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
                onOpen={(meta) => {
                  openMemory(meta.project, meta.slug);
                  setMainMode("editor");
                }}
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
                activeProject={activeTab?.project ?? null}
                onOpenMemory={(project, slug) => {
                  openMemory(project, slug);
                  setMainMode("editor");
                }}
              />
            )}
          </aside>
        )}

        <main className="flex min-w-0 flex-1 flex-col">
          <Tabs
            tabs={tabs}
            activeKey={activeKey}
            onSelect={(key) => {
              setActiveKey(key);
              setMainMode("editor");
            }}
            onClose={closeTab}
            mainMode={mainMode}
            onCloseSpecial={() => setMainMode("editor")}
            onSelectMode={(mode) => setMainMode(mode)}
          />

          <div className="min-h-0 flex-1 overflow-hidden">
            {mainMode === "graph" ? (
              <GraphView
                projects={library.projects}
                activeProject={activeTab?.project ?? null}
                onOpenMemory={(project, slug) => {
                  openMemory(project, slug);
                  setMainMode("editor");
                }}
              />
            ) : mainMode === "mcp" ? (
              <McpView
                health={library.health}
                endpoint={channel?.endpoint ?? null}
                binaryPath={channel?.binaryPath ?? null}
              />
            ) : mainMode === "theme" ? (
              <ThemeView theme={theme} onTheme={setTheme} />
            ) : mainMode === "settings" ? (
              <SettingsView
                health={library.health}
                theme={theme}
                onTheme={setTheme}
                onChanged={library.reload}
                onOpenThemeStudio={() => setMainMode("theme")}
              />
            ) : activeTab ? (
              <Editor
                tab={activeTab}
                saving={saving}
                error={saveError}
                onChange={(next) =>
                  setTabs((prev) => prev.map((tab) => (tab.key === next.key ? next : tab)))
                }
                onSave={saveTab}
                onDelete={deleteActive}
                onOpenWikilink={(targetSlug) => {
                  const project = activeTab.project;
                  openMemory(project, targetSlug);
                }}
              />
            ) : (
              <Welcome
                error={library.error}
                onOpenGraph={() => {
                  setView("graph");
                  setMainMode("graph");
                }}
                onOpenMcp={() => setMainMode("mcp")}
                onOpenTheme={() => setMainMode("theme")}
              />
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
              backlinksFor={activeTab?.slug ?? null}
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
        activeProject={activeTab?.project ?? null}
        dirtyCount={dirtyCount}
        theme={theme}
        onTheme={setTheme}
        onTogglePanel={() => setPanelOpen((v) => !v)}
        onOpenThemeStudio={() => setMainMode("theme")}
      />

      {quickOpen && (
        <QuickOpen
          memories={allMemories}
          onPick={(meta) => {
            setQuickOpen(false);
            openMemory(meta.project, meta.slug);
          }}
          onClose={() => setQuickOpen(false)}
        />
      )}

      {ask && <Prompt ask={ask} onCancel={() => setAsk(null)} />}
    </div>
  );
}

function Welcome({
  error,
  onOpenGraph,
  onOpenMcp,
  onOpenTheme,
}: {
  error: string | null;
  onOpenGraph?: () => void;
  onOpenMcp?: () => void;
  onOpenTheme?: () => void;
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
          Pick a memory from the Explorer on the left, or press{" "}
          <kbd className="rounded border border-line px-1.5 py-0.5 font-mono text-[11px] bg-shell">Ctrl+P</kbd> to quickly search memories.
        </p>
      )}

      <div className="flex flex-wrap items-center justify-center gap-2 mt-1">
        {onOpenGraph && (
          <button
            type="button"
            onClick={onOpenGraph}
            className="flex items-center gap-1.5 rounded-lg border border-line bg-raised px-3 py-1.5 text-xs text-ink hover:bg-hover hover:border-tag transition-all shadow-xs"
          >
            <GraphIcon className="h-3.5 w-3.5 text-tag" />
            <span>Knowledge Graph</span>
          </button>
        )}

        {onOpenMcp && (
          <button
            type="button"
            onClick={onOpenMcp}
            className="flex items-center gap-1.5 rounded-lg border border-line bg-raised px-3 py-1.5 text-xs text-ink hover:bg-hover hover:border-accent transition-all shadow-xs"
          >
            <PlugIcon className="h-3.5 w-3.5 text-accent" />
            <span>AI &amp; MCP Integration</span>
          </button>
        )}

        {onOpenTheme && (
          <button
            type="button"
            onClick={onOpenTheme}
            className="flex items-center gap-1.5 rounded-lg border border-line bg-raised px-3 py-1.5 text-xs text-ink hover:bg-hover hover:border-accent transition-all shadow-xs"
          >
            <PaletteIcon className="h-3.5 w-3.5 text-accent" />
            <span>Theme Studio</span>
          </button>
        )}
      </div>

      <dl className="mt-4 grid grid-cols-[auto_1fr] gap-x-5 gap-y-1.5 text-left font-mono text-[11px] text-ink-faint border-t border-line/60 pt-4">
        <dt>Ctrl+P</dt>
        <dd>go to memory</dd>
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
