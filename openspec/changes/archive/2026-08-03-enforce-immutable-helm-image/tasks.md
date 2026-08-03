## 1. Helm 镜像契约

- [x] 1.1 修改 `deployments/helm/aegiscore-user-service/values.yaml`，移除生产默认 `image.tag: latest`，改为必填不可变 `image.ref`，保留 `image.pullPolicy: IfNotPresent` 仅用于不可变镜像引用。
- [x] 1.2 修改 `deployments/helm/aegiscore-user-service/Chart.yaml`，移除 `appVersion: latest` fallback，避免 chart 元数据继续表达浮动应用版本。
- [x] 1.3 修改 Helm image helper，使 Deployment 与 RBAC seed Job 直接使用同一个 `image.ref`，并在缺失、空值或 `latest` tag 时通过 Helm `fail` 阻止渲染。
- [x] 1.4 更新 `deployments/helm/aegiscore-user-service/values-local.yaml`，本地示例也显式使用非 `latest` 的测试用不可变 tag 或 digest。

## 2. CI/CD 与发布文档

- [x] 2.1 更新 `.github/workflows/ci.yml` 或生产发布 workflow 的 user-service 镜像链路，使构建、push、镜像身份记录、漏洞扫描和 SBOM 复用同一个 `sha-<commit>` tag 或 digest 工件。
- [x] 2.2 更新 Helm 发布命令示例，使 `helm upgrade --install` 必须通过 `--set-string image.ref=<immutable-ref>` 或等价环境 values 传入不可变镜像引用。
- [x] 2.3 更新 `deployments/helm/aegiscore-user-service/README.md`，说明生产禁止 `latest`、Deployment 与 RBAC seed Job 使用同一不可变镜像、回滚必须使用历史不可变 image ref。
- [x] 2.4 搜索 Helm/Kubernetes 生产发布文档和示例，移除 user-service 生产路径中的 `latest` 镜像示例，仅保留明确标记为本地 Compose 或测试用途的浮动 tag。

## 3. 验证与防回归

- [x] 3.1 增加或更新 Helm/架构检查，验证缺少 `image.ref` 时 `helm lint` 或 `helm template` 失败。
- [x] 3.2 增加或更新 Helm/架构检查，验证显式传入 `:latest` 时 `helm lint` 或 `helm template` 失败。
- [x] 3.3 增加或更新 Helm/架构检查，验证传入 digest 或 `sha-<commit>` tag 时 Deployment 与 RBAC seed Job 渲染为完全相同的 image ref，且渲染结果不包含 `:latest`。
- [x] 3.4 运行 `helm lint deployments/helm/aegiscore-user-service --set-string image.ref=<immutable-test-ref>` 和对应 `helm template` 验证命令，确认安全上下文、探针、资源、RBAC seed Job、NetworkPolicy 和无 migration Job 行为未漂移。
- [x] 3.5 运行 `make user-service-architecture-lint`，确认部署资产结构化检查通过。

## 4. 收尾验证

- [x] 4.1 运行 OpenSpec 校验命令，确认 `enforce-immutable-helm-image` change 的 proposal、design、tasks 和 `delivery-operations` spec delta 可解析。
- [x] 4.2 检查 `git diff`，确认没有修改 Go 业务代码、HTTP API、OpenAPI 生成物、Ent schema 或 Atlas migration。
- [x] 4.3 将本次预期代码、文档和 OpenSpec 变更加到暂存区。
- [x] 4.4 运行 `make lint`，未通过时修复后重跑。
- [x] 4.5 运行 `make verify`，未通过时修复后重跑，确保最终无未暂存预期 diff 阻塞验证。
