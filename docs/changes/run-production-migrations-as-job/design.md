# Design

## Overview

本变更把用户服务 migration 执行从“应用容器默认启动动作”调整为“发布流程显式阶段”。

目标发布顺序：

```text
build image
  -> validate migrations
  -> run migration job
  -> run RBAC seed job when needed
  -> start or roll out user-service replicas
```

容器镜像仍包含 Atlas CLI、`user-service/migrations/` 和 `migrate-apply.sh`。差异在于普通 HTTP service 容器默认只启动服务，不自动迁移。需要迁移时，由 CI/CD、Kubernetes Job、Helm pre-upgrade Job 或 Compose one-shot service 显式运行同一个 `migrate-apply.sh`。

## Current Behavior

`user-service/scripts/entrypoint.sh` 当前逻辑是：

```sh
if [ "${RUN_MIGRATIONS:-true}" = "true" ]; then
  /app/user-service/scripts/migrate-apply.sh
fi

exec "$@"
```

这意味着未设置 `RUN_MIGRATIONS` 的任何容器启动都会先执行 Atlas migration。

Compose 已经显式设置：

```yaml
RUN_MIGRATIONS: "false"
```

并提供独立服务：

```yaml
user-service-migrate:
  entrypoint: ["/app/user-service/scripts/migrate-apply.sh"]
```

因此本地 Compose 已经接近目标模式，主要风险来自镜像默认行为和生产部署约定仍允许普通服务副本默认迁移。

## Target Behavior

### Entrypoint Default

将入口脚本改为：

```sh
if [ "${RUN_MIGRATIONS:-false}" = "true" ]; then
  /app/user-service/scripts/migrate-apply.sh
fi
```

语义：

- `RUN_MIGRATIONS` 未设置：不执行迁移。
- `RUN_MIGRATIONS=false` 或其他非 `true` 值：不执行迁移。
- `RUN_MIGRATIONS=true`：启动服务前执行迁移。

保留显式开关的原因：

- 兼容极简部署平台或临时环境。
- 便于手工验证镜像中 migration assets 是否完整。
- 避免在没有生产部署模板的阶段一次性移除已有脚本能力。

但文档必须明确：生产多副本滚动发布不应依赖该开关。

### Migration Job Command

独立 migration job 使用同一镜像和同一脚本：

```text
/app/user-service/scripts/migrate-apply.sh
```

必需环境变量：

```text
DATABASE_URL=postgres://...
```

Job 不需要启动 HTTP server，也不需要读取业务 YAML 配置。它只需要镜像内的 Atlas CLI、`migrations/atlas.hcl`、`migrations/*.sql`、`atlas.sum` 和目标数据库 URL。

### RBAC Seed Ordering

RBAC seed 仍是独立运维入口，不随 `serve` 自动执行。发布顺序应保持：

```text
migration job -> rbac seed job -> user-service rollout
```

原因：

- RBAC seed 依赖数据库 schema 已经存在。
- seed 不是在线 policy refresh 机制。
- 服务副本启动后如果 seed 才执行，授权基线可能短暂缺失。

### Compose

Compose 继续保留 `user-service-migrate` one-shot service。该服务模拟 release migration job：

- 等待 PostgreSQL healthy。
- 执行 `/app/user-service/scripts/migrate-apply.sh`。
- 成功后 `rbac-seed` 才运行。
- 成功后 `user-service` 才启动。

`user-service` 服务继续显式设置 `RUN_MIGRATIONS=false`。即使 entrypoint 默认值改变，该显式配置也能表达本地编排意图。

### Kubernetes

当前 `deployments/k8s/` 没有可直接运行的清单。实施时应优先更新 README，明确未来新增清单时的要求：

- 新增单独 Job 运行 migration。
- Deployment 不设置 `RUN_MIGRATIONS=true`。
- Deployment 只在 migration Job 成功后 rollout。
- Job 通过 Secret 或部署系统注入 `DATABASE_URL`。
- Job 使用和应用相同的镜像版本，避免 migration SQL 与应用版本不一致。

