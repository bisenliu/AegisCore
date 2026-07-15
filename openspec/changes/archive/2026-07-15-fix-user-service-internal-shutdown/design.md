## Context

`user-service/cmd/serve.go` 当前使用手动 `Start`/等待/`Stop` 模式：配置加载成功后启动 Fx App，只等待由上游 context、`SIGINT` 或 `SIGTERM` 取消的 context，然后使用未被取消的上游 context value 和配置化停止预算调用 `App.Stop()`。该模式便于通过命令实例局部 factory 测试，但最小 `lifecycleApp` 接口未暴露 `App.Wait()`。

`user-service/internal/bootstrap/server.go` 与 `pprof.go` 已在异步 `Serve` 非预期返回时调用 `fx.Shutdowner`。由于调用未携带 `fx.ExitCode(1)`，且命令层不等待 `fx.ShutdownSignal`，内部故障既不能唤醒 `runServe`，也不能稳定传递失败退出语义。受影响方是 user-service 运维、容器编排和依赖该进程退出码判断健康状态的交付流程。

本 change 只修改 `user-service/cmd` 与 `user-service/internal/bootstrap` 的 Go 生命周期接线，并同步 `runtime-observability`、`delivery-operations` delta specs。`common/` 不新增生命周期 helper，`internal/shared` 与 `internal/integration` 不承载该逻辑；`deployments/` 和其他 `docs/` 不需要修改。

## Goals / Non-Goals

**Goals:**

- 建立“内部 listener/server 故障 -> `fx.Shutdowner` -> `App.Wait()` -> 单次 `App.Stop()` -> Cobra error -> 非零进程退出码”的稳定链路。
- 让外部 context、`SIGINT`、`SIGTERM` 与内部 shutdown signal 共用一次配置化停止预算，并保证竞争时只调用一次 `App.Stop()`。
- 保留手动 `Start`/`Stop` 模式、命令实例局部 app factory 和最小接口测试替身。
- 保留内部非零 exit code 与停止错误的可诊断性，并覆盖命令层和 bootstrap 回归测试。

**Non-Goals:**

- 不改用 `app.Run()`，不在 Cobra 命令内部调用 `os.Exit`，也不改变 `main` 统一把 Cobra error 转为进程退出码的职责。
- 不调整 HTTP/pprof endpoint、监听地址、业务 API、配置字段、数据库 schema、Atlas migration、OpenAPI 生成物、RBAC、认证或安全边界。
- 不修改 `common/`、Docker、Compose、Kubernetes、Helm、Prometheus、Grafana 或终止宽限期。

## Decisions

### Decision: 在最小 App 接口中直接消费 `fx.ShutdownSignal`

`lifecycleApp` 增加与 `*fx.App.Wait` 一致的 `Wait() <-chan fx.ShutdownSignal`。`runServe` 在 `Start` 成功后，同时等待信号 context 和 `app.Wait()`；无论哪个先就绪，都只离开等待阶段一次并进入统一停止阶段。

选择直接使用 Fx 类型，是因为 exit code 是 Fx shutdown 协议的一部分，额外定义服务内信号 DTO 只会增加无业务价值的转换。备选方案 `app.Run()` 会接管信号、Start、Stop 和退出码流程，破坏现有手动生命周期预算与局部 factory 测试模式，因此不采用。仅把 `Wait` 包装成布尔或 error channel 会丢失 `ExitCode`，也不采用。

### Decision: 所有成功启动后的退出来源汇聚到一次 Stop

等待阶段使用单个 `select` 决定一个退出结果，不为每个来源启动独立停止 goroutine，也不由 bootstrap server 直接调用 `App.Stop()`。选择结果后，命令层以 `context.WithoutCancel(upstreamCtx)` 保留上游 value，并叠加 `runtime.lifecycle.stop_timeout` 调用一次 `App.Stop()`。

该结构天然保证外部取消与内部 signal 同时发生时只执行一次 Stop，无需引入跨层 `sync.Once` 或额外协调器。Start 失败继续遵循 Fx 的启动回滚语义并直接返回，不进入成功启动后的等待和停止流程。

### Decision: 内部 exit code 和 Stop error 都由 Cobra error 表达

