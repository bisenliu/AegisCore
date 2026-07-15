## ADDED Requirements

### Requirement: RBAC composition graph 实例投影

RBAC feature 的 Fx composition MUST 在同一 provider 同时被 concrete 与多个 interface 消费时提供同一实例的全部必需视图。无 DI metadata、配置转换、错误包装、业务语义或 lifecycle 职责的构造器 MUST 直接注册或使用普通参数，不得通过额外 helper 隐式表达 identity projection。RBAC composition 结构调整 MUST 保持权限目录、角色绑定、授权判断、policy reload、Redis policy version、Pub/Sub watcher、health 和 metrics 的业务语义不变。

#### Scenario: concrete 与 interface 使用同一实例

- **WHEN** RBAC provider 构造的对象同时被 concrete 类型和一个或多个 interface 消费
- **THEN** Fx graph MUST 暴露该对象的 concrete 视图和所有必需 interface 视图
- **AND** 这些视图 MUST 指向同一次 constructor 调用产生的同一实例
- **AND** 不得通过重复注册同一 constructor 产生多个等价实例来满足不同消费者

#### Scenario: 保留 concrete 消费方所需视图

- **WHEN** provider 的 concrete 类型仍被初始加载、health check、watcher 输入、显式 lifecycle invoke 或其他内部消费者使用
- **THEN** composition MUST 保留该 concrete 输出
- **AND** interface projection MUST NOT 使 concrete 消费方无法解析

#### Scenario: 无语义构造器直接表达

- **WHEN** 构造器只转发到另一个 constructor、只把同一个 concrete 返回为 interface、或只包装一个无 `name`、`optional`、group tag 的普通依赖
- **THEN** composition MUST 直接注册真实 constructor、使用 provider annotation 表达 projection 或使用普通函数参数
- **AND** 不得为了 DI 形态保留无业务语义、无配置转换、无错误包装且无 lifecycle 职责的 helper

#### Scenario: 必需依赖不通过 optional 静默降级

- **WHEN** 正式 RBAC graph 始终注册某个 provider，且禁用行为由该 provider 返回 no-op 实现表达
- **THEN** 消费方 MUST 将该依赖声明为必需输入
- **AND** 缺少该 provider 时 graph MUST 构造失败
- **AND** 禁用配置 MUST 继续通过既有 no-op 实现保持运行语义

#### Scenario: composition 标准化不改变 RBAC 行为

- **WHEN** RBAC composition 的 provider 注册方式、projection 表达或普通参数形态发生调整
- **THEN** 权限目录、route diff、角色、角色权限、用户角色、Casbin reload、Redis policy version、Pub/Sub、watcher 补偿和授权结果 MUST 保持不变
- **AND** HTTP API、OpenAPI、Ent schema、Atlas migration、配置、部署资产、metrics 名称和日志字段 MUST 保持不变
