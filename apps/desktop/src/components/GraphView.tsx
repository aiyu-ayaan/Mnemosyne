import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../lib/bridge";
import type { GraphData, GraphNode, Project } from "../lib/types";
import {
  CodeIcon,
  DocIcon,
  PauseIcon,
  PlayIcon,
  RefreshIcon,
  ResetIcon,
  SettingsIcon,
  ZoomInIcon,
  ZoomOutIcon,
} from "./Icons";

export type GraphMode = "memories" | "codegraph";

interface SimNode extends GraphNode {
  kind: "memory" | "code";
  detail?: string;
  x: number;
  y: number;
  vx: number;
  vy: number;
  radius: number;
  degree: number;
  color: string;
}

interface SimEdge {
  source: string;
  target: string;
}

type CodeSymbol = {
  node: {
    id: string;
    kind: string;
    name: string;
    qualifiedName: string;
    filePath: string;
    startLine: number;
    endLine: number;
    docstring?: string;
  };
  score: number;
};

type Props = {
  projects?: Project[];
  activeProject?: string | null;
  onOpenMemory?: (project: string, slug: string) => void;
  standalone?: boolean;
};

const KIND_COLORS: Record<string, string> = {
  memory: "#0078d4",
  struct: "#4ec9b0",
  function: "#dcdcaa",
  method: "#dcdcaa",
  type_alias: "#4ec9b0",
  property: "#9cdcfe",
  file: "#c586c0",
  constant: "#4fc1ff",
  default: "#9d9d9d",
};

