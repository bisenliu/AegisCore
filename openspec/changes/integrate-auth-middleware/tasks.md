## 1. 配置与依赖

- [x] 1.1 在 `common/go.mod` 增加 `github.com/golang-jwt/jwt/v5` 依赖，并通过 `go mod tidy` 更新 `common/go.sum`。
- [x] 1.2 在 `common/config` 新增 `AuthConfig`、`JWTConfig` 等结构，并挂载到根 `config.Config`。
- [x] 1.3 更新 `common/config` 配置加载测试，覆盖 auth YAML 反序列化、`AEGISCORE_` 环境变量覆盖和缺失 auth 配置不报错。
- [x] 1.4 更新 `user-services/configs/config.yaml`，新增 `auth` 配置示例、JWT 配置注释和默认公开白名单路径。

## 2. 共享认证基础能力

- [x] 2.1 在 `common` 新增认证相关 context key 与 helper，支持在 Gin context 和 Go `context.Context` 中读写当前认证用户 ID。
- [x] 2.2 在 `common` 新增 JWT service，使用 `github.com/golang-jwt/jwt/v5` 解析 token、校验签名方法、过期时间、可选 issuer/audience 和非空 `user_id` claim。
- [x] 2.3 为 JWT service 增加单元测试，覆盖有效 token、过期 token、签名错误、issuer/audience 不匹配和缺少 `user_id`。
- [x] 2.4 在 `common/middleware` 新增 Auth middleware，迁移参考实现的白名单、Bearer header 校验、token 解析和 context 写入流程，并避免在日志中记录 token 原文。
- [x] 2.5 为 Auth middleware 增加单元测试，覆盖白名单放行、缺失 header、格式错误、空 token、无效 token、成功认证和 handler abort 行为。

## 3. 用户服务运行时集成

- [x] 3.1 调整 `user-services/internal/bootstrap` 的 Fx wiring，提供 JWT service 或 Auth middleware 所需依赖，保持 `common` 共享能力不在服务模块重复实现。
- [x] 3.2 调整 `user-services/internal/router`，对 `/api/v1` 用户业务路由启用认证，并确保 `/healthz`、Swagger、`/docs`、`/api-docs` 保持公开可访问。
- [x] 3.3 更新或新增路由/运行时测试，验证公开路径无 token 可访问、用户查询和创建接口无 token 返回 HTTP 401，且认证失败不进入 controller。
- [x] 3.4 如 Swagger 注解描述用户 API 安全要求，则更新注解并重新生成 Swagger 文档；若无需生成，记录无需变更的原因。

## 4. API 行为验证

- [x] 4.1 验证 `GET /api/v1/users/:id` 携带有效 Bearer token 时保持原查询成功、参数错误、not found 和 internal error 行为。
- [x] 4.2 验证 `POST /api/v1/users` 携带有效 Bearer token 时保持原创建成功、请求校验失败、冲突和持久化错误行为。
- [x] 4.3 验证受保护接口在缺失 header、Bearer 格式错误、空 token 和无效 token 时均返回统一 HTTP 401 响应信封与 `CodeUnauthenticated`。
- [x] 4.4 验证认证成功后 `user_id` 同时写入 Gin context 和 request context，且日志继续包含同一个 `trace-id`。

## 5. 格式化与回归测试

- [x] 5.1 对修改的 Go 文件运行 `gofmt`。
- [x] 5.2 在 `common/` 运行 `go test ./...` 并修复失败。
- [x] 5.3 在 `user-services/` 运行 `go test ./...` 并修复失败。
- [x] 5.4 确认本变更未修改 Ent schema、未手写 `user-services/ent/` 生成代码，且无需 Atlas migration。
