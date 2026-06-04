## 1. HTTP Server Lifecycle

- [x] 1.1 更新 `user-services/internal/bootstrap/server.go`，在 Fx `OnStart` 中使用 `net.Listen` 同步绑定配置的 HTTP 地址并返回监听失败错误。
- [x] 1.2 将成功创建的 listener 交给 `server.Serve` 在 goroutine 中处理请求，并保留 `http.ErrServerClosed` 的正常关闭语义。
- [x] 1.3 保持 `OnStop` graceful shutdown timeout 选择和日志行为不变。

## 2. Verification

- [x] 2.1 添加或更新 `user-services/internal/bootstrap` 测试，覆盖端口已占用时 Fx 启动返回错误。
- [x] 2.2 添加或更新测试，覆盖可监听地址启动成功并可通过 Fx stop 正常关闭。
- [x] 2.3 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 2.4 在 `user-services/` 运行 `go test ./...` 验证用户服务模块。
