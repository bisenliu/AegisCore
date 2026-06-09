## Why

用户状态枚举当前在 `user/domain` 和 `user/api` 两处重复定义，常量、校验枚举值和 JSON/query 解析逻辑存在漂移风险。将 `domain.UserStatus` 作为唯一事实标准可以避免新增状态时漏改 API DTO，并保持用户状态业务规则集中在领域层。

## What Changes

- 删除 `user-services/internal/features/user/api/request.go` 中重复的 `UserStatus` 类型、常量和枚举/反序列化方法。
- 让用户 request DTO 的 `status` 字段直接使用 `userdomain.UserStatus` 指针类型。
- `CreateUserRequest.SetDefaults` 使用 `userdomain.UserStatusNormal` 设置默认状态。
- 简化 HTTP controller 中 DTO status 到 command status 的映射，避免 `userapi.UserStatus` 到 `userdomain.UserStatus` 的强制转换。
- 保持现有 HTTP 请求字段、JSON/query tag、`validate:"omitempty,enum"`、Swagger example、响应字段和错误语义不变。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `user-domain-boundary`: 用户状态枚举、状态合法性校验和状态解析逻辑必须由用户领域层统一拥有，API DTO 复用领域枚举而不是重复定义状态类型。

## Impact

- 影响代码：`user-services/internal/features/user/api/request.go`、`user-services/internal/features/user/transport/http/controller.go`、`user-services/internal/features/user/app/mapper.go` 及相关测试中对 `userapi.UserStatus*` 的引用。
- API 兼容性：`POST /api/v1/users` 的 `status` JSON 字段、`GET /api/v1/users?status=` query 字段、校验失败响应和成功响应字段保持不变。
- 分层影响：`api` 包会依赖同 capability 内的 `domain` 包以复用稳定枚举；`domain` 不依赖 `api`，不会形成循环依赖。
- 数据与配置：不涉及 Ent schema、Atlas migration、Redis key、运行时配置或生成代码。
