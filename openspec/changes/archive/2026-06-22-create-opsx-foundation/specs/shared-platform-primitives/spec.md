## ADDED Requirements

### Requirement: 跨服务契约基础

系统 MUST 在 `common/` 中维护跨服务共享的错误、响应 envelope、分页和 HTTP response helper，以保证服务之间的外部契约保持一致。

#### Scenario: 返回统一响应

- **WHEN** 服务处理成功响应或错误响应
- **THEN** 系统 MUST 使用共享响应和错误契约表达 code、message、data、pagination 或错误详情

#### Scenario: 新服务复用契约

- **WHEN** 新服务模块需要对外暴露 HTTP API
- **THEN** 该服务 MUST 优先复用 `common/contract/` 和 `common/http/response/` 中的稳定契约，而不是定义不兼容的 envelope

#### Scenario: 契约变更需要规格化

- **WHEN** 共享错误码、响应 envelope 或分页结构需要改变
- **THEN** change MUST 更新相关主规格或 delta spec，并评估所有使用 `common/contract/` 的服务影响

### Requirement: HTTP 与安全中间件基础

系统 MUST 在 `common/http/` 和 `common/security/` 中提供可复用的绑定、校验、认证、授权、CORS、metrics、logging、recovery、OpenAPI 和 pprof 基础能力。

#### Scenario: HTTP 请求进入服务

- **WHEN** HTTP 请求被 Gin 路由处理
- **THEN** 服务 MUST 能复用共享 middleware 完成认证上下文、授权检查、日志字段、metrics 采集、panic recovery 和 span error 记录

#### Scenario: 输入校验失败

- **WHEN** 请求绑定或字段校验失败
- **THEN** 系统 MUST 通过共享 binding、validation 和 response helper 返回一致的校验错误结构

#### Scenario: OpenAPI 输出

- **WHEN** 服务生成或转换 OpenAPI 文档
- **THEN** 系统 MUST 复用 `common/http/openapi/` 的转换与渲染约束，避免服务间文档格式漂移

### Requirement: Runtime primitive 基础

系统 MUST 在 `common/runtime/` 中维护配置加载、数据存储、logger、metrics、tracing、scheduler、workerpool、localcache、Redis key 和 timezone 等 runtime primitive。

#### Scenario: 服务启动加载配置

- **WHEN** 服务通过配置文件启动
- **THEN** 系统 MUST 使用共享配置 loader 与 validation 解析 runtime、HTTP、auth、Postgres、Redis、metrics、tracing 和 logger 配置

#### Scenario: runtime 依赖初始化

- **WHEN** 服务需要连接 Postgres、Redis、logger、metrics 或 tracing provider
- **THEN** 服务 MUST 优先复用 `common/runtime/` 中的 provider 和 Fx module

#### Scenario: 后台任务执行

- **WHEN** 服务需要执行定时任务、分布式锁或固定 worker pool 任务
- **THEN** 系统 MUST 使用共享 scheduler、lock、workerpool 和 metrics 约束，并记录失败、拒绝、panic 和完成事件

### Requirement: 测试基础设施

系统 MUST 在 `common/testing/` 中提供可复用的容器和 fixture 能力，用于支撑 Postgres、Redis 和测试数据场景。

#### Scenario: 集成测试需要依赖服务

- **WHEN** Go 测试需要真实 Postgres 或 Redis
- **THEN** 测试 MUST 优先使用 `common/testing/containers/` 启动依赖，避免每个模块重复实现容器生命周期

#### Scenario: 测试数据需要稳定生成

- **WHEN** 测试需要生成用户、角色、权限或其他输入数据
- **THEN** 测试 MUST 优先使用共享 fixture 或本 feature 内明确的 builder，避免随机数据破坏可重复性
