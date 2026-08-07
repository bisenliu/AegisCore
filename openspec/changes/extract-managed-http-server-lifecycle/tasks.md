## 1. 实现共享 HTTP server 生命周期

- [x] 1.1 创建 `common/runtime/httpserver/doc.go`、`errors.go` 和 `options.go`，实现目标公开 API、中文导出注释、严格 options 校验与包含 name、addr、操作信息的可匹配错误，确认 `New` 不监听也不创建 goroutine
- [x] 1.2 创建 `common/runtime/httpserver/managed.go`，实现 created/running/failed/stopping/stopped 状态机、`net.ListenConfig.Listen` 同步绑定、异步 `Serve`、正常退出分类以及锁外且恰好一次的 `OnServeError`
- [x] 1.3 在 `managed.go` 实现单一后台 cleanup owner、显式 listener close、Shutdown 失败后的强制 Close、Serve goroutine 等待、`errors.Join` 最终结果，以及调用方 context 仅控制本次 Stop 等待的并发重复停止语义
- [x] 1.4 创建 `common/runtime/httpserver/drain.go`，实现 handler 进入/退出跟踪、panic unwind 计数恢复、active 归零通知和可由 context 唤醒且不泄漏等待 goroutine 的 `Wait`

## 2. 覆盖共享包行为与竞态

- [x] 2.1 创建 `managed_test.go`，覆盖全部 options 校验、构造无监听/无 goroutine 副作用、地址占用同步失败、Start 返回时已绑定、`HTTPServer` addr/handler/timeout 映射和 Stop before Start
- [x] 2.2 使用包内私有 listener seam 覆盖异常 `Serve` callback 恰好一次、正常关闭不 callback、重复 Start、Stop 后 Start、Start 后立即 Stop 且 listener 与 Serve goroutine 无泄漏
- [x] 2.3 覆盖多 goroutine 并发 Stop 只有一个 cleanup owner、首次 caller Stop 超时后第二次继续等待、所有后续 Stop 返回同一最终结果、慢 handler 优雅完成、超时强制 Close 与忽略取消时保留 drain timeout
- [x] 2.4 创建 `drain_test.go` 与 `example_test.go`，覆盖正常返回、panic unwind、context 唤醒、并发等待以及完整 New/Start/Stop 示例，并执行 `cd common && go test -race ./runtime/httpserver`

## 3. 一次性迁移业务 HTTP server

- [x] 3.1 将 `user-service/internal/bootstrap/server.go` 重写为 `HTTPRuntime` composition DTO 和薄 Fx adapter；disabled 时不构造 Managed 或注册 hook，enabled 时把 Gin engine 与 `server.http` 地址和 timeout 显式映射到 `httpserver.Options`
- [x] 3.2 让业务 OnStart/OnStop 直接调用 `Managed.Start(ctx)`/`Managed.Stop(ctx)`，保留 service、environment、timezone 与 component 启停日志，并由服务侧 `OnServeError` 记录 `http server failed` 后调用 `fx.Shutdowner.Shutdown(fx.ExitCode(1))`
- [x] 3.3 迁移 `user-service/internal/bootstrap/http_test.go`，覆盖监听失败阻断 Fx 启动、异常退出策略触发 exit code 1、正常关闭不触发全局 shutdown、disabled 不构造/启动、请求 drain 以及配置映射；删除所有依赖旧 helper 或旧 tracker 内部状态的测试
- [x] 3.4 从业务 bootstrap 删除 `httpDrainTracker`、`newHTTPDrainTracker`、`closeHTTPServerAfterShutdownError`、`wrapHTTPServerCloseError`、`wrapHTTPDrainWaitError`、`serveHTTPWithLifecycle`、`handleHTTPServeExit`、`shutdownOnHTTPServeError` 和 `isExpectedHTTPServeCloseError`，不保留同签名 wrapper 或兼容入口

## 4. 一次性迁移 pprof server 与运行时装配

- [x] 4.1 将 `user-service/internal/bootstrap/pprof.go` 重写为 `PprofRuntime` composition DTO 和薄 Fx adapter；继续使用 `observability.pprof` enabled/addr 与 `common/http/pprof.Handler`，由 composition 显式选择 `server.http.shutdown_timeout` 并构造独立 Managed
- [x] 4.2 让 pprof hook 直接调用 Managed Start/Stop，并由服务侧异常回调记录 `pprof server failed` 和触发 exit code 1；删除 `servePprofServer`、`handlePprofServeExit` 及旧关闭逻辑，不保留转发入口
- [x] 4.3 迁移 `pprof_test.go`，覆盖 disabled 不监听、独立 handler、监听失败阻断启动、异常/正常退出策略、重复 Stop、强关与 drain，并验证 pprof 和业务 server 使用不同的 Managed 实例
- [x] 4.4 更新 `user-service/internal/bootstrap/app.go` 与装配验证，使 runtime graph 显式解析 `HTTPRuntime` 和 `PprofRuntime`，并补充断言 `runtime.lifecycle.stop_timeout` 大于或等于两个 Managed 的内部 shutdown timeout

## 5. 同步架构与规格

- [x] 5.1 更新 `docs/ARCHITECTURE.md`，记录 `common/runtime/httpserver` 所有权、状态与关闭边界，以及 user-service bootstrap 仅负责配置、Fx 和日志策略
- [x] 5.2 更新 `docs/opsx/CAPABILITY_MAP.md`，把 managed HTTP server 纳入 `shared-platform-primitives` 代码位置与说明，并保持 `runtime-observability` 对业务/pprof composition 的归属清晰
- [x] 5.3 检查本 change 的 proposal、design、两份 spec delta 与实现一致，执行 `openspec validate extract-managed-http-server-lifecycle` 和 `make user-service-architecture-lint`
- [x] 5.4 使用 `rg` 确认 `user-service/internal/bootstrap` 不再包含任务 3.4 与 4.2 列出的旧符号，也不再实现通用 `net.Listen`、`Serve`、`Shutdown`、`Close` 或 drain 状态机

## 6. 验证与交付门禁

- [x] 6.1 执行 `cd common && go test -race ./runtime/httpserver` 以及 `cd ../user-service && go test -race ./internal/bootstrap/...`，修复任何失败或 race 后再标记完成
- [x] 6.2 从仓库根执行相关包测试与 `make user-service-architecture-lint`，确认无需运行 OpenAPI、Ent 或部署观测生成流程且现有生成物没有 drift
- [x] 6.3 在不 reset、覆盖或混入现有 index 用户改动的前提下，暂存本 change 的新代码、bootstrap、文档和 OpenSpec 文件；对已有其他改动的共享文件使用 `git add -p`，并检查 `git diff --cached` 与 `git diff` 确认本 change 预期修改均已暂存且无意外未暂存 drift
- [x] 6.4 在任务 1-5 全部完成且任务 6.3 暂存完成后运行 `make lint`；只有命令通过才能标记本任务完成
- [x] 6.5 在 `make lint` 通过后运行 `make verify`，确认最终 `git diff --exit-code` 门禁通过；任一验证未通过或未运行时不得将 change 视为完成
