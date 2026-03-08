<p align="center">
  <br />
  <code>&nbsp;▄▀█ █▄▄ █▀█ ▀▄▀&nbsp;</code><br />
  <code>&nbsp;█▀█ █▄█ █▄█ █&nbsp;█&nbsp;</code>
  <br />
  <br />
  <strong>AI Agent 的 Serverless 平台</strong>
  <br />
  <sub>在沙箱中运行 Agent。通过 Web、Telegram、Discord、Slack、飞书、企业微信交互。全程流式输出。</sub>
  <br />
  <br />
  <a href="./README.md">English</a> · 中文
  <br />
  <br />
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go" alt="Go" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License" /></a>
  <a href="#"><img src="https://img.shields.io/badge/build-passing-brightgreen.svg" alt="Build" /></a>
  <a href="https://github.com/jiusanzhou/abox-skills"><img src="https://img.shields.io/badge/skills-20+-orange.svg" alt="Skills" /></a>
</p>

---

**ABox** 是一个在隔离沙箱中运行 AI Agent 的平台。提供 Web 控制台、IM 集成和 CLI — 后端由 Go API 服务驱动，运行时、存储和执行器均可插拔配置。

## ✨ 亮点

| | |
|---|---|
| 🧊 **沙箱隔离** | Docker、K8s、E2B 云虚拟机或本地进程 |
| 🌐 **Web 控制台** | 聊天、运行、技能市场、设置 — 完整 Next.js UI |
| 💬 **IM 频道** | Telegram、Discord、Slack、企业微信、飞书、Webhook |
| 🌊 **流式输出** | 逐 token 推送到 Web (SSE) 和 IM (防抖编辑) |
| 🛡️ **权限网关** | IM 用户通过内联按钮审批/拒绝 Agent 工具调用 |
| 🤖 **11 种运行时** | Claude Code、Codex、Gemini、Aider、Cursor、Goose、OpenHands、OpenCode、OpenClaw、Custom、HTTP |
| 📁 **本地文件** | 通过 WebDAV 桥接 + WebSocket 隧道访问宿主机文件 |
| 🔌 **可插拔** | 执行器、存储、制品、频道全部可配置切换 |
| 📝 **Markdown 原生** | `AGENTS.md` 进，结果出，无需 SDK |

## 快速开始

```bash
git clone https://github.com/jiusanzhou/agentbox.git && cd agentbox

# 后端
make
./bin/abox --config config.yaml

# 前端（另开终端）
cd web && pnpm install && pnpm dev
# → http://localhost:3000
```

或使用 Docker Compose：

```bash
docker-compose up
# 后端 → :8080，前端 → :3000
```

## Web 控制台

Next.js 前端提供以下页面：

| 页面 | 说明 |
|------|------|
| `/` | 落地页 |
| `/dashboard` | 运行概览与用量统计 |
| `/chat` | 聊天界面，SSE 流式输出 + Markdown 渲染 |
| `/runs` | 列表、创建、查看运行详情 |
| `/skills` | 技能市场 — 浏览并一键运行 |
| `/settings` | 管理面板、API 密钥、配置 |
| `/integrations` | 连接自己的 IM 机器人（按用户） |
| `/login`、`/register` | 邮箱密码 + GitHub OAuth 登录 |

## IM 频道

所有频道均支持流式输出（防抖消息编辑）和内联按钮回调。

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
      guild_id: "可选-服务器ID过滤"
```

### Slack

```yaml
channels:
  - type: slack
    config:
      bot_token: "xoxb-..."
      app_token: "xapp-..."   # Socket Mode
```

### 企业微信 (WeCom)

```yaml
channels:
  - type: wecom
    config:
      corp_id: "企业ID"
      agent_id: "应用ID"
      secret: "应用密钥"
      token: "回调Token"
      encoding_aes_key: "AES密钥"
      callback_path: "/api/v1/wecom/callback"
