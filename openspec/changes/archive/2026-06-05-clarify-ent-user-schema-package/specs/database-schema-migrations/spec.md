## ADDED Requirements

### Requirement: Avoid Ent schema subpackage ambiguity with generated packages

系统 SHALL 在按领域拆分用户服务 Ent schema source 时，为 `user-services/ent/schema/` 下的 schema 子包使用能明确表达 schema source 语义的包名。schema 子包名称 MUST 避免与 Ent codegen 生成的查询包名称混淆；根 `user-services/ent/schema` package MUST 继续作为 Ent codegen 和 Atlas schema source 的稳定入口。该命名优化 MUST NOT 改变 `User` 字段、索引、字段注释、表结构或 migration 历史。

#### Scenario: User schema subpackage uses explicit schema name
- **WHEN** 用户服务将 `User` schema 的字段和索引定义保留在根 `schema` package 之外的子包中
- **THEN** 子包名称 MUST 明确表达 schema source 语义，例如 `userschema`
- **THEN** 子包名称 MUST NOT 与 Ent 生成查询包 `github.com/aegiscore/user-services/ent/user` 同名为 `user`

#### Scenario: Ent generation remains stable after schema subpackage rename
- **WHEN** 开发者在 `user-services` 模块运行 `go generate ./ent`
- **THEN** Ent codegen MUST 继续通过根 `user-services/ent/schema` package 读取完整 `User` schema
- **THEN** 实现 MUST NOT 手写 `user-services/ent/` 下的生成代码表达该命名优化

#### Scenario: Schema subpackage rename does not create migration
- **WHEN** 本次命名优化仅移动或重命名 schema source 子包且未改变字段、索引、字段注释、默认值或约束
- **THEN** `user-services/migrations/` MUST NOT 因本次变更新增 SQL migration
- **THEN** 既有 `atlas.sum` MUST NOT 因无数据库语义变化而修改
