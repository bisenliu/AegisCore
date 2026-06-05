## ADDED Requirements

### Requirement: Preserve schema semantics when decoupling default value sources

系统 SHALL 在仅调整 Ent schema 默认值来源的重构中保持数据库 schema 语义不变。若字段类型、默认值数字、注释、索引和约束未变化，系统 MUST NOT 生成新的 Atlas SQL migration 或修改既有 migration 历史。

#### Scenario: User status default source changes without migration
- **Given** `User` Ent schema 的 `status` 字段默认值来源从业务 domain 常量改为 schema 本地持久化契约值
- **When** 开发者审查本次 schema source 变更
- **Then** `status` 字段的数据库默认值 MUST 继续为 `100`
- **Then** 用户表字段类型、字段注释、索引和约束 MUST 保持不变
- **Then** `user-services/migrations/` 中 MUST NOT 因本次默认值来源重构新增 SQL migration
- **Then** 既有 `atlas.sum` MUST NOT 因无数据库语义变化而修改

#### Scenario: Ent generation remains the only generated-code path
- **Given** Ent schema 默认值来源已与业务 domain 解耦
- **When** 开发者需要刷新 Ent 生成代码
- **Then** 开发者 MUST 在 `user-services` 模块运行 `go generate ./ent`
- **Then** 实现 MUST NOT 手写 `user-services/ent/` 下的生成代码
- **Then** 生成结果 MUST 与用户表 `status` 字段的既有数据库默认值语义保持一致
