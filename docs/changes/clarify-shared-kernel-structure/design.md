# Design

## Current Findings

`deployments/k8s/user-services/` 当前只有：

```text
deployments/k8s/user-services/README.md
```

README 说明该目录预留给未来用户服务运行时 Kubernetes 清单，当前没有提交可直接运行的清单，并要求只有在部署形态、必需配置、密钥处理方式和验证步骤明确后才新增清单。

`user-service/internal/shared/` 当前只有：

```text
user-service/internal/shared/
  identity/
    errors.go
    user_status.go
  rbacbaseline/
    catalog.go
```

现有架构规则已经把 shared 限定为用户服务内稳定业务内核，只允许被至少两个 feature 真实消费的纯类型、值对象、系统内置规格、稳定错误和无副作用判断。

## Kubernetes Content Migration Decision

不迁移 `deployments/k8s/user-services/` 中的任何内容到 `user-service/internal/shared/`。

理由：

- 当前目录没有 Kubernetes YAML，只有部署资产边界说明，没有可迁移的业务规格。
- Kubernetes 清单属于部署资产，所有权在 `deployments/k8s/`，不是 Go 业务包。
- Kubernetes 配置表达的是 runtime wiring、镜像、环境变量、Secret 引用、探针、资源配额、副本数和服务暴露方式；这些都不满足 shared 的纯业务内核准入条件。
- 即使未来新增 YAML，也应保留在 `deployments/k8s/`、`deployments/helm/` 或对应部署模板中。应用内代码最多读取已经归一化后的业务配置，不应反向依赖部署清单。
- 如果未来 Kubernetes 清单需要引用稳定业务概念，例如 service name、RBAC seed 命令名或健康检查路径，也应该由部署模板引用现有文档或配置约定，不应把 Kubernetes 片段搬入 `internal/shared`。

### Future Placement Rules

未来如 `deployments/k8s/user-services/` 增加内容，建议按以下规则放置：

| 内容 | 推荐位置 | 是否进入 `internal/shared` | 理由 |
|---|---|---|---|
| Deployment、Service、Ingress、HPA、PodDisruptionBudget | `deployments/k8s/user-services/` 或 Helm template | 否 | 部署拓扑，不是业务内核 |
| ConfigMap、Secret 示例或引用 | `deployments/k8s/`、`deployments/helm/`、部署文档 | 否 | 运行时配置和密钥处理，不是纯值对象 |
| 健康检查 URL | 部署清单直接使用 `/healthz`，长期规则登记在架构文档 | 否 | HTTP runtime route，由 router 拥有 |
| CLI 命令示例 | 部署文档或 Makefile | 否 | 操作入口，不是共享业务类型 |
| 系统角色、系统权限、默认绑定 | `internal/shared/rbacbaseline` | 是，已存在 | role/permission 共同消费的稳定 RBAC 业务规格 |
| 用户状态和账号生命周期判断 | `internal/shared/identity` | 是，已存在 | user/auth 共同消费的稳定身份业务规格 |

## User Status File Naming Decision

将 `user-service/internal/shared/identity/status.go` 重命名为：

```text
user-service/internal/shared/identity/user_status.go
```

命名依据：

- Go 文件名应优先表达文件内主要类型或职责。该文件核心类型是 `UserStatus`，文件名应包含 `user` 这一 subject。
- Shared 后续会继续出现公共错误、枚举和值对象；尽早使用 subject-specific 文件名，可以避免未来出现多种 status 时再做整理。
- 包名提供上位上下文，文件名提供局部语义：`identity/user_status.go` 读作“身份内核中的用户状态”。
- 当前包内还有 `errors.go`。`user_status.go` 与 `errors.go` 的分工清晰：一个承载用户状态和值行为，一个承载身份稳定错误。
- Go import 不受文件名影响，重命名不改变 package API 或调用方。

不推荐命名为 `account_status.go`，除非代码中的领域语言也从 `UserStatus` 调整为 `AccountStatus`。文件名、类型名和业务术语应保持一致。

## Shared Directory Classification Decision

当前不需要按模块对 `user-service/internal/shared/` 重新分类。

现有结构：

```text
user-service/internal/shared/
  identity/
  rbacbaseline/
```

推荐继续保留。理由：

