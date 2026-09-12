import { useState } from "react";

import type { Library } from "../lib/useLibrary";
import type { Meta } from "../lib/types";
import { Chevron, DocIcon, PlusIcon } from "./Icons";

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

  // Everything starts expanded: a fresh root has one project, and hiding its
  // contents behind a click would make the window look empty on first run.
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [tagsOpen, setTagsOpen] = useState(true);

  const toggle = (slug: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (!next.delete(slug)) next.add(slug);
      return next;
    });

  const visible = (metas: Meta[]) =>
    tagFilter ? metas.filter((meta) => (meta.tags ?? []).includes(tagFilter)) : metas;

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <header className="flex items-center justify-between px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-ink-dim">
        <span>Mnemosyne</span>
        <button
          type="button"
          title="New project"
          aria-label="New project"
          onClick={onNewProject}
          className="rounded p-0.5 text-ink-dim hover:bg-hover hover:text-ink"
        >
          <PlusIcon />
        </button>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {projects.length === 0 && !library.loading && (
          <p className="px-3 py-2 text-ink-faint">
            No projects yet. An agent creates one on its first write, or add one above.
          </p>
        )}

        {projects.map((project) => {
          const open = !collapsed.has(project.slug);
          const metas = visible(memories[project.slug] ?? []);

          return (
            <section key={project.slug}>
              <div className="group flex items-center gap-1 pr-1 hover:bg-hover">
                <button
                  type="button"
                  onClick={() => toggle(project.slug)}
                  aria-expanded={open}
                  className="flex min-w-0 flex-1 items-center gap-1 py-[3px] pl-2 text-left"
                >
                  <Chevron open={open} />
                  <span className="truncate">{project.name}</span>
                  <span className="ml-auto pl-2 text-ink-faint">{project.memoryCount}</span>
                </button>
                <button
                  type="button"
                  title={`New memory in ${project.slug}`}
                  aria-label={`New memory in ${project.slug}`}
                  onClick={() => onNewMemory(project.slug)}
                  className="hidden rounded p-0.5 text-ink-dim hover:text-ink group-hover:block"
                >
                  <PlusIcon />
                </button>
                <button
                  type="button"
                  title={`Delete ${project.slug} and every memory in it`}
                  aria-label={`Delete project ${project.slug}`}
                  onClick={() => onDeleteProject(project.slug)}
                  className="hidden px-1 text-ink-dim hover:text-danger group-hover:block"
                >
                  ×
                </button>
              </div>

              {open &&
                metas.map((meta) => {
                  const key = `${meta.project}/${meta.slug}`;
                  return (
                    <button
                      key={key}
                      type="button"
                      onClick={() => onOpen(meta)}
                      title={meta.title}
                      className={`flex w-full items-center gap-1.5 py-[3px] pl-7 pr-2 text-left ${
                        activeKey === key ? "bg-selected text-ink" : "hover:bg-hover"
                      }`}
                    >
                      <DocIcon className="h-3.5 w-3.5 shrink-0 text-ink-faint" />
                      <span className="truncate">{meta.title || meta.slug}</span>
                    </button>
                  );
                })}

              {open && metas.length === 0 && (
                <p className="py-[3px] pl-7 text-ink-faint">
                  {tagFilter ? `nothing tagged #${tagFilter}` : "empty"}
                </p>
              )}
            </section>
          );
        })}
      </div>

      {tags.length > 0 && (
        <div className="shrink-0 border-t border-line">
          <button
            type="button"
            onClick={() => setTagsOpen((v) => !v)}
            aria-expanded={tagsOpen}
            className="flex w-full items-center gap-1 px-2 py-2 text-[11px] font-semibold uppercase tracking-wide text-ink-dim hover:bg-hover"
          >
            <Chevron open={tagsOpen} />
            Tags
            {tagFilter && <span className="ml-auto normal-case text-accent">#{tagFilter}</span>}
          </button>

          {tagsOpen && (
            <div className="max-h-48 overflow-y-auto pb-1">
              {tags.map(({ tag, count }) => (
                <button
                  key={tag}
                  type="button"
                  // Clicking the active tag clears the filter, so there is no
                  // separate "clear" control to find.
                  onClick={() => onTagFilter(tagFilter === tag ? null : tag)}
                  className={`flex w-full items-center py-[3px] pl-7 pr-3 text-left ${
                    tagFilter === tag ? "bg-selected" : "hover:bg-hover"
                  }`}
                >
                  <span className="truncate text-tag">#{tag}</span>
                  <span className="ml-auto pl-2 text-ink-faint">{count}</span>
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
