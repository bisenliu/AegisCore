# Design

## Overview

本变更把部署资产集中到 `deployments/`，并保留用户服务运行时路径不变：

```text
deployments/
  docker/
    user-service.Dockerfile
  compose/
    README.md
    ...
  k8s/
    README.md
    ...
  helm/
    README.md
    aegiscore-user-services/
      ...

user-service/
  configs/
  migrations/
  scripts/
  cmd/
  internal/
```

Dockerfile 的文件位置会变化，但 build context 不变化。用户仍从仓库根目录构建镜像，Dockerfile 仍复制 workspace、`common/` 和 `user-service/`：

```bash
docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .
```

这样可以避免因为 Dockerfile 移入 `deployments/docker/` 而让 `COPY ../../user-service` 这类相对路径进入 Dockerfile。Dockerfile 中所有 `COPY` 源路径继续基于仓库根目录 context。

## Target Ownership

### `deployments/docker/`

该目录拥有镜像构建资产。当前只需要用户服务 Dockerfile：

```text
deployments/docker/user-service.Dockerfile
```

原 `user-service/Dockerfile` 应通过 `git mv` 或等效 tracked move 移入该目录。迁移后可选择删除旧文件，避免两个 Dockerfile 漂移。如果确实需要过渡 alias，必须只作为短期兼容文件，并在 tasks 中说明移除时机；本变更默认不保留 alias。

Dockerfile 顶部注释应更新为：

```dockerfile
# Build from the repository root:
#   docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .
```

### `deployments/compose/`

该目录拥有本地依赖启动配置或本地服务组合启动配置。现有 `deployments/compose/postgres/initdb/` 是依赖初始化骨架，但当前没有可运行 Compose file。

实施时有两种可接受路径：

- 若新增 Compose file：放在 `deployments/compose/compose.yaml` 或清晰命名的同级文件，并引用本变更后的 Dockerfile 路径。
- 若暂不新增 Compose file：添加 `deployments/compose/README.md`，说明该目录用于本地 PostgreSQL/Redis 等依赖启动配置，当前用户服务仍可通过 `make run-user-service` 或 Docker build/run 手工启动。

如新增 Compose file，必须保持配置语义不变，不发明新的 app config key，不引入新云平台依赖。

### `deployments/k8s/`

该目录拥有 Kubernetes YAML。现有 `deployments/k8s/user-services/` 是空目录骨架。

如果当前没有实际 YAML，实施应添加 `deployments/k8s/README.md` 或 feature-local README，说明后续 K8s manifest 的放置规则。不得为了填充目录而新增未验证的 Deployment、Service、Ingress 或 Secret 示例。

### `deployments/helm/`

该目录拥有 Helm chart。现有 `deployments/helm/aegiscore-user-services/templates/` 是空目录骨架。

如果当前没有完整 chart，实施应添加 `deployments/helm/README.md` 或 chart-level README，说明 chart 边界和后续 values/template 放置规则。不得为了填充目录而新增未验证的 chart templates 或云厂商资源。

## Docker Build Details

Dockerfile 迁移前位于：

```text
user-service/Dockerfile
```

迁移后位于：

```text
deployments/docker/user-service.Dockerfile
```

Build context 保持仓库根目录，因此 Dockerfile 中这些路径保持有效：

```dockerfile
COPY go.work go.work.sum ./
COPY common ./common
COPY user-service ./user-service
COPY user-service/configs /app/user-service/configs
COPY user-service/migrations /app/user-service/migrations
COPY user-service/scripts /app/user-service/scripts
```

构建阶段继续从 `user-service/` 编译：

```dockerfile
WORKDIR /src/user-service
RUN go build -o /out/user-services ./cmd
```

运行阶段继续使用当前容器路径：

```dockerfile
COPY --from=builder /out/user-services /app/user-service/bin/user-services
ENTRYPOINT ["/app/user-service/scripts/entrypoint.sh"]
CMD ["/app/user-service/bin/user-services", "serve", "--config", "/app/user-service/configs/config.yaml"]
```

不要把容器内路径改为 `/app/deployments` 或 `/app/user-services`。`deployments/` 是仓库部署资产位置，不是运行镜像内应用路径。

