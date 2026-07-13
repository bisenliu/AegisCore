## Why

认证 command use case 的构造参数当前在 application 包内直接嵌入 `fx.In`，并且 refresh token use case 通过完整 `*config.Config` 获取单个开关。这使应用层构造 API 暴露 DI 框架语义和过宽配置依赖，增加单测隔离、服务抽取和依赖边界审查成本。

## What Changes

- **BREAKING** 移除 `user-service/internal/features/auth/application/command` 对 `go.uber.org/fx` 的源码依赖，删除 application deps struct 中的 `fx.In`。
- **BREAKING** 将认证 command use case 的 Fx 参数结构移动到 `user-service/internal/features/auth/fx.go`，由 feature composition root 负责将 Fx 注入结果转换为 application 构造参数。
- **BREAKING** `auth.Module` 不再直接 `fx.Provide(authcommand.NewXxxUseCase)`，改为提供 `newLoginUseCase`、`newRefreshTokenUseCase`、`newChangePasswordUseCase`、`newLogoutCurrentSessionUseCase` 和 `newLogoutAllSessionsUseCase` wrapper。
- **BREAKING** `RefreshTokenDeps` 不再接收完整 `*config.Config`，改为接收只包含 `RefreshTokenRotation` 的窄 settings。
- 保持认证 command use case 内现有 `common/runtime/logger` 包级调用不变，不在本变更中调整 logger 全局 fallback、trace/span 字段或日志注入方式。
- 更新认证 command 包相关单测，使测试按新的纯 application deps 构造 use case。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 收紧认证 command use case 的最小依赖边界，明确 application constructor 不得嵌入 `fx.In` 或接收完整运行时配置对象来读取单个 refresh rotation 设置。

## Impact

- 影响代码：`user-service/internal/features/auth/application/command/dependencies.go`、`refresh_token.go`、相关 command use case 测试，以及 `user-service/internal/features/auth/fx.go`。
- 影响 API：不改变 HTTP API、OpenAPI 响应、JWT claims、refresh session、password-change session 或 token version 运行时语义。
- 影响依赖：application command 源码不再直接导入 `go.uber.org/fx`；`common/runtime/logger` 间接依赖保持现状。
- 影响配置：不新增配置项，不改变 `auth.refresh_token_rotation` 的含义，只改变它进入 use case 的方式。
- 影响数据库和部署：不涉及 Ent schema、Atlas migration、Redis key schema、Docker、Compose、Kubernetes、Helm 或观测资产。
- 影响测试：command 包测试需要按新的 deps/settings 构造 use case，并验证 `fx.In` 不再出现在 `application/command` 源码中。
