## Context

`require-explicit-config` 已经实现了“移除默认配置、主要配置缺失就失败”的行为，但当前实现把字段存在性、取值范围和环境变量绑定 key 混在 `common/config/loader.go` 的手工逻辑里。`common/config/config.go` 的结构体定义没有表达哪些字段必填、哪些字段可选，导致配置契约与实际校验分散。

项目已经依赖 `go-playground/validator/v10`，用户 controller 也使用该库进行请求校验。配置加载可以复用同一校验库，在 struct tag 上声明 `required`、`omitempty`、`min`、`max`、`gt` 等规则，使基础字段校验更直接。

## Goals / Non-Goals

**Goals:**

- 在 `common/config` 结构体字段上标注必填/可选和基础范围规则。
- 使用 `validator.New(validator.WithRequiredStructEnabled())` 对解码后的 `Config` 执行结构体验证。
- 删除或大幅收敛 `validateConfig` 中逐字段 if 判断，让 loader 只负责读取、环境绑定、反序列化和统一错误包装。
- 保持没有 Viper 默认值的策略：主要配置仍必须来自 YAML 或 `AEGISCORE_` 环境变量。
- 保留环境变量显式绑定所需的 key 列表，但避免把它当作主要字段校验逻辑的唯一来源。
- 明确可选字段：例如 Redis username/password、PostgreSQL password、HTTP trusted proxies 可以为空或省略；必填字段仍用 `validate:"required"` 或等价规则声明。

**Non-Goals:**

- 不恢复任何代码默认值或 Viper `SetDefault`。
- 不改变配置文件查找路径、CLI `--config` 行为或环境变量前缀规则。
- 不改变 `require-explicit-config` 已确定的外部行为：主要配置缺失仍阻止启动。
- 不新增业务能力、HTTP API、Ent schema 或生成代码变更。

## Decisions

1. 将配置字段约束放在 struct tag 上。

   `config.go` 是配置契约的中心位置。将校验规则放到字段旁边，可以让读代码的人直接看到字段是否必填、是否允许空值、数值范围是什么。这样比在 loader 中维护一长串 if 更容易随字段变化同步维护。

   备选方案是保留手写 validator helper，但这仍会让规则与字段定义分离，无法解决用户指出的“手动校验做法有点蠢”的问题。

2. 使用 validator 负责值语义，Viper `IsSet` 只用于必要的“显式配置”检测。

   validator 能判断空字符串、端口范围、正数等值语义，但 Go 零值无法天然区分“未配置”与“显式配置为零”。对于需要允许零值的字段，例如 `redis.db=0`，如果必须确认显式提供，仍需要 `v.IsSet("redis.db")` 或等价机制。因此实现应把 `IsSet` 保留为少量显式存在性检查，而非继续维护全部字段的范围校验。

   备选方案是把 `redis.db` 改为指针以区分未设置和零值，但会污染业务使用端并增加 nil 处理成本，不符合最小改动原则。

3. 对可选字段使用 `omitempty` 或无 `required` 标签。

   Redis username/password、PostgreSQL password、HTTP trusted proxies 等字段可能合法为空。它们不应被 `required` 拒绝，但若团队希望它们必须显式出现，应用少量 `IsSet` 检查表达“出现但可为空”。

4. 保持错误可诊断。

   validator 默认错误信息偏 Go 字段名。实现可以保持现有测试关注关键字段名，必要时增加一个小型格式化 helper，把 `ValidationErrors` 转换为 `mapstructure`/配置路径风格的错误，例如 `database.postgres.host is required`。

## Risks / Trade-offs

- [Risk] validator tag 无法表达“字段必须出现但允许零值/空值”。→ Mitigation：仅对这类字段保留小范围 `IsSet` 显式存在性检查。
- [Risk] 错误信息从手写字符串变为 validator 默认格式，影响测试和排障体验。→ Mitigation：添加或保留错误格式化 helper，让错误包含配置路径。
- [Risk] struct tag 过多影响可读性。→ Mitigation：只标注基础必填和范围规则，不把复杂业务逻辑塞进 tag。
- [Risk] 环境变量绑定 key 列表仍需维护。→ Mitigation：将其定位为 Viper 环境绑定清单，避免同时承担字段校验职责。
