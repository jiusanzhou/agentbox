# Changelog

All notable changes to ABox are documented here.

## [Unreleased]

### Added
- **OpenClaw runtime** — integrate OpenClaw Gateway as agent backend
- **E2B cloud sandbox executor** — serverless microVM agent runtime
- **Local process executor** — run agents without Docker
- **E2E tests** — 10 E2E tests covering full API flow with mock executor
- **Unit tests** — 49 tests across 7 packages

## [0.3.0] — IM Channels & Integrations

### Added
- **Per-user IM integrations** — users configure their own IM bots via web UI
- **Discord channel** — bot support with DM and mention-based interaction
- **Slack channel** — Socket Mode bot with message editing and button callbacks
- **WeCom channel** — 企业微信 integration with AES encryption and HMAC verification
- **Feishu channel** — 飞书/Lark integration with event deduplication
- **Webhook channel** — generic HTTP webhook with HMAC-SHA256 verification
- **Permission gateway** — IM users approve/deny agent tool use via inline buttons (5-min auto-deny)
- **Streaming to IM** — token-by-token output with debounced message editing across all channels
- **10 agent runtimes** — Claude Code, Codex, Gemini, Aider, Cursor, Goose, OpenHands, OpenCode, Custom, HTTP
- **File upload** — upload files to running sessions via web UI or API
- **Log streaming** — SSE endpoint for real-time log tailing

## [0.2.0] — SaaS Foundation

### Added
- **Web dashboard** — full Next.js app with landing page, dashboard, chat, runs, skills marketplace, settings, integrations
- **User system** — registration, login, JWT + API key authentication
- **GitHub OAuth** — login/register with GitHub account
- **Chat interface** — SSE streaming with markdown rendering
- **Skills API** — dynamic skills endpoint, frontend loads from API
- **Admin panel** — server configuration, channel management, runtime listing
- **Session TTL** — auto-cleanup idle sessions with configurable TTL
- **Rate limiting** — per-user request rate limiting
- **CORS middleware** — configurable cross-origin support
- **API key passthrough** — pass API keys to agent runtimes
- **Session recovery** — recover running sessions on server restart

### Infrastructure
- **Dockerfile** — multi-stage Alpine build for minimal image size
- **docker-compose.yaml** — one-command local deployment (API + Web)
- **CI/CD** — GitHub Actions for test, build, and image push
- **MIT License**

## [0.1.0] — Foundation

### Added
- **Core engine** — run and session management
- **Docker executor** — isolated container execution
- **Kubernetes executor** — K8s Job and Pod management
- **SQLite store** — persistent run/session storage
- **Local file storage** — artifact storage on local filesystem
- **Streaming output** — token-by-token real-time output
- **Interactive chat** — `aboxctl chat` with arrow-key history and multi-turn context
- **Local file bridge** — WebDAV + MCP bridge for host file access
- **Auto-inject capabilities** — helper scripts injected into containers
- **Telegram channel** — first IM integration with long polling
- **WebSocket tunnel** — sandbox ↔ local client communication
- **CLI** — `abox` server + `aboxctl` client (run, chat, session, bridge)
- **Agent format** — markdown-native AGENTS.md workflow definition
- **Pluggable backends** — `go.zoe.im/x` factory pattern for executor, store, storage
- **Skill marketplace** — 20+ pre-built agent skills ([abox-skills](https://github.com/jiusanzhou/abox-skills))
