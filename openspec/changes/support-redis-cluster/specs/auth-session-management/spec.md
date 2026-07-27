## ADDED Requirements

### Requirement: 认证 Redis 存储兼容 Redis Cluster

认证 Redis adapter MUST 在 Redis Cluster client 上保持 refresh session、password-change session、token version 投影和全部退出的安全语义。所有参与同一原子操作、Lua 脚本、transaction/pipeline 或批量删除的认证 Redis key MUST 使用同一用户维度 hash tag 落入同一 hash slot。

#### Scenario: 同一用户多 key 操作落入同一 slot

- **WHEN** 系统为同一用户创建 refresh session、用户 session zset、purge zset、password-change session 或 token version projection key
- **THEN** 这些 key MUST 使用同一 `user_id` hash tag
- **AND** create、rotate、delete、delete-all、consume password-change session 和 token version 投影更新 MUST NOT 触发 Redis Cluster `CROSSSLOT` 错误

#### Scenario: Cluster 下保持安全撤销语义

- **WHEN** Redis Cluster 可用且用户 refresh、退出全部会话、已认证改密或强制改密成功
- **THEN** 系统 MUST 保持原有 token version 主事实、Redis 投影、refresh session 撤销和一次性 session 原子消费语义
- **AND** Redis Cluster 操作失败时 MUST fail-closed，并按现有撤销不完整或无效凭据错误语义返回

#### Scenario: 不迁移旧 Redis session

- **WHEN** 系统从旧 Redis 单机配置切换到 Redis Cluster 配置
- **THEN** 系统 MUST NOT 查询、迁移、删除或双写旧 Redis prefix 或旧 Redis 实例中的认证数据
- **AND** 旧 refresh session 或 password-change session 缺失 MUST 按未找到或无效凭据处理
