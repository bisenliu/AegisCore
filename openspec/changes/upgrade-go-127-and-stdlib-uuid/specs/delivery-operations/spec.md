## ADDED Requirements

### Requirement: Go 1.27 工具链一致性

仓库 MUST 使用 Go 1.27 当前最新稳定补丁版 1.27.3 作为统一构建工具链版本；workspace、全部一方 Go module、CI workflow、Docker builder 和现行文档 MUST 保持版本一致。UUID 类型迁移后的 Ent 生成物 MUST 与 schema 一致，PostgreSQL UUID 列 MUST 继续使用数据库 `uuid` 类型。

#### Scenario: 仓库与 CI 使用一致版本

- **WHEN** 执行仓库架构检查或 CI 构建
- **THEN** `go.work`、`common`、`user-service`、`tools/openapi-convert`、`tools/nacos-config-seed`、CI workflow 和 Docker builder MUST 指向 Go 1.27.3
- **AND** 架构版本检查及其 fixture MUST 覆盖全部四个 module，并能发现版本不一致

#### Scenario: UUID 持久化结构不漂移

- **WHEN** 从 Ent schema 重新生成客户端并比较 Atlas 迁移状态
- **THEN** Ent 生成代码 MUST 使用标准库 UUID 的可读写映射，现有 PostgreSQL UUID 列 MUST 保持 `uuid` 类型
- **AND** 仅因 Go UUID 类型切换 MUST NOT 产生 SQL 结构变更或空 migration
