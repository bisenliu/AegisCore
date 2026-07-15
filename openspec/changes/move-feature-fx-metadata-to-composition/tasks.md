## 1. 审计与边界确认

- [x] 1.1 审计 `user-service/internal/features/{user,auth,role,permission}` 的 application/domain 生产包，记录并清理 `go.uber.org/fx` import、`fx.In` 和仅用于 Fx DI 的 struct tag。
- [x] 1.2 确认当前变更只涉及 role/permission command 构造与 composition 接线，不修改 RBAC command/query 业务语义、port 所有权、transport DTO、OpenAPI、Ent schema 或部署资产。

## 2. Application 构造器去 Fx 化

- [x] 2.1 将 `permission/application/command` 的 `PermissionCommandParams` 改为强类型普通参数，移除 application command 包中的 Fx import 和 `fx.In`。
- [x] 2.2 将 `role/application/command` 的 `RoleCommandParams` 改为强类型普通参数，移除 application command 包中的 Fx import 和 `fx.In`。
- [x] 2.3 保留 `PolicyChangeNotifier` 等必需依赖的 fail-fast 或拒绝装配语义，不新增 no-op notifier 或无业务意义的大接口。

## 3. Feature Composition 注册

- [x] 3.1 在 `user-service/internal/features/permission/fx.go` 中直接注册 permission command 普通构造器，不新增无 DI metadata 价值的重复 Params adapter。
- [x] 3.2 在 `user-service/internal/features/role/fx.go` 中直接注册 role command 普通构造器，不新增无 DI metadata 价值的重复 Params adapter。
- [x] 3.3 更新 `fx.Provide` 注册，确保正式 permission/role feature module 继续成功构图。

## 4. 测试调整

- [x] 4.1 更新 permission command 直接构造单元测试，使测试无需 import Fx 即可提供普通依赖并覆盖 notifier 必需性。
- [x] 4.2 更新 role command 直接构造单元测试，使测试无需 import Fx 即可提供普通依赖并覆盖 notifier 必需性。
- [x] 4.3 更新或保留 permission/role `fx_test.go`，验证正式 feature module 仍可成功构图和启动。
- [x] 4.4 运行相关 package 测试，例如 `go test ./internal/features/permission/... ./internal/features/role/...`（在 `user-service` module 下执行）。

## 5. 架构规则与文档

- [x] 5.1 更新 `user-service/scripts/architecture-lint.sh`，禁止 user/auth/role/permission application/domain 生产包新增 `go.uber.org/fx` import、`fx.In` 或仅用于 Fx DI 的 struct tag。
- [x] 5.2 更新 architecture lint fixture 或脚本测试，确保 application/domain Fx 违规样例会被 lint 命中且合理的 feature `fx.go`、infrastructure、transport 使用不被误伤。
- [x] 5.3 更新 `docs/ARCHITECTURE.md` 的 feature 分层说明，明确 application/domain 不承载 DI framework metadata，无 metadata 需求时 composition 直接注册普通构造器。

## 6. 验证与交付

- [x] 6.1 运行 `make user-service-architecture-lint`，确认新增规则和 fixture 通过。
- [x] 6.2 运行 `openspec validate move-feature-fx-metadata-to-composition`，确认 change artifacts 有效。
- [x] 6.3 执行 `rg "go\.uber\.org/fx|fx\.In|name:\"|optional:\"" user-service/internal/features/{user,auth,role,permission} --glob '*.go' --glob '!**/*_test.go'` 并确认 application/domain 生产包无违规，允许 feature composition、infrastructure 和 transport 中的合理 Fx 使用。
- [x] 6.4 暂存本次预期代码、文档和 OpenSpec 变更后运行 `make lint`。
- [x] 6.5 暂存本次预期代码、文档和 OpenSpec 变更后运行 `make verify`，确保最终验证不被未暂存预期 diff 阻塞。
