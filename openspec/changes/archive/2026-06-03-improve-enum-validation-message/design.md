## Context

`request-validation` 在 `common/validation` 中注册共享 `enum` 自定义校验规则，目前 `Enum` 契约仅包含 `IsValid() bool`，因此校验失败翻译只能输出 `{字段名}不合法`。该消息能表达失败结果，但不能告诉调用方允许值。

用户状态等业务枚举位于 `user-services/internal/domain`，controller 通过请求 DTO 的 `validate:"enum"` 复用共享校验规则。为了避免在 controller 或 service 重复硬编码状态取值列表，允许值展示应作为共享校验翻译能力的一部分实现。

## Goals / Non-Goals

**Goals:**

- 在 enum 校验失败的字段级明细中输出更具体的中文消息：`{字段名}取值不合法，允许值为：{值1}、{值2}、{值3}`。
- 保持现有 `Enum` 接口兼容，未提供允许值列表的枚举仍能安全校验和安全降级。
- 支持值类型与指针类型 enum，nil 指针和配置错误不得 panic。
- 让用户状态枚举提供允许值列表，避免 controller/service 层重复实现。

**Non-Goals:**

- 不改变 HTTP status、业务错误码、响应信封结构或顶层校验失败 message。
- 不迁移用户业务枚举到 `common`，用户状态仍属于 `user-services` 域模型。
- 不修改 Ent schema、数据库 migration 或生成代码。

## Decisions

- 增加可选接口 `EnumValues`，包含 `AllowedValues() []string`。原因是允许值列表只用于增强错误消息，不应成为 `Enum` 的强制兼容要求；替代方案是直接扩展 `Enum`，但这会破坏所有现有 enum 实现。
- 翻译逻辑优先从 `validator.FieldError.Value()` 读取失败值，再解引用指针并判断是否实现 `EnumValues`。原因是翻译阶段需要基于具体字段类型生成消息；替代方案是在翻译模板中写死允许值，但不同 enum 类型的允许值不同，不适合共享规则。
- 允许值使用字符串列表并以中文顿号拼接。原因是响应消息需要面向 API 调用方可读，且用户请求参数通常以数字或字符串形式表达；替代方案是返回 `[]any`，但会增加格式化复杂度且无明显收益。
- 如果无法获取允许值列表，返回 `{字段名}取值不合法`。原因是 misconfigured enum、nil 指针或旧 enum 实现仍应给出稳定错误消息；替代方案是沿用 `{字段名}不合法`，但新文案可以统一表达字段取值问题。

## Risks / Trade-offs

- [Risk] enum 类型实现 `AllowedValues()` 返回顺序不稳定会导致错误消息和测试不稳定 → 要求实现返回固定顺序，并在测试中覆盖。
- [Risk] 指针 enum 为 nil 时无法通过实例获取允许值 → 降级为 `{字段名}取值不合法`，保持安全不 panic。
- [Risk] API 调用方如果断言完整 `errors[].message` 文案，增强消息会改变断言结果 → 保持错误码、HTTP status、字段名、规则名和顶层 message 不变，文案变化仅限字段级明细。
- [Risk] 用户业务枚举值被误迁移到 `common` → 只在 `common` 定义接口，不移动用户状态常量。
