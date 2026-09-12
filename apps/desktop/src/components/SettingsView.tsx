import { useEffect, useState } from "react";

import { api, bridge } from "../lib/bridge";
import { THEMES, type Health, type Settings, type ThemeId } from "../lib/types";
import { CheckIcon, FolderIcon, PaletteIcon } from "./Icons";

type Props = {
  health: Health | null;
  theme: ThemeId;
  onTheme: (theme: ThemeId) => void;
  onChanged: () => void;
};

export default function SettingsView({ health, theme, onTheme, onChanged }: Props) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [draft, setDraft] = useState("");
  const [status, setStatus] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api<Settings>("GET", "/v1/settings")
      .then((s) => {
        setSettings(s);
        setDraft(s.root);
      })
      .catch((err) => setStatus(err instanceof Error ? err.message : String(err)));
  }, []);

  const save = async (root: string) => {
    setBusy(true);
    setStatus(null);
    try {
      const saved = await api<Settings>("PUT", "/v1/settings", { root });
      setSettings(saved);
      setDraft(saved.root);
      setStatus(`Now serving ${saved.root}`);
      onChanged();
    } catch (err) {
      setStatus(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const browse = async () => {
    const picked = await bridge.pickDirectory();
    if (picked) await save(picked);
  };

  return (
    <div className="flex h-full flex-col gap-5 overflow-y-auto p-3 text-ink-dim select-none">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-line pb-2">
        <h2 className="text-[11px] font-semibold uppercase tracking-wider text-ink">Settings</h2>
      </div>

      {/* Theme Section */}
      <section>
        <div className="flex items-center gap-1.5 mb-1.5">
          <PaletteIcon className="h-4 w-4 text-accent" />
          <h3 className="font-semibold text-xs text-ink">Appearance & Theme</h3>
        </div>
        <p className="text-[11px] text-ink-faint mb-3">
          Select a developer theme tailored for long coding sessions and knowledge work.
        </p>

        <div className="grid grid-cols-1 gap-2">
          {THEMES.map((item) => {
            const isSelected = theme === item.id;
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => onTheme(item.id)}
                className={`group flex items-center justify-between rounded-md border p-2 text-left transition-all ${
                  isSelected
                    ? "border-accent bg-selected/20 shadow-xs"
                    : "border-line bg-raised/60 hover:border-ink-faint hover:bg-raised"
                }`}
              >
                <div className="flex items-center gap-2.5">
                  {/* Theme Color Palette Preview Swatch */}
                  <div className="flex items-center -space-x-1 rounded border border-line p-0.5 bg-shell shrink-0">
                    <span
                      className="h-4 w-4 rounded-full border border-black/20"
                      style={{ backgroundColor: item.primaryBg }}
                      title="Shell"
                    />
                    <span
                      className="h-4 w-4 rounded-full border border-black/20"
                      style={{ backgroundColor: item.editorBg }}
                      title="Editor"
                    />
                    <span
                      className="h-4 w-4 rounded-full border border-black/20"
                      style={{ backgroundColor: item.accent }}
                      title="Accent"
                    />
                    <span
                      className="h-4 w-4 rounded-full border border-black/20"
                      style={{ backgroundColor: item.tag }}
                      title="Tag"
                    />
                  </div>

                  <div>
                    <div className="text-xs font-medium text-ink">{item.name}</div>
                    <div className="font-mono text-[10px] text-ink-faint capitalize">
                      {item.category} mode
                    </div>
                  </div>
                </div>

                {isSelected && (
                  <span className="flex items-center gap-1 rounded bg-accent/20 px-1.5 py-0.5 text-[10px] font-semibold text-accent">
                    <CheckIcon className="h-3 w-3" />
                    Active
                  </span>
                )}
              </button>
            );
          })}
        </div>
      </section>

      {/* Memory Root Storage Section */}
      <section className="border-t border-line pt-4">
        <div className="flex items-center gap-1.5 mb-1.5">
          <FolderIcon className="h-4 w-4 text-accent" />
          <h3 className="font-semibold text-xs text-ink">Memory Storage Root</h3>
        </div>
        <p className="text-[11px] text-ink-faint mb-2.5">
          The directory containing your Markdown memories organized by project. Fully portable and plain text.
        </p>

        <form
          className="flex flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (draft.trim()) save(draft.trim());
          }}
        >
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            spellCheck={false}
            aria-label="Memory root path"
            className="w-full rounded border border-line bg-editor px-2.5 py-1.5 font-mono text-[11px] text-ink focus:border-accent outline-none"
          />
          <div className="flex flex-wrap gap-1.5">
            <button
              type="button"
              onClick={browse}
              disabled={busy}
              className="rounded border border-line bg-raised px-2.5 py-1 text-xs text-ink hover:bg-hover transition-colors disabled:opacity-40"
            >
              Browse…
            </button>
            <button
              type="submit"
              disabled={busy || !draft.trim() || draft.trim() === settings?.root}
              className="rounded bg-accent px-3 py-1 text-xs font-semibold text-accent-ink hover:opacity-95 transition-opacity disabled:opacity-40"
            >
              {busy ? "Applying…" : "Apply"}
            </button>
            {settings && (
              <button
                type="button"
                onClick={() => bridge.reveal(settings.root)}
                className="rounded border border-line px-2.5 py-1 text-xs text-ink-dim hover:bg-hover hover:text-ink transition-colors"
              >
                Reveal
              </button>
            )}
          </div>
        </form>

        {settings?.defaultRoot && settings.defaultRoot !== settings.root && (
          <button
            type="button"
            onClick={() => save(settings.defaultRoot!)}
            className="mt-2 text-[11px] text-accent hover:underline"
          >
            Reset to default location
          </button>
        )}

        {status && <p className="mt-2 text-[11px] text-tag">{status}</p>}
      </section>

      {/* Installation Diagnostics Card */}
      <section className="border-t border-line pt-4">
        <h3 className="font-semibold text-xs text-ink mb-2">System Diagnostics</h3>
        <div className="grid grid-cols-2 gap-2 text-[11px]">
          <div className="rounded border border-line bg-raised/50 p-2">
            <span className="text-ink-faint block font-mono text-[10px]">Version</span>
            <span className="font-semibold text-ink">{health?.version ?? "0.1.0"}</span>
          </div>
          <div className="rounded border border-line bg-raised/50 p-2">
            <span className="text-ink-faint block font-mono text-[10px]">Mode</span>
            <span className="font-semibold text-ink">{health?.portable ? "Portable" : "Installed"}</span>
          </div>
          <div className="rounded border border-line bg-raised/50 p-2">
            <span className="text-ink-faint block font-mono text-[10px]">Memories</span>
            <span className="font-semibold text-ink font-mono">{health?.memories ?? 0}</span>
          </div>
          <div className="rounded border border-line bg-raised/50 p-2">
            <span className="text-ink-faint block font-mono text-[10px]">Index Size</span>
            <span className="font-semibold text-ink font-mono">
              {health ? `${Math.round(health.indexBytes / 1024)} KB` : "—"}
            </span>
          </div>
        </div>
      </section>
    </div>
  );
}
