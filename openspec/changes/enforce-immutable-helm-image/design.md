## Context

当前 `deployments/helm/aegiscore-user-service/values.yaml` 是生产基线 values，默认 `image.tag: latest` 且 `image.pullPolicy: IfNotPresent`。模板 helper 将 `image.repository` 与 `image.tag` 拼成 `repository:tag`，Deployment 与 RBAC seed Job 都复用该结果；`Chart.appVersion` 也保留 `latest` fallback。默认渲染结果会生成 `aegiscore-user-service:latest`，在多节点 Kubernetes 中会受到节点镜像缓存影响，导致同一 Helm release 下 Pod 或 Job 运行不同镜像 digest。

本 change 只处理 user-service 生产 Helm 发布镜像不可变问题，归属 `delivery-operations`。变更影响部署资产、CI/CD 发布契约和文档，不影响 Go 业务代码、HTTP API、OpenAPI、Ent schema、Atlas migration、Nacos 配置 schema 或观测指标语义。

## Goals / Non-Goals

**Goals:**

- Helm chart 的生产路径必须显式接收不可变 image ref，默认渲染不得产生 `:latest`。
- Deployment 与 RBAC seed Job 必须使用同一个已构建、已扫描、已生成 SBOM 的不可变镜像工件。
- CI/CD 必须以 digest 或 `sha-<commit>` tag 表达发布镜像，并将同一引用传入 Helm。
- Helm lint、template 或架构检查必须能阻止 `latest` 进入生产渲染结果。
- README 和 OpenSpec 明确发布、验证和回滚边界。

**Non-Goals:**

- 不保留 `latest`、`Chart.appVersion: latest` 或空 tag fallback 的生产兼容路径。
- 不为旧 release、旧 values schema 或旧镜像命名增加兼容分支。
- 不改变 Dockerfile 的编译方式、基础镜像 digest、运行时 UID/GID 或容器安全上下文。
- 不修改业务 API、数据库 schema、Nacos 配置字段、RBAC seed 业务逻辑或 Kubernetes migration 策略。

## Decisions

### 使用单一不可变 image ref 作为 Helm 生产镜像契约

Helm values 使用单一字段表达最终镜像引用，例如 `image.ref: registry.example.com/aegiscore/user-service@sha256:<digest>`。Deployment 和 RBAC seed Job 直接消费该字段，避免 `repository`、`tag` 和 `appVersion` 多处组合导致 fallback 漂移。

选择该方案的原因是生产发布需要审计最终工件，而不是在 chart 内推导 tag。digest 是首选表达；若发布平台只能使用 tag，也必须使用 `sha-<commit>` 这类不可变 tag，并在任务中保留 digest 身份记录。

不采用继续保留 `image.repository` + `image.tag` 的方案，因为它容易继续允许空 tag fallback、`latest` 覆盖或不同模板自行拼接。

### 模板层直接失败而不是静默替换

Helm 模板必须在缺少 image ref 或检测到 `:latest` 时失败。失败应发生在 `helm lint` 或 `helm template` 阶段，避免将有风险的 manifest 交给 Kubernetes 或 GitOps 控制器。

不采用将 `latest` 自动替换为 `sha-<commit>` 的方案，因为 chart 无法可靠知道 CI 构建产物、registry digest 和安全扫描结果，静默替换会掩盖发布输入错误。

### CI/CD 发布使用同一镜像工件贯穿扫描和 Helm 发布

CI/CD 构建 user-service 镜像后必须 push 不可变引用，记录 image ID/digest，并对同一工件执行内容断言、漏洞扫描和 SBOM 生成。Helm 发布只接收该已验证引用。

不采用扫描 `:ci`、发布 `:latest` 或重新构建发布镜像的方案，因为这会使安全证据与生产运行工件脱钩。

### 文档和规格同步表达生产禁止 `latest`

Helm README、发布流程和 `delivery-operations` delta 必须明确生产 chart 不允许 `latest`。本 change 的实现完成后，归档会把该要求合并到主规格，作为长期交付契约。

## Risks / Trade-offs

- [Risk] 现有本地 Helm 示例依赖 `latest`，切换为必填不可变 ref 后本地渲染命令会失败。→ Mitigation：本地示例也显式传入测试用 `sha-<commit>` tag 或 digest，占位值必须不是 `latest`。
- [Risk] 生产发布平台当前可能没有 push 镜像或解析 digest 的步骤。→ Mitigation：实现时把构建、push、digest 解析、扫描和 Helm 参数传递作为同一发布任务链路，不保留旧发布入口。
- [Risk] 仅检查 values 文件可能漏掉命令行 `--set image.ref=...:latest`。→ Mitigation：在 Helm 模板 helper 中 fail，并补充渲染失败测试或架构检查。
- [Risk] 使用 digest 后镜像字符串不再显示可读版本号。→ Mitigation：CI 仍可同时推送 `sha-<commit>` tag 并在 release 记录中保存 tag 与 digest 映射，但 Helm 生产部署以不可变引用为准。

## Migration Plan

1. 修改 Helm values schema，移除生产默认 `latest`，改为必填不可变 image ref。
2. 修改 Helm image helper、Deployment 和 RBAC seed Job，使二者直接使用同一个 image ref，并在缺失或 `latest` 时失败。
3. 修改 CI/CD 发布流程，使构建、push、digest 解析、扫描、SBOM 和 Helm 发布复用同一镜像工件。
4. 更新 Helm README 和发布说明，移除生产 `latest` 示例，增加不可变镜像渲染和失败验证命令。
5. 增加 `helm lint`、`helm template` 或架构检查，覆盖默认 chart 不产生 `latest`，显式 `latest` 必须失败。
6. 回滚时只回退到上一版不可变 image ref；不得重新指向或复用 `latest`。

## Open Questions

- 无阻塞问题。实现阶段按 digest 优先、`sha-<commit>` tag 仅作为可读辅助引用执行。
