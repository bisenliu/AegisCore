## Context

`user-services/internal/features/user/domain/user_status.go` 已经定义用户状态枚举、有效值校验、允许值列表、登录状态规则和 JSON/query 解析方法。`user-services/internal/features/user/api/request.go` 当前重复定义了同名状态类型、常量、`IsValid`、`AllowedValues`、`UnmarshalText` 和 `UnmarshalJSON`，导致 API DTO 与领域枚举存在双重维护成本。

该变更属于用户资料 capability 内部边界整理：`domain` 继续拥有用户状态规则，`api` 作为 HTTP DTO 边界复用同 capability 的领域枚举。变更不改变 controller/service/repository 职责，不涉及 Ent schema、Atlas migration、Redis、配置、Fx 启动依赖或 HTTP 响应契约。

## Goals / Non-Goals

**Goals:**

- 将 `userdomain.UserStatus` 作为用户状态枚举的唯一事实标准。
- 删除 `userapi.UserStatus` 重复定义和重复解析/校验方法。
- 让创建用户和列表用户请求 DTO 的 `Status` 字段直接使用 `*userdomain.UserStatus`。
- 保持 `validate:"omitempty,enum"` 可继续调用领域枚举的 `IsValid`/`AllowedValues`。
- 简化 controller 中状态字段映射，减少不必要的类型转换。

**Non-Goals:**

- 不改变用户状态允许值，仍为 `100`、`200`、`300`。
- 不改变 `status` 的 JSON/query 字段名、Swagger example、请求校验错误语义或响应字段。
- 不把用户状态枚举上移到 `common`，因为它仍是用户 capability 的业务枚举。
- 不调整 Ent schema 默认值、数据库字段类型或 migration。

## Decisions

- `api/request.go` 直接导入 `userdomain` 并使用 `*userdomain.UserStatus`。理由：用户状态是用户领域规则，DTO 使用领域枚举可以复用绑定和校验逻辑，同时保持依赖方向为 `api -> domain`，不会形成 `domain -> api` 反向依赖。替代方案是在 `api` 包做类型别名并保留 API 常量名，但用户明确要求彻底删除 API 层冗余定义，直接使用领域类型更符合唯一事实标准。
- 保留 `domain.UserStatus` 的 `UnmarshalText` 和 `UnmarshalJSON`。理由：Gin query/JSON 绑定和共享 enum 校验可以直接基于该类型工作，避免 transport 层重复解析状态值。替代方案是在 validation 层手动 parse `status`，但会把枚举解析逻辑从领域类型拆散。
- 删除或简化 `toCommandStatus`。理由：DTO status 和 command status 将是同一领域类型指针，controller 不再需要强制类型转换。替代方案是保留转换函数返回输入指针，可减少调用点变更，但函数本身将只剩空壳；实现时可根据可读性选择直接传递或保留极简 helper。
- 更新响应映射和测试中对 `userapi.UserStatus*` 的引用。理由：删除 API 状态常量后，响应 DTO 的 `Status` 字段若仍使用 `userapi.UserStatus` 会无法编译；测试应断言领域状态值，避免重新引入 API 层枚举。

## Risks / Trade-offs

- [Risk] `api` 包依赖 `domain` 可能被误解为跨层耦合。→ Mitigation：依赖发生在同一 user capability 内，方向为 DTO 复用领域值对象；`domain` 不依赖 `api`，不破坏领域层独立性。
- [Risk] 删除 `userapi.UserStatus*` 常量会导致测试或 Swagger 文档模型引用编译失败。→ Mitigation：实现时全量搜索 `userapi.UserStatus` 并替换为 `userdomain.UserStatus` 或领域常量。
- [Risk] enum 校验行为可能因类型变更回归。→ Mitigation：保留 domain 类型的 `IsValid`/`AllowedValues` 方法，并运行用户 feature 的 controller/validation 测试覆盖 create/list status 校验。
