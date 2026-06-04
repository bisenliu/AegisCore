## Context

`user-services/internal/domain/user_status.go` 中的 `UserStatus.AllowedValues` 当前通过 `fmt.Sprint` 将固定枚举值转换为字符串。该路径属于 `shared-enum-contracts` 关联的用户域枚举契约，返回值稳定且只包含已知数字常量，因此使用通用格式化函数会带来不必要的接口装箱、格式化分派和 import 依赖。

本次变更同时要求在 `common/` 和 `user-services/` 范围内审查类似格式化用法。由于 `fmt.Sprintf` 在错误消息、日志内容、复合模板和多字段格式化中通常更清晰，优化不能以牺牲可读性或改变输出为代价。

## Goals / Non-Goals

**Goals:**

- 将 `UserStatus.AllowedValues` 中可静态确定的枚举允许值改为更直接的字符串实现。
- 搜索 Go workspace 内 `fmt.Sprint`、`fmt.Sprintf` 等格式化用法，识别仅用于简单类型转换或拼接的低效场景。
- 对性能收益明确、替换后可读性不下降的代码做最小修改。
- 在实现结果中说明每处修改原因、优化方式，以及保留未改场景的依据。

**Non-Goals:**

- 不改变 `UserStatus` 枚举值、校验逻辑、JSON/text 反序列化行为或 API 响应内容。
- 不迁移用户业务枚举到 `common`，也不新增跨服务枚举能力。
- 不改动 Ent 生成代码、数据库 schema、Atlas migration、HTTP 路由、错误码或配置结构。
- 不为低频错误消息或复杂格式化模板引入难读的手工拼接。

## Decisions

- `UserStatus.AllowedValues` 使用 `strconv.FormatInt(int64(<enum>), 10)` 基于枚举常量生成允许值，而不是 `fmt.Sprint` 或硬编码字符串字面量。这样避免 `fmt` 的通用格式化开销，同时保持枚举值变更时只有常量定义一个事实来源。
- 项目范围审查以语义为准，不机械替换所有 `fmt.Sprintf`。仅当格式化函数只是把单个基础类型转字符串，或拼接少量字符串且普通拼接更清晰时才替换；错误消息、日志、SQL/URL 模板、包含宽度/精度/转义语义的格式化保持不变。
- 删除因优化后不再使用的 `fmt` import，保持 Go 文件依赖最小化，并通过 `gofmt` 保持格式一致。
- 验证优先运行受影响模块测试；如全量测试受本地环境限制失败，结果中明确说明失败原因和已完成的验证范围。

## Risks / Trade-offs

- `[Risk]` 字符串字面量与枚举数字常量未来可能不同步。`→` 缓解方式：仅用于固定对外枚举契约；实现时检查是否已有测试覆盖 `AllowedValues`，必要时补充或说明验证方式。
- `[Risk]` 过度替换 `fmt.Sprintf` 可能降低错误消息和日志构造的可读性。`→` 缓解方式：保留格式化语义明显或替换收益不明确的场景，并在最终结果中列出判断依据。
- `[Risk]` 项目范围搜索可能命中生成代码或第三方代码。`→` 缓解方式：不手写修改 `user-services/ent/` 生成代码，也不修改 vendor/cache 类目录。
