export default function GraphView() {
  return (
    <div className="flex h-full flex-col gap-2 p-3 text-ink-dim">
      <h2 className="text-[11px] font-semibold uppercase tracking-wide">Graph</h2>
      <p>
        The link graph lands in Phase 4, once <code className="font-mono">[[wikilinks]]</code> and
        tags are parsed into edges and stored in the index.
      </p>
    </div>
  );
}
