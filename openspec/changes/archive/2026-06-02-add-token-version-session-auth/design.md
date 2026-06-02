## Context

现有代码已经部分满足目标方案的基础前提：`common/jwt/service.go` 使用 `github.com/golang-jwt/jwt/v5` 解析 JWT，并校验 HMAC 签名、`exp`、可选 `iss` 和 `aud`；`common/middleware/auth.go` 已作为 Gin 全局中间件保护非白名单路径，并通过 `common/response.Envelope` 返回 401；`common/contextutil/auth.go` 已传播 `user_id`；`user-services/internal/bootstrap/bootstrap.go` 已通过 Fx 注入 JWT service、`cache_redis` 和 `user_db`；`common/infrastructure/redis.go` 和 PostgreSQL/Ent provider 已具备命名实例连接能力。

差距也很明确：当前没有登录接口、刷新接口、登出接口、Refresh Token、Redis 会话记录、用户会话索引、`token_version` 字段、版本缓存、密码哈希校验、修改密码/管理员强制下线流程；`Access Token` claims 仅包含 `user_id` 和标准 claims；认证中间件只做 token 自身校验，不读取 Redis 或 PostgreSQL；用户创建仍将明文 `password` 写入数据库，与目标认证链路所需的密码哈希存在安全冲突。

本变更应采取兼容性优先、最小改动优先策略：复用现有 JWT service、Gin 中间件、响应信封、Fx wiring、`cache_redis`、`user_db`、Ent repository 分层和 Atlas migration 工作流，在这些边界内补齐状态化认证能力，而不是重写现有服务结构。

## Goals / Non-Goals

**Goals:**

- PostgreSQL 持久化 `users.token_version`，默认从 `1` 开始，并作为认证状态最终真值。
- Access Token 携带 `user_id`、`token_version`、`session_id`、`exp`、`iss`、`aud`，认证中间件校验 token 版本和服务端版本一致。
- Redis 只承担 `token_version` 缓存、Refresh Token 会话记录和用户活跃会话索引，不作为 `token_version` 真值来源。
- 登录、刷新、退出当前设备、退出全部设备均通过 `common/response.Envelope` 返回，并遵循 controller/service/repository 分层。
- Refresh Token 支持可撤销会话，刷新链路必须检查 Redis 会话和当前 `token_version`，并优先实现 Refresh Token 轮转。
- 用户级安全事件按“先更新 PostgreSQL，再删除 Redis 缓存和会话”执行，禁止使用 `iat` 作为安全事件失效判断依据。

**Non-Goals:**

- 不在本变更中实现完整管理员后台、角色授权、权限点或审计系统。
- 不把 Redis 设计为认证安全状态的最终来源。
- 不用 JWT blacklist 替代 `token_version`。
- 不依赖运行时 `client.Schema.Create(ctx)` 做数据库结构变更。
- 不要求现有用户查询/创建 API 路径重命名。

## Decisions

### 1. 在现有 `user-authentication` 上扩展，而不是重写中间件

复用 `common/middleware/auth.go` 的白名单、Authorization header 解析、失败响应、trace-id 日志和上下文传播。新增一个版本校验依赖，例如 `common/middleware.TokenVersionValidator` 接口，由 `user-services` 注入基于 Redis+repository 的实现，避免 `common` 直接依赖服务侧 Ent 代码。

替代方案是把认证中间件完全迁移到 `user-services`。该方案会破坏现有共享认证能力边界，并重复响应/日志逻辑，因此不采用。

### 2. `token_version` 真值只在 PostgreSQL

`user-services/ent/schema/user.go` 增加 `field.Int64("token_version").Default(1).Comment("认证令牌版本")`，生成 Ent 代码和 Atlas SQL migration。登录签发 Access Token 时从 PostgreSQL 读取当前版本；认证中间件先读 Redis 缓存，未命中再查 PostgreSQL 并回填 Redis；退出全部设备、修改密码、强制下线先在 PostgreSQL 原子递增版本，再删除 Redis 版本缓存和会话。

替代方案是只在 Redis 保存版本。该方案在服务重启、缓存丢失、多实例切换时会丢失安全状态，与目标一致性原则冲突，因此不采用。

### 3. Refresh Token 会话存 Redis，使用会话 ID 做可撤销控制

登录成功后生成 `session_id`，签发 Access Token 和 Refresh Token，并写入 Redis：会话记录 key 保存用户 ID、会话 ID、当前 token_version、refresh token 标识或哈希、过期时间；用户会话索引保存该用户全部活跃 session_id。退出当前设备只删除当前 session，不递增 `token_version`；退出全部设备删除该用户所有 session 并递增版本。

建议 Redis key：`auth:user:{user_id}:token_version`、`auth:session:{session_id}`、`auth:user:{user_id}:sessions`。会话 TTL 与 Refresh Token TTL 对齐；版本缓存 TTL 独立配置，过期后可回源 PostgreSQL。

### 4. Claims 字段最小扩展

`common/jwt.Claims` 增加 `TokenVersion int64 json:"token_version"` 和 `SessionID string json:"session_id"`。`ParseToken` 继续校验 `user_id`，并对受保护 API 要求 `token_version > 0`、`session_id != ""`。签发能力可放在 `common/jwt.Service`，由 `user-services` auth service 传入当前版本、session、TTL。

Access Token 和 Refresh Token 的 subject/token type 常量应由 `common/jwt` 统一提供，例如 `SubjectAccess`、`SubjectRefresh` 和 `TokenTypeBearer`。`SignAccessToken` 与 `SignRefreshToken` MUST 在 `common/jwt.Service` 内部强制设置对应 subject，调用方不得通过可变入参决定 subject，避免 Refresh Token 被误签为 `access` 后在刷新流程被拒绝。服务侧只负责选择签发 Access 还是 Refresh，不重复维护 subject 字符串。

