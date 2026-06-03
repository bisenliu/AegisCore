## Why

`user-services/internal/apperror` 目前只承载错误消息常量，包名容易与 `common/response` 中的应用错误模型混淆。将消息常量迁移到更明确的 `user-services/internal/errmsg` 可以让服务内错误文案与错误类型/响应映射职责分离。

## What Changes

- 删除 `user-services/internal/apperror` 包。
- 新增 `user-services/internal/errmsg/messages.go`，保留现有中文错误消息常量及常量名称。
- 更新 user-services 内引用 `apperror.Msg*` 的代码改为引用 `errmsg.Msg*`。
- 保持 HTTP 状态码、业务错误码、响应信封结构和对外错误消息文本不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `api-response-contract`: 调整 user-services 内错误消息常量的归属包；外部响应契约和错误语义不变。

## Impact

- 影响代码：`user-services/internal/apperror/messages.go`、新增 `user-services/internal/errmsg/messages.go`、所有引用 `user-services/internal/apperror` 的 controller/service/repository 或 auth 相关代码。
- API 兼容性：无对外 API 路径、响应信封、HTTP 状态码或业务错误码变化。
- 数据兼容性：不涉及 Ent schema、数据库迁移或持久化数据。
- 依赖影响：不引入新的第三方依赖。
