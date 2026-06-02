## Why

现有 `user-authentication` capability 已具备 JWT Bearer 验签、过期校验、issuer/audience 校验和认证用户上下文传播，但认证状态仍主要依赖 Access Token 自身有效期，缺少服务端可撤销会话和用户级统一失效机制。为支持修改密码、管理员强制下线、退出全部设备、安全事件处置以及单设备退出，需要在兼容现有 Gin/Fx/Ent/Redis 基础设施的前提下，引入“短期 Access Token + 可撤销 Refresh Token + 用户级 token_version”认证状态模型。

## What Changes

- 在用户表中新增 `token_version` 字段，默认值为 `1`，作为 PostgreSQL 持久化的用户安全状态真值。
- 扩展 Access Token claims，加入 `token_version` 和 `session_id`，继续保留现有 `user_id`、`exp`、`iss`、`aud` 语义。
- 修改认证中间件流程：验签和标准 claims 校验通过后，按 `user_id` 优先读取 Redis 缓存的服务端 `token_version`，缓存未命中时查询 PostgreSQL 并回填 Redis；版本不一致时返回未认证。
- 新增 Refresh Token 会话能力：登录后创建 Redis 会话记录和用户会话索引，刷新时校验 Refresh Token、Redis 会话状态与当前 `token_version`，并支持推荐的 Refresh Token 轮转。
- 明确 Refresh Token 输入契约：受保护 API 的 `Authorization` header 继续必须使用 `Bearer <token>`，刷新接口请求体 `refresh_token` 字段首选裸 token；服务端兼容可选 `Bearer ` 前缀并在解析前规范化。
- 新增认证 API：登录、刷新 Access Token、退出当前设备、退出全部设备；这些接口使用现有 `common/response.Envelope` 响应契约。
- 用户级安全事件处理遵循“先更新 PostgreSQL，再删除 Redis 缓存/会话”的一致性原则；不使用 `iat` 作为安全失效判断依据。
- 继续复用现有 `cache_redis`、`user_db`、JWT 配置、Zap 日志、trace-id 和 Fx 依赖注入边界；不引入 Redis 作为 token_version 真值源。

## Capabilities

### New Capabilities

- `user-session-control`: 覆盖登录、Refresh Token 会话存储与轮转、退出当前设备、退出全部设备，以及 Redis 会话 key/index 约定。

### Modified Capabilities

- `user-authentication`: 将现有 JWT Bearer 认证从纯 token 验签扩展为包含 `token_version` 服务端版本校验和 `session_id` 上下文传播的认证流程。
- `user-profile-create`: 用户创建时必须持久化或依赖数据库默认值初始化 `token_version=1`，并保留现有创建用户 API 的响应兼容性。
- `database-schema-migrations`: 用户 Ent schema 变更必须通过 Atlas 在 `user-services/migrations/` 生成新增 `token_version` 字段的 SQL migration，不得通过运行时 `Schema.Create` 自动改表。
- `shared-infrastructure`: 认证会话能力复用用户服务已声明的 `cache_redis` 和 `user_db`，并要求配置加载支持 Refresh Token TTL 等认证配置字段但仍不在 `common/config.Load` 阶段执行校验。

## Impact

- 代码模块：`common/jwt/`、`common/middleware/auth.go`、`common/contextutil/auth.go`、`common/config/config.go`、`user-services/internal/bootstrap/`、`user-services/internal/router/router.go`、`user-services/internal/controller/`、`user-services/internal/service/`、`user-services/internal/repository/`、`user-services/ent/schema/user.go`。
- 数据模型：`users` 表新增 `token_version` 非空整数字段，默认 `1`；Ent 生成代码和 Atlas migration 需要同步更新。
- Redis key：新增用户版本缓存、Refresh Token 会话记录、用户活跃会话索引；Redis 只作为缓存和会话层，不作为 token_version 真值。
- API：新增登录、刷新、退出当前设备、退出全部设备接口；现有 `/api/v1/users` 相关接口继续受 Bearer 认证保护；刷新接口的 `refresh_token` 请求体字段接受裸 token，并兼容 `Bearer <token>` 输入。
- 配置：需要补充 Refresh Token TTL、Access Token TTL 签发使用、版本缓存 TTL、Refresh Token 轮转策略等认证配置；配置加载仍保持只反序列化不校验。
- 兼容性：现有没有登录/刷新/登出接口，因此新增 API 不破坏已存在路由；但 Access Token claims 变为必须携带 `token_version`，旧测试或手工签发的 token 需要调整，否则会被认证中间件拒绝。
