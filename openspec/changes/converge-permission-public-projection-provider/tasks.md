## 1. Permission Runtime 聚合

- [x] 1.1 在 `user-service/internal/features/permission/fx_authorization.go` 或相邻 composition 文件中新增 `PermissionRuntime` 和 `newPermissionRuntime`，字段覆盖 authorizer、policy health、watcher status、notifier、initializer、watcher runner 和 user-role resolver lifecycle。
- [x] 1.2 调整 permission Fx provider 列表，使 `PermissionRuntime` 从既有已构造组件组装，并确保 watcher 状态视图和运行器视图复用同一 watcher 实例。
- [x] 1.3 删除或收敛纯 named/private 到 public 的重复 projection provider，只保留父模块或相邻 feature 必需的稳定 contract 解包。

## 2. 消费方迁移

- [x] 2.1 更新 `user-service/internal/features/permission/fx_lifecycle.go`，使 policy initializer、watcher runner 和 user-role resolver lifecycle 通过 `PermissionRuntime` 或保留的稳定 contract 获取。
- [x] 2.2 检查并更新 `user-service/internal/providers/routes.go` 和 `user-service/internal/providers/health.go` 中对 permission authorizer、policy health、watcher status 的依赖，保持 HTTP route 和 health/status 行为不变。
- [x] 2.3 检查并更新 `user-service/internal/features/role/fx.go` 中对 permission notifier 或 lookup contract 的依赖，确保 role application 不导入 permission infrastructure 或 HTTP transport。

## 3. 测试与验证

- [x] 3.1 更新 `user-service/internal/features/permission/fx_test.go`，验证 `*PermissionRuntime` 可解析且对外 authorizer、policy health、watcher status、notifier 和 lifecycle 依赖仍可用。
- [x] 3.2 运行 `go test ./user-service/internal/features/permission/... ./user-service/internal/features/role/... ./user-service/internal/providers/...`，修复失败后再继续。
- [x] 3.3 运行 `make user-service-architecture-lint`，确认 feature 边界和 forbidden import 约束未被破坏。

## 4. 收尾门禁

- [x] 4.1 确认没有修改 OpenAPI、Ent schema、Atlas migration、部署清单或观测资产；如出现非预期生成物 diff，回到实现阶段修正原因。
- [x] 4.2 将本次预期代码、OpenSpec artifact 和必要文档变更加到暂存区。
- [x] 4.3 运行 `make lint` 并修复所有失败。
- [x] 4.4 运行 `make verify` 并修复所有失败，确认最终验证通过。
