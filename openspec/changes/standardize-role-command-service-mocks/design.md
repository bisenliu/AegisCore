## Context

`user-service/internal/features/role/application/command` 负责角色创建、更新、启停、用户角色绑定、角色权限绑定和 RBAC policy change 通知编排。该包通过 application port 消费 `RoleStore`、`UserRoleStore`、`RolePermissionStore`、`PermissionLookup`，并通过 permission application 的 `PolicyChangeNotifier` 触发 policy reload 或用户角色缓存失效。

该包已有 `mock_generate.go` 覆盖上述 role application port，并已有 `mock_permission_test.go` 覆盖 `PolicyChangeNotifier`。测试迁移应使用这些生成物表达依赖调用，避免手写 store/notifier double 把调用记录、状态变更和错误注入隐藏在自定义结构体中。

## Goals / Non-Goals

**Goals:**

- 将 role command service 测试中的外部协作者统一迁移到已有 gomock 生成物。
- 移除 `roleTestStore`、`userRoleTestStore`、`rolePermissionTestStore`、`permissionLookupTestStore`、`recordingRolePolicyChangeNotifier` 等实现 port 的手写 double。
- 用 expectation、`gomock.InOrder`、matcher 或 `DoAndReturn` 明确表达角色写操作、用户角色绑定、角色权限绑定、权限查找和 policy change 通知。
- 继续覆盖系统角色保护、输入去重、绑定替换、权限不存在或不可用、以及 policy change 通知失败被 command service 吞掉的行为。

**Non-Goals:**

- 不修改 role command 生产代码、role domain 规则、permission application notifier 实现或 PostgreSQL adapter。
- 不迁移 role query、role seed、HTTP transport 或 permission feature 测试。
- 不新增生产接口、跨包共享 mock 仓库或中央测试替身包。
- 不改变 HTTP API、OpenAPI、数据库 schema、Atlas migration、Redis policy sync、部署清单、观测资产或安全边界。

## Decisions

### Decision: 以已有生成 mock 作为唯一外部 collaborator 表达

实现时使用 `mock_generate.go` 已生成的 `NewMockRoleStore`、`NewMockUserRoleStore`、`NewMockRolePermissionStore`、`NewMockPermissionLookup` 和 `mock_permission_test.go` 已生成的 `NewMockPolicyChangeNotifier`。这样测试断言与 production port 方法签名保持同源，接口变化会通过编译错误或 expectation drift 直接暴露。

备选方案是继续保留手写 store/notifier double 并补充字段记录调用。该方案会维持两套协作者契约，且难以表达禁止调用、调用次数和通知失败吞掉等行为，因此不采用。

### Decision: 用 expectation 表达行为，用 matcher 处理领域参数

角色创建、更新、启停、用户角色绑定和角色权限绑定测试应优先用 `EXPECT()` 声明依赖调用、参数和错误返回。涉及去重、替换后的 ID 集合、`PermissionReference`、`PolicyChange` 或更新输入时，可以用当前包内 matcher 校验关键字段；确实需要模拟 store 返回或捕获参数时使用 `DoAndReturn`。

备选方案是大量使用 `gomock.Any()` 放宽参数。该方案会让去重、权限引用和 policy change reason 等关键契约失去断言价值，因此只应用于与当前断言无关的参数。

### Decision: 不为了测试迁移引入生产抽象

本变更只调整 command 包测试和既有生成 mock 的使用方式。若 `make user-service-generate` 暴露生成物 drift，只更新对应 `mock_*.go` 生成物；不得为了让测试更容易而新增生产接口、adapter 或共享 helper。

备选方案是提取新的测试基础设施或跨 feature mock 包。该方案扩大了影响面，并与本次仅标准化 role command service 测试的目标不匹配，因此不采用。

## Risks / Trade-offs

- [Risk] expectation 过细导致测试对无关调用顺序敏感 -> Mitigation：只对必须先校验再写入、写入成功后再通知等安全相关顺序使用 `gomock.InOrder`，其他路径按参数和调用次数断言。
- [Risk] 去重或集合比较在 matcher 中忽略顺序语义 -> Mitigation：对 command service 明确要求有序输出的场景使用顺序 matcher；仅对业务上无序的输入集合使用集合 matcher。
- [Risk] policy change 通知失败吞掉路径遗漏 -> Mitigation：为角色写操作、用户角色绑定和角色权限绑定成功后通知失败分别保留 expectation，并断言 command service 返回成功结果。
- [Risk] 生成 mock 与 `mock_generate.go` 不一致 -> Mitigation：执行 `make user-service-generate`，确认 `mock_test.go` 和 `mock_permission_test.go` 无 drift。

## Migration Plan

1. 梳理 `service_test.go` 中手写 store/notifier double 的使用点，并按角色生命周期、用户角色绑定、角色权限绑定和 policy change 通知分组。
2. 使用 `NewMockRoleStore`、`NewMockUserRoleStore`、`NewMockRolePermissionStore`、`NewMockPermissionLookup` 和 `NewMockPolicyChangeNotifier` 替换对应测试替身。
3. 将旧 double 内的状态断言迁移为 expectation、matcher、`gomock.InOrder` 或 `DoAndReturn`。
4. 删除不再使用的手写 store/notifier double 类型及其方法，保留不实现外部 port 的纯构造 helper。
5. 执行 `make user-service-generate`，确认 mockgen 输出无 drift。
6. 执行 `cd user-service && go test ./internal/features/role/application/command`。
7. 执行 `make user-service-architecture-lint`。

回滚方式是还原本次测试文件和 OpenSpec change artifacts；由于不改生产代码、schema、配置或部署资产，不需要运行时回滚步骤。

## Open Questions

- 无。
