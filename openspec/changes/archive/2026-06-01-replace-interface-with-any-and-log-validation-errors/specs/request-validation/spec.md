## ADDED Requirements

### Requirement: Log validation failures with field details

系统必须在共享请求校验器的失败出口记录结构化错误日志。使用 `BindOrAbort` 处理请求校验失败时，日志必须使用 error 级别；当归一化校验错误包含字段级明细时，日志必须包含 `errors` 字段，且该字段必须复用对外响应中的字段级校验明细。

#### Scenario: Log validation failure at error level
- **Given** controller 使用共享校验器 `BindOrAbort` 绑定并校验请求
- **When** 请求绑定或校验失败
- **Then** 系统必须通过共享 logger 记录一条 error 级别日志
- **Then** 日志必须包含原始错误信息和请求 path

#### Scenario: Include field details in validation failure log
- **Given** controller 使用共享校验器 `BindOrAbort` 处理 validator tag 校验失败
- **When** 归一化校验错误包含字段级明细
- **Then** 系统记录的 error 日志必须包含 `errors` 字段
- **Then** `errors` 字段中的每条明细必须包含请求字段名、字段显示名、触发规则和中文错误消息

#### Scenario: Omit field details when unavailable
- **Given** controller 使用共享校验器 `BindOrAbort` 处理请求失败
- **When** 归一化错误不包含字段级明细
- **Then** 系统仍必须记录 error 级别日志
- **Then** 日志不得要求存在非空 `errors` 字段
