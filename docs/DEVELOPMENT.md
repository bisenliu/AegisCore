# AegisCore 开发说明

## 1. 环境前提

- Go 1.26。
- `make`。
- `golangci-lint`。
- Docker，用于本地依赖、Compose 和 Testcontainers 场景。
- OpenSpec CLI，用于 `/opsx:*` 变更工作流。
- Atlas 相关本地脚本通过 `user-service/scripts/` 调用；容器化发布使用专用 Atlas/migration 镜像。

## 2. 查看命令

```bash
make help
make -C user-service help
```

## 3. 构建和运行

构建全部服务：

```bash
make build
```

运行 user-service：

```bash
make user-service-run
```

指定配置：

```bash
USER_SERVICE_CONFIG=/absolute/path/to/config.yaml make user-service-run
```

直接在模块内运行：

```bash
cd user-service
go run ./cmd serve --config ./configs/config.yaml
```

## 4. 测试和 lint

运行全部测试：

```bash
make test
```

运行全部 lint：

```bash
make lint
```

运行 user-service 架构边界检查：

```bash
make user-service-architecture-lint
```

运行完整验证：

```bash
make verify
```

`make verify` 会执行 lint、架构边界检查、测试、OpenAPI 生成，并用 `git diff --exit-code` 暴露生成物 drift。

## 5. Ent 和 migration

Ent schema 变化后生成代码：

```bash
make user-service-generate
```

生成 Atlas migration：

```bash
make user-service-migrate-diff name=<migration-name>
```

校验 migration：

```bash
make user-service-migrate-validate
```

应用 migration：

```bash
DATABASE_URL='<postgres-url>' make user-service-migrate-apply
```

普通 user-service 运行时镜像不包含 Atlas。容器化环境应先运行 `deployments/docker/user-service-migration.Dockerfile` 构建的 migration 镜像完成 `atlas migrate apply`，再启动服务镜像。

## 6. OpenAPI

生成 OpenAPI 3 文档：

```bash
make user-service-openapi-generate
```

生成物位于：

- `user-service/docs/openapi.go`
- `user-service/docs/openapi.json`
- `user-service/docs/openapi.yaml`

API 注解、路由、request 或 response 变化后必须重新生成并检查 diff。

## 7. RBAC 引导

初始化系统角色、权限和绑定：

```bash
make user-service-seed-rbac
```

创建或复用超级管理员：

```bash
ADMIN_PASSWORD='<password>' make user-service-create-super-admin
```

可选参数：

```bash
ADMIN_USERNAME=admin \
ADMIN_NICKNAME=Admin \
ADMIN_RESET_PASSWORD=true \
ADMIN_PASSWORD='<password>' \
make user-service-create-super-admin
```

## 8. 本地部署和观测

Compose 文档位于 `deployments/compose/README.md`。通用观测文档位于 `deployments/observability/README.md`。

生成 Compose Grafana dashboard：

```bash
make compose-dashboard-generate
```

检查 dashboard drift：

```bash
make compose-dashboard-check
```

构建 Docker 镜像：

```bash
docker build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-services .
docker build -f deployments/docker/user-service-migration.Dockerfile -t aegiscore-user-services-migration .
```

## 9. OPSX 初始化和变更

如果仓库缺少 `openspec/`，先初始化：

```bash
openspec init --tools none --force
```

创建变更：

```text
/opsx:propose <change-name>
```

实施变更：

```text
/opsx:apply <change-name>
```

归档变更：

```text
/opsx:archive <change-name>
```

OpenSpec 主规格、change artifacts 和 OPSX 相关文档正文必须使用简体中文。
