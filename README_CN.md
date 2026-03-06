<p align="center">
  <br />
  <code>&nbsp;▄▀█ █▄▄ █▀█ ▀▄▀&nbsp;</code><br />
  <code>&nbsp;█▀█ █▄█ █▄█ █&nbsp;█&nbsp;</code>
  <br />
  <br />
  <strong>AI Agent 的 Serverless 平台</strong>
  <br />
  <sub>用 Markdown 定义工作流，在沙箱中执行，访问本地文件，流式输出结果。</sub>
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

**ABox** 在隔离容器中运行 AI Agent。给它一个 Markdown 文件，拿回结果。支持一次性运行、带上下文记忆的交互式会话、以及本地文件访问 — 一个 CLI 搞定全部。

## ✨ 亮点

| | |
|---|---|
| 🧊 **沙箱隔离** | 每次运行启动全新 Docker/K8s 容器 |
| 💬 **交互会话** | 持久化容器，多轮对话上下文保持 |
| 🌊 **流式输出** | 逐 token 实时打印 |
| 📁 **本地文件** | 通过 WebDAV 桥接访问宿主机文件 |
| 🔌 **可插拔** | 执行器、存储、制品存储全部可配置切换 |
| 📝 **Markdown 原生** | `AGENTS.md` 进，结果出，无需 SDK |

## 快速开始

```bash
git clone https://github.com/jiusanzhou/agentbox.git && cd agentbox
make

# 启动服务
./bin/abox --config config.yaml

# 运行你的第一个 Agent
./bin/aboxctl run examples/vm0-hn-curator/AGENTS.md
```

## 使用示例

### 一次性运行

提交 Agent，完成后获取结果：

```bash
aboxctl run examples/vm0-hn-curator/AGENTS.md
aboxctl list
aboxctl get <run-id>
```

### 交互式聊天

启动持久化会话，流式输出，方向键翻历史，多轮上下文：

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

让 Agent 读写宿主机文件：

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

编程式控制：

```bash
aboxctl ss create "你是一个数据分析师"
aboxctl ss send <id> "分析这份 CSV 数据..."
aboxctl ss send <id> "现在画个图表"
aboxctl ss ls
aboxctl ss stop <id>
```

### REST API

所有操作均可通过 HTTP 调用：

```bash
# 一次性运行
curl -X POST localhost:8080/api/v1/run \
  -H "Content-Type: application/json" \
  -d '{"name":"my-agent","agent_file":"# Agent\n## Workflow\n- echo hello"}'

# 创建会话
curl -X POST localhost:8080/api/v1/session \
  -d '{"name":"bot","agent_file":"你是一个助手"}'

# 发送消息
curl -X POST localhost:8080/api/v1/sessionmessage \
  -d '{"session_id":"<id>","message":"你好"}'
```

## 架构

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
                              宿主机文件
                           ~/Documents
                           ~/projects
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

## API 参考

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/v1/run` | 提交一次性运行 |
| `GET` | `/api/v1/runs` | 列出所有运行 |
| `GET` | `/api/v1/run/:id` | 获取运行详情 |
| `DELETE` | `/api/v1/run/:id` | 取消运行 |
| `POST` | `/api/v1/session` | 创建交互式会话 |
| `POST` | `/api/v1/sessionmessage` | 发送消息到会话 |
| `DELETE` | `/api/v1/session/:id` | 停止会话 |
| `GET` | `/api/v1/healthz` | 健康检查 |

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

Store 和 Storage 同理。

## Skill 市场

浏览 20+ 预置 Agent 技能：**[abox-skills](https://github.com/jiusanzhou/abox-skills)**

分类：内容与研究 · 开发 · 数据分析 · DevOps · 设计 · 自动化

## 贡献

Fork → `git checkout -b feat/amazing` → commit → push → PR

## 开源协议

[MIT](./LICENSE)

---

<p align="center">
  基于 <a href="https://go.zoe.im/x"><code>go.zoe.im/x</code></a> 构建
</p>
