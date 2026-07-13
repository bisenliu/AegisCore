## ADDED Requirements

### Requirement: 共享 runtime primitive 测试避免非必要进程级状态

`common/` 共享 runtime primitive 的测试 MUST 避免为了日志捕获或断言便利而非必要地修改进程级可变状态。对 logger 相关测试，系统 MUST 优先使用 context logger 或局部 logger 注入；只有测试目标本身是进程级默认 logger 行为时，才 MAY 调用 `logger.SetDefault`，并 MUST 保存和恢复原状态。

#### Scenario: 共享 HTTP binding 日志测试注入 request context logger

- **WHEN** `common/http/binding` 测试需要捕获绑定、校验或响应相关日志
- **THEN** 测试 MUST 通过 request context 注入局部 logger
- **AND** 测试 MUST NOT 通过 `logger.SetDefault` 修改进程级默认 logger

#### Scenario: 必要进程级状态测试具备恢复边界

- **WHEN** `common/` 测试必须替换 logger 默认值或其他进程级 runtime primitive 状态
- **THEN** 测试 MUST 在测试 helper 内保存原状态并通过 cleanup 恢复
- **AND** 该 helper MUST 限定在相关 package 测试内，不得新增业务无关生产 API

#### Scenario: 并行测试不依赖被替换的默认 logger

- **WHEN** 测试使用 `t.Parallel()` 或可能与其他 package 测试同进程执行
- **THEN** 测试 MUST NOT 依赖被 `logger.SetDefault` 替换的进程级 logger
- **AND** 测试 MUST 使用局部 logger、context logger 或显式参数表达日志依赖
