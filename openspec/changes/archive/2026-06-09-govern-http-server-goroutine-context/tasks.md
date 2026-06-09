## 1. 实现

- [x] 1.1 调整 `user-services/internal/bootstrap/server.go` 的 HTTP server lifecycle，为 `Serve` goroutine 创建并传入可取消 context 或等价停止信号。
- [x] 1.2 在 HTTP server `OnStop` 中先触发 goroutine lifecycle 取消，再保留现有 `http.shutdown_timeout`/`defaultHTTPShutdownTimeout` graceful shutdown 逻辑。
- [x] 1.3 保持 `net.Listen` 同步绑定失败返回、`http.ErrServerClosed` 非失败处理和非预期 `Serve` 错误触发 `fx.Shutdowner` 的现有语义。
- [x] 1.4 为 HTTP server goroutine 的正常退出、context 取消退出和异常退出补充清晰日志或等价可观测行为，避免正常停止使用 Error 级别误报。

## 2. 测试与验证

- [x] 2.1 更新 `user-services/internal/bootstrap/http_test.go`，覆盖 `OnStop` 会取消 HTTP server goroutine lifecycle context，且 goroutine 可在有超时的等待中退出。
- [x] 2.2 保留并按需增强现有测试，验证正常 `http.ErrServerClosed` 不触发 shutdown、不输出失败日志。
- [x] 2.3 保留并按需增强现有测试，验证非预期 `Serve` 错误仍记录失败并触发 Fx 应用级 shutdown。
- [x] 2.4 运行 `gofmt -w` 格式化修改过的 Go 文件。
- [x] 2.5 在 `user-services/` 运行 `go test ./...` 验证用户服务模块。
