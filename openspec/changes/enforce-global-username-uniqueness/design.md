## Context

用户创建能力当前以 `username` 作为创建用户的唯一业务身份，但主规格仍要求 service 执行用户名存在性检查，并在数据库唯一冲突时兜底转换错误。软删除相关迁移规格也允许在全表唯一和 `deleted_at IS NULL` partial unique index 之间选择。该组合会使软删除后用户名是否可复用、大小写用户名是否等价、并发创建时以预查还是数据库约束为准出现歧义。

本变更将 `username` 定义为全表全局唯一的规范化登录名，创建时统一转为小写；`nickname` 保持展示名语义，可重复；跨业务引用用户必须使用外部 `user_id`。

## Goals / Non-Goals

**Goals:**

- 明确创建用户输入清洗：`username` 在写入前统一小写，`nickname` 只裁剪和校验展示名，不参与唯一约束。
- 移除创建路径的 `ExistsByUsername` 预查，避免并发窗口和重复查询。
- 以数据库 `UNIQUE(username)` 作为用户名全局唯一最终约束，软删除后不释放用户名。
- 保持 controller/service/repository 分层：controller 处理请求绑定和基础校验，service 编排密码哈希和领域错误映射，repository 处理 Ent/PostgreSQL 写入和数据库错误转换。
- 通过 Ent schema、生成代码和 Atlas migration 表达数据库约束变更。

**Non-Goals:**

- 不新增用户删除、恢复、更新用户名或修改昵称 API。
- 不改变创建用户 API 路径、认证要求、成功响应字段或统一响应信封结构。
- 不把 `username` 改为对外业务引用键；业务引用继续使用 `user_id`。

## Decisions

1. `username` 创建时统一小写。

   理由：数据库唯一约束通常按实际存储值判断，写入前小写可避免 `Alice` 与 `alice` 产生两个业务等价账号。相比依赖数据库表达式索引或不区分大小写类型，应用侧规范化更直接，且与现有 Go 校验/DTO 流程兼容。

2. 使用全表 `UNIQUE(username)`，不使用 `WHERE deleted_at IS NULL` 的 partial unique index。

   理由：软删除后不释放用户名是长期业务规则，全表唯一可以让数据库直接保障该规则。partial unique index 会允许软删除后复用用户名，不符合本变更目标。

3. 创建流程不做 `ExistsByUsername` 预查。

   理由：预查无法避免并发写入竞争，还会引入额外查询和与数据库约束不一致的风险。repository 在 create 时统一捕获 Ent 唯一约束错误并转换为 `ErrUserAlreadyExists`，service 再映射为 409。

4. `nickname` 仅作为展示名，可重复。

   理由：展示名不是身份标识，不应影响创建唯一性，也不应作为业务引用键。

5. 所有业务引用继续使用 `user_id`。

   理由：`user_id` 是稳定、不可变、对外 UUID 身份；`username` 是登录名/唯一账号名，不适合作为跨业务关系引用字段。

## Risks / Trade-offs

- [Risk] 既有数据存在大小写不同但小写后相同的用户名，或软删除与未删除记录用户名重复 -> Mitigation: migration review 必须包含数据冲突检查和部署前清理策略，不能静默创建唯一约束失败。
- [Risk] 移除预查后部分测试仍期望 service 调用 `ExistsByUsername` -> Mitigation: 更新单元测试以验证 repository unique conflict 到 `ErrUserAlreadyExists` 再到 409 的路径。
- [Risk] Ent 生成代码和 SQL migration 不一致 -> Mitigation: 修改 Ent schema 后运行 `go generate ./ent`，再通过 Atlas 生成/审查 migration，并校验 `atlas.sum`。

## Migration Plan

1. 更新 Ent `User` schema 中 `username` 的唯一索引策略，确保全表唯一且不包含 `deleted_at` partial 条件。
2. 运行 `go generate ./ent` 更新 Ent 生成代码，不手写 `user-services/ent/` 下文件。
3. 在 `user-services/` 运行 migration diff，审查 SQL 是否表达全表 `UNIQUE(username)`，并检查现有数据冲突风险。
4. 如人工调整 SQL，运行 `atlas migrate hash --dir file://migrations` 后再执行迁移校验脚本。
5. 部署前先应用 migration；若唯一约束创建失败，应停止发布并先处理数据冲突。

## Open Questions

- 是否需要为历史数据提供一次性用户名小写回填 SQL，由实现阶段根据当前 migration 历史和目标库数据决定。