旧的测试手工 token 需要补齐新 claims。该行为是预期安全收紧，因为目标方案要求 Access Token 必须携带服务端版本。

### 5. API 与模块最小新增

新增 `user-services/internal/controller/auth_controller.go`、`internal/service/auth_service.go`、`internal/repository/auth_repository.go` 或在 `user_repository.go` 中补充认证查询/版本递增方法。为保持用户资料与认证职责清晰，建议新增 `AuthController`、`AuthService`、`SessionStore`，并让 `UserRepository` 增加按邮箱查询、按 ID 查询 token_version、递增 token_version 的方法。

建议路由：`POST /api/v1/auth/login`、`POST /api/v1/auth/refresh`、`POST /api/v1/auth/logout`、`POST /api/v1/auth/logout-all`。登录和刷新加入 auth 白名单；退出接口要求 Bearer Access Token，以便读取当前 `user_id` 和 `session_id`。

Refresh Token 的传输契约不应与受保护 API 的 `Authorization` header 完全等同：中间件处理的是 HTTP 认证方案，因此必须要求 `Authorization: Bearer <access_token>`；刷新接口接收 JSON body 字段 `refresh_token`，首选值应为裸 Refresh Token。为降低客户端集成成本，`AuthService.Refresh` 在解析前应对 `refresh_token` 执行规范化：去除首尾空白，若大小写匹配 `Bearer ` 前缀则剥离该前缀后再解析；剥离后为空仍按 token 无效处理。该兼容只适用于刷新接口请求体，不放宽受保护 API 的 Authorization header 格式校验。

### 6. 密码处理必须补齐哈希，但不扩大为完整账号体系重写

当前创建用户将明文 `password` 入库，这与登录校验前提冲突。最小调整是在用户创建和登录校验中引入密码哈希工具，例如 `common/password` 或 `user-services/internal/security`，创建时保存哈希，登录时校验哈希。历史明文用户需要迁移策略：本地开发可重建数据；生产数据应通过专门迁移或首次登录重哈希策略另行评估。

## Risks / Trade-offs

- [旧 Access Token 被拒绝] → 认证中间件新增 `token_version` 和 `session_id` 要求后，旧测试 token 和客户端旧 token 无法继续使用；通过更新测试签发 helper 和上线窗口内要求重新登录缓解。
- [Redis 不可用影响认证] → 现有用户服务启动已要求 `cache_redis` ping 成功；运行期 Redis 失败时可选择回源 PostgreSQL 校验版本，但 Refresh Token 会话控制依赖 Redis，刷新/登出应失败或返回未认证。
- [版本缓存短暂陈旧] → 用户级安全事件必须先更新 PostgreSQL，再删除 Redis 版本缓存；删除失败会导致旧 Access Token 可能在缓存 TTL 内继续通过。实现应在删除失败时记录高优先级错误，并可对版本缓存使用较短 TTL。
- [Refresh Token 轮转并发] → 同一 Refresh Token 并发刷新可能产生竞态；会话记录应保存 refresh token 标识或哈希，并使用 Redis 原子操作或事务删除旧会话、创建新会话。
- [Refresh Token 输入形式不一致] → 客户端可能把响应中的 token 加上 `Bearer ` 后放入刷新请求体；刷新服务应兼容可选前缀并在 DTO 示例/Swagger 中标明裸 token 为首选形式。
- [Token subject 误用] → 如果 Access/Refresh subject 由服务侧传入字符串，Refresh Token 可能被误签为 `access` 并在刷新时被拒绝；将 subject/token type 枚举上移到 `common/jwt`，并由对应签发方法内部强制设置。
- [明文密码历史数据] → 现有字段名为 `password`，但语义应调整为密码哈希；实施时需明确开发数据重建或生产迁移策略。

## Migration Plan

1. 修改 Ent schema 增加 `token_version` 默认值，运行 `go generate ./ent`，使用 `./scripts/migrate-diff.sh add_user_token_version` 生成 SQL migration，审查 SQL 并校验 `atlas.sum`。
2. 扩展配置结构和 YAML 示例，增加 Refresh Token TTL、版本缓存 TTL 和轮转开关；保持 `common/config.Load` 不做 required/range 校验。
3. 扩展 JWT claims 和签发/解析测试。
4. 新增认证 repository/service/session store/controller/router，并接入 Fx。
5. 修改认证中间件接入 token version validator 和 session id 上下文传播。
6. 更新 Swagger 注释、单元测试和集成测试；分别在 `common/` 与 `user-services/` 运行 `go test ./...`。
7. 部署前先 apply migration，再发布服务；发布后旧 token 因 claims 不完整需要客户端重新登录。

回滚时应先回滚服务版本，再评估是否保留 `token_version` 字段；保留新增字段通常不会影响旧服务读取用户资料。若 migration 已部署，不建议删除字段作为第一回滚动作。

## Open Questions

- 生产环境是否已有明文密码数据需要兼容迁移；如果有，需要单独设计密码哈希迁移策略。
- Refresh Token 是否必须存储原 token 哈希还是存储 JWT ID；建议存储不可逆哈希或 `jti`，避免 Redis 泄露时暴露可用 refresh token。
- Access Token TTL 和 Refresh Token TTL 的默认值是否沿用现有 `access_token_ttl: 2h`，还是收敛为更短 Access Token，例如 15-30 分钟。