export default function GraphView({
  projects = [],
  activeProject = null,
  onOpenMemory,
  standalone = false,
}: Props) {
  const [mode, setMode] = useState<GraphMode>("memories");
  const [filterProject, setFilterProject] = useState<string>(activeProject ?? "");
  const [filterQuery, setFilterQuery] = useState<string>("");
  const [paused, setPaused] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [physics, setPhysics] = useState({
    repulsion: 450,
    springLength: 80,
    centerGravity: 0.005,
    showLabels: true,
  });

  // Graph data state
  const [rawGraph, setRawGraph] = useState<GraphData>({ nodes: [], edges: [] });
  const [codeSymbols, setCodeSymbols] = useState<CodeSymbol[]>([]);
  const [codeQuery, setCodeQuery] = useState("memory");
  const [codeStatus, setCodeStatus] = useState<{ available: boolean; status?: string } | null>(null);

  // Selection & hover
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [hoveredNode, setHoveredNode] = useState<SimNode | null>(null);
  const [tooltipPos, setTooltipPos] = useState<{ x: number; y: number } | null>(null);

  // Canvas refs & transform
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const nodesRef = useRef<SimNode[]>([]);
  const edgesRef = useRef<SimEdge[]>([]);
  const transformRef = useRef<{ x: number; y: number; zoom: number }>({ x: 0, y: 0, zoom: 1 });
  const animFrameRef = useRef<number | null>(null);
  const draggingNodeRef = useRef<SimNode | null>(null);
  const isPanningRef = useRef(false);
  const panStartRef = useRef<{ x: number; y: number; originX: number; originY: number }>({
    x: 0,
    y: 0,
    originX: 0,
    originY: 0,
  });
  const mouseDownPosRef = useRef<{ x: number; y: number }>({ x: 0, y: 0 });

  // 1. Fetch memories graph
  const loadMemoriesGraph = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const qs = filterProject ? `?project=${encodeURIComponent(filterProject)}` : "";
      const data = await api<GraphData>("GET", `/v1/graph${qs}`);
      setRawGraph(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [filterProject]);

  // 2. Fetch CodeGraph status and symbols
  const loadCodeGraphStatus = useCallback(async () => {
    try {
      const res = await api<{ available: boolean; status?: string }>("GET", "/v1/codegraph/status");
      setCodeStatus(res);
    } catch {
      setCodeStatus({ available: false });
    }
  }, []);

  const searchCodeGraph = useCallback(async (q: string) => {
    if (!q.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const res = await api<{ available: boolean; results?: CodeSymbol[]; error?: string }>(
        "GET",
        `/v1/codegraph/query?q=${encodeURIComponent(q.trim())}`,
      );
      if (res.results) {
        setCodeSymbols(res.results);
      } else if (res.error) {
        setError(res.error);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (mode === "memories") {
      loadMemoriesGraph();
    } else {
      loadCodeGraphStatus();
      searchCodeGraph(codeQuery);
    }
  }, [mode, loadMemoriesGraph, loadCodeGraphStatus, searchCodeGraph, codeQuery]);

  // Build simulation nodes & edges when rawGraph or codeSymbols change
  useEffect(() => {
    const width = containerRef.current?.clientWidth || 800;
    const height = containerRef.current?.clientHeight || 600;
    const cx = width / 2;
    const cy = height / 2;

    if (mode === "memories") {
      const degreeMap: Record<string, number> = {};
      rawGraph.edges.forEach((e) => {
        degreeMap[e.source] = (degreeMap[e.source] || 0) + 1;
        degreeMap[e.target] = (degreeMap[e.target] || 0) + 1;
      });

      const existingMap = new Map(nodesRef.current.map((n) => [n.id, n]));

      const simNodes: SimNode[] = rawGraph.nodes.map((n, idx) => {
        const deg = degreeMap[n.id] || 0;
        const prev = existingMap.get(n.id);
        const angle = (idx / Math.max(1, rawGraph.nodes.length)) * Math.PI * 2;
        const dist = 80 + Math.random() * 120;

        return {
          ...n,
          kind: "memory",
          detail: n.project ? `${n.project}/${n.slug}` : n.slug,
          degree: deg,
          radius: Math.min(18, Math.max(5, 5 + Math.sqrt(deg) * 3)),
          color: n.tags && n.tags.length > 0 ? "#4ec9b0" : "#0078d4",
          x: prev ? prev.x : cx + Math.cos(angle) * dist,
          y: prev ? prev.y : cy + Math.sin(angle) * dist,
          vx: prev ? prev.vx * 0.5 : (Math.random() - 0.5) * 2,
          vy: prev ? prev.vy * 0.5 : (Math.random() - 0.5) * 2,
        };
      });

      nodesRef.current = simNodes;
      edgesRef.current = rawGraph.edges;
    } else {
      // CodeGraph mode: convert code symbols to nodes and synthetic file edges
      const simNodes: SimNode[] = [];
      const edges: SimEdge[] = [];
      const fileGroup: Record<string, string[]> = {};

      codeSymbols.forEach((item, idx) => {
        const filePath = item.node.filePath || "code";
        const id = item.node.id || `${filePath}:${item.node.startLine}`;
        const kind = item.node.kind || "function";
        const color = KIND_COLORS[kind] || KIND_COLORS.default || "#9d9d9d";
        const angle = (idx / Math.max(1, codeSymbols.length)) * Math.PI * 2;
        const dist = 100 + Math.random() * 140;

        simNodes.push({
          id,
          project: "codebase",
          slug: item.node.name,
          title: item.node.name,
          kind: "code",
          detail: `${filePath}:${item.node.startLine}`,
          degree: 1,
          radius: 7,
          color,
          x: cx + Math.cos(angle) * dist,
          y: cy + Math.sin(angle) * dist,
          vx: (Math.random() - 0.5) * 2,
          vy: (Math.random() - 0.5) * 2,
          tags: [kind, filePath.split("/").pop() || ""],
        });

        const group = fileGroup[filePath] ?? [];
        group.push(id);
        fileGroup[filePath] = group;
      });

      // Link symbols within the same file
      Object.values(fileGroup).forEach((ids) => {
        for (let i = 0; i < ids.length - 1; i++) {
          const s = ids[i];
          const t = ids[i + 1];
          if (s && t) {
            edges.push({ source: s, target: t });
          }
        }
      });

      nodesRef.current = simNodes;
      edgesRef.current = edges;
    }
  }, [mode, rawGraph, codeSymbols]);

  // Center camera on nodes
  const resetCamera = useCallback(() => {
    const width = containerRef.current?.clientWidth || 800;
    const height = containerRef.current?.clientHeight || 600;
    transformRef.current = { x: width / 2, y: height / 2, zoom: 1 };
  }, []);

  useEffect(() => {
    resetCamera();
  }, [resetCamera, mode]);

  // Simulation loop & rendering
  useEffect(() => {
    let animId: number;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const render = () => {
      const rect = containerRef.current?.getBoundingClientRect();
      const width = rect?.width || 800;
      const height = rect?.height || 600;
      const dpr = window.devicePixelRatio || 1;

      if (canvas.width !== width * dpr || canvas.height !== height * dpr) {
        canvas.width = width * dpr;
        canvas.height = height * dpr;
      }

      ctx.save();
      ctx.scale(dpr, dpr);

      // Background
      ctx.fillStyle = "#181818";
      ctx.fillRect(0, 0, width, height);

      const { x: panX, y: panY, zoom } = transformRef.current;

      // Draw subtle grid dots
      const gridSize = 32 * zoom;
      if (gridSize > 12) {
        ctx.fillStyle = "#262626";
        const startX = ((panX % gridSize) + gridSize) % gridSize;
        const startY = ((panY % gridSize) + gridSize) % gridSize;
        for (let gx = startX; gx < width; gx += gridSize) {
          for (let gy = startY; gy < height; gy += gridSize) {
            ctx.beginPath();
            ctx.arc(gx, gy, 1, 0, Math.PI * 2);
            ctx.fill();
          }
        }
      }

      const nodes = nodesRef.current;
      const edges = edgesRef.current;
      const draggingNode = draggingNodeRef.current;

      // Simulation Physics Step
      if (!paused && nodes.length > 0) {
        const repulsion = physics.repulsion;
        const springLength = physics.springLength;
        const springK = 0.04;
        const damping = 0.88;
        const centerGravity = physics.centerGravity;

        // Repulsion (Coulomb)
        for (let i = 0; i < nodes.length; i++) {
          const n1 = nodes[i];
          if (!n1) continue;
          for (let j = i + 1; j < nodes.length; j++) {
            const n2 = nodes[j];
            if (!n2) continue;
            const dx = n2.x - n1.x;
            const dy = n2.y - n1.y;
            const distSq = dx * dx + dy * dy + 1;
            const dist = Math.sqrt(distSq);
            if (dist < 320) {
              const force = ((repulsion / distSq) * (n1.radius * n2.radius)) / 40;
              const fx = (dx / dist) * force;
              const fy = (dy / dist) * force;
              n1.vx -= fx;
              n1.vy -= fy;
              n2.vx += fx;
              n2.vy += fy;
            }
          }
        }

        // Spring Attraction (Hooke)
        const nodeMap = new Map<string, SimNode>();
        nodes.forEach((n) => nodeMap.set(n.id, n));

        for (const edge of edges) {
          const u = nodeMap.get(edge.source);
          const v = nodeMap.get(edge.target);
          if (!u || !v) continue;

          const dx = v.x - u.x;
          const dy = v.y - u.y;
          const dist = Math.sqrt(dx * dx + dy * dy) || 1;
          const displacement = dist - springLength;
          const force = displacement * springK;
          const fx = (dx / dist) * force;
          const fy = (dy / dist) * force;

          u.vx += fx;
          u.vy += fy;
          v.vx -= fx;
          v.vy -= fy;
        }

        // Center gravity and integration
        for (const n of nodes) {
          if (n === draggingNode) continue;
          n.vx += -n.x * centerGravity;
          n.vy += -n.y * centerGravity;

          n.vx *= damping;
          n.vy *= damping;
          n.x += n.vx;
          n.y += n.vy;
        }
      }

      // World transform
      ctx.save();
      ctx.translate(panX, panY);
      ctx.scale(zoom, zoom);

      const nodeMap = new Map<string, SimNode>();
      nodes.forEach((n) => nodeMap.set(n.id, n));

      const queryLower = filterQuery.trim().toLowerCase();
      const activeId = selectedId || hoveredNode?.id;

      // Draw Edges
      for (const edge of edges) {
        const u = nodeMap.get(edge.source);
        const v = nodeMap.get(edge.target);
        if (!u || !v) continue;

        const isHighlighted =
          activeId && (u.id === activeId || v.id === activeId);

        ctx.beginPath();
        ctx.moveTo(u.x, u.y);
        ctx.lineTo(v.x, v.y);

        if (isHighlighted) {
          ctx.strokeStyle = "rgba(0, 120, 212, 0.8)";
          ctx.lineWidth = 2 / zoom;
        } else {
          ctx.strokeStyle = "rgba(75, 85, 99, 0.35)";
          ctx.lineWidth = 1 / zoom;
        }
        ctx.stroke();

        // Direction arrow
        if (zoom > 0.6) {
          const midX = (u.x + v.x) / 2;
          const midY = (u.y + v.y) / 2;
          const angle = Math.atan2(v.y - u.y, v.x - u.x);
          ctx.save();
          ctx.translate(midX, midY);
          ctx.rotate(angle);
          ctx.beginPath();
          ctx.moveTo(3 / zoom, 0);
          ctx.lineTo(-3 / zoom, -2.5 / zoom);
          ctx.lineTo(-3 / zoom, 2.5 / zoom);
          ctx.closePath();
          ctx.fillStyle = isHighlighted ? "#0078d4" : "rgba(100, 116, 139, 0.7)";
          ctx.fill();
          ctx.restore();
        }
      }

      // Draw Nodes
      for (const n of nodes) {
        const matchesFilter =
          !queryLower ||
          n.title.toLowerCase().includes(queryLower) ||
          n.slug.toLowerCase().includes(queryLower) ||
          (n.tags && n.tags.some((t) => t.toLowerCase().includes(queryLower)));

        const isSelected = n.id === selectedId;
        const isHovered = n.id === hoveredNode?.id;
        const isConnected =
          activeId &&
          edges.some(
            (e) =>
              (e.source === activeId && e.target === n.id) ||
              (e.target === activeId && e.source === n.id),
          );

        const dimmed =
          (queryLower && !matchesFilter) ||
          (activeId && !isSelected && !isHovered && !isConnected);

        ctx.save();
        ctx.globalAlpha = dimmed ? 0.25 : 1.0;

        // Glow for active/hovered
        if (isSelected || isHovered) {
          ctx.beginPath();
          ctx.arc(n.x, n.y, n.radius + 6 / zoom, 0, Math.PI * 2);
          ctx.fillStyle = isSelected
            ? "rgba(0, 120, 212, 0.35)"
            : "rgba(78, 201, 176, 0.35)";
          ctx.fill();
        }

        // Base circle
        ctx.beginPath();
        ctx.arc(n.x, n.y, n.radius, 0, Math.PI * 2);
        ctx.fillStyle = n.color;
        ctx.fill();

        // Border ring
        ctx.strokeStyle = isSelected ? "#ffffff" : isHovered ? "#9cdcfe" : "#1f1f1f";
        ctx.lineWidth = (isSelected || isHovered ? 2 : 1) / zoom;
        ctx.stroke();

        // Labels
        const showLabel =
          physics.showLabels &&
          (isSelected ||
            isHovered ||
            isConnected ||
            zoom > 1.1 ||
            (zoom > 0.7 && n.degree > 1));

        if (showLabel) {
          ctx.font = `${Math.max(10, 11 / zoom)}px "Cascadia Code", ui-monospace, sans-serif`;
          ctx.fillStyle = isSelected || isHovered ? "#ffffff" : "#cccccc";
          ctx.textAlign = "center";
          ctx.textBaseline = "top";
          const label = n.title || n.slug;
          ctx.fillText(label, n.x, n.y + n.radius + 4 / zoom);
        }

        ctx.restore();
      }

      ctx.restore(); // restore world transform
      ctx.restore(); // restore dpr scale

      animId = requestAnimationFrame(render);
    };

    animId = requestAnimationFrame(render);
    animFrameRef.current = animId;

    return () => {
      if (animId) cancelAnimationFrame(animId);
    };
  }, [paused, filterQuery, selectedId, hoveredNode]);

  // Coordinate conversion helper
  const screenToWorld = useCallback((screenX: number, screenY: number) => {
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) return { x: 0, y: 0 };
    const { x: panX, y: panY, zoom } = transformRef.current;
    const relX = screenX - rect.left;
    const relY = screenY - rect.top;
    return {
      x: (relX - panX) / zoom,
      y: (relY - panY) / zoom,
    };
  }, []);

  // Mouse / Pointer Interaction handlers
  const handleWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) return;

    const mouseX = e.clientX - rect.left;
    const mouseY = e.clientY - rect.top;
    const zoomFactor = e.deltaY < 0 ? 1.12 : 0.89;

    const current = transformRef.current;
    const newZoom = Math.min(4, Math.max(0.15, current.zoom * zoomFactor));

    // Zoom towards mouse position
    const worldX = (mouseX - current.x) / current.zoom;
    const worldY = (mouseY - current.y) / current.zoom;

    transformRef.current = {
      zoom: newZoom,
      x: mouseX - worldX * newZoom,
      y: mouseY - worldY * newZoom,
    };
  };

  const findNodeAt = useCallback(
    (screenX: number, screenY: number): SimNode | null => {
      const world = screenToWorld(screenX, screenY);
      const nodes = nodesRef.current;
      const zoom = transformRef.current.zoom;

      for (let i = nodes.length - 1; i >= 0; i--) {
        const n = nodes[i];
        if (!n) continue;
        const dx = world.x - n.x;
        const dy = world.y - n.y;
        const hitRadius = Math.max(n.radius, 8 / zoom);
        if (dx * dx + dy * dy <= hitRadius * hitRadius) {
          return n;
        }
      }
      return null;
    },
    [screenToWorld],
  );

  const handleMouseDown = (e: React.MouseEvent) => {
    if (e.button !== 0) return; // Only left click
    mouseDownPosRef.current = { x: e.clientX, y: e.clientY };

    const hit = findNodeAt(e.clientX, e.clientY);
    if (hit) {
      draggingNodeRef.current = hit;
      setSelectedId(hit.id);
    } else {
      isPanningRef.current = true;
      panStartRef.current = {
        x: e.clientX,
        y: e.clientY,
        originX: transformRef.current.x,
        originY: transformRef.current.y,
      };
    }
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) return;

    if (draggingNodeRef.current) {
      const world = screenToWorld(e.clientX, e.clientY);
      draggingNodeRef.current.x = world.x;
      draggingNodeRef.current.y = world.y;
      draggingNodeRef.current.vx = 0;
      draggingNodeRef.current.vy = 0;
      return;
    }

    if (isPanningRef.current) {
      const dx = e.clientX - panStartRef.current.x;
      const dy = e.clientY - panStartRef.current.y;
      transformRef.current.x = panStartRef.current.originX + dx;
      transformRef.current.y = panStartRef.current.originY + dy;
      return;
    }

    // Hover inspection
    const hit = findNodeAt(e.clientX, e.clientY);
    if (hit !== hoveredNode) {
      setHoveredNode(hit);
      if (hit) {
        setTooltipPos({ x: e.clientX - rect.left + 14, y: e.clientY - rect.top + 14 });
      } else {
        setTooltipPos(null);
      }
    }
  };

  const handleMouseUp = (e: React.MouseEvent) => {
    const dx = Math.abs(e.clientX - mouseDownPosRef.current.x);
    const dy = Math.abs(e.clientY - mouseDownPosRef.current.y);

    if (dx < 4 && dy < 4) {
      const hit = findNodeAt(e.clientX, e.clientY);
      if (!hit) {
        setSelectedId(null);
      }
    }

    draggingNodeRef.current = null;
    isPanningRef.current = false;
  };

  const handleDoubleClick = (e: React.MouseEvent) => {
    const hit = findNodeAt(e.clientX, e.clientY);
    if (hit && hit.kind === "memory" && onOpenMemory) {
      onOpenMemory(hit.project, hit.slug);
    }
  };

  const zoomBy = (factor: number) => {
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) return;
    const cx = rect.width / 2;
    const cy = rect.height / 2;
    const current = transformRef.current;
    const newZoom = Math.min(4, Math.max(0.15, current.zoom * factor));
    const worldX = (cx - current.x) / current.zoom;
    const worldY = (cy - current.y) / current.zoom;

    transformRef.current = {
      zoom: newZoom,
      x: cx - worldX * newZoom,
      y: cy - worldY * newZoom,
    };
  };

  // Selected node details object
  const selectedNode = useMemo(() => {
    if (!selectedId) return null;
    return nodesRef.current.find((n) => n.id === selectedId) ?? null;
  }, [selectedId]);

  return (
    <div
      ref={containerRef}
      className={`relative flex h-full w-full flex-col overflow-hidden bg-shell select-none ${
        standalone ? "border-l border-line" : ""
      }`}
    >
      {/* Top Floating Control Bar */}
      <div className="absolute left-3 top-3 z-10 flex flex-wrap items-center gap-2 rounded-md border border-line bg-raised/90 p-1.5 shadow-md backdrop-blur-sm">
        {/* Mode Selector */}
        <div className="flex rounded border border-line bg-shell p-0.5 text-[11px] font-medium">
          <button
            type="button"
            onClick={() => setMode("memories")}
            className={`flex items-center gap-1 rounded px-2 py-1 transition-colors ${
              mode === "memories"
                ? "bg-hover text-ink font-semibold"
                : "text-ink-faint hover:text-ink"
            }`}
          >
            <DocIcon className="h-3 w-3" />
            <span>Memories</span>
          </button>
          <button
            type="button"
            onClick={() => setMode("codegraph")}
            className={`flex items-center gap-1 rounded px-2 py-1 transition-colors ${
              mode === "codegraph"
                ? "bg-hover text-ink font-semibold"
                : "text-ink-faint hover:text-ink"
            }`}
          >
            <CodeIcon className="h-3 w-3" />
            <span>CodeGraph</span>
          </button>
        </div>

        {/* Project Filter (Memories mode) */}
        {mode === "memories" && projects.length > 0 && (
          <select
            value={filterProject}
            onChange={(e) => setFilterProject(e.target.value)}
            className="rounded border border-line bg-shell px-2 py-1 text-[11px] text-ink outline-none"
          >
            <option value="">All Projects</option>
            {projects.map((p) => (
              <option key={p.slug} value={p.slug}>
                {p.name || p.slug}
              </option>
            ))}
          </select>
        )}

        {/* CodeGraph Search query (CodeGraph mode) */}
        {mode === "codegraph" && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              searchCodeGraph(codeQuery);
            }}
            className="flex items-center gap-1"
          >
            <input
              type="text"
              value={codeQuery}
              onChange={(e) => setCodeQuery(e.target.value)}
              placeholder="query codebase symbols..."
              className="w-36 rounded border border-line bg-shell px-2 py-1 text-[11px] text-ink outline-none focus:border-accent"
            />
            <button
              type="submit"
              className="rounded bg-accent px-2 py-1 text-[11px] font-medium text-accent-ink hover:opacity-90"
            >
              Scan
            </button>
          </form>
        )}

        {/* Node search filter */}
        <div className="relative flex items-center">
          <input
            type="text"
            value={filterQuery}
            onChange={(e) => setFilterQuery(e.target.value)}
            placeholder="Filter nodes..."
            className="w-32 rounded border border-line bg-shell px-2 py-1 text-[11px] text-ink outline-none focus:border-accent"
          />
          {filterQuery && (
            <button
              type="button"
              onClick={() => setFilterQuery("")}
              className="absolute right-1 text-ink-faint hover:text-ink"
            >
              ✕
            </button>
          )}
        </div>

        {/* Action icons */}
        <div className="flex items-center gap-1 border-l border-line pl-1">
          <button
            type="button"
            title="Zoom In"
            onClick={() => zoomBy(1.2)}
            className="rounded p-1 text-ink-faint hover:bg-hover hover:text-ink"
          >
            <ZoomInIcon className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            title="Zoom Out"
            onClick={() => zoomBy(0.8)}
            className="rounded p-1 text-ink-faint hover:bg-hover hover:text-ink"
          >
            <ZoomOutIcon className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            title="Reset Camera"
            onClick={resetCamera}
            className="rounded p-1 text-ink-faint hover:bg-hover hover:text-ink"
          >
            <ResetIcon className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            title={paused ? "Resume Simulation" : "Pause Simulation"}
            onClick={() => setPaused((p) => !p)}
            className="rounded p-1 text-ink-faint hover:bg-hover hover:text-ink"
          >
            {paused ? <PlayIcon className="h-3.5 w-3.5" /> : <PauseIcon className="h-3.5 w-3.5" />}
          </button>
          <button
            type="button"
            title="Reload Data"
            onClick={() => (mode === "memories" ? loadMemoriesGraph() : searchCodeGraph(codeQuery))}
            className="rounded p-1 text-ink-faint hover:bg-hover hover:text-ink"
          >
            <RefreshIcon className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            title="Graph Physics Settings"
            onClick={() => setShowSettings((s) => !s)}
            className={`rounded p-1 transition-colors ${
              showSettings ? "bg-accent text-accent-ink" : "text-ink-faint hover:bg-hover hover:text-ink"
            }`}
          >
            <SettingsIcon className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {/* Physics Settings HUD Drawer */}
      {showSettings && (
        <div className="absolute left-3 top-16 z-20 w-64 rounded-md border border-line bg-raised/95 p-3 shadow-xl backdrop-blur-md text-ink-dim">
          <div className="flex items-center justify-between border-b border-line pb-1.5 mb-2.5">
            <span className="text-[11px] font-semibold uppercase tracking-wider text-ink">
              Graph Forces
            </span>
            <button
              type="button"
              onClick={() => setShowSettings(false)}
              className="text-xs text-ink-faint hover:text-ink"
            >
              ✕
            </button>
          </div>

          <div className="flex flex-col gap-2.5 text-xs">
            <div>
              <div className="flex justify-between mb-1">
                <span>Repulsion Force</span>
                <span className="font-mono text-[11px] text-ink">{physics.repulsion}</span>
              </div>
              <input
                type="range"
                min="150"
                max="900"
                step="25"
                value={physics.repulsion}
                onChange={(e) =>
                  setPhysics((p) => ({ ...p, repulsion: Number(e.target.value) }))
                }
                className="w-full accent-accent"
              />
            </div>

            <div>
              <div className="flex justify-between mb-1">
                <span>Link Distance</span>
                <span className="font-mono text-[11px] text-ink">{physics.springLength}px</span>
              </div>
              <input
                type="range"
                min="40"
                max="220"
                step="10"
                value={physics.springLength}
                onChange={(e) =>
                  setPhysics((p) => ({ ...p, springLength: Number(e.target.value) }))
                }
                className="w-full accent-accent"
              />
            </div>

            <div>
              <div className="flex justify-between mb-1">
                <span>Center Gravity</span>
                <span className="font-mono text-[11px] text-ink">{physics.centerGravity}</span>
              </div>
              <input
                type="range"
                min="0.001"
                max="0.02"
                step="0.001"
                value={physics.centerGravity}
                onChange={(e) =>
                  setPhysics((p) => ({ ...p, centerGravity: Number(e.target.value) }))
                }
                className="w-full accent-accent"
              />
            </div>

            <div className="flex items-center justify-between pt-1 border-t border-line/60">
              <span>Show Labels</span>
              <input
                type="checkbox"
                checked={physics.showLabels}
                onChange={(e) =>
                  setPhysics((p) => ({ ...p, showLabels: e.target.checked }))
                }
                className="accent-accent"
              />
            </div>

            <button
              type="button"
              onClick={() =>
                setPhysics({
                  repulsion: 450,
                  springLength: 80,
                  centerGravity: 0.005,
                  showLabels: true,
                })
              }
              className="mt-1 w-full rounded border border-line py-1 text-[11px] text-ink-dim hover:bg-hover hover:text-ink transition-colors"
            >
              Reset Forces to Default
            </button>
          </div>
        </div>
      )}

      {/* Status Badges Overlay (Top Right) */}
      <div className="pointer-events-none absolute right-3 top-3 z-10 flex items-center gap-2">
        {loading && (
          <span className="rounded bg-accent/20 px-2 py-0.5 text-[11px] font-mono text-accent">
            Updating...
          </span>
        )}
        <span className="rounded border border-line bg-raised/80 px-2 py-0.5 font-mono text-[11px] text-ink-dim">
          {nodesRef.current.length} nodes · {edgesRef.current.length} links
        </span>
        {mode === "codegraph" && codeStatus && (
          <span
            className={`rounded px-2 py-0.5 font-mono text-[11px] ${
              codeStatus.available ? "bg-tag/20 text-tag" : "bg-danger/20 text-danger"
            }`}
          >
            CodeGraph {codeStatus.available ? "Ready" : "Offline"}
          </span>
        )}
      </div>

      {/* Main Canvas Viewport */}
      <canvas
        ref={canvasRef}
        onWheel={handleWheel}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onDoubleClick={handleDoubleClick}
        className="h-full w-full cursor-grab active:cursor-grabbing"
      />

      {/* Hover Card / Tooltip */}
      {hoveredNode && tooltipPos && (
        <div
          className="pointer-events-none absolute z-20 max-w-xs rounded border border-line bg-raised/95 p-2 shadow-lg backdrop-blur-sm"
          style={{ left: tooltipPos.x, top: tooltipPos.y }}
        >
          <div className="flex items-center gap-1.5 font-medium text-ink">
            <span
              className="inline-block h-2.5 w-2.5 rounded-full"
              style={{ backgroundColor: hoveredNode.color }}
            />
            <span className="truncate">{hoveredNode.title || hoveredNode.slug}</span>
          </div>
          <div className="mt-1 font-mono text-[10px] text-ink-faint">
            {hoveredNode.detail || hoveredNode.slug}
          </div>
          {hoveredNode.tags && hoveredNode.tags.length > 0 && (
            <div className="mt-1 flex flex-wrap gap-1">
              {hoveredNode.tags.map((t) => (
                <span key={t} className="rounded bg-shell px-1 py-0.2 font-mono text-[9px] text-tag">
                  #{t}
                </span>
              ))}
            </div>
          )}
          <div className="mt-1 border-t border-line/60 pt-1 text-[10px] text-ink-dim">
            {hoveredNode.degree} connected {hoveredNode.degree === 1 ? "link" : "links"}
          </div>
        </div>
      )}

      {/* Selected Node Inspector Drawer (Bottom Left) */}
      {selectedNode && (
        <div className="absolute bottom-3 left-3 z-10 flex max-w-sm flex-col gap-1.5 rounded-md border border-line bg-raised/95 p-3 shadow-lg backdrop-blur-sm">
          <div className="flex items-center justify-between gap-2 border-b border-line pb-1.5">
            <div className="flex items-center gap-1.5 font-semibold text-ink">
              <span
                className="inline-block h-3 w-3 rounded-full"
                style={{ backgroundColor: selectedNode.color }}
              />
              <span className="truncate text-sm">{selectedNode.title || selectedNode.slug}</span>
            </div>
            <button
              type="button"
              onClick={() => setSelectedId(null)}
              className="text-ink-faint hover:text-ink"
            >
              ✕
            </button>
          </div>

          <div className="font-mono text-[11px] text-ink-dim">
            <span className="text-ink-faint">Path: </span>
            {selectedNode.detail || `${selectedNode.project}/${selectedNode.slug}`}
          </div>

          {selectedNode.tags && selectedNode.tags.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {selectedNode.tags.map((tag) => (
                <span key={tag} className="rounded bg-shell px-1.5 py-0.5 font-mono text-[10px] text-tag">
                  #{tag}
                </span>
              ))}
            </div>
          )}

          <div className="flex items-center justify-between pt-1">
            <span className="font-mono text-[11px] text-ink-faint">
              Connections: {selectedNode.degree}
            </span>
            {selectedNode.kind === "memory" && onOpenMemory && (
              <button
                type="button"
                onClick={() => onOpenMemory(selectedNode.project, selectedNode.slug)}
                className="rounded bg-accent px-2.5 py-1 text-[11px] font-medium text-accent-ink hover:opacity-90"
              >
                Open in Editor
              </button>
            )}
          </div>
        </div>
      )}

      {/* Empty State / Error */}
      {error && (
        <div className="absolute bottom-3 right-3 z-10 rounded border border-danger/40 bg-danger/20 px-3 py-1.5 text-[11px] text-danger">
          {error}
        </div>
      )}

      {nodesRef.current.length === 0 && !loading && (
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center text-ink-dim">
          <DocIcon className="h-10 w-10 text-ink-faint opacity-40 mb-2" />
          <p className="font-medium text-ink">No graph nodes to display</p>
          <p className="mt-1 max-w-sm text-xs text-ink-faint">
            {mode === "memories"
              ? "Create memories with [[wikilinks]] or #tags to build your knowledge graph."
              : "CodeGraph returned 0 symbols. Try indexing or scanning with a different query."}
          </p>
        </div>
      )}
    </div>
  );
}

