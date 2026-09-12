import { useState } from "react";
import { THEMES, type ThemeId, type ThemeInfo } from "../lib/types";
import { CheckIcon, PaletteIcon, SunIcon } from "./Icons";

type Props = {
  theme: ThemeId;
  onTheme: (theme: ThemeId) => void;
};

const THEME_DESCRIPTIONS: Record<ThemeId, string> = {
  dark: "Classic Visual Studio Code dark gray theme with familiar blue accents.",
  midnight: "Obsidian-inspired deep void black with electric violet highlights for deep focus.",
  tokyo: "Celebrated neon nightlife palette with cold blues and cyber purple hues.",
  nord: "Arctic-inspired calm Scandinavian palette with muted blues, frost, and slate.",
  catppuccin: "Warm, soothing pastel palette with lavender and mint for relaxed coding.",
  monokai: "Classic hacker aesthetic with vibrant lime green, cyber cyan, and charcoal gray.",
  light: "Crisp white paper aesthetic with high-contrast slate text and sky blue accents.",
};

export default function ThemeView({ theme, onTheme }: Props) {
  const [previewTheme, setPreviewTheme] = useState<ThemeId>(theme);
  const activeThemeObj = THEMES.find((t) => t.id === theme) || THEMES[0]!;

  return (
    <div className="flex h-full flex-col gap-6 overflow-y-auto p-6 max-w-5xl mx-auto w-full">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 border-b border-line pb-4">
        <div>
          <div className="flex items-center gap-2">
            <PaletteIcon className="h-5 w-5 text-accent" />
            <h2 className="text-xl font-semibold text-ink">Appearance & Theme Studio</h2>
          </div>
          <p className="mt-1 text-xs text-ink-dim">
            Curated developer color themes tailored for markdown knowledge work, memory graphing, and prolonged coding sessions.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="flex items-center gap-1.5 rounded-full border border-accent/40 bg-accent/10 px-3 py-1 text-xs font-medium text-accent">
            <span className="h-2 w-2 rounded-full bg-accent animate-pulse" />
            Active: {activeThemeObj.name}
          </span>
          {theme !== "dark" && (
            <button
              type="button"
              onClick={() => onTheme("dark")}
              className="rounded-md border border-line bg-shell px-2.5 py-1 text-xs text-ink-dim hover:text-ink hover:bg-hover transition-colors"
            >
              Reset to VS Code Dark
            </button>
          )}
        </div>
      </div>

      {/* Theme Cards Grid */}
      <div>
        <h3 className="text-xs font-semibold uppercase tracking-wider text-ink-faint mb-3">
          Available Color Schemes ({THEMES.length})
        </h3>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5">
          {THEMES.map((item: ThemeInfo) => {
            const isSelected = theme === item.id;
            const isPreviewing = previewTheme === item.id;

            return (
              <div
                key={item.id}
                role="button"
                tabIndex={0}
                onClick={() => {
                  onTheme(item.id);
                  setPreviewTheme(item.id);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    onTheme(item.id);
                    setPreviewTheme(item.id);
                  }
                }}
                onMouseEnter={() => setPreviewTheme(item.id)}
                className={`group relative flex flex-col justify-between rounded-xl border p-4 text-left transition-all cursor-pointer shadow-xs ${
                  isSelected
                    ? "border-accent bg-selected/20 ring-1 ring-accent/40 shadow-sm"
                    : isPreviewing
                      ? "border-ink-faint bg-raised/70"
                      : "border-line bg-raised/40 hover:border-line/80 hover:bg-raised/60"
                }`}
              >
                <div>
                  {/* Card Top: Title, Badge, and Selected Pill */}
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <div className="flex items-center gap-1.5">
                        <span className="font-semibold text-sm text-ink">{item.name}</span>
                        {item.category === "light" ? (
                          <SunIcon className="h-3.5 w-3.5 text-amber-500" />
                        ) : null}
                      </div>
                      <span className="font-mono text-[10.5px] text-ink-faint capitalize">
                        {item.category} mode
                      </span>
                    </div>

                    {isSelected ? (
                      <span className="flex items-center gap-1 rounded-full bg-accent/20 px-2 py-0.5 text-[10.5px] font-semibold text-accent">
                        <CheckIcon className="h-3 w-3" />
                        Active
                      </span>
                    ) : (
                      <span className="rounded px-2 py-0.5 text-[10.5px] text-ink-faint opacity-0 group-hover:opacity-100 transition-opacity">
                        Click to apply
                      </span>
                    )}
                  </div>

                  {/* Description */}
                  <p className="mt-2 text-xs text-ink-dim leading-relaxed line-clamp-2">
                    {THEME_DESCRIPTIONS[item.id] || "Tailored developer color scheme."}
                  </p>
                </div>

                {/* Color Palette Preview Swatches */}
                <div className="mt-4 pt-3 border-t border-line/60 flex items-center justify-between">
                  <div className="flex items-center gap-1.5">
                    <div
                      className="h-5 w-5 rounded-md border border-black/20 shadow-xs"
                      style={{ backgroundColor: item.primaryBg }}
                      title={`Shell: ${item.primaryBg}`}
                    />
                    <div
                      className="h-5 w-5 rounded-md border border-black/20 shadow-xs"
                      style={{ backgroundColor: item.editorBg }}
                      title={`Editor: ${item.editorBg}`}
                    />
                    <div
                      className="h-5 w-5 rounded-md border border-black/20 shadow-xs"
                      style={{ backgroundColor: item.accent }}
                      title={`Accent: ${item.accent}`}
                    />
                    <div
                      className="h-5 w-5 rounded-md border border-black/20 shadow-xs"
                      style={{ backgroundColor: item.tag }}
                      title={`Tag: ${item.tag}`}
                    />
                  </div>
                  <span className="font-mono text-[10px] text-ink-faint">
                    {item.accent}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Live Sample Note Preview Card */}
      <section className="rounded-xl border border-line bg-shell p-5">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <h3 className="font-semibold text-xs text-ink uppercase tracking-wider">
              Live Preview in Current Theme ({activeThemeObj.name})
            </h3>
          </div>
          <span className="font-mono text-[11px] text-ink-faint">
            Real-time CSS variable evaluation
          </span>
        </div>

        <div className="rounded-lg border border-line bg-editor p-4 transition-colors">
          <div className="flex items-center justify-between border-b border-line pb-2 mb-3">
            <div className="flex items-center gap-2">
              <span className="h-2 w-2 rounded-full bg-accent" />
              <span className="font-mono text-xs font-medium text-ink">project-architecture.md</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="rounded bg-accent/15 px-2 py-0.5 font-mono text-[10px] font-semibold text-accent">
                #backend
              </span>
              <span className="rounded bg-tag/15 px-2 py-0.5 font-mono text-[10px] font-semibold text-tag">
                #mcp
              </span>
            </div>
          </div>

          <h1 className="text-base font-bold text-ink mb-1.5">
            Mnemosyne Memory Architecture & Knowledge Graph
          </h1>
          <p className="text-xs text-ink-dim leading-relaxed mb-3">
            Mnemosyne coordinates persistent project memories across multiple AI tools including{" "}
            <span className="text-accent font-medium">Google Antigravity</span>,{" "}
            <span className="text-accent font-medium">Claude Code</span>, and{" "}
            <span className="text-accent font-medium">OpenAI Codex</span>. It stores Markdown files with{" "}
            <span className="underline decoration-accent/60 underline-offset-2">[[backlinks]]</span> and SQLite-vec embeddings.
          </p>

          <pre className="rounded bg-shell p-2.5 font-mono text-[11px] text-ink leading-normal border border-line">
            <code>
              <span className="text-tag font-semibold">func</span> <span className="text-accent font-semibold">ServeLocalChannel</span>() error &#123;<br />
              &nbsp;&nbsp;pipe := channel.Resolve(&quot;mnemosyne.ROOT&quot;)<br />
              &nbsp;&nbsp;<span className="text-tag font-semibold">return</span> daemon.Run(pipe)<br />
              &#125;
            </code>
          </pre>
        </div>
      </section>
    </div>
  );
}
