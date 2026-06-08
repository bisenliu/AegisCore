## Context

用户资料创建与查询当前保持 controller/service/repository 分层，HTTP 解析在 controller，业务编排在 service，Ent/PostgreSQL 访问在 `repository/postgres`。但用户资料 service 依赖的 `UserProfileRepository` 接口和 `CreateUserInput`、`ListUsersInput` 输入模型定义在根 `repository` 包中，导致 service 层必须感知 repository 包结构。Fx 组装还通过 `postgres.AsUserProfileRepository` 将聚合仓储强制转换为资料专用接口，说明依赖绑定逻辑泄露到了具体实现包。

本变更只修正内部接口所有权和依赖注入边界，不修改 HTTP API、响应信封、认证要求、Ent schema、Atlas migration、Redis/PostgreSQL 配置或日志契约。

## Goals / Non-Goals

**Goals:**

- 让用户资料 service 自己声明其消费的持久化端口和输入模型。
- 让 PostgreSQL 用户仓储实现适配 service 端口，同时继续实现认证相关 repository 端口。
- 删除 `postgres.AsUserProfileRepository` 胶水 provider，把具体实现到消费侧端口的绑定放到 Fx 组装层。
- 保持用户资料创建、按 ID 查询和列表查询行为完全兼容。

**Non-Goals:**

- 不新增用户资料 API。
- 不修改 controller 的请求绑定、响应输出或 Swagger 行为。
- 不修改 Ent schema、生成代码或数据库 migration。
- 不重构认证服务的凭证仓储和 token version 仓储边界，除非为了编译和 Fx 组装做最小适配。

## Decisions

1. 在 `internal/service` 声明用户资料持久化端口。

   `UserProfileStore` 表达 `userService` 实际消费的 `Create`、`GetByUserID`、`ListUsers` 方法，并与 `CreateUserInput`、`ListUsersInput` 一起放在 service 包中。这样 service 的业务编排不再依赖根 repository 包中的资料接口或输入模型。

   替代方案是在 `internal/domain` 中定义端口。该方案会让 domain 承担应用层用例输入模型，不符合当前仓库按 controller/service/repository 分层的组织方式。

2. PostgreSQL 实现导入 service 端口类型。

   `repository/postgres` 作为低层适配器可以依赖高层消费端口类型，方法签名接收 `service.CreateUserInput` 和 `service.ListUsersInput`。移除 service 对 repository 的依赖后不会形成 import cycle。

   替代方案是保留 repository 包输入 DTO 并只移动接口。该方案仍让 service 依赖 repository 包结构，不能彻底解决反向绑定。

3. 在 bootstrap 层完成 Fx 接口绑定。

   使用 `fx.Annotate` 和 `fx.As` 将 `postgres.NewUserRepository` 返回的具体实现同时暴露为 `service.UserProfileStore`、`repository.UserCredentialRepository` 和 `repository.UserTokenVersionRepository`，或在 bootstrap 中提供等价 provider。具体实现包不再导出 `AsUserProfileRepository`。

   替代方案是继续保留 `postgres.AsUserProfileRepository`。该方案会把依赖注入适配逻辑留在 PostgreSQL 实现包，无法清理冗余导出。

4. 保留 controller 依赖 `service.UserService`。

   严格按消费方声明接口可以进一步把 controller 需要的用例接口移到 controller 包，但本变更的主要耦合风险在 service 到 repository 边界。为控制变更范围，本次只修复 service 消费的持久化端口，后续可单独评估 controller 用例接口位置。

## Risks / Trade-offs

- Fx 多接口绑定配置错误可能导致启动期依赖解析失败 → 通过 `user-services` 全量测试覆盖 bootstrap 和 controller/service 构造路径。
- PostgreSQL 实现导入 service 包会让低层适配器知道应用层端口名称 → 这是依赖倒置的预期方向，且不会影响 Ent 查询职责仍位于 repository/postgres。
- 只修复 service 到 repository，不移动 controller 消费的 `UserService` 接口 → 保持本次变更最小，避免把 controller 测试和 Fx 绑定一次性扩大；该问题可后续独立处理。
- 规格只改变内部架构约束，不改变外部行为 → 验证重点放在 Go 编译、单元测试和现有 API 行为测试，不需要 migration 验证。
