<p align="center">
  <br />
  <code>&nbsp;▄▀█ █▄▄ █▀█ ▀▄▀&nbsp;</code><br />
  <code>&nbsp;█▀█ █▄█ █▄█ █&nbsp;█&nbsp;</code>
  <br />
  <br />
  <strong>Serverless for AI Agents</strong>
  <br />
  <sub>Run agents in sandboxes. Chat from Web, Telegram, Discord, Slack, Feishu, WeCom. Stream everything.</sub>
  <br />
  <br />
  <a href="./README_CN.md">中文</a> · English
  <br />
  <br />
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go" alt="Go" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License" /></a>
  <a href="#"><img src="https://img.shields.io/badge/build-passing-brightgreen.svg" alt="Build" /></a>
  <a href="https://github.com/jiusanzhou/abox-skills"><img src="https://img.shields.io/badge/skills-20+-orange.svg" alt="Skills" /></a>
</p>

---

**ABox** is a platform for running AI agents in isolated sandboxes. It provides a web dashboard, IM integrations, and a CLI — all backed by a Go API server with pluggable runtimes, stores, and executors.

## ✨ Highlights

| | |
|---|---|
| 🧊 **Sandboxed** | Docker, K8s, E2B cloud VMs, or local processes |
| 🌐 **Web Dashboard** | Chat, runs, skills marketplace, settings — full Next.js UI |
| 💬 **IM Channels** | Telegram, Discord, Slack, WeCom, Feishu, Webhook |
| 🌊 **Streaming** | Token-by-token output to web (SSE) and IM (debounced edits) |
| 🛡️ **Permission Gateway** | IM users approve/deny agent tool use via inline buttons |
| 🤖 **11 Runtimes** | Claude Code, Codex, Gemini, Aider, Cursor, Goose, OpenHands, OpenCode, OpenClaw, Custom, HTTP |
| 📁 **Local Files** | Access host files via WebDAV bridge + WebSocket tunnel |
| 🔌 **Pluggable** | Swap executor, store, storage, channels via config |
| 📝 **Markdown-Native** | `AGENTS.md` in, results out — no SDK needed |

## Quick Start

```bash
git clone https://github.com/jiusanzhou/agentbox.git && cd agentbox

# Backend
make
./bin/abox --config config.yaml

# Frontend (separate terminal)
cd web && pnpm install && pnpm dev
# → http://localhost:3000
```

Or with Docker Compose:

```bash
docker-compose up
# Backend → :8080, Frontend → :3000
```

## Web Dashboard

The Next.js frontend provides:

| Page | Description |
|------|-------------|
| `/` | Landing page |
| `/dashboard` | Runs overview and usage stats |
| `/chat` | Chat interface with SSE streaming + markdown rendering |
| `/runs` | List, create, and inspect runs |
| `/skills` | Skills marketplace — browse and one-click run |
| `/settings` | Admin panel, API keys, configuration |
| `/integrations` | Connect your own IM bots (per-user) |
| `/login`, `/register` | Email/password + GitHub OAuth |

## IM Channels

All channels support streaming output (debounced message edits) and inline button callbacks.

### Telegram

```yaml
channels:
  - type: telegram
    config:
      token: "123456:ABC-DEF"
```

### Discord

```yaml
channels:
  - type: discord
    config:
      token: "your-bot-token"
      guild_id: "optional-guild-filter"
```

### Slack

```yaml
channels:
  - type: slack
    config:
      bot_token: "xoxb-..."
      app_token: "xapp-..."   # Socket Mode
```

### WeCom (企业微信)

```yaml
channels:
  - type: wecom
    config:
      corp_id: "your-corp-id"
      agent_id: "your-agent-id"
      secret: "your-secret"
      token: "callback-token"
      encoding_aes_key: "aes-key"
      callback_path: "/api/v1/wecom/callback"
```

### Feishu (飞书)

```yaml
channels:
  - type: feishu
    config:
      app_id: "your-app-id"
      app_secret: "your-app-secret"
      verification_token: "token"
      encrypt_key: "key"
      callback_path: "/api/v1/feishu/callback"
```

### Webhook

```yaml
channels:
  - type: webhook
    config:
      path: "/api/v1/webhook"
      secret: "hmac-secret"
      response_url: "https://example.com/callback"
```

## Permission Gateway

When an agent requests tool access, IM users receive inline buttons to approve or deny:

```
🔧 Agent wants to use: execute_command
   Command: rm -rf /tmp/cache

  [✅ Allow]  [❌ Deny]
```

- 5-minute timeout → auto-deny
- Per-request granularity
- Works across all IM channels with button support

## Agent Runtimes

ABox supports multiple agent backends. Set the runtime per run:

| Runtime | Description |
|---------|-------------|
| `claude` | Claude Code (Anthropic) |
| `codex` | Codex CLI (OpenAI) |
| `gemini` | Gemini CLI (Google) |
| `aider` | Aider — AI pair programming |
| `cursor` | Cursor editor agent |
| `goose` | Goose (Block) |
| `openhands` | OpenHands (formerly OpenDevin) |
| `opencode` | OpenCode CLI |
| `openclaw` | OpenClaw Gateway |
| `custom` | Custom command — bring your own agent |
| `http` | HTTP-based agent — call any API |

