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
