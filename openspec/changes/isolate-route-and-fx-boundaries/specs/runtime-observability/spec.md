## MODIFIED Requirements

### Requirement: 健康检查与运行时端点

系统 MUST 在业务 API 之外提供 `/livez`、`/readyz`、`/startupz`、配置化 metrics endpoint、OpenAPI 文档端点和可选 pprof 诊断监听，并保持这些端点的访问边界和启停语义明确。健康检查 MUST 只通过稳定 public contract 读取跨 feature 运行状态，MUST NOT 直接依赖 feature infrastructure concrete implementation。

#### Scenario: 存活、就绪与启动检查

- **WHEN** 调用 `/livez`
- **THEN** endpoint MUST 只证明进程可响应，并 MAY 在外部依赖异常时继续成功
- **WHEN** PostgreSQL、Redis、Casbin policy 或 policy watcher 等就绪依赖不可用
- **THEN** `/readyz` 或 `/startupz` MUST 失败并返回可定位且不含 secret、DSN、SQL、token、Cookie、stacktrace 的信息

#### Scenario: 运行时路由不经过业务授权

- **WHEN** user-service 注册健康检查、OpenAPI 或 metrics 路由
- **THEN** 路由 MUST 位于 `/api/v1` 之外，MUST NOT 经过 RBAC 业务授权
- **AND** metrics 配置无效时路由注册 MUST 返回错误，而不是静默使用错误配置

#### Scenario: HTTP 服务禁用

- **WHEN** `server.http.enabled=false`
- **THEN** user-service MUST 不启动 HTTP 监听
- **AND** 依赖 HTTP 的健康检查、OpenAPI 和 metrics 路由 MUST 不对外暴露

#### Scenario: pprof 受控暴露

- **WHEN** pprof 未显式启用
- **THEN** 系统 MUST 不启动 pprof listener
- **WHEN** pprof 显式启用
- **THEN** 系统 MUST 使用独立诊断 listener，并默认限制在 loopback 或受控网络边界

#### Scenario: 健康检查依赖 public contract

- **WHEN** service-level provider 构造 Casbin policy 或 policy watcher 健康检查
- **THEN** provider MUST 依赖 permission feature 暴露的只读 health/status interface
- **AND** provider MUST NOT import permission infrastructure casbin、redis watcher 或其他 concrete implementation 包

### Requirement: user-service HTTP route 总装边界

系统 MUST 由 user-service composition root 统一维护 HTTP route 的访问层级，并通过明确的 route registrar contract 接入 feature 路由。route registrar MUST 按 public、authenticated 和 authorized 层级注册，MUST NOT 依赖 Fx value group 的 slice 顺序表达安全或冲突语义。

#### Scenario: 分层注册业务路由

- **WHEN** user-service 注册 `/api/v1` 路由
- **THEN** public auth route MUST 不经过普通 access token middleware
- **AND** authenticated auth route MUST 经过 token version validator 认证 middleware
- **AND** permission、role 和 user 业务 route MUST 先经过认证 middleware，再经过 RBAC authorizer middleware

#### Scenario: 新增 feature route

- **WHEN** 新 feature 需要挂载 `/api/v1` 业务路由
- **THEN** feature MUST 通过对应访问层级的 route registrar contract 接入 composition root
- **AND** 新 feature MUST NOT 要求在 `RegisterRouteParams` 或 `router.RouteParams` 中新增 feature controller 字段

#### Scenario: 禁止依赖 value group 顺序

- **WHEN** route registrar 通过 Fx value group 注入
- **THEN** 注册逻辑 MUST NOT 假设 group slice 顺序稳定
- **AND** 如果存在 path 冲突、顺序或 middleware 层级要求，composition root MUST 使用显式编排或稳定排序规则表达该要求

#### Scenario: route graph 行为保持

- **WHEN** route registrar 化完成后运行 route graph 测试或 route diff 诊断
- **THEN** 已有健康检查、OpenAPI、metrics、auth、permission、role 和 user route 的 path、method、访问层级和 route template MUST 保持不变
- **AND** 本变更 MUST NOT 改变 OpenAPI 文档语义或业务 controller 行为
