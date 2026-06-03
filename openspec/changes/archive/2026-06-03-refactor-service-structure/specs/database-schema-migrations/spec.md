## ADDED Requirements

### Requirement: Organize Ent schema files by domain without changing database schema
系统 SHALL 将用户服务 Ent schema 源文件实际组织为可扩展的领域分类。当前 `User` schema MUST 纳入用户领域分类，并且根 `schema` package MUST 继续作为 Ent codegen 和 Atlas schema source 的稳定入口。该分类变更 MUST NOT 改变 `User` 字段、索引、字段注释、表结构或 migration 历史。

#### Scenario: User schema is moved into domain classification
- **Given** 当前用户服务仅包含 `User` Ent schema
- **When** 本次结构重构完成
- **Then** `User` schema MUST 位于用户领域分类路径中
- **Then** 根 `user-services/ent/schema` package MUST 继续向 Ent codegen 暴露 `User` schema
- **Then** `User` 字段、索引和注释语义 MUST 与分类前保持一致
- **Then** 本次变更 MUST NOT 因目录规划生成新的数据库 migration

#### Scenario: Ent generation reads categorized schema
- **Given** `User` schema 已移动到领域分类路径
- **When** 开发者在 `user-services` 模块运行 `go generate ./ent`
- **Then** Ent codegen MUST 成功读取根 `schema` package 暴露的 `User` schema
- **Then** 生成的 `user-services/ent/` 代码 MUST 与 `User` 字段和索引语义保持一致
- **Then** 实现 MUST NOT 手写 `user-services/ent/` 下的生成代码表达 schema 分类变更

#### Scenario: Atlas source remains stable after schema categorization
- **Given** Ent schema 已按领域分类且根 `schema` package 保持聚合入口
- **When** Atlas 使用 `ent://ent/schema` 或等价配置读取用户服务 schema source
- **Then** Atlas MUST 能读取完整 `User` schema
- **Then** 未发生字段或索引语义变更时 MUST NOT 生成新的数据库结构 migration
