## 1. 配置语义实现

- [x] 1.1 调整 `common/runtime/config` 中 `auth.token_version_cache_ttl` 校验，允许 `0` 和负数通过，同时保持 JWT access/refresh TTL 等字段必须为正数。
- [x] 1.2 更新或新增配置校验测试，覆盖正数 `auth.token_version_cache_ttl`、`0`、负数均可通过，并覆盖其他关键 duration 字段仍拒绝非正数。
- [x] 1.3 检查 `user-service/internal/features/auth/infrastructure/redis/session_store.go` 中 token version TTL fallback 注释和逻辑，确保非正数只回退默认 TTL，不会写入永久 Redis key。
- [x] 1.4 更新或新增 auth Redis session store 测试，验证正数 TTL 使用显式值，`0` 和负数 TTL 使用默认 TTL。

## 2. 质量收敛

- [x] 2.1 修复 `user-service/cmd/main.go` 中 `fxgraph` Cobra `RunE` 未使用 `cmd` 参数导致的 revive `unused-parameter` 问题。
- [x] 2.2 检查 `user-service/internal/features/role/infrastructure/postgres/role_permission_store_test.go` 的 `client.Schema.Create` 用法，优先改为现有 migration/test harness；若现有 harness 不支持，记录明确原因并避免改动运行时代码。
- [x] 2.3 保持角色 HTTP controller 拆分不纳入本次行为变更；如发现必须触碰该文件，仅做与验证失败直接相关的最小修改。

## 3. 验证

- [x] 3.1 运行相关配置和 auth Redis adapter 单元测试，确认 token version TTL 默认值与显式值行为符合规格。
- [x] 3.2 运行角色 PostgreSQL adapter 相关测试，确认 RBAC 绑定测试仍通过或记录外部依赖限制。
- [x] 3.3 运行 `make lint`，确认 `fxgraph` 参数问题和本次代码变更均通过静态检查。
- [x] 3.4 运行 `make verify`；若环境依赖导致无法完成，记录失败命令、失败原因和已完成的替代验证。
- [x] 3.5 运行 `make user-service-architecture-lint`，确认 OpenSpec change artifacts 与架构规则无 drift。
