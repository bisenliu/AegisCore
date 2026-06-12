# Normalize deployments layout

## What

整理 `deployments/` 目录，使 Docker、Compose、Kubernetes 和 Helm 部署资产与长期架构文档中的最终结构一致。

本变更建议把当前位于 `user-service/Dockerfile` 的用户服务镜像构建资产迁移到 `deployments/docker/`，并补齐或整理 `deployments/compose/`、`deployments/k8s/`、`deployments/helm/` 下的部署入口说明与路径引用。实现后，部署资产目录应表达清晰边界：

- `deployments/docker/` 放 Dockerfile 或统一构建资产。
- `deployments/compose/` 放本地依赖或本地服务启动配置。
- `deployments/k8s/` 放 Kubernetes YAML。
- `deployments/helm/` 放 Helm chart。

同时更新仓库中引用 `user-service/Dockerfile`、Docker build command、Compose/K8s/Helm 中 user-service 路径和容器 entrypoint/CMD 的说明，保证 Docker build context 仍以仓库根目录为准，镜像内 `/app/user-service` 路径、entrypoint 和 service config 路径仍正确。

## Why

当前长期规则已经声明 `deployments/` 是 Docker、Compose、Kubernetes 和 Helm 部署资产的边界，但实际用户服务 Dockerfile 仍在 `user-service/` 目录下。这样会让服务代码目录同时承载应用代码和部署构建资产，后续新增 Compose、K8s 或 Helm 文件时也容易出现同一部署能力散落多处的情况。

将 Dockerfile 和部署入口集中到 `deployments/` 可以让仓库结构更直观：`user-service/` 只维护服务代码、配置样例、脚本和迁移资产；`deployments/` 维护如何构建和部署这些资产。这样也便于文档说明本地启动方式，避免开发者在 `user-service/` 与 `deployments/` 之间猜测哪个路径是当前入口。

## Scope

包括：

- 移动或重建用户服务 Dockerfile 到 `deployments/docker/`，例如 `deployments/docker/user-service.Dockerfile`。
- 更新 Docker build 命令，明确从仓库根目录执行：
  - `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`
- 保持 Docker build context 为仓库根目录，使 Dockerfile 仍可复制 `go.work`、`common/` 和 `user-service/`。
- 保持镜像内运行路径和 entrypoint 语义：
  - binary 位于 `/app/user-service/bin/user-services`。
  - config 位于 `/app/user-service/configs/config.yaml`。
  - migrations 位于 `/app/user-service/migrations/`。
  - scripts 位于 `/app/user-service/scripts/`。
  - entrypoint 为 `/app/user-service/scripts/entrypoint.sh`。
- 更新 `user-service/scripts/entrypoint.sh` 注释中的 Dockerfile/CMD 示例，使二进制名与当前 Dockerfile 一致。
- 将本地依赖启动或服务启动 Compose 文件放在 `deployments/compose/`；如果当前没有可用 Compose 文件，至少提供 README 说明该目录用途和预期入口。
- 将 Kubernetes YAML 放在 `deployments/k8s/`；如果当前没有可用 YAML，至少提供 README 说明目录边界和后续放置规则。
- 将 Helm chart 放在 `deployments/helm/`；如果当前没有完整 chart，至少提供 README 说明目录边界和后续放置规则。
- 更新 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 中关于部署资产位置、本地启动方式和 Docker build command 的说明。
- 更新 CI、Makefile 或脚本中如存在的 Dockerfile 路径引用。
- 检查历史 `user-services` 目录引用，确保不会把服务目录、容器路径或 image tag 混淆。

不包括：

- 不改变应用运行时配置语义。
- 不改变 `aegiscore-user-services` 作为运行时 service name、CLI name、image tag 示例或配置样例的语义。
- 不改变 HTTP API、JWT、Redis/PostgreSQL named resources、migration SQL 或 Ent schema。
- 不引入新云平台依赖。
- 不新增真实外部服务 client、broker、eventbus、outbox 或后台 worker。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- 用户服务 Dockerfile 位于 `deployments/docker/`，仓库当前路径不再依赖 `user-service/Dockerfile` 作为主入口。
- Docker build command 使用仓库根目录作为 context，并能解析 `common/` 与 `user-service/`：
  - `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`
- Dockerfile 中所有 `COPY user-service/...`、`COPY common ...`、`WORKDIR`、entrypoint 和 CMD 路径与当前 `user-service` 目录一致。
- 镜像内 entrypoint 仍调用 `/app/user-service/scripts/entrypoint.sh`，服务启动命令仍使用 `/app/user-service/bin/user-services serve --config /app/user-service/configs/config.yaml`。
- `entrypoint.sh` 注释中的 CMD 示例与实际二进制路径一致。
- `deployments/compose/`、`deployments/k8s/` 和 `deployments/helm/` 均有明确入口文件或 README，说明该目录存放什么、如何被本地启动或部署流程使用。
- 文档说明本地启动方式，包括本地依赖启动入口、Docker build 命令，以及运行用户服务的方式。
- 长期规则文档中 `deployments/` 的 Docker、Compose、K8s、Helm 边界与实际目录一致。
- 仓库中不再出现当前规则文档或脚本引用 `user-service/Dockerfile` 作为主入口；历史 change 记录中的旧路径可保留为历史上下文。
- 没有新增云平台特定依赖，也没有修改应用运行时配置语义。
