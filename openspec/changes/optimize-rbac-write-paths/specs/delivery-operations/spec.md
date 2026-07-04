## MODIFIED Requirements

### Requirement: 代码生成与数据库迁移

系统 MUST 提供 Ent 代码生成、Atlas migration diff、migration validate 和 migration hash 校验入口，并要求 schema 相关变更同步生成物。系统 MUST NOT 提供通过仓库 Makefile、脚本或部署资产直接连接数据库并执行 `atlas migrate apply` 的入口。user-service Ent 生成配置 MUST 启用支持 RBAC bulk insert 唯一冲突忽略所需的生成特性。

#### Scenario: 生成 Ent 代码

- **WHEN** Ent schema 或 Ent 生成特性变化后执行 `make user-service-generate`
- **THEN** 系统 MUST 运行 `go generate ./ent` 并更新 Ent 生成代码
- **AND** 生成代码 MUST 支持 RBAC 批量写入路径使用 bulk create 的唯一冲突忽略能力

#### Scenario: 生成 migration

- **WHEN** 数据库 schema 变化需要生成 migration
- **THEN** 协作者 MUST 执行 `make user-service-generate` 和 `make user-service-migrate-diff name=<migration-name>` 生成 Ent 代码与 Atlas migration，并审查 SQL 与 `atlas.sum`

#### Scenario: 校验 migration

- **WHEN** migration 准备进入环境或发布流程
- **THEN** 系统 MUST 支持 `make user-service-migrate-validate` 校验已提交 SQL migration 和 `atlas.sum`
- **AND** 系统 MUST NOT 支持通过 `DATABASE_URL` 执行 `make user-service-migrate-apply` 或等价仓库命令连接数据库自动应用 migration

#### Scenario: 手动调整 migration SQL

- **WHEN** 生成的 SQL migration 被手动调整
- **THEN** 协作者 MUST 刷新并提交 `atlas.sum`，且 MUST 确保 `make user-service-migrate-validate` 通过

#### Scenario: 受控执行 SQL migration

- **WHEN** SQL migration 已通过 validate 并准备进入目标数据库
- **THEN** 协作者 MUST 将 SQL migration 和权限要求提交到 Git，并通过 DBA 工单或受控发布平台人工或受控执行
- **AND** 仓库文档 MUST 将标准流程描述为 Ent schema -> Atlas diff 生成 SQL -> Atlas validate/hash 校验 SQL 目录 -> SQL 进 Git -> DBA 工单或受控发布平台执行

#### Scenario: pg_trgm 扩展前置

- **WHEN** SQL migration 使用 `gin_trgm_ops` 或其他 `pg_trgm` 能力
- **THEN** 首个 SQL migration 文件 MUST 在相关索引创建前包含 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`
- **AND** 文档或任务 MUST 提醒生产库执行该语句可能需要 DBA 权限

#### Scenario: 运行时不修改 schema

- **WHEN** user-service 正常启动或 E2E 初始化数据库 schema
- **THEN** schema MUST 来自已提交 Atlas SQL migration，运行时服务代码 MUST NOT 使用 `client.Schema.Create(ctx)` 表达 schema 变更

#### Scenario: Ent 生成特性 drift 检查

- **WHEN** user-service Ent 生成特性发生变化但生成物未同步
- **THEN** `make verify` 或等价完整验证 MUST 通过重新生成和 `git diff --exit-code` 暴露 drift
