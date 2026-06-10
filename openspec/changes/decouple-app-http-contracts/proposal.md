## Why

当前用户与认证 use case 直接返回 HTTP response DTO、分页响应契约和 `common/contract/response` 应用错误，使 `app` 层被 HTTP 表达绑定。短期可用，但后续 CLI、事件、RPC 或批处理复用这些 use case 时会被迫携带 HTTP DTO 和响应信封语义，因此需要一次性收紧应用层边界。

## What Changes

- 调整用户资料与认证会话 app service 返回值：由 `api/*Response`、`response.PaginatedData[...]` 改为领域模型或应用结果对象。
- 将 HTTP response DTO 映射、分页响应包装和 HTTP 应用错误映射移动到对应 feature 的 `transport/http` 层。
- 保持 controller 对外返回的 `common/contract/response.Envelope`、JSON 字段、HTTP status code 和业务错误码兼容。
- 在 app 层保留 command/query、领域模型、消费侧 ports 和 common 安全原语，避免导入 feature `api`、Gin、HTTP validation 或 HTTP 响应契约。
- 不修改数据库 schema、Redis key、JWT claims、路由路径、Swagger 对外字段或运行时配置。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-domain-boundary`: 明确 app service、commands、queries、ports、mapper 和 use case 结果不得返回 HTTP DTO、HTTP 分页响应契约或 HTTP 应用错误；transport 层负责 HTTP DTO、分页信封和错误响应映射。
- `api-response-contract`: 明确分页响应契约和 HTTP 应用错误构造属于 HTTP transport 输出边界，app 层可以返回领域错误或应用错误分类，controller 必须映射为既有统一响应信封。

## Impact

- 主要代码影响：
  - `user-services/internal/features/user/app/ports.go`
  - `user-services/internal/features/user/app/service.go`
  - `user-services/internal/features/user/app/mapper.go`
  - `user-services/internal/features/user/transport/http/controller.go`
  - `user-services/internal/features/user/transport/http/*_test.go`
  - `user-services/internal/features/auth/app/ports.go`
  - `user-services/internal/features/auth/app/service.go`
  - `user-services/internal/features/auth/app/tokens.go`
  - `user-services/internal/features/auth/transport/http/controller.go`
  - `user-services/internal/features/auth/transport/http/*_test.go`
- 相关 capability：`user-profile-query`、`user-profile-create`、`user-list-query`、`user-session-control`、`user-domain-boundary`、`api-response-contract`。
- 外部兼容性：HTTP 路径、请求 DTO、响应 JSON 字段、分页结构、错误码和认证行为必须保持不变。
- 依赖影响：不新增第三方依赖，不修改 Ent 生成代码、Atlas migration、Redis/PostgreSQL 命名实例或配置 key。
