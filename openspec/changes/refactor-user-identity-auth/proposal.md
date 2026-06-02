## Why

当前用户模型以自增 `id` 和 `email` 作为主要对外身份与登录凭据，既会暴露内部数据库主键，也不符合后续统一用户身份标识和用户名登录的需求。需要将外部身份改为稳定 UUID `user_id`，并将登录凭据切换为 `username`，同时把密码哈希能力沉淀到 `common`，避免认证逻辑重复或继续依赖不符合要求的加密方式。

## What Changes

- **BREAKING** 移除用户表和用户 API 响应中的 `email` 字段，新增必填且唯一的 `username` 字段，创建用户时改为提交和校验 `username`。
- **BREAKING** 用户表新增唯一且不可变的 `user_id` UUID 字段，创建用户时由服务端自动生成 UUIDv7；所有用户资料对外响应不再暴露内部 `id`，统一返回 `user_id`。
- **BREAKING** 登录请求从邮箱登录改为用户名登录，认证服务按 `username` 查询用户并校验密码。
- **BREAKING** 移除用户服务中按 `email` 查询、检查、过滤或登录的公开与内部方法，统一改为 `username` 语义和命名。
- 新增 `common` 模块统一密码能力，提供 Argon2id 密码哈希和密码校验方法，用户创建与登录校验调用该公共方法。
- 通过 Ent schema 和 Atlas migration 表达 `users` 表字段变更：删除 `email`、新增 `username`、新增 `user_id`，并更新相关唯一约束和字段注释。
- 更新 controller/service/repository/DTO/Swagger/测试，使 API 响应继续使用 `common/response.Envelope`，但 `data` 中公开字段改为 `user_id`、`username`、`name`、`active`、`created_at`、`updated_at`。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `user-profile-create`: 创建用户请求、唯一身份、持久化字段、密码哈希和成功响应字段变更。
- `user-profile-query`: 查询用户响应字段从内部 `id`/`email` 调整为外部 `user_id`/`username`，并保持不返回密码哈希。
- `user-list-query`: 列表响应字段和过滤参数从 `email` 调整为 `username`，并移除邮箱过滤语义。
- `user-authentication`: JWT 认证上下文中的用户身份必须使用稳定外部 `user_id`，避免依赖内部数据库 `id`。
- `user-session-control`: 登录凭据从 `email` 改为 `username`，登录密码校验改为公共 Argon2id 方法，token/session 中的用户身份使用 `user_id`。
- `database-schema-migrations`: 用户表 schema 变更必须通过 Ent schema 和 Atlas SQL migration 表达，迁移目录和 `atlas.sum` 必须同步维护。
- `shared-infrastructure`: `common` 模块新增可复用密码哈希与校验公共方法，不应创建额外运行时依赖。
- `api-response-contract`: 用户相关响应 envelope 保持不变，但用户资料 `data` 字段集合发生破坏性变化。

## Impact

- 数据模型：`user-services/ent/schema/user.go`、Ent 生成代码、`user-services/migrations/` 和 `atlas.sum` 需要更新；内部主键 `id` 仍可作为数据库主键保留，但不得映射到外部用户资料响应；数据库层面必须删除 `email` 列和相关唯一约束。
- API：`POST /api/v1/users` 请求从 `email` 改为 `username`，成功响应、`GET /api/v1/users/:user_id` 响应和 `GET /api/v1/users` 列表响应不再包含 `id`、`email`；用户列表过滤从 `email` 改为 `username`；登录请求从邮箱字段改为用户名字段。
- 认证：登录 repository 按 `username` 查找用户；`ExistsByEmail`、`GetByEmail`、`Email` 过滤输入、email 日志字段和 email 测试夹具必须移除或重命名为 username 语义；JWT claims、认证上下文、session store 与 token version 校验以 `user_id` 表达当前用户身份，并按需要在内部回源映射到数据库记录。
- 安全：用户创建时必须调用 `common` Argon2id 哈希方法后再持久化密码；登录时必须调用 `common` Argon2id 校验方法，不得记录明文密码、哈希参数或完整 hash。
- 依赖：可能需要新增 Go 官方推荐 UUIDv7 生成依赖或使用 Go 生态推荐库；Argon2id 使用 `golang.org/x/crypto/argon2` 并在 `common` 中封装参数、盐生成、编码和校验。
- 文档与测试：Swagger、DTO、controller/service/repository 测试、认证与会话相关测试需要同步调整，确保响应 envelope 结构不变且敏感字段不外泄。
