## ADDED Requirements

### Requirement: 角色权限绑定基础设施关键路径测试

role infrastructure MUST 提供默认可执行的测试覆盖角色权限绑定中不依赖 PostgreSQL 行锁的持久化路径，包括列表、删除、系统绑定同步、缺失权限、重复输入去重、失败保持和映射 helper。依赖 `FOR UPDATE` 的新增和替换路径 MUST 保持生产 PostgreSQL 锁语义不变，并 MAY 由显式 Docker-backed PostgreSQL 集成测试覆盖。

#### Scenario: 默认测试覆盖非锁定绑定路径
- **WHEN** 协作者执行 `go test -cover ./user-service/internal/features/role/infrastructure/postgres`
- **THEN** 测试 MUST 默认执行 `RolePermissionStore.ListByRoleID`、`Remove`、`EnsureSystemBindings`、`SyncSystemBindings` 和映射 helper 的成功与错误路径
- **AND** 默认覆盖率 MUST 达到 70% 以上

#### Scenario: 同步失败保持原绑定
- **WHEN** `RolePermissionStore.SyncSystemBindings` 请求引用缺失权限
- **THEN** 测试 MUST 断言方法返回明确错误
- **AND** 测试 MUST 断言失败前已有角色权限绑定保持不变

#### Scenario: 默认测试覆盖系统绑定同步
- **WHEN** 默认测试执行 `RolePermissionStore.SyncSystemBindings`
- **THEN** 测试 MUST 覆盖新增缺失绑定、删除多余绑定和保留既有绑定
- **AND** 测试 MUST 断言返回的新增与删除统计符合持久化结果

#### Scenario: 同步失败保持可诊断结果
- **WHEN** `SyncSystemBindings` 因缺失权限、查询失败或事务写入失败无法完成
- **THEN** 测试 MUST 覆盖错误映射或 rollback 路径
- **AND** 测试 MUST 断言不会把部分成功伪装为完整同步成功

#### Scenario: PostgreSQL 集成测试不承担默认覆盖唯一来源
- **WHEN** `AEGISCORE_TEST_CONTAINERS` 未设置
- **THEN** 默认测试 MAY 跳过 Docker-backed PostgreSQL 集成测试
- **AND** 默认测试仍 MUST 覆盖角色权限绑定非锁定核心路径
- **AND** 生产代码 MUST NOT 为 SQLite 测试新增跳过 `FOR UPDATE` 的兼容分支
