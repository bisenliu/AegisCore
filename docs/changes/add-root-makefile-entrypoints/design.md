# Design

## Overview

本变更在仓库根目录新增一个轻量 Makefile，作为开发命令的统一调度层。Makefile 不承载业务逻辑，也不复写已有 shell 脚本内部细节；它只负责：

- 在正确目录执行 Go 命令。
- 构建当前服务二进制。
- 聚合跨模块 test/lint。
- 将迁移与 Swagger 操作委托给 `user-service/scripts/` 下现有脚本。
- 通过 `make help` 暴露可发现的命令清单。

仓库根目录仍然是 Go workspace，不是业务 Go module。Makefile 命令需要显式进入 `common/` 或 `user-service/`，避免依赖调用者当前目录。

## Makefile Structure

Makefile 使用明确的 phony targets，并将目录定义为变量：

```makefile
COMMON_DIR := common
USER_SERVICE_DIR := user-service
USER_SERVICE_CONFIG ?= ./user-service/configs/config.yaml
USER_SERVICE_BIN ?= ./bin/user-service

.PHONY: help build build-user-service test test-common test-user-service lint \
        lint-common lint-user-service run-user-service generate migrate-diff \
        migrate-validate migrate-apply swagger-generate
```

建议的命令分组：

- Help：`help`
- Build：`build`、`build-user-service`
- Test：`test`、`test-common`、`test-user-service`
- Lint：`lint`、`lint-common`、`lint-user-service`
- Runtime：`run-user-service`
- Generation：`generate`、`swagger-generate`
- Migration：`migrate-diff`、`migrate-validate`、`migrate-apply`

`help` 可通过 `awk` 从 target 注释中生成列表，例如约定 `##` 后是说明文本。这让新增命令时只维护一处。

## Command Behavior

### Build

`make build` 调用当前唯一服务构建入口：

```bash
go build -o ./bin/user-service ./user-service/cmd
```

输出路径通过 `USER_SERVICE_BIN` 变量覆盖：

```bash
make build USER_SERVICE_BIN=./bin/aegiscore-user-service
```

根目录 `.gitignore` 已忽略 `bin/`，因此默认构建产物不会进入版本控制。

### Test

`make test` 依赖或顺序调用：

- `make test-common`
- `make test-user-service`

各单模块 target 分别执行：

```bash
cd common && go test ./...
cd user-service && go test ./...
```

这样满足“等价于分别测试 common 和 user-service”的验收要求，也保留失败即停止的 Make 默认行为。

### Lint

`make lint` 依赖或顺序调用：

- `make lint-common`
- `make lint-user-service`

各单模块 target 分别执行：

```bash
cd common && golangci-lint run ./...
cd user-service && golangci-lint run ./...
```

Makefile 不安装 `golangci-lint`，缺少工具时由命令自身失败，并由 `docs/DEVELOPMENT.md` 继续说明安装建议。

### Run

`make run-user-service` 从仓库根目录执行：

```bash
go run ./user-service/cmd serve --config ./user-service/configs/config.yaml
```

配置路径通过 `USER_SERVICE_CONFIG` 变量覆盖：

```bash
make run-user-service USER_SERVICE_CONFIG=./path/to/config.yaml
```

默认值保持现有文档中的仓库根目录运行方式。

### Generate

`make generate` 聚焦当前唯一需要生成 Go 代码的用户服务 Ent：

```bash
cd user-service && go generate ./ent
```

如果未来新增其他生成任务，可在不改变调用方入口的前提下扩展该 target，或新增更细粒度 target。

### Migrations

迁移 target 委托现有脚本：

```bash
cd user-service && ./scripts/migrate-diff.sh "$(name)"
cd user-service && ./scripts/migrate-validate.sh
cd user-service && ./scripts/migrate-apply.sh
```

`migrate-diff` 需要迁移名。Makefile 应在 `name` 为空时提前给出清晰错误，例如：

```makefile
test -n "$(name)" || (echo "Usage: make migrate-diff name=<migration-name>" >&2; exit 2)
```

`migrate-apply` 不应在 Makefile 中解析或改写 `DATABASE_URL`，继续让现有脚本校验该环境变量。

### Swagger

`make swagger-generate` 委托现有脚本：

```bash
cd user-service && ./scripts/swagger-generate.sh
```

生成工具版本继续由脚本中的 `go run github.com/swaggo/swag/cmd/swag@v1.16.6` 控制。

## Documentation Updates

`docs/DEVELOPMENT.md` 的常用命令表应改为根目录 Makefile 优先入口，同时保留执行目录或底层命令说明，方便排错。

`AGENTS.md` 的 Development Commands 应同步更新：

- 运行全部测试：`make test`
- 构建用户服务二进制：`make build`
- 运行全部 lint：`make lint`
- 运行用户服务：`make run-user-service`
- 生成 Ent 代码：`make generate`
- 生成迁移：`make migrate-diff name=<name>`
- 校验迁移：`make migrate-validate`
- 执行迁移：`DATABASE_URL='<postgres-url>' make migrate-apply`
- 生成 Swagger：`make swagger-generate`

文档不应暗示底层脚本被废弃；Makefile 是统一入口，现有脚本仍是迁移和 Swagger 的实现来源。

## Compatibility

本变更不影响运行时协议或模块边界：

- Go workspace 仍包含 `common` 与 `user-service`。
- 用户服务 feature 分层不变。
- HTTP API、响应 envelope、配置 key、数据库 schema、migration SQL 和 Redis key 语义不变。
- `user-service/scripts/` 下脚本继续可直接调用。
- CI/CD 可逐步选择使用 Makefile，也可保留当前底层命令。

## Verification Strategy

- `make help`：确认命令清单可读且包含新增入口。
- `make build`：确认用户服务二进制可构建到 `bin/user-service`。
- `make test`：确认按顺序测试 `common` 与 `user-service`。
- `make test-common` 与 `make test-user-service`：确认单模块入口可用。
- `make lint`：在本地具备 `golangci-lint` 时确认跨模块 lint 调度正确；如果工具缺失，记录未运行原因。
- `make migrate-diff`：无 `name` 时应给出用法并以非 0 退出。
- `make migrate-validate`：确认委托现有迁移校验脚本。
- `make swagger-generate`：确认委托现有 Swagger 生成脚本；如本地网络或工具下载失败，记录原因。
- 文档检查：确认 `AGENTS.md` 与 `docs/DEVELOPMENT.md` 命令保持一致。