## Entrypoint Alignment

`user-service/scripts/entrypoint.sh` 当前注释中 CMD 示例使用：

```text
/app/user-service/bin/user-service
```

但 Dockerfile 实际生成并调用：

```text
/app/user-service/bin/user-services
```

实施时应修正注释，使示例与实际 CMD 一致。脚本行为不变：

- `RUN_MIGRATIONS` 未设置或为 `true` 时执行 `/app/user-service/scripts/migrate-apply.sh`。
- 其他值跳过迁移。
- 最后 `exec "$@"` 启动 CMD。

## Documentation Updates

### `AGENTS.md`

更新 Repository Shape 或 Key Entry Points：

- `deployments/docker/` 是用户服务 Dockerfile 或统一构建资产位置。
- `deployments/compose/` 是本地依赖/Compose 启动配置位置。
- `deployments/k8s/` 是 Kubernetes YAML 位置。
- `deployments/helm/` 是 Helm chart 位置。

如果列出 Dockerfile 入口，应使用 `deployments/docker/user-service.Dockerfile`。

### `docs/ARCHITECTURE.md`

在 `deployments` module boundary 或 Infrastructure 相关段落中明确上述目录职责。不要把 Dockerfile 描述为 `user-service/` 内资产。

### `docs/DEVELOPMENT.md`

新增或更新本地运行说明：

- 本地直接运行服务：
  - `make run-user-service`
- 构建容器镜像：
  - `docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .`
- 本地依赖：
  - 使用 `deployments/compose/` 下的 Compose 配置；如果当前仅有 README，则说明 PostgreSQL/Redis 仍需开发者按配置样例准备。

如果新增 Compose file，也要说明启动命令，例如：

```bash
docker compose -f deployments/compose/compose.yaml up -d
```

### Historical Change Records

历史 `docs/changes/*` 中对 `user-service/Dockerfile` 的引用可以保留为历史上下文。验收扫描应聚焦当前代码、脚本和长期规则文档；如果历史记录被当前文档直接引用为现行规范，则需要修正。

## Verification Strategy

实施后运行：

```bash
test -f deployments/docker/user-service.Dockerfile
test ! -f user-service/Dockerfile
rg -n "user-service/Dockerfile|deployments/docker|docker build" AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md Makefile .github user-service deployments
```

期望：

- 当前规则文档和脚本不再把 `user-service/Dockerfile` 作为主入口。
- Docker build command 指向 `deployments/docker/user-service.Dockerfile`。
- `entrypoint.sh` 注释与实际 CMD 一致。

如果本地 Docker 可用，运行：

```bash
docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .
```

可选检查镜像内关键文件：

```bash
docker run --rm --entrypoint sh aegiscore-user-services -c 'test -x /app/user-service/bin/user-services && test -x /app/user-service/scripts/entrypoint.sh && test -f /app/user-service/configs/config.yaml && test -f /app/user-service/migrations/atlas.hcl'
```

如果新增 Compose file，运行：

```bash
docker compose -f deployments/compose/compose.yaml config
```

如果新增 Helm chart，运行：

```bash
helm lint deployments/helm/aegiscore-user-services
```

只有在实际新增 YAML/chart 内容时才需要对应 lint；README-only 边界说明不要求这些工具存在。

## Risks

Risk: Dockerfile 移动后误以为 build context 也变成 `deployments/docker/`，导致 `COPY common` 或 `COPY user-service` 失败。

Mitigation: 文档和 Dockerfile 顶部注释明确必须从仓库根目录 build，并在验收中运行 docker build 或至少静态检查 copy path。

Risk: 修路径时误改运行时 service name `aegiscore-user-services`。

Mitigation: 只迁移部署资产路径，不修改 app name、CLI name、JWT issuer、日志 filename、Redis key prefix 或 image tag 示例语义。

Risk: 为了填充 K8s/Helm/Compose 目录新增未验证资源，反而引入错误默认配置。

Mitigation: 没有真实可验证部署资产时使用 README 声明边界；真实 manifest/chart 另行设计或在本变更中补齐 lint 与验证。
