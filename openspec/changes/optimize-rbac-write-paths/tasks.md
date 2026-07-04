## 1. 写侧返回契约调整

- [x] 1.1 调整 permission application command 和 port，使权限更新、启用、停用成功后不返回完整 `PermissionResult`，并更新生成 mock。
- [x] 1.2 调整 role application command 和 port，使角色更新、启用、停用成功后不返回完整 `RoleResult`，并更新生成 mock。
- [x] 1.3 调整 permission HTTP controller、response mapper 和 controller 测试，使权限更新、启用、停用成功响应不包含实体响应体。
- [x] 1.4 调整 role HTTP controller、response mapper 和 controller 测试，使角色更新、启用、停用成功响应不包含实体响应体。

## 2. Store 更新路径优化

- [x] 2.1 调整 `PermissionStore.Update` 和 `PermissionStore.SetActive`，成功写入后不调用 `GetByPermissionID`，并保留 NotFound 与唯一冲突错误映射。
- [x] 2.2 调整 `RoleStore.Update` 和 `RoleStore.SetActive`，成功写入后不调用 `GetByRoleID`，并保留 NotFound 与唯一冲突错误映射。
- [x] 2.3 更新 permission 和 role command service 测试，明确系统保护前置查询保留、store 写后 refetch 不再作为返回实体依赖。
- [x] 2.4 更新 permission 和 role PostgreSQL store 测试，覆盖更新、启停、NotFound 和唯一冲突语义。

## 3. Ent 生成与批量绑定写入

- [x] 3.1 修改 `user-service/ent/generate.go`，启用 `sql/upsert` 生成特性。
- [x] 3.2 执行 `make user-service-generate`，提交 Ent 生成物并审查 `OnConflict`、`DoNothing` 和 bulk create 相关 diff。
- [x] 3.3 将 `RolePermissionStore.Replace` 的新增绑定循环替换为事务内 `CreateBulk`，空集合不执行 bulk create。
- [x] 3.4 将 `RolePermissionStore.EnsureSystemBindings` 和 `SyncSystemBindings` 的新增绑定循环替换为 bulk create，并在幂等路径保留唯一冲突忽略语义。
- [x] 3.5 将 `UserRoleStore.Replace` 的新增绑定循环替换为事务内 `CreateBulk`，空集合不执行 bulk create。
- [x] 3.6 补充角色权限和用户角色 store 测试，覆盖批量写入成功、空集合、唯一冲突幂等、非冲突错误回滚和事务不产生部分结果。

## 4. OpenAPI 与规格同步

- [x] 4.1 更新 permission 和 role HTTP 注解，表达更新、启用、停用成功响应不返回实体。
- [x] 4.2 执行 `make user-service-openapi-generate`，提交 `user-service/docs/openapi.go`、`openapi.json` 和 `openapi.yaml` 生成物。
- [x] 4.3 执行 `make user-service-architecture-lint`，确认 OpenSpec delta、架构边界和文档语言规则通过。

## 5. 验证与收尾

- [x] 5.1 执行相关包测试：`go test ./internal/features/permission/... ./internal/features/role/...`，并修复失败。
- [x] 5.2 执行 `make user-service-test`，确认 user-service 测试通过。
- [x] 5.3 使用 `git diff --exit-code` 或等价检查确认生成物 drift 已纳入预期变更。
- [x] 5.4 将本次预期代码、生成物、OpenSpec artifact 和文档变更加到暂存区。
- [x] 5.5 执行 `make lint`，通过后将本任务标记完成。
- [x] 5.6 执行 `make verify`，通过后将本 change 所有已完成任务标记为 `- [x]`。
