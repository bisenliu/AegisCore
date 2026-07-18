## Why

当前 pprof 诊断 server 在 Fx `OnStop` 中只调用 `server.Shutdown(ctx)`。当 stop context 已取消或 deadline 极短导致 graceful shutdown 失败时，pprof listener 和 `Serve` goroutine 可能缺少 best-effort 强制回收路径，与业务 HTTP server 已具备的关闭失败后 `server.Close()` 行为不一致。

## What Changes

- pprof server 的 `OnStop` 在 `Shutdown` 失败后 MUST 执行 best-effort `server.Close()`，强制关闭 listener 和活动连接。
- pprof server 停止失败时 MUST 保留 graceful shutdown 错误，并合并返回强制关闭错误，便于诊断。
- 覆盖已取消 context、极短 deadline、listener 关闭、`Serve` goroutine 退出和重复停止等停止路径。
- 不改变 pprof 是否启用、监听地址校验、生产类环境 loopback 限制或非预期 `Serve` 失败触发 Fx shutdown signal 的行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 明确 pprof listener 在 graceful shutdown 失败后的 best-effort 强制回收语义。

## Impact

- 影响代码：`user-service/internal/bootstrap/pprof.go` 及新增或更新的 pprof lifecycle 测试。
- 影响运行时：pprof 关闭路径在 context 取消或 deadline 耗尽时会尝试 `server.Close()`，降低 listener 或 goroutine 滞留风险。
- 不影响外部 HTTP API、OpenAPI、数据库 schema、RBAC、安全策略、配置项、部署资产或共享契约。
