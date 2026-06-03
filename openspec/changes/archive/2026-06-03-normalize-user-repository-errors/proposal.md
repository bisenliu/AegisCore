## Why

`user-services/internal/repository/postgres.UserRepository` 中仍有部分用户不存在路径直接构造 `common/response` 应用错误，导致 repository 层泄露 HTTP 响应契约职责。需要将这些路径统一为用户领域错误，让 service 层集中完成应用错误映射，保持分层边界一致。

## What Changes

- 将 PostgreSQL 用户 repository 中 token version 与凭据更新相关的 Ent not found 路径统一返回 `domain.ErrUserNotFound`。
- 保持 service 层负责把 `domain.ErrUserNotFound` 映射为 not found 应用错误。
- 保持现有 HTTP 状态码、业务错误码、响应信封和公开错误消息不变。
- 不新增 API、配置项、数据库字段或 migration。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 明确会话控制相关 repository 方法在用户不存在时返回领域错误，由 service 层映射为应用错误。

## Impact

- 主要影响代码：`user-services/internal/repository/postgres/user_repository.go`，以及必要的 service/repository 单元测试。
- API 兼容性：外部 HTTP 响应保持兼容，仍通过 `common/response.Envelope` 返回既有状态码、业务码和消息。
- 数据兼容性：不改变 Ent schema、数据库表结构或 Atlas migration。
- 依赖影响：不新增第三方依赖。
