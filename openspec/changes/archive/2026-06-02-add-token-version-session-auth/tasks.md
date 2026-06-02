## 1. 数据模型与配置

- [x] 1.1 在 `user-services/ent/schema/user.go` 增加 `token_version` 字段，默认值为 `1`，保留字段注释。
- [x] 1.2 在 `user-services` 模块运行 `go generate ./ent`，确认未手写修改 `user-services/ent/` 生成代码。
- [x] 1.3 运行 `./scripts/migrate-diff.sh add_user_token_version` 生成用户服务 Atlas SQL migration，审查 SQL 并同步 `atlas.sum`。
- [x] 1.4 运行 `./scripts/migrate-validate.sh` 验证迁移目录有效。
- [x] 1.5 扩展 `common/config.AuthConfig`/`JWTConfig` 和 `user-services/configs/config.yaml`，加入 Refresh Token TTL、token version 缓存 TTL、Refresh Token 轮转配置。
- [x] 1.6 更新配置加载测试，覆盖 YAML 加载和 `AEGISCORE_` 环境变量覆盖，确认 `common/config.Load` 不做 required/range 校验。

## 2. JWT Claims 与上下文

- [x] 2.1 扩展 `common/jwt.Claims`，加入 `token_version` 和 `session_id`，并为 Access Token 解析增加必填校验。
- [x] 2.2 为 `common/jwt.Service` 增加 Access Token 和 Refresh Token 签发方法，使用配置的 secret、issuer、audience 和 TTL。
- [x] 2.3 在 `common/contextutil/auth.go` 增加 `session_id` context key、Gin key 和读取/写入 helper。
- [x] 2.4 更新 `common/jwt` 与 `common/contextutil` 单元测试，覆盖缺失 `token_version`、缺失 `session_id`、过期 token、issuer/audience 校验。

## 3. 认证中间件版本校验

- [x] 3.1 在 `common/middleware` 定义 token version 校验接口，避免 `common` 直接依赖服务侧 repository 或 Ent。
- [x] 3.2 修改 `common/middleware.Auth`，在 JWT 校验通过后调用版本校验接口，并在版本不一致或回源失败时返回 401。
- [x] 3.3 修改认证成功上下文传播逻辑，同时写入 `user_id` 和 `session_id`。
- [x] 3.4 更新 `common/middleware/auth_test.go` 和 `user-services/internal/bootstrap/http_test.go`，覆盖白名单、无 header、无效 token、缺少版本、版本不一致、版本一致放行。

## 4. Repository 与 Session Store

- [x] 4.1 扩展用户 repository，支持按邮箱查询登录用户、按用户 ID 查询 `token_version`、原子递增 `token_version`。
- [x] 4.2 引入密码哈希与校验工具，在创建用户时保存密码哈希，在登录时校验哈希，避免继续写入明文密码。
- [x] 4.3 新增 Redis session store，管理 `auth:user:{user_id}:token_version`、`auth:session:{session_id}`、`auth:user:{user_id}:sessions`。
- [x] 4.4 在 session store 中实现 token version 缓存回填、创建会话、读取会话、删除当前会话、删除用户全部会话。
- [x] 4.5 为 repository 与 session store 增加单元测试，覆盖缓存命中/未命中、Redis 会话缺失、版本递增和会话清理。

## 5. Auth Service 与 API

- [x] 5.1 新增登录 DTO、刷新 DTO、token 响应 DTO、退出响应 DTO，确保响应不返回密码或密码哈希。
- [x] 5.2 新增 `AuthService`，实现登录流程：PostgreSQL 校验用户和密码、读取真实 `token_version`、创建会话、签发 Access/Refresh Token、写 Redis。
- [x] 5.3 实现 Refresh Token 刷新流程：验签、检查 Redis 会话、校验当前 `token_version`、签发新 Access Token，并按配置执行 Refresh Token 轮转。
- [x] 5.4 实现退出当前设备：读取认证上下文中的 `user_id` 和 `session_id`，删除当前 Redis 会话，不修改 PostgreSQL `token_version`。
- [x] 5.5 实现退出全部设备：先递增 PostgreSQL `token_version`，再删除 Redis token version 缓存、全部会话记录和会话索引。
- [x] 5.6 新增 `AuthController`，所有认证 API 使用 `common/response.Envelope` 输出成功和失败响应。
- [x] 5.7 在 `user-services/internal/router/router.go` 注册 `POST /api/v1/auth/login`、`POST /api/v1/auth/refresh`、`POST /api/v1/auth/logout`、`POST /api/v1/auth/logout-all`。
- [x] 5.8 更新 auth 白名单，使登录和刷新可匿名访问；退出当前设备和退出全部设备继续要求 Bearer Access Token。
- [x] 5.9 在 `user-services/internal/bootstrap/bootstrap.go` 接入 Auth repository/service/session store/controller 和 token version validator 的 Fx wiring。

## 6. 文档、Swagger 与测试验证

- [x] 6.1 更新 Swagger 注释并重新生成用户服务 Swagger 文档，覆盖登录、刷新、退出当前设备、退出全部设备。
- [x] 6.2 更新现有用户创建测试，断言新用户 `token_version=1` 且响应不包含 `token_version` 或密码字段。
- [x] 6.3 增加 auth controller/service 测试，覆盖登录成功、凭据错误、刷新成功、会话撤销、版本变化拒绝刷新、退出当前设备、退出全部设备。
- [x] 6.4 增加认证中间件集成测试，覆盖 Redis 缓存命中、缓存未命中回源 PostgreSQL、版本不一致立即拒绝。
- [x] 6.5 运行 `gofmt -w` 格式化所有修改的 Go 文件。
- [x] 6.6 分别在 `common/` 和 `user-services/` 运行 `go test ./...`，修复失败测试。
- [x] 6.7 复查 `docs/opsx/CAPABILITY_MAP.md` 是否需要新增 `user-session-control` capability 映射，并在实现完成后准备归档更新主规格。

## 7. Refresh Token 输入规范化

- [x] 7.1 在 `AuthService.Refresh` 解析 Refresh Token 前增加请求体 token 规范化逻辑：trim 空白，并兼容剥离可选 `Bearer ` 前缀。
- [x] 7.2 更新 `dto.RefreshTokenRequest` 示例或说明，使 `refresh_token` 首选裸 token，避免误导客户端认为请求体必须携带 Bearer scheme。
- [x] 7.3 增加 Refresh Token 刷新测试，覆盖裸 token 成功、`Bearer <token>` 成功、仅 `Bearer ` 或空 token 返回 token invalid。
- [x] 7.4 重新生成 Swagger 文档并运行 `go test ./...` 验证 common 与 user-services 模块。

## 8. JWT Subject 枚举集中化

- [x] 8.1 在 `common/jwt` 中新增公共枚举常量：Access Token subject、Refresh Token subject、Bearer token type。
- [x] 8.2 移除 `SignInput.Subject` 或停止使用调用方传入的 subject，让 `SignAccessToken` 和 `SignRefreshToken` 内部强制设置正确 subject。
- [x] 8.3 更新 `user-services/internal/service/auth_service.go`，移除服务侧重复 subject/token type 字符串常量，改用 `common/jwt` 公共枚举。
- [x] 8.4 增加/更新测试，覆盖 Refresh Token subject 必为公共 refresh 枚举、Access Token 被提交到刷新接口时必须被拒绝。
- [x] 8.5 运行 `gofmt`、`go test ./...`、必要的 Swagger 生成和 `openspec validate --changes add-token-version-session-auth`。
