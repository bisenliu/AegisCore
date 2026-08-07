## ADDED Requirements

### Requirement: 业务中立的 managed HTTP server 生命周期

系统 MUST 在 `common/runtime/httpserver` 提供不依赖 Gin、Fx、服务私有配置或业务日志字段的 managed `net/http` server。公开 API MUST 包含严格的 `Options`、`Managed`、`New`、`Start`、`Stop`、`HTTPServer` 以及可匹配的 `ErrInvalidOptions`、`ErrAlreadyStarted` 和 `ErrStopped`；启用策略、地址来源、默认 timeout、日志和进程 shutdown 策略 MUST 由调用服务拥有。

#### Scenario: 构造严格校验且无运行时副作用

- **WHEN** 调用方执行 `New(options)`
- **THEN** `Name`、`Addr` 和 `Handler` MUST 非空，`ShutdownTimeout` MUST 大于零，read/write/idle timeout MUST 不小于零
- **AND** 任一字段非法时错误 MUST 包装 `ErrInvalidOptions` 并包含 server name、addr 与具体字段或操作
- **AND** `Options` MUST NOT 提供默认值，`New` MUST NOT 监听端口或创建 goroutine
- **WHEN** 构造成功
- **THEN** `HTTPServer()` MUST 返回 addr 与 timeout 均匹配 options 的 `*http.Server`，其 `Handler` MUST 是内部 drain tracker 而不是原始 handler

#### Scenario: 同步启动与不可重启状态机

- **WHEN** created 状态的实例执行 `Start(ctx)`
- **THEN** 系统 MUST 使用 `net.ListenConfig.Listen` 同步绑定 `Addr`，成功绑定并保存 listener 后才启动异步 `Serve`
- **WHEN** 地址被占用或监听失败
- **THEN** `Start` MUST 直接返回包含 server name、addr 与 listen 操作的错误，MUST NOT 留下 listener 或 goroutine，也 MUST NOT 调用 `OnServeError`
- **WHEN** `Start` 成功返回
- **THEN** 地址 MUST 已经可以接受连接，状态 MUST 为 running
- **WHEN** running 或 failed 状态再次调用 `Start`
- **THEN** 系统 MUST 返回包装 `ErrAlreadyStarted` 的稳定错误
- **WHEN** stopping 或 stopped 状态调用 `Start`
- **THEN** 系统 MUST 返回包装 `ErrStopped` 的稳定错误且 MUST NOT 重启

#### Scenario: Serve 退出分类与故障回调

- **WHEN** `Serve` 因 `http.ErrServerClosed`、`net.ErrClosed` 或停止期间的 context cancellation 返回
- **THEN** 系统 MUST 将其视为正常退出且 MUST NOT 调用 `OnServeError`
- **WHEN** `Serve` 返回其他错误
- **THEN** 状态 MUST 从 running 进入 failed，错误 MUST 被保存，`OnServeError` MUST 恰好调用一次
- **AND** 调用 `OnServeError` 时 MUST NOT 持有内部锁，callback MUST 能安全触发调用方 shutdown

#### Scenario: 单一后台 cleanup 与调用方独立等待

- **WHEN** created 状态首次调用 `Stop(ctx)`
- **THEN** 系统 MUST 直接进入 stopped 并返回 nil
- **WHEN** running 或 failed 状态首次调用 `Stop(ctx)`
- **THEN** 系统 MUST 只启动一个使用 `context.Background()` 和 `ShutdownTimeout` 的后台 cleanup，并切换为 stopping
- **AND** cleanup MUST 调用 `http.Server.Shutdown`，无论其是否识别到 listener 都 MUST 显式关闭 Managed 持有的 listener
- **WHEN** `Shutdown` 失败
- **THEN** cleanup MUST 调用 `http.Server.Close` 强制关闭活跃连接
- **AND** cleanup MUST 等待 drain tracker 与 `Serve` goroutine，使用 `errors.Join` 保存最终错误，切换为 stopped 并只关闭一次完成 channel
- **WHEN** 多个 goroutine 并发或重复调用 `Stop`
- **THEN** 所有调用 MUST 共享同一 cleanup 与最终结果，MUST NOT 重复关闭、panic 或永久阻塞
- **WHEN** 某次 `Stop(ctx)` 的调用方 context 在 cleanup 完成前取消或超时
- **THEN** 本次调用 MUST 返回 `ctx.Err()`，后台 cleanup MUST 继续，后续 `Stop` MUST 能继续等待并取得同一个最终结果

#### Scenario: 已进入 handler 的请求 drain

- **WHEN** 请求进入真实 handler
- **THEN** drain tracker MUST 在调用前增加 active，并通过 defer 在 handler 返回或 panic unwind 时减少 active，active 归零时 MUST 唤醒等待者
- **WHEN** `Wait(ctx)` 等待期间 context 取消或超时
- **THEN** wait MUST 被唤醒并返回对应 context error，MUST NOT 泄漏永久等待 goroutine
- **WHEN** 慢 handler 在 shutdown timeout 内完成
- **THEN** `Stop` MUST 等待其完成并优雅退出
- **WHEN** handler 超过 shutdown timeout
- **THEN** 系统 MUST 强制关闭连接；handler 仍忽略取消时 drain timeout MUST 保留到最终错误
- **AND** tracker MUST 只代表已进入 handler 的请求，MUST NOT 声称管理 hijacked connection、WebSocket、进程外负载均衡 drain 或强杀 goroutine
