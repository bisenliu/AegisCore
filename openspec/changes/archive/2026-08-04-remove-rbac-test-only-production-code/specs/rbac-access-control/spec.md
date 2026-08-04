## ADDED Requirements

### Requirement: RBAC 生产源码与测试辅助代码边界

RBAC 生产源码 MUST 只保留生产运行、生成期框架入口或稳定服务 API 真实消费的实现。仅为测试 fake、测试数据组装或未来示例存在的构造器、helper、alias、wrapper 和注释伪代码 MUST 位于测试编译单元或被删除；测试 MUST 直接验证现有生产入口和能力所有者，不得为测试便利扩大生产 API。

#### Scenario: watcher 测试使用 fake 依赖

- **WHEN** permission Redis watcher 测试需要注入 `policySubscriptionStore` fake 或测试 metrics
- **THEN** 测试专用构造 MUST 位于 `_test.go` 并复用生产内部核心构造
- **AND** 生产源码 MUST 只保留真实运行路径消费的 `NewWatcher` 及其内部实现，MUST NOT 保留测试专用转发构造器或扩大 `WatcherParams` 依赖类型

#### Scenario: permission transport 验证显式授权白名单

- **WHEN** permission HTTP transport 测试验证显式授权白名单绕过语义
- **THEN** transport MUST 直接使用 `common/http/middleware` 拥有的 Casbin whitelist rule 和 option
- **AND** permission feature MUST NOT 保留没有服务专用语义的 type alias、转发 wrapper、兼容名称或虚构生产消费者

#### Scenario: 默认角色 catalog 只描述当前基线

- **WHEN** `internal/shared/rbacbaseline` 定义当前默认系统角色及权限绑定
- **THEN** 生产 catalog MUST 只包含当前真实角色和绑定所需代码
- **AND** 系统 MUST NOT 为尚未存在的默认角色保留未消费 helper、注释掉的 role block、示例 ID 或兼容分支

#### Scenario: 生产调用图静态检查

- **WHEN** CI 或开发者对 user-service 运行不包含测试入口的生产调用图 deadcode 检查
- **THEN** RBAC 手写生产代码 MUST NOT 报告仅由测试引用的 watcher 构造器、baseline helper 或 authorization wrapper
- **AND** Ent schema/mixin 生成期入口和其他明确归属 capability 的测试支持入口 MUST 单独复核，不得通过删除共享公开 API 或生成期入口消除报告
