## Why

当前共享 `enum` 校验失败时只返回 `{字段名}不合法`，调用方无法直接知道允许取值，排查请求参数问题时需要额外查阅代码或文档。

## What Changes

- 优化 `common/validation` 中 `enum` 自定义规则的失败消息，能够在枚举类型提供允许值列表时返回 `{字段名}取值不合法，允许值为：{值1}、{值2}、{值3}`。
- 为共享 enum 校验契约增加可选的允许值列表接口，保持现有仅实现 `IsValid() bool` 的枚举类型可继续使用。
- 为用户状态等已知 enum 类型补充允许值列表实现，使校验错误明细包含可读取值范围。
- 在无法获取允许值列表时保留安全降级文案，避免 misconfigured enum 或 nil 指针导致 panic。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `request-validation`: enum 校验失败的字段级中文错误消息需要在可获取允许值列表时包含允许值。

## Impact

- 影响代码：`common/validation/` 的 enum 翻译逻辑与枚举接口定义，以及实现共享 enum 校验的业务枚举类型。
- API 影响：HTTP status、业务错误码、响应信封结构和顶层 message 保持不变；`errors[].message` 的 enum 字段明细会变得更具体。
- 兼容性：现有仅实现 `IsValid() bool` 的枚举类型不需要立即修改，但不会自动输出允许值列表；实现新接口后可获得增强错误文案。
