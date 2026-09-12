import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { api, bridge } from "./bridge";
import type { Health, Meta, Project } from "./types";

export type Library = {
  projects: Project[];
  /** Memory metadata by project slug. Bodies are fetched only when opened. */
  memories: Record<string, Meta[]>;
  health: Health | null;
  /** Tag counts across every project, for the sidebar's TAGS section. */
  tags: { tag: string; count: number }[];
  error: string | null;
  loading: boolean;
  reload: () => void;
};

/**
 * Loads the whole library and keeps it current from the daemon's event stream.
 *
 * ponytail: every project's metadata is fetched up front, in one pass. That is
 * what makes global tag counts and quick-open work without a second index in
 * the renderer, and metadata is small — no bodies are fetched. The ceiling is
 * one request per project per reload; if a root ever holds enough projects for
 * that to matter, add a single aggregate endpoint rather than paging here.
 */
export function useLibrary(): Library {
  const [projects, setProjects] = useState<Project[]>([]);
  const [memories, setMemories] = useState<Record<string, Meta[]>>({});
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [nonce, setNonce] = useState(0);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  // Events arrive in bursts — writing a memory reindexes it, which publishes
  // again. Reloading on each one would refetch the library several times for
  // one user action, so they are coalesced.
  const pending = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    const stop = bridge.onEvent(() => {
      if (pending.current) clearTimeout(pending.current);
      pending.current = setTimeout(reload, 120);
    });
    return () => {
      if (pending.current) clearTimeout(pending.current);
      stop();
    };
  }, [reload]);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        const [{ projects: list }, current] = await Promise.all([
          api<{ projects: Project[] }>("GET", "/v1/projects"),
          api<Health>("GET", "/v1/health"),
        ]);
        if (cancelled) return;

        const entries = await Promise.all(
          list.map(async (project) => {
            const { memories: metas } = await api<{ memories: Meta[] }>(
              "GET",
              `/v1/projects/${encodeURIComponent(project.slug)}/memories`,
            );
            return [project.slug, metas] as const;
          }),
        );
        if (cancelled) return;

        setProjects(list);
        setMemories(Object.fromEntries(entries));
        setHealth(current);
        setError(null);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [nonce]);

  const tags = useMemo(() => {
    const counts = new Map<string, number>();
    for (const metas of Object.values(memories)) {
      for (const meta of metas) {
        for (const tag of meta.tags ?? []) counts.set(tag, (counts.get(tag) ?? 0) + 1);
      }
    }
    return [...counts.entries()]
      .map(([tag, count]) => ({ tag, count }))
      .sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag));
  }, [memories]);

  return { projects, memories, health, tags, error, loading, reload };
}
