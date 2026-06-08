## ADDED Requirements

### Requirement: Repository integration tests cover persistence boundaries
用户服务 MUST 具备 Repository 集成测试，用于验证用户数据访问层在真实 Ent client 上的持久化行为、错误映射和边界条件。测试 MUST 覆盖已存在的用户创建、按外部 `user_id` 查询、按 `username` 查询、用户列表、凭证更新、token version 读取与递增、唯一性冲突、not found 和软删除排除语义。默认测试路径 SHOULD 使用 Ent SQLite 测试库保持快速确定性；仅当行为依赖 PostgreSQL 方言、事务隔离或生产索引语义时，测试 MUST 使用真实 PostgreSQL 集成环境或明确说明跳过条件。

#### Scenario: Repository covers user create and lookup
- **Given** Repository 测试创建真实 Ent 测试 client
- **When** 测试通过用户 Repository 创建用户并按 `user_id` 或 `username` 查询
- **Then** 查询结果 MUST 与持久化的用户资料、凭证字段和 token version 保持一致
- **Then** Repository MUST NOT 依赖 HTTP controller 或 service stub 才能完成验证

#### Scenario: Repository covers conflict and not found mapping
- **Given** Repository 测试数据库中存在唯一 `username` 或唯一 `user_id` 的用户记录
- **When** 测试创建重复用户或查询不存在的用户
- **Then** Repository MUST 将唯一性冲突映射为领域级已存在错误
- **Then** Repository MUST 将不存在记录映射为领域级 not found 错误

#### Scenario: Repository covers list boundaries
- **Given** Repository 测试数据库中存在多条用户记录和软删除记录
- **When** 测试执行列表查询、分页、空结果页和过滤条件查询
- **Then** Repository MUST 返回稳定排序后的未删除用户
- **Then** Repository MUST 对 limit、offset 和过滤条件边界给出确定性结果

#### Scenario: Repository covers token version mutation
- **Given** Repository 测试数据库中存在一个用户且 token version 为当前值
- **When** 测试读取并递增该用户 token version
- **Then** 后续读取 MUST 返回递增后的 token version
- **Then** 不存在用户的 token version 操作 MUST 返回领域级 not found 错误

### Requirement: HTTP integration tests cover the complete service route chain
用户服务 MUST 具备 HTTP 集成测试，通过 `httptest`、真实 Gin engine、真实用户服务路由注册入口和共享中间件链验证请求响应流程。测试 MUST 覆盖 trace-id 注入与传播、panic recovery、request logging、CORS、认证中间件挂载边界、统一响应信封、以及 401、403、404、500 错误场景。测试可以使用内存 repository 或测试 repository 控制业务返回，但 MUST 保持真实路由分组和中间件顺序。

#### Scenario: Protected route rejects unauthenticated requests
- **Given** HTTP 集成测试通过真实路由注册入口构建 Gin engine
- **When** 调用方请求受保护用户 API 且未携带有效 Bearer token
- **Then** 请求 MUST 在进入用户 controller 前被认证中间件拒绝
- **Then** 响应 MUST 为 HTTP 401 和 `common/contract/response.Envelope` 失败格式

#### Scenario: Public routes bypass authentication
- **Given** HTTP 集成测试通过真实路由注册入口构建 Gin engine
- **When** 调用方请求健康检查、Swagger、登录、刷新或公开改密入口且未携带普通 Access Token
- **Then** 请求 MUST 到达对应公开 handler
- **Then** 系统 MUST NOT 因缺少普通 Access Token 返回认证中间件产生的 HTTP 401

#### Scenario: Trace id and request log are emitted through real route chain
- **Given** HTTP 请求包含合法 `X-Trace-ID` header 并命中用户服务真实路由链
- **When** 请求处理完成
- **Then** 响应 header MUST 包含同一个 `X-Trace-ID`
- **Then** 请求日志 MUST 包含 `trace-id`、method、path、status、latency、client_ip 和可用的认证 `user_id` 字段

#### Scenario: Route-level not found returns envelope
- **Given** HTTP 集成测试中的用户查询 service 或 repository 返回领域级 not found 错误
- **When** 调用方携带有效 token 请求 `GET /api/v1/users/:user_id`
- **Then** 响应 MUST 为 HTTP 404
- **Then** 响应 MUST 使用统一失败响应信封并保留应用错误码语义

#### Scenario: Route-level authorization failure returns envelope
- **Given** HTTP 集成测试通过测试 handler 或可注入授权失败点模拟受保护路由授权拒绝
- **When** 调用方携带认证通过但无权访问的请求进入该失败点
- **Then** 响应 MUST 为 HTTP 403
- **Then** 响应 MUST 使用统一失败响应信封

#### Scenario: Route-level internal error returns envelope
- **Given** HTTP 集成测试中的业务依赖返回未预期系统错误
- **When** 调用方请求对应用户服务 API
- **Then** 响应 MUST 为 HTTP 500
- **Then** 响应 MUST 使用统一失败响应信封

#### Scenario: Panic recovery returns envelope and logs trace id
- **Given** HTTP 集成测试中的 handler 发生 panic
- **When** 请求经过真实 recovery 中间件
- **Then** 响应 MUST 为 HTTP 500 和统一失败响应信封
- **Then** recovery 日志 MUST 包含 `trace-id`、panic 内容和 stack 字段

### Requirement: Redis token version integration tests cover cache and authentication interaction
用户服务 MUST 具备 Redis token version 集成测试，使用 `miniredis` 或等价内存 Redis 验证 token version 缓存 repository、认证 token version validator 和 HTTP 认证中间件之间的贯通行为。测试 MUST 覆盖 cache miss 回源、cache backfill、TTL、缓存失效、过期或旧 token 被拒绝、以及 token version 变更后旧 Access Token 无法继续访问受保护 API。

#### Scenario: Cache miss backfills token version from repository
- **Given** Redis token version cache 中不存在目标用户版本
- **Given** 用户 Repository 中存在该用户的当前 token version
- **When** 认证 token version validator 校验该用户的 Access Token
- **Then** validator MUST 从用户 Repository 回源读取 token version
- **Then** validator MUST 将读取到的 token version 写入 Redis cache 并设置 TTL

#### Scenario: Cache hit avoids repository lookup
- **Given** Redis token version cache 中存在目标用户当前 token version
- **When** 认证 token version validator 校验版本一致的 Access Token
- **Then** validator MUST 使用缓存值完成校验
- **Then** 测试 MUST 能证明用户 Repository 未被额外读取

#### Scenario: Stale access token is rejected through HTTP middleware
- **Given** 用户当前 token version 已递增且 Redis cache 反映新版本或可从 Repository 回源到新版本
- **Given** 调用方持有递增前签发的旧 Access Token
- **When** 调用方使用旧 Access Token 请求受保护 API
- **Then** 认证中间件 MUST 返回 HTTP 401
- **Then** 请求 MUST NOT 进入受保护业务 handler

#### Scenario: Token version cache invalidation is observable
- **Given** 用户 token version cache 中存在旧值
- **When** 登出全部设备、修改密码或等价会话控制流程导致 token version 变更并清理或刷新缓存
- **Then** 后续认证校验 MUST 使用更新后的 token version
- **Then** 旧 token MUST 被拒绝且响应使用统一失败信封
