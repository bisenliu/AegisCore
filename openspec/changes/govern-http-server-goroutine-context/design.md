## Context

`user-services/internal/bootstrap/server.go` 当前在 Fx `OnStart` 中同步 `net.Listen` 绑定 HTTP 地址，然后启动 goroutine 调用 `server.Serve(listener)`。正常停止依赖 Fx `OnStop` 中的 `server.Shutdown` 让 `Serve` 返回 `http.ErrServerClosed`，意外 `Serve` 错误会记录失败并触发 `fx.Shutdowner`。

这个实现通常可以随 graceful shutdown 收敛，但 goroutine 本身没有显式绑定到可取消 context。该问题位于 `http-service-runtime` capability 的 bootstrap lifecycle 边界，应限制在 HTTP server 生命周期实现和测试内解决，不改变 controller/service/store 分层、路由、响应信封、认证边界、Redis/PostgreSQL/Ent 初始化或配置加载策略。

## Goals / Non-Goals

**Goals:**

- 让 HTTP server `Serve` goroutine 由 Fx lifecycle 创建的可取消 context 明确托管。
- 在 `OnStop` 中先触发该 context 取消，再执行现有 graceful shutdown，确保未来 goroutine 内扩展循环、重试或多资源等待时仍有直接退出信号。
- 保持当前监听失败同步返回、`http.ErrServerClosed` 非失败、非预期 `Serve` 错误触发应用级 shutdown 的语义不变。
- 增加测试覆盖 context 取消路径、正常关闭路径和非预期 serve failure 处理边界。

**Non-Goals:**

- 不新增 HTTP API、健康检查聚合、readiness probe、配置项或外部监控接口。
- 不修改 Gin 路由注册、中间件顺序、认证路由分组、响应 envelope 或错误码。
- 不修改 Redis、PostgreSQL、Ent schema、Atlas migration 或 datastore lifecycle。

## Decisions

- 在 `NewHTTPServer` 的 lifecycle closure 中创建 `serveCtx, cancelServe := context.WithCancel(context.Background())`，并让 goroutine 内显式监听 `serveCtx.Done()`。
  备选方案是继续只依赖 `server.Shutdown` 让 `Serve` 返回，但这无法满足“异步 goroutine 必须受 context 控死”的审查规则，也不利于未来在 goroutine 中增加额外阻塞逻辑。

- `OnStop` 先调用 `cancelServe()`，再按现有规则选择 `http.shutdown_timeout` 或 `defaultHTTPShutdownTimeout` 并调用 `server.Shutdown(shutdownCtx)`。
  备选方案是只在 `Shutdown` 之后取消 context，但如果未来 goroutine 中存在除 `Serve` 外的等待点，取消信号会到得太晚；先取消可以表达 lifecycle 正在停止，`Shutdown` 仍负责 HTTP graceful close。

- goroutine 内保持 `server.Serve(listener)` 的错误分类：`nil` 和 `http.ErrServerClosed` 不触发失败，其他错误继续记录并触发 `fx.Shutdowner`。
  备选方案是把 context cancellation 作为 `Serve` 的错误来源统一处理，但 Go HTTP server 的正常关闭仍通过 `http.ErrServerClosed` 表达，复用现有错误分类更稳定。

- 为 goroutine 退出补充 debug/info 级日志，至少能区分 context 取消路径、正常 server closed 路径和异常 serve failure 路径。
  备选方案是只依赖停止日志，但停止日志来自 `OnStop`，不能证明后台 goroutine 已观察到生命周期结束。

## Risks / Trade-offs

- [Risk] 先取消 context 再 `Shutdown` 可能让测试误以为无需调用 `server.Shutdown`。→ Mitigation: 测试仍断言 `OnStop` 完成 graceful shutdown，context 只是 goroutine 退出边界，不替代 HTTP server shutdown。
- [Risk] goroutine 退出日志如果使用 Error 级别会把正常停止误报为故障。→ Mitigation: 仅非预期 `Serve` 错误使用 Error；context 取消和 `http.ErrServerClosed` 使用非错误日志或不记录错误字段。
- [Risk] 用真实 HTTP listener 测试 goroutine 退出可能产生时序不稳定。→ Mitigation: 使用 `127.0.0.1:0` 和有超时的 channel 等待退出信号，避免固定端口和无限等待。

## Migration Plan

该变更仅调整用户服务内部 HTTP runtime lifecycle。部署时无需数据迁移、配置迁移或客户端配合；如出现回归，可回滚 `user-services/internal/bootstrap/server.go` 和对应测试，外部 API 合约不受影响。

## Open Questions

无。
