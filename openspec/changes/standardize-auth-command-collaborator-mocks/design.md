## Context

`user-service/internal/features/auth/application/command` 负责认证 command use case 编排，覆盖登录、刷新、强制改密、退出当前会话和退出全部会话。该包已有 `mock_generate.go` 生成 `UserCredentialStore`、`UserTokenVersionStore`、`TokenVersionCache`、`RefreshSessionStore`、`Metrics`、`Verifier`、`Issuer` 和 `Lifecycle` mock，但 `service_test.go` 仍保留手写 credential/session/token/metrics collaborator double。

当前手写替身把调用记录、状态变更和错误注入混在自定义结构体中，导致失败路径和指标记录需要通过替身内部状态间接断言。后续继续调整 auth session 行为时，生成 mock 与手写替身会形成两套测试契约，增加 drift 风险。

## Goals / Non-Goals

**Goals:**

- 将 `auth/application/command` 包内 use case 测试的外部协作者统一迁移到已有 gomock 生成物。
- 移除 `authCredentialTestStore`、`authSessionTestStore`、`recordingAuthMetrics`、`refreshRotationTokenIssuer` 等旧手写 collaborator double。
- 用 expectation、`gomock.InOrder`、自定义 matcher 或 `DoAndReturn` 直接表达依赖调用、失败路径、状态传递和指标记录。
- 保留只负责构造输入、生成领域对象或提供真实轻量纯函数依赖的测试 helper。

**Non-Goals:**

- 不修改 auth command 生产代码、JWT 签发实现、密码服务实现、Redis store、PostgreSQL adapter 或配置语义。
- 不迁移 `auth/application/credentials`、`auth/application/sessions`、`auth/application/validators` 包测试。
- 不新增跨包共享 mock 仓库，不改变 `mock_generate.go` 覆盖范围，除非执行 `make user-service-generate` 暴露生成物 drift。
- 不改变 HTTP API、OpenAPI、数据库 migration、部署清单、观测资产或安全边界。

## Decisions

### Decision: 以现有生成 mock 作为唯一外部 collaborator 表达

实现时使用 `mock_generate.go` 已覆盖的接口生成物替换手写 credential、session、token、metrics 和 lifecycle collaborator。这样测试断言与 production port 的方法签名保持同源，接口签名变化会通过编译或 gomock expectation 明确暴露。

备选方案是继续保留手写替身并补充更多字段记录调用。该方案会继续维持两套契约，且字段状态断言难以表达调用顺序与禁止调用，因此不采用。

### Decision: 用 expectation 表达行为，用 `DoAndReturn` 承载必要状态

登录、刷新、改密、退出当前会话和退出全部会话测试应优先通过 `EXPECT()` 声明依赖调用、参数和错误返回。确实需要模拟状态转移时，使用 `DoAndReturn` 在单个 expectation 内构造返回值或捕获参数；需要顺序保证时使用 `gomock.InOrder`。

备选方案是把所有状态抽成新的测试 fixture 类型。该方案容易重新演化成手写 double，因此只允许保留纯构造 helper，不保留实现 collaborator interface 的自定义类型。

### Decision: matcher 只用于领域对象参数的稳定断言

当 expectation 参数包含 `authdomain.AuthSession`、`authdomain.UpdateCredentialsInput` 或 token claims 等结构时，可以使用自定义 matcher 校验关键字段，避免 brittle 的全对象深比较。matcher 应放在 command 包测试内，且只服务当前包测试。

备选方案是全部使用 `gomock.Any()`。该方案会丢失关键参数契约，不利于覆盖刷新 rotation、token version 和 session revoke 等安全敏感路径，因此不采用。

## Risks / Trade-offs

- [Risk] gomock expectation 过细导致测试对无关调用顺序敏感 -> Mitigation：只对安全语义相关顺序使用 `gomock.InOrder`，其他路径按参数和调用次数断言。
- [Risk] 迁移过程中遗漏旧手写替身覆盖的错误注入路径 -> Mitigation：按登录、刷新、改密、退出当前会话、退出全部会话分组迁移，并在每组完成后运行 command 包测试。
- [Risk] 生成 mock 与 `mock_generate.go` 不一致 -> Mitigation：执行 `make user-service-generate`，确认 `mock_*.go` 无 drift。
- [Risk] 指标记录从状态断言迁移到 expectation 时漏掉失败 reason -> Mitigation：为成功和失败路径分别声明 `Metrics` expectation，失败路径显式匹配 reason 字符串。

## Migration Plan

1. 梳理 `service_test.go` 中旧手写 collaborator double 的调用点和被测 use case 分组。
2. 按登录、刷新、改密、退出当前会话、退出全部会话逐步替换为 `NewMock...` 生成物，并用 expectation 表达依赖调用和失败返回。
3. 删除不再使用的 `authCredentialTestStore`、`authSessionTestStore`、`recordingAuthMetrics`、`refreshRotationTokenIssuer` 类型及其方法。
4. 执行 `make user-service-generate`，确认 mockgen 输出无 drift。
5. 执行 `cd user-service && go test ./internal/features/auth/application/command`。
6. 执行 `make user-service-architecture-lint`。

回滚方式是还原本次测试文件和 OpenSpec change artifacts；由于不改生产代码、schema、配置或部署资产，不需要运行时回滚步骤。

## Open Questions

- 无。
