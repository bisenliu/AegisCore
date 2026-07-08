## Context

user-service 使用 Ent schema 表达实体字段、索引和 edge 关系，并通过 Atlas 生成版本化 SQL migration。当前直接使用 `ent://ent/schema` 作为 Atlas 目标 schema 来源时，Ent edge 会被转换为数据库真实外键约束，导致初始化 SQL 中包含 `FOREIGN KEY` 和 `REFERENCES`。

目标状态是数据库只保留普通关联列、唯一索引和查询索引，引用完整性由 application/store 层维护；Ent edge 继续作为代码层关联查询和 eager loading 的类型化定义。

## Decisions

- 将 Atlas 目标 schema 来源固定为 user-service 自有 external schema loader，不再直接使用 `ent://ent/schema`。
- external schema loader 基于 Ent 生成的 `migrate.Tables` 导出 PostgreSQL DDL，并在导出前移除表级 `ForeignKeys`，确保输出目标 schema 不包含真实数据库外键。
- 不删除或弱化 Ent schema 中的 edge 定义，不改变 `user_roles`、`role_permissions` 的关联字段和唯一索引。
- 删除旧初始化 SQL 并重新生成新的 `init_sql` migration，不保留带真实外键的兼容迁移文件。
- 迁移目录 hash 通过 `atlas.sum` 重新计算，后续 migration diff 继续通过统一脚本进入 external schema loader。

## Risks

- 数据库不再阻止孤儿关联记录，应用层绑定写入、删除清理和测试必须继续覆盖关联存在性。
- external schema loader 直接决定 Atlas 目标 schema 输出，后续 Ent 版本升级时需要保持 loader 和 Ent 生成表结构兼容。

## Rejected Alternatives

- 不删除 Ent edge：删除 edge 会破坏代码层关联查询能力，不符合目标。
- 不保留旧初始化 SQL：旧 SQL 含真实外键，与目标架构冲突，因此直接替换。
- 不保留双轨迁移：本次要求不保留兼容方案，所有后续迁移统一使用无真实外键目标 schema。
