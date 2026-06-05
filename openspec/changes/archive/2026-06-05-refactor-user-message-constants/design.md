## Context

用户服务当前通过 `user-services/internal/errmsg/messages.go` 集中保存可复用的中文错误提示文案。该文件只包含字符串常量，但包名 `errmsg` 容易被理解为 error 实例或 error 构造相关代码；常量名统一带 `Msg` 前缀后，调用侧形成 `errmsg.MsgUserNotFound` 这类重复表达，不符合 Go 包名与导出标识符组合时避免 stuttering 的惯例。

该包位于 `user-services/internal`，属于服务内展示文案组织，不属于 `common` 的跨服务响应契约实现，也不改变 controller/service/repository 分层职责。变更需要保持 `common/contract/response.Envelope`、HTTP status、业务错误码和错误映射不变。

## Goals / Non-Goals

**Goals:**

- 将展示文案包命名为 `messages`，准确表达“客户端展示文案常量”的职责。
- 将导出常量从 `Msg*` 改为无冗余前缀的领域名，例如 `UserNotFound`。
- 优化现有中文提示文案，使其语气更统一、专业、适合直接展示给最终用户。
- 同步更新用户服务内所有 import、引用和测试，保证 Go 编译与响应契约测试通过。

**Non-Goals:**

- 不新增错误码、不修改 HTTP status、不改变响应信封 JSON 字段。
- 不把服务特定文案上移到 `common`，除非未来出现跨服务复用需求。
- 不引入 i18n、多语言、文案模板系统或动态配置能力。
- 不修改 Ent schema、数据库 migration、Redis/PostgreSQL 配置或运行时启动流程。

## Decisions

- 使用包名 `messages`，不使用 `msg`。
  - 原因：`messages` 可读性更强，准确表达包中内容是面向用户或客户端的文案集合；`msg` 虽更短，但缩写降低可发现性，也不利于未来在同包内区分不同文案类别。
  - 备选：`msg`。优点是极简；缺点是语义略弱，团队成员需要额外上下文判断其职责。
- 移动目录到 `user-services/internal/messages`，保持文件名 `messages.go`。
  - 原因：Go import path 由目录决定，仅修改 `package` 声明不足以消除调用侧 `errmsg` 限定名；目录与包名同步后调用形态才会自然变为 `messages.UserNotFound`。
  - 备选：保留目录 `errmsg` 但包名写作 `messages`。该方案会造成 import path 与包名不一致，不利于维护。
- 去除所有导出常量的 `Msg` 前缀。
  - 原因：调用侧已经通过包名表达“这是 message”，常量保留 `Msg` 会形成重复命名。推荐形态为 `messages.InvalidUsername`、`messages.UserNotFound`。
  - 备选：只改包名不改常量名。该方案迁移成本较低，但保留了 `messages.MsgUserNotFound` 的 stuttering。
- 文案优化遵循“语义不变、表达更清晰”的边界。
  - 原因：文案是外部可观察响应的一部分，但本次目标不是重新设计业务错误分类。实现应避免新增业务含义，例如不要把“用户不存在”改成“账号已注销或不存在”。
  - 推荐示例：`用户名不能为空` 改为 `请输入用户名`；`用户ID格式不正确` 改为 `用户ID格式不正确，请检查后重试`；`用户名或密码错误` 改为 `用户名或密码不正确，请检查后重试`；`登录会话无效` 改为 `登录状态无效或已过期，请重新登录`。

## Risks / Trade-offs

- [Risk] 包目录重命名会导致未同步的 import 或限定名编译失败。→ Mitigation：使用仓库范围搜索 `internal/errmsg`、`errmsg.` 和 `Msg*`，同步更新后在 `user-services` 模块运行 `go test ./...`。
- [Risk] 文案变更可能影响断言精确字符串的测试或客户端快照。→ Mitigation：同步更新仓库内测试；在 proposal/spec 中明确响应结构、状态码和业务码不变，仅 `message` 文本优化。
- [Risk] 过度优化文案可能改变业务语义。→ Mitigation：逐条建立旧值到新值映射，只做语气和可读性优化，不引入额外状态或内部细节。
