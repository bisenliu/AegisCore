## ADDED Requirements

### Requirement: Go 测试按维护主题组织

认证与 provider 相关 Go 测试 MUST 按可独立维护的行为主题组织文件，避免单个测试文件长期承载多个不相关子主题。测试拆分 MUST 保持原有业务断言覆盖，不得通过删除关键场景降低覆盖范围。

#### Scenario: auth Redis session store 测试拆分

- **WHEN** 协作者维护 `user-service/internal/features/auth/infrastructure/redis` 的 session store 测试
- **THEN** token version cache、token version validator、refresh session 创建查询删除、refresh session rotation、全量 session 删除、purge pool/Fx lifecycle 和 Redis key schema 测试 MUST 分布在按主题命名的 `_test.go` 文件中
- **AND** 旧的跨主题大型 `session_store_test.go` MUST NOT 继续承载这些全部场景

#### Scenario: auth command use case 测试拆分

- **WHEN** 协作者维护 `user-service/internal/features/auth/application/command` 的 command use case 测试
- **THEN** login、change-password、refresh、logout current、logout all 和共享构造 helper MUST 分布在按主题命名的 `_test.go` 文件中
- **AND** 旧的跨 use case 大型 `service_test.go` MUST NOT 继续承载这些全部场景

#### Scenario: provider routes 与 Gin engine 测试拆分

- **WHEN** 协作者维护 `user-service/internal/providers` 的 routes 或 Gin engine 测试
- **THEN** auth middleware、route 注册冲突、tracing、request ID、HTTP metrics、panic recovery 和 runtime endpoint skip 测试 MUST 分布在按主题命名的 `_test.go` 文件中
- **AND** 单个 provider 测试文件 MUST NOT 同时承载所有 route、metrics、tracing 和 panic 场景

#### Scenario: 拆分后保持测试集合完整

- **WHEN** 大型测试文件被拆分
- **THEN** 协作者 MUST 对比拆分前后的 `Test` 函数集合或等价测试清单
- **AND** 目标包 `go test` MUST 通过

### Requirement: 复杂测试替身使用生成 mock

Go 测试中表示外部 collaborator port 调用契约的复杂 fake、stub 或 spy MUST 使用包内 `mockgen` 生成物替代。仅用于构造领域值、提供无行为分支统计快照、真实 miniredis/localcache 夹具或简单不可变配置的测试 helper MAY 保留在 `_test.go` 文件内。

#### Scenario: collaborator 调用契约使用 mockgen

- **WHEN** 测试需要断言 credential store、token issuer、refresh session store、token version store/cache、RBAC seed service、authorizer、watcher 或 metrics collaborator 的调用、参数、顺序或失败路径
- **THEN** 测试 MUST 使用同包或同 feature 测试包内的 `go.uber.org/mock/mockgen` 生成 mock 设置 expectation
- **AND** 测试 MUST NOT 通过复杂手写 fake/stub/spy 字段隐藏这些 collaborator 调用契约

#### Scenario: mock 生成入口归属

- **WHEN** 新增或替换测试 collaborator mock
- **THEN** `mock_generate.go` MUST 位于消费该 mock 的包或 feature-local 测试边界内
- **AND** 生成 mock MUST NOT 放入全局 `mocks/` 包或跨 feature 共享 mock 包
- **AND** 生成入口 MUST 使用可复现的 `go tool mockgen` 或仓库约定等价入口

#### Scenario: 允许轻量测试 helper

- **WHEN** 测试 helper 只构造领域对象、返回固定 stats、运行真实 workerpool task、包装 miniredis、包装真实 localcache 或提供无外部调用契约的配置值
- **THEN** helper MAY 保留为 `_test.go` 内部类型或函数
- **AND** helper MUST NOT 替代 mockgen 记录外部 port 调用、失败注入或调用顺序

### Requirement: Metrics no-op 生成约定一致

feature-local 业务 metrics interface 的 no-op 实现 MUST 继续通过业务中立生成器或统一生成约定维护。`common/runtime/observability/metrics` MUST 只承载生成器和通用 runtime metrics 能力，不得承载 user-service feature 的业务 metrics 方法。

#### Scenario: feature-local no-op 生成

- **WHEN** auth、permission 或其他 feature 定义 `Metrics` interface 且需要默认空实现
- **THEN** feature MUST 通过统一的 `nopgen` 生成约定生成 `metrics_nop_gen.go` 或等价 no-op 生成物
- **AND** no-op 生成物 MUST 与 feature-local `Metrics` interface 编译匹配

#### Scenario: common 不承载业务指标方法

- **WHEN** 统一 metrics no-op 生成约定被调整
- **THEN** `common/runtime/observability/metrics` MUST NOT 定义 auth 登录、refresh、logout、session purge、RBAC policy reload、watcher 或 route diff 等 user-service 业务指标方法
- **AND** auth/permission 业务指标方法 MUST 保留在所属 feature 边界内

#### Scenario: 生成物 drift 可验证

- **WHEN** metrics interface 或 mock 源 interface 变化
- **THEN** 仓库生成与完整验证流程 MUST 能更新对应生成物
- **AND** 未同步生成物 MUST 通过 `git diff --exit-code` 或等价 drift 检查暴露
