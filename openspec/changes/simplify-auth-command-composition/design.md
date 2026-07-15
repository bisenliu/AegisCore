## Context

本变更聚焦 user-service auth feature 的 command use case 与 infrastructure adapter 在 Fx composition 中的依赖表达。当前五个 command use case 已在 application 包内通过 `LoginDeps`、`RefreshTokenDeps`、`ChangePasswordDeps`、`LogoutCurrentSessionDeps` 和 `LogoutAllSessionsDeps` 声明最小 collaborator，但 composition root 又维护五组几乎相同的 Fx Params 和 wrapper；这些 wrapper 只做字段搬运、可选 Metrics 降级或从完整配置中取出 refresh rotation。

正式 App graph 已由服务级 providers 稳定提供 `*commonmetrics.Provider`，auth 模块通过 `newAuthMetrics` 在 metrics enabled 时注册 Prometheus recorder，在 metrics disabled 时提供 `authapplication.NopMetrics()`。因此 Metrics 的正式降级语义应由 `newAuthMetrics` 内部表达，而不是让五个 use case 或 Redis SessionStore 通过 `optional:"true"` 将 graph 缺边解释为 nil/no-op。

auth composition 中仍存在若干有真实职责的 adapter：配置裁剪、feature cache 配置解释、named 多输出、lifecycle hook、Prometheus collector 注册、named Ent/Redis/worker pool metadata 和 HTTP controller 多依赖参数对象。这些结构不是本变更要机械删除的目标。

## Goals / Non-Goals

**Goals:**

- 删除五个与 application constructor deps 重复的 auth command Fx Params 和 wrapper，直接注册五个 command use case constructor。
- 保留 use case 独立依赖声明，避免重新引入跨 use case 的共享 command 依赖容器。
- 将 refresh token rotation 的配置裁剪集中到 `newRefreshTokenSettings(*serviceconfig.Config) authcommand.RefreshTokenSettings`。
- 将五个 command use case 与 Redis SessionStore 的 `authapplication.Metrics` 输入收紧为正式 graph 必选单值依赖。
- 使用 Fx 原生 positional `fx.From` 适配 `authcredentials.NewVerifier` 的第二个 `authcredentials.PasswordService` 参数，删除无业务逻辑的 `newCredentialVerifier`。
- 保留有真实配置、缓存、lifecycle、多输出、观测注册或 named resource metadata 职责的 adapter。

**Non-Goals:**

- 不改变登录、refresh rotation、强制改密、logout、session 上限、token version、缓存失效或错误映射行为。
- 不改变 JWT claim、HTTP API、OpenAPI、Ent schema、Atlas migration、Redis key、metrics family/label、日志字段、部署资产或配置字段。
- 不合并五个 use case，不创建共享 command service，不把 auth DTO、port 或 settings 移到 `common` 或 `internal/shared`。
- 不用 package-level config、service locator、全局 metrics 或 no-op provider 隐藏正式 graph 缺失。
- 不删除 `newAuthSessionLifecycle`，因为它把完整服务配置裁剪为 `MaxActiveSessionsPerUser` 标量，并连接 token version invalidator；除非后续独立 change 设计更窄且更清晰的 settings，否则不得把完整 config 下沉到 application。
- 不删除 `newTokenVersionLocalCache` 及其 Params/Result，因为它执行 feature cache 配置解释、enabled/disabled 实现选择、loader 构造、named 多接口输出和 lifecycle close hook。
- 不删除 `newTokenVersionValidator` 及其 Params/Result，因为它从 named cache 构造 validator，同时输出 metrics-decorated `commonauth.TokenVersionValidator` 和原始 local invalidator 两个不同视图。
- 不删除 `newAuthMetrics`，因为它负责 enabled/disabled 实现选择、Prometheus collector 构造、注册和错误传播。
- 不删除 auth infrastructure 中 named Ent/Redis、named worker pool 和 lifecycle 所需的真实 Fx metadata。
- 不删除 `SessionPurgePoolParams.Redis`；该依赖用于建立 lifecycle hook 注册顺序，使 Fx 逆序停止时先关闭 purge pool、再关闭 Redis。
- 不重写 `authhttp.AuthControllerParams`，除非后续独立审计证明其多依赖参数对象造成重复 dependency list。