/**
 * GraphSidebar: Minimalist VS Code/Obsidian style sidebar panel when View === "graph"
 */
export function GraphSidebar({
  projects,
  activeProject,
  onOpenMemory,
}: {
  projects: Project[];
  activeProject: string | null;
  onOpenMemory: (project: string, slug: string) => void;
}) {
  const [graphData, setGraphData] = useState<GraphData>({ nodes: [], edges: [] });
  const [loading, setLoading] = useState(false);
  const [filter, setFilter] = useState("");
  const [selectedProject, setSelectedProject] = useState(activeProject ?? "");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const qs = selectedProject ? `?project=${encodeURIComponent(selectedProject)}` : "";
      const res = await api<GraphData>("GET", `/v1/graph${qs}`);
      setGraphData(res);
    } catch {
      setGraphData({ nodes: [], edges: [] });
    } finally {
      setLoading(false);
    }
  }, [selectedProject]);

  useEffect(() => {
    load();
  }, [load]);

  const filteredNodes = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return graphData.nodes;
    return graphData.nodes.filter(
      (n) =>
        n.title.toLowerCase().includes(q) ||
        n.slug.toLowerCase().includes(q) ||
        (n.tags && n.tags.some((t) => t.toLowerCase().includes(q))),
    );
  }, [graphData.nodes, filter]);

  return (
    <div className="flex h-full flex-col overflow-hidden text-ink-dim">
      <div className="flex items-center justify-between border-b border-line px-3 py-2">
        <h2 className="text-[11px] font-semibold uppercase tracking-wide text-ink">Graph Overview</h2>
        <span className="font-mono text-[10px] text-ink-faint">
          {graphData.nodes.length} nodes · {graphData.edges.length} links
        </span>
      </div>

      {projects.length > 0 && (
        <div className="border-b border-line px-3 py-1.5">
          <select
            value={selectedProject}
            onChange={(e) => setSelectedProject(e.target.value)}
            className="w-full rounded border border-line bg-raised px-2 py-1 text-[11px] text-ink outline-none"
          >
            <option value="">All Projects</option>
            {projects.map((p) => (
              <option key={p.slug} value={p.slug}>
                {p.name || p.slug}
              </option>
            ))}
          </select>
        </div>
      )}

      <div className="border-b border-line px-3 py-1.5">
        <div className="relative flex items-center">
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Search nodes & tags..."
            className="w-full rounded border border-line bg-raised px-2 py-1 text-[11px] text-ink outline-none focus:border-accent"
          />
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-1">
        {filteredNodes.length === 0 ? (
          <p className="px-2 py-3 text-center text-[11px] text-ink-faint">
            {loading ? "Loading graph..." : "No memories found."}
          </p>
        ) : (
          filteredNodes.map((n) => (
            <button
              key={n.id}
              type="button"
              onClick={() => onOpenMemory(n.project, n.slug)}
              className="flex w-full items-center justify-between rounded px-2 py-1 text-left text-[11px] hover:bg-hover"
            >
              <div className="min-w-0 flex-1 truncate">
                <span className="text-ink font-medium">{n.title || n.slug}</span>
                <span className="ml-1.5 font-mono text-[10px] text-ink-faint">
                  {n.project}/{n.slug}
                </span>
              </div>
              {n.tags && n.tags.length > 0 && (
                <span className="ml-1 shrink-0 font-mono text-[9px] text-tag">#{n.tags[0]}</span>
              )}
            </button>
          ))
        )}
      </div>
    </div>
  );
}
