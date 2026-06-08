## Context

用户服务 HTTP runtime 在 `user-services/internal/bootstrap/server.go` 中创建 `http.Server`，并通过 Fx lifecycle 启停。`OnStart` 已经先执行 `net.Listen`，因此端口占用或地址不可绑定会作为启动错误返回；listener 绑定成功后，`server.Serve(listener)` 在 goroutine 中异步处理请求。

当前 goroutine 对非 `http.ErrServerClosed` 错误只记录日志。由于错误发生在 `OnStart` 返回之后，Fx 不会自动感知该失败，进程可能继续存活但 HTTP 服务已经不可用。

## Goals / Non-Goals

**Goals:**

- 在 HTTP server 异步 `Serve` 非预期退出时触发 Fx 应用级 shutdown。
- 保留启动期 `net.Listen` 失败返回错误的现有语义。
- 保留 graceful shutdown 下 `http.ErrServerClosed` 不作为服务失败的语义。
- 保持 HTTP 路由、响应契约、配置、Redis/PostgreSQL/Ent 依赖和 CLI 参数不变。

**Non-Goals:**

- 不新增健康检查聚合、自动重启 HTTP server 或进程 supervisor 能力。
- 不改变 controller/service/repository 分层。
- 不修改数据库 schema、Ent 生成代码或 Atlas migration。
- 不新增外部配置项控制该行为。

## Decisions

- 在 `HTTPServerParams` 中注入 `fx.Shutdowner`，由 `Serve` goroutine 在遇到非 `http.ErrServerClosed` 错误时调用 `Shutdown`。

  选择该方式是因为 Fx 已经是用户服务的应用 lifecycle 管理边界，触发 Fx shutdown 可以复用现有 CLI 停止流程和所有 `OnStop` 清理逻辑。备选方案是在 goroutine 中调用 `os.Exit`，但这会绕过 graceful shutdown 和其他资源释放；另一个备选方案是只增加错误通道给 `OnStart` 等待，但无法处理启动成功后的运行期异常。

- `OnStart` 继续以 listener 绑定成功作为启动成功边界，不额外引入较长阻塞等待。

  当前实现已经解决端口绑定失败不能被发现的问题。短暂等待 `Serve` 是否立即失败可作为补充，但不能替代运行期 shutdown；为了最小化启动时序变化，本变更优先要求 goroutine 非预期退出触发应用关闭。

- 对 `http.ErrServerClosed` 继续直接忽略。

  `server.Shutdown` 正常关闭会导致 `Serve` 返回 `http.ErrServerClosed`。该路径属于预期停止流程，不应记录为失败，也不应再次触发 shutdown。

## Risks / Trade-offs

- [Risk] `Shutdown` 调用失败只记录日志可能掩盖关闭失败原因 → Mitigation: 在实现中对 `Shutdown` 返回错误写入 Zap 错误日志，保留原始 `Serve` 错误上下文。
- [Risk] 异步关闭触发后进程退出时序依赖 CLI 对 Fx app done signal 的处理 → Mitigation: 不改变 CLI lifecycle，使用 Fx 官方 `Shutdowner` 进入现有停止路径。
- [Risk] 测试直接诱发真实 `Serve` 异常较困难 → Mitigation: 优先通过可控 listener、关闭 listener 或可注入场景验证非 `ErrServerClosed` 路径触发 shutdown，同时保留现有监听失败和正常关闭测试。
