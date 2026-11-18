# CubeSandbox vs ABox/NanoBox 对比调研

> 调研对象：[TencentCloud/CubeSandbox](https://github.com/TencentCloud/CubeSandbox)
> 对比对象：ABox (`internal/executor/nanobox`, `internal/executor/docker`, `internal/executor/e2b`)
> 日期：2026-04-22

## TL;DR

CubeSandbox 用 **RustVMM + KVM MicroVM + containerd shim v2 + eBPF** 这一套重型组合，把 AI Agent 沙箱做到了 **<60ms 冷启动、<5MB overhead**，并且**兼容 E2B SDK**。对 ABox 来说：

- **可抄的 3 件事（P0）**：① 预热池 + 快照克隆思路（Docker checkpoint/CRIU 或直接维护 warm container pool）；② E2B SDK 协议兼容（已经有 e2b executor，但没把自己的 API 做成 E2B-compatible 服务端）；③ CubeProxy 那种 `Host: sandbox-id.domain` 反向代理路由模式。
- **可借鉴的 2 件事（P1）**：④ Master-Cubelet 两层调度（给多节点 nanobox 铺路）；⑤ 资源池预分配 + 空闲回收的生命周期管理。
- **不建议抄的**：RustVMM/KVM/eBPF/containerd-shim 全家桶——对个人项目过重，x86_64-only，不支持 macOS 开发，ROI 极低。

---

## 1. CubeSandbox 架构速览

```
SDK (E2B-compatible)
        │
   CubeAPI (Rust, REST 网关, E2B 协议)
        │
   CubeMaster (集群调度) ──► CubeProxy (Host header 路由)
        │
   Cubelet (节点 agent)
        │
   CubeHypervisor (KVM MicroVM) + CubeShim (containerd shim v2)
        │
   CubeVS (eBPF 内核级网络隔离)
```

**核心卖点**：
- 冷启动：资源池预分配 + 快照克隆，60ms 端到端
- 隔离级别：独立内核（MicroVM）+ eBPF，接近传统 VM 但性能接近容器
- 内存开销：<5MB（镜像做了极致裁剪）
- E2B 协议兼容：`export E2B_API_URL=http://本地` 就能无缝切换

## 2. 对比表

| 维度 | CubeSandbox | ABox (nanobox/docker/e2b) |
|---|---|---|
| **定位** | 基础设施：高性能安全沙箱 | 应用平台：Serverless for Agents |
| **实现语言** | Rust | Go |
| **隔离级别** | KVM MicroVM + eBPF | Docker namespace（默认）/ E2B 云端 VM |
| **冷启动** | <60ms（快照） | Docker 秒级，E2B 云端更慢 |
| **内存 overhead** | <5MB | 依 Docker 镜像大小（当前 `agentbox-sandbox` node:22-slim + claude-code 几百 MB） |
| **兼容协议** | E2B SDK drop-in | 自有 API（aboxctl/REST），E2B 作为 executor 后端 |
| **平台** | Linux x86_64 only（需 KVM） | 跨平台（Go + Docker，macOS 开发友好） |
| **多节点** | Master-Cubelet 原生 | 单节点 server，K8s executor 走 k8s 调度 |
| **网络隔离** | eBPF + virtual switch | Docker 默认网络（较弱） |
| **目标场景** | 高并发、高密度（千级/节点）、对抗 RL 训练 | 个人/小团队跑 Agent，IM 集成优先 |
| **成熟度** | 腾讯云背书，Apache 2.0 | 个人项目，MIT |

## 3. 可借鉴的点

### P0-1 · Warm Pool + 快照克隆（冷启动优化）

**CubeSandbox 做法**：维护一个 pre-provisioned 沙箱池，请求到来时克隆快照而非冷启一个新 VM。

**ABox 现状**：每次 `nanobox.CreateSession` / docker executor 都会起新容器，claude-code + node 初始化有秒级开销。

**落地建议**（从轻到重）：
1. **nanobox/docker**：维护一个 idle warm 容器池（比如 N=3），收到请求直接 attach+`--continue` 切到新 workdir。淘汰策略：LRU + TTL。
2. **docker checkpoint (CRIU)**：把初始化完的容器状态 dump，新请求 restore。Linux-only，macOS 不可用。
3. 纯 Go 实现思路：在 `pkg/agent` 或新加的 `pkg/pool` 里封装 `Pool[Sandbox]`，跟 `internal/executor/executor.go` 的接口一起 wire。

### P0-2 · E2B 协议 drop-in 兼容（让 ABox 成为 E2B 的开源替代）

**CubeSandbox 做法**：CubeAPI 直接实现 E2B 的 REST 协议，用户只改 `E2B_API_URL` 就无缝切换。

**ABox 现状**：`internal/executor/e2b/e2b.go` 是**把 E2B 作为 executor 后端调用**（客户端方向），但 ABox 自己的 server 没有对外暴露 E2B 兼容 API。

**落地建议**：
- 在 `internal/service` 或 `cmd/abox` 加一个 `--e2b-compat` 开关，暴露 `/sandboxes`、`/sandboxes/:id/files`、`/sandboxes/:id/process` 这些 E2B 协议端点，底层 route 到 nanobox/docker executor。
- 收益：E2B 生态的 Python/TS SDK 可以直接用 ABox，迁移成本 = 0；Zoe 要去打 E2B 时，这是最大杠杆。
- 优先级比 P0-1 更高——**生态兼容 > 性能优化**。

### P0-3 · CubeProxy 的 Host header 路由（多沙箱反向代理）

**CubeSandbox 做法**：请求 `https://<sandbox-id>-<port>.domain` 时，CubeProxy 解析 Host 路由到对应 MicroVM。

**ABox 现状**：`internal/executor/tunnel/tunnel.go` 有 WebSocket tunnel，但没有基于子域名的多沙箱 HTTP 反代。浏览器自动化、web 预览这种需求得靠 port-forward。

**落地建议**：
- 加一个 `internal/proxy` 包，基于 `httputil.ReverseProxy` 实现 `{sandbox}-{port}.abox.io` 路由。
- 对 web dashboard 的 "预览 agent 生成的前端" 场景刚需。

### P1-4 · Master-Cubelet 两层调度

**CubeSandbox 做法**：CubeMaster 做集群调度，Cubelet 管节点内生命周期。对标 K8s master/kubelet。

**ABox 现状**：`internal/scheduler/scheduler.go` 只有 227 行（瞄了一眼），多节点时走 K8s executor 借用 K8s 调度器。

**落地建议**：
- **短期不做**。对个人项目复杂度太高，K8s executor 够用。
- **长期如果要自营 nanobox 集群**，参考这个两层结构：`abox-master` + `abox-agent`（每节点一份）。再往后就是 ignite 那套了。

### P1-5 · 镜像精简（<5MB overhead）

**CubeSandbox 做法**：MicroVM 镜像极致裁剪，aggressively stripped。

**ABox 现状**：`agentbox-sandbox:latest` 基于 `node:22-slim`，装了 claude-code@2.1.61。保守估计 300-500MB。

**落地建议**：
- 换 `node:22-alpine` 或 `distroless/nodejs`。
- helper 工具（local-ls/cat/get/put/find）用 Go 单二进制，别依赖 shell。
- 目标：<100MB（不求 <5MB，那是 VM 特有）。

## 4. 不建议抄的

### ❌ RustVMM / KVM MicroVM

**理由**：
- x86_64 + Linux only，Zoe 在 Mac 上开发直接挂掉
- Rust 栈重建成本太高（ABox 已经是 Go 生态）
- 安全隔离对 ABox 的目标用户（个人/小团队跑自己的 Agent）**是伪需求**——不是多租户 PaaS，不需要抗对抗 RL。
- Docker namespace 隔离对信任链内的用户完全够用

### ❌ containerd shim v2 集成

**理由**：ABox 没有跑 containerd 的场景，K8s executor 通过 K8s API 而不是 shim 层。

### ❌ eBPF 网络隔离

**理由**：
- macOS 不支持
- Docker bridge / K8s NetworkPolicy 够用
- eBPF 调试成本极高

## 5. 落地建议（ROI 排序）

| # | 事项 | 工作量 | 收益 | 优先级 |
|---|---|---|---|---|
| 1 | **E2B 协议兼容层**（server 侧） | 3-5 天 | 🟢🟢🟢 生态杠杆 | P0 |
| 2 | **nanobox warm pool**（Go 池化） | 2-3 天 | 🟢🟢 冷启动从秒级到 <500ms | P0 |
| 3 | **subdomain 反向代理** | 1-2 天 | 🟢🟢 web 预览刚需 | P0 |
| 4 | **sandbox 镜像瘦身** | 半天 | 🟢 传输/启动都快 | P1 |
| 5 | 多节点调度（master-agent） | 1-2 周 | 🟡 单节点够用 | P2 / 长期 |

## 6. 一句话结论

> **CubeSandbox 是基础设施的极致，ABox 是产品/应用的极致。**
> 学它的**生态兼容策略（E2B drop-in）和 warm pool 思路**，不学它的**底层虚拟化路线**——那不是 Go + macOS 开发者的赛道。

---

**References**
- https://github.com/TencentCloud/CubeSandbox
- https://github.com/TencentCloud/CubeSandbox/blob/master/docs/architecture/overview.md
- ABox nanobox executor: `/Users/zoe/projects/labs.zoe.im/agentbox/internal/executor/nanobox/nanobox.go`
