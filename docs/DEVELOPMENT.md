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

运行前需要准备 Nacos，并设置来源环境变量。以下示例复用 Compose 暴露到主机的 PostgreSQL、Redis、Nacos 和 Jaeger，并选择主机配置对应的 Namespace：

```bash
export AEGISCORE_SERVICE=user-service
export AEGISCORE_NACOS_ADDR=127.0.0.1:8848
export AEGISCORE_NACOS_NAMESPACE=loca-host
export AEGISCORE_NACOS_GROUP=AEGISCORE
make user-service-run
```

直接在模块内运行：

```bash
cd user-service
go run ./cmd serve
```

配置从 Nacos 按默认顺序加载 `base.yaml`、`resources.yaml`、`${AEGISCORE_SERVICE}.yaml`。仓库在 `deployments/nacos/local-host/` 和 `local-docker/` 中分别保存三份完整配置：`loca-host` 使用宿主机映射端口，`loca-docker` 使用 Compose DNS 和容器端口。环境变量只选择 Nacos 来源，不覆盖业务字段。完整加载顺序固定为：`source -> documents -> deep merge -> raw digest -> strict decode（defaults 初值、raw 覆盖、unknown key 拒绝）-> normalize -> validate -> effective encode -> redact/render`。raw digest 只描述合并后的原始来源，不受默认值或 effective encode 变化影响；`config render` 编码最终生效配置后再脱敏敏感字段。

`common/runtime/config` 只拥有业务中立的 document/source contract、YAML deep merge、raw digest、strict decode、effective encode、redact 和 render 原语；`common/runtime/config/nacos` 是独立 adapter，拥有 Nacos 环境变量、HTTP client、认证、timeout、failover 和文档读取。user-service 的 `internal/config.DefaultConfig()` 集中组装完整默认初值，并由显式 normalize 和 validate 完成服务级处理；composition root 再派生 `AuthSettings`、`RBACSettings`、`EntSettings` 和 `ResourceSettings`，auth、permission/RBAC、Ent 与具名资源 provider 不读取完整根配置。共享核心配置只有 `app`、`runtime`、`server`、`log`、`observability`；user-service 在 `resources` 下声明 `cache_redis` 和 `primary_db`，并在 `auth.token_version_cache`、`rbac.user_role_cache` 下声明 feature cache。

配置读取直接使用 Nacos 3.x 的 v3 Client HTTP API，只需要访问 `8848`。`AEGISCORE_NACOS_ADDR` 支持逗号分隔的多个地址并按顺序 failover；地址可以包含自定义 context path。启用 Nacos client auth 时必须同时设置 `AEGISCORE_NACOS_USERNAME` 和 `AEGISCORE_NACOS_PASSWORD`，客户端登录一次后复用 access token。

`AEGISCORE_NACOS_NAMESPACE` 必须填写 namespace ID，不是控制台显示名。通过控制台创建 namespace 时，如果选择自动生成 ID，应从 namespace 列表复制实际 ID；本地 Compose 初始化工具会把 ID 和显示名分别固定为 `loca-host`、`loca-docker`。版本化配置统一维护在 `deployments/nacos/`，Nacos 控制台中的内容是发布结果，不是第二配置来源。

本地 Nacos Dockerfile 参考官方最新部署说明，使用 `nacos/nacos-server:latest`，在镜像内固定 standalone、`FUNCTION_MODE=microservice` 和 `docker-startup.sh` 入口，只加载 Config 与 Naming；Compose 不提供或覆盖 Nacos 启动命令，仅通过 v3 Admin API 初始化 namespace 和配置。该功能模式要求 Nacos 3.2.2 或更高版本。浮动 tag 适合本地验证最新版兼容性；生产部署仍应在验证后固定具体版本或镜像 digest。

本地 `docker run` 示例和 Compose 同时关闭 Client、Admin API、Console API 三类鉴权。Nacos 3.x 的三个鉴权开关相互独立，只设置 `NACOS_AUTH_ENABLE=false` 不会关闭控制台鉴权；生产环境不得沿用本地关闭鉴权的配置。

`serve` 由 CLI 在创建 App 前单次加载 service config，再通过 `bootstrap.AppOptions` 将该对象及其派生 runtime config 交给 composition root。`runtime.lifecycle.start_timeout` 和 `stop_timeout` 同时用于 App 顶层 Fx options 与 CLI 显式 Start/Stop context；当前手动生命周期的实际 deadline 由传给 `App.Start`/`App.Stop` 的 context 决定。`fx.StartTimeout` 不限制配置加载或 `fx.New` 中的同步 constructor/invoke，构造期 I/O 必须由自身边界治理。

本地时区由 `runtime.timezone` 控制。日志只写 stdout/stderr。需要 trace 或 pprof 时修改 Nacos 中对应 dataId；不要把独立诊断端口直接暴露到公网。可用 `go run ./cmd config sources`、`config validate` 和 `config render` 诊断配置来源和合成结果，`render` 默认脱敏敏感字段。

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
3. 通过 DBA 工单或受控发布平台执行 SQL migration；`users.nickname` substring 模糊查询统一使用 `pg_trgm` 提供的 GIN `gin_trgm_ops` 索引，不保留普通索引、无扩展 fallback 或双索引兼容分支，执行前必须确认目标库创建 `pg_trgm` 所需的权限或 DBA 前置动作。

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

ID 约束：

- 普通运行时业务实体继续使用 `common/runtime/id.NewUUID()` 生成 UUID v7。
- 系统内置 RBAC 角色、权限和 bootstrap 用户 ID 由 `user-service/internal/shared/rbacbaseline/ids.go` 中的手写固化 UUID 字符串常量定义。
- 从基础框架初始化全新项目时可以写入新的系统 ID 常量；已有项目重命名时不得默认修改、重算或复用这些 ID，也不得修改既有数据库中的 RBAC 数据。

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

RBAC policy watcher 的正式配置位于 `rbac.policy_watcher`，包含 `check_interval`、`subscribe_timeout`、`max_staleness` 和 `retry_backoff.initial|max`。发布时先部署能够使用默认值且暴露新 watcher 指标的二进制，确认全部副本正常后再发布包含这些键的 Nacos 文档；Prometheus rules、通用 Grafana dashboard 及其 Compose 生成副本随同一变更发布。由于配置使用 strict decode 且不保留旧接口、旧指标或配置别名，回滚必须先从 Nacos 移除整个 `rbac.policy_watcher` 配置块，再回滚旧二进制和旧观测资产。

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
