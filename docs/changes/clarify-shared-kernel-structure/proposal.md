# Clarify Shared Kernel Structure

## What

评估 `deployments/k8s/user-services/`、`user-service/internal/shared/identity/status.go` 和 `user-service/internal/shared/` 当前结构，形成一组明确的共享内核边界决策：

- 不从 `deployments/k8s/user-services/` 迁移任何内容到 `user-service/internal/shared/`。
- 将 `user-service/internal/shared/identity/status.go` 重命名为 `user_status.go`，为未来 shared 公共枚举建立“按业务语义命名”的文件规则。
- 不按 feature 模块重新分类 `user-service/internal/shared/`，继续保留当前按业务内核子域命名的结构。
- 补充未来如需扩展 shared 时的推荐目录方案和准入理由，避免把公共 err、enum、部署资产、运行时配置、Kubernetes 清单或 feature 私有 helper 放入兜底目录。

## Why

`deployments/k8s/user-services/` 当前只有 Kubernetes 清单目录说明，明确表示目录预留给未来用户服务运行时 Kubernetes 清单，当前没有可直接运行的清单。Kubernetes Deployment、Service、Ingress、Secret、ConfigMap、探针、资源限制和镜像参数都属于部署资产或运行时发布配置，不是用户服务内稳定业务内核。

`internal/shared` 的现有准入规则要求能力必须已被至少两个 feature 真实消费、表达稳定业务规格或纯值对象、且不能归入 `common`。部署目录中的内容不满足这些条件，也不能导入 Go 业务代码。把部署内容迁入 shared 会混淆部署边界和业务内核边界。

`identity/status.go` 当前只承载 `UserStatus` 枚举、解析和账号生命周期判断。考虑到后续一定会出现更多公共错误或枚举，如果现在继续使用过于泛化的 `status.go`，未来容易在同一 shared 子包里出现多个状态模型时再被动调整。因此本变更将其一次性调整为 `user_status.go`，并规定公共枚举按 `<subject>_status.go`、`<subject>_type.go` 或 `<subject>_kind.go` 命名。

当前 `internal/shared` 只有 `identity` 与 `rbacbaseline` 两个稳定子包。它们分别对应 user/auth 共同身份内核与 role/permission 共同 RBAC 系统规格。后续即使出现更多公共 err 或 enum，也应进入 owning shared 子包，例如 `identity/errors.go` 或 `identity/user_status.go`，而不是新增根级 `shared/errors`、`shared/enums`、`shared/types`、`shared/utils` 或 `shared/helpers`。再按 `user`、`auth`、`role`、`permission` 等 feature 模块重新分类，会把 shared 重新拉回 feature ownership，反而削弱“跨 feature 稳定子域”的语义。

## Scope

包括：

- 明确 `deployments/k8s/user-services/README.md` 没有可迁移到 shared 的内容。
- 给出未来 Kubernetes 内容的归属判断：保留在 `deployments/k8s/` 或部署模板体系，不进入 `internal/shared`。
- 说明 `identity/status.go` 重命名为 `user_status.go` 的依据。
- 给出 shared 增长后的公共错误、枚举和值对象放置规则。
- 提供后续实现任务，便于在 `/opsx:apply` 或普通实现流程中落地文档补充。

## Non-Goals

- 不新增 `openspec/` 或 `docs/opsx/` 工件。
- 不迁移 Kubernetes 清单、部署 README、配置样例或 Secret/ConfigMap 模板到 Go shared 包。
- 不修改 Kubernetes 部署形态，不新增 Deployment、Service、Ingress、Secret、ConfigMap、Helm chart 或 Kustomize overlay。
- 不修改 Go package import path。
- 不把 `internal/shared` 重组为按 feature 命名的 `user/`、`auth/`、`role/`、`permission/` 子目录。
- 不新增业务 shared 子包，不放宽 shared 准入规则。

## Acceptance Criteria

- 变更文档明确说明 `deployments/k8s/user-services/` 当前没有内容需要迁移到 `internal/shared`。
- 变更文档列出如果未来出现 Kubernetes 清单或部署模板，哪些内容仍应留在 `deployments/`，哪些最多可作为应用内稳定业务规格被其他路径消费。
- `identity/status.go` 已重命名为 `identity/user_status.go`，Go package import path 不变。
- 变更文档明确当前不需要按 feature 模块重分类 `internal/shared`，并给出推荐目录结构。
- `AGENTS.md`、`docs/ARCHITECTURE.md` 和 architecture lint 明确禁止根级 shared 兜底包，部署资产不得迁入 shared。
