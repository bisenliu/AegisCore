## 1. Command Constructor Refactor

- [x] 1.1 审计 `user-service/internal/features/auth/application/command` 中 `LoginDeps`、`RefreshTokenDeps`、`ChangePasswordDeps`、`LogoutCurrentSessionDeps` 和 `LogoutAllSessionsDeps` 的字段、调用点和测试 fixture。
- [x] 1.2 将 `NewLoginUseCase` 改为直接接收 credentials、tokens、sessions 和 metrics，并删除 `LoginDeps`。
- [x] 1.3 将 `NewRefreshTokenUseCase` 改为直接接收 tokens、sessions、metrics 和 `RefreshTokenSettings`，并删除 `RefreshTokenDeps`。
- [x] 1.4 将 `NewChangePasswordUseCase` 改为直接接收 credentials、tokens、sessions 和 metrics，并删除 `ChangePasswordDeps`。
- [x] 1.5 将 `NewLogoutCurrentSessionUseCase` 与 `NewLogoutAllSessionsUseCase` 分别改为直接接收 sessions 和 metrics，并删除对应 `*Deps`。
- [x] 1.6 更新 command 包所有直接构造测试和 fixture，按每个 use case 的最小参数显式传入 mock collaborator 和 metrics。
- [x] 1.7 确认 `user-service/internal/features/auth/application/command` 不导入 `go.uber.org/fx`，且 constructor 参数不包含 `optional` 等 Fx struct tag。

## 2. Auth Fx Composition

- [x] 2.1 修改 `user-service/internal/features/auth/fx.go`，删除 `loginUseCaseParams`、`refreshTokenUseCaseParams`、`changePasswordUseCaseParams`、`logoutCurrentSessionUseCaseParams`、`logoutAllSessionsUseCaseParams` 及五个 `new*UseCase` wrapper。
- [x] 2.2 新增 `newRefreshTokenSettings(*serviceconfig.Config) authcommand.RefreshTokenSettings` provider，只投影 `Auth.RefreshTokenRotation`。
- [x] 2.3 在 auth module 中直接注册五个 application constructor 和 `newRefreshTokenSettings`，保持五个 command use case 的依赖独立声明。
- [x] 2.4 删除五个 command use case Metrics 输入上的 `optional:"true"` 降级路径，确保正式 Fx graph 存在明确的 `authapplication.Metrics` 必选单值输入边。
- [x] 2.5 使用 `fx.Annotate(authcredentials.NewVerifier, fx.From(new(authapplication.UserCredentialStore), new(*password.Service)))` 或经验证等价的 positional annotation 替换 `newCredentialVerifier`。
- [x] 2.6 为 credential verifier 输入映射增加 module graph 测试，验证第一个 `authapplication.UserCredentialStore` 输入未被错误重映射，第二个 `*password.Service` concrete 确实实现并注入 `authcredentials.PasswordService`。

## 3. Redis Session Store And Lifecycle

- [x] 3.1 将 `authredis.SessionStoreParams.Metrics` 改为必选输入，删除 `optional:"true"` tag。
- [x] 3.2 更新全部直接构造 Redis SessionStore 的测试，不观察指标时显式传入 `authapplication.NopMetrics()`。
- [x] 3.3 按 design 决策保留 `metricsRecorder()` 的 nil receiver/field 防御，并确保测试不把该防御当作正式 graph 降级路径。
- [x] 3.4 为 `SessionPurgePoolParams.Redis` 补充 ordering-only 字段注释，说明该依赖用于保证 Fx 逆序停止时先关闭 purge pool、再关闭 Redis。
- [x] 3.5 保留并运行 lifecycle hook 数量与 `purge_pool,redis` stop order 回归测试，防止 Redis ordering-only 依赖被误删。

## 4. Preserved Adapter Coverage

- [x] 4.1 确认 `newAuthSessionLifecycle` 保留完整配置裁剪到 `MaxActiveSessionsPerUser` 标量和 token version invalidator 接线职责。
- [x] 4.2 确认 `newTokenVersionLocalCache` 及其 Params/Result 保留 feature cache 配置解释、enabled/disabled 实现选择、loader 构造、named 多接口输出和 lifecycle close hook。
- [x] 4.3 确认 `newTokenVersionValidator` 及其 Params/Result 保留 named cache 构造 validator、metrics-decorated `commonauth.TokenVersionValidator` 和原始 local invalidator 双输出。
- [x] 4.4 确认 `newAuthMetrics` 保留 enabled/disabled 实现选择、Prometheus collector 构造、注册和错误传播职责。
- [x] 4.5 确认 auth infrastructure 中 named Ent/Redis、named worker pool 和 lifecycle 所需 Fx metadata 未被机械删除。
- [x] 4.6 确认 `authhttp.AuthControllerParams` 未被本 change 顺带重写，并保留正式 controller graph 覆盖。

## 5. Graph And Behavior Tests

- [x] 5.1 新增或扩展正式 auth module graph 测试，populate 五个 command use case、credential verifier、refresh settings 和 Metrics。
- [x] 5.2 覆盖 metrics enabled 配置下 Prometheus recorder 注入，验证五个 command use case 与 Redis SessionStore 成功构图。
- [x] 5.3 覆盖 metrics disabled 配置下 `authapplication.NopMetrics()` 注入，验证 use case 和 SessionStore 不接收 nil Metrics。
- [x] 5.4 覆盖 refresh rotation true/false 配置，验证 `newRefreshTokenSettings` 只投影 `Auth.RefreshTokenRotation`。
- [x] 5.5 覆盖 token validator/local invalidator 双输出，验证 metrics-decorated validator 与原始 local invalidator 两个视图仍可解析。
- [x] 5.6 覆盖正式 controller graph，确认 auth HTTP controller 依赖未因 command constructor 和 Metrics 接线调整而漂移。

## 6. Validation

- [x] 6.1 在 `user-service` module 下运行 `go test -count=1 ./internal/features/auth/... ./internal/providers/... ./internal/bootstrap/...`。
- [x] 6.2 在仓库根目录运行 `make user-service-architecture-lint`。
- [x] 6.3 在仓库根目录运行 `openspec validate simplify-auth-command-composition`。
- [x] 6.4 检查生成物、OpenAPI 和 Fx dependency graph 无 drift；如 API 注解或生成输入发生变更，运行对应生成命令并检查 diff。
- [x] 6.5 暂存本 change 的预期代码、测试、规格和文档后运行 `make lint`。
- [x] 6.6 在预期变更已暂存后运行 `make verify`，确认最终 drift check 不被未暂存的预期变更阻塞。
