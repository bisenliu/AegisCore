## 1. Runtime Implementation

- [x] 1.1 在 `user-services/internal/bootstrap/server.go` 的 `HTTPServerParams` 中注入 `fx.Shutdowner`。
- [x] 1.2 更新 `Serve` goroutine：非 `http.ErrServerClosed` 错误继续记录失败日志，并调用 Fx shutdown 进入应用停止流程。
- [x] 1.3 处理 `Shutdown` 返回错误的日志记录，保留原始 `Serve` 错误上下文且不改变正常关闭路径。

## 2. Verification

- [x] 2.1 添加或更新测试，验证异步 `Serve` 返回非 `http.ErrServerClosed` 错误时触发应用级 shutdown。
- [x] 2.2 验证正常 graceful shutdown 返回 `http.ErrServerClosed` 时不记录为服务失败且不重复触发 shutdown。
- [x] 2.3 验证 HTTP listener 绑定失败仍由 `OnStart` 返回错误，服务不会报告启动成功。

## 3. Quality Checks

- [x] 3.1 对修改的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `user-services/` 运行 `go test ./...`。
