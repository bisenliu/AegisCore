## 1. 镜像边界调整

- [x] 1.1 新增专用 migration 镜像构建入口，使其基于 `arigaio/atlas` 并复制 `user-service/migrations/` 与 `migrations/atlas.hcl`。
- [x] 1.2 修改 `deployments/docker/user-service.Dockerfile`，移除 Atlas stage、`COPY --from=atlas`、运行时 migration 文件复制，并确保 HTTP 运行时镜像仍能启动 user-service 与执行 RBAC CLI。
- [x] 1.3 调整 `user-service/scripts/entrypoint.sh` 和相关 migration 脚本边界，确保普通服务镜像不再因 `RUN_MIGRATIONS=true` 尝试执行 Atlas migration。
- [x] 1.4 优化 Dockerfile 中服务二进制权限设置，避免复制二进制后用单独 `RUN chmod` 层重复写入大文件。

## 2. 部署资产调整

- [x] 2.1 修改 `deployments/compose/docker-compose.yml`，让 `user-service-migrate` 使用专用 migration 镜像执行 `atlas migrate apply`，并保持 `user-service-migrate`、`rbac-seed`、`user-service` 启动顺序不变。
- [x] 2.2 修改 `deployments/k8s/user-services/migration-job.yaml`，让 migration Job 使用独立 Atlas/migration 镜像和 Atlas command，并继续通过 Secret 注入 `DATABASE_URL`。
- [x] 2.3 修改 Helm chart templates，使 migration Job 使用独立 migration image，RBAC seed Job 和 Deployment 继续使用 user-service image。
- [x] 2.4 修改 Helm `values.yaml` 和环境覆盖示例，新增 migration image 配置，并说明默认 tag 与 user-service release tag 的对齐规则。

## 3. 文档和规格同步

- [x] 3.1 更新 Compose、Kubernetes、Helm README，说明 migration Job 使用独立 Atlas/migration 镜像，普通 user-service 镜像不包含 Atlas。
- [x] 3.2 更新仓库 README、user-service README、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md` 中与 `RUN_MIGRATIONS`、migration apply 和发布顺序相关的说明。
- [x] 3.3 更新 `docs/opsx/CAPABILITY_MAP.md` 或相关 OPSX 文档中 delivery-operations 的说明，确保能力地图仍准确指向 Docker、Compose、Kubernetes、Helm 和 migration 资产。

## 4. 验证

- [x] 4.1 运行 `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services:verify .`，并通过容器内检查确认 `/usr/local/bin/atlas` 不存在。
- [x] 4.2 构建专用 migration 镜像，并通过容器内检查确认 `atlas` 可执行且 `user-service/migrations/` 与 `migrations/atlas.hcl` 存在。
- [x] 4.3 运行 `make user-service-migrate-validate`，确认已提交 migration 目录仍有效。
- [x] 4.4 运行 Compose 配置检查，确认 `user-service-migrate` 使用 migration 镜像且依赖顺序正确。
- [x] 4.5 运行 Helm 验证命令，包括 `helm lint deployments/helm/aegiscore-user-services` 和 `helm template`，并检查 migration Job、RBAC seed Job、Deployment 的镜像与 command。
- [x] 4.6 对 Kubernetes 原生清单运行 YAML/schema 或 server-side dry-run 检查，确认 migration Job 镜像、command、Secret 引用和安全上下文合法。
- [x] 4.7 运行 `make user-service-architecture-lint`，确认 OpenSpec 和中文文档规则通过。
- [x] 4.8 运行 `make lint`、相关 Go 测试和 `make verify`；如某项因本地依赖缺失无法运行，记录原因和替代验证结果。
- [x] 4.9 检查 `git diff --exit-code` 或审查 diff，确认没有未提交生成物 drift，并记录 user-service 镜像移除 Atlas 前后的大小对比。
