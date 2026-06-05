## MODIFIED Requirements

### Requirement: User service owns error message constants separately from application errors

用户服务 MUST 将服务内复用、可直接展示给客户端或最终用户的错误消息文本常量放在 `user-services/internal/messages` 包中，并 MUST NOT 使用容易暗示 Go error 实例或错误构造职责的包承载仅包含消息文本的常量。消息常量命名 MUST 避免与包名重复，MUST 使用无 `Msg` 前缀的领域语义名称。迁移 MUST 保持既有响应信封、HTTP 状态码和业务错误码不变；对外 `message` 文本 MAY 在不改变原始业务语义的前提下优化为更统一、专业、可读且适合直接展示给最终用户的表达。

#### Scenario: Reuse service error message text
- **Given** user-services controller、service 或相关业务代码需要复用用户错误消息文本
- **When** 代码引用消息常量
- **Then** 系统 MUST 从 `user-services/internal/messages` 引用无 `Msg` 前缀的常量
- **Then** 调用形态 MUST 类似 `messages.UserNotFound`，而不是 `errmsg.MsgUserNotFound` 或 `messages.MsgUserNotFound`

#### Scenario: Preserve external error response contract
- **Given** 请求触发用户服务中使用迁移后消息常量的失败路径
- **When** controller 或中间件返回统一失败响应
- **Then** 响应 MUST 继续使用 `common/contract/response.Envelope` 失败信封
- **Then** HTTP 状态码和业务错误码 MUST 与迁移前一致
- **Then** 对外 `message` 文本 MUST 保持原始业务语义，不得暴露内部数据库、密码、token 签名或底层依赖细节

#### Scenario: Optimize user-facing message wording
- **Given** 现有用户服务错误消息文本用于直接展示给客户端或最终用户
- **When** 实现优化消息常量值
- **Then** 空用户名消息 MUST 表达需要输入用户名
- **Then** 用户 ID 格式错误消息 MUST 表达格式不正确并提示检查
- **Then** 空密码消息 MUST 表达需要输入密码
- **Then** 凭证错误消息 MUST 表达用户名或密码不正确并提示检查
- **Then** 用户已存在、用户不存在和会话无效消息 MUST 使用统一、专业且不泄露内部细节的中文表达

#### Scenario: Remove misleading message package references
- **Given** 迁移完成后的 user-services 代码
- **When** 仓库中搜索 `user-services/internal/errmsg`、`errmsg.` 或 `MsgUserNotFound` 等旧限定名
- **Then** 系统 MUST 不再存在对旧包或旧 `Msg*` 常量名的引用
