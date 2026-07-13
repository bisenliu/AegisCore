## 1. Application 构造边界

- [x] 1.1 修改 `user-service/internal/features/auth/application/command/dependencies.go`，移除 `go.uber.org/fx` 和 `config` 导入，删除所有 deps struct 中的 `fx.In`。
- [x] 1.2 在 `dependencies.go` 中新增 `RefreshTokenSettings` 或等价窄 settings，并让 `RefreshTokenDeps` 只接收 tokens、sessions、metrics 和该 settings。
- [x] 1.3 修改 `user-service/internal/features/auth/application/command/refresh_token.go`，从 `deps.Settings.RefreshTokenRotation` 读取 rotation 开关，并删除依赖完整 `*config.Config` 的 helper。

## 2. Feature Composition Root

- [x] 2.1 在 `user-service/internal/features/auth/fx.go` 新增登录、刷新、强制改密、退出当前会话和退出全部会话的 Fx params 结构，所有 `fx.In` 只保留在这些 composition root 参数结构中。
- [x] 2.2 在 `auth/fx.go` 新增 `newLoginUseCase`、`newRefreshTokenUseCase`、`newChangePasswordUseCase`、`newLogoutCurrentSessionUseCase` 和 `newLogoutAllSessionsUseCase` wrapper，将 Fx params 转换为纯 application deps。
- [x] 2.3 替换 `auth.Module` 中直接 provide 的 `authcommand.NewXxxUseCase`，改为 provide feature-local wrapper，并保持现有 logger 调用方式不变。

## 3. 测试更新

- [x] 3.1 更新 `user-service/internal/features/auth/application/command` 包测试中所有 use case 构造调用，移除对旧 `fx.In` deps 形态和完整 `*config.Config` 的依赖。
- [x] 3.2 为 refresh rotation 相关测试显式传入 `RefreshTokenSettings{RefreshTokenRotation: true}`，不关心 rotation 的测试保留零值 false。
- [x] 3.3 运行 `go test ./user-service/internal/features/auth/application/command`，确认 command 包测试通过。
- [x] 3.4 运行 `go test ./user-service/internal/features/auth/...`，确认 auth feature 相关包通过。

## 4. 边界与规格验证

- [x] 4.1 运行 `grep -R "go.uber.org/fx\|fx.In" user-service/internal/features/auth/application/command`，确认无输出。
- [x] 4.2 运行 `make user-service-architecture-lint`，确认规格和架构边界检查通过。
- [x] 4.3 检查本变更未修改 HTTP API、OpenAPI 生成物、Ent schema、Atlas migration、部署资产或 `common/runtime/logger` 行为。

## 5. 最终验证

- [x] 5.1 将本次预期代码、测试和 OpenSpec artifact 变更加到暂存区。
- [x] 5.2 运行 `make lint`，确认 lint 通过。
- [x] 5.3 运行 `make verify`，确认完整验证通过且未暂存的非预期 drift 不阻塞最终检查。
