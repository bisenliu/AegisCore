## Why

认证失败是受保护 API 的预期外部输入场景，例如缺少 `Authorization` header、Bearer 格式错误、无效 token 或过期 token。当前认证中间件把这些调用方失败记录为 Error，会污染正式环境错误日志并降低真实依赖异常的可观测性。

## What Changes

- 调整 `user-authentication` 中 JWT Bearer 中间件的日志等级分类。
- 将预期认证失败降级为 Warn 或 Info，包括缺少认证头、认证头格式错误、空 Bearer token、无效 token、过期 token、subject 不匹配、token claim 缺失或 token version mismatch。
- 保留真正依赖或配置异常的 Error，包括 JWT secret 缺失、token version validator 依赖失败等服务端异常。
- 不改变 HTTP 状态码、响应信封、错误码、路由、配置字段或 JWT claim 格式。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-authentication`: 认证中间件必须按预期认证失败与服务端异常区分日志等级，避免把调用方认证失败记录为 Error。

## Impact

- 影响代码：`common/http/middleware/auth.go` 及相关测试。
- 影响可观测性：认证失败日志等级会从 Error 降为 Warn 或 Info，减少错误日志噪音。
- API 兼容性：无 HTTP API、响应 code/message、JWT claim、配置或数据模型变更。
- 依赖影响：无新增外部依赖。
