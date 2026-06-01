## Why

当前请求参数校验失败时，API 只返回统一文案 `请求参数验证失败`，调用方无法稳定识别具体字段、显示名称、触发规则和字段级错误消息。需要在保持现有 HTTP 400 与业务码 `10001` 契约的基础上，补充结构化字段错误明细，提升前端表单错误展示和 API 调试体验。

## What Changes

- 参数校验失败响应继续使用 `common/response.Envelope`，并在 `success: false`、`code: 10001`、`message: 请求参数验证失败`、`data: null` 外增加 `errors` 数组。
- `errors` 数组中的每个元素包含 `field`、`label`、`rule`、`message`，分别表示请求字段名、字段显示名、触发的校验规则和中文字段级错误消息。
- 共享校验能力需要从 DTO tag 与 validator 错误中提取字段错误明细，优先使用 `label` tag 作为显示名。
- 非字段校验错误、请求体为空、JSON 类型不匹配等通用 bad request 场景不纳入本次字段级 `errors` 契约，继续使用现有失败响应。
- 不改变现有数字错误码、HTTP status code、业务成功响应或用户资料业务逻辑。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `api-response-contract`: 参数校验失败失败信封新增结构化 `errors` 字段，明确 code `10001` 时的响应体形态。
- `request-validation`: 共享请求校验能力需要生成字段级校验明细，包括字段名、显示名、规则和中文错误消息。

## Impact

- 影响 `common/response` 的失败信封结构或校验失败 helper，使其能够输出字段级错误明细。
- 影响 `common/validation` 的错误归一化和 `BindOrAbort` 响应映射，使 validator tag 校验失败时携带结构化 `errors`。
- 影响使用共享 validator 的业务 API，例如用户创建或用户查询请求参数校验失败响应。
- API 行为保持向后兼容：HTTP status 仍为 400，业务码仍为 `10001`，顶层 `message` 仍为 `请求参数验证失败`；新增 `errors` 字段供调用方增强展示。
