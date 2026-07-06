## Why

当前 user-service 的默认 Kubernetes NetworkPolicy 入站仅依赖跨 namespace 可伪造的 Pod 标签，出站仅按端口放行 PostgreSQL、Redis 和 OTLP，无法约束具体上游与依赖端点。用户角色绑定写路径也允许绑定已停用角色，虽然授权热路径不会授予权限，但管理侧会出现“绑定成功但不生效”的语义不一致。

## What Changes

- **BREAKING** 收紧 user-service 默认 NetworkPolicy：入站来源必须显式约束 namespace 与 Pod，出站 PostgreSQL、Redis、OTLP Collector 必须显式约束目的 namespace、podSelector 或 ipBlock，不再用任意 namespace 或任意目的地址作为生产默认值。
- **BREAKING** 增加准入标签治理要求：普通租户或未授权 namespace 不得自行使用 user-service 网络准入标签绕过 NetworkPolicy 来源约束。
- **BREAKING** 收紧用户角色绑定契约：Add 和 Replace 用户角色绑定仅接受存在且启用的角色，任一角色停用时整体拒绝写入。
- 保持授权热路径现有安全语义：Casbin policy、用户角色 resolver 和有效权限查询继续只使用启用角色与启用权限。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: 部署资产中的 user-service NetworkPolicy 默认安全边界从标签提示式放行收紧为显式来源与目的地约束，并要求 admission policy 限制准入标签使用。
- `rbac-access-control`: 用户角色绑定从“存在的角色”收紧为“存在且启用的角色”，停用角色绑定请求必须被拒绝。

## Impact

- 部署资产：影响 `deployments/k8s/user-services/networkpolicy.yaml`、`deployments/helm/aegiscore-user-services/values.yaml` 和 Helm 渲染出的 `NetworkPolicy`。
- 集群治理：需要新增或更新 Kubernetes admission policy，限制 `aegiscore.io/allow-user-service` 或替代准入标签的使用范围。
- RBAC 写路径：影响 `user-service/internal/features/role/application/command/binding.go`、角色领域错误、HTTP 错误映射和相关单元测试。
- OpenSpec：需要同步 `delivery-operations` 与 `rbac-access-control` 的行为约束。
- 兼容性：已有环境如果依赖任意 namespace 标签放行或任意目的端口 egress，需要改为显式声明允许来源和依赖端点；已有自动化若尝试绑定停用角色，将改为失败。
