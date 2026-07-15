## ADDED Requirements

### Requirement: user-service App 配置单一来源与可复用装配

user-service 正式 `serve` 启动路径 MUST 在创建 Fx App 前只解析一次 service config，并将同一个已解析 `*serviceconfig.Config` 及由它派生的共享 runtime config 交给 composition root。user-service bootstrap MUST 提供无配置文件 I/O 的基础 Fx options 构建入口，供正式 App 和装配测试复用；该入口 MUST NOT 保留 `ConfigPath -> serviceconfig.NewConfig` 的第二套 provider 链。

#### Scenario: serve 启动只解析一次配置

- **WHEN** `aegiscore-user-services serve` 使用配置文件启动
- **THEN** CLI MUST 在创建 Fx App 前解析并校验一次 service config
- **AND** App factory MUST 接收该已解析配置对象而不是再次接收配置路径
- **AND** composition root MUST NOT 再次读取该配置文件

#### Scenario: service config 与 runtime config 同源

- **WHEN** composition root 构建 user-service Fx options
- **THEN** 系统 MUST supply CLI 已解析的同一个 `*serviceconfig.Config`
- **AND** 共享 runtime config MUST 由该 service config 在 composition root 中派生并 supply
- **AND** logger、observability、server 和资源 provider MUST 消费这一组同源配置对象

#### Scenario: 正式 App 与测试复用基础 options

- **WHEN** 正式 `NewApp` 或 bootstrap/providers 装配测试构建 user-service 依赖图
- **THEN** 它们 MUST 能复用同一个无 I/O 基础 Fx options 构建入口
- **AND** 测试 MAY 在该入口上追加 `fx.NopLogger`、`fx.Populate` 或测试所需替代项
- **AND** 正式代码 MUST NOT 为测试引入可变全局 loader、test-only flag 或第二套 service config provider

#### Scenario: 配置失败不创建 App

- **WHEN** service config 文件不存在、包含未知字段或未通过校验
- **THEN** CLI MUST 在调用 App factory 或 `fx.New` 前返回配置错误
- **AND** 系统 MUST NOT 创建部分 App、资源或 lifecycle hook

#### Scenario: 保持配置与业务契约

- **WHEN** user-service 将 App 配置来源统一为已解析对象
- **THEN** 系统 MUST 保持配置字段、默认值、环境变量覆盖和校验语义不变
- **AND** 系统 MUST NOT 改变 HTTP/OpenAPI、数据库 schema、migration、认证、RBAC 或资源运行时行为
