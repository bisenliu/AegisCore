## Why

当前用户创建规格同时包含用户名存在性预查和数据库唯一约束兜底，且软删除场景下用户名唯一性策略仍允许实现选择，容易导致创建路径、并发冲突处理和软删除后用户名复用语义不一致。需要将 `username` 明确为全局唯一且创建时统一小写的登录/业务标识，依赖数据库唯一约束作为最终一致性边界。

## What Changes

- 创建用户时必须在持久化前将 `username` 规范化为小写，`nickname` 仅作为展示名且可重复。
- `username` 必须全表全局唯一；软删除用户不得释放原 `username`。
- 创建流程不得执行 `ExistsByUsername` 预查，必须依赖数据库 `UNIQUE(username)` 约束处理并发和重复创建。
- repository 必须将创建时的数据库唯一冲突转换为用户领域 `ErrUserAlreadyExists`。
- service 必须将 `ErrUserAlreadyExists` 转换为 HTTP 409 用户已存在响应，保持统一失败信封和业务码 `40000`。
- 所有业务引用用户身份时必须使用外部 `user_id`，不得把 `username` 作为跨业务引用键。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-profile-create`: 修改用户创建的 `username` 规范化、唯一性冲突处理、软删除后不释放用户名、`nickname` 展示名语义和业务引用身份要求。
- `database-schema-migrations`: 修改用户表用户名唯一索引策略，要求全表 `UNIQUE(username)`，不允许 `deleted_at IS NULL` partial unique index 释放软删除用户名。

## Impact

- 影响 `user-services/internal/controller`、`user-services/internal/service`、`user-services/internal/repository` 与 `user-services/internal/repository/postgres` 的创建用户流程和错误映射。
- 影响 `user-services/ent/schema` 的用户名索引语义，以及 `user-services/migrations/` 中 Atlas SQL migration 和 `atlas.sum`。
- API 路径和响应信封结构不变；重复用户名创建继续返回 HTTP 409、业务码 `40000`、用户已存在文案。
- 数据兼容性风险：如果目标数据库已有仅大小写不同或软删除后重复的 `username`，迁移到全局唯一约束前必须清理或合并数据。
