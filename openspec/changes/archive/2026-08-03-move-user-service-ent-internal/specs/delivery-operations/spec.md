## MODIFIED Requirements

### Requirement: Ent/Atlas migration 交付

Ent schema MUST 是数据库结构来源，Atlas SQL migration MUST 是可审查交付工件。user-service 的 Ent schema、client、entity、predicate、`enttest` 和 `migrate` 生成物 MUST 位于 `user-service/internal/persistence/ent/`，并通过 Go `internal` 规则作为 user-service 私有持久化实现受保护。仓库 MUST 支持生成、diff、validate 和 hash 校验，但 MUST NOT 提供自动连接目标数据库执行 `atlas migrate apply` 的入口，也 MUST NOT 保留模块根级 `github.com/aegiscore/user-service/ent` 兼容包、别名、shim 或双路径支持。

#### Scenario: Schema、migration 与数据库结构

- **WHEN** Ent schema 或生成特性变化
- **THEN** 协作者 MUST 执行 `make user-service-generate` 和 `make user-service-migrate-diff name=<migration-name>` 并审查 SQL 与 `atlas.sum`；interface、生成指令、Ent 生成物、SQL 或 hash 不一致时验证 MUST 失败
- **WHEN** 生成或审查 SQL migration
- **THEN** migration MUST NOT 包含 `FOREIGN KEY` 或 `REFERENCES`，并 MUST 保留 Ent edge、关联字段和必要唯一索引
- **WHEN** migration 使用 `gin_trgm_ops`
- **THEN** 首个 migration MUST 在索引前创建 `pg_trgm` 并提示 DBA 权限
- **AND** Atlas dev Dockerfile、diff 脚本、`atlas.hcl` 与 Compose 本地 image tag MUST 一致，lint MUST 检测 drift

#### Scenario: Migration 校验与受控执行

- **WHEN** migration 准备发布或 SQL 经手工调整
- **THEN** `make user-service-migrate-validate` MUST 校验 SQL 和 `atlas.sum`，手工调整后 MUST 刷新 hash 并重新验证
- **AND** SQL MUST 提交 Git，并通过 DBA 工单或受控发布平台执行
- **AND** user-service、E2E、Makefile、脚本和部署资产准备数据库时 MUST 使用已提交 migration，运行时代码 MUST NOT 使用 `client.Schema.Create(ctx)` 表达 schema 变更
- **AND** 仓库 MUST NOT 提供 `migrate-apply`、自动 migration Job 或等价 Atlas apply 入口

#### Scenario: Ent 生成包路径受 internal 保护

- **WHEN** 执行 `make user-service-generate` 或 Ent 生成入口
- **THEN** Ent 生成物 MUST 收敛到 `github.com/aegiscore/user-service/internal/persistence/ent` 及其子包
- **AND** 生成流程 MUST NOT 创建、更新或依赖 `user-service/ent/` 根级目录
- **WHEN** user-service 内部 provider、feature infrastructure、RBAC CLI、Atlas schema helper 或测试需要访问 Ent client、entity、predicate、`enttest` 或 `migrate`
- **THEN** 代码 MUST 导入 `github.com/aegiscore/user-service/internal/persistence/ent` 及其子包
- **AND** 代码 MUST NOT 导入 `github.com/aegiscore/user-service/ent` 及其子包
- **WHEN** 其他 workspace module 或未来服务尝试直接 import user-service 的 Ent 包
- **THEN** Go `internal` 规则 MUST 阻止该导入，调用方 MUST 通过正式服务边界访问 user-service 能力
