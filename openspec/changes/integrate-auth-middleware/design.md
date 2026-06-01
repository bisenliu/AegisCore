## Context

AegisCore 当前在 `common` 模块提供配置、日志、响应信封和 HTTP 中间件，在 `user-services` 中通过 Fx 组装 Gin engine、路由和运行时依赖。参考实现位于 `/Users/liubisen/Desktop/sander/Project/my/go-micro-scaffold/common/middleware/auth.go`，其核心流程是白名单跳过、读取 `Authorization`、校验 `Bearer ` 前缀、解析 JWT、写入用户 ID 到 context，并在失败时返回未认证响应。

本次迁移不能直接复制参考项目的包路径和旧版 JWT API；AegisCore 需要使用 `github.com/golang-jwt/jwt/v5`，并保持 `common/response.Envelope`、Zap trace-id 日志和 `common`/`user-services` 模块边界一致。

## Goals / Non-Goals

**Goals:**

- 在 `common` 模块提供可复用的 JWT Bearer 认证中间件和 token 解析能力。
- 在共享配置中新增 `auth` 配置段，并通过既有 YAML 与 `AEGISCORE_` 环境变量覆盖机制加载。
- 在用户服务运行时对 `/api/v1` 业务路由启用认证，同时允许 `/healthz`、Swagger 文档等公开路径跳过认证。
- 对缺失 header、Bearer 格式错误、空 token 和 token 解析失败统一返回 HTTP 401 与 `CodeUnauthenticated` 响应信封。
- 将认证后的 `user_id` 写入 Gin context 与 Go `context.Context`，供后续 controller/service 使用。

**Non-Goals:**

- 不实现登录、注册登录态、刷新 token、注销、RBAC/权限模型或会话存储。
- 不修改 Ent schema，不生成数据库 migration。
- 不改变 controller/service/repository 分层职责。
- 不引入参考项目中的旧版 JWT v4 依赖或其包路径结构。

## Decisions

1. 认证基础能力放在 `common` 模块。

   理由：认证中间件、JWT 解析和 context key 属于跨服务共享能力，放在 `common/middleware`、`common/jwt` 和 `common/contextutil` 可避免未来服务重复实现。替代方案是在 `user-services` 中本地实现，但会破坏共享基础能力优先放在 `common` 的边界。

2. JWT 使用 `github.com/golang-jwt/jwt/v5`。

   理由：用户明确要求使用 v5，且 v5 API 对 claims 解析、签名方法校验和标准 claims 支持更适合新实现。替代方案是继续使用 go-micro-scaffold 的 v4 API，但会引入过时依赖并增加后续升级成本。

3. 中间件只负责认证，不做业务授权。

   理由：当前需求是验证调用方身份并传播 `user_id`，业务资源所有权、角色权限和访问策略尚无稳定模型。替代方案是在中间件中绑定权限判断，但会在没有业务规格的情况下过度设计。

4. 白名单保留在 auth 配置中，并在用户服务路由注册时应用。

   理由：公开路径可能随服务变化，配置化可避免硬编码到中间件实现；默认配置应覆盖 `/healthz`、`/swagger`、`/docs` 和 `/api-docs`。替代方案是在路由分组时只对 `/api/v1` group 加认证，但仍需要确保 Swagger 和健康检查不受影响，配置白名单更接近参考实现且更易扩展。

5. token 失败日志不记录 token 原文。

   理由：参考实现会记录 token 前缀，仍有泄露敏感凭证风险；迁移时应只记录失败类型、trace-id 和必要上下文。替代方案是截断 token 后记录，但在认证失败路径没有必要。

## Risks / Trade-offs

- 受保护 API 行为变化导致未携带 token 的现有调用失败 -> 在配置示例、Swagger 注解和测试中明确需要 `Authorization: Bearer <token>`，并保留健康检查/文档白名单。
- JWT 配置缺失导致服务启动后所有受保护请求失败 -> 为本地配置提供明确示例，并在 token 服务初始化或解析时返回可诊断错误。
- 白名单使用前缀匹配可能误放行相似路径 -> 实现时优先使用明确前缀列表并在测试中覆盖 `/api/v1` 不被公开前缀误匹配。
- 多服务复用时 claims 字段约定不一致 -> 定义稳定的 `user_id` claim 与 context key，并将 issuer/audience 校验作为配置能力。
- 引入新依赖影响两个 Go module 的依赖图 -> 只在需要 JWT 实现的 `common` module 增加 v5 依赖，用户服务通过 replace 引用 `common`。

## Migration Plan

1. 在 `common/config` 新增 `AuthConfig` 并挂载到根 `Config`，更新用户服务 `configs/config.yaml` 示例。
2. 在 `common` 新增 context key、JWT service 和 auth middleware，使用 `github.com/golang-jwt/jwt/v5` 实现解析与标准 claims 校验。
3. 调整 `user-services/internal/bootstrap` 和 `router` wiring，将认证中间件接入 `/api/v1` 业务路由或全局中间件白名单策略。
4. 更新 Swagger 注解和测试，覆盖公开路径、成功认证、缺失 header、格式错误、空 token 和无效 token。
5. 分别在 `common/` 与 `user-services/` 运行 `go test ./...`。

回滚策略：移除用户服务对认证中间件的注册即可恢复公开访问；保留 `common` 中的认证代码不会影响未接入的路由。若依赖引入出现问题，可回滚 `common/go.mod` 与相关新增代码。

## Open Questions

- `auth.jwt.secret` 的生产注入方式由部署环境或 Secret 管理系统提供，本变更只定义配置入口。
- 是否需要强制校验 issuer/audience 可按配置开关处理；未配置时只校验签名与标准过期时间。
