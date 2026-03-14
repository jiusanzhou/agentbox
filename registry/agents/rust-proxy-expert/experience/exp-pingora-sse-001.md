# Pingora SSE Streaming 下游写入阻塞

## 领域
Rust / Reverse Proxy / Streaming

## 难度
advanced

## 摘要
Pingora 代理中 upstream SSE chunks 正常接收，但 downstream client 收到 0 bytes。

## 现象
- Non-streaming 请求正常，只有 `stream: true` 有问题
- `response_filter` 正常返回 `Ok(())`
- `response_body_filter` 日志确认数据到达

## 排查过程
1. 排除 `enable_retry_buffering` / `read_request_body` — 跳过仍失败
2. 排除 `response_filter` 卡住 — 日志确认正常返回
3. 排除 `response_body_filter` 吃 body — `needs_conversion=false` 时跳过

## 根因
`response_body_filter` 返回 `Some(Duration)` 作为 idle timeout，Pingora 内部对这个 Duration 做了 `time::sleep(duration).await`，阻塞了 downstream write 循环。

## 解决方案
SSE 流场景下 `response_body_filter` 返回 `None`（不设 idle timeout），让 Pingora 的 pipe_response 循环自由流动。

## 教训
- Pingora 的 `response_body_filter` 返回值语义不直观，Duration 不是"超时阈值"而是"立即 sleep"
- SSE 流和普通 HTTP 响应需要不同的 filter 策略
- 调试代理层问题时，先确认数据到达每个 hook，再逐个排除
