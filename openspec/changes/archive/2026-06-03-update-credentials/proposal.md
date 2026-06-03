## Why

当前用户认证流程在修改密码时依赖 `UpdatePasswordHashAndStatus` 仓储方法，该名称把实现约束在“密码哈希 + 状态”两个字段上，难以表达更通用的凭证更新语义。将其调整为 `UpdateCredentials` 可让 `user-session-control` 能力以凭证更新为边界，同时保持密码修改后递增 `token_version` 并失效旧凭据的安全语义。

## What Changes

- 将用户仓储接口中的 `UpdatePasswordHashAndStatus` 改为 `UpdateCredentials`，使用输入结构表达 `password_hash`、目标用户状态等凭证更新参数。
- 修改认证服务的改密流程调用新的凭证更新方法，保持修改密码成功后更新 `password_hash`、将用户状态恢复为正常、递增 `token_version` 的行为不变。
- 更新相关单元测试 stub 和断言，覆盖新方法名称与输入结构。
- 不新增 HTTP API、不修改请求或响应 DTO、不改变错误码、token subject、Redis session key 或数据库字段。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 明确修改密码场景通过通用凭证更新仓储契约更新 `password_hash`、状态和 `token_version`，而不是依赖仅描述密码哈希和状态的旧方法名称。

## Impact

- 影响代码：`user-services/internal/repository/user_repository.go`、`user-services/internal/service/auth_service.go`、认证和用户服务相关测试 stub。
- API 兼容性：外部 HTTP API、响应信封、业务错误码和 token 行为不变。
- 数据模型兼容性：不新增或删除字段，不需要 Ent schema 或 Atlas migration。
- 配置和依赖：不新增配置项、Redis key 或外部依赖。
