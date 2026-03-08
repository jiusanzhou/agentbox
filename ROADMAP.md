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
- [ ] Auth: OAuth (GitHub, Google)
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
- [ ] CORS configuration
- [ ] WebSocket endpoint for real-time chat

**1.4 Infrastructure**
- [x] LICENSE (MIT)
- [x] CI/CD (GitHub Actions: test, build, push image)
- [x] Server Dockerfile (abox server image)
- [x] docker-compose.yaml (one-click local dev)
- [ ] Helm chart for K8s deployment
- [ ] Goreleaser for multi-platform binaries

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
- [ ] WeChat channel
- [ ] IM ↔ user account binding
- [ ] Per-IM session management

**2.3 Billing & Quotas**
- [ ] Usage tracking (compute time, tokens, storage)
- [ ] Free tier (X runs/month, Y minutes compute)
- [ ] Pro plan (higher limits, priority scheduling)
- [ ] Stripe integration
- [ ] Usage dashboard

**2.4 Reliability**
- [ ] Tests (unit + integration)
- [ ] Postgres store (production-grade)
- [ ] S3 storage (production-grade)
- [ ] Session persistence (survive server restart)
- [ ] Health checks + metrics (Prometheus)
- [ ] Structured logging

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

- [ ] Plugin system (custom executors, channels, tools)
- [ ] Workflow chaining (agent A output → agent B input)
- [ ] Scheduled agents (cron-based daemon mode)
- [ ] Team workspaces (shared agents, shared sessions)
- [ ] API for third-party integrations
- [ ] Self-hosted enterprise edition
- [ ] SDK (Python, TypeScript) for programmatic agent creation

## Non-Goals (for now)
- Training/fine-tuning models
- Building a new LLM
- Mobile app (web is mobile-friendly enough)
- Real-time collaboration (v2+)
