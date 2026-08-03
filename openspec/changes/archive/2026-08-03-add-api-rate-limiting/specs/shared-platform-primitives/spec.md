## ADDED Requirements

### Requirement: 共享限流错误与 HTTP 映射

系统 MUST 在 `common/contract` 中提供业务中立的限流应用错误语义，并在 `common/http/response` 中将该错误渲染为 `429 Too Many Requests`。限流错误 MUST 使用稳定低基数 `Kind`、`Reason`、`Code` 和公开 `Message`，不得暴露 limiter key、IP、User ID、token、内部分片或实现细节。

#### Scenario: 限流错误响应

- **WHEN** HTTP middleware 或 handler 返回限流应用错误
- **THEN** `common/http/response` MUST 渲染 `429 Too Many Requests`
- **AND** 响应 envelope MUST 为 `success=false`、稳定限流 code 和公开限流 message

#### Scenario: 未知错误不伪装为限流

- **WHEN** 系统返回 nil、未知错误或内部错误
- **THEN** `common/http/response` MUST 保持现有内部错误归一化语义
- **AND** 系统 MUST NOT 将非限流错误映射为 `429 Too Many Requests`

### Requirement: 业务中立本地限流 primitive

`common/http/middleware` 或 `common/runtime` MUST 提供业务中立的本地限流 primitive。该 primitive MUST 支持基于调用方提供 key 的 `Allow` 判定、`golang.org/x/time/rate` token bucket、分片存储、后台清理和显式关闭。

#### Scenario: 调用方提供限流 key

- **WHEN** 调用方使用共享限流 middleware
- **THEN** 调用方 MUST 提供 key resolver 或等价 key 来源
- **AND** 共享 primitive MUST NOT 内置 user-service 路由、业务 DTO、权限目录、Casbin subject 或服务私有限流阈值

#### Scenario: 本地 limiter 并发访问

- **WHEN** 多个请求并发访问不同 IP 或 User ID 对应的限流 key
- **THEN** 本地限流 store MUST 使用分片或等价机制降低单一全局锁竞争
- **AND** 每个 key MUST 拥有独立 token bucket 状态