如果实现阶段选择新增示例 YAML，应放在 `deployments/k8s/user-services/`，并保持无云厂商绑定。示例可以使用 placeholder Secret 名称，但不得提交真实凭据。

### Helm

当前 `deployments/helm/aegiscore-user-services/` 没有可运行 chart。实施时应优先更新 README，明确未来 chart 设计：

- 提供 `migrationJob.enabled` 一类开关。
- Job 使用 chart 当前 image tag。
- Job command 为 `/app/user-service/scripts/migrate-apply.sh`。
- `DATABASE_URL` 从 Secret 或 external secret reference 注入。
- Deployment 默认不启用 startup migration。

是否使用 Helm hook 应由未来 chart 变更单独设计。本变更不默认引入 hook，因为 hook 的失败保留、重试、回滚和 GitOps 行为需要结合发布平台确定。

## Documentation Model

需要统一替换容易误导的表述：

- 原有：“迁移应由 CI/CD release job 或容器 entrypoint 在 HTTP runtime 启动前执行。”
- 目标：“生产迁移应由 CI/CD release job 或独立 migration Job 在 HTTP runtime rollout 前执行；容器 entrypoint 迁移仅作为显式兼容开关。”

文档应区分三类场景：

- 开发生成：`make migrate-diff name=<name>`。
- 校验：`make migrate-validate`。
- 发布执行：`DATABASE_URL=... make migrate-apply` 或部署平台中的 migration Job。

## Failure Semantics

Migration Job 失败：

- 发布流程停止。
- 不启动新版本 HTTP 副本。
- 保留失败日志，排查 Atlas 输出、数据库连接、SQL 兼容性和锁等待。

HTTP service 启动失败：

- 不应自动重新执行 migration，除非部署者显式设置 `RUN_MIGRATIONS=true`。
- 排查范围集中在配置、依赖、探针、RBAC seed 或应用 runtime。

多副本场景：

- Atlas lock 仍能保护显式并发迁移调用。
- 生产编排不应让每个普通服务副本都参与竞争 migration lock。
- Job 应以单实例 one-shot 方式执行。

## Compatibility

保持不变：

- `make migrate-apply` 仍调用 `user-service/scripts/migrate-apply.sh`。
- `migrate-apply.sh` 仍要求 `DATABASE_URL`。
- 容器镜像仍包含 Atlas CLI 和 migration assets。
- Compose 仍能自动完成本地 migration、RBAC seed 和服务启动顺序。
- 设置 `RUN_MIGRATIONS=true` 仍可恢复入口脚本迁移行为。

行为变化：

- 未设置 `RUN_MIGRATIONS` 的服务容器不再迁移数据库。
- 依赖镜像默认启动自动迁移的部署必须改为显式 migration Job，或临时设置 `RUN_MIGRATIONS=true` 作为兼容过渡。

## Verification Strategy

实施后运行：

```bash
sh -n user-service/scripts/entrypoint.sh
make migrate-validate
rg -n "RUN_MIGRATIONS|容器 entrypoint|migration job|migrate-apply" user-service deployments docs AGENTS.md
```

期望：

- `entrypoint.sh` shell syntax 通过。
- `make migrate-validate` 通过。
- 文档不再表达“生产默认通过服务容器入口迁移”。
- Compose 中 `user-service` 保持 `RUN_MIGRATIONS=false`。
- Compose 中 `user-service-migrate` 仍调用 `/app/user-service/scripts/migrate-apply.sh`。

如果有本地 Docker/Compose 环境，额外运行：

```bash
docker compose -f deployments/compose/docker-compose.yml config
```

有可用本地依赖时，可运行：

```bash
docker compose -f deployments/compose/docker-compose.yml up --build user-service-migrate rbac-seed user-service
```

生产数据库不得作为本变更验证目标。
