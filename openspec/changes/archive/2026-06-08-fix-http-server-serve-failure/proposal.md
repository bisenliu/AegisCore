## Why

HTTP server 在 listener 成功绑定后通过 goroutine 执行 `Serve`，但非 `http.ErrServerClosed` 的异常退出目前只记录日志，不会触发 Fx 应用关闭。该行为会导致服务已经不可用但进程仍然存活，Fx 也继续认为应用运行正常。

## What Changes

- 在 `http-service-runtime` 中明确 HTTP server 异步 `Serve` 非预期退出必须触发应用级 shutdown。
- 保持 `net.Listen` 启动失败继续在 `OnStart` 中返回错误。
- 保持正常 graceful shutdown 返回 `http.ErrServerClosed` 时不记录为服务失败。
- 保持现有 CLI、HTTP 路由、响应契约、配置项和数据模型兼容，不新增对外 API 或配置。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `http-service-runtime`: 增加 HTTP server 在异步运行期非预期退出时必须触发 Fx 应用级关闭的运行时要求。

## Impact

- 影响代码：`user-services/internal/bootstrap/server.go` 的 HTTP server lifecycle 处理。
- 可能影响测试：需要覆盖 `Serve` 非预期失败触发 shutdown、正常关闭不误报失败、监听失败仍在启动阶段返回错误。
- 外部兼容性：不改变 HTTP API、错误码、配置格式、数据库 schema 或 CLI 参数。
