## Why

auth feature 的 command use case Fx composition 当前在 application constructor `*Deps` 之外又维护五组重复的 Fx Params 和 wrapper，导致每次新增依赖都必须同步两份清单，增加遗漏和错误接线风险。审计同时发现正式 auth Metrics graph 已经可以由 `auth.Module` 提供唯一的 `authapplication.Metrics`，历史 `optional:"true"` tag 与 credential verifier wrapper 已不再表达真实降级或业务职责。

审计事实如下：

- `loginUseCaseParams`、`changePasswordUseCaseParams`、`logoutCurrentSessionUseCaseParams` 和 `logoutAllSessionsUseCaseParams` 与各自 application `*Deps` 字段重复，只额外携带 `optional:"true"` Metrics tag。
- `refreshTokenUseCaseParams` 同样重复 tokens、sessions 和 metrics，只额外负责从 `*serviceconfig.Config` 提取 `RefreshTokenSettings.RefreshTokenRotation`。
- `newAuthMetrics` 在 metrics 启用时返回 Prometheus recorder，在 provider 禁用或直接调用时传入 nil provider 的边界返回 `authapplication.NopMetrics()`；正式 App graph 本身始终提供 `*commonmetrics.Provider`，因此 `auth.Module` 能够提供唯一的 `authapplication.Metrics`，五个 `optional:"true"` tag 已不是有效的降级机制。
- 缺失 `*commonmetrics.Provider` 时 Fx 仍必须构图失败，不得被解释为自动 no-op。
- 五个 application constructor 当前接收 `LoginDeps`、`RefreshTokenDeps`、`ChangePasswordDeps`、`LogoutCurrentSessionDeps` 和 `LogoutAllSessionsDeps`，composition 又维护对应 Params 和字段映射，新增依赖时存在两份清单同步遗漏风险。
- `newCredentialVerifier` 本身没有业务逻辑；它只因为 `authcredentials.NewVerifier` 消费 `authcredentials.PasswordService` interface、Fx graph 提供 `*password.Service` concrete 而进行输入类型适配。
- `authredis.SessionStoreParams.Metrics` 同样标记为 optional，但 `auth.Module` 在 SessionStore 之前稳定注册 `newAuthMetrics`，且 disabled 模式提供 `NopMetrics()`；正式 graph 不需要把缺失 Metrics 当作降级路径。

## What Changes

- 将五个 auth command constructor 改为直接接收各 use case 的强类型最小依赖，删除五个 `*Deps` 类型；所有依赖仍按 use case 独立声明，不重新引入跨 use case 的共享依赖容器。
- `NewLoginUseCase` 直接接收 credentials、tokens、sessions 和 metrics。
- `NewChangePasswordUseCase` 直接接收 credentials、tokens、sessions 和 metrics。
- `NewLogoutCurrentSessionUseCase` 与 `NewLogoutAllSessionsUseCase` 分别直接接收 sessions 和 metrics。
- `NewRefreshTokenUseCase` 直接接收 tokens、sessions、metrics 和保留的 `RefreshTokenSettings`；application 继续不得读取完整 `*serviceconfig.Config`。
- 在 auth feature composition 中新增一个有真实配置裁剪职责的 `newRefreshTokenSettings(*serviceconfig.Config) authcommand.RefreshTokenSettings` provider，只投影 `Auth.RefreshTokenRotation`，然后直接注册五个 application constructor。
- 删除 `loginUseCaseParams`、`refreshTokenUseCaseParams`、`changePasswordUseCaseParams`、`logoutCurrentSessionUseCaseParams`、`logoutAllSessionsUseCaseParams` 及五个 `new*UseCase` wrapper。
- 将正式 auth Metrics 作为必选单值依赖注入，删除上述 `optional:"true"` tag；metrics 禁用时必须继续由 `newAuthMetrics` 注入 `NopMetrics()`，不得让 use case 在正式 graph 中收到 nil。
- application constructor 是否继续通过 `metricsOrNop` 容忍非 Fx 直接调用传入 nil，必须在 design 中明确；无论选择为何，正式 Fx graph 都必须存在明确 Metrics 输入边，且测试必须覆盖 disabled provider 使用 NopMetrics。
- 将 `authredis.SessionStoreParams.Metrics` 改为必选输入，删除 `optional:"true"`；不观察指标的直接 store 测试显式传入 `authapplication.NopMetrics()`。
- `metricsRecorder()` 是否保留 nil receiver/field 防御由 design 决定，但不得用它掩盖正式 graph 缺边。
- 使用 `fx.Annotate(authcredentials.NewVerifier, fx.From(new(authapplication.UserCredentialStore), new(*password.Service)))` 或经验证等价的 Fx 原生输入映射，让 constructor 的第二个消费侧 `authcredentials.PasswordService` 参数从 graph 中的 `*password.Service` concrete 解析，并删除 `newCredentialVerifier`。
- `fx.From` 是 positional annotation；实现必须显式覆盖并验证两个参数位置，确保第一个 `authapplication.UserCredentialStore` 输入不被错误重映射，第二个 `*password.Service` 确实实现 `authcredentials.PasswordService`。
- 新增或扩展正式 auth module graph 测试，populate 五个 command use case、credential verifier、refresh settings 和 Metrics，验证 enabled/disabled metrics 配置均成功构图。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`：更新 command constructor 最小依赖边界，允许 constructor 使用强类型普通参数而不是保留 `*Deps` 参数结构，同时继续禁止 application/domain 持有 Fx metadata 或完整服务 config。
- `runtime-observability`：增加正式 auth Metrics graph 要求，规定五个 command use case 与 Redis SessionStore 的 Metrics edge 必须是必选单值依赖，metrics 启用时注入 Prometheus recorder，禁用时注入 NopMetrics。

## Impact

- 影响 `user-service/internal/features/auth/application/command` 的五个 command use case constructor、直接构造测试和 fixture。
- 影响 `user-service/internal/features/auth/fx.go` 的 provider 注册、refresh settings 投影、credential verifier 输入映射和 auth module graph 测试。
- 影响 `user-service/internal/features/auth/infrastructure/redis` 的 SessionStore Params、直接构造测试、auth session purge pool 字段注释和 lifecycle stop order 回归测试。
- 影响正式 Fx dependency graph：缺失 `*commonmetrics.Provider` 或 `authapplication.Metrics` 的正式接线必须 fail-fast；metrics disabled 配置仍通过 `newAuthMetrics` 提供 `NopMetrics()`。
- 不影响 HTTP API、OpenAPI、JWT claim、Ent schema、Atlas migration、Redis key、metrics family/label、日志字段、部署资产或配置字段。
