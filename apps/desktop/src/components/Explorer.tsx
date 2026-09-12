import { useMemo, useState } from "react";

import type { Meta } from "../lib/types";
import type { Library } from "../lib/useLibrary";
import {
  Chevron,
  DocIcon,
  FilterIcon,
  FolderIcon,
  FolderOpenIcon,
  PlusIcon,
  TagIcon,
  TrashIcon,
} from "./Icons";

type Props = {
  library: Library;
  activeKey: string | null;
  tagFilter: string | null;
  onOpen: (meta: Meta) => void;
  onTagFilter: (tag: string | null) => void;
  onNewMemory: (project: string) => void;
  onNewProject: () => void;
  onDeleteProject: (project: string) => void;
};

export default function Explorer({
  library,
  activeKey,
  tagFilter,
  onOpen,
  onTagFilter,
  onNewMemory,
  onNewProject,
  onDeleteProject,
}: Props) {
  const { projects, memories, tags } = library;

  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [tagsOpen, setTagsOpen] = useState(true);
  const [quickFilter, setQuickFilter] = useState("");

  const toggle = (slug: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (!next.delete(slug)) next.add(slug);
      return next;
    });

  const totalMemories = useMemo(
    () => projects.reduce((acc, p) => acc + (p.memoryCount || 0), 0),
    [projects],
  );

  const visible = (metas: Meta[]) => {
    let result = metas;
    if (tagFilter) {
      result = result.filter((meta) => (meta.tags ?? []).includes(tagFilter));
    }
    if (quickFilter.trim()) {
      const q = quickFilter.trim().toLowerCase();
      result = result.filter(
        (meta) =>
          meta.title.toLowerCase().includes(q) ||
          meta.slug.toLowerCase().includes(q) ||
          (meta.tags ?? []).some((t) => t.toLowerCase().includes(q)),
      );
    }
    return result;
  };

  return (
    <div className="flex h-full flex-col overflow-hidden text-ink-dim select-none">
      {/* Explorer Header */}
      <header className="flex items-center justify-between border-b border-line px-3 py-2 text-[11px] font-semibold uppercase tracking-wider text-ink">
        <div className="flex items-center gap-1.5">
          <span>Memories</span>
          <span className="rounded bg-raised px-1.5 py-0.2 font-mono text-[10px] text-ink-faint">
            {totalMemories}
          </span>
        </div>
        <button
          type="button"
          title="New project"
          aria-label="New project"
          onClick={onNewProject}
          className="rounded p-1 text-ink-faint hover:bg-hover hover:text-ink transition-colors"
        >
          <PlusIcon className="h-3.5 w-3.5" />
        </button>
      </header>

      {/* Quick Filter Input */}
      <div className="border-b border-line px-2 py-1.5">
        <div className="relative flex items-center">
          <input
            type="text"
            value={quickFilter}
            onChange={(e) => setQuickFilter(e.target.value)}
            placeholder="Filter memories..."
            className="w-full rounded border border-line bg-editor px-2 py-1 text-[11px] text-ink placeholder:text-ink-faint outline-none focus:border-accent"
          />
          {quickFilter && (
            <button
              type="button"
              onClick={() => setQuickFilter("")}
              className="absolute right-1.5 text-xs text-ink-faint hover:text-ink"
            >
              ✕
            </button>
          )}
        </div>
      </div>

      {/* Active Tag Filter Indicator */}
      {tagFilter && (
        <div className="flex items-center justify-between border-b border-line bg-selected/30 px-3 py-1 text-[11px] text-tag">
          <span className="flex items-center gap-1 font-mono">
            <FilterIcon className="h-3 w-3" />
            <span>#{tagFilter}</span>
          </span>
          <button
            type="button"
            onClick={() => onTagFilter(null)}
            className="rounded px-1 text-[10px] text-ink-faint hover:text-ink"
          >
            Clear ✕
          </button>
        </div>
      )}

      {/* Tree Content */}
      <div className="min-h-0 flex-1 overflow-y-auto py-1">
        {projects.length === 0 && !library.loading && (
          <div className="p-4 text-center">
            <p className="text-xs text-ink-faint mb-2">No projects created yet.</p>
            <button
              type="button"
              onClick={onNewProject}
              className="rounded border border-line bg-raised px-2.5 py-1 text-xs text-ink hover:bg-hover transition-colors"
            >
              + Create First Project
            </button>
          </div>
        )}

        {projects.map((project) => {
          const open = !collapsed.has(project.slug);
          const metas = visible(memories[project.slug] ?? []);

          return (
            <section key={project.slug} className="mb-0.5">
              {/* Project Row */}
              <div className="group flex items-center justify-between px-2 py-1 hover:bg-hover rounded-sm transition-colors">
                <button
                  type="button"
                  onClick={() => toggle(project.slug)}
                  aria-expanded={open}
                  className="flex min-w-0 flex-1 items-center gap-1.5 text-left text-xs text-ink font-medium"
                >
                  <Chevron open={open} className="h-3 w-3 text-ink-faint shrink-0" />
                  {open ? (
                    <FolderOpenIcon className="h-3.5 w-3.5 text-accent shrink-0" />
                  ) : (
                    <FolderIcon className="h-3.5 w-3.5 text-accent shrink-0" />
                  )}
                  <span className="truncate">{project.name || project.slug}</span>
                  <span className="ml-1 font-mono text-[10px] text-ink-faint">
                    {project.memoryCount}
                  </span>
                </button>

                <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    type="button"
                    title={`New memory in ${project.slug}`}
                    aria-label={`New memory in ${project.slug}`}
                    onClick={() => onNewMemory(project.slug)}
                    className="rounded p-1 text-ink-faint hover:text-ink hover:bg-raised transition-colors"
                  >
                    <PlusIcon className="h-3 w-3" />
                  </button>
                  <button
                    type="button"
                    title={`Delete project ${project.slug}`}
                    aria-label={`Delete project ${project.slug}`}
                    onClick={() => onDeleteProject(project.slug)}
                    className="rounded p-1 text-ink-faint hover:text-danger hover:bg-raised transition-colors"
                  >
                    <TrashIcon className="h-3 w-3" />
                  </button>
                </div>
              </div>

              {/* Memory Children Rows */}
              {open && (
                <div className="ml-3 pl-2 border-l border-line/60">
                  {metas.map((meta) => {
                    const key = `${meta.project}/${meta.slug}`;
                    const active = activeKey === key;
                    return (
                      <button
                        key={key}
                        type="button"
                        onClick={() => onOpen(meta)}
                        title={meta.title || meta.slug}
                        className={`group flex w-full items-center justify-between rounded px-2 py-1 text-left text-xs transition-colors ${
                          active
                            ? "bg-selected text-ink font-medium shadow-xs"
                            : "hover:bg-hover text-ink-dim hover:text-ink"
                        }`}
                      >
                        <div className="flex min-w-0 items-center gap-1.5">
                          <DocIcon
                            className={`h-3.5 w-3.5 shrink-0 ${
                              active ? "text-accent" : "text-ink-faint"
                            }`}
                          />
                          <span className="truncate">{meta.title || meta.slug}</span>
                        </div>
                        {meta.tags && meta.tags.length > 0 && (
                          <span className="ml-1 shrink-0 font-mono text-[9px] text-tag opacity-70 group-hover:opacity-100">
                            #{meta.tags[0]}
                          </span>
                        )}
                      </button>
                    );
                  })}

                  {metas.length === 0 && (
                    <p className="px-2 py-1 text-[11px] text-ink-faint italic">
                      {tagFilter || quickFilter ? "no matching memories" : "empty"}
                    </p>
                  )}
                </div>
              )}
            </section>
          );
        })}
      </div>

      {/* Tags Section */}
      {tags.length > 0 && (
        <div className="shrink-0 border-t border-line">
          <button
            type="button"
            onClick={() => setTagsOpen((v) => !v)}
            aria-expanded={tagsOpen}
            className="flex w-full items-center justify-between px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-ink-dim hover:bg-hover transition-colors"
          >
            <div className="flex items-center gap-1.5">
              <Chevron open={tagsOpen} className="h-3 w-3" />
              <TagIcon className="h-3 w-3 text-tag" />
              <span>Tags</span>
            </div>
            <span className="font-mono text-[10px] text-ink-faint">{tags.length}</span>
          </button>

          {tagsOpen && (
            <div className="max-h-40 overflow-y-auto px-2 py-1">
              <div className="flex flex-wrap gap-1">
                {tags.map(({ tag, count }) => {
                  const active = tagFilter === tag;
                  return (
                    <button
                      key={tag}
                      type="button"
                      onClick={() => onTagFilter(active ? null : tag)}
                      className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 font-mono text-[10.5px] transition-colors ${
                        active
                          ? "bg-tag/20 border border-tag text-tag font-semibold"
                          : "border border-line bg-raised text-ink-dim hover:border-tag/60 hover:text-tag"
                      }`}
                    >
                      <span>#{tag}</span>
                      <span className="text-[9px] text-ink-faint font-normal">{count}</span>
                    </button>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
