## Context

`user-service/internal/bootstrap/pprof.go` 为可选 pprof 诊断监听注册独立 Fx lifecycle。当前 `OnStop` 只调用 `server.Shutdown(ctx)`；如果 Fx stop context 已取消或 deadline 已耗尽，`Shutdown` 可能立即失败，导致 listener 或 `Serve` goroutine 缺少显式强制关闭路径。业务 HTTP server 已在 graceful shutdown 失败后执行 `server.Close()`，pprof 需要对齐这一关闭兜底语义。

本变更只影响 pprof 诊断 server 的停止路径。它不改变 pprof endpoint 暴露条件、监听地址安全校验、业务 HTTP server、OpenAPI、数据库 schema、部署清单或 RBAC 行为。

## Goals / Non-Goals

**Goals:**

- 在 pprof `Shutdown` 失败后执行 best-effort `server.Close()`，确保 listener 和活动连接被强制回收。
- 保留 `Shutdown` 失败诊断信息，并在 `Close` 也失败时合并错误返回。
- 用测试覆盖已取消 context、极短 deadline、listener 关闭、`Serve` goroutine 退出和重复停止的关键路径。

**Non-Goals:**

- 不新增 pprof 配置项或改变默认地址、启用方式、生产类环境 loopback 限制。
- 不迁移 pprof 到 `common`，也不为测试引入与生产行为无关的新抽象层。
- 不改变业务 HTTP server 的生命周期实现。

## Decisions

- 在 pprof `OnStop` 内直接采用 `Shutdown` 后 `Close` 的顺序。原因是 `Shutdown` 仍应作为首选 graceful 路径，只有失败时才强制回收；这与现有 HTTP server 行为和 Go `http.Server` 语义一致。备选方案是只延长 shutdown timeout，但该方案无法处理已取消 context，也不能保证 listener 被关闭。
- 使用 `errors.Join` 合并 `fmt.Errorf("shutdown pprof server: %w", shutdownErr)` 和 `server.Close()` 的返回值。原因是调用方需要保留 graceful shutdown 的根因，同时不丢失强制关闭失败。备选方案是只返回 shutdown 错误，但会隐藏 `Close` 异常；只返回 close 错误则会丢失最先失败的 graceful shutdown 诊断。
- 测试优先基于真实 `http.Server`、真实 listener 和 Fx lifecycle 行为验证，不为 pprof 停止路径添加额外 production-only seam。原因是此变更是生命周期语义修正，真实网络 listener 更能覆盖 `Serve` goroutine 退出和重复停止场景。备选方案是通过 mock server 接口测试，但会引入非必要接口并削弱对 Go runtime 行为的覆盖。

## Risks / Trade-offs

- `server.Close()` 可能返回 `http.ErrServerClosed` 或其他关闭相关错误 -> 使用 `errors.Join` 原样返回，保留完整诊断，测试只断言关键语义而不过度绑定错误字符串。
- `Close` 会强制中断 pprof 活动连接 -> 该行为仅发生在 graceful shutdown 已失败时，符合停止阶段 best-effort 回收目标。
- lifecycle 测试可能因网络调度产生竞态 -> 使用 loopback listener、明确同步 `Serve` 退出信号和有限 timeout，避免依赖固定 sleep。

## Migration Plan

无需数据迁移或部署顺序调整。代码合入后随 user-service 发布生效；如需回滚，回退 pprof `OnStop` 的关闭逻辑和对应测试即可。

验证方式：运行 pprof/bootstrap 相关 Go 测试；普通代码变更合并前按仓库规则优先运行相关包测试，必要时运行 `make verify`。

## Open Questions

无。
