## 1. OpenSpec 与迁移契约

- [x] 1.1 在 `delivery-operations` spec delta 中明确 user-service migration MUST NOT 生成数据库真实外键。
- [x] 1.2 明确 Ent edge 必须保留为代码层关联定义，不能通过删除 edge 规避外键生成。

## 2. Atlas/Ent 生成链路

- [x] 2.1 新增 user-service Atlas external schema loader，统一导出不含真实数据库外键的 Ent 目标 schema。
- [x] 2.2 修改 `user-service/migrations/atlas.hcl`，将 `env "local".src` 切换到 external schema loader。
- [x] 2.3 更新 `user-service/scripts/migrate-diff.sh` 和 Ent 说明文档，删除直接 `ent://ent/schema` 作为迁移 schema 来源的描述。

## 3. 初始化 SQL

- [x] 3.1 删除旧 `user-service/migrations/20260701032412_init_sql.sql`。
- [x] 3.2 重新创建新的 `init_sql` migration，确保 SQL 不包含 `FOREIGN KEY` 或 `REFERENCES`。
- [x] 3.3 刷新 `user-service/migrations/atlas.sum`。

## 4. 验证

- [x] 4.1 运行 external schema loader，确认输出 DDL 不包含真实外键。
- [x] 4.2 运行 `make user-service-migrate-validate`。
- [x] 4.3 运行 `make user-service-architecture-lint`。
