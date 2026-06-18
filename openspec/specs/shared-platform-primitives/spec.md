## Purpose

定义 `common/` 提供的跨服务共享契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力，保证服务间基础行为一致。

## Requirements

### Requirement: 跨服务契约基础

系统 MUST 在 `common/` 中维护跨服务共享的错误、响应 envelope、分页和 HTTP response helper，以保证服务之间的外部契约保持一致，并保持业务中立。

#### Scenario: 返回统一响应

- **WHEN** 服务处理成功响应或错误响应
- **THEN** 系统 MUST 使用共享响应和错误契约表达 code、message、data、pagination 或错误详情

#### Scenario: 新服务复用契约

- **WHEN** 新服务模块需要对外暴露 HTTP API
- **THEN** 该服务 MUST 优先复用 `common/contract/` 和 `common/http/response/` 中的稳定契约，而不是定义不兼容的 envelope

#### Scenario: 契约变更需要规格化

- **WHEN** 共享错误码、响应 envelope 或分页结构需要改变
- **THEN** change MUST 更新相关主规格或 delta spec，并评估所有使用 `common/contract/` 的服务影响

#### Scenario: 新增错误码

- **WHEN** 需要在 `common/contract/errors` 新增跨服务错误分类
- **THEN** 错误码 MUST 保持业务中立，并可通过公共响应 helper 渲染

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

#### Scenario: 服务特定授权行为

- **WHEN** 授权行为依赖 user-service 的 `user:<uuid>` subject、角色、权限目录、route diff 或超级管理员基线
- **THEN** 行为 MUST 留在 user-service permission/shared 边界，不得放入 `common/security/casbin` 或通用 HTTP middleware

#### Scenario: 服务特定 OpenAPI 元数据

- **WHEN** OpenAPI 元数据描述 user-service API server、认证方案、源码扫描范围、健康路径或输出目录
- **THEN** 元数据 MUST 位于 user-service 脚本或薄 wrapper，不得放入 `common/http/openapi`

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

#### Scenario: auth Redis key schema

- **WHEN** 认证功能需要 refresh session、token version 或撤销相关 Redis key
- **THEN** 认证 infrastructure MUST 拥有功能 key schema，只能复用 `common/runtime/rediskey` 的通用构造规则

#### Scenario: workerpool 业务边界

- **WHEN** feature 代码使用 `common/runtime/workerpool` 提交后台任务
- **THEN** workerpool MUST 只提供并发控制、生命周期、日志和统计能力，MUST NOT 承载 refresh session 上限裁剪、token version 撤销、可靠消息、eventbus、outbox 或业务一致性协议

#### Scenario: scheduler 分布式锁

- **WHEN** 定时任务具有多实例副作用
- **THEN** 任务 MUST 声明锁策略，锁 TTL MUST 为正值，长任务 SHOULD 具备续租策略

### Requirement: 服务内 shared kernel

系统 MUST 将 `user-service/internal/shared` 限制为服务内稳定业务内核，只承载已被至少两个 feature 真实消费、边界稳定且不能归入 `common` 的纯业务规格。

#### Scenario: 当前允许共享包

- **WHEN** user/auth 需要用户状态或身份错误，role/permission 需要系统 RBAC 基线
- **THEN** 系统 MUST 分别使用 `internal/shared/identity` 和 `internal/shared/rbacbaseline`，并 MUST NOT 在 feature 内复制常量或保留兼容 alias

#### Scenario: 新增共享 helper

- **WHEN** helper 只被一个 feature 使用或只是技术工具函数
- **THEN** 系统 MUST NOT 将其放入 `internal/shared`

#### Scenario: shared 子包命名

- **WHEN** 确需新增或调整 shared 子包
- **THEN** 包名 MUST 使用稳定业务内核语义，MUST NOT 新增根级 `errors`、`enums`、`types`、`utils` 或 `helpers` 兜底包；公共错误和枚举 MUST 放入 owning 子包的具体文件

#### Scenario: shared 禁止依赖

- **WHEN** 拟新增 shared 包需要 Ent、SQL、Redis、Gin、Fx、HTTP response、runtime provider、feature use case、store port、DTO、外部调用或部署资产
- **THEN** 设计 MUST 被拒绝或移动到所属 feature 的 application/infrastructure 边界

### Requirement: 外部集成边界

系统 MUST 将 `user-service/internal/integration/http|grpc|events` 作为真实外部系统协议适配和防腐层边界，避免推测性集成污染服务内架构。

#### Scenario: 新增外部 HTTP client

- **WHEN** feature 需要调用真实外部 HTTP 系统
- **THEN** feature application MUST 拥有最小消费侧端口，`integration/http` MAY 实现协议适配器

#### Scenario: 禁止推测性集成

- **WHEN** 没有单独批准的设计
- **THEN** 系统 MUST NOT 新增 Kafka、RabbitMQ、NATS、Redis Stream、eventbus、outbox、producer、subscriber、consumer handler、dispatcher、Ent hook 或 transaction wrapper

#### Scenario: gRPC 与事件边界分离

- **WHEN** user-service 暴露真实入站 gRPC API 或消费真实外部 broker 事件
- **THEN** 入站 gRPC handler MUST 位于所属 feature 的 `transport/grpc`；broker subscription、ack 和协议机制 MAY 位于 `integration/events`，feature-specific 映射和 handler adapter MUST 位于所属 feature 的 `infrastructure/consumers`

### Requirement: 测试基础设施

系统 MUST 在 `common/testing/` 中提供可复用的容器和 fixture 能力，用于支撑 Postgres、Redis 和测试数据场景。

#### Scenario: 集成测试需要依赖服务

- **WHEN** Go 测试需要真实 Postgres 或 Redis
- **THEN** 测试 MUST 优先使用 `common/testing/containers/` 启动依赖，避免每个模块重复实现容器生命周期

#### Scenario: 测试数据需要稳定生成

- **WHEN** 测试需要生成用户、角色、权限或其他输入数据
- **THEN** 测试 MUST 优先使用共享 fixture 或本 feature 内明确的 builder，避免随机数据破坏可重复性
