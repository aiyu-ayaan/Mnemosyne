// These mirror the JSON the Go API emits. They are hand-written rather than
// generated: the surface is small, and a codegen step for six shapes would be
// more build machinery than the shapes are worth. If they drift, the API tests
// on the Go side are what catch it.

export type Project = {
  slug: string;
  id: string;
  name: string;
  description?: string;
  created: string;
  memoryCount: number;
};

/** A memory without its body. Listings return these. */
export type Meta = {
  project: string;
  slug: string;
  id: string;
  title: string;
  tags?: string[];
  links?: string[];
  created: string;
  updated: string;
};

export type Memory = Meta & { body: string };

export type Hit = {
  project: string;
  slug: string;
  id: string;
  title: string;
  tags?: string[];
  snippet: string;
  score: number;
};

export type Health = {
  version: string;
  root: string;
  portable: boolean;
  configPath: string;
  projects: number;
  memories: number;
  indexBytes: number;
  embeddings?: boolean;
  embeddingModel?: string;
};

export type Settings = {
  root: string;
  defaultRoot?: string;
  portable?: boolean;
};

export type ChangeEvent = {
  kind:
    | "memory.written"
    | "memory.deleted"
    | "project.written"
    | "project.deleted"
    | "index.reconciled"
    | "settings.changed";
  project?: string;
  memory?: string;
  count?: number;
  at: string;
};

/** Where the daemon listens, as the main process resolved it. */
export type ChannelInfo = {
  endpoint: string;
  root: string;
  portable: boolean;
  /** A development run: its own memory root, config, and channel. */
  dev: boolean;
  binaryPath?: string;
};

export type GraphNode = {
  id: string;
  project: string;
  slug: string;
  title: string;
  tags?: string[];
};

export type GraphEdge = {
  source: string;
  target: string;
};

export type GraphData = {
  nodes: GraphNode[];
  edges: GraphEdge[];
};

export type ThemeId =
  | "dark"
  | "midnight"
  | "tokyo"
  | "nord"
  | "catppuccin"
  | "monokai"
  | "light";

export type ThemeInfo = {
  id: ThemeId;
  name: string;
  category: "dark" | "light";
  primaryBg: string;
  editorBg: string;
  accent: string;
  tag: string;
};

export const THEMES: ThemeInfo[] = [
  {
    id: "dark",
    name: "VS Code Dark",
    category: "dark",
    primaryBg: "#181818",
    editorBg: "#1f1f1f",
    accent: "#0078d4",
    tag: "#4ec9b0",
  },
  {
    id: "midnight",
    name: "Obsidian Midnight",
    category: "dark",
    primaryBg: "#0a0c10",
    editorBg: "#0f1219",
    accent: "#8b5cf6",
    tag: "#a78bfa",
  },
  {
    id: "tokyo",
    name: "Tokyo Night",
    category: "dark",
    primaryBg: "#16161e",
    editorBg: "#1a1b26",
    accent: "#7aa2f7",
    tag: "#7dcfff",
  },
  {
    id: "nord",
    name: "Nord Arctic",
    category: "dark",
    primaryBg: "#242933",
    editorBg: "#2e3440",
    accent: "#88c0d0",
    tag: "#8fbcbb",
  },
  {
    id: "catppuccin",
    name: "Catppuccin Mocha",
    category: "dark",
    primaryBg: "#11111b",
    editorBg: "#1e1e2e",
    accent: "#cba6f7",
    tag: "#94e2d5",
  },
  {
    id: "monokai",
    name: "Monokai Pro",
    category: "dark",
    primaryBg: "#1e1f1c",
    editorBg: "#272822",
    accent: "#a6e22e",
    tag: "#66d9ef",
  },
  {
    id: "light",
    name: "Clean Paper",
    category: "light",
    primaryBg: "#f8fafc",
    editorBg: "#ffffff",
    accent: "#0284c7",
    tag: "#0d9488",
  },
];

