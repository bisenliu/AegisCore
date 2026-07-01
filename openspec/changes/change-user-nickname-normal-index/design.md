## Context

`user-service/ent/schema/user.go` 当前为 `users.nickname` 定义了 PostgreSQL GIN trigram 索引，依赖 `entsql.IndexTypes`、`entsql.OpClass` 和 `gin_trgm_ops`。用户要求将该字段改为普通索引，因为当前生成变更不能自动生成插件相关内容，同时需要将历史 SQL migration 归并为单个最新迁移文件。

该变更仅影响 user-service 的 Ent schema 和 Atlas migration 工件，不改变用户 HTTP API、OpenAPI、认证、RBAC、common 模块或部署清单。

## Goals / Non-Goals

**Goals:**

- 将 `users.nickname` 索引改为 Ent 原生普通字段索引，避免插件相关 index annotation。
- 整理 `user-service/migrations/*.sql`，删除旧 SQL 文件，只保留一个反映最终 schema 的最新迁移文件。
- 用 diff 方式展示实现后的文件变化，便于审查 schema 和 migration 结果。
- 保持用户资料创建、查询、列表、状态约束、用户名唯一性和 HTTP 响应语义不变。

**Non-Goals:**

- 不新增 PostgreSQL extension 管理能力或 trigram 索引生成插件。
- 不调整用户列表查询条件、分页协议、OpenAPI 注解或 RBAC 权限基线。
- 不迁移到新的数据库表结构或改变已有字段类型、默认值、约束语义。
- 不修改 common、deployments 或观测资产。

## Decisions

- 决策：在 Ent schema 中使用 `index.Fields("nickname")` 作为普通索引。
  备选方案：保留 GIN trigram annotation 并手工维护 SQL。该方案继续依赖生成链路外的插件和 opclass 内容，不符合本次“生成变更不能自动生成插件相关内容”的约束。

- 决策：删除 `ent/dialect` 和 `ent/dialect/entsql` import。
  备选方案：保留未使用 import 或空 annotation。该方案会导致编译或 lint 问题，也不能表达普通索引意图。

- 决策：迁移文件采用单个最新完整 SQL，而不是在旧 `init_schema.sql` 后追加 drop/create delta。
  备选方案：保留旧迁移并新增修正迁移。该方案不满足“删除当前旧的所有 SQL 文件，只保留一个最新的迁移文件”的交付要求。

- 决策：只更新 `user-identity-management` 规格中的用户查询索引支撑要求。
  备选方案：新增独立迁移整理 capability。该方案会把一次性迁移工件整理伪装成长期业务能力，不符合 capability 边界。

## Risks / Trade-offs

- [Risk] 昵称 contains 查询失去 trigram GIN 索引后，在大数据量下前置通配符查询性能可能下降。→ Mitigation：规格明确普通索引是当前持久化要求；如未来重新引入插件索引，应通过新的 change 同步 extension 管理、生成链路和性能验证。
- [Risk] 删除旧迁移文件会改变全新环境的迁移历史文件名。→ Mitigation：只保留一个最新完整迁移文件，并运行 migration validate 验证 SQL 可审查；已部署环境需要由发布流程确认迁移历史策略。
- [Risk] 手工整理 SQL 可能遗漏当前 schema 的其他索引。→ Mitigation：以现有 SQL 内容为基线，保留当前已存在的非 nickname 优化索引，仅将 nickname 索引保持为普通索引。

## Migration Plan

1. 修改 `user-service/ent/schema/user.go`，将 `nickname` 索引改为普通 `index.Fields("nickname")`。
2. 删除现有旧 SQL migration 文件，创建单个最新完整迁移文件，包含当前 schema 的表、注释、唯一索引、普通 nickname 索引和其他已存在优化索引。
3. 运行 `make user-service-migrate-validate` 验证迁移文件。
4. 使用 `git diff -- user-service/ent/schema/user.go user-service/migrations` 输出最终修改内容。

Rollback：如果需要恢复 trigram 索引，应通过新的 change 重新引入 Ent annotation、PostgreSQL extension/opclass 管理和对应 migration，而不是在本次变更中保留旧 SQL。

## Open Questions

- 无。本次按用户明确要求将 `nickname` 改为普通索引并归并迁移文件。
