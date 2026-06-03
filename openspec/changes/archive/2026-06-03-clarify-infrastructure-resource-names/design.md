## Context

`common/infrastructure` 目前通过 `NameUserDB`、`NameCommonDB`、`NameCacheRedis` 集中维护用户服务运行时依赖名称。这些常量被 Redis/PostgreSQL provider 和用户服务 Ent wiring 复用，属于 `shared-infrastructure` 的命名约定，而不是单一配置加载逻辑。

当前文件名 `names.go` 过于宽泛，且常量组缺少用途说明。维护者可能误以为应将其合并进 `config.go`，但这会弱化常量跨 datastore 与 Ent wiring 的公共契约含义。

## Goals / Non-Goals

**Goals:**

- 将 `common/infrastructure/names.go` 重命名为 `common/infrastructure/resource_names.go`，让文件职责更明确。
- 为运行时资源名常量组补充中文注释，说明其用于 datastore 和 Ent 的 Fx wiring。
- 保持 `NameUserDB`、`NameCommonDB`、`NameCacheRedis` 的名称、值和包路径不变。

**Non-Goals:**

- 不合并到 `config.go`，不把资源名常量表达为配置加载实现细节。
- 不新增、删除或重命名任何运行时资源名常量。
- 不修改 YAML key、`AEGISCORE_` 环境变量、Fx name struct tag 或数据库连接行为。
- 不修改 Ent schema、生成代码、Atlas migration、HTTP API 或响应契约。

## Decisions

- 使用文件重命名而不是移动常量到其他实现文件。

  理由：`resource_names.go` 能表达常量职责，同时保持 `infrastructure` 包内集中维护，不与 `config.go`、`postgres.go` 或 `redis.go` 的具体实现职责混在一起。

- 注释使用中文，描述常量用途而不是重复字面值含义。

  理由：仓库规范和 OpenSpec 文档主要使用中文；注释应说明这些常量用于 datastore 与 Ent 的 Fx wiring，避免无价值的逐项解释。

- 保持常量值和 Go API 完全不变。

  理由：这些常量参与服务侧运行时依赖名称约定，变更值会影响配置路径、Fx wiring 或 repository 注入行为；本变更只做组织和说明。

## Risks / Trade-offs

- [Risk] 文件重命名后工具或文档仍引用旧路径。→ Mitigation：搜索 `common/infrastructure/names.go` 和资源名常量引用，必要时更新主动维护文档。
- [Risk] 注释过度承诺具体服务实现。→ Mitigation：注释只描述通用用途，不承诺新增 datastore、支付或其他未实现能力。
- [Risk] 误改常量值或 struct tag 会破坏运行时 wiring。→ Mitigation：实现时只重命名文件和添加注释，并运行 `go test ./...` 验证依赖图。
