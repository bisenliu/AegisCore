## ADDED Requirements

### Requirement: 认证会话测试时间确定性

认证会话、refresh session 和 token version validator 测试 MUST 避免使用固定 `time.Sleep` 作为 Redis session 排序、本地缓存过期或异步状态变化的唯一依据。测试 MUST 使用确定性 score/clock、可观察条件或真实 cache 的 eventually-style 断言表达预期。

#### Scenario: refresh session 上限裁剪测试使用确定性顺序
- **WHEN** Redis refresh session store 测试验证超过每用户活跃 session 上限时裁剪最旧 session
- **THEN** 测试 MUST 使用确定性 Redis score、可注入时间输入或可观察排序条件建立 session 顺序
- **AND** 测试 MUST NOT 依赖循环中的固定 `time.Sleep` 制造不同创建时间

#### Scenario: token version 本地缓存过期测试使用条件等待
- **WHEN** token version validator 测试验证本地缓存 TTL 过期后重新回源
- **THEN** 测试 MUST 使用 `require.Eventually` 或等价条件等待直到重新回源发生
- **AND** 测试 MUST 保留真实 `localcache` 实例验证缓存行为
- **AND** 测试 MUST NOT 在固定 `time.Sleep` 后直接断言回源调用次数

#### Scenario: 认证测试不引入测试专用生产 API
- **WHEN** 认证测试需要控制时间、顺序或异步状态
- **THEN** 测试 MUST 优先使用现有可观测存储状态、测试数据构造、gomock expectation、通道或局部 helper
- **AND** 正式代码 MUST NOT 仅为了测试新增无运行时职责的全局 clock、test hook 或兼容分支
