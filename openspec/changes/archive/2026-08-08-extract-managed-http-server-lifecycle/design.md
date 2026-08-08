## Context

`user-service/internal/bootstrap/server.go` 当前同时承担服务配置映射、Fx hook、listener 创建、`Serve` goroutine、异常分类、强制关闭和 Gin handler drain；`user-service/internal/bootstrap/pprof.go` 又维护一套较简单但行为不同的实现。业务 server 可以等待 handler，但第一次 Fx stop context 到期后无法可靠地由后续调用继续等待；pprof 则缺少相同的 drain 与统一异常分类。这些逻辑只依赖标准库，适合作为跨服务稳定 runtime primitive。

变更影响 `common/runtime/httpserver/`、`user-service/internal/bootstrap/`、`docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 与两个 OpenSpec capability。它不改变 HTTP 路由、OpenAPI、数据库、部署端口、观测资产或安全边界，也不进入 `common/http`、`common/runtime/observability`、`user-service/internal/shared` 或 `user-service/internal/integration`。

## Goals / Non-Goals

**Goals:**

- 提供严格、业务中立且可复用的 `net/http` server 生命周期，覆盖同步监听、异步服务、异常退出通知、优雅关闭、强制关闭、handler drain 和并发重复停止。
- 用显式状态机保证不可重启、单次异常回调和单一 cleanup owner，并使调用方等待 context 与后台 cleanup 生命周期解耦。
- 将业务 HTTP 与 pprof 一次性迁移到两个独立 `Managed` 实例，使 bootstrap 只保留配置映射、Fx hook、enabled 状态和服务日志策略。
- 通过 race test、bootstrap lifecycle test、架构 lint、lint 与全量验证证明没有 listener、goroutine、旧 helper 或生成物 drift。

**Non-Goals:**

- 不管理 Gin 路由、Fx App、服务配置默认值、结构化日志字段、进程信号或部署负载均衡 drain。
- 不处理 hijacked connection、WebSocket 或无法由 Go 强制终止的 handler goroutine。
- 不改变 HTTP API、OpenAPI 生成物、Ent schema、Atlas migration、RBAC、认证授权、部署清单或 Prometheus/Grafana 资产。
- 不保留旧 helper、兼容 wrapper、type alias、feature flag 或新旧双实现。

## Decisions

### Decision: 以最小公开 API 表达生命周期所有权

`common/runtime/httpserver` 公开 `Options`、`Managed`、`New`、`Start`、`Stop`、`HTTPServer` 以及 `ErrInvalidOptions`、`ErrAlreadyStarted`、`ErrStopped`。`Options` 严格采用调用方提供的 `Name`、`Addr`、`Handler`、read/write/idle timeout、`ShutdownTimeout` 和可选 `OnServeError`，不提供默认值；`New` 只校验和构造，不监听、不创建 goroutine。错误包装包含 server name、addr 和操作，并支持 `errors.Is` 匹配稳定 sentinel。

备选方案是把该能力放入 `common/http` 或直接提供 Fx module。前者混淆 HTTP helper 与 runtime 资源所有权，后者会把 DI framework 和服务接线策略带入核心，因此拒绝。

### Decision: 用受锁状态机协调 Start、Serve 与 Stop

内部状态固定为 `created -> running -> stopping -> stopped`，异常 `Serve` 可先从 `running` 进入 `failed`，再由 `Stop` 进入 `stopping -> stopped`。`Start` 仅接受 `created`：运行或失败后重复调用返回包装 `ErrAlreadyStarted`，停止开始或完成后返回包装 `ErrStopped`，不支持重启。

`Start` 通过 `net.ListenConfig.Listen` 同步绑定，绑定成功并保存 listener 后才启动唯一 `Serve` goroutine。监听失败不改变为已启动状态、不留下后台资源且不调用 `OnServeError`。实现保留包内私有 listener factory seam 供同包测试注入异常 listener，不把测试控制面暴露为生产 API。

备选方案是直接调用 `http.Server.ListenAndServe`，但它无法在 `Start` 返回前可靠地区分绑定成功与后台失败；允许 restart 又会使 `http.Server`、tracker 和完成 channel 的所有权显著复杂化，因此均拒绝。

### Decision: 明确区分正常 Serve 退出与故障回调

停止流程导致的 `http.ErrServerClosed`、`net.ErrClosed` 或 context cancellation 视为正常退出。其他 `Serve` 错误被保存并将状态置为 `failed`，`OnServeError` 恰好调用一次；回调复制后在锁外执行，使回调能够安全触发 `fx.Shutdowner.Shutdown(fx.ExitCode(1))`，并允许 Fx 随后调用同一实例的 `Stop`。

核心包不记录日志。user-service 的业务与 pprof adapter 分别在回调中记录稳定的英文故障消息，并触发 exit code 1；正常停止不触发全局 shutdown。

### Decision: 第一次 Stop 只启动 cleanup，调用方 context 只控制等待

第一次 `Stop` 成为唯一 cleanup owner；其他并发或后续调用都等待同一个完成 channel。若状态仍为 `created`，它直接切换为 `stopped` 并返回 nil。对已启动实例，cleanup 使用 `context.Background()` 与 `Options.ShutdownTimeout` 创建独立 context，依次执行：

1. 切换到 `stopping` 并调用 `http.Server.Shutdown`。
2. 无论 `Shutdown` 结果如何都显式关闭持有的 listener，覆盖 Start/Stop 紧邻发生的竞态。
3. `Shutdown` 失败时调用 `http.Server.Close` 强制关闭活跃连接。
4. 使用同一内部 cleanup context 等待 drain tracker，并等待 `Serve` goroutine 退出。
5. 用 `errors.Join` 保存 shutdown、listener close、forced close、drain 和非预期 serve 错误，切换为 `stopped` 并只关闭一次完成 channel。

每次 `Stop(ctx)` 只在 cleanup 完成和调用方 `ctx.Done()` 间选择；调用方超时返回 `ctx.Err()`，后台 cleanup 继续，后续调用仍能等待并在完成后取得同一个最终结果。这避免 Fx 的第一次 stop budget 用尽后留下无法继续收敛的半关闭对象。

备选方案是直接把首次调用方 context 传入 `Shutdown`。该方案会让任一调用方取消共享 cleanup，并使后续 Stop 无法恢复等待，故拒绝。

### Decision: handler tracker 只承诺进程内请求 drain

`HTTPServer().Handler` 始终是内部 `drainTracker`，真实 handler 在进入前增加 active，并通过 defer 在正常返回或 panic unwind 时减少 active；active 归零时广播条件变量。`Wait(ctx)` 使用 context 唤醒条件变量，不为每次等待创建永久 goroutine。tracker 只覆盖已经进入 handler 的请求；强制 `Close` 取消请求 context 后仍忽略取消的 handler 会使 drain wait 返回内部 timeout，并将该错误保留在最终结果中。

不尝试管理 hijacked connection、WebSocket、外部负载均衡器或强杀 goroutine，这些能力需要更高层协议与部署协作。

### Decision: user-service 使用 enabled composition DTO 和薄 Fx hook

业务侧保留 `HTTPRuntime`、pprof 侧保留 `PprofRuntime` 这类仅表达 `Enabled` 与可选 `*httpserver.Managed` 的 composition DTO。disabled 时不调用 `httpserver.New`、不注册 hook、不监听；enabled 时 constructor 映射配置并注册 OnStart/OnStop，hook 直接调用 `Managed.Start(ctx)` 与 `Managed.Stop(ctx)`。`registerRuntimeServers` 显式解析两个 DTO，bootstrap 不再拥有 listener、Serve、Shutdown、Close 或 drain 状态机。

业务 server 从 `server.http` 映射地址与全部 timeout，handler 为 Gin engine。pprof 使用 `common/http/pprof.Handler`、从 `observability.pprof` 读取 enabled 和 addr，并由 composition 显式选择现有 `server.http.shutdown_timeout` 作为其内部 shutdown timeout；核心包不回退到业务默认值。两者各自调用 `httpserver.New`，不得共享 `Managed`。

服务启动日志中的 service、environment、timezone 与 component 字段继续留在 bootstrap。配置层既有 `runtime.lifecycle.stop_timeout` 校验保证总预算至少覆盖 `server.http.shutdown_timeout`，因此同时覆盖两个 `Managed` 的内部 timeout。

## Risks / Trade-offs

- [Risk] `ShutdownTimeout` 同时约束 graceful shutdown 与 tracker wait，忽略取消的 handler 可能在 cleanup 完成后继续运行 -> Mitigation：强制 `Close`、在最终错误中保留 drain timeout，并明确不承诺强杀 goroutine。
- [Risk] 异常 `Serve` 与外部 Stop 同时发生可能重复分类或回调 -> Mitigation：所有状态转换和 callback once 由同一锁保护，回调在锁外执行，race test 覆盖交错。
- [Risk] listener 显式关闭可能产生重复关闭错误 -> Mitigation：将 `http.ErrServerClosed`、`net.ErrClosed` 和停止期间的 context cancellation 归为预期错误，只保留真正异常。
- [Risk] pprof 复用业务 HTTP shutdown timeout 会保持两个实例预算一致，无法单独调优 -> Mitigation：当前配置没有独立 pprof timeout 且需求不引入新配置；未来若出现真实运维需求，再通过独立 OpenSpec change 扩展配置。
- [Risk] 一次性删除 bootstrap 私有 helper 会使旧测试无法编译 -> Mitigation：先在 common 覆盖状态机与并发语义，再把服务测试迁移到公开 `Managed` 行为和 Fx hook，最后用 `rg` 与架构 lint 检查无旧符号。

## Migration Plan

1. 新增 `common/runtime/httpserver`、单元测试、race test 和示例，先独立验证公开契约与资源回收。
2. 一次性迁移业务 HTTP adapter，再迁移 pprof adapter与 runtime graph 显式绑定；同一提交删除所有旧 helper 与旧测试入口，不保留双实现窗口。
3. 更新架构文档、能力地图和规格，执行 common 与 bootstrap 定向 race 测试、架构 lint、`make lint` 和 `make verify`。
4. 发布不需要数据库迁移、OpenAPI 生成或部署清单变更。回滚只能整体回退代码与文档变更；由于没有外部 API、持久化格式或配置 schema 变化，不需要数据回滚。

## Open Questions

无。
