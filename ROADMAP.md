# ABox Roadmap

## Vision

ABox is an AI Agent SaaS platform. Users sign up, pick an agent, click run — done. No setup, no infra.
For power users who need local file access, browser automation, or custom MCP tools — install the client.

## Architecture

```
                    ┌─────────────────────────┐
                    │      ABox Cloud          │
                    │                          │
  Browser ─────────►  Web App (Next.js)       │
  Telegram ────────►  API Server (Go)         │
  Discord ─────────►  K8s Executor            │
                    │  └─ sandbox pods         │
                    └───────────┬──────────────┘
                                │ WebSocket tunnel
                    ┌───────────▼──────────────┐
                    │   ABox Client (optional)  │
                    │   ├─ Local files (WebDAV) │
                    │   ├─ Browser (Chrome)     │
                    │   └─ MCP tools            │
                    └──────────────────────────┘
```

## Phases

### Phase 0 — Foundation ✅
> Core runtime: sandbox execution, sessions, streaming, local bridge

- [x] Pluggable executor (Docker + K8s)
- [x] Pluggable store (SQLite + Postgres skeleton)
- [x] Pluggable storage (Local + S3 skeleton)
- [x] One-shot runs (Job/container)
- [x] Interactive sessions (persistent container + `--continue`)
- [x] Streaming output (token-by-token)
- [x] CLI: `aboxctl run/chat/session/bridge`
- [x] Local file bridge (WebDAV + MCP)
- [x] Auto-inject bridge tools into containers
- [x] IM channels (Telegram)
- [x] Agent skills repo (20+ skills)

### Phase 1 — SaaS Foundation ✅
> Multi-tenant platform with user accounts and web UI

**1.1 User System**
- [x] User model (id, email, name, avatar, plan, api_key)
- [x] Auth: email/password
- [x] Auth: OAuth (GitHub)
- [x] JWT tokens for API + session cookies for web
- [x] API key auth for programmatic access
- [x] Multi-tenant isolation: user can only see own runs/sessions

**1.2 Web App (Product UI)**
- [x] Landing page with sign-up CTA
- [x] Dashboard: my runs, my sessions, usage stats
- [x] Agent marketplace: browse skills, one-click run
- [x] Chat interface: web-based SSE streaming session
- [x] Run detail page: logs, artifacts, status
- [x] Settings: API keys, connected IMs, admin panel
- [ ] Billing UI (Stripe)

**1.3 API Hardening**
- [x] Auth middleware (JWT + API key)
- [x] Rate limiting per user/plan
- [ ] Request validation (structured)
- [x] CORS configuration
- [x] WebSocket endpoint for real-time chat

**1.4 Infrastructure**
- [x] LICENSE (MIT)
- [x] CI/CD (GitHub Actions: test, build, push image)
- [x] Server Dockerfile (abox server image)
- [x] docker-compose.yaml (one-click local dev)
- [x] Helm chart for K8s deployment
- [x] Goreleaser for multi-platform binaries

### Phase 2 — Product Polish
> Make it feel like a real product

**2.1 Agent Marketplace**
- [x] Skill discovery + search + tags
- [x] One-click run from marketplace
- [ ] Community submissions (PR-based)
- [ ] Ratings and usage stats
- [ ] Skill versioning

**2.2 IM Integrations**
- [x] Discord channel
- [x] Slack channel
- [x] 飞书 channel
- [x] WeChat channel
- [x] IM ↔ user account binding
- [x] Per-IM session management

**2.3 Billing & Quotas**
- [x] Usage tracking (compute time, tokens, storage)
- [ ] Free tier (X runs/month, Y minutes compute)
- [ ] Pro plan (higher limits, priority scheduling)
- [x] Stripe integration
- [x] Usage dashboard

**2.4 Reliability**
- [x] Tests (unit + integration)
- [x] Postgres store (production-grade)
- [x] S3 storage (production-grade)
- [x] Session persistence (survive server restart)
- [x] Health checks + metrics (Prometheus)
- [x] Structured logging

### Phase 3 — ABox Client
> Desktop client for power users who need local capabilities

**3.1 Client Core**
- [ ] Cross-platform binary (macOS, Linux, Windows)
- [ ] Auto-update mechanism
- [ ] System tray / menu bar app
- [ ] WebSocket tunnel to cloud (authenticated)

**3.2 Local Capabilities**
- [ ] File bridge (WebDAV, expose selected dirs)
- [ ] Browser automation (headless Chrome / Playwright)
- [ ] Custom MCP tools (user-defined)
- [ ] Local model support (Ollama integration)

**3.3 Hybrid Mode**
- [ ] Cloud sandbox ↔ local client communication
- [ ] Agent can request local file access (user approves)
- [ ] Agent can control local browser (user approves)
- [ ] Secure tunnel with E2E encryption

### Phase 4 — Scale & Ecosystem
> Platform effects and community

- [x] Plugin system (custom executors, channels, tools)
- [x] Workflow chaining (agent A output → agent B input)
- [x] Scheduled agents (cron-based daemon mode)
- [x] Team workspaces (shared agents, shared sessions)
- [x] API for third-party integrations
- [ ] Self-hosted enterprise edition
- [ ] SDK (Python, TypeScript) for programmatic agent creation

## Non-Goals (for now)
- Training/fine-tuning models
- Building a new LLM
- Mobile app (web is mobile-friendly enough)
- Real-time collaboration (v2+)

## ABox vs NanoBox — Boundary Definition

### ABox (this repo) — Agent Platform Layer
- **Owns**: User system, auth, billing, IM channels, agent marketplace, workflow engine, scheduling, SDK, web dashboard, CLI
- **Responsibility**: Agent lifecycle management — from discovery to execution to billing
- **Executor interface**: Pluggable — Docker, K8s, E2B, Local, **NanoBox**
- **Does NOT own**: VM/container isolation internals, sandbox kernel, resource enforcement at hypervisor level

### NanoBox (labs.zoe.im/nanobox) — Sandbox Execution Layer
- **Owns**: Firecracker microVM management, snapshot/restore, pre-warm pool, template system, resource limits (CPU/mem/disk/net/time), node agent
- **Responsibility**: Provide fast, secure, isolated execution environments via API
- **API contract**: REST/gRPC — CreateSandbox, Execute, Upload/Download, Destroy
- **Does NOT own**: User identity, billing, IM routing, agent format/spec, workflow logic

### Integration Point
NanoBox is an ABox executor plugin. ABox calls NanoBox API through `internal/executor/nanobox/` (to be implemented). Same interface as Docker/K8s/E2B executors.

```
ABox Executor Interface
├── docker   → Docker Engine API
├── k8s      → Kubernetes API
├── e2b      → E2B Cloud API
├── local    → OS process
└── nanobox  → NanoBox REST/gRPC API ← the bridge
```
