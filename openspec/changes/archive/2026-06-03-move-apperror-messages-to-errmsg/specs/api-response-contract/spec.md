## ADDED Requirements

### Requirement: User service owns error message constants separately from application errors

用户服务 MUST 将服务内复用的错误消息文本常量放在 `user-services/internal/errmsg` 包中，并 MUST NOT 使用 `user-services/internal/apperror` 承载仅包含消息文本的常量。迁移 MUST 保持既有响应信封、HTTP 状态码、业务错误码和对外错误消息文本不变。

#### Scenario: Reuse service error message text
- **Given** user-services controller、service 或相关业务代码需要复用用户错误消息文本
- **When** 代码引用消息常量
- **Then** 系统 MUST 从 `user-services/internal/errmsg` 引用 `Msg*` 常量
- **Then** 常量值 MUST 与迁移前对应消息文本一致

#### Scenario: Preserve external error response contract
- **Given** 请求触发用户服务中使用迁移后消息常量的失败路径
- **When** controller 或中间件返回统一失败响应
- **Then** 响应 MUST 继续使用 `common/response.Envelope` 失败信封
- **Then** HTTP 状态码和业务错误码 MUST 与迁移前一致
- **Then** 对外 `message` 文本 MUST 与迁移前一致

#### Scenario: Remove misleading application error package
- **Given** 迁移完成后的 user-services 代码
- **When** 仓库中搜索 `user-services/internal/apperror` 或 `apperror.Msg`
- **Then** 系统 MUST 不再存在对旧包或旧限定名的引用
