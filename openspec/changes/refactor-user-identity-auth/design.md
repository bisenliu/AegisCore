## Context

AegisCore 当前用户服务使用 Ent `User` schema 和 `users` 表承载用户资料，外部创建与查询响应包含内部自增 `id` 和 `email`，登录能力按邮箱读取用户并校验持久化密码。该设计需要调整为用户名登录和 UUID 外部用户标识，以避免暴露数据库主键，并为认证、会话和后续跨服务引用提供稳定身份。

本变更横跨 `common` 和 `user-services`：`common` 新增密码哈希公共方法；`user-services` 调整 Ent schema、Atlas migration、DTO、controller/service/repository、认证与会话调用点、Swagger 和测试。HTTP 响应仍使用 `common/response.Envelope`，controller/service/repository 分层职责保持不变。

## Goals / Non-Goals

**Goals:**

- 将用户持久化模型从 `email` 业务身份迁移到 `username`，并新增唯一不可变的 UUID `user_id`。
- 创建用户时自动生成 UUIDv7 `user_id`，对外用户资料统一返回 `user_id` 而不是内部 `id`。
- 登录请求使用 `username`，认证与会话中的用户身份使用外部 `user_id`。
- 在 `common` 模块封装 Argon2id 密码哈希与校验方法，用户创建和登录统一调用。
- 通过 Ent schema、Ent 生成代码和 Atlas SQL migration 完成数据库结构变更。

**Non-Goals:**

- 不实现邮箱登录兼容、邮箱找回密码、用户名修改、用户删除或细粒度授权。
- 不引入运行时自动 schema create，不通过 HTTP runtime 修改数据库结构。
- 不把 Argon2id 参数做成运行时配置；本次使用公共模块内稳定默认参数。
- 不改变响应 envelope 的 `success`、`code`、`message`、`data` 外层结构。

## Decisions

1. 内部主键保留，外部身份使用 `user_id` UUIDv7。

   `users.id` 继续作为 Ent 和数据库内部主键，避免大规模破坏 Ent 关系、迁移历史和内部查询性能；新增 `user_id` 作为唯一、不可变、非空字段，并在所有用户资料响应、JWT claims、认证上下文和会话索引中使用。备选方案是把 Ent 主键直接改为 UUID，但这会显著扩大 migration 风险并影响现有内部代码路径。

2. UUIDv7 由服务端在创建用户时生成。

   repository 创建记录前接收 service 生成的 UUIDv7，或在 repository 内调用封装的生成方法；推荐放在 service 层表达业务身份生成，repository 只负责持久化。Go 官方推荐的时间有序 UUIDv7 方案应通过维护良好的 Go UUID 库实现，并在实现时确认库支持 UUIDv7、随机源和字符串格式输出。

3. `email` 被 `username` 替代为唯一业务身份。

   Ent schema 删除 `email` 字段，新增 `username` 字段，设置非空、最大长度、唯一索引和字段注释。创建 DTO、登录 DTO、列表过滤 DTO 和响应 DTO 使用 `username`，校验必填和长度，不再执行邮箱格式校验。登录 repository 从按邮箱查询改为按用户名查询。

   代码层面必须移除 email 语义方法和字段，而不是保留空壳兼容：`ExistsByEmail` 改为 `ExistsByUsername`，`GetByEmail` 改为 `GetByUsername`，列表查询输入中的 `Email` 改为 `Username`，Ent 查询谓词从 `user.EmailEQ` 改为 `user.UsernameEQ` 或用户名匹配策略。日志字段也从 `email` 改为 `username`，并避免记录密码。

4. 查询接口使用外部 `user_id`。

   路由从 `GET /api/v1/users/:id` 调整为 `GET /api/v1/users/:user_id`，controller 校验 UUID 字符串格式，service/repository 按 `user_id` 查询。这样 URL、响应和认证上下文都不再暴露内部数据库主键。备选方案是保留路径 `:id` 但仅修改响应字段；该方案仍会在外部 API 中暴露内部 ID，不满足统一外部身份目标。

