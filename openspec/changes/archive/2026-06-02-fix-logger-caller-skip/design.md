## Context

`common/logger` 提供共享 Zap logger 与 context API，业务代码通常通过 `logger.Info(ctx, ...)` 输出日志并自动携带 `trace-id`。当前 `common/logger/factory.go` 使用 `zap.New(..., zap.AddCaller())` 启用 caller，但 `common/logger/context.go` 的 `Info`、`Debug`、`Warn`、`Error` 会先调用 `FromContext(ctx)`，再调用底层 `zap.Logger`。

Zap 的 caller 信息来自调用日志方法时的运行时调用栈。启用 `zap.AddCaller()` 后，Zap 会跳过自身内部栈帧并记录第一个外部调用点；当项目封装了 `logger.Info` 时，第一个外部调用点就是 `common/logger/context.go` 的包装函数。因此 `user-services/internal/service/user_service.go` 中的 `logger.Info(ctx, "create user", ...)` 会输出类似 `"caller":"logger/context.go:72"`，而不是业务调用行。

## Goals / Non-Goals

**Goals:**

- 让通过 `common/logger` context API 输出的业务日志记录实际调用点。
- 保留现有业务调用方式，不要求调用方直接使用 `zap.Logger`。
- 保留 `trace-id` 字段、日志格式、分类文件、默认无 stacktrace 等既有日志行为。
- 用测试固定封装 logger 的 caller 行为，避免后续重构回退。

**Non-Goals:**

- 不新增 HTTP API、配置字段、错误码或数据模型。
- 不重写 logger 架构，不引入新的日志依赖。
- 不改变 request logger、中间件日志和基础设施直接使用 `*zap.Logger` 的语义，除非它们通过共享 context API 输出。

## Decisions

- 在共享 logger 边界处理 caller skip。

  业务代码已经以 `common/logger` 作为统一日志入口，caller 修正应作为共享基础设施能力实现。让每个业务调用点改为直接调用 `zap.Logger` 会丢失统一 `trace-id` 注入约定，也会扩散重复代码。

- 优先使用 `zap.AddCallerSkip` 调整封装层栈帧。

  Zap 的 `AddCallerSkip(n)` 是为日志封装场景提供的标准机制。当前 context API 的典型调用链为业务调用点 -> `logger.Info` 包装函数 -> `zap.Logger.Info`，因此需要跳过项目包装层，让 Zap 记录包装层上一层的业务调用点。

- 避免在每次 `FromContext(ctx)` 中重复累加 caller skip。

  `FromContext(ctx)` 和 `WithContext(base, ctx)` 会在每次日志调用时派生 logger 并注入 `trace-id`。如果在这些函数里无条件追加 `AddCallerSkip`，同一个 logger 在 context 中传递或多次包装时可能出现 skip 叠加，导致 caller 跳过过多。实现应在 logger 创建或受控包装点设置固定 skip，并通过测试验证 context logger 和 default logger 两条路径。

- 将规格更新放在 `shared-infrastructure`。

  caller 字段属于共享 Zap 日志能力的一部分，与用户创建或查询业务无关。修复不应改变 `user-services/internal/service` 的业务分层。

## Risks / Trade-offs

- [Risk] caller skip 数值过小仍指向 `common/logger/context.go`，过大则可能指向 service 上游调用者或测试框架。→ Mitigation: 增加针对 `logger.Info(ctx, ...)` 包装调用的测试，断言 caller 包含测试业务调用文件且不包含 `logger/context.go`。
- [Risk] 对已经手动创建并通过 `logger.SetDefault` 或 `logger.ToContext` 注入的 `*zap.Logger`，caller skip 可能未被统一应用。→ Mitigation: 明确测试 default logger 与 context logger 注入路径，根据现有 API 行为决定是在 `NewWithConfig`、`SetDefault` 或专用包装函数中应用 skip。
- [Risk] 修改 caller skip 可能影响基础设施直接使用 Zap logger 的 caller。→ Mitigation: 限定目标为通过 `common/logger` context API 输出的日志，避免改变不经过该 API 的直接 Zap 调用约定。
