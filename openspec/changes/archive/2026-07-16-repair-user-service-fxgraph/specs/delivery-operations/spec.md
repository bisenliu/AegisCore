## MODIFIED Requirements

### Requirement: user-service Fx 依赖图生成

系统 MUST 为 user-service 提供可执行的 Fx 依赖图生成入口，并通过带 `user-service-` 前缀的交付命令暴露给协作者。该入口 MUST 复用正式 App 的基础 input/options builder 组装诊断图，并通过命令层提供 service config、派生 runtime config、logger 和无外部副作用的资源替身，使诊断图与正式运行图保持一致且不连接真实外部依赖。

#### Scenario: 生成 user-service 依赖图

- **WHEN** 协作者执行 user-service Fx 依赖图生成命令
- **THEN** 系统 MUST 基于 user-service 当前顶层 Fx module 生成依赖图文件
- **AND** 生成过程 MUST 复用 `common/` 中的业务中立 Fx 依赖图 helper
- **AND** 生成过程 MUST 复用正式 App 的基础 input/options builder
- **AND** 生成过程 MUST 提供 `*serviceconfig.Config` 和由该配置派生的 runtime config 输入

#### Scenario: 依赖图生成不产生外部副作用

- **WHEN** 协作者执行 `cd user-service && go run ./cmd fxgraph --config ./configs/config.yaml --output /tmp/aegis-fx.dot`
- **THEN** 命令 MUST 成功写入非空 DOT 文件
- **AND** 输出 MUST 包含 AppModule 或等价顶层 App module 以及 auth、user、role、permission 等关键 feature 节点或依赖边
- **AND** 命令 MUST NOT 连接真实 PostgreSQL、Redis、OTLP 或启动 HTTP server

#### Scenario: 根 Makefile 使用服务前缀

- **WHEN** 仓库根 `Makefile` 暴露 user-service Fx 依赖图生成能力
- **THEN** 目标名称 MUST 使用 `user-service-` 前缀
- **AND** 根 `Makefile` MUST NOT 新增无服务上下文的 `fxgraph-generate`、`dependency-graph` 或等价目标

#### Scenario: 依赖图 drift 可检查

- **WHEN** user-service provider、module 或 invoke 关系变化后重新生成依赖图
- **THEN** 系统 MUST 能通过提交的生成物 diff 或专用 check 命令暴露依赖图 drift
- **AND** drift 检查 MUST 覆盖版本控制中的 user-service Fx dependency graph 资产

#### Scenario: fxgraph 测试验证真实渲染

- **WHEN** `user-service/cmd` 测试验证 fxgraph 装配
- **THEN** 测试 MUST 实际调用 `common/runtime/fxgraph` 的 DOT renderer 或等价渲染入口
- **AND** 测试 MUST 断言输出包含关键 AppModule、feature 节点或依赖边
- **AND** 缺少 `*serviceconfig.Config` 等关键 App 输入时测试 MUST 失败
- **AND** 测试 MUST NOT 仅通过 option 数量断言替代真实渲染验证
