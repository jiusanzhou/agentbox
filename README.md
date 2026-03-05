# AgentBox

在隔离沙箱中运行自然语言描述的 Agent 工作流。

## Architecture

```
CLI / API
    ↓
Talk Server (go.zoe.im/x/talk)
    ↓
Engine (调度 + 生命周期管理)
    ↓
Executor (docker | k8s | 天琴)  →  Sandbox Container
    ↓
Storage (local | s3)
```

核心设计：所有后端通过 `x.TypedLazyConfig` + `factory.NewFactory` 实现可插拔。

## Quick Start

```bash
make
./bin/agentbox --config config.yaml

# CLI
./bin/agentboxctl run examples/hn-curator/AGENTS.md
./bin/agentboxctl list
./bin/agentboxctl get <id>
./bin/agentboxctl cancel <id>
```

## Configuration

```yaml
store:
  type: memory          # memory | postgres
storage:
  type: local           # local | s3
  config:
    root: ./data/artifacts
executor:
  type: docker          # docker | kubernetes
  config:
    image: agentbox-sandbox:latest
server:
  type: http
  config:
    addr: ":8080"
```

## Adding Backends

```go
// Register a new executor
executor.Register("tianqin", func(cfg x.TypedLazyConfig, opts ...any) (executor.Executor, error) {
    var c TianqinConfig
    cfg.Unmarshal(&c)
    return NewTianqinExecutor(c)
})
```

Then in config:
```yaml
executor:
  type: tianqin
  config:
    service_name: my-agent
```

## License

MIT
