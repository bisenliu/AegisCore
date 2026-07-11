## MODIFIED Requirements

### Requirement: 共享 runtime primitive 测试稳定性

`common/runtime` 中 localcache、workerpool、scheduler 和 timezone 等共享 runtime primitive 的测试 MUST 避免使用固定 `time.Sleep` 或手动 `os.Setenv` 恢复来表达异步进度、过期状态或全局环境隔离。测试 MUST 使用可观察条件、通道同步、testing 环境隔离或确定性输入表达预期。涉及 `TZ`、`time.Local` 或 timezone 包级初始化状态的测试 MUST 将这些进程级全局状态作为同一个隔离单元管理，并 MUST 通过包内受控 reset helper 重置初始化状态。

#### Scenario: localcache 过期测试使用条件等待
- **WHEN** localcache 测试验证 TTL 过期后缓存未命中
- **THEN** 测试 MUST 使用 `require.Eventually` 或等价条件等待断言未命中状态
- **AND** 测试 MUST NOT 在固定 `time.Sleep` 后立即断言过期结果

#### Scenario: localcache 并发回源测试使用通道同步
- **WHEN** localcache 测试验证同 key 并发 miss 被 `singleflight` 合并
- **THEN** 测试 MUST 通过通道、atomic 计数或 wait group 明确确认 goroutine 已进入目标等待点
- **AND** 测试 MUST NOT 依赖固定 `time.Sleep` 猜测 goroutine 调度状态

#### Scenario: workerpool 状态等待使用条件断言
- **WHEN** workerpool 测试等待任务进入 running、waiting、completed、failed 或 stopped 状态
- **THEN** 测试 MUST 使用条件等待 helper、`require.Eventually` 或通道信号
- **AND** 测试 MUST NOT 使用固定 `time.Sleep` 表达后台状态已经变化

#### Scenario: scheduler 自动续租测试使用可观察条件
- **WHEN** scheduler 测试验证自动续租或任务取消行为
- **THEN** 测试 MUST 通过锁记录、任务通道或 eventually-style 条件断言观察续租结果
- **AND** 测试 MUST NOT 仅通过任务内部固定 sleep 制造续租窗口

#### Scenario: timezone 测试隔离全局环境
- **WHEN** timezone 测试修改 `TZ`、`time.Local` 或包级初始化状态
- **THEN** 测试 MUST 使用集中 helper 保存并恢复 `TZ`、`time.Local` 和包级初始化状态
- **AND** 测试 MUST 使用 `t.Setenv` 管理环境变量恢复
- **AND** 包级初始化状态 reset MUST 通过持有 `timezoneState` 内部锁的包内受控 helper 完成
- **AND** 测试 MUST 通过 `t.Cleanup` 恢复 `time.Local` 和包级状态
- **AND** 这些测试 MUST NOT 使用 `t.Parallel`
