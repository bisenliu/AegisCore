## Context

`common/runtime/config` 是跨服务共享的配置加载 primitive，当前 `Load` 通过 Viper 读取 YAML 和环境变量后，使用 `mapstructure` decode hook 将字符串转换为 `time.Duration`、slice 等配置类型。代码中仍直接导入旧版 `github.com/mitchellh/mapstructure`，但 Go module 依赖图中已存在 `github.com/go-viper/mapstructure/v2`，这会导致配置解析依赖与 Viper 生态当前推荐版本不一致。

本变更属于 `shared-platform-primitives`，主要影响 `common/runtime/config`、Go module 依赖、配置测试和相关说明。`user-service` 会通过 workspace 继续消费 `common` 的配置加载能力，但不需要新增服务内配置适配层。

## Goals / Non-Goals

**Goals:**

- 将所有 `mapstructure` 导入统一迁移到 `github.com/go-viper/mapstructure/v2`。
- 按 v2 API 和标准行为调整 decode hook 组合、配置解析调用和测试预期。
- 移除旧版 `github.com/mitchellh/mapstructure` 的依赖残留，不保留兼容层或旧行为开关。
- 用配置加载测试覆盖 duration、slice、环境变量覆盖和错误路径，证明迁移后行为稳定。
- 同步更新涉及配置解析依赖或验证命令的文档和 OpenSpec delta。

**Non-Goals:**

- 不改变 HTTP API、OpenAPI、数据库 schema、Atlas migration、部署清单或观测资产。
- 不重构 Viper 作为配置来源，也不引入新的配置文件格式。
- 不新增 user-service feature-local 配置 loader 或业务配置兼容适配器。
- 不为旧版 `mapstructure` decode 行为提供双写、fallback、alias 或构建标签兼容路径。

## Decisions

1. 使用 `github.com/go-viper/mapstructure/v2` 作为唯一 `mapstructure` 导入路径。

   选择原因：v2 是 Go module 语义版本化路径，能与当前 Viper 生态保持一致，并避免继续依赖旧仓库路径。备选方案是保留 `github.com/mitchellh/mapstructure`，但这会延续旧依赖并违背本次迁移目标。

2. 保持配置加载能力归属在 `common/runtime/config`，不向 `user-service/internal/shared` 或 feature 包下沉。

   选择原因：配置 loader 是跨服务 runtime primitive，现有主规格已要求服务优先复用 `common/runtime`。备选方案是在 user-service 内新增适配层，但会复制共享配置语义并污染业务边界。

3. 直接采用 v2 标准 decode 行为，并用测试锁定当前项目需要的显式行为。

   选择原因：用户明确要求不保留旧版本或兼容性代码；测试应验证项目依赖的 duration、slice、env override 和 validation 行为，而不是断言旧版内部细节。备选方案是实现自定义 decode hook 兼容旧差异，但会形成长期兼容负担。

4. 通过 `go mod tidy` 清理模块依赖，而不是手工编辑所有间接依赖项。

   选择原因：Go 工具链能准确判断 `common` 与 `user-service` 的真实导入链。备选方案是手工删除旧依赖，但容易误删当前工具或测试仍需要的间接模块。

## Risks / Trade-offs

- v2 decode 行为与旧版存在细节差异导致配置解析测试失败或启动配置预期变化 -> 以 v2 标准行为为准调整测试和调用方预期，不增加兼容层。
- `user-service` module 可能仅通过 `common` 间接消费 v2，tidy 后依赖标记发生变化 -> 分别在 `common` 和 `user-service` 目录运行 `GOWORK=off go mod tidy`，检查 diff 是否仅包含本次依赖迁移相关内容。
- 配置测试覆盖不足导致环境变量覆盖或 slice 解析回归 -> 新增或调整 `common/runtime/config` 测试，覆盖文件值、环境变量覆盖、duration、slice 和 validation 失败路径。
- 文档或规格遗漏导致实施边界不清 -> 在 delta spec 和 tasks 中明确该变更不影响 API、DB、OpenAPI、部署和观测资产。

## Migration Plan

1. 搜索所有 `mapstructure` 导入和调用点，确认迁移范围。
2. 将 Go 源码导入路径切换到 `github.com/go-viper/mapstructure/v2`，按 v2 API 调整 decode hook 使用。
3. 更新配置加载测试，验证 v2 下项目依赖的解析行为。
4. 在 `common` 和必要服务模块中运行 `GOWORK=off go mod tidy`，移除旧版依赖残留。
5. 运行相关 package 测试，再运行 `make lint` 和 `make verify`。

回滚策略：如迁移后验证失败且无法在当前变更内修正，应回滚本 change 的源码、测试和依赖文件改动；不提供运行时开关回退到旧版 `mapstructure`。

## Open Questions

无待决问题。迁移范围以仓库内所有 `mapstructure` 导入、Go module 依赖和配置加载测试为准。
