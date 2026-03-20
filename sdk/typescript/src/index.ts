import type {
  Run,
  Agent,
  Workflow,
  WorkflowRun,
  Schedule,
  Team,
  TeamMember,
  CreateRunRequest,
  CreateWorkflowRequest,
  CreateScheduleRequest,
  CreateTeamRequest,
  AddTeamMemberRequest,
} from "./types";

export type { Run, Agent, Workflow, WorkflowRun, Schedule, Team, TeamMember } from "./types";
export type {
  RunConfig,
  Result,
  Session,
  Message,
  WorkflowStep,
  SendMessageRequest,
  CreateRunRequest,
  CreateWorkflowRequest,
  CreateScheduleRequest,
  CreateTeamRequest,
  AddTeamMemberRequest,
} from "./types";

export class AgentBox {
  private baseURL: string;
  private apiKey: string;

  constructor(baseURL: string, apiKey: string) {
    this.baseURL = baseURL.replace(/\/$/, "");
    this.apiKey = apiKey;
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }

    const resp = await fetch(`${this.baseURL}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!resp.ok) {
      const text = await resp.text();
      throw new Error(`API error ${resp.status}: ${text}`);
    }

    if (resp.status === 204) {
      return undefined as T;
    }

    return resp.json() as Promise<T>;
  }

  // Run methods
  async createRun(req: CreateRunRequest): Promise<Run> {
    return this.request<Run>("POST", "/api/v1/runs", req);
  }

  async getRun(id: string): Promise<Run> {
    return this.request<Run>("GET", `/api/v1/runs/${id}`);
  }

  async listRuns(): Promise<Run[]> {
    return this.request<Run[]>("GET", "/api/v1/runs");
  }

  async deleteRun(id: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/runs/${id}`);
  }

  // Session methods
  async createSession(req: CreateRunRequest): Promise<Run> {
    return this.request<Run>("POST", "/api/v1/runs", { ...req, mode: "session" });
  }

  async sendMessage(sessionId: string, message: string): Promise<{ response: string }> {
    return this.request("POST", `/api/v1/sessions/${sessionId}/message`, { message });
  }

  async getSession(id: string): Promise<Run> {
    return this.getRun(id);
  }

  async listSessions(): Promise<Run[]> {
    return this.request<Run[]>("GET", "/api/v1/runs?mode=session");
  }

  async *streamSession(sessionId: string, message: string): AsyncGenerator<string, void, unknown> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      Accept: "text/event-stream",
    };
    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }

    const resp = await fetch(`${this.baseURL}/api/v1/stream`, {
      method: "POST",
      headers,
      body: JSON.stringify({ session_id: sessionId, message }),
    });

    if (!resp.ok) {
      const text = await resp.text();
      throw new Error(`API error ${resp.status}: ${text}`);
    }

    const reader = resp.body?.getReader();
    if (!reader) throw new Error("No response body");

    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";

      for (const line of lines) {
        if (line.startsWith("data: ")) {
          yield line.slice(6);
        }
      }
    }
  }

  // Agent registry methods
  async listAgents(): Promise<Agent[]> {
    return this.request<Agent[]>("GET", "/api/v1/registry/agents");
  }

  async installAgent(agentId: string): Promise<void> {
    return this.request<void>("POST", `/api/v1/registry/agents/${agentId}/hire`);
  }

  // Workflow methods
  async createWorkflow(req: CreateWorkflowRequest): Promise<Workflow> {
    return this.request<Workflow>("POST", "/api/v1/workflows", req);
  }

  async runWorkflow(workflowId: string): Promise<WorkflowRun> {
    return this.request<WorkflowRun>("POST", `/api/v1/workflows/${workflowId}/run`);
  }

  // Schedule methods
  async createSchedule(req: CreateScheduleRequest): Promise<Schedule> {
    return this.request<Schedule>("POST", "/api/v1/schedules", req);
  }

  async listSchedules(): Promise<Schedule[]> {
    return this.request<Schedule[]>("GET", "/api/v1/schedules");
  }

  async deleteSchedule(id: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/schedules/${id}`);
  }

  // Team methods
  async createTeam(req: CreateTeamRequest): Promise<Team> {
    return this.request<Team>("POST", "/api/v1/teams", req);
  }

  async listTeams(): Promise<Team[]> {
    return this.request<Team[]>("GET", "/api/v1/teams");
  }

  async getTeam(id: string): Promise<Team> {
    return this.request<Team>("GET", `/api/v1/teams/${id}`);
  }

  async addTeamMember(teamId: string, req: AddTeamMemberRequest): Promise<TeamMember> {
    return this.request<TeamMember>("POST", `/api/v1/teams/${teamId}/members`, req);
  }

  async removeTeamMember(teamId: string, userId: string): Promise<void> {
    return this.request<void>("DELETE", `/api/v1/teams/${teamId}/members/${userId}`);
  }

  async listTeamMembers(teamId: string): Promise<TeamMember[]> {
    return this.request<TeamMember[]>("GET", `/api/v1/teams/${teamId}/members`);
  }
}
