## ADDED Requirements

### Requirement: Ent schema 内部 helper 重构校验

当 user-service Ent schema 通过 schema 包内部 helper 或 mixin 收敛重复字段、校验器或 annotation 声明时，仓库 MUST 保持 Ent schema 作为数据库结构来源，并 MUST 通过生成和 Atlas migration 校验证明该重构没有引入非预期数据库结构漂移。

#### Scenario: 时间戳 mixin 保持字段语义

- **WHEN** `user-service/internal/persistence/ent/schema` 中的 `created_at` 或 `updated_at` 字段改为由 schema 包内部 mixin 声明
- **THEN** 生成后的 Ent schema MUST 保持原有字段名、列类型、毫秒 Unix timestamp 默认值、中文注释和是否 `Immutable` 的语义
- **AND** `updated_at` 字段 MUST 保持基于 `time.Now().UnixMilli()` 的 `UpdateDefault` 更新策略

#### Scenario: 枚举校验 helper 保持 schema 内部归属

- **WHEN** 字符串枚举校验逻辑从单个 Ent schema 文件提取为共享 helper
- **THEN** helper MUST 留在 `user-service/internal/persistence/ent/schema` 包内，并 MUST NOT 放入 `common/`、`user-service/internal/shared` 或 feature 包
- **AND** 使用该 helper 的 schema MUST 保持原有允许值集合和校验失败语义

#### Scenario: 生成与 migration 校验无非预期漂移

- **WHEN** Ent schema helper 或 mixin 重构完成
- **THEN** 协作者 MUST 执行 `make user-service-generate` 和 `make user-service-migrate-diff name=standardize-ent-schema-helpers` 并审查 Ent 生成物、SQL migration 和 `atlas.sum`
- **AND** 协作者 MUST 执行 `make user-service-migrate-validate`
- **AND** 如 Atlas diff 产生 SQL，协作者 MUST 明确审查并确认 SQL 是否为预期；非预期表、列、索引、注释、默认值或约束变化 MUST 阻止本次 change 完成
