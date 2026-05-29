## Why

`common/validation` 已经注册中文校验翻译并能生成中文错误消息，但 `user-services` 的用户查询 controller 在参数绑定失败后丢弃了该错误，改为硬编码英文 `invalid user id`。这会让同一套请求校验能力在服务层表现不一致，也与面向调用方的中文错误消息预期不符。

## What Changes

- 将 `GET /api/v1/users/:id` 的参数绑定失败响应改为使用共享校验器返回的公开错误消息。
- 移除用户查询 controller 中针对用户 ID 校验失败的英文硬编码消息和调试输出。
- 为用户 ID 非数字、非正数等失败场景补充或调整测试，确保响应信封仍为 `BAD_REQUEST`，但 `message` 来源于中文校验错误。
- 不改变成功查询、用户不存在、内部错误、响应信封结构或 HTTP 状态码。

## Capabilities

### New Capabilities
- 无。

### Modified Capabilities
- `user-profile-query`: 用户 ID 参数无效时，失败响应消息从 controller 硬编码英文改为共享校验器提供的中文公开消息。

## Impact

- 影响代码：`user-services/internal/controller/user_controller.go`、用户 controller 相关测试，以及必要时的共享校验测试。
- API 影响：`GET /api/v1/users/:id` 在参数无效时仍返回 HTTP 400、`BAD_REQUEST` 和统一响应信封，但 `message` 将从 `invalid user id` 变为中文校验消息。
- 依赖影响：不新增外部依赖，不修改配置、数据库 schema 或 migration。
- 兼容性：这是错误消息文本的外部可观察变化；依赖英文消息文本的调用方需要适配，错误码和 HTTP 状态码保持兼容。
