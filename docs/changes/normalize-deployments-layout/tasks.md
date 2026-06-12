# Tasks

## Implementation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只整理部署资产位置和路径引用。
- [x] 使用 `git mv user-service/Dockerfile deployments/docker/user-service.Dockerfile` 移动用户服务 Dockerfile。
- [x] 更新 `deployments/docker/user-service.Dockerfile` 顶部 build command 注释为 `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`。
- [x] 检查 Dockerfile 的 build context 假设，确认 `COPY go.work go.work.sum ./`、`COPY common ./common`、`COPY user-service ./user-service` 仍以仓库根目录为 context。
- [x] 检查 Dockerfile 的运行镜像路径，确认 binary、configs、migrations、scripts、entrypoint 和 CMD 仍使用 `/app/user-service/...`。
- [x] 删除或避免保留 `user-service/Dockerfile` 兼容副本，防止两个 Dockerfile 漂移。
- [x] 更新 `user-service/scripts/entrypoint.sh` 注释，把 CMD 示例修正为 `/app/user-service/bin/user-services serve --config /app/user-service/configs/config.yaml`。
- [x] 检查 `deployments/compose/` 当前内容；如没有可运行 Compose file，添加 README 说明本地依赖启动配置归属和当前本地启动方式。
- [x] 如新增 Compose file，放在 `deployments/compose/`，并引用 `deployments/docker/user-service.Dockerfile`；不得改变应用配置 key 或引入新云平台依赖。
- [x] 检查 `deployments/k8s/` 当前内容；如没有实际 YAML，添加 README 说明 Kubernetes manifest 放置规则。
- [x] 检查 `deployments/helm/` 当前内容；如没有完整 chart，添加 README 或 chart-level README 说明 Helm chart 放置规则。
- [x] 更新 `AGENTS.md` 中 deployments 结构说明和关键入口，把 Dockerfile 位置改为 `deployments/docker/user-service.Dockerfile`。
- [x] 更新 `docs/ARCHITECTURE.md` 中 deployments module boundary，使 Docker、Compose、K8s、Helm 目录职责与实际目录一致。
- [x] 更新 `docs/DEVELOPMENT.md`，补充或修正本地启动方式、Docker build command、Compose 入口说明。
- [x] 更新 Makefile、CI workflow、脚本或 README 中所有当前路径引用，避免继续把 `user-service/Dockerfile` 作为主入口。
- [x] 确认没有修改 HTTP API、运行时配置语义、JWT/Redis/PostgreSQL 配置、Ent schema、migration SQL 或业务逻辑。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 `test -f deployments/docker/user-service.Dockerfile`。
- [x] 运行 `test ! -f user-service/Dockerfile`，确认旧 Dockerfile 主入口已移除。
- [x] 运行 `rg -n "user-service/Dockerfile" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md Makefile .github user-service deployments`，确认当前规则文档、脚本和部署资产不再引用旧主入口。
- [x] 运行 `rg -n "docker build|deployments/docker|entrypoint|/app/user-service/bin/user-service" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md user-service deployments Makefile .github`，确认 Docker build command 和 entrypoint/CMD 路径一致。
- [x] 在 Docker 可用时运行 `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`。
- [x] 在 Docker 可用且镜像构建成功后运行镜像文件检查：

```bash
docker run --rm --entrypoint sh aegiscore-user-services -c 'test -x /app/user-service/bin/user-services && test -x /app/user-service/scripts/entrypoint.sh && test -f /app/user-service/configs/config.yaml && test -f /app/user-service/migrations/atlas.hcl'
```

- [x] 如新增 Compose file，运行 `docker compose -f deployments/compose/compose.yaml config`。本次未新增 Compose file，仅新增 README 边界说明。
- [x] 如新增 Helm chart templates 或 values，运行 `helm lint deployments/helm/aegiscore-user-services`。本次未新增 chart templates 或 values，仅新增 README 边界说明。
- [x] 检查 `git diff -- deployments user-service/scripts AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md Makefile .github`，确认只有部署布局、路径引用和说明变更。

## Review Notes

- [x] 确认 `aegiscore-user-services` 作为运行时 service name、CLI name、配置样例和 image tag 示例没有被误改。
- [x] 确认 Docker build context 仍是仓库根目录，而不是 `deployments/docker/`。
- [x] 确认 `deployments/compose/`、`deployments/k8s/`、`deployments/helm/` 没有新增未经验证的云平台或生产环境默认资源。
- [x] 确认文档已经说明本地直接运行和容器镜像构建方式。
