export interface Run {
  id: string;
  user_id?: string;
  team_id?: string;
  name: string;
  mode: "run" | "session" | "daemon";
  status: "pending" | "running" | "completed" | "failed" | "cancelled" | "interrupted";
  runtime?: string;
  executor?: string;
  agent_file: string;
  config: RunConfig;
  result?: Result;
  created_at: string;
  started_at?: string;
  ended_at?: string;
}

export interface RunConfig {
  image?: string;
  timeout?: number;
  env?: Record<string, string>;
}

export interface Result {
  exit_code: number;
  output?: string;
  artifacts?: string[];
  error?: string;
}

export interface Session {
  id: string;
  run_id: string;
  status: string;
  created_at: string;
}

export interface Message {
  role: string;
  content: string;
  timestamp: string;
}

export interface Agent {
  id: string;
  name: string;
  description: string;
  slug: string;
  version: string;
}

export interface Workflow {
  id: string;
  name: string;
  description?: string;
  steps: WorkflowStep[];
  status: string;
}

export interface WorkflowStep {
  id: string;
  agent_id: string;
  runtime: string;
  input: string;
  depends_on: string[];
}

export interface WorkflowRun {
  id: string;
  workflow_id: string;
  status: string;
  started_at?: string;
  ended_at?: string;
}

export interface Schedule {
  id: string;
  name: string;
  agent_id: string;
  cron_expr: string;
  timezone: string;
  input: string;
  enabled: boolean;
  next_run_at?: string;
}

export interface Team {
  id: string;
  name: string;
  owner_id: string;
  created_at: string;
}

export interface TeamMember {
  team_id: string;
  user_id: string;
  role: "owner" | "admin" | "member";
  joined_at: string;
}

export interface CreateRunRequest {
  name: string;
  mode?: string;
  runtime?: string;
  agent_file: string;
  config?: RunConfig;
  team_id?: string;
}

export interface SendMessageRequest {
  message: string;
}

export interface CreateWorkflowRequest {
  name: string;
  description?: string;
  steps: Omit<WorkflowStep, "id">[];
}

export interface CreateScheduleRequest {
  name: string;
  agent_id: string;
  runtime?: string;
  cron_expr: string;
  timezone?: string;
  input: string;
  enabled: boolean;
}

export interface CreateTeamRequest {
  name: string;
}

export interface AddTeamMemberRequest {
  email: string;
  role?: string;
}
