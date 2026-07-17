## 1. Adapter Constructor 清理

- [x] 1.1 修改 `permission/infrastructure/postgres` 的 `PermissionStore` constructor，移除 `fx.In` Params 和 `primary_db` tag，改为接收普通 `*ent.Client`。
- [x] 1.2 修改 `permission/infrastructure/casbin` 的 policy `Loader` constructor，移除 `fx.In` Params 和 DI tag，保持 policy 权威来源、超级管理员 wildcard 和用户软删除过滤语义不变。
- [x] 1.3 修改 `permission/infrastructure/casbin` 的 `Engine` constructor，移除 `fx.In` Params，使 loader、user role resolver、metrics 和 logger 通过普通参数或无 DI metadata options 注入。
- [x] 1.4 修改 `permission/infrastructure/redis` 的 policy `Store` 和 `VersionTracker` 构造路径，移除 `fx.In` Params、`cache_redis` tag、`fx.As`/`fx.Self` 依赖和 DI metadata options。

## 2. Feature Composition 适配

- [x] 2.1 更新 `user-service/internal/features/permission/fx.go`，在 composition 边界显式选择 `primary_db`、`cache_redis`、配置、logger 和 lifecycle 依赖。
- [x] 2.2 用普通 provider 函数构造一个 Casbin `Engine`，并显式赋值暴露 concrete、`permissionauthorization.Engine` 和 `permissionapplication.PolicyReloadEngine`，禁止重复构造。
- [x] 2.3 用普通 provider 函数构造一个 Redis policy `Store`，并显式赋值暴露 concrete 与 `permissionapplication.PolicyVersionPublisher` 等接口视图，禁止重复构造。
- [x] 2.4 保留本 change 范围外的 Casbin initial load、watcher `Start/Stop` 和用户角色缓存 `Close` 生命周期语义，不迁移到新的生命周期模型。

## 3. 测试更新

- [x] 3.1 更新 PostgreSQL、Casbin 和 Redis adapter 单元测试，使测试通过普通 Go 参数构造目标 adapter，不依赖 Fx/Dig tag 或 wrapper。
- [x] 3.2 更新 permission Fx composition 测试，断言 authorization/reload port 指向同一个 `Engine` 实例。
- [x] 3.3 更新 Redis policy store/publisher 相关测试，断言 concrete/interface 视图指向同一个 `Store` 或 `VersionTracker` 实例。
- [x] 3.4 运行 `cd user-service && go test ./internal/features/permission/infrastructure/... -count=1` 并修复失败。

## 4. 架构与规格验证

- [x] 4.1 检查除后续生命周期文件和 `features/permission/fx.go` 外，permission 生产包不再导入 Fx/Dig 或使用 DI tags。
- [x] 4.2 运行 `openspec validate remove-fx-from-permission-adapters` 并修复失败。
- [x] 4.3 运行 `make user-service-architecture-lint` 并修复失败。

## 5. 全量交付验证

- [x] 5.1 暂存本次预期代码、测试和 OpenSpec artifact 变更，避免最终 verify 被未暂存预期 diff 阻塞。
- [x] 5.2 运行 `make lint` 并修复失败。
- [x] 5.3 运行 `make verify` 并修复失败。
- [x] 5.4 检查最终 diff，确认未包含 OpenAPI、migration、部署资产或范围外生命周期迁移。
