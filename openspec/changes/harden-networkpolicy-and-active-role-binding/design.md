## Context

user-service 当前 Kubernetes 原生清单与 Helm 默认值均渲染 `NetworkPolicy`，但入站来源使用 `namespaceSelector: {}` 加单个 Pod 标签，出站业务依赖端口没有 `to` 目的地约束。该策略依赖租户标签治理，一旦普通 namespace 可以自行打准入标签或 Pod 可以访问任意同端口服务，就无法通过 NetworkPolicy 精确表达最小网络访问意图。

RBAC 当前写路径允许把已停用角色绑定给用户。授权热路径、有效权限查询和 Casbin policy loader 已过滤停用角色，因此不存在直接越权，但管理接口会保存不会生效的绑定，和“绑定后后续访问权限生效”的契约不一致。

本 change 同时影响部署资产与 role feature 写侧语义，需要同步 OpenSpec、Kubernetes/Helm 清单、准入策略说明、Go 领域错误、应用层校验和测试。

## Goals / Non-Goals

**Goals:**

- 将 user-service 默认 NetworkPolicy 收敛到显式来源和显式目的地，避免任意 namespace 标签和任意目的端口放行成为生产默认值。
- 要求 admission policy 限制 user-service 网络准入标签的使用范围，防止未授权租户通过标签旁路入站控制。
- 将用户角色绑定写路径收紧为只接受存在且启用的角色，停用角色绑定整体拒绝且不写入关系。
- 保持授权热路径继续只使用启用角色与启用权限，不改变 Casbin subject/object/action schema。

**Non-Goals:**

- 不新增数据库字段、Ent schema 或 Atlas migration。
- 不改变 HTTP 路由、请求 DTO、OpenAPI 路径结构或 Casbin model。
- 不引入新的跨服务 common helper，也不把 user-service RBAC 业务规则移动到 `common/` 或 `internal/shared/`。
- 不支持对旧 NetworkPolicy 宽松默认值或停用角色绑定行为的兼容开关。

## Decisions

### NetworkPolicy 默认值使用显式选择器

静态 Kubernetes 清单和 Helm values 的默认 ingress MUST 使用具体 `namespaceSelector` 与 `podSelector` 组合表达允许来源。egress MUST 将 PostgreSQL、Redis、OTLP Collector 与 DNS 拆分为独立规则，每条业务依赖规则必须包含 `to`，并通过目标 namespace、目标 Pod 标签或目标 `ipBlock` 精确约束目的地。

备选方案：继续保留 `namespaceSelector: {}`，只在注释中要求环境覆盖。该方案无法把安全边界固化为仓库默认行为，容易在生产沿用宽松默认值，因此拒绝。

### Helm 暴露结构化 networkPolicy values

Helm chart 继续直接渲染 `.Values.networkPolicy.ingress` 和 `.Values.networkPolicy.egress`，但默认 values 必须改为安全基线。环境需要访问外部托管 PostgreSQL、Redis 或 Collector 时，通过覆盖 values 中对应 `to.ipBlock.cidr` 或选择器完成，不新增模板分支。

备选方案：在模板中加入数据库、Redis、Collector 专用字段并生成规则。该方案会增加模板逻辑和重复表达，当前直接 values 渲染已能覆盖 Kubernetes NetworkPolicy 原生能力，因此不采用。

### admission policy 作为标签治理工件

新增 Kubernetes admission policy 资产或说明，限制 `aegiscore.io/allow-user-service` 或替代准入标签只能由受信任 namespace、网关或平台受控 workload 使用。该规则属于部署安全边界，放在 `deployments/`，不进入 Go 运行时代码。

备选方案：只依赖 RBAC 或人工约定管理标签。该方案无法在集群准入阶段阻止旁路标签，因此拒绝。

### 用户角色绑定在 application 层拒绝 inactive role

`roleCommandService.AddUserRole` 与 `ReplaceUserRoles` 应在调用 `UserRoleStore` 写入前检查 `RoleStore` 返回的角色 `Active` 状态。任一角色停用时返回明确领域错误并跳过写入、通知和缓存失效。该规则属于 role feature 业务契约，放在 `user-service/internal/features/role/application/command` 与 role domain 错误中，不下沉到 Ent predicate 或 permission feature。

备选方案：在 PostgreSQL store 的 `GetByRoleID/GetByRoleIDs` 中追加 `ActiveEQ(true)`。该方案会改变通用角色查询端口语义，影响查询、更新、停用保护和其他需要读取停用角色的场景，因此拒绝。

### 停用角色绑定错误保持明确业务错误

新增或复用明确错误，使 HTTP 层能映射为客户端可理解的失败，而不是返回泛化内部错误。错误应位于 role domain，避免在 transport 或 infrastructure 中定义业务语义。

备选方案：把停用角色视为 `ErrRoleNotFound`。该方案隐藏真实原因，不利于管理侧诊断，因此拒绝。

## Risks / Trade-offs

- [Risk] 不同集群的 PostgreSQL、Redis 和 Collector 部署标签不一致，默认 NetworkPolicy 可能需要环境覆盖。→ Mitigation：默认 values 使用清晰的目标标签约定，并在 README 或 tasks 中要求用 `helm template` 检查渲染结果。
- [Risk] 外部托管数据库或 Redis 没有 Pod selector 可用。→ Mitigation：允许环境覆盖使用 `ipBlock` 精确 CIDR，但不得回退为无 `to` 的任意目的端口放行。
- [Risk] admission policy 能力依赖 Kubernetes 版本或集群准入控制器。→ Mitigation：仓库提供 Kubernetes 原生 `ValidatingAdmissionPolicy` 或等价说明；不支持时必须用 Gatekeeper/Kyverno 等平台等价策略承接。
- [Risk] 已有自动化流程尝试绑定停用角色会失败。→ Mitigation：这是预期的非兼容收紧；调用方必须先启用角色再绑定。
- [Risk] 仅修改 application 层校验可能无法阻止直接数据库写入。→ Mitigation：仓库契约只承诺服务 API 和 CLI 写路径；直接数据库写入不属于受支持路径。

## Migration Plan

1. 更新 OpenSpec delta，明确 NetworkPolicy 与用户角色绑定的新强约束。
2. 更新 Kubernetes 静态清单、Helm 默认 values、admission policy 资产或说明，并验证 `helm lint` 与 `helm template` 输出。
3. 更新 role domain 错误、应用层 Add/Replace 用户角色绑定校验、HTTP 错误映射和相关测试。
4. 运行相关 Go 包测试、Helm 渲染验证、`make user-service-architecture-lint`。
5. 完成实现、规格和文档任务后，暂存本次预期变更，再运行 `make lint` 和 `make verify`。

回滚方式：如果目标环境 NetworkPolicy 收紧导致连接失败，回滚到上一版部署资产或用明确 selector/ipBlock 修正 values 后重新发布；不得恢复无目的地约束的端口放行。若角色绑定收紧影响调用方，调用方应启用目标角色后重试，不提供兼容开关。

## Open Questions

- 无。