## Decisions

- 决策：五个 command constructor 改为强类型普通参数，而不是继续接收 `*Deps` 参数结构。
  理由：每个 constructor 的依赖清单已经稳定且较短，普通参数能让 Fx graph 直接表达真实输入边，避免 application deps struct 与 composition Params 的双清单同步。use case struct 仍只保存自身真实 collaborator，且不会引入 Fx metadata。
  备选方案：保留 `*Deps` 并删除 composition wrapper。该方案仍要求 application 暴露只服务于构造的聚合结构，不能彻底消除新增依赖时维护两层清单的风险。

- 决策：`NewLoginUseCase` 直接接收 credentials、tokens、sessions 和 metrics；`NewChangePasswordUseCase` 直接接收 credentials、tokens、sessions 和 metrics；两个 logout constructor 分别直接接收 sessions 和 metrics。
  理由：这些参数就是 use case 的真实 collaborator，直接注入不会扩大依赖面，也不会让某个 use case 获得无关 collaborator。
  备选方案：创建一个跨 command 的共享依赖容器。该方案会回到旧的隐藏依赖面问题，违反最小依赖边界。

- 决策：`NewRefreshTokenUseCase` 直接接收 tokens、sessions、metrics 和 `RefreshTokenSettings`，并由 composition root 新增 `newRefreshTokenSettings` 投影 `Auth.RefreshTokenRotation`。
  理由：refresh use case 需要的是一个窄布尔策略，不应读取完整 `*serviceconfig.Config`。单独 provider 有真实配置裁剪职责，因此可以保留在 composition root。
  备选方案：让 application constructor 接收完整 config。该方案会破坏 application/config 边界并扩大测试 fixture。备选方案：把 refresh rotation 作为裸 bool 注入。该方案可行但可读性弱于保留 `RefreshTokenSettings`，且未来窄 refresh settings 扩展时不够明确。

- 决策：正式 auth Metrics 输入必须是必选单值依赖，metrics disabled 由 `newAuthMetrics` 返回 `authapplication.NopMetrics()` 表达。
  理由：正式 graph 始终提供 `*commonmetrics.Provider` 和唯一 `authapplication.Metrics`，缺失 provider 是 composition 缺陷，应 fail-fast。`optional:"true"` 会把接线错误伪装成 no-op 降级。
  备选方案：继续使用 optional Metrics 并在 use case 内兜底。该方案降低 graph 可观测性，无法区分显式 disabled 与 provider 缺失。

- 决策：application constructor 继续保留 `metricsOrNop` 的 nil 容忍，仅作为非 Fx 直接调用和历史测试的防御，不作为正式 graph 降级机制。
  理由：这能降低直接构造测试迁移风险，同时正式 graph 测试必须 populate Metrics 并验证 disabled provider 注入 NopMetrics。代码注释和测试应明确 nil 容忍不是 DI contract。
  备选方案：完全移除 nil 容忍并 panic 或返回错误。该方案让 constructor contract 更严格，但会扩大直接调用点改动，并且 constructor 当前并非返回 error 的形态。

- 决策：`authredis.SessionStoreParams.Metrics` 删除 `optional:"true"`，直接 store 测试显式传入 `authapplication.NopMetrics()`。
  理由：SessionStore 在正式 auth.Module 中位于 `newAuthMetrics` 之后，缺失 Metrics 不应是降级路径。直接测试不观察指标时显式传入 Nop 更清楚。
  备选方案：保留 optional 并依赖 `metricsRecorder()` 防御。该方案会继续掩盖正式 graph 缺边。

