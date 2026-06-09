## Why

当前 `http-service-runtime` 已能通过 Fx `OnStop` 调用 `server.Shutdown` 收敛 HTTP server，但 `OnStart` 中异步启动的 `Serve` goroutine 没有在规格层明确受可取消 context 托管。这会让现有实现依赖间接关闭路径，也给后续在 goroutine 中加入循环、重试或多资源监听时留下泄露风险。

## What Changes

- 明确 HTTP server 的异步 `Serve` goroutine 属于 Fx lifecycle 托管资源，必须由可取消 context 控制退出边界。
- 在 HTTP server lifecycle 中补充显式取消路径、退出日志和错误处理约定，同时保留现有 `net.Listen` 同步绑定、`server.Shutdown` graceful shutdown 和 `http.ErrServerClosed` 非失败语义。
- 补充测试要求，覆盖 goroutine 可随 lifecycle context 取消退出，以及未来新增异步 HTTP runtime goroutine 必须具备可取消上下文。
- 不改变外部 HTTP API、响应信封、配置格式、数据库结构或 Redis/PostgreSQL 运行时依赖声明。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `http-service-runtime`: 增加 HTTP server 异步 goroutine 必须受可取消 context 托管并可观测退出的运行时要求。

## Impact

- 影响代码：`user-services/internal/bootstrap/server.go` 及相关 HTTP lifecycle 测试。
- 影响规格：`openspec/specs/http-service-runtime/spec.md` 的 shutdown/runtime lifecycle 要求。
- 外部兼容性：不改变路由、HTTP 响应、错误码、配置键、数据模型或依赖拓扑。
- 测试影响：需要更新或新增用户服务 bootstrap 单元测试，验证正常关闭、意外 `Serve` 错误和 context 取消路径不会互相误报。
