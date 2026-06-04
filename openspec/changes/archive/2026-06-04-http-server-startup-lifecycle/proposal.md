## Why

当前用户服务 HTTP server 在 Fx `OnStart` 中异步调用 `ListenAndServe` 后立即返回成功；若端口被占用、地址不可绑定或监听失败，错误只会在 goroutine 中记录，Fx 启动流程仍可能成功结束，导致进程看似启动但服务实际不可用。

该风险直接影响 `http-service-runtime` 的启动可用性契约，需要让监听绑定失败在 Fx 启动阶段同步返回，避免部署和运维误判服务健康状态。

## What Changes

- 调整 HTTP server 启动生命周期，使监听端口或地址绑定失败能在 Fx `OnStart` 返回错误。
- 保持正常运行时的异步请求服务模型，成功绑定后继续由 goroutine 执行 HTTP serving。
- 保持优雅关闭行为不变，`http.ErrServerClosed` 仍不记录为服务失败。
- 增加或更新覆盖端口绑定失败同步返回的测试，验证 Fx 启动不会假装成功。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `http-service-runtime`: 补充 HTTP server 启动阶段必须同步暴露监听绑定失败的要求。

## Impact

- 受影响代码：`user-services/internal/bootstrap/server.go` 以及相关测试。
- API 兼容性：HTTP 路由、响应格式、错误码和请求/响应契约不变。
- 配置兼容性：继续使用现有 `http.host`、`http.port`、timeout 和 shutdown timeout 配置，不新增配置项。
- 运行时行为：端口占用或地址不可绑定时，服务启动命令将失败返回，而不是仅异步记录错误后继续运行。
