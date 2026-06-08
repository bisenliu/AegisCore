## 1. Repository 集成测试

- [x] 1.1 审查 `user-services/internal/repository/postgres/user_repository_test.go` 的现有 fixture，确认可复用的 Ent SQLite test client、用户 seed helper 和领域错误断言方式
- [x] 1.2 补齐创建用户后按 `user_id`、`username` 查询的集成断言，覆盖资料字段、凭证字段和 token version 初始值
- [x] 1.3 补齐唯一性冲突和 not found 错误映射测试，确保 Repository 返回领域级错误而不是泄漏 Ent 底层错误
- [x] 1.4 补齐 `ListUsers` 边界测试，覆盖空结果页、limit/offset、过滤条件、稳定排序和软删除记录排除
- [x] 1.5 补齐 token version 读取与递增测试，覆盖成功递增、重复读取一致性和不存在用户错误
- [x] 1.6 判断是否存在 PostgreSQL 特有语义需要容器测试；如需要，新增明确跳过策略的 `testcontainers-go` PostgreSQL 测试，否则记录继续使用 SQLite 的原因

## 2. HTTP 端到端测试

- [x] 2.1 审查 `user-services/internal/bootstrap/http_test.go` 和路由注册测试 helper，整理可构建真实 Gin engine、真实中间件链和可控业务依赖的测试装配
- [x] 2.2 新增受保护路由 401 集成测试，验证缺失或无效 Bearer token 在进入 controller 前被拒绝并返回统一失败信封
- [x] 2.3 新增公开路由绕过认证测试，覆盖健康检查、Swagger、登录、刷新和公开改密入口不会因缺少普通 Access Token 返回认证中间件 401
- [x] 2.4 新增 trace-id 与 request logging 集成测试，使用 Zap observer 或内存 sink 断言 `trace-id`、method、path、status、latency、client_ip 和认证 `user_id` 字段
- [x] 2.5 新增 404 集成测试，通过真实路由链触发用户 not found 并断言 HTTP 404、失败信封和应用错误码语义
- [x] 2.6 新增 403 集成测试，通过测试 handler 或可注入授权失败点模拟授权拒绝，断言 HTTP 403 和统一失败信封，不新增生产授权能力
- [x] 2.7 新增 500 业务错误集成测试，通过业务依赖返回系统错误并断言 HTTP 500 和统一失败信封
- [x] 2.8 新增 panic recovery 集成测试，通过真实 recovery 中间件断言 HTTP 500 失败信封以及包含 `trace-id`、panic 内容和 stack 的日志

## 3. Redis Token Version 集成测试

- [x] 3.1 审查 `user-services/internal/repository/redis/auth_session_repository_test.go` 和 auth component 测试，复用 `miniredis` 与 token version key/TTL 断言 helper
- [x] 3.2 新增 cache miss 回源测试，组合 token version validator、用户 Repository 和 Redis session repository，断言回源读取、cache backfill 和 TTL
- [x] 3.3 新增 cache hit 测试，断言 validator 使用 Redis 缓存值并证明用户 Repository 未被额外读取
- [x] 3.4 新增旧 Access Token 经 HTTP 认证中间件拒绝的集成测试，覆盖 token version 递增后旧 token 请求受保护 API 返回 401 且业务 handler 未执行
- [x] 3.5 新增 token version cache 失效或刷新测试，覆盖登出全部设备、修改密码或等价会话控制流程后的后续认证行为

## 4. 验证与整理

- [x] 4.1 对新增或修改的 Go 测试文件运行 `gofmt -w`
- [x] 4.2 在 `common/` 运行 `go test ./...`，确认共享中间件和日志相关测试通过
- [x] 4.3 在 `user-services/` 运行 `go test ./...`，确认 Repository、HTTP 和 Redis 集成测试通过
- [x] 4.4 如新增 Go module 依赖，检查 `go.mod` 和 `go.sum` 仅包含必要测试依赖且不引入生产 runtime 行为变化
- [x] 4.5 复查新增测试未修改 HTTP API、错误码、配置键、Ent schema、Atlas migration 或生成代码