## Usage Examples

### One-Shot Run

```bash
aboxctl run examples/vm0-hn-curator/AGENTS.md
aboxctl list
aboxctl get <run-id>
```

### Interactive Chat

```bash
# Default assistant
aboxctl chat

# Custom persona
aboxctl chat "You are a Go expert. Be concise."

# From AGENTS.md file
aboxctl chat examples/vm0-deep-research/AGENTS.md
```

```
  ABox Session  f8870923  running
  Ctrl+C or /quit to exit. Arrow keys for history.

> My name is Zoe and my favorite language is Go.
< Nice to meet you, Zoe! Go is a great language. How can I help?

> What is my name?
< Your name is Zoe.                          ← context preserved
```

### Local File Access

```bash
# Terminal 1: start bridge
aboxctl bridge --roots ~/Documents,~/projects

# Terminal 2: chat with file access
aboxctl chat "You have access to local files. Read LOCAL_FILES.md for instructions."
```

The agent gets helper commands automatically:

```bash
local-ls /r0/              # list directory
local-cat /r0/src/main.go  # read file
local-get /r0/data.csv     # download to workspace
local-put ./out.md /r0/    # upload to host
local-find /r0/ ".go"      # search files
```

### Session Management

```bash
aboxctl ss create "You are a data analyst."
aboxctl ss send <id> "Analyze this CSV data..."
aboxctl ss send <id> "Now create a chart"
aboxctl ss ls
aboxctl ss stop <id>
```

### File Upload

Upload files to running sessions via web UI (drag & drop) or API:

```bash
curl -X POST localhost:8080/api/v1/upload \
  -H "Authorization: Bearer <token>" \
  -F "file=@data.csv" \
  -F "run_id=<id>"
```

## Architecture

```
 Browser ─────────────┐
                       │
 aboxctl ──────────────┤
                       │  HTTP/SSE
 Telegram ─────────────┤
 Discord ──────────────┤
 Slack ────────────────┤
 WeCom ────────────────┤
 Feishu ───────────────┤
 Webhook ──────────────┤
                       ▼
             ┌──────────────────┐     ┌─────────────┐
             │  ABox API Server │────►│  Next.js UI │
             │  (Go)            │     │  :3000      │
             └────────┬─────────┘     └─────────────┘
                      │
             ┌────────▼─────────┐
             │     Engine       │
             │  run · session   │
             │  auth · channels │
             └──┬──┬──┬──┬──┬──┘
                │  │  │  │  │
     ┌──────┐ ┌▼──▼┐ │ ┌▼──▼───┐
     │Docker│ │SQL- │ │ │Local  │
     │ K8s  │ │ite │ │ │FS / S3│
     │ E2B  │ │ PG │ │ └───────┘
     │Local │ └────┘ │
     └──┬───┘        │
        │       ┌────▼─────────────┐
        ▼       │  WebSocket       │
   ┌─────────┐  │  Tunnel          │
   │ Sandbox │  └────┬─────────────┘
   │         │◄──────┘
   │ Claude/ │       ┌──────────────┐
   │ Codex/  │       │ aboxctl      │
   │ Gemini/ │◄──────│ bridge       │
   │ ...     │ WebDAV│ (MCP+WebDAV) │
   └─────────┘       └──────────────┘
                           ▲
                      Host Files
                    ~/Documents
                    ~/projects
```

## API Reference

### Auth

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/auth/register` | Register new account |
| `POST` | `/api/v1/auth/login` | Login (returns JWT) |
| `GET` | `/api/v1/auth/me` | Get current user |
| `POST` | `/api/v1/auth/apikey` | Generate API key |
| `GET` | `/api/v1/auth/github` | GitHub OAuth login |
| `GET` | `/api/v1/auth/github/callback` | GitHub OAuth callback |

### Runs & Sessions

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/run` | Submit one-shot run |
| `GET` | `/api/v1/runs` | List all runs |
| `GET` | `/api/v1/run/:id` | Get run details |
| `DELETE` | `/api/v1/run/:id` | Cancel run |
| `POST` | `/api/v1/session` | Create interactive session |
| `POST` | `/api/v1/session_message` | Send message to session |
| `DELETE` | `/api/v1/session/:id` | Stop session |

### Streaming & Files

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/stream` | SSE streaming for session messages |
| `POST` | `/api/v1/upload` | Upload file to session (multipart) |
| `GET` | `/api/v1/logs/:id` | Stream logs via SSE |
| `GET` | `/api/v1/tunnel` | WebSocket tunnel for sandbox ↔ client |

### Skills

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/skills` | List available skills |

### Integrations (per-user IM channels)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/integrations` | List user integrations |
| `POST` | `/api/v1/integrations` | Create integration |
| `GET` | `/api/v1/integrations/:id` | Get integration |
| `PUT` | `/api/v1/integrations/:id` | Update integration |
| `DELETE` | `/api/v1/integrations/:id` | Delete integration |
| `POST` | `/api/v1/integrations/:id/test` | Test integration |

