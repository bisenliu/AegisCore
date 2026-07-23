## MODIFIED Requirements

### Requirement: 业务与运行时路由访问边界

系统 MUST 由 user-service composition root 统一维护 HTTP route 的访问层级，并显式集中挂载当前固定的 auth、permission、role 和 user feature 路由。composition root MUST 按 public、authenticated 和 authorized 层级注册路由，MUST NOT 依赖 Fx value group 的 slice 顺序表达安全、冲突或必需路由语义。

#### Scenario: 分层注册业务路由

- **WHEN** user-service 注册 `/api/v1` 路由
- **THEN** public auth route MUST 不经过普通 access token middleware
- **AND** authenticated auth route MUST 经过 token version validator 认证 middleware
- **AND** permission、role 和 user 业务 route MUST 先经过认证 middleware，再经过 RBAC authorizer middleware

#### Scenario: 固定 feature 路由集中注册

- **WHEN** user-service 注册当前固定 feature 的 `/api/v1` 业务路由
- **THEN** composition root MUST 显式挂载 auth、permission、role 和 user 的 transport route 函数
- **AND** 生产路由装配 MUST NOT 要求这些固定 feature 通过 feature-local route registrar 或 Fx route value group 接入
- **AND** 如果存在 path 冲突、顺序或 middleware 层级要求，composition root MUST 使用显式编排表达该要求

#### Scenario: route graph 可验证

- **WHEN** 运行 route graph 测试或 route diff 诊断
- **THEN** 健康检查、OpenAPI、metrics、auth、permission、role 和 user route 的 path、method、访问层级和 route template MUST 可被稳定验证
- **AND** 必需认证或授权依赖缺失时系统 MUST 拒绝部分注册，而不是降级开放
- **AND** 必需 feature controller 缺失导致固定业务路由无法挂载时系统 MUST 在构图或注册阶段失败，而不是启动缺失路由的服务
