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