5. 密码能力放入 `common/password`。

   新增 `common/password` 包，提供 `Hash(plain string) (string, error)` 和 `Verify(plain, encodedHash string) (bool, error)`。哈希使用 `golang.org/x/crypto/argon2` 的 Argon2id，随机盐使用 `crypto/rand`，编码格式包含算法版本、参数、salt 和 hash，例如 `$argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>`。调用点不直接接触参数、盐或底层 Argon2 API。

6. 密码错误映射保持认证失败语义。

   创建用户时 `Hash` 失败映射为内部错误，不记录明文密码。登录时用户不存在、hash 格式非法或密码校验失败均返回统一凭据无效响应，避免泄露账号存在性；底层解析错误只进入安全日志，不包含明文密码或完整 hash。

7. 数据库迁移通过 Ent 和 Atlas 完成。

   修改 `user-services/ent/schema/user.go` 后在 `user-services` 运行 `go generate ./ent`，再运行 `./scripts/migrate-diff.sh refactor_user_identity_auth` 生成 SQL migration，并运行 `./scripts/migrate-validate.sh` 校验 `atlas.sum`。migration 需要表达新增 `user_id`、新增 `username`、删除 `email` 列、删除 email 唯一约束和新增 username/user_id 唯一约束。已有数据迁移策略应在 SQL review 中明确：若存在历史用户，需要为 `user_id` 回填 UUIDv7 或数据库可接受的 UUID 值，并为 `username` 提供可审查的回填来源；没有可用来源时部署前必须制定人工迁移方案。迁移完成后应用代码不得再引用 Ent 生成代码中的 `Email` 字段或 `EmailEQ` 谓词。

## Risks / Trade-offs

- [Risk] 删除 `email` 与登录/列表过滤字段变更会破坏现有客户端。→ Mitigation：在 proposal、spec、Swagger 和 release notes 中明确破坏性变更，测试覆盖旧字段不再接受。
- [Risk] 现有数据无法自动推导 `username`。→ Mitigation：migration SQL review 阶段要求明确回填策略；无法安全推导时阻止自动迁移并采用人工数据准备。
- [Risk] Argon2id 参数过高导致登录 CPU/内存压力。→ Mitigation：在 `common/password` 中使用稳定默认值并提供单元测试，后续如需调参再单独提变更。
- [Risk] JWT/session 仍有旧代码使用内部 `id`。→ Mitigation：集中调整 claims、contextutil、auth service、session store 和 token version 查询调用点，并增加测试验证 claims `user_id` 为 UUID 字符串。
- [Risk] Ent 生成代码或 migration 校验遗漏。→ Mitigation：任务中要求 `go generate ./ent`、`go test ./...` 和 migration validate；不得手写 `user-services/ent/` 生成代码。

## Migration Plan

1. 修改 `common/password` 并增加 Argon2id 哈希与校验测试。
2. 修改 `user-services/ent/schema/user.go`，新增 `user_id` 和 `username`，删除 `email`，更新唯一索引、字段注释和默认值策略。
3. 在 `user-services` 运行 `go generate ./ent`，生成 Ent 代码。
4. 生成并审查 Atlas migration，确认 `users` 表字段、唯一约束、历史数据回填和 `atlas.sum`。
5. 调整 DTO、controller、service、repository、auth/session 逻辑和 Swagger，删除或重命名所有 email 语义方法、字段、日志字段、测试夹具和文档示例。
6. 运行 `common` 与 `user-services` 测试，并运行 migration validate。
7. 部署时先 apply migration，再启动 HTTP runtime。

Rollback 需要按数据库迁移状态决定：若 migration 已删除 `email`，回滚必须依赖备份或额外回滚 migration 恢复字段；应用层可回滚到旧版本前必须确保数据库 schema 兼容旧代码。

## Open Questions

- 历史用户的 `username` 回填来源需要在实施前确认；如果当前环境没有生产历史数据，可在 migration 中按测试数据策略处理。
- UUIDv7 具体 Go 库需要在实现时基于当前依赖兼容性确认。
