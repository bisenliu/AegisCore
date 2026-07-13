## Context

`auth-session-management` 已经要求认证 command use case 通过自身 constructor 声明最小依赖，并避免通过公共依赖容器隐藏单个 use case 的真实依赖面。当前代码已拆分 `LoginDeps`、`RefreshTokenDeps`、`ChangePasswordDeps`、`LogoutCurrentSessionDeps` 和 `LogoutAllSessionsDeps`，但这些 deps struct 仍位于 `application/command` 包并嵌入 `fx.In`，导致 application constructor API 暴露 Fx 装配语义。

同时，`RefreshTokenDeps` 当前接收完整 `*config.Config`，而 use case 只需要 `Auth.RefreshTokenRotation`。这使 refresh use case 的 settings 依赖面大于真实需要，也让测试和未来抽取必须构造或理解完整运行时配置结构。

本变更限定在认证 command use case 构造边界和 feature composition root，不改变认证 HTTP API、JWT、session、token version、Redis key、数据库 schema、OpenAPI 或部署资产。`common/runtime/logger` 的包级 context logger 调用保持现状，不在本变更中调整全局 logger fallback。

## Goals / Non-Goals

**Goals:**

- 使 `user-service/internal/features/auth/application/command` 源码不再导入 `go.uber.org/fx`，并不再嵌入 `fx.In`。
- 将 Fx 参数结构集中到 `user-service/internal/features/auth/fx.go`，由 feature composition root 将容器注入参数转换为纯 application deps。
- 将 refresh token rotation 配置从完整 `*config.Config` 收窄为 `RefreshTokenSettings` 或等价的窄 settings。
- 让 command 包单测直接构造纯 deps/settings，避免测试 API 携带 Fx 语义。
- 保持现有认证运行时行为、错误语义、metrics 记录和 logger 调用方式不变。

**Non-Goals:**

- 不调整 `common/runtime/logger/context.go` 的 `defaultLogger`、`FromContext`、`Info/Warn/Error` 设计。
- 不向 auth command use case 注入 `*zap.Logger`，不新增 logger wrapper 或 logger interface。
- 不改变 HTTP request/response、OpenAPI、JWT claims、refresh session 存储、password-change session、token version 校验或 RBAC 行为。
- 不新增配置项，不改变 `auth.refresh_token_rotation` 的配置位置或含义。
- 不新增兼容构造器，不保留旧 `fx.In` deps 入口。

## Decisions

### Decision: `fx.In` 只保留在 feature composition root

`LoginDeps`、`RefreshTokenDeps`、`ChangePasswordDeps`、`LogoutCurrentSessionDeps` 和 `LogoutAllSessionsDeps` 保留在 `application/command` 包，但删除嵌入的 `fx.In`。`auth/fx.go` 新增 `loginUseCaseParams`、`refreshTokenUseCaseParams`、`changePasswordUseCaseParams`、`logoutCurrentSessionUseCaseParams` 和 `logoutAllSessionsUseCaseParams`，这些类型嵌入 `fx.In` 并只服务于 Fx 装配。

理由：use case 构造器属于 application 边界，应描述业务协作者和 settings，而不是描述具体 DI 容器的参数解析规则。把 `fx.In` 收回到 `auth/fx.go` 可以保留现有 Fx 装配能力，同时让 application 包对非 Fx 场景更容易测试和抽取。

备选方案：保留当前 deps struct 的 `fx.In` 并仅通过注释说明其用于装配。该方案不能消除框架语义进入 application 构造 API 的问题，拒绝采用。

### Decision: `auth.Module` 使用 wrapper provider 适配 application constructor

`auth.Module` 不再直接 provide `authcommand.NewLoginUseCase` 等 application constructor，而是 provide `newLoginUseCase` 等 feature-local wrapper。wrapper 接收 Fx params，并调用 application constructor。

理由：Fx `optional` tag、完整 `*config.Config` 获取和 provider 装配细节都属于 composition root 职责。wrapper 是最小改动方式，不需要新增 shared 包、adapter 包或跨 feature 抽象。

