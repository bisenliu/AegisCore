## 1. 收敛 domain 与 application 写路径

- [x] 1.1 删除 `user-service/internal/features/role/domain/role.go` 中的 `RoleMutation` 和 `ProtectSystemMutation`，同步删除只验证事务外系统角色保护的 domain 测试。
- [x] 1.2 修改 `application/command/role.go`，让 `UpdateRole` 和 `SetRoleActive` 在输入规范化后直接调用普通 `RoleStore`，删除系统角色预读、保护判断和 no-op 快速返回；保持 store 成功后才发送 policy change 通知。
- [x] 1.3 修改 `application/command/binding.go`，删除 add/replace 的角色存在性预读，保留 permission application 最小查询端口的单条或批量合法性校验，并让 add、replace、remove 的目标角色存在性和可写性统一由事务内 `RolePermissionStore` 决定。
- [x] 1.4 更新 role command 测试，断言 metadata、状态和角色权限 store 返回 `ErrSystemRoleProtected` 时错误原样传播、结果为空且 notifier 零调用；同步更新普通角色相同值写入进入 store 的单一路径语义。

## 2. 在 PostgreSQL 普通写边界强制系统角色不变量

- [x] 2.1 在 `role/infrastructure/postgres` 新增未导出的普通角色锁定 helper，通过当前 `*ent.Tx`、外部 `role_id` 和 `ForUpdate()` 查询最新角色，稳定映射 `ErrRoleNotFound`，并在 `IsSystem=true` 时返回 `ErrSystemRoleProtected`。
- [x] 2.2 修改 `RoleStore.Update` 和 `SetActive`，在 `transactPolicyChange` mutation closure 的第一步调用锁定 helper，随后按锁定角色内部 ID 且 `is_system=false` 执行 metadata 或状态 UPDATE，保留唯一冲突与 not found 错误语义。
- [x] 2.3 修改 `RolePermissionStore.Add`、`Replace`、`Remove`，统一调用同一锁定 helper 后再重校验权限和修改关系；删除名不副实的旧 `getLockedRoleByExternalID`，固定角色先于 permission、role_permissions 的锁定与访问顺序。
- [x] 2.4 检查 `SeedRoleStore`、`SeedRolePermissionStore` 和 Fx 接线，确认 seed 继续只使用受信端口且普通 command service 不获得 seed 依赖、bypass 参数、feature flag 或兼容分支。

## 3. 证明拒绝路径和并发原子性

- [x] 3.1 扩展真实 PostgreSQL role store 测试，覆盖系统角色 description-only、相同 metadata、状态变化和相同状态写入，逐例断言 `ErrSystemRoleProtected`，并比较角色、绑定、revision counter、revision、outbox 前后精确快照。
- [x] 3.2 扩展真实 PostgreSQL role permission store 测试，分别覆盖系统角色 add、replace、remove，使用合法且能区分前后集合的基线权限，断言绑定、角色、revision counter、revision、outbox 全部不变。
- [x] 3.3 增加 PostgreSQL 并发测试：使用独立连接和受控 transaction 让 seed 等价 UPDATE 先持有目标角色行锁，再启动普通 metadata、状态和权限绑定写入；提交 seed 后断言所有普通写稳定返回 `ErrSystemRoleProtected` 且没有普通写副作用，不为测试增加生产 hook、分支或接口。
- [x] 3.4 保留并验证普通角色 metadata、状态、权限 add/replace/remove、权限事务内重校验、revision/outbox 原子提交和 seed 基线维护的既有成功路径。

## 4. 同步 HTTP 契约与 OpenAPI

- [x] 4.1 扩展 role HTTP controller 测试，覆盖 metadata、状态及权限 add、replace、remove 的 `ErrSystemRoleProtected` 均渲染为 `409 Conflict` 和既有系统角色保护 envelope。
- [x] 4.2 修正上述 controller 的 Swagger failure 注解，明确系统角色保护为 `409`，运行 `make user-service-openapi-generate` 更新 `user-service/docs/openapi.go`、`openapi.json`、`openapi.yaml`。
- [x] 4.3 再次运行 `make user-service-openapi-generate` 并检查 `git diff --exit-code -- user-service/docs`，确认 OpenAPI 生成物无 drift，且路由、成功请求和响应结构没有变化。

## 5. 定向验证与范围检查

- [x] 5.1 对修改的 Go 文件运行 `gofmt`，执行 role feature 现有定向 `go generate` 入口并同步生成 mocks；再次运行生成命令并确认没有未预期 drift。
- [x] 5.2 运行 `go test ./user-service/internal/features/role/domain ./user-service/internal/features/role/application/command ./user-service/internal/features/role/infrastructure/postgres ./user-service/internal/features/role/transport/http`，并运行 `go test ./internal/features/role/infrastructure/postgres -count=1 -args -aegiscore.testcontainers`，确认普通写、保护拒绝和真实 PostgreSQL 并发测试全部通过。
- [x] 5.3 运行 `openspec validate enforce-atomic-system-role-protection` 和 `make user-service-architecture-lint`，确认 delta、feature 边界和受信 seed 端口符合主规格。
- [x] 5.4 检查 `git diff`，确认不存在 `RoleMutation`、application 系统角色预检查、普通 store bypass、feature flag 或兼容分支，且没有 Ent schema、migration、`common/`、`internal/shared`、policy sync、部署或观测资产变更。

## 6. 合并前门禁

- [x] 6.1 使用显式路径暂存本 change 的 OpenSpec artifacts、role 代码、测试、生成 mocks 和 OpenAPI 生成物，检查 `git status --short`，确认暂存范围只包含预期变更。
- [x] 6.2 在全部预期变更已暂存后运行 `make lint`；仅在命令通过后将本任务标记完成。
- [x] 6.3 在全部预期变更已暂存后运行 `make verify`；仅在相关测试、生成检查和最终 `git diff --exit-code` 全部通过后将本任务及 change 标记完成。
