# Design

## Overview

本变更将用户服务 Atlas 配置收敛到 migration 目录：

```text
user-service/
  ent/
    schema/
    migrate/main.go
  migrations/
    atlas.hcl
    atlas.sum
    *.sql
  scripts/
    migrate-diff.sh
    migrate-validate.sh
    migrate-apply.sh
```

根目录 Makefile 仍是开发入口，迁移脚本仍从 `user-service/` 执行。脚本通过 Atlas `--config file://migrations/atlas.hcl` 显式指定配置文件，避免依赖调用目录中存在默认 `atlas.hcl`。

这是一项路径归一化变更，不改变 migration SQL、数据库 schema 或运行时业务行为。

## Location Decision

### Option A: Keep `user-service/atlas.hcl`

优点：

- 当前脚本天然可用，Atlas 默认会从工作目录读取 `atlas.hcl`。
- 变更范围最小，不需要移动文件或更新 Dockerfile 复制路径。

缺点：

- Atlas 配置和 migration directory 分离，开发者审查迁移资产时需要同时看 `user-service/atlas.hcl` 与 `user-service/migrations/`。
- Dockerfile、脚本和文档需要同时维护两个 migration 相关路径。
- 后续多服务或更细粒度迁移目录扩展时，根部配置更容易和服务运行时文件混在一起。

### Option B: Move to `user-service/migrations/atlas.hcl`

优点：

- SQL migration、`atlas.sum` 和 Atlas migration 配置同目录，边界更清晰。
- Docker image 只需复制完整 `migrations/` 即可携带迁移所需资产。
- CI 和文档可以把 `user-service/migrations/` 描述为完整 migration asset 目录。
- 脚本通过 `--config` 显式指定配置，减少对隐式工作目录约定的依赖。

缺点：

- 需要更新所有旧路径引用。
- Atlas config 内的相对路径需要继续以脚本工作目录为基准验证，避免 `ent://ent/schema` 或 `file://migrations` 被错误解析。

### Decision

采用 Option B：移动到 `user-service/migrations/atlas.hcl`。

关键约束是迁移脚本继续 `cd "$(dirname "$0")/.."` 到 `user-service/`，因此配置文件内容可以保持：

```hcl
src = "ent://ent/schema"
dir = "file://migrations"
```

脚本只负责显式传入配置路径：

```bash
atlas migrate diff "$1" --config file://migrations/atlas.hcl --env local
atlas migrate apply --config file://migrations/atlas.hcl --env deploy
```

`atlas migrate validate --dir file://migrations` 本身只校验 migration directory 和 `atlas.sum`，不需要读取 `atlas.hcl`。为了避免不必要耦合，validate 脚本可以继续使用显式 `--dir`。脚本注释需要说明 Atlas 配置位于 `migrations/atlas.hcl`，但 validate 的校验对象仍是 `migrations/`。

## Script Changes

### `migrate-diff.sh`

保留现有行为：

- 要求一个 migration name。
- 切换到 `user-service/`。
- 使用 `GOWORK=off`，避免 Atlas 读取 Ent schema source 时的 workspace 冲突。
- 生成后执行 `atlas migrate hash --dir file://migrations`。

调整：

```bash
GOWORK=off atlas migrate diff "$1" --config file://migrations/atlas.hcl --env local
```

注释更新为：

- Atlas 配置位于 `./migrations/atlas.hcl`。
- SQL 文件和 `atlas.sum` 位于 `./migrations/`。
- `ATLAS_DEV_URL` 覆盖的是 `migrations/atlas.hcl` 中的 dev URL variable。

### `migrate-validate.sh`

保留：

```bash
atlas migrate validate --dir file://migrations
```

原因：

- Validate 目标是 migration directory integrity。
- 该命令不需要加载 `local` 或 `deploy` env。
- 避免让纯 hash 校验依赖更多 Atlas config 解析。

注释更新为 `user-service/migrations/atlas.sum` 和 `user-service/migrations/`。

### `migrate-apply.sh`

保留：

- 切换到 `user-service/`。
- 检查 `DATABASE_URL`。
- 使用 `deploy` env。
- 不生成迁移。

调整：

```bash
atlas migrate apply --config file://migrations/atlas.hcl --env deploy
```

注释更新为 Atlas 读取 `./migrations/atlas.hcl` 并应用 `./migrations/` 中已提交 SQL。

