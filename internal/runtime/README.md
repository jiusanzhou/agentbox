# Runtimes

Agentbox supports multiple agent CLI runtimes. Each runtime wraps a different coding agent and provides a unified interface for session management, streaming, and API key passthrough.

## Supported Runtimes

| Runtime | Type | CLI | Session Resume | Output Format | API Key Env |
|---------|------|-----|----------------|---------------|-------------|
| `claude` | local | `claude -p` | `--continue` | stream-json | `ANTHROPIC_API_KEY` |
| `codex` | local | `codex exec --json` | `--previous-response-id` | JSONL | `OPENAI_API_KEY` |
| `gemini` | local | `gemini -p` | N/A | plain text | `GEMINI_API_KEY` |
| `aider` | local | `aider --message` | N/A | plain text | `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` |
| `cursor` | local | `cursor agent` | N/A | plain text | `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` |
| `opencode` | local | `opencode --non-interactive` | N/A | plain text | `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY` |
| `goose` | local | `goose run --text` | N/A | plain text | `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` |
| `openhands` | local | `openhands -t` | N/A | plain text | `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` |
| `custom` | local | user script | N/A | plain text | — |
| `http` | remote | curl to endpoint | N/A | JSON | `ABOX_HTTP_ENDPOINT` |

## Interface

```go
type Runtime interface {
    Name() string
    Image() string
    BuildExecArgs(message string, continued bool) []string
    ParseStreamLine(line string) (token string, result string, done bool)
    EnvKeys() []string
    SetupCommands() []string
}
```

- **Name**: identifier used in API requests (`"runtime": "claude"`)
- **Image**: default Docker image tag for the sandbox container
- **BuildExecArgs**: constructs the CLI command for a given prompt
- **ParseStreamLine**: parses one line of stdout into streaming tokens or a final result
- **EnvKeys**: env var names the runtime needs (API keys are mapped from the `x-api-key` header)
- **SetupCommands**: optional commands to run before first execution

## Runtime Details

### Codex (OpenAI)

Uses `codex exec --json --full-auto -` with prompt piped via stdin. Outputs JSONL with event types:
- `thread.started` — session started
- `item.completed` — agent response (text in `item.text`)
- `turn.completed` — turn finished with token usage
- `turn.failed` / `error` — error events

### Cursor

Uses `cursor agent --message` for CLI agent mode. Cursor CLI is not publicly distributable as a standalone binary; the Dockerfile is a placeholder.

### OpenCode (Crush)

Uses `opencode --non-interactive --message`. OpenCode was renamed to Crush by the charmbracelet team. Multi-provider support via `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`.

### HTTP Adapter

For external agents exposing an HTTP API (e.g. OpenClaw). Calls `$ABOX_HTTP_ENDPOINT` with a JSON payload via curl. Set `ABOX_HTTP_TOKEN` for bearer auth.

## Adding a New Runtime

1. Create `internal/runtime/<name>.go`
2. Implement the `Runtime` interface
3. Register in `init()` via `Register("<name>", &YourRuntime{})`
4. Add a Dockerfile at `deploy/sandbox/Dockerfile.<name>`
5. Build the image: `docker build -f deploy/sandbox/Dockerfile.<name> -t agentbox-sandbox:<name> deploy/sandbox/`

## Custom Runtime

The `custom` runtime runs a user-provided script. Set `AGENTBOX_CUSTOM_SCRIPT` to the script path (defaults to `/workspace/agent.sh`). The script receives the prompt as its first argument and should write output to stdout.