```

### 飞书 (Feishu)

```yaml
channels:
  - type: feishu
    config:
      app_id: "应用ID"
      app_secret: "应用密钥"
      verification_token: "验证Token"
      encrypt_key: "加密密钥"
      callback_path: "/api/v1/feishu/callback"
```

### Webhook

```yaml
channels:
  - type: webhook
    config:
      path: "/api/v1/webhook"
      secret: "HMAC密钥"
      response_url: "https://example.com/callback"
```

## 权限网关

当 Agent 请求使用工具时，IM 用户会收到内联按钮进行审批或拒绝：

```
🔧 Agent 请求使用: execute_command
   命令: rm -rf /tmp/cache

  [✅ 允许]  [❌ 拒绝]
```

- 5 分钟超时 → 自动拒绝
- 逐请求粒度
- 所有支持按钮的 IM 频道均可使用

## Agent 运行时

ABox 支持多种 Agent 后端，可按运行设置运行时：

| 运行时 | 说明 |
|--------|------|
| `claude` | Claude Code (Anthropic) |
| `codex` | Codex CLI (OpenAI) |
| `gemini` | Gemini CLI (Google) |
| `aider` | Aider — AI 配对编程 |
| `cursor` | Cursor 编辑器 Agent |
| `goose` | Goose (Block) |
| `openhands` | OpenHands（原 OpenDevin） |
| `opencode` | OpenCode CLI |
| `openclaw` | OpenClaw Gateway |
| `custom` | 自定义命令 — 接入你自己的 Agent |
| `http` | HTTP Agent — 调用任意 API |

## 使用示例

### 一次性运行

```bash
aboxctl run examples/vm0-hn-curator/AGENTS.md
aboxctl list
aboxctl get <run-id>
```

### 交互式聊天

```bash
# 默认助手
aboxctl chat

# 自定义人设
aboxctl chat "你是一个 Go 专家，用中文回答"

# 用 AGENTS.md 文件
aboxctl chat examples/vm0-deep-research/AGENTS.md
```

```
  ABox Session  f8870923  running
  Ctrl+C or /quit to exit. Arrow keys for history.

> 我叫 Zoe，最喜欢的语言是 Go
< 你好 Zoe！Go 是很棒的语言，有什么可以帮你的？

> 我叫什么？
< 你叫 Zoe。                                ← 上下文保持
```

### 本地文件访问

```bash
# 终端 1: 启动桥接
aboxctl bridge --roots ~/Documents,~/projects

# 终端 2: 聊天，自动获得文件访问能力
aboxctl chat "你可以访问本地文件，先读 LOCAL_FILES.md"
```

Agent 自动获得 helper 命令：

```bash
local-ls /r0/              # 列目录
local-cat /r0/src/main.go  # 读文件
local-get /r0/data.csv     # 下载到工作区
local-put ./out.md /r0/    # 上传到宿主机
local-find /r0/ ".go"      # 搜索文件
```

### 会话管理

```bash
aboxctl ss create "你是一个数据分析师"
aboxctl ss send <id> "分析这份 CSV 数据..."
aboxctl ss send <id> "现在画个图表"
aboxctl ss ls
aboxctl ss stop <id>
```

### 文件上传

通过 Web UI（拖拽上传）或 API 上传文件到运行中的会话：

```bash
curl -X POST localhost:8080/api/v1/upload \
  -H "Authorization: Bearer <token>" \
  -F "file=@data.csv" \
  -F "run_id=<id>"
