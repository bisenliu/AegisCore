## MODIFIED Requirements

### Requirement: RBAC policy sync 故障注入验收

系统 SHALL 提供可在 CI 中运行的 RBAC policy sync 故障注入验收测试，覆盖数据库 revision、同步通知、dispatcher、watcher、Casbin projection 和用户角色 cache 在故障、乱序、重放和并发写入下的最终收敛。测试 MUST 使用通道、barrier、eventually-style 条件或明确 deadline 控制并发时序，MUST NOT 使用固定 `time.Sleep` 作为状态已变化的主要判断依据。

#### Scenario: Redis 故障恢复后副本无需新写即可收敛

- **WHEN** 在线 RBAC 写入已成功提交数据库 revision，但 Redis version 发布或 Pub/Sub 通知在故障注入下失败，随后 Redis 恢复且没有新的 RBAC 写入
- **THEN** 故障注入测试 MUST 验证 watcher 或周期性版本补偿最终使所有参与副本的 lag 归零
- **AND** 每个副本的 applied revision MUST 收敛到数据库最新 revision
- **AND** 每个副本的 Casbin projection 和用户角色 cache 解析结果 MUST 与数据库权威关系一致

#### Scenario: reload 逆序完成时最新 revision 保持权威

- **WHEN** 两次 RBAC policy reload 被故障注入控制为后发 revision 先完成、先发 revision 后完成
- **THEN** 故障注入测试 MUST 验证最终 applied revision 仍为最新 revision
- **AND** 旧 revision 的 reload 结果 MUST NOT 覆盖较新的 Casbin projection 或用户角色 cache 状态
- **AND** 授权 allow/deny 结果 MUST 与最新数据库关系一致

#### Scenario: Add Remove Replace 重放保持幂等收敛

- **WHEN** 角色权限或用户角色绑定的 Add、Remove、Replace 同步事件被故障注入为重复投递、乱序投递或 dispatcher 重试
- **THEN** 故障注入测试 MUST 验证通知不丢失且重放不会产生非幂等破坏
- **AND** 最终数据库 revision、applied revision、Casbin projection 和用户角色 cache MUST 收敛到最后一次成功提交的数据库状态

#### Scenario: 100 并发 RBAC 写入最终收敛

- **WHEN** 测试并发执行 100 个 RBAC 写操作，并注入 loader 阻塞、watcher 消息乱序或 cache loader 延迟
- **THEN** 故障注入测试 MUST 验证所有成功提交写入对应的最终数据库 revision 可被观察到
- **AND** 所有参与副本的 applied revision MUST 最终等于最新数据库 revision
- **AND** 抽样或完整授权断言 MUST 证明 Casbin projection 和用户角色 cache 与最终数据库关系一致

#### Scenario: 测试说明记录风险与收敛条件

- **WHEN** 新增或更新 RBAC policy sync 故障注入测试
- **THEN** `docs/TESTING.md` 或相关测试说明 MUST 记录每个故障注入场景对应的风险、预期收敛条件和运行方式
- **AND** 文档 MUST 明确完整真实 PostgreSQL/Redis 容器门禁通过根 `make test-containers` 运行，窄化调试通过显式 `-args -aegiscore.testcontainers` 启用
