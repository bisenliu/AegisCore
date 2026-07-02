## 1. Command 依赖结构调整

- [x] 1.1 在 `user-service/internal/features/auth/application/command` 中定义各 use case 专属依赖参数结构：`LoginDeps`、`RefreshTokenDeps`、`ChangePasswordDeps`、`LogoutCurrentSessionDeps` 和 `LogoutAllSessionsDeps`。
- [x] 1.2 将 `metricsRecorder` 逻辑替换为 constructor 阶段使用的 `metricsOrNop` helper，确保业务方法内不再依赖 `UseCaseDeps.metricsRecorder()`。
- [x] 1.3 将 `UseCaseDeps.issueTokenPair` 替换为接收显式 `authtokens.Issuer` 和 `authsessions.Lifecycle` 参数的包内窄 helper。
- [x] 1.4 更新 `loginUseCase` 字段和方法访问，使其只持有 credential verifier、token issuer、session lifecycle 和 metrics。
- [x] 1.5 更新 `refreshTokenUseCase` 字段和方法访问，使其只持有 token issuer/verifier、session lifecycle、metrics 和 refresh token rotation 配置值。
- [x] 1.6 更新 `changePasswordUseCase` 字段和方法访问，使其只持有 credential verifier、token issuer/verifier 和 session lifecycle。
- [x] 1.7 更新 `logoutCurrentSessionUseCase` 和 `logoutAllSessionsUseCase` 字段和方法访问，使其只持有 session lifecycle 和 metrics。
- [x] 1.8 删除 `UseCaseDeps`、`UseCaseDepsParams` 和 `NewUseCaseDeps`，确认 command 包内不存在 `deps.credentials`、`deps.tokens`、`deps.sessions`、`deps.refreshTokenRotation` 或 `deps.metricsRecorder()` 访问。

## 2. 装配与测试更新

- [x] 2.1 从 `user-service/internal/features/auth/fx.go` 移除 `authcommand.NewUseCaseDeps` provider。
- [x] 2.2 更新认证 command 测试 fixture，按各 use case 新 constructor 直接传入最小 mock collaborator。
- [x] 2.3 更新所有直接调用 `NewLoginUseCase`、`NewRefreshTokenUseCase`、`NewChangePasswordUseCase`、`NewLogoutCurrentSessionUseCase` 和 `NewLogoutAllSessionsUseCase` 的测试或 helper。
- [x] 2.4 运行 `go test ./user-service/internal/features/auth/application/command`，确认 command use case 行为未变化。
- [x] 2.5 运行 `rg -n "UseCaseDeps|NewUseCaseDeps|deps\\.(credentials|tokens|sessions|refreshTokenRotation|metricsRecorder\\()" user-service/internal/features/auth/application/command user-service/internal/features/auth/fx.go`，确认旧公共容器和旧访问模式已清理。

## 3. 规格与结构验证

- [x] 3.1 运行 `openspec status --change remove-auth-usecase-deps-container`，确认 proposal、design、specs 和 tasks 均满足 apply 前置条件。
- [x] 3.2 运行 `make user-service-architecture-lint`，确认 feature 边界和架构约束未被破坏。
- [x] 3.3 检查 `git diff -- user-service/internal/features/auth/application/command user-service/internal/features/auth/fx.go openspec/changes/remove-auth-usecase-deps-container`，确认没有非预期文件或生成物漂移。
- [x] 3.4 将本次预期代码、测试和 OpenSpec artifact 加入暂存区。
- [x] 3.5 运行 `make lint`，确认 lint 通过。
- [x] 3.6 运行 `make verify`，确认全量验证通过且最终 diff 检查不被未暂存预期变更阻塞。
