export interface Run {
  id: string;
  name: string;
  status: "pending" | "running" | "completed" | "failed";
  agent_file: string;
  config?: Record<string, unknown>;
  output?: string;
  error?: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface HealthStatus {
  status: string;
  version?: string;
}

export interface User {
  id: string;
  name: string;
  email: string;
}

export interface Session {
  id: string;
  user_id: string;
  status: "active" | "closed";
  system_prompt?: string;
  created_at: string;
  updated_at: string;
}

export interface Message {
  role: "user" | "assistant";
  content: string;
  timestamp?: string;
}

export interface Skill {
  id: string;
  name: string;
  description: string;
  category: string;
  agent_file: string;
}

// Agent Marketplace types
export interface AgentManifest {
  id: string;
  name: string;
  version: string;
  description: string;
  emoji?: string;
  avatar?: string;
  author?: string;
  license?: string;
  persona?: {
    style: string;
    language?: string[];
    tone: string;
    principles?: string[];
  };
  skills?: { name: string; version?: string }[];
  adapters?: {
    frameworks?: { name: string; version?: string; native?: boolean }[];
    tools?: {
      required?: { name: string; reason?: string }[];
      recommended?: { name: string; reason?: string }[];
      optional?: { name: string; reason?: string }[];
    };
    agent_apps?: { name: string; role: string; alternatives?: string[] }[];
    services?: { name: string; type: string }[];
  };
  model?: {
    minimum?: string;
    recommended?: string;
    context_window?: string;
  };
  experience?: {
    level?: string;
    packs?: number;
    domains?: string[];
    highlights?: { id: string; summary: string; difficulty?: string }[];
  };
  collaboration?: {
    can_delegate?: boolean;
    can_receive?: boolean;
    protocols?: string[];
  };
  marketplace?: {
    category?: string;
    tags?: string[];
    pricing?: {
      model?: string;
      base?: string;
      trial?: number;
    };
    stats?: {
      users?: number;
      rating?: number;
      tasks_completed?: number;
    };
  };
}

export interface AgentDNA {
  id: string;
  slug: string;
  version: string;
  user_id: string;
  identity: {
    name: string;
    description?: string;
    role?: string;
  };
  soul?: {
    personality?: string;
    tone?: string;
    voice?: string;
  };
  manifest?: {
    version: string;
    author?: string;
    runtime?: string;
    tags?: string[];
    pricing_model?: string;
  };
  status: string;
  downloads: number;
  rating: number;
  created_at: string;
  updated_at: string;
  published_at?: string;
  // Enriched manifest (from /manifest endpoint or search)
  agent_manifest?: AgentManifest;
}
