import { useEffect, useRef } from "react";

import { parseList, withEdits, type OpenTab } from "../lib/tabs";

type Props = {
  tab: OpenTab;
  saving: boolean;
  error: string | null;
  onChange: (next: OpenTab) => void;
  onSave: () => void;
  onDelete: () => void;
};

export default function Editor({ tab, saving, error, onChange, onSave, onDelete }: Props) {
  const titleRef = useRef<HTMLInputElement>(null);

  // A new memory needs a title before it can be saved — the backend derives the
  // slug from it — so that is where the cursor starts.
  useEffect(() => {
    if (tab.slug === null) titleRef.current?.focus();
  }, [tab.key, tab.slug]);

  const edit = (edits: Partial<OpenTab>) => onChange(withEdits(tab, edits));

  return (
    <div className="flex h-full min-h-0 flex-col bg-editor">
      <div className="shrink-0 border-b border-line px-4 py-3">
        <input
          ref={titleRef}
          value={tab.title}
          onChange={(e) => edit({ title: e.target.value })}
          placeholder="Title"
          aria-label="Title"
          className="w-full bg-transparent text-lg font-medium text-ink placeholder:text-ink-faint focus:outline-none"
        />

        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          <label className="flex items-center gap-2 text-ink-dim">
            <span className="w-10 shrink-0 font-mono text-[11px]">tags</span>
            <input
              value={tab.tags.join(", ")}
              onChange={(e) => edit({ tags: parseList(e.target.value) })}
              placeholder="decision, search"
              aria-label="Tags, comma separated"
              className="min-w-0 flex-1 rounded border border-line bg-editor px-2 py-0.5 font-mono text-[11px] text-tag focus:border-accent focus:outline-none"
            />
          </label>
          <label className="flex items-center gap-2 text-ink-dim">
            <span className="w-10 shrink-0 font-mono text-[11px]">links</span>
            <input
              value={tab.links.join(", ")}
              onChange={(e) => edit({ links: parseList(e.target.value) })}
              placeholder="other-memory-slug"
              aria-label="Links, comma separated"
              className="min-w-0 flex-1 rounded border border-line bg-editor px-2 py-0.5 font-mono text-[11px] text-ink focus:border-accent focus:outline-none"
            />
          </label>
        </div>

        <div className="mt-3 flex items-center gap-2">
          <button
            type="button"
            onClick={onSave}
            disabled={saving || !tab.dirty || tab.title.trim() === ""}
            title="Ctrl+S"
            className="rounded bg-accent px-3 py-0.5 font-medium text-accent-ink disabled:opacity-40"
          >
            {saving ? "Saving…" : "Save"}
          </button>

          {tab.slug && (
            <button
              type="button"
              onClick={onDelete}
              className="rounded border border-line px-2 py-0.5 text-ink-dim hover:border-danger hover:text-danger"
            >
              Delete
            </button>
          )}

          <span className="ml-auto font-mono text-[11px] text-ink-faint">
            {tab.slug ? `${tab.project}/${tab.slug}.md` : `${tab.project}/ — new`}
            {tab.updated && ` · updated ${new Date(tab.updated).toLocaleString()}`}
          </span>
        </div>

        {tab.title.trim() === "" && (
          <p className="mt-2 text-ink-faint">A title is needed — it also seeds the filename.</p>
        )}
        {error && <p className="mt-2 text-danger">{error}</p>}
      </div>

      {/* A plain textarea, not a code editor component. The files are Markdown
          the user can also open in their own editor, and syntax highlighting a
          memory is not worth the dependency. */}
      <textarea
        value={tab.body}
        onChange={(e) => edit({ body: e.target.value })}
        placeholder="The memory itself, as Markdown."
        aria-label="Memory body"
        spellCheck={false}
        className="min-h-0 flex-1 resize-none bg-editor px-4 py-3 font-mono text-[12.5px] leading-relaxed text-ink placeholder:text-ink-faint focus:outline-none"
      />
    </div>
  );
}
