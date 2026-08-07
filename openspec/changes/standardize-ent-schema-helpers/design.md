## Context

当前 `user-service/internal/persistence/ent/schema` 中多个 schema 直接重复声明 `created_at`、`updated_at` 字段，并分别内联 `time.Now().UnixMilli()` 默认值、`UpdateDefault` 和中文注释。`rbacpolicyoutboxevent.go` 还私有定义了 `oneOfStrings`，后续其他 schema 如果需要字符串枚举校验容易复制实现。

Ent schema 是 user-service 的数据库结构来源，Atlas SQL migration 是可审查交付工件。即使本次目标是实现去重，改动仍会触及 schema 源文件和 Ent 生成物，因此必须把“无意改变数据库结构”的约束放入设计和验证流程。

## Goals / Non-Goals

**Goals:**

- 在 schema 包内部提取时间戳 mixin，统一 `created_at` 和 `updated_at` 的字段注释、默认值和更新策略。
- 在 schema 包内部提取字符串枚举校验 helper，避免 `oneOfStrings` 继续停留在单个 schema 文件中。
- 替换现有重复字段声明，保持字段名、类型、storage key、注释、默认时间语义和 `UpdateDefault` 行为不变。
- 重新生成 Ent 代码，并通过 Atlas migration diff/validate 审查生成物和数据库结构漂移。

**Non-Goals:**

- 不修改 HTTP API、OpenAPI、业务 DTO、controller、application use case 或 domain 语义。
- 不改变 user、role、permission、RBAC policy revision 或 outbox 表的列、索引、唯一约束、edge 或业务含义。
- 不把 schema helper 放入 `common/`、`user-service/internal/shared` 或 feature 包。
- 不新增自动 migration apply、部署清单、观测资产或发布流程。

## Decisions

### Decision: helper 只放在 Ent schema 包内部

将时间戳 mixin 和 `oneOfStrings` 放在 `user-service/internal/persistence/ent/schema` 包内，供本包 schema 类型复用。

理由：这些 helper 只服务 Ent schema 定义，不是跨服务 primitive，也不属于 user-service feature 业务内核。放在 schema 包内部可以避免污染 `common/` 和 `internal/shared` 边界。

备选方案：放入 `common/runtime` 或 `user-service/internal/shared`。该方案会把持久化实现细节扩散到共享边界，违反当前架构约束，因此不采用。

### Decision: 使用独立 mixin 表达 created_at 和 updated_at

提取 `createdAtMillisMixin` 和 `updatedAtMillisMixin`，分别声明 `created_at` 与 `updated_at` 字段。需要两个字段的 schema 同时引入两个 mixin；只需要创建时间的关联表或 revision 表只引入 `createdAtMillisMixin`。

理由：现有 schema 中不是所有表都有 `updated_at`，拆分 mixin 可以保持字段集合最小化，并避免给关联表或 revision 表引入非预期列。

备选方案：提供一个组合 `timestampMillisMixin` 一次性声明两个字段。该方案对只需要 `created_at` 的 schema 不适用，容易引入额外字段，因此不采用。

### Decision: 保持毫秒时间语义和 Ent 更新策略不变

时间字段 helper 继续使用 `time.Now().UnixMilli()`。`created_at` 必须保持 `Immutable()`；`updated_at` 必须保持 `UpdateDefault(func() int64 { return time.Now().UnixMilli() })`。

理由：当前业务和测试默认时间字段是毫秒级 Unix timestamp，变更为数据库默认值、秒级 timestamp、`time.Time` 或自定义 clock 都会改变数据语义或生成物，不属于本次范围。

备选方案：改用数据库 `DEFAULT` 或 `time.Time` 字段。该方案会影响列类型、migration 和业务序列化语义，因此不采用。

### Decision: migration diff 作为无结构漂移门禁

实施后必须执行 `make user-service-generate` 和 `make user-service-migrate-diff name=standardize-ent-schema-helpers`，并审查是否产生 SQL；随后执行 `make user-service-migrate-validate`。

理由：Ent mixin 可能改变生成代码来源和 annotation 展开方式，Atlas diff 是确认数据库结构是否漂移的必要门禁。

备选方案：只运行 Go 测试。该方案无法覆盖 Ent 生成物和 Atlas schema 判断，不足以证明 schema 重构安全，因此不采用。

## Risks / Trade-offs

- [Risk] mixin 引入顺序改变 Ent 生成字段顺序或生成代码 diff。Mitigation：审查 `make user-service-generate` 后的 diff，确认仅为预期来源变化或无差异。
- [Risk] 某个 schema 错误引入 `updatedAtMillisMixin` 导致新增列。Mitigation：逐个核对现有 `created_at`、`updated_at` 使用点，并用 Atlas diff 检测非预期 SQL。
- [Risk] 时间字段注释或默认策略与现有声明不完全一致。Mitigation：helper 中集中定义中文注释、`Immutable()`、`DefaultFunc` 和 `UpdateDefault`，并运行 schema 相关测试。
- [Risk] `oneOfStrings` 提取后测试覆盖不足。Mitigation：保留或调整现有 outbox schema 测试，确保枚举校验失败路径仍被覆盖。

## Migration Plan

实施步骤是代码重构和生成校验，不需要运行时数据迁移。发布时随普通 user-service 代码发布即可，前提是 Atlas diff 未产生非预期 SQL。

如果生成了非预期 migration SQL，应停止实施并回到 schema helper 设计检查字段声明差异；不得提交用于迁移无意结构变化的 SQL。回滚方式是恢复 schema helper 替换前的字段声明并重新运行生成和 validate。

## Open Questions

无。
