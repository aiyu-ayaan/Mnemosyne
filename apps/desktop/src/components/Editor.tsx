import { useEffect, useRef, useState } from "react";

import { withEdits, type OpenTab } from "../lib/tabs";
import {
  BoldIcon,
  CheckIcon,
  ChecklistIcon,
  CodeBlockIcon,
  CopyIcon,
  EyeIcon,
  FolderIcon,
  ItalicIcon,
  LinkIcon,
  ListIcon,
  PencilIcon,
  QuoteIcon,
  SplitIcon,
  TagIcon,
  TrashIcon,
} from "./Icons";

type Props = {
  tab: OpenTab;
  saving: boolean;
  error: string | null;
  onChange: (next: OpenTab) => void;
  onSave: () => void;
  onDelete: () => void;
  onOpenWikilink?: (slug: string) => void;
};

type ViewMode = "edit" | "split" | "preview";

export default function Editor({
  tab,
  saving,
  error,
  onChange,
  onSave,
  onDelete,
  onOpenWikilink,
}: Props) {
  const titleRef = useRef<HTMLInputElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const [viewMode, setViewMode] = useState<ViewMode>("edit");
  const [newTagInput, setNewTagInput] = useState("");
  const [showTagInput, setShowTagInput] = useState(false);
  const [newLinkInput, setNewLinkInput] = useState("");
  const [showLinkInput, setShowLinkInput] = useState(false);
  const [copiedCode, setCopiedCode] = useState<string | null>(null);

  useEffect(() => {
    if (tab.slug === null) titleRef.current?.focus();
  }, [tab.key, tab.slug]);

  const edit = (edits: Partial<OpenTab>) => onChange(withEdits(tab, edits));

  // Toolbar action helpers
  const insertFormat = (before: string, after: string = "", defaultText: string = "") => {
    const textarea = textareaRef.current;
    if (!textarea) return;

    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const current = textarea.value;
    const selected = current.substring(start, end) || defaultText;

    const replacement = `${before}${selected}${after}`;
    const nextBody = current.substring(0, start) + replacement + current.substring(end);

    edit({ body: nextBody });

    setTimeout(() => {
      textarea.focus();
      textarea.setSelectionRange(start + before.length, start + before.length + selected.length);
    }, 10);
  };

  const handleAddTag = () => {
    const trimmed = newTagInput.trim().replace(/^#/, "");
    if (trimmed && !tab.tags.includes(trimmed)) {
      edit({ tags: [...tab.tags, trimmed] });
    }
    setNewTagInput("");
    setShowTagInput(false);
  };

  const handleRemoveTag = (tagToRemove: string) => {
    edit({ tags: tab.tags.filter((t) => t !== tagToRemove) });
  };

  const handleAddLink = () => {
    const trimmed = newLinkInput.trim().replace(/^\[\[/, "").replace(/\]\]$/, "");
    if (trimmed && !tab.links.includes(trimmed)) {
      edit({ links: [...tab.links, trimmed] });
    }
    setNewLinkInput("");
    setShowLinkInput(false);
  };

  const handleRemoveLink = (linkToRemove: string) => {
    edit({ links: tab.links.filter((l) => l !== linkToRemove) });
  };

  // Stats calculation
  const wordCount = tab.body.trim() ? tab.body.trim().split(/\s+/).length : 0;
  const charCount = tab.body.length;
  const estTokens = Math.ceil(charCount / 4);

  // Simple, safe Markdown parser for live preview
  const renderMarkdown = (content: string) => {
    const lines = content.split("\n");
    const elements: React.ReactNode[] = [];
    let inCodeBlock = false;
    let codeBlockBuffer: string[] = [];
    let codeBlockLang = "";

    lines.forEach((line, idx) => {
      // Fenced Code Block
      if (line.startsWith("```")) {
        if (!inCodeBlock) {
          inCodeBlock = true;
          codeBlockLang = line.slice(3).trim();
          codeBlockBuffer = [];
        } else {
          inCodeBlock = false;
          const codeText = codeBlockBuffer.join("\n");
          elements.push(
            <div key={`codeblock-${idx}`} className="group relative my-3">
              <div className="flex items-center justify-between rounded-t bg-raised px-3 py-1 text-[11px] font-mono text-ink-faint border border-b-0 border-line">
                <span>{codeBlockLang || "code"}</span>
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard.writeText(codeText);
                    setCopiedCode(codeText);
                    setTimeout(() => setCopiedCode(null), 1500);
                  }}
                  className="flex items-center gap-1 hover:text-ink transition-colors"
                >
                  {copiedCode === codeText ? (
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
              <pre className="m-0 rounded-b border border-line bg-shell p-3 overflow-x-auto font-mono text-xs text-ink leading-relaxed">
                <code>{codeText}</code>
              </pre>
            </div>,
          );
        }
        return;
      }

      if (inCodeBlock) {
        codeBlockBuffer.push(line);
        return;
      }

      // Headings
      if (line.startsWith("# ")) {
        elements.push(
          <h1 key={idx} className="mt-4 mb-2 text-xl font-bold text-ink border-b border-line pb-1">
            {formatInline(line.slice(2))}
          </h1>,
        );
        return;
      }
      if (line.startsWith("## ")) {
        elements.push(
          <h2 key={idx} className="mt-3 mb-1.5 text-lg font-semibold text-ink border-b border-line pb-0.5">
            {formatInline(line.slice(3))}
          </h2>,
        );
        return;
      }
      if (line.startsWith("### ")) {
        elements.push(
          <h3 key={idx} className="mt-2.5 mb-1 text-base font-semibold text-ink">
            {formatInline(line.slice(4))}
          </h3>,
        );
        return;
      }

      // Blockquotes
      if (line.startsWith("> ")) {
        elements.push(
          <blockquote key={idx} className="my-1.5 border-l-2 border-accent pl-3 text-ink-dim italic">
            {formatInline(line.slice(2))}
          </blockquote>,
        );
        return;
      }

      // Checkboxes
      if (line.match(/^[-*]\s+\[([ xX])\]/)) {
        const checked = line.includes("[x]") || line.includes("[X]");
        const text = line.replace(/^[-*]\s+\[([ xX])\]\s*/, "");
        elements.push(
          <div key={idx} className="flex items-center gap-2 my-0.5 pl-1">
            <input
              type="checkbox"
              checked={checked}
              readOnly
              className="rounded border-line bg-editor text-accent focus:ring-0"
            />
            <span className={checked ? "line-through text-ink-faint" : "text-ink"}>
              {formatInline(text)}
            </span>
          </div>,
        );
        return;
      }

      // Bullet lists
      if (line.startsWith("- ") || line.startsWith("* ")) {
        elements.push(
          <li key={idx} className="ml-5 my-0.5 list-disc text-ink">
            {formatInline(line.slice(2))}
          </li>,
        );
        return;
      }

      // Horizontal Rule
      if (line === "---" || line === "***") {
        elements.push(<hr key={idx} className="my-4 border-line" />);
        return;
      }

      // Empty line
      if (!line.trim()) {
        elements.push(<div key={idx} className="h-2" />);
        return;
      }

      // Normal paragraph
      elements.push(
        <p key={idx} className="my-1 text-ink leading-relaxed">
          {formatInline(line)}
        </p>,
      );
    });

    return elements;
  };

  // Inline formatting: bold, italic, code, and [[wikilinks]]
  const formatInline = (text: string): React.ReactNode => {
    // Match [[target]] or [[target|alias]]
    const parts = text.split(/(\[\[[^\]]+\]\]|`[^`]+`|\*\*[^*]+\*\*|\*[^*]+\*)/g);

    return parts.map((part, i) => {
      if (part.startsWith("[[") && part.endsWith("]]")) {
        const raw = part.slice(2, -2);
        const [target, alias] = raw.split("|");
        const display = alias || target;
        return (
          <button
            key={i}
            type="button"
            onClick={() => onOpenWikilink && target && onOpenWikilink(target.trim())}
            title={`Open [[${target}]]`}
            className="inline-flex items-center gap-0.5 rounded bg-selected/60 px-1.5 py-0.2 font-mono text-[11px] font-medium text-accent hover:bg-selected hover:underline"
          >
            <span>[[</span>
            <span>{display}</span>
            <span>]]</span>
          </button>
        );
      }
      if (part.startsWith("`") && part.endsWith("`")) {
        return (
          <code
            key={i}
            className="rounded border border-line bg-raised px-1 py-0.2 font-mono text-[11px] text-code"
          >
            {part.slice(1, -1)}
          </code>
        );
      }
      if (part.startsWith("**") && part.endsWith("**")) {
        return (
          <strong key={i} className="font-semibold text-ink">
            {part.slice(2, -2)}
          </strong>
        );
      }
      if (part.startsWith("*") && part.endsWith("*")) {
        return (
          <em key={i} className="italic text-ink">
            {part.slice(1, -1)}
          </em>
        );
      }
      return part;
    });
  };

  return (
    <div className="flex h-full min-h-0 flex-col bg-editor">
      {/* Top Header Card */}
      <div className="shrink-0 border-b border-line bg-shell/50 px-4 py-2.5">
        {/* Project Breadcrumb & Save Status */}
        <div className="flex items-center justify-between gap-2 mb-1.5">
          <div className="flex items-center gap-1.5 font-mono text-[11px] text-ink-faint">
            <FolderIcon className="h-3.5 w-3.5 text-accent" />
            <span className="text-ink-dim font-medium">{tab.project}</span>
            <span>/</span>
            <span className="text-ink">{tab.slug ? `${tab.slug}.md` : "draft.md"}</span>
          </div>

          <div className="flex items-center gap-2 text-[11px]">
            {tab.dirty ? (
              <span className="flex items-center gap-1 text-warn font-mono">
                <span className="inline-block h-1.5 w-1.5 rounded-full bg-warn" />
                Unsaved changes
              </span>
            ) : (
              <span className="flex items-center gap-1 text-tag font-mono">
                <span className="inline-block h-1.5 w-1.5 rounded-full bg-tag" />
                Saved
              </span>
            )}
            {tab.updated && (
              <span className="hidden sm:inline font-mono text-[10px] text-ink-faint">
                {new Date(tab.updated).toLocaleTimeString()}
              </span>
            )}
          </div>
        </div>

        {/* Title Input */}
        <input
          ref={titleRef}
          value={tab.title}
          onChange={(e) => edit({ title: e.target.value })}
          placeholder="Memory title…"
          aria-label="Title"
          className="w-full bg-transparent text-lg font-semibold text-ink placeholder:text-ink-faint focus:outline-none"
        />

        {/* Metadata: Tags & Links Pills */}
        <div className="mt-2.5 flex flex-wrap items-center gap-x-4 gap-y-2 text-[11px]">
          {/* Tag Chips */}
          <div className="flex flex-wrap items-center gap-1.5">
            <TagIcon className="h-3 w-3 text-tag shrink-0" />
            {tab.tags.map((tag) => (
              <span
                key={tag}
                className="group inline-flex items-center gap-1 rounded bg-raised border border-line px-1.5 py-0.5 font-mono text-[10.5px] text-tag"
              >
                <span>#{tag}</span>
                <button
                  type="button"
                  onClick={() => handleRemoveTag(tag)}
                  title={`Remove #${tag}`}
                  className="opacity-50 hover:opacity-100"
                >
                  ✕
                </button>
              </span>
            ))}
            {showTagInput ? (
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  handleAddTag();
                }}
                className="inline-flex items-center"
              >
                <input
                  type="text"
                  value={newTagInput}
                  onChange={(e) => setNewTagInput(e.target.value)}
                  onBlur={handleAddTag}
                  placeholder="tag name..."
                  autoFocus
                  className="w-20 rounded border border-accent bg-editor px-1 py-0.2 font-mono text-[10.5px] text-tag outline-none"
                />
              </form>
            ) : (
              <button
                type="button"
                onClick={() => setShowTagInput(true)}
                className="rounded border border-dashed border-line px-1.5 py-0.5 font-mono text-[10.5px] text-ink-faint hover:border-tag hover:text-tag transition-colors"
              >
                + tag
              </button>
            )}
          </div>

          {/* Links Pills */}
          <div className="flex flex-wrap items-center gap-1.5">
            <LinkIcon className="h-3 w-3 text-accent shrink-0" />
            {tab.links.map((link) => (
              <span
                key={link}
                className="group inline-flex items-center gap-1 rounded bg-selected/40 border border-accent/40 px-1.5 py-0.5 font-mono text-[10.5px] text-accent"
              >
                <button
                  type="button"
                  onClick={() => onOpenWikilink && onOpenWikilink(link)}
                  className="hover:underline"
                >
                  [[{link}]]
                </button>
                <button
                  type="button"
                  onClick={() => handleRemoveLink(link)}
                  title={`Remove link to ${link}`}
                  className="opacity-50 hover:opacity-100"
                >
                  ✕
                </button>
              </span>
            ))}
            {showLinkInput ? (
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  handleAddLink();
                }}
                className="inline-flex items-center"
              >
                <input
                  type="text"
                  value={newLinkInput}
                  onChange={(e) => setNewLinkInput(e.target.value)}
                  onBlur={handleAddLink}
                  placeholder="linked-slug..."
                  autoFocus
                  className="w-24 rounded border border-accent bg-editor px-1 py-0.2 font-mono text-[10.5px] text-accent outline-none"
                />
              </form>
            ) : (
              <button
                type="button"
                onClick={() => setShowLinkInput(true)}
                className="rounded border border-dashed border-line px-1.5 py-0.5 font-mono text-[10.5px] text-ink-faint hover:border-accent hover:text-accent transition-colors"
              >
                + link
              </button>
            )}
          </div>
        </div>

        {/* Primary Action Buttons */}
        <div className="mt-2.5 flex items-center justify-between border-t border-line/60 pt-2">
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={onSave}
              disabled={saving || !tab.dirty || tab.title.trim() === ""}
              title="Save memory (Ctrl+S)"
              className="flex items-center gap-1.5 rounded bg-accent px-3 py-1 text-xs font-semibold text-accent-ink hover:opacity-95 transition-opacity disabled:opacity-40"
            >
              <CheckIcon className="h-3.5 w-3.5" />
              <span>{saving ? "Saving…" : "Save"}</span>
            </button>

            {tab.slug && (
              <button
                type="button"
                onClick={onDelete}
                title="Delete memory"
                className="flex items-center gap-1 rounded border border-line px-2 py-1 text-xs text-ink-dim hover:border-danger hover:text-danger transition-colors"
              >
                <TrashIcon className="h-3 w-3" />
                <span>Delete</span>
              </button>
            )}
          </div>

          {/* View Mode Toggle Switcher */}
          <div className="flex rounded border border-line bg-raised p-0.5 text-[11px]">
            <button
              type="button"
              title="Edit Mode"
              onClick={() => setViewMode("edit")}
              className={`flex items-center gap-1 rounded px-2 py-0.5 transition-colors ${
                viewMode === "edit" ? "bg-hover text-ink font-semibold" : "text-ink-faint hover:text-ink"
              }`}
            >
              <PencilIcon className="h-3 w-3" />
              <span className="hidden sm:inline">Edit</span>
            </button>
            <button
              type="button"
              title="Split View"
              onClick={() => setViewMode("split")}
              className={`flex items-center gap-1 rounded px-2 py-0.5 transition-colors ${
                viewMode === "split" ? "bg-hover text-ink font-semibold" : "text-ink-faint hover:text-ink"
              }`}
            >
              <SplitIcon className="h-3 w-3" />
              <span className="hidden sm:inline">Split</span>
            </button>
            <button
              type="button"
              title="Preview Mode"
              onClick={() => setViewMode("preview")}
              className={`flex items-center gap-1 rounded px-2 py-0.5 transition-colors ${
                viewMode === "preview" ? "bg-hover text-ink font-semibold" : "text-ink-faint hover:text-ink"
              }`}
            >
              <EyeIcon className="h-3 w-3" />
              <span className="hidden sm:inline">Preview</span>
            </button>
          </div>
        </div>

        {tab.title.trim() === "" && (
          <p className="mt-1.5 text-xs text-warn">A title is required to save and create a slug.</p>
        )}
        {error && <p className="mt-1.5 text-xs text-danger">{error}</p>}
      </div>

      {/* Formatting Toolbar (Visible in Edit & Split mode) */}
      {viewMode !== "preview" && (
        <div className="flex items-center gap-0.5 border-b border-line bg-shell/70 px-3 py-1 text-ink-dim">
          <button
            type="button"
            title="Bold (**text**)"
            onClick={() => insertFormat("**", "**", "bold")}
            className="rounded p-1 hover:bg-hover hover:text-ink transition-colors"
          >
            <BoldIcon />
          </button>
          <button
            type="button"
            title="Italic (*text*)"
            onClick={() => insertFormat("*", "*", "italic")}
            className="rounded p-1 hover:bg-hover hover:text-ink transition-colors"
          >
            <ItalicIcon />
          </button>
          <button
            type="button"
            title="Inline Code (`code`)"
            onClick={() => insertFormat("`", "`", "code")}
            className="rounded p-1 font-mono text-[11px] font-semibold hover:bg-hover hover:text-ink transition-colors"
          >
            {"` `"}
          </button>
          <button
            type="button"
            title="Code Block"
            onClick={() => insertFormat("```\n", "\n```", "code block")}
            className="rounded p-1 hover:bg-hover hover:text-ink transition-colors"
          >
            <CodeBlockIcon />
          </button>

          <span className="mx-1 h-3.5 w-px bg-line" />

          <button
            type="button"
            title="Bullet List (- item)"
            onClick={() => insertFormat("- ", "", "item")}
            className="rounded p-1 hover:bg-hover hover:text-ink transition-colors"
          >
            <ListIcon />
          </button>
          <button
            type="button"
            title="Checklist (- [ ] task)"
            onClick={() => insertFormat("- [ ] ", "", "task")}
            className="rounded p-1 hover:bg-hover hover:text-ink transition-colors"
          >
            <ChecklistIcon />
          </button>
          <button
            type="button"
            title="Blockquote (> quote)"
            onClick={() => insertFormat("> ", "", "quote")}
            className="rounded p-1 hover:bg-hover hover:text-ink transition-colors"
          >
            <QuoteIcon />
          </button>
          <button
            type="button"
            title="Wikilink ([[target]])"
            onClick={() => insertFormat("[[", "]]", "target")}
            className="rounded p-1 hover:bg-hover hover:text-ink transition-colors"
          >
            <LinkIcon />
          </button>

          {/* Document Counter Badges in toolbar */}
          <div className="ml-auto flex items-center gap-3 font-mono text-[10px] text-ink-faint">
            <span>{wordCount} words</span>
            <span>·</span>
            <span>{charCount} chars</span>
            <span>·</span>
            <span title="Estimated tokens (char / 4)">~{estTokens} tokens</span>
          </div>
        </div>
      )}

      {/* Editor & Preview Body Area */}
      <div className="flex min-h-0 flex-1">
        {/* Editor Textarea Pane */}
        {viewMode !== "preview" && (
          <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
            <textarea
              ref={textareaRef}
              value={tab.body}
              onChange={(e) => edit({ body: e.target.value })}
              placeholder="Write your project memory in Markdown... Use [[wikilinks]] to link memories and #tags to organize."
              aria-label="Memory body"
              spellCheck={false}
              className="h-full w-full resize-none bg-editor px-4 py-3 font-mono text-[13px] leading-relaxed text-ink placeholder:text-ink-faint focus:outline-none"
            />
          </div>
        )}

        {/* Vertical Divider in Split Mode */}
        {viewMode === "split" && <div className="w-px bg-line shrink-0" />}

        {/* Live Preview Pane */}
        {viewMode !== "edit" && (
          <div className="flex min-h-0 flex-1 flex-col overflow-y-auto bg-shell/30 px-6 py-4">
            <div className="markdown-body max-w-3xl">
              {tab.body.trim() ? (
                renderMarkdown(tab.body)
              ) : (
                <p className="text-ink-faint italic">Preview will appear here as you type.</p>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
