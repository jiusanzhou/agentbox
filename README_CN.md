<p align="center">
  <br />
  <code>&nbsp;▄▀█ █▄▄ █▀█ ▀▄▀&nbsp;</code><br />
  <code>&nbsp;█▀█ █▄█ █▄█ █&nbsp;█&nbsp;</code>
  <br />
  <br />
  <strong>AI Agent 的 Serverless 平台</strong>
  <br />
  <sub>用 Markdown 定义工作流，在沙箱中执行，自动收集产出物。</sub>
  <br />
  <br />
  <a href="./README.md">English</a> · 中文
  <br />
  <br />
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go" alt="Go Version" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License" /></a>
  <a href="#"><img src="https://img.shields.io/badge/build-passing-brightgreen.svg" alt="Build Status" /></a>
</p>

---

**ABox** 接收一个自然语言编写的 Agent 定义文件（`AGENTS.md`），在隔离容器中执行 — Docker 或 Kubernetes。无需 SDK，无需样板代码，Markdown 进，结果出。

## 特性

- 🧊 **沙箱执行** — 每次运行启动全新容器（Docker 或 K8s Job）
- 📝 **Markdown 原生** — 用 `AGENTS.md` 定义 Agent，不需要任何 SDK
- 🔌 **可插拔后端** — 执行器、存储、制品存储全部可通过配置切换
- 🖥️ **Web 管理台** — Next.js 仪表盘，创建、监控、查看运行详情
- ⚡ **异步设计** — 提交即返回，后台协程执行
- 🗄️ **持久化历史** — 默认 SQLite，可选 PostgreSQL 或内存
- 📦 **制品存储** — 本地文件系统或 S3 兼容对象存储
- 🔒 **超时与取消** — 基于 Context 的超时控制，支持运行中取消
- 🛠️ **CLI + REST API** — 命令行和 HTTP 双通道

## 快速开始

### 1. 安装

```bash
git clone https://github.com/jiusanzhou/agentbox.git && cd agentbox
make
```

构建产物：`bin/agentbox`（服务端）和 `bin/agentboxctl`（CLI 客户端）。

### 2. 配置

```bash
cp config.yaml config.local.yaml
# 默认配置开箱即用，按需修改
```

### 3. 运行

```bash
# 启动服务
./bin/agentbox --config config.yaml

# 提交一个 Agent 工作流
./bin/agentboxctl run examples/hn-curator/AGENTS.md

# 查看状态
./bin/agentboxctl list
./bin/agentboxctl get <run-id>
```

## 架构

```
                    ┌──────────────────────────┐
                    │     Web UI (Next.js)      │
                    └────────────┬─────────────┘
                                 │
        ┌────────────┐           │  HTTP / JSON
        │ agentboxctl│───────────┤
        └────────────┘           │
                                 ▼
                    ┌──────────────────────────┐
                    │   Talk Server (REST API)  │
                    │   go.zoe.im/x/talk        │
                    └────────────┬─────────────┘
                                 │
                                 ▼
                    ┌──────────────────────────┐
                    │         Engine            │
                    │    (调度 + 生命周期管理)    │
                    └──┬─────────┬──────────┬──┘
                       │         │          │
              ┌────────▼──┐ ┌───▼────┐ ┌───▼──────┐
              │  Executor  │ │ Store  │ │ Storage  │
              │docker | k8s│ │sqlite  │ │local | s3│
              └────────┬───┘ │postgres│ └──────────┘
                       │     │memory  │
                       ▼     └────────┘
              ┌────────────────┐
              │   Sandbox      │
              │   Container    │
              │  (AGENTS.md    │
              │   → AI Agent)  │
              └────────────────┘
```

## Agent 定义格式

Agent 用简单的 Markdown 文件（`AGENTS.md`）定义：

```markdown
# HN 策展

## Instructions
你是一个 Hacker News 策展人。浏览首页，
挑选最有趣的 5 个故事，撰写精简摘要。

## Workflow
- 抓取 https://news.ycombinator.com
- 按技术价值分析和排序
- 输出到 `output/digest.md`

## Guidelines
- 聚焦技术和科学内容
- 每篇摘要不超过 3 句话
```

## 配置

```yaml
# 存储后端：memory | sqlite | postgres
store:
  type: sqlite
  config:
    path: ./data/agentbox.db

# 制品存储：local | s3
storage:
  type: local
  config:
    root: ./data/artifacts

# 执行器：docker | kubernetes
executor:
  type: docker
  config:
    image: agentbox-sandbox:latest

# 服务端传输
server:
  type: http
  config:
    addr: ":8080"
```

## API 参考

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/run` | 提交新的 Agent 运行 |
| `GET` | `/api/v1/run/:id` | 获取运行详情和结果 |
| `GET` | `/api/v1/runs` | 运行列表（支持分页） |
| `DELETE` | `/api/v1/run/:id` | 取消运行中的任务 |
| `GET` | `/api/v1/healthz` | 健康检查 |

## 可插拔后端

所有后端基于 `go.zoe.im/x` 的工厂模式实现。添加自定义后端只需三步：

**1. 实现接口**

```go
type MyExecutor struct{}

func (e *MyExecutor) Execute(ctx context.Context, req *Request) (*Response, error) { ... }
func (e *MyExecutor) Logs(ctx context.Context, id string) (string, error) { ... }
func (e *MyExecutor) Stop(ctx context.Context, id string) error { ... }
```

**2. 注册工厂**

```go
func init() {
    executor.Register("my-executor", func(cfg x.TypedLazyConfig, opts ...any) (executor.Executor, error) {
        var c MyConfig
        cfg.Unmarshal(&c)
        return NewMyExecutor(c)
    })
}
```

**3. 配置使用**

```yaml
executor:
  type: my-executor
  config:
    endpoint: https://my-service.example.com
```

Store 和 Storage 同理。

## Skill 市场

浏览和分享可复用的 Agent 工作流定义：**[abox-skills](https://github.com/jiusanzhou/abox-skills)**

## Web 管理台

ABox 内置 Next.js 管理后台：

```bash
cd web && pnpm install && pnpm dev
```

## 贡献

欢迎提交 Pull Request！

1. Fork 仓库
2. 创建特性分支（`git checkout -b feat/amazing-feature`）
3. 提交改动（`git commit -m 'feat: add amazing feature'`）
4. 推送分支（`git push origin feat/amazing-feature`）
5. 发起 Pull Request

## 开源协议

[MIT](./LICENSE)

---

<p align="center">
  基于 <a href="https://go.zoe.im/x"><code>go.zoe.im/x</code></a> 构建
</p>
