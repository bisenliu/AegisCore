## ADDED Requirements

### Requirement: token version validator 测试替身一致性

token version validator 的单元测试 MUST 使用本包已有 gomock 生成物表达 `UserTokenVersionStore` 与 `TokenVersionCache` 依赖交互，并 MUST 保留真实 `localcache` 实例验证本地缓存行为。测试 MUST NOT 保留 `tokenVersionUserTestStore` 或 `tokenVersionSessionTestStore` 手写兼容替身。

#### Scenario: Redis miss 后回源并回填
- **WHEN** token version validator 在本地缓存未命中且 Redis token version 投影未命中时执行校验
- **THEN** 测试 MUST 通过 gomock expectation 表达 Redis miss、PostgreSQL 当前值回源和 Redis 投影回填
- **AND** 测试 MUST 继续使用真实 `localcache` 验证后续本地缓存命中

#### Scenario: singleflight 合并并发回源
- **WHEN** 同一用户的多个并发 token version 校验同时触发回源路径
- **THEN** 测试 MUST 通过 `DoAndReturn`、channel、mutex 或 atomic 计数表达并发控制
- **AND** 测试 MUST 断言 PostgreSQL 当前值回源被 singleflight 合并

#### Scenario: 按用户隔离并发校验
- **WHEN** 不同用户的并发 token version 校验同时触发回源路径
- **THEN** 测试 MUST 表达不同用户之间不共享 singleflight 结果
- **AND** 每个用户的依赖调用 MUST 通过 gomock expectation 独立断言

#### Scenario: 失效后重新加载
- **WHEN** token version validator 的本地 token version 缓存被失效后再次校验同一用户
- **THEN** 测试 MUST 通过 gomock expectation 表达重新读取 Redis 或 PostgreSQL 当前值
- **AND** 旧本地缓存值 MUST NOT 继续作为校验依据
