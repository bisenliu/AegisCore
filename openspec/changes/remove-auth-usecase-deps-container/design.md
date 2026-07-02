## Context

`user-service/internal/features/auth/application/command` 当前用 `UseCaseDeps` 聚合凭证校验、token 签发解析、session lifecycle、metrics 和 refresh token rotation 配置。登录、刷新、改密、退出当前会话和退出全部会话都接收同一个 `*UseCaseDeps`，因此同包内任一 use case 都能访问未声明属于自身职责的依赖。

本变更只调整 auth command 应用层内部结构和 Fx 装配形态，不改变认证 HTTP API、JWT 语义、Redis session 语义、数据库 schema 或部署流程。

## Goals / Non-Goals

**Goals:**

- 删除 `UseCaseDeps` 与 `NewUseCaseDeps`，不保留旧 constructor 兼容层。
- 每个 auth command use case 使用独立 `fx.In` 参数结构声明最小依赖。
- 每个 use case 结构体字段只保存实际需要的 collaborator。
- 将原 `UseCaseDeps.issueTokenPair` 和 `metricsRecorder` 改为不扩大依赖面的包内 helper。
- 更新 Fx provider 与测试 fixture，使编译期能暴露多余或缺失依赖。

**Non-Goals:**

- 不改变认证端点、HTTP DTO、错误映射或 OpenAPI 文档。
- 不调整 refresh session、token version、密码校验或 JWT 生成解析的业务行为。
- 不新增 common/shared 抽象，不移动 auth feature 边界。
- 不引入新的第三方依赖、数据库 migration 或部署资产。

## Decisions

1. 用 use case 专属 `fx.In` 参数替代共享容器。

   每个 constructor 直接接收自己的参数结构，例如 `LoginDeps`、`RefreshTokenDeps`、`ChangePasswordDeps`、`LogoutCurrentSessionDeps` 和 `LogoutAllSessionsDeps`。这样依赖范围由 constructor 签名和结构体字段共同表达，`LogoutCurrentSession` 无法再通过一个共享 `deps` 字段访问 credential 或 token collaborator。

2. 将共享逻辑改为窄 helper，而不是保留容器方法。

   `issueTokenPair` 需要 token issuer 与 session lifecycle，因此改为接收显式参数的包内函数，调用方必须把自身已持有的最小依赖传入。metrics 默认值处理改为 `metricsOrNop` helper，constructor 在创建 use case 时完成归一化，业务方法内直接使用 `u.metrics`。

3. Fx 装配同步移除 `NewUseCaseDeps`。

   `user-service/internal/features/auth/fx.go` 不再 provide 公共容器，只 provide 各 use case constructor。Fx 负责按新的参数结构注入 `Credentials`、`Tokens`、`Sessions`、`Config` 和可选 `Metrics`。

4. 测试 fixture 按新 constructor 直接组装。

   command 测试继续使用已有 mock collaborator，但不再通过 `newUseCaseDeps` 或 `UseCaseDepsParams` 间接传递。每个测试只为被测 use case 构造所需参数，避免测试 helper 再次形成隐藏的大容器。

## Risks / Trade-offs

- [Risk] 一次性删除公共容器会造成较多 constructor 和测试 fixture 编译错误。→ Mitigation：按 use case 文件逐个替换，并在每步运行相关包测试定位遗漏。
- [Risk] helper 抽取不当可能重新形成隐性共享依赖。→ Mitigation：helper 只接收执行动作所需的显式参数，不持有状态结构。
- [Risk] metrics 可选依赖默认值处理分散后可能出现 nil 调用。→ Mitigation：所有 constructor 统一调用 `metricsOrNop`，业务方法只访问归一化后的 `u.metrics` 字段。
- [Risk] 该变更是内部破坏性重构，回滚需要恢复旧构造形态。→ Mitigation：保持外部行为不变，若实现阶段测试失败且无法快速修复，可整 change 回滚到旧 `UseCaseDeps` 版本。

## Migration Plan

1. 在 `auth/application/command` 中新增各 use case 专属依赖参数结构和 `metricsOrNop`、窄 `issueTokenPair` helper。
2. 逐个替换 `loginUseCase`、`refreshTokenUseCase`、`changePasswordUseCase`、`logoutCurrentSessionUseCase`、`logoutAllSessionsUseCase` 的字段和方法访问。
3. 删除 `UseCaseDeps`、`UseCaseDepsParams` 和 `NewUseCaseDeps`。
4. 从 auth feature Fx provider 移除 `authcommand.NewUseCaseDeps`。
5. 更新 command 测试 fixture 和构造 helper。
6. 运行认证 command 相关测试、`make user-service-architecture-lint`，必要时再运行 `make user-service-test`。

## Open Questions

无。