外部 context、`SIGINT`、`SIGTERM` 或 exit code 为 `0` 的内部 shutdown，在 Stop 成功时返回 `nil`。`fx.ShutdownSignal.ExitCode` 非零时，命令层在 Stop 完成后返回包含该 code 的错误；Stop 自身失败时也必须返回错误。两者同时发生时使用可保留两项原因的组合错误语义，不能让 Stop error 覆盖内部故障，也不能因内部 exit code 丢弃关闭失败。

保持 `main` 现有 `Execute` error -> `os.Exit(1)` 入口，避免 Cobra 命令产生不可测试的进程级副作用。此设计不承诺把任意 Fx exit code 原样映射为同值 OS exit code；当前稳定契约是非零 shutdown code 最终产生非零进程退出码。

### Decision: HTTP 与 pprof 非预期 Serve 退出统一发送失败 signal

HTTP 和 pprof 的异步 `Serve` 只有在错误代表非预期 listener/server 故障时调用 `Shutdown(fx.ExitCode(1))`。`http.ErrServerClosed` 以及能由已进入正常生命周期停止状态证明的关闭错误继续视为正常结束，不触发故障 signal。

HTTP 继续使用其 lifecycle context 区分主动 listener 关闭；pprof 的正常 `Server.Shutdown` 以 `http.ErrServerClosed` 结束，不能无条件把独立 listener 的 `net.ErrClosed` 当作正常退出，否则外部或意外 listener 关闭仍会被吞掉。Shutdown 调用失败继续记录英文 log message 和结构化 error，不在 bootstrap goroutine 中直接 Stop 或退出进程。

### Decision: 以分层回归测试验证退出协议

命令层使用实现最小接口的测试替身和可控 channel 覆盖外部取消、内部零/非零 exit code、Stop error 以及并发竞争，断言 Stop 次数和 context 预算。bootstrap 测试记录传入 `fx.ShutdownOption` 后解析出的 signal，断言 HTTP/pprof 非预期故障携带 exit code `1`，正常关闭不触发 shutdown。

不为测试增加正式构建中的 test-only hook；测试复用当前局部 factory、辅助 listener 和同包可见 helper。

## Risks / Trade-offs

- [Risk] 外部 context 与 Fx signal 同时就绪时，`select` 的来源不确定 -> Mitigation：两条正常路径共享相同 Stop 语义，非零内部 signal 若恰与外部终止竞争可能未被选中；测试固定核心不变量为不重复 Stop、不死锁，运行时 listener 故障通常先发出 signal 并立即唤醒等待者。
- [Risk] Stop error 与内部 exit code 组合后错误文本变化 -> Mitigation：测试通过错误包含和 cause 语义验证关键信息，不把完整字符串作为外部协议。
- [Risk] pprof 对 `net.ErrClosed` 的分类收紧可能把此前静默的 listener 关闭报告为故障 -> Mitigation：正常 `Server.Shutdown` 仍按 `http.ErrServerClosed` 过滤，并新增正常关闭与意外 listener 关闭的对照测试。
- [Risk] Fx 与 `signal.NotifyContext` 都观察 OS signal -> Mitigation：命令层只从单个 `select` 进入一次 Stop；两种观察结果对外部信号均对应正常退出。

## Migration Plan

1. 先扩展命令层最小接口与测试替身，加入双来源等待、单次停止和错误合并测试。
2. 再将 HTTP/pprof 故障 shutdown 改为携带 `fx.ExitCode(1)`，补齐 option 与正常关闭分类测试。
3. 执行 `cd user-service && go test ./cmd ./internal/bootstrap -count=1`、OpenSpec 校验和架构 lint；实现与规格任务完成并暂存预期变更后执行 `make lint` 与 `make verify`。
4. 按现有 user-service 发布流程构建和部署，无数据迁移、配置迁移或清单变更。

回滚时整体恢复命令层 `Wait` 接线和 bootstrap exit code 选项即可；不涉及持久化数据、API 兼容或部署资源回滚。回滚会重新暴露内部故障不能终止进程的原问题，因此只用于处理新实现引发的阻断性回归。

## Open Questions

无。非零 Fx exit code 统一转换为 Cobra error，并由现有 `main` 输出进程退出码 `1`，不在本 change 引入任意 exit code 的原样透传。
