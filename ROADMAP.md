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

### Phase 1 — SaaS Foundation 🔄
> Multi-tenant platform with user accounts and web UI

**1.1 User System**
- [ ] User model (id, email, name, avatar, plan, api_key)
- [ ] Auth: email/password + OAuth (GitHub, Google)
- [ ] JWT tokens for API + session cookies for web
- [ ] API key auth for programmatic access
- [ ] Multi-tenant isolation: user can only see own runs/sessions

**1.2 Web App (Product UI)**
- [ ] Landing page with sign-up CTA
- [ ] Dashboard: my runs, my sessions, usage stats
- [ ] Agent marketplace: browse skills, one-click run
- [ ] Chat interface: web-based interactive session (WebSocket)
- [ ] Run detail page: logs, artifacts, status
- [ ] Settings: API keys, connected IMs, billing

**1.3 API Hardening**
- [ ] Auth middleware (JWT + API key)
- [ ] Rate limiting per user/plan
- [ ] Request validation
- [ ] CORS configuration
- [ ] WebSocket endpoint for real-time chat

**1.4 Infrastructure**
- [ ] LICENSE (MIT)
- [ ] CI/CD (GitHub Actions: test, build, push image)
- [ ] Server Dockerfile (abox server image)
- [ ] docker-compose.yaml (one-click local dev)
- [ ] Helm chart for K8s deployment
- [ ] Goreleaser for multi-platform binaries

### Phase 2 — Product Polish
> Make it feel like a real product

**2.1 Agent Marketplace**
- [ ] Skill discovery + search + tags
- [ ] One-click run from marketplace
- [ ] Community submissions (PR-based)
- [ ] Ratings and usage stats
- [ ] Skill versioning

**2.2 IM Integrations**
- [ ] Discord channel
- [ ] Slack channel
- [ ] 飞书 channel
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
