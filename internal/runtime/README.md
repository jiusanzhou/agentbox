# Runtimes

Agentbox supports multiple agent CLI runtimes. Each runtime wraps a different coding agent and provides a unified interface for session management, streaming, and API key passthrough.

## Supported Runtimes

| Runtime | CLI | Package | API Key Env |
|---------|-----|---------|-------------|
| `claude` | `claude` | `@anthropic-ai/claude-code` | `ANTHROPIC_API_KEY` |
| `codex` | `codex` | `@openai/codex` | `OPENAI_API_KEY` |
| `gemini` | `gemini` | `@google/gemini-cli` | `GEMINI_API_KEY` |
| `aider` | `aider` | `aider-chat` (pip) | `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` |
| `openhands` | `python -m openhands.core.main` | `openhands` (pip) | `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` |
| `goose` | `goose` | `goose-ai` (pip) | `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` |
| `custom` | user-provided script | — | — |

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

## Adding a New Runtime

1. Create `internal/runtime/<name>.go`
2. Implement the `Runtime` interface
3. Register in `init()` via `Register("<name>", &YourRuntime{})`
4. Add a Dockerfile at `deploy/sandbox/Dockerfile.<name>`
5. Build the image: `docker build -f deploy/sandbox/Dockerfile.<name> -t agentbox-sandbox:<name> deploy/sandbox/`

## Custom Runtime

The `custom` runtime runs a user-provided script. Set `AGENTBOX_CUSTOM_SCRIPT` to the script path (defaults to `/workspace/agent.sh`). The script receives the prompt as its first argument and should write output to stdout.