```

## 架构

```
 浏览器 ──────────────┐
                       │
 aboxctl ──────────────┤
                       │  HTTP/SSE
 Telegram ─────────────┤
 Discord ──────────────┤
 Slack ────────────────┤
 企业微信 ─────────────┤
 飞书 ─────────────────┤
 Webhook ──────────────┤
                       ▼
             ┌──────────────────┐     ┌─────────────┐
             │  ABox API 服务   │────►│  Next.js UI │
             │  (Go)            │     │  :3000      │
             └────────┬─────────┘     └─────────────┘
                      │
             ┌────────▼─────────┐
             │     引擎          │
             │  run · session   │
             │  auth · channels │
             └──┬──┬──┬──┬──┬──┘
                │  │  │  │  │
     ┌──────┐ ┌▼──▼┐ │ ┌▼──▼───┐
     │Docker│ │SQL- │ │ │本地   │
     │ K8s  │ │ite │ │ │FS / S3│
     │ E2B  │ │ PG │ │ └───────┘
     │本地  │ └────┘ │
     └──┬───┘        │
        │       ┌────▼─────────────┐
        ▼       │  WebSocket       │
   ┌─────────┐  │  隧道            │
   │ 沙箱    │  └────┬─────────────┘
   │         │◄──────┘
   │ Claude/ │       ┌──────────────┐
   │ Codex/  │       │ aboxctl      │
   │ Gemini/ │◄──────│ bridge       │
   │ ...     │ WebDAV│ (MCP+WebDAV) │
   └─────────┘       └──────────────┘
                           ▲
                        宿主机文件
                      ~/Documents
                      ~/projects
```

## API 参考

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/auth/register` | 注册新账户 |
| `POST` | `/api/v1/auth/login` | 登录（返回 JWT） |
| `GET` | `/api/v1/auth/me` | 获取当前用户 |
| `POST` | `/api/v1/auth/apikey` | 生成 API 密钥 |
| `GET` | `/api/v1/auth/github` | GitHub OAuth 登录 |
| `GET` | `/api/v1/auth/github/callback` | GitHub OAuth 回调 |

### 运行与会话

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/run` | 提交一次性运行 |
| `GET` | `/api/v1/runs` | 列出所有运行 |
| `GET` | `/api/v1/run/:id` | 获取运行详情 |
| `DELETE` | `/api/v1/run/:id` | 取消运行 |
| `POST` | `/api/v1/session` | 创建交互式会话 |
| `POST` | `/api/v1/session_message` | 发送消息到会话 |
| `DELETE` | `/api/v1/session/:id` | 停止会话 |

### 流式与文件

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/stream` | SSE 流式会话消息 |
| `POST` | `/api/v1/upload` | 上传文件到会话（multipart） |
| `GET` | `/api/v1/logs/:id` | SSE 流式日志 |
| `GET` | `/api/v1/tunnel` | WebSocket 隧道（沙箱 ↔ 客户端） |

### 技能

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/skills` | 列出可用技能 |

### 集成（按用户 IM 频道）

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/integrations` | 列出用户集成 |
| `POST` | `/api/v1/integrations` | 创建集成 |
| `GET` | `/api/v1/integrations/:id` | 获取集成详情 |
| `PUT` | `/api/v1/integrations/:id` | 更新集成 |
| `DELETE` | `/api/v1/integrations/:id` | 删除集成 |
| `POST` | `/api/v1/integrations/:id/test` | 测试集成 |

### 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/admin/config` | 获取服务配置 |
| `PUT` | `/api/v1/admin/config` | 更新服务配置 |
| `GET` | `/api/v1/admin/config/channels` | 列出频道 |
| `POST` | `/api/v1/admin/config/channels` | 添加频道 |
| `DELETE` | `/api/v1/admin/config/channels/:index` | 移除频道 |
| `GET` | `/api/v1/admin/runtimes` | 列出可用运行时 |

### Webhook

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/hook/:id` | 集成 Webhook 接收 |
| `POST` | `/api/v1/wecom/callback` | 企业微信事件回调 |
| `POST` | `/api/v1/feishu/callback` | 飞书事件回调 |
| `POST` | `/api/v1/webhook` | 通用 Webhook |
| `GET` | `/api/v1/healthz` | 健康检查 |

## 配置

```yaml
# 服务器
addr: ":8080"

# 认证
auth:
  enabled: true
  jwt_secret: "your-secret"           # 或环境变量: ABOX_JWT_SECRET
  github_client_id: ""                 # 环境变量: ABOX_GITHUB_CLIENT_ID
  github_client_secret: ""             # 环境变量: ABOX_GITHUB_CLIENT_SECRET
  github_callback_url: ""              # 环境变量: ABOX_GITHUB_CALLBACK_URL