- `identity` 是 user/auth 共同消费的身份业务内核，不属于单一 feature。
- `rbacbaseline` 是 role/permission 共同消费的 RBAC 系统规格，不属于单一 feature。
- 按 feature 模块重组为 `shared/user`、`shared/auth`、`shared/role`、`shared/permission` 会暗示 shared 只是 feature 私有代码外置，容易造成跨 feature helper 下沉。
- 当前子包数量很少，增加中间层会降低可读性。
- 现有目录已与架构文档和 `AGENTS.md` 对齐，重分类收益不足。

同时不新增根级技术分类包：

- 不新增 `shared/errors`。公共错误放在 owning shared 子包的 `errors.go`，例如 `identity/errors.go`。
- 不新增 `shared/enums`。公共枚举放在 owning shared 子包，并按业务语义命名，例如 `identity/user_status.go`。
- 不新增 `shared/types`、`shared/utils` 或 `shared/helpers`。如果能力无法归入一个稳定 shared 子域，说明它还不满足 shared 准入条件。

## Recommended Structure

短期推荐结构：

```text
user-service/internal/shared/
  identity/
    errors.go
    doc.go
    user_status.go
    user_status_test.go
  rbacbaseline/
    doc.go
    catalog.go
    catalog_test.go
```

如果未来 shared 增长，仍推荐按稳定业务内核子域分类，而不是按 feature 分类：

```text
user-service/internal/shared/
  identity/
    errors.go
    user_status.go
  rbacbaseline/
    catalog.go
  <new-shared-kernel>/
    ...
```

新增 `<new-shared-kernel>` 必须先满足：

- 至少两个 feature 已经真实消费。
- 能力边界稳定，不是为了避免 import 或复用小 helper。
- 内容是纯类型、值对象、系统内置规格、稳定错误或少量无副作用判断。
- 无 Gin、Ent、Redis、SQL、Fx、HTTP response、runtime provider、controller、DTO、store port、use case、外部调用或数据库/缓存访问。
- 同步更新 `AGENTS.md` 和 `docs/ARCHITECTURE.md`，登记 owner、消费方、准入理由和禁止事项。
- 不新增根级 `errors`、`enums`、`types`、`utils` 或 `helpers` 兜底包。

推荐文件命名：

| 内容 | 推荐文件名 | 示例 |
|---|---|---|
| 共享稳定错误 | `errors.go` | `identity/errors.go` |
| 状态枚举 | `<subject>_status.go` | `identity/user_status.go` |
| 类型或种类枚举 | `<subject>_type.go`、`<subject>_kind.go` | `example/resource_type.go` |
| 系统内置规格 | `catalog.go` 或 `<subject>_catalog.go` | `rbacbaseline/catalog.go` |

## Alternatives Considered

### Move Deployment Facts Into Shared

拒绝。部署事实属于部署资产和发布流程。即使其中包含 service name、container port 或 health path，也不应通过 Go shared 包管理 Kubernetes 清单。

### Keep `status.go`

拒绝。当前虽然只有一个状态类型，但用户明确希望一次性为未来公共错误和枚举建立清晰规则。`user_status.go` 更适合作为后续 shared 枚举文件的样板。

### Reclassify Shared By Feature Module

拒绝。Shared 的价值在于表达跨 feature 稳定业务内核。按 feature 分类会弱化准入规则，并鼓励把 feature 私有能力搬到 shared。

### Add `shared/rbac` Parent Directory

暂缓。`rbacbaseline` 当前只承载系统初始规格；permission feature 仍拥有授权、route diff 和 Casbin adapter，role feature 仍拥有角色生命周期。新增 `shared/rbac/` 容易被误解为 RBAC 全域 shared owner。

## Implementation Impact

本变更落地轻量结构调整和长期规则，不改变业务行为。

落地项：

- 将 `identity/status.go` 与测试重命名为 `identity/user_status.go` 和 `identity/user_status_test.go`。
- 新增 `internal/shared/README.md`，记录 shared 目录规则。
- 新增 `identity/doc.go` 和 `rbacbaseline/doc.go`，让每个 shared 子包自描述 owner 和职责。
- 在 `docs/ARCHITECTURE.md` 和 `AGENTS.md` 补充部署内容不进入 shared、公共 err/enum 不进根级兜底包的规则。
- 在 architecture lint 中禁止根级 `shared/errors`、`shared/enums`、`shared/types`、`shared/utils`、`shared/helpers`，并要求 identity 用户状态文件使用 `user_status.go`。

验证方式：

```bash
make architecture-lint
```

如只改文档，可额外运行：

```bash
rg -n "deployments/k8s|internal/shared|identity/(status|user_status)\\.go" AGENTS.md docs/ARCHITECTURE.md docs/changes/clarify-shared-kernel-structure
```
