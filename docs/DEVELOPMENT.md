# AegisCore 开发说明

## 1. 环境前提

- Go 1.26。
- `make`。
- `golangci-lint`。
- Docker，用于本地依赖、Compose 和 Testcontainers 场景。
- OpenSpec CLI，用于 `/opsx:*` 变更工作流。
- Atlas 相关本地脚本通过 `user-service/scripts/` 生成和校验 SQL；数据库变更执行由 DBA 工单或受控发布平台完成。

### 1.1 Go 工具链版本治理

Go 工具链使用固定版本声明和 Renovate 升级 PR/MR 治理。CI 合并门禁不得使用 `stable`、`latest` 或 `GOTOOLCHAIN=auto`，避免 lint、`go tool`、analyzer、生成物和测试行为随运行时间漂移。

当前固定声明位置：

- `go.work` 的 `go 1.x.y` 与 `toolchain go1.x.y`。
- 各 Go module 的 `go.mod` 中 `go 1.x.y`。
- GitHub Actions workflow 的 `GO_VERSION` 和 `GOTOOLCHAIN`。
- 未来 GitLab CI 的 `.gitlab-ci.yml` 或 `.gitlab/ci/*.yml` 中 `GO_VERSION` 和 `GOTOOLCHAIN`。

`go 1.x.y` 表示模块语言和语义基线以及最低 Go patch 版本；`go.work` 的 `toolchain go1.x.y`、`GO_VERSION` 和 `GOTOOLCHAIN` 表示实际使用的 Go patch 工具链。所有必需位置必须保持一致。Go module 的 `toolchain go1.x.y` 行由 Go/Renovate 工具维护，存在时必须与上述版本一致。

Go patch、minor 和 major 升级由 Renovate 根据仓库根目录 `renovate.json` 自动创建 PR/MR：

- patch 升级在必需检查通过后允许 Renovate 自动合并。
- minor 和 major 升级必须人工 review，确认 `make verify`、生成物 diff、lint/analyzer 行为和构建结果。
- GitHub Actions、GitLab CI include/component、Dockerfile 和 Go module 依赖同样由 Renovate 创建升级 PR/MR。

本地验证 Renovate 配置：

```bash
git add -N renovate.json
npx --yes --package renovate renovate --platform=local --onboarding=false --require-config=required --dry-run=lookup
git reset -- renovate.json
```

`RE2 not usable, falling back to RegExp` 是本地 `npx` 临时安装缺少可选正则加速依赖的警告，可忽略。查询 GitHub release 可能需要 `GITHUB_COM_TOKEN` 以避免 API 限流。

GitHub 使用 Renovate GitHub App 读取 `renovate.json`。迁移 GitLab 后继续保留同一份 `renovate.json`，通过 scheduled pipeline 运行 `renovate/renovate:latest`，并配置 `RENOVATE_TOKEN` 创建 MR。

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

配置文件使用严格 unknown-key 校验。共享核心只有 `app`、`server`、`log`、`observability`；user-service 在 `resources` 下声明 `cache_redis` 和 `primary_db`，并在 `auth.token_version_cache`、`rbac.user_role_cache` 下声明 feature cache。环境变量对应完整路径，例如 `AEGISCORE_SERVER_HTTP_PORT`、`AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_TIMEOUT` 和 `AEGISCORE_RESOURCES_POSTGRES_PRIMARY_DB_POOL_MAX_OPEN_CONNS`。

`serve` 由 CLI 在创建 App 前单次加载 service config，再通过 `bootstrap.AppOptions` 将该对象及其派生 runtime config 交给 composition root。`runtime.lifecycle.start_timeout` 和 `stop_timeout` 同时用于 App 顶层 Fx options 与 CLI 显式 Start/Stop context；当前手动生命周期的实际 deadline 由传给 `App.Start`/`App.Stop` 的 context 决定。`fx.StartTimeout` 不限制配置加载或 `fx.New` 中的同步 constructor/invoke，构造期 I/O 必须由自身边界治理。

本地时区用标准 `TZ=Asia/Shanghai`。日志只写 stdout/stderr。需要 trace 时设置 `AEGISCORE_OBSERVABILITY_TRACING_ENABLED=true` 和 OTLP endpoint；需要 pprof 时临时设置 `AEGISCORE_OBSERVABILITY_PPROF_ENABLED=true AEGISCORE_OBSERVABILITY_PPROF_ADDR=127.0.0.1:6060`，不要把独立诊断端口直接暴露到公网。

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

提交和执行 migration：

1. 运行 `make user-service-migrate-validate` 校验 SQL 目录和 `atlas.sum`。
2. 将 SQL migration 和 `atlas.sum` 提交到 Git。
3. 通过 DBA 工单或受控发布平台执行 SQL migration；如 SQL 包含 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`，确认目标库权限或 DBA 前置动作。

普通 user-service 运行时镜像不包含 Atlas。容器化环境应先确认数据库 SQL migration 已受控执行，再启动服务镜像。

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

一次性创建初始超级管理员：

```bash
ADMIN_BOOTSTRAP_PASSWORD='<temporary-password>' \
ADMIN_USERNAME='initial-admin' \
ADMIN_NICKNAME='Initial Administrator' \
make user-service-bootstrap-super-admin
```

参数约束：

- `ADMIN_USERNAME` 必填，无默认值；命令会 trim 后转为小写。
- `ADMIN_NICKNAME` 可选；trim 后为空时回退为归一化 username。
- `ADMIN_BOOTSTRAP_PASSWORD` 必填，Makefile 不在命令行展开或输出密码值。

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
docker buildx build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-service .
```

user-service 镜像使用 BuildKit manifest-first 构建：先复制 `go.work`、`go.work.sum` 和各 workspace module 的 `go.mod`/`go.sum` 准备依赖，再复制源码编译。构建阶段挂载 `/go/pkg/mod` 和 Go build cache，并使用 `CGO_ENABLED=0`、`-mod=readonly`、`-trimpath`、固定 builder/runtime digest 和显式 VCS metadata 参数。

运行时镜像基于固定 digest 的 `gcr.io/distroless/static-debian12:nonroot`，默认 UID/GID 为 `65532`，不包含 shell、`apk`、`wget`、`curl`、`grep` 或 Atlas。验证本地镜像内容：

```bash
IMAGE=aegiscore-user-service:latest make user-service-image-verify
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