# 数据存储后端
store:
  type: sqlite                         # sqlite | postgres | memory
  config:
    path: ./data/abox.db

# 文件存储
storage:
  type: local                          # local | s3
  config:
    root: ./data/artifacts

# 执行器
executor:
  type: docker                         # docker | kubernetes | local | e2b
  config:
    image: agentbox-sandbox:latest     # docker
    # work_dir: ~/.abox/sessions       # local
    # api_key: ""                      # e2b
    # kubeconfig: ""                   # kubernetes

# IM 频道（全局）
channels:
  - type: telegram
    config:
      token: "123456:ABC-DEF"
  - type: discord
    config:
      token: "bot-token"

# 会话生命周期
session_ttl: "30m"                     # 自动清理空闲会话
cleanup_interval: "5m"

# 限流
rate_limit:
  requests_per_minute: 60
  burst_size: 10

# 跨域
cors:
  allowed_origins: ["http://localhost:3000"]
  allow_credentials: true
```

环境变量：

| 变量 | 说明 |
|------|------|
| `ABOX_JWT_SECRET` | JWT 签名密钥 |
| `ABOX_GITHUB_CLIENT_ID` | GitHub OAuth 客户端 ID |
| `ABOX_GITHUB_CLIENT_SECRET` | GitHub OAuth 客户端密钥 |
| `ABOX_GITHUB_CALLBACK_URL` | GitHub OAuth 回调 URL |
| `TELEGRAM_BOT_TOKEN` | Telegram 机器人 Token（替代配置方式） |

## Agent 格式

```markdown
# 研究助手

## Instructions
你是一个研究助手。从多个来源收集信息并创建摘要报告。

## Workflow
1. 确定 3-5 个相关信息源
2. 从每个来源提取关键信息
3. 撰写单独的报告
4. 合并为 output/report.md

## Guidelines
- 报告保持简洁易读
- 标注来源
```

## CLI 参考

```
abox                              服务端
aboxctl run <AGENTS.md>           一次性运行
aboxctl list                      列出所有运行
aboxctl get <id>                  运行详情
aboxctl cancel <id>               取消运行
aboxctl chat [prompt|file]        交互式会话（流式输出）
aboxctl ss create [prompt|file]   创建会话
aboxctl ss send <id> <msg>        发送消息
aboxctl ss ls                     列出会话
aboxctl ss stop <id>              停止会话
aboxctl bridge -r <dirs>          启动数据桥接
```

## 部署

### Docker Compose

```bash
docker-compose up -d
# abox (API) → :8080
# web (UI)   → :3000
```

`docker-compose.yaml` 包含 API 服务、Web 前端和持久化数据卷。Docker socket 挂载供 Docker 执行器使用。

### Goreleaser

多平台二进制发布，支持 linux、darwin (amd64/arm64) 和 windows (amd64)：

```bash
goreleaser release --snapshot --clean
```

### 手动部署

```bash
# 后端
make
./bin/abox --config config.yaml

# 前端
cd web
pnpm install
pnpm build   # 生产环境
pnpm dev     # 开发环境
```

## 可插拔后端

所有后端基于 `go.zoe.im/x` 工厂模式：

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

Store 和 Storage 同理。

## Skill 市场

浏览 20+ 预置 Agent 技能：**[abox-skills](https://github.com/jiusanzhou/abox-skills)**

分类：内容与研究 · 开发 · 数据分析 · DevOps · 设计 · 自动化

技能通过 `/api/v1/skills` 端点动态加载，在 Web 市场中展示。

## 贡献

Fork → `git checkout -b feat/amazing` → commit → push → PR

## 开源协议

[MIT](./LICENSE)

---

<p align="center">
  基于 <a href="https://go.zoe.im/x"><code>go.zoe.im/x</code></a> 构建
</p>
