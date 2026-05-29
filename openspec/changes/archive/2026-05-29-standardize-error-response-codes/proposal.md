## Why

当前 API 错误响应使用字符串型业务码，例如 `BAD_REQUEST`、`NOT_FOUND`，难以稳定承载前端、移动端和外部调用方的分类处理需求，也无法区分通用请求错误、参数校验失败、认证、授权、冲突等常见错误类别。现在需要收敛 `api-response-contract` 的错误码、HTTP 状态码和错误文案维护方式，为后续业务接口扩展提供一致契约。

## What Changes

- **BREAKING** 将 `common/response.Code` 从 `string` 改为 `int`，响应体 `code` 从字符串改为数字业务码。
- 定义最小通用业务码集合：成功、通用请求错误、参数校验失败、未认证、无权限、业务冲突、资源不存在、服务内部错误。
- 明确业务码不等于 HTTP status code，失败响应同时保留标准 HTTP 状态码语义。
- 为 `common/response` 增加可变参数错误构造函数，固定文案和格式化文案共用同一个 helper，避免额外 `f` 后缀 API。
- 增加 `WrapInternal(err, publicMessage)`，支持记录内部错误原因并返回安全的对外文案。
- 将参数校验失败与普通请求错误区分：`common/validation` 产生校验类错误时返回 `CodeValidationFailed`，请求体格式、类型解析等请求错误仍返回 `CodeBadRequest`。
- 在 `user-services/internal/apperror` 集中维护用户服务业务错误文案常量和模板，服务侧复用 `common/response` 通用 helper，不再为每条业务错误封装专用函数。
- 更新现有测试断言，使成功、请求错误、校验失败、资源不存在和内部错误响应均验证数字业务码与消息。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `api-response-contract`: 将响应信封中的 `code` 契约从字符串错误码调整为数字业务码，并扩展标准错误类别、HTTP 状态映射、错误构造 helper 和参数校验错误映射规则。

## Impact

- 受影响代码：`common/response/`、`common/validation/`、`common/middleware/recovery.go`、`user-services/internal/controller/`、`user-services/internal/service/`、`user-services/internal/repository/`、新增 `user-services/internal/apperror/`。
- API 兼容性：这是响应体 `code` 字段类型和值的破坏性变更，调用方需要从字符串匹配迁移到数字业务码匹配。
- HTTP 语义：HTTP status 继续使用标准状态码，业务错误分类由响应体 `code` 表达。
- 测试影响：`common` 和 `user-services` 中现有响应断言需要更新；新增或调整测试覆盖格式化文案、固定 `%` 文案、校验失败码和内部错误包装。
- 非目标：本变更不新增认证、授权、用户写入、支付或其他业务 API，仅标准化已有响应契约和错误文案组织方式。
