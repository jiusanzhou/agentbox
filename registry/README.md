# ABox Agent Registry

Official registry of ABox agents. Like Homebrew taps, but for AI agents.

## Install an Agent

```bash
abox agent install rust-proxy-expert
```

## Publish Your Agent

1. Fork this repo
2. Create `agents/<your-agent-id>/agent.yaml` (see [spec](https://github.com/jiusanzhou/agentbox/blob/main/pkg/agent/schema/agent.schema.json))
3. Add `SOUL.md`, `AGENTS.md`, and optional `experience/` directory
4. Open a PR — CI will validate your `agent.yaml`
5. Once merged, your agent is available to everyone

## Directory Structure

```
agents/
├── <agent-id>/
│   ├── agent.yaml          # Required: Agent Manifest
│   ├── SOUL.md             # Required: Personality definition
│   ├── AGENTS.md           # Required: Work instructions
│   ├── IDENTITY.md         # Optional: Visual identity
│   ├── experience/         # Optional: Experience packs
│   │   ├── index.yaml
│   │   └── *.md
│   └── README.md           # Optional: Marketplace description
└── ...
```

## Add a Custom Registry (Tap)

```yaml
# ~/.abox/config.yaml
registries:
  - name: "my-tap"
    type: "github"
    repo: "username/my-agents"
```

## License

MIT