备选方案：使用 `fx.Annotate` 或 `fx.ParamTags` 直接适配 application constructor。该方案仍会让 constructor 参数设计受 Fx 能力约束，且 refresh settings 收窄不如 wrapper 清晰，拒绝采用。

### Decision: refresh use case 接收窄 settings

`RefreshTokenDeps` 删除 `Config *config.Config`，改为接收 `Settings RefreshTokenSettings`，其中包含 `RefreshTokenRotation bool`。`auth/fx.go` wrapper 从 `params.Config.Auth.RefreshTokenRotation` 提取该值后传入 application constructor。

理由：refresh use case 只需要一个布尔开关，接收完整配置对象会扩大依赖面并降低测试隔离度。窄 settings 明确表达真实需求，也避免 application 包导入 runtime config。

备选方案：constructor 直接接收 `refreshTokenRotation bool`。该方案更短，但后续如果 refresh use case 增加同类窄 setting，会反复修改签名；使用小 settings struct 更稳定，采用 settings struct。

### Decision: 不调整 logger 调用

认证 command use case 内现有 `logger.Info(ctx, ...)`、`logger.Warn(ctx, ...)`、`logger.Error(ctx, ...)` 保持不变。

理由：本变更的目标是在不调整 log 的约束下解决 Fx 和过宽 settings 的边界问题。logger 全局 fallback 是另一个横切设计点，涉及 `common/runtime/logger` 包结构、trace/span 字段和调用方习惯，不应混入本次 change。

备选方案：同步引入显式 `*zap.Logger` 或 context-aware logger wrapper。该方案会扩大变更范围，并与当前约束冲突，拒绝采用。

## Risks / Trade-offs

- `go list -deps ./user-service/internal/features/auth/application/command` 仍可能因 `common/runtime/logger` 间接出现 `go.uber.org/fx` → 本次验收以 `application/command` 源码不导入 `go.uber.org/fx` 且不出现 `fx.In` 为硬性边界，logger 间接依赖另行治理。
- 删除旧 deps 形态会导致现有测试和 provider 编译失败 → 同步更新 command 包测试和 `auth/fx.go` wrapper，不保留兼容构造器。
- wrapper provider 增加少量 composition root 代码 → 该代码集中在 `auth/fx.go`，换取 application constructor 纯净边界。
- refresh settings 默认零值为 `RefreshTokenRotation=false` → 与当前 `refreshTokenRotationEnabled(nil)` 返回 false 的行为一致，测试中需要显式设置 true 覆盖 rotation 分支。

## Migration Plan

1. 修改 `application/command/dependencies.go`，移除 `go.uber.org/fx` 和 `config` 导入，删除所有 deps struct 内的 `fx.In`，新增 `RefreshTokenSettings`。
2. 修改 `refresh_token.go`，让 constructor 从 `deps.Settings.RefreshTokenRotation` 读取开关，并删除完整 config helper。
3. 修改 `auth/fx.go`，新增五组 Fx params 和 wrapper provider，并替换 `fx.Provide` 中直接提供的 application constructor。
4. 修改 command 包测试中 `RefreshTokenDeps` 的构造方式，rotation 分支显式传入 `RefreshTokenSettings{RefreshTokenRotation: true}`。
5. 运行验证命令：`go test ./user-service/internal/features/auth/application/command`、`go test ./user-service/internal/features/auth/...`、`grep -R "go.uber.org/fx\|fx.In" user-service/internal/features/auth/application/command`。

回滚方式：恢复 `dependencies.go` 中 `fx.In` 和 `RefreshTokenDeps.Config`，恢复 `auth.Module` 直接 provide application constructor，并还原 refresh 测试构造参数。本变更不涉及数据迁移、API 迁移或部署资产回滚。

## Open Questions

- 无。本变更明确不处理 logger 全局 fallback，后续如需治理应单独提出 `runtime-observability` 或 `shared-platform-primitives` 相关 change。
