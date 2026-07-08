## Why

当前 user-service 的 Atlas 初始 SQL migration 会为 Ent edge 生成真实数据库外键约束。目标架构要求保留 Ent 代码层关联关系和索引约束，但数据库层不创建真实外键，避免 PostgreSQL 在写入、删除、发布迁移和跨环境数据修复时承担引用完整性约束。

## What Changes

- **BREAKING** user-service 的 Atlas migration 生成链路全局禁用数据库真实外键，不再生成 `FOREIGN KEY` 或 `REFERENCES` 约束。
- 保留 Ent schema 中的 edge、绑定表 `*_id` 字段和唯一索引，由代码层继续维护关联关系与重复绑定约束。
- 将 Atlas schema 来源从直接 `ent://ent/schema` 改为受控 external schema loader，由 loader 统一传入 `migrate.WithForeignKeys(false)`。
- 删除旧 `init_sql` migration 并重新生成不含真实外键的新初始化 SQL。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: Ent/Atlas migration 生成流程全局禁止数据库真实外键，初始化 SQL 与后续 diff 都必须通过受控 loader 输出无 FK 的目标 schema。

## Impact

- 迁移资产：影响 `user-service/migrations/atlas.hcl`、`user-service/scripts/migrate-diff.sh`、`user-service/migrations/*init_sql.sql` 和 `atlas.sum`。
- 交付文档：需要说明 Atlas migration 生成必须使用无真实外键的 external schema loader。
- 数据完整性：数据库不再兜底引用完整性，绑定关系存在性和清理继续由 application/store 层负责。
