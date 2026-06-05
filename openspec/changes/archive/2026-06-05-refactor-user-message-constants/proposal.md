## Why

`user-services/internal/errmsg/messages.go` 当前用于集中管理面向前端或客户端展示的提示文案，但包名 `errmsg` 容易暗示其中保存的是 Go `error` 实例，常量名又重复携带 `Msg` 前缀，形成命名 stuttering。随着用户资料、认证与会话相关接口增多，展示文案需要更清晰的归属边界和更统一、专业、适合直接展示给最终用户的表达。

## What Changes

- 将用户服务内部展示文案包从 `errmsg` 重命名为更准确的 `messages`，用于表达“纯文案集中管理”职责。
- 去除展示文案常量名中的 `Msg` 前缀，使调用形态从 `errmsg.MsgUserNotFound` 变为 `messages.UserNotFound`。
- 在不改变原始业务语义、错误码或响应结构的前提下，统一优化现有中文提示文案的措辞、语气和可读性。
- 更新用户服务内所有引用，保持 Go 编译、测试和现有 API 响应信封兼容。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `api-response-contract`: 明确用户服务面向客户端的错误提示文案应使用语义准确的 messages 包集中管理，常量命名应避免与包名重复，并保持可直接展示给最终用户的专业一致措辞。

## Impact

- 影响代码位置：`user-services/internal/errmsg/messages.go` 及用户服务内引用该包和 `Msg*` 常量的 controller/service 等调用点。
- API 兼容性：不改变 HTTP 路由、状态码、响应信封结构、错误码和数据模型；仅优化响应中的中文 `message` 文案。
- 依赖影响：不新增第三方依赖，不修改 `common` 模块，不涉及 Ent schema 或 Atlas migration。
- 命名影响：由于该包位于 `user-services/internal`，变更不影响仓库外部 Go import 方；仓库内部引用需要同步迁移。
