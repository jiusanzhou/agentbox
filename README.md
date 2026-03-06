<p align="center">
  <br />
  <code>&nbsp;▄▀█ █▄▄ █▀█ ▀▄▀&nbsp;</code><br />
  <code>&nbsp;█▀█ █▄█ █▄█ █&nbsp;█&nbsp;</code>
  <br />
  <br />
  <strong>Serverless for AI Agents</strong>
  <br />
  <sub>Define workflows in markdown. Execute in sandboxes. Access local files. Stream results.</sub>
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

**ABox** runs AI agents in isolated containers. Give it a markdown file, get results back. Supports one-shot runs, interactive sessions with context memory, and local file access — all from one CLI.

## ✨ Highlights

| | |
|---|---|
| 🧊 **Sandboxed** | Every run in a fresh Docker/K8s container |
| 💬 **Sessions** | Persistent containers with multi-turn context |
| 🌊 **Streaming** | Token-by-token output in real time |
| 📁 **Local Files** | Access host files via WebDAV bridge |
| 🔌 **Pluggable** | Swap executor, store, storage via config |
| 📝 **Markdown-Native** | `AGENTS.md` in, results out — no SDK |

## Quick Start

```bash
git clone https://github.com/jiusanzhou/agentbox.git && cd agentbox
make

# Start server
./bin/abox --config config.yaml

# Run your first agent
./bin/aboxctl run examples/vm0-hn-curator/AGENTS.md
```

## Usage Examples

### One-Shot Run

Submit an agent, get results when done:

```bash
# Submit
aboxctl run examples/vm0-hn-curator/AGENTS.md

# Check status
aboxctl list
aboxctl get <run-id>
```

### Interactive Chat

Start a persistent session with streaming output, arrow-key history, and multi-turn context:

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

Give agents read/write access to your host files:

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

For programmatic control:

```bash
# Create a session
aboxctl ss create "You are a data analyst."

# Send messages
aboxctl ss send <id> "Analyze this CSV data..."
aboxctl ss send <id> "Now create a chart"

# List active sessions
aboxctl ss ls

# Stop
aboxctl ss stop <id>
```

### REST API

All operations available via HTTP:

```bash
# One-shot run
curl -X POST localhost:8080/api/v1/run \
  -H "Content-Type: application/json" \
  -d '{"name":"my-agent","agent_file":"# Agent\n## Workflow\n- echo hello"}'

# Session
curl -X POST localhost:8080/api/v1/session \
  -d '{"name":"bot","agent_file":"You are helpful."}'

# Send message
curl -X POST localhost:8080/api/v1/sessionmessage \
  -d '{"session_id":"<id>","message":"hello"}'
```

## Architecture

```
 aboxctl ──────────────┐
                        │  HTTP
 Web Dashboard ─────────┤
                        ▼
              ┌──────────────────┐
              │  Talk REST API   │
              └────────┬─────────┘
                       │
              ┌────────▼─────────┐
              │     Engine       │
              │  run · session   │
              └──┬──────┬─────┬──┘
                 │      │     │
           ┌─────▼┐  ┌──▼──┐ ┌▼───────┐
           │Docker│  │SQLite│ │Local FS│
           │  K8s │  │ PG  │ │   S3   │
           └──┬───┘  └─────┘ └────────┘
              │
     ┌────────▼────────┐       ┌──────────────┐
     │  Sandbox         │      │ aboxctl      │
     │  Container       │◄─────│ bridge       │
     │                  │ WebDAV│ (MCP+WebDAV) │
     │  Claude Code /   │      └──────────────┘
     │  Any LLM Agent   │           ▲
     └──────────────────┘           │
                              Host Files
                           ~/Documents
                           ~/projects
```

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

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/run` | Submit one-shot run |
| `GET` | `/api/v1/runs` | List all runs |
| `GET` | `/api/v1/run/:id` | Get run details |
| `DELETE` | `/api/v1/run/:id` | Cancel run |
| `POST` | `/api/v1/session` | Create interactive session |
| `POST` | `/api/v1/sessionmessage` | Send message to session |
| `DELETE` | `/api/v1/session/:id` | Stop session |
| `GET` | `/api/v1/healthz` | Health check |

## Configuration

```yaml
store:
  type: sqlite                    # sqlite | postgres | memory
  config:
    path: ./data/abox.db

storage:
  type: local                     # local | s3
  config:
    root: ./data/artifacts

executor:
  type: docker                    # docker | kubernetes
  config:
    image: agentbox-sandbox:latest

server:
  type: http
  config:
    addr: ":8080"
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

## Contributing

1. Fork → `git checkout -b feat/amazing` → commit → push → PR

## License

[MIT](./LICENSE)

---

<p align="center">
  Built with <a href="https://go.zoe.im/x"><code>go.zoe.im/x</code></a>
</p>
