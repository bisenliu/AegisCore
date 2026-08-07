## 1. Schema Helper 实现

- [x] 1.1 在 `user-service/internal/persistence/ent/schema` 包内新增或扩展 helper 文件，定义 `createdAtMillisMixin`、`updatedAtMillisMixin` 和共享 `oneOfStrings`。
- [x] 1.2 将 `user.go`、`role.go`、`permission.go`、`rolepermission.go`、`userrole.go`、`rbacpolicyrevision.go` 和 `rbacpolicyoutboxevent.go` 中重复的 `created_at`、`updated_at` 字段声明替换为对应 mixin。
- [x] 1.3 从 `rbacpolicyoutboxevent.go` 移除文件私有 `oneOfStrings`，并确保 outbox `kind`、`status` 的允许值和校验失败语义不变。

## 2. 生成物与测试

- [x] 2.1 执行 `make user-service-generate`，审查 Ent 生成物 diff，确认字段名、列类型、注释、默认值和 `UpdateDefault` 语义未产生非预期变化。
- [x] 2.2 执行 schema 相关测试或最小相关包测试，覆盖时间字段和 `oneOfStrings` 校验路径。
- [x] 2.3 执行 `make user-service-migrate-diff name=standardize-ent-schema-helpers`，审查 SQL migration 和 `atlas.sum`，确认没有非预期数据库结构漂移。
- [x] 2.4 执行 `make user-service-migrate-validate`，确认 migration hash 和 SQL 校验通过。

## 3. OpenSpec 与最终验证

- [x] 3.1 执行 `openspec validate standardize-ent-schema-helpers`、`openspec list --specs` 和 `openspec validate --specs`。
- [x] 3.2 执行 `make user-service-architecture-lint`，确认 schema helper 归属不违反架构边界。
- [x] 3.3 暂存本次预期代码、生成物、migration 和 OpenSpec artifact 变更。
- [x] 3.4 执行 `make lint` 和 `make verify`，确认全仓 lint、测试和生成物 drift 检查通过。
