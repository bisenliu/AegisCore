# OPSX 能力地图

本文件把 AegisCore 的长期稳定 capability、主要代码位置和 OpenSpec 主规格连接起来。处理新需求时，先在这里确认归属，再决定是否创建或更新 change。

## 1. 基线能力

| Capability | 业务说明 | 主要代码位置 | 主规格 | 状态 |
|---|---|---|---|---|
| `opsx-foundation` | 仓库级 OPSX/OpenSpec 目录、配置、文档入口、能力地图和变更工作流 | `AGENTS.md`、`docs/opsx/`、`openspec/config.yaml` | `openspec/specs/opsx-foundation/spec.md` | ready |
| `shared-platform-primitives` | 跨服务契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力 | `common/contract/`、`common/http/`、`common/runtime/`、`common/security/`、`common/testing/`、`common/validation/` | `openspec/specs/shared-platform-primitives/spec.md` | ready |
| `user-identity-management` | 用户资料创建、查询、列表、状态约束和用户 HTTP 边界 | `user-service/internal/features/user/`、`user-service/internal/shared/identity/` | `openspec/specs/user-identity-management/spec.md` | ready |
| `auth-session-management` | 登录、令牌签发、刷新、退出、改密、会话和 token version 校验 | `user-service/internal/features/auth/`、`common/security/auth/`、`common/security/password/` | `openspec/specs/auth-session-management/spec.md` | ready |
| `rbac-access-control` | 权限目录、角色、角色权限、用户角色、Casbin 授权、RBAC seed 和超级管理员引导 | `user-service/internal/features/permission/`、`user-service/internal/features/role/`、`common/security/casbin/`、`user-service/cmd/rbac.go` | `openspec/specs/rbac-access-control/spec.md` | ready |
| `runtime-observability` | 健康检查、OpenAPI、metrics、tracing、logging、pprof、Prometheus/Grafana 资产 | `user-service/internal/router/`、`common/runtime/observability/`、`common/http/middleware/`、`deployments/observability/`、`deployments/compose/grafana/` | `openspec/specs/runtime-observability/spec.md` | ready |
| `delivery-operations` | 构建、测试、lint、OpenAPI 生成、Ent/Atlas migration、运行时镜像、migration 镜像、Docker、Compose、Kubernetes、Helm 和发布顺序 | `Makefile`、`user-service/Makefile`、`user-service/scripts/`、`deployments/` | `openspec/specs/delivery-operations/spec.md` | ready |

## 2. 关键入口点

- 服务入口：`user-service/cmd/main.go`。
- RBAC CLI：`user-service/cmd/rbac.go`。
- HTTP 聚合路由：`user-service/internal/router/router.go`。
- 认证路由：`user-service/internal/features/auth/transport/http/routes.go`。
- 权限路由：`user-service/internal/features/permission/transport/http/routes.go`。
- 角色路由：`user-service/internal/features/role/transport/http/routes.go`。
- 用户路由：`user-service/internal/features/user/transport/http/routes.go`。
- 共享契约：`common/contract/` 和 `common/http/response/`。
- 配置与依赖：`common/runtime/config/`、`common/runtime/datastore/`、`user-service/internal/providers/`。
- 观测资产：`deployments/observability/` 和 `deployments/compose/grafana/`。

## 3. 交叉依赖

- `auth-session-management` 依赖 `user-identity-management` 的用户状态和用户 ID，也依赖 `shared-platform-primitives` 的 JWT、password、Redis 和 response helper。
- `rbac-access-control` 依赖用户 ID、权限目录、角色绑定、Casbin authorizer 和 HTTP route scanner。
- `runtime-observability` 横跨 `common/runtime/observability/`、HTTP middleware、router、deployments 和 dashboard 生成脚本。
- `delivery-operations` 横跨 Makefile、脚本、Ent、Atlas、OpenAPI、运行时镜像、migration 镜像、Docker、Compose、Kubernetes、Helm 和发布说明。
- `opsx-foundation` 不改变业务行为，但约束所有后续 capability 的文档和变更流程。

## 4. 待补能力

这些能力当前并入较大的主规格，后续如果变更频繁，可以拆成独立主规格：

| 候选 capability | 当前归属 | 拆分触发条件 |
|---|---|---|
| `contract-response-envelope` | `shared-platform-primitives` | 响应 envelope、错误码或分页契约需要独立演进 |
| `runtime-scheduler` | `shared-platform-primitives` | scheduler、lock 或 workerpool 成为多个服务共享的独立变更热点 |
| `openapi-generation` | `runtime-observability` 和 `delivery-operations` | OpenAPI 生成、转换或发布流程需要单独治理 |
| `migration-management` | `delivery-operations` | Ent/Atlas schema 和 migration 流程出现跨服务扩展 |

## 5. 使用规则

1. 新需求先定位 capability；找不到归属时先更新本文件或提出新的 capability。
2. 如果需求改变稳定行为，必须创建 `openspec/changes/<change-name>/specs/<capability>/spec.md`。
3. 如果只改实现且不改变规格行为，tasks 中仍要引用对应 capability 和验证命令。
4. change 完成并归档后，检查 `openspec/specs/<capability>/spec.md` 是否与本地图一致。