- 决策：保留 `metricsRecorder()` 的 nil receiver/field 防御，但只作为防御式实现和直接构造兜底，不作为正式 DI 语义。
  理由：Redis store 的方法可能被直接测试或未来局部构造，防御可避免无指标路径 panic；正式 graph 是否完整由 Fx graph 测试和 Params 必选输入保证。
  备选方案：删除所有 nil 防御。该方案更严格，但会把非 Fx 局部构造错误变成运行时 panic，不是本变更必要收益。

- 决策：使用 `fx.Annotate(authcredentials.NewVerifier, fx.From(new(authapplication.UserCredentialStore), new(*password.Service)))` 或等价 positional annotation 替换 `newCredentialVerifier`。
  理由：`newCredentialVerifier` 只做 concrete-to-interface 输入适配，没有业务逻辑。`fx.From` 能在注册处明确参数位置：第一个参数保持 `authapplication.UserCredentialStore`，第二个参数从 graph 中的 `*password.Service` concrete 解析为 `authcredentials.PasswordService`。
  备选方案：保留 wrapper。该方案继续隐藏真实 constructor，并保留无语义函数。备选方案：让 `authcredentials.NewVerifier` 直接依赖 `*password.Service`。该方案会让 application/credentials 依赖 concrete security service，扩大边界。

- 决策：保留 `SessionPurgePoolParams.Redis` 并补充字段注释。
  理由：虽然 purge pool constructor 不读取 Redis client，该依赖用于让 Fx 建立 lifecycle hook 注册顺序，使停止时先关闭 purge pool、再关闭 Redis。必须保留 `purge_pool,redis` stop order 回归测试。
  备选方案：删除未读取字段。该方案会丢失 stop order 约束，可能让 Redis 先关闭导致 purge pool 停止阶段仍访问已关闭资源。

## Risks / Trade-offs

- 风险：`fx.From` positional annotation 参数顺序写错，导致 credential store 被错误重映射或 password service 无法解析。缓解：module graph 测试 populate credential verifier，并断言 `*password.Service` concrete 实现 `authcredentials.PasswordService`，同时覆盖两个参数位置。
- 风险：删除 wrapper 后某个 use case 依赖遗漏。缓解：正式 auth module graph 测试 populate 五个 command use case、refresh settings 和 Metrics；直接构造测试按 use case 独立依赖更新。
- 风险：Metrics 从 optional 改为必选后，局部测试 graph 未注册 `newAuthMetrics` 会失败。缓解：直接构造测试显式传入 `authapplication.NopMetrics()`；正式 graph 测试覆盖 enabled/disabled 配置。
- 风险：误删有真实职责的 adapter。缓解：tasks 中显式列出必须保留的 adapter 和回归测试，包括 token cache 双输出、session lifecycle、auth metrics、named infrastructure metadata、purge pool stop order 和 controller graph。
- 取舍：保留 `metricsOrNop` 与 `metricsRecorder()` nil 防御会让局部直接调用更宽容，但正式 graph 通过必选输入边和测试保证不依赖 nil 降级。

## Migration Plan

- 本变更只调整内部 Go composition、constructor 形态、测试和 OpenSpec delta，不涉及数据迁移、OpenAPI 生成、部署资产或配置迁移。
- 实施顺序为先调整 command constructor 和直接测试，再调整 auth `fx.go` provider 注册与 credential verifier annotation，然后收紧 Redis SessionStore Metrics 和 purge pool 注释，最后补齐 module graph 测试。
- 回滚策略是恢复五个 `*Deps`、五组 Params/wrapper、credential verifier wrapper 和 SessionStore optional Metrics；由于外部契约和持久化数据不变，回滚不需要数据修复。
- 验证方式包括 `go test -count=1 ./internal/features/auth/... ./internal/providers/... ./internal/bootstrap/...`、`make user-service-architecture-lint`、`openspec validate simplify-auth-command-composition`、`make lint` 和 `make verify`。

## Open Questions

- 无待决问题。
