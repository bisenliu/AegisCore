## 1. 实现 pprof 停止兜底

- [x] 1.1 更新 `user-service/internal/bootstrap/pprof.go` 的 pprof `OnStop`，在 `server.Shutdown(ctx)` 成功时保持原有返回语义。
- [x] 1.2 在 `server.Shutdown(ctx)` 返回错误时调用同一 `server` 的 best-effort `server.Close()`，并用 `errors.Join` 合并 `shutdown pprof server` 包装错误和 `Close` 错误。
- [x] 1.3 确认 pprof 未启用、监听地址校验、生产类环境 loopback 限制、`Serve` 非预期失败触发 Fx shutdown signal 的现有行为不变。

## 2. 测试覆盖

- [x] 2.1 为 pprof 停止路径新增或更新测试，覆盖已取消 context 或极短 deadline 下 `Shutdown` 失败后会执行强制关闭。
- [x] 2.2 覆盖 listener 关闭后 `Serve` goroutine 能退出，且正常关闭导致的 `http.ErrServerClosed` 不触发非零内部 shutdown signal。
- [x] 2.3 覆盖重复停止或已关闭 server 的停止路径，确认不会 panic、不会阻塞，并保留可诊断错误语义。

## 3. 验证与收尾

- [x] 3.1 运行 pprof/bootstrap 相关 Go 测试，例如 `go test ./internal/bootstrap`。
- [x] 3.2 运行 `make user-service-architecture-lint`，确认 OpenSpec 和架构边界变更可通过检查。
- [x] 3.3 暂存本次预期代码、测试和 OpenSpec artifacts 后运行 `make lint`。
- [x] 3.4 在预期变更已暂存的状态下运行 `make verify`，确认无测试、lint、生成物或 drift 失败。
