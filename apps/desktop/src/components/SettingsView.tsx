import { useEffect, useState } from "react";

import { api, bridge } from "../lib/bridge";
import type { Health, Settings } from "../lib/types";

type Props = {
  health: Health | null;
  theme: "dark" | "light";
  onTheme: (theme: "dark" | "light") => void;
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
    <div className="flex h-full flex-col gap-4 overflow-y-auto p-2">
      <h2 className="px-1 text-[11px] font-semibold uppercase tracking-wide text-ink-dim">
        Settings
      </h2>

      <section className="px-1">
        <h3 className="mb-1 font-medium text-ink">Memory root</h3>
        <p className="mb-2 text-ink-faint">
          The folder holding your memories, one directory per project. Plain Markdown — back it up
          by copying it.
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
            className="w-full rounded border border-line bg-editor px-2 py-1 font-mono text-[11px] text-ink focus:border-accent focus:outline-none"
          />
          <div className="flex gap-2">
            <button
              type="button"
              onClick={browse}
              disabled={busy}
              className="rounded border border-line px-2 py-0.5 text-ink-dim hover:bg-hover hover:text-ink disabled:opacity-40"
            >
              Browse…
            </button>
            <button
              type="submit"
              disabled={busy || !draft.trim() || draft.trim() === settings?.root}
              className="rounded bg-accent px-2 py-0.5 font-medium text-accent-ink disabled:opacity-40"
            >
              {busy ? "Moving…" : "Use this root"}
            </button>
            {settings && (
              <button
                type="button"
                onClick={() => bridge.reveal(settings.root)}
                className="rounded border border-line px-2 py-0.5 text-ink-dim hover:bg-hover hover:text-ink"
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
            className="mt-2 text-accent hover:underline"
          >
            Back to the default location
          </button>
        )}

        {/* Changing the root changes which memories every agent sees, so the
            outcome is stated rather than left to be inferred from the tree. */}
        {status && <p className="mt-2 text-ink-dim">{status}</p>}
      </section>

      <section className="px-1">
        <h3 className="mb-1 font-medium text-ink">Appearance</h3>
        <div className="flex gap-2">
          {(["dark", "light"] as const).map((value) => (
            <button
              key={value}
              type="button"
              onClick={() => onTheme(value)}
              aria-pressed={theme === value}
              className={`rounded border px-2 py-0.5 capitalize ${
                theme === value
                  ? "border-accent bg-selected text-ink"
                  : "border-line text-ink-dim hover:bg-hover hover:text-ink"
              }`}
            >
              {value}
            </button>
          ))}
        </div>
      </section>

      <section className="border-t border-line px-1 pt-3 text-ink-dim">
        <h3 className="mb-1 font-medium text-ink">This installation</h3>
        <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1">
          <dt>version</dt>
          <dd className="font-mono text-[11px]">{health?.version ?? "—"}</dd>
          <dt>mode</dt>
          <dd>{health?.portable ? "portable" : "installed"}</dd>
          <dt>config</dt>
          <dd className="selectable break-all font-mono text-[11px]">{health?.configPath ?? "—"}</dd>
          <dt>index</dt>
          <dd className="font-mono text-[11px]">
            {health ? `${Math.round(health.indexBytes / 1024)} KB` : "—"}
          </dd>
        </dl>
        <p className="mt-2 text-ink-faint">
          The index is derived from the files. Deleting it costs one rebuild and nothing else.
        </p>
      </section>
    </div>
  );
}