### `ent/migrate/main.go`

该 helper 当前直接执行：

```go
atlas migrate diff <name> --env local
```

如果保留该 helper，必须同步为：

```go
atlas migrate diff <name> --config file://migrations/atlas.hcl --env local
```

否则直接调用 `go run ./ent/migrate/main.go diff <name>` 会因默认配置路径缺失而失败。Inspect path 使用 `atlas schema inspect`，不依赖 Atlas config，可不改。

## Docker Image

当前 Dockerfile 复制：

```dockerfile
COPY user-service/atlas.hcl /app/user-service/atlas.hcl
COPY user-service/migrations /app/user-service/migrations
```

迁移后改为只复制：

```dockerfile
COPY user-service/migrations /app/user-service/migrations
```

运行时脚本在 `/app/user-service` 下执行，并通过 `--config file://migrations/atlas.hcl` 找到配置。`entrypoint.sh` 不需要路径变更，因为它只调用 `/app/user-service/scripts/migrate-apply.sh`。

## CI

当前仓库只有 lint workflow，没有 migration validation workflow。实施时应新增轻量 CI 入口，例如 `.github/workflows/migrations.yml`：

- 触发条件与 lint 保持一致：pull request 和 main/master push。
- Checkout 代码。
- 安装或获取 Atlas CLI。
- 运行 `make migrate-validate`。

如果选择不新增独立 workflow，也至少应在现有 CI 中增加 `make migrate-validate` job。无论采用哪种形式，CI 命令必须通过脚本入口执行，避免 CI 自己硬编码旧路径。

## Documentation Updates

需要同步更新：

- `AGENTS.md`
  - Key Entry Points 中 Atlas 配置位置改为 `user-service/migrations/atlas.hcl`。
  - Repository Shape 或 Current Feature Areas 中将 `user-service/migrations/` 描述为包含 SQL、`atlas.sum` 和 Atlas config 的迁移资产目录。
- `docs/ARCHITECTURE.md`
  - Database Migrations 章节更新配置位置。
  - Module boundary 中可继续用 `user-service/migrations/` 表示服务内 migration 资产。
- `docs/DEVELOPMENT.md`
  - Directory layout 更新。
  - 迁移流程中所有 `atlas.hcl` 路径说明更新。
  - 手工 hash 和 validate 命令保持 `file://migrations`。
- `docs/TESTING.md`
  - 迁移验证说明确认仍运行 `./scripts/migrate-validate.sh` 或 `make migrate-validate`。
- `user-service/ent/README.md`
  - 如出现 Atlas 配置路径说明，需要同步更新。
- 脚本注释
  - `migrate-diff.sh`、`migrate-apply.sh` 中旧 `./atlas.hcl` 描述替换为 `./migrations/atlas.hcl`。

## Compatibility

保持不变：

- 根目录命令：
  - `make migrate-diff name=<name>`
  - `make migrate-validate`
  - `DATABASE_URL='<postgres-url>' make migrate-apply`
- 底层脚本名称和调用方式。
- Migration directory URL：`file://migrations`。
- Ent schema source：`ent://ent/schema`。
- Atlas env 名称：`local` 和 `deploy`。
- `ATLAS_DEV_URL` 覆盖行为。
- `DATABASE_URL` 覆盖行为。

不保持兼容的内部路径：

- 直接依赖 `user-service/atlas.hcl` 的外部脚本需要改为 `user-service/migrations/atlas.hcl`，或优先调用 repo 脚本入口。

## Verification Strategy

实施后运行：

```bash
rg -n "user-service/atlas\\.hcl|\\./atlas\\.hcl|atlas\\.hcl" AGENTS.md docs user-service .github Makefile
make migrate-validate
```

期望：

- 除变更记录中说明历史路径外，不再出现旧路径引用。
- `make migrate-validate` 通过。

如果本地 Atlas dev database 可用，额外运行：

```bash
make migrate-diff name=verify_atlas_location
git status --short user-service/migrations
```

若没有 schema diff，Atlas 不应生成新的 SQL；若生成测试迁移，应删除该验证产物并确认没有 SQL 语义变更进入提交。

如果有可用测试数据库，额外运行：

```bash
DATABASE_URL='<postgres-url>' make migrate-apply
```

生产环境不得作为本变更验证目标。
