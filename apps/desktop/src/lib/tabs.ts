import type { Memory } from "./types";

/** The non-memory surfaces. Each opens as a tab, the way VS Code opens settings. */
export type ViewId = "graph" | "mcp" | "theme" | "settings";

export const VIEW_TITLES: Record<ViewId, string> = {
  graph: "Knowledge Graph",
  mcp: "MCP Clients",
  theme: "Theme Studio",
  settings: "Settings",
};

/**
 * An open memory tab.
 *
 * `slug` is null for a memory that has not been saved yet: the backend derives
 * the slug from the title on first write, so the renderer must not invent one.
 * `key` is therefore a stable synthetic id rather than the path.
 */
export type MemoryTab = {
  kind: "memory";
  key: string;
  project: string;
  slug: string | null;
  title: string;
  body: string;
  tags: string[];
  links: string[];
  dirty: boolean;
  /** What was last read from or written to disk, for the dirty comparison. */
  saved: { title: string; body: string; tags: string[]; links: string[] };
  updated: string | null;
};

/** Graph, MCP, Theme, or Settings, open in the same strip as the memories. */
export type ViewTab = {
  kind: "view";
  key: string;
  view: ViewId;
  title: string;
};

/**
 * One tab strip holds both.
 *
 * These used to be two parallel systems — a `tabs` array for memories and a
 * `mainMode` enum for everything else — which meant opening Settings hid every
 * memory you had open, and the tab bar carried four hand-copied blocks that
 * could not be a list. A discriminated union is what lets them share one array,
 * one active key, and one close path.
 */
export type OpenTab = MemoryTab | ViewTab;

export function tabKey(project: string, slug: string): string {
  return `${project}/${slug}`;
}

export function viewKey(view: ViewId): string {
  return `view:${view}`;
}

export function viewTab(view: ViewId): ViewTab {
  return { kind: "view", key: viewKey(view), view, title: VIEW_TITLES[view] };
}

/** Narrows to a memory tab, for the many callers that only apply to one. */
export function asMemory(tab: OpenTab | null | undefined): MemoryTab | null {
  return tab?.kind === "memory" ? tab : null;
}

let untitledCounter = 0;

/** A tab for a memory that does not exist on disk yet. */
export function draftTab(project: string): MemoryTab {
  untitledCounter += 1;
  return {
    kind: "memory",
    key: `${project}/#draft-${untitledCounter}`,
    project,
    slug: null,
    title: "",
    body: "",
    tags: [],
    links: [],
    // A draft is dirty from the start: it has nothing on disk behind it, so
    // closing it without saving does lose something.
    dirty: true,
    saved: { title: "", body: "", tags: [], links: [] },
    updated: null,
  };
}

export function tabFromMemory(memory: Memory): MemoryTab {
  const snapshot = {
    title: memory.title,
    body: memory.body,
    tags: memory.tags ?? [],
    links: memory.links ?? [],
  };
  return {
    kind: "memory",
    key: tabKey(memory.project, memory.slug),
    project: memory.project,
    slug: memory.slug,
    ...snapshot,
    dirty: false,
    saved: snapshot,
    updated: memory.updated,
  };
}

/** Recomputes `dirty` from the saved snapshot after an edit. */
export function withEdits(tab: MemoryTab, edits: Partial<MemoryTab>): MemoryTab {
  const next = { ...tab, ...edits };
  next.dirty =
    tab.slug === null ||
    next.title !== next.saved.title ||
    next.body !== next.saved.body ||
    !sameList(next.tags, next.saved.tags) ||
    !sameList(next.links, next.saved.links);
  return next;
}

/** Parses the comma-separated tag and link inputs the way the backend would. */
export function parseList(value: string): string[] {
  const seen = new Set<string>();
  for (const part of value.split(",")) {
    const item = part.trim().toLowerCase();
    if (item) seen.add(item);
  }
  return [...seen];
}

function sameList(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((item, i) => item === b[i]);
}
