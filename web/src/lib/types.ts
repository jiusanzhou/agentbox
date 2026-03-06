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
