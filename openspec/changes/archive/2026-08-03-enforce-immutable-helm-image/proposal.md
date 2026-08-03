## Why

当前 Helm chart 的生产基线默认渲染 `aegiscore-user-service:latest` 且显式设置 `imagePullPolicy: IfNotPresent`，在多节点 Kubernetes 集群中会因为节点本地缓存状态不同导致同一 release 运行不同镜像内容。生产发布前必须将 Helm 发布契约收敛为不可变镜像引用，确保 Deployment、RBAC seed Job、扫描和回滚都指向同一可审查工件。

## What Changes

- **BREAKING**: Helm 生产发布不再允许默认或显式使用 `latest` tag，必须由 CI/CD 传入不可变镜像引用。
- **BREAKING**: Helm chart 不再以 `Chart.appVersion: latest` 或空 `image.tag` 作为生产镜像 fallback。
- 将 user-service Helm chart 的镜像契约调整为单一不可变 image ref，Deployment 与 RBAC seed Job 共享同一引用。
- CI/CD 构建、扫描、SBOM、镜像身份记录和 Helm 发布必须复用同一个已推送镜像 digest 或 `sha-<commit>` tag。
- 增加 Helm 渲染或 lint 检查，阻止生产 chart 默认值、环境 values 或渲染结果包含 `:latest`。
- 更新 Helm README、交付规格和验证任务，明确发布顺序、回滚边界和不可变镜像要求。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: 补强 Helm/Kubernetes 生产发布镜像不可变要求，禁止 `latest` 进入生产渲染和发布路径。

## Impact

- 影响 `deployments/helm/aegiscore-user-service/values.yaml`、`Chart.yaml`、模板 helper、Deployment 模板和 RBAC seed Job 模板的镜像字段契约。
- 影响 `.github/workflows/ci.yml` 或后续生产发布 workflow 的镜像构建、推送、digest 解析、扫描、SBOM 和 Helm 发布参数传递。
- 影响 Helm README、Kubernetes/Helm 发布说明和相关架构检查或渲染验证命令。
- 不影响业务 HTTP API、OpenAPI、Ent schema、Atlas SQL migration、Nacos 配置 schema 或运行时代码逻辑。
- 安全影响：消除浮动 tag 与 `IfNotPresent` 组合导致的版本漂移，提升供应链可追踪性和回滚确定性。
