import type { Memory } from "./types";

/**
 * An open editor tab.
 *
 * `slug` is null for a memory that has not been saved yet: the backend derives
 * the slug from the title on first write, so the renderer must not invent one.
 * `key` is therefore a stable synthetic id rather than the path.
 */
export type OpenTab = {
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

export function tabKey(project: string, slug: string): string {
  return `${project}/${slug}`;
}

let untitledCounter = 0;

/** A tab for a memory that does not exist on disk yet. */
export function draftTab(project: string): OpenTab {
  untitledCounter += 1;
  return {
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

export function tabFromMemory(memory: Memory): OpenTab {
  const snapshot = {
    title: memory.title,
    body: memory.body,
    tags: memory.tags ?? [],
    links: memory.links ?? [],
  };
  return {
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
export function withEdits(tab: OpenTab, edits: Partial<OpenTab>): OpenTab {
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