### Admin

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/admin/config` | Get server config |
| `PUT` | `/api/v1/admin/config` | Update server config |
| `GET` | `/api/v1/admin/config/channels` | List channels |
| `POST` | `/api/v1/admin/config/channels` | Add channel |
| `DELETE` | `/api/v1/admin/config/channels/:index` | Remove channel |
| `GET` | `/api/v1/admin/runtimes` | List available runtimes |

### Webhooks

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/hook/:id` | Integration webhook receiver |
| `POST` | `/api/v1/wecom/callback` | WeCom event callback |
| `POST` | `/api/v1/feishu/callback` | Feishu event callback |
| `POST` | `/api/v1/webhook` | Generic webhook |
| `GET` | `/api/v1/healthz` | Health check |

## Configuration

```yaml
# Server
addr: ":8080"

# Authentication
auth:
  enabled: true
  jwt_secret: "your-secret"           # or env: ABOX_JWT_SECRET
  github_client_id: ""                 # env: ABOX_GITHUB_CLIENT_ID
  github_client_secret: ""             # env: ABOX_GITHUB_CLIENT_SECRET
  github_callback_url: ""              # env: ABOX_GITHUB_CALLBACK_URL

# Store backend
store:
  type: sqlite                         # sqlite | postgres | memory
  config:
    path: ./data/abox.db

# File storage
storage:
  type: local                          # local | s3
  config:
    root: ./data/artifacts

# Executor
executor:
  type: docker                         # docker | kubernetes | local | e2b
  config:
    image: agentbox-sandbox:latest     # docker
    # work_dir: ~/.abox/sessions       # local
    # api_key: ""                      # e2b
    # kubeconfig: ""                   # kubernetes

# IM channels (server-wide)
channels:
  - type: telegram
    config:
      token: "123456:ABC-DEF"
  - type: discord
    config:
      token: "bot-token"

# Session lifecycle
session_ttl: "30m"                     # auto-cleanup idle sessions
cleanup_interval: "5m"

# Rate limiting
rate_limit:
  requests_per_minute: 60
  burst_size: 10

# CORS
cors:
  allowed_origins: ["http://localhost:3000"]
  allow_credentials: true
```

Environment variables:

| Variable | Description |
|----------|-------------|
| `ABOX_JWT_SECRET` | JWT signing key |
| `ABOX_GITHUB_CLIENT_ID` | GitHub OAuth client ID |
| `ABOX_GITHUB_CLIENT_SECRET` | GitHub OAuth client secret |
| `ABOX_GITHUB_CALLBACK_URL` | GitHub OAuth callback URL |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token (alt config) |

## Agent Format

```markdown
# Research Agent

## Instructions
You are a research assistant. Gather information
from multiple sources and create summary reports.

## Workflow
1. Identify 3-5 relevant sources
2. Extract key information from each
3. Write individual reports
4. Combine into output/report.md

## Guidelines
- Keep reports concise and readable
- Cite sources
```

## CLI Reference

```
abox                              Server
aboxctl run <AGENTS.md>           One-shot run
aboxctl list                      List runs
aboxctl get <id>                  Run details
aboxctl cancel <id>               Cancel run
aboxctl chat [prompt|file]        Interactive session (streaming)
aboxctl ss create [prompt|file]   Create session
aboxctl ss send <id> <msg>        Send message
aboxctl ss ls                     List sessions
aboxctl ss stop <id>              Stop session
aboxctl bridge -r <dirs>          Start data bridge
```

## Deployment

### Docker Compose

```bash
docker-compose up -d
# abox (API) → :8080
# web (UI)   → :3000
```

`docker-compose.yaml` includes the API server, web frontend, and a persistent data volume. The Docker socket is mounted for the Docker executor.

### Goreleaser

Multi-platform binary releases for linux, darwin (amd64/arm64) and windows (amd64):

```bash
goreleaser release --snapshot --clean
```

### Manual

```bash
# Backend
make
./bin/abox --config config.yaml

# Frontend
cd web
pnpm install
pnpm build   # production
pnpm dev     # development
```

## Pluggable Backends

All backends use `go.zoe.im/x` factory pattern:

```go
func init() {
    executor.Register("my-backend", func(cfg x.TypedLazyConfig, opts ...any) (executor.Executor, error) {
        var c Config
        cfg.Unmarshal(&c)
        return New(c)
    })
}
```

```yaml
executor:
  type: my-backend
  config: { ... }
```

Same pattern for `store` and `storage`.

## Skill Marketplace

Browse 20+ pre-built agent skills: **[abox-skills](https://github.com/jiusanzhou/abox-skills)**

Categories: Content & Research · Development · Data & Analytics · DevOps · Design · Automation

Skills are loaded dynamically via the `/api/v1/skills` endpoint and displayed in the web marketplace.

## Contributing

1. Fork → `git checkout -b feat/amazing` → commit → push → PR

## License

[MIT](./LICENSE)

---

<p align="center">
  Built with <a href="https://go.zoe.im/x"><code>go.zoe.im/x</code></a>
</p>
