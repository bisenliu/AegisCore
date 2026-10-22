## Context

`go.work`、`common`、`user-service` 和两个仓库工具 module 当前使用 Go 1.26.5；CI、Docker builder 和架构检查 fixture 也固定该版本。项目直接使用 `github.com/google/uuid`，其中 `common/runtime/id` 默认生成 UUID v7，user-service 的 feature、Ent schema 和生成代码共享同一 Go 类型。

Go 1.27 标准库新增 `uuid`。截至本次提案，Go 官方最新的 1.27 补丁版是 1.27.3。标准库 API 与旧包存在差异：`NewV7` 不返回 error，`Nil` 是函数，且 `uuid.UUID` 不实现 Ent `field.UUID` 要求的 `driver.Valuer`/`sql.Scanner`。因此需要同时处理源码类型、数据库映射和生成物。

## Goals / Non-Goals

**目标：**

- 让全部一方 Go module、workspace、CI 和 Docker builder 使用同一 Go 1.27.3 工具链。
- 将一方 UUID Go 类型统一到标准库 `uuid.UUID`，保持跨服务默认 UUID v7 以及已有 v4 使用场景。
- 保持 HTTP、JWT、Redis 和 PostgreSQL 中的 UUID 表示及数据库结构不变，并完成跨模块编译、测试和生成校验。

**非目标：**

- 不借升级改动业务授权、认证会话、ID 字符串格式、数据库表或发布顺序。
- 不因传递依赖仍使用旧包而强行改动第三方模块，不新增通用 UUID 兼容包。

## Decisions

### 统一固定 Go 1.27.3

更新 `go.work` 的 `go`/`toolchain`、四个 `go.mod`、`.github/workflows/ci.yml`、`.github/workflows/lint.yml`、两个 `deployments/docker/*Dockerfile` 的不可变 builder 镜像，以及 `user-service/scripts/architecture/lint-test.sh` 的版本 fixture。Docker 镜像在实施时核实对应 `1.27.3` tag 与 digest。`README.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和 `openspec/config.yaml` 同步版本说明；扩展 `user-service/scripts/architecture/lint.sh` 的版本一致性检查，使其覆盖当前漏检的 `tools/nacos-config-seed/go.mod`，并相应更新 fixture。最后扫描所有非历史、非生成的仓库文件，确认没有遗留的 Go 1.26 版本声明。

备选为仅将 `go` 指令升至 `1.27`、继续使用旧补丁镜像；这会使本地、CI 与容器实际编译版本不一致，因此不采用。

### UUID 直接使用标准库类型

`common/runtime/id` 保持现有导出函数签名和默认 v7 语义，返回标准库 `uuid.UUID`；标准库无 error 的生成 API 在现有返回 error 的函数中返回 `nil` error。user-service 的 feature 和测试直接导入 `uuid`；用 `uuid.Nil()`、`uuid.NewV4()`/`uuid.New()` 和基于 UUID 字节的版本断言替换不兼容 API，并分别保留原有 v4/v7 用途。`common` 不引入 user-service 专属适配，`internal/shared` 与 `internal/integration` 也不承载迁移 shim。

Go 1.27 将 `uuid` 列为标准库包；非生成源码中的 `uuid` 导入与其他标准库包放在同一组。现有 `goimports` formatter 只用 `local-prefixes` 区分仓库模块与第三方包，且会保留已有导入块的空行；迁移时清除旧第三方导入留下的空行。`Prefix(...)` 是 GCI 的独立自定义分组语法，不用于标准库分组。本次不增加另一套导入排序检查，Ent 生成物仍由生成命令维护。

备选为保留第三方包并仅升级 Go，或引入跨模块类型别名；两者都不能完成标准库类型迁移，且会掩盖编译期类型不兼容。

### Ent 映射留在 user-service 持久化边界

Ent 的 `field.UUID` 要求 Go 类型实现 `driver.Valuer`，不能直接接收标准库 `uuid.UUID`。在 `user-service/internal/persistence/ent/schema/` 使用支持自定义 Go 类型和 `ValueScanner` 的字段定义，将标准库 UUID 在数据库边界转换为字符串；PostgreSQL schema type 继续显式为 `uuid`，SQLite 测试读写也保持可用。重复字段可共用 schema-local 定义；不得把数据库适配放进 `common/runtime/id`，也不得手工编辑 `internal/persistence/ent/` 下的生成代码。运行 `make user-service-generate` 并审查生成 diff；再运行 Atlas diff/validate，若确认无数据库结构变化，不提交空 migration。

当前 `golang.org/x/tools v0.47.0` 无法解码 Go 1.27 的 export data v5；Ent 生成前将 user-service 和 common 的该间接依赖更新到首个支持 v5 的 `v0.50.0`，保证 workspace 与独立 module 的生成命令均可运行，随后统一运行完整生成命令。现有 `swag v1.16.6` 在 `--parseInternal` 下会解析 Go 1.27 标准库源码并遇到不支持的语法；OpenAPI 脚本继续显式扫描服务目录和依赖模型，但不要求扫描标准库源码，生成后比较文档 diff。

备选为给标准库类型添加方法，Go 不允许给外部包类型定义方法；另一个备选是保留旧包作为 Ent 字段类型，这会让 feature 与 Ent 生成类型分裂并扩散转换代码。

## Risks / Trade-offs

- [风险] Ent 生成类型或扫描转换仍保留旧包，或 PostgreSQL UUID 列意外变成文本列。→ 使用生成代码检查、SQLite/真实 PostgreSQL 的 UUID 读写测试和 Atlas schema diff；任何结构 diff 都先分析原因。
- [风险] 标准库与旧包在随机生成、零值、解析边界或 JSON 编码上有细节差异。→ 覆盖 v7 版本位、现有 v4 用途、合法/非法 UUID、字符串和 JSON 表示，以及 JWT/Redis 使用路径。
- [风险] Go 1.27 的工具、代码生成器或固定 Docker 镜像 digest 不兼容。→ 在固定版本环境运行生成、架构 lint、`make lint`、`make verify` 和镜像构建检查，并审查生成物变化。
- [权衡] `common/runtime/id` 的 error 返回签名继续存在，但标准库正常生成路径不返回 error。→ 保留调用方 API 稳定性，本次不同时重构所有错误处理路径。

## Migration Plan

1. 确认开发与 CI 可获取 Go 1.27.3 和相应不可变 Docker builder 镜像；统一版本配置。
2. 迁移 `common` 与 user-service 的 UUID 导入及调用；调整 Ent schema 并仅通过生成命令更新 Ent 代码，运行相关单测和数据库读写测试。
3. 运行 Atlas diff/validate、OpenAPI 生成检查与架构 lint，确认 SQL、OpenAPI 和对外字符串契约无意外变化；同步文档、依赖元数据和规格。
4. 暂存本次预期变更后执行 `make lint` 和 `make verify`，再按现有发布顺序构建、扫描并交付镜像。

回滚采用上一版 Go 1.26.5 构建配置、源码与已验证镜像整体回退。预期没有 SQL migration，也不需要数据回滚；若实施时发现数据库结构必须改变，需先修订本 change 的设计与迁移计划。

## Open Questions

无；实施时以当前官方发布页复核 1.27 补丁版本和 builder 镜像 digest。
