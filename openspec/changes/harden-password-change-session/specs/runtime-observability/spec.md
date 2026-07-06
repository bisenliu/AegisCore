## ADDED Requirements

### Requirement: 强制改密安全指标与告警

系统 MUST 为强制改密一次性会话和安全撤销链路提供 Prometheus 指标与告警。指标标签 MUST 保持低基数，MUST NOT 包含用户 ID、session ID、jti、token、Redis key、SQL、IP、用户名、邮箱、trace/span ID、原始错误或 stacktrace。

#### Scenario: 一次性会话消费失败指标
- **WHEN** password-change session 消费因不存在、过期、撤销、复用或 claims 不一致而失败
- **THEN** 系统 MUST 递增强制改密会话消费失败指标
- **AND** 指标标签 MUST 只使用固定枚举原因
- **AND** 指标 MUST NOT 暴露具体用户、session 或 token 标识

#### Scenario: 重复消费拒绝指标
- **WHEN** 同一个 password-change token 被重复使用并被拒绝
- **THEN** 系统 MUST 记录可聚合的重复消费拒绝指标
- **AND** SRE MUST 能基于该指标发现 token 重放或客户端重试异常

#### Scenario: 撤销投影失败指标
- **WHEN** 强制改密成功更新凭据后，本地 token version cache 失效、Redis token version 投影刷新或 refresh session 删除任一步骤失败
- **THEN** 系统 MUST 递增强制改密撤销投影失败指标
- **AND** 指标 MUST 能区分失败步骤的固定枚举类型
- **AND** 指标 MUST NOT 包含原始错误文本或高基数标识

#### Scenario: 补偿失败指标
- **WHEN** 系统尝试记录或执行强制改密撤销补偿但失败
- **THEN** 系统 MUST 递增强制改密撤销补偿失败指标
- **AND** 系统 MUST 记录包含固定错误分类的日志
- **AND** 日志 MUST NOT 包含 token、jti、session ID 或 Redis key 明文

#### Scenario: 告警覆盖安全撤销失败
- **WHEN** 强制改密撤销投影失败或补偿失败指标在观察窗口内大于 0
- **THEN** Prometheus alert rules MUST 产生可行动告警
- **AND** 告警说明 MUST 指向稳定 runbook 或排查说明
- **AND** 告警 MUST 提示优先检查 Redis、token version 投影、本地缓存失效和 refresh session 删除链路

#### Scenario: metrics load 验证强制改密指标
- **WHEN** 执行观测资产或 metrics load 验证脚本
- **THEN** 验证 MUST 覆盖强制改密会话消费失败、重复消费拒绝、撤销投影失败和补偿失败指标的 presence 或 PromQL 查询
- **AND** 指标缺失或 PromQL drift MUST 能被验证流程发现
