## MODIFIED Requirements

### Requirement: 用户 feature 边界与日志

系统 MUST 将用户资料能力按 feature-local 的 application、domain、transport 和 infrastructure 边界组织；application port MUST 由消费侧拥有，HTTP DTO MUST 留在 transport，Ent 访问 MUST 留在 PostgreSQL adapter。用户 feature 的 domain、application、infrastructure 和 transport 生产包 MUST 使用 framework-neutral constructor 与消费侧 port 注入，MUST NOT 导入 Fx/Dig 或携带 Fx/Dig metadata；正式业务路径的日志 MUST 来自 constructor 注入的 logger 或 request context，MUST NOT 依赖可变的 package-level 默认 logger。

#### Scenario: 用户写侧和读侧用例

- **WHEN** 新增或维护用户写侧、查询或列表行为
- **THEN** 写侧编排 MUST 位于 `application/command`，读侧编排 MUST 位于 `application/query`
- **AND** application MUST 通过自身拥有的最小 port 访问基础设施，MUST NOT 导入 HTTP DTO 或 Ent predicate 包

#### Scenario: HTTP 输入和持久化边界

- **WHEN** controller 处理 path、query 或 body 字段
- **THEN** request/response DTO MUST 位于 `transport/http`，输入裁剪和归一化 MUST 在 feature-local input preparer 完成
- **AND** input preparer MUST NOT 查询 store、调用 use case、执行授权或写 HTTP 响应
- **AND** Ent 查询和 predicate 构造 MUST 留在 `infrastructure/postgres`

#### Scenario: framework-neutral 基础设施构造

- **WHEN** 用户 feature 的基础设施 adapter 暴露生产 constructor
- **THEN** constructor MUST 使用显式普通 Go 参数表达依赖，并由消费侧通过 application port 注入
- **AND** domain、application、infrastructure 和 transport 生产包 MUST NOT 导入 `go.uber.org/fx`、`go.uber.org/dig` 或声明 `fx.In`、`fx.Out`、`dig.In`、`dig.Out`
- **AND** 基础设施 constructor MUST NOT 通过 `name:"primary_db"` 等 Fx/Dig struct tag 暴露服务级命名资源 metadata

#### Scenario: Fx composition 层适配

- **WHEN** 服务运行时通过 Fx 组装用户资料 feature
- **THEN** Fx module MAY 在 composition 文件中适配服务级命名资源并提供用户 application port
- **AND** 该适配 MUST NOT 要求 PostgreSQL adapter 保留 Fx/Dig 专用参数结构、兼容 wrapper 或旧 constructor

#### Scenario: 显式日志依赖

- **WHEN** 用户 application、HTTP 边界或关键 PostgreSQL adapter 记录正式业务日志
- **THEN** logger MUST 由 constructor 显式注入或从当前 request context 获取
- **AND** request logger MUST 保留可用的 `request_id`、`trace_id` 和 `span_id`
- **AND** 缺省构造 MUST 使用局部 nop logger 或 fail-fast，MUST NOT 安装或读取可变进程级默认 logger
