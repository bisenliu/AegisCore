## ADDED Requirements

### Requirement: 应用错误码段治理

系统 MUST 将 `common/contract/errors.Code` 作为稳定公开应用码治理，并 MUST 在共享契约中维护明确的错误码段分配、预留范围和扩展准入规则。`Code` MUST NOT 作为 HTTP status 使用，HTTP status MUST 继续只根据低基数 `Kind` 推导。

#### Scenario: 使用既定错误码段

- **WHEN** 系统定义或审查 `common/contract/errors.Code`
- **THEN** `0` MUST 只用于成功响应 `CodeOK`
- **AND** `10xxx` MUST 用于请求解析、绑定和字段校验错误
- **AND** `20xxx` MUST 用于认证、凭证、token、session 和账号登录态错误
- **AND** `30xxx` MUST 用于授权、访问控制和策略拒绝错误
- **AND** `40xxx` MUST 用于业务冲突、资源状态不允许和幂等冲突错误
- **AND** `50xxx` MUST 用于资源不存在或不可见错误
- **AND** `60xxx` MUST 预留给限流、配额或用量约束，启用前必须先定义对应 `Kind`、HTTP 映射和测试
- **AND** `70xxx` 至 `89xxx` MUST 保持预留，未经规格变更不得使用
- **AND** `90xxx` MUST 用于内部错误、依赖不可用和服务端临时故障

#### Scenario: 新增错误码准入

- **WHEN** 系统新增应用错误码
- **THEN** 新错误码 MUST 优先复用现有低基数 `Kind`，并使用稳定 `Reason` 表达可细分原因
- **AND** 系统 MUST NOT 按 feature、目录、临时实现任务或调用方便利随意开辟错误码段
- **AND** 新错误码 MUST 位于其语义对应的既定段位内，不得复用其他稳定公开错误码数值
- **AND** 内部错误对外 MUST 使用非敏感公开消息，`Cause` 不得进入响应 envelope

#### Scenario: 新增 Kind 同步 HTTP 映射

- **WHEN** 现有 `Kind` 无法表达新的低基数 HTTP 映射语义，系统新增 `Kind`
- **THEN** 系统 MUST 同步更新 `common/http/response.statusCode` 的 HTTP status 推导
- **AND** 系统 MUST 添加或更新响应测试，覆盖新增 `Kind` 到 HTTP status 和响应 code 的映射
- **AND** 未定义 HTTP 映射的 `Kind` MUST NOT 作为公开业务错误进入 feature 或 HTTP transport
