## Context

`user-service/internal/features/auth/application/sessions` 负责 refresh session 生命周期、token version 投影回源、会话旋转、全量撤销和本地 token version cache 失效等认证会话语义。当前 `lifecycle_test.go` 中仍保留手写 `sessionUserTestStore`、`authSessionTestStore` 和 `tokenVersionRecordingInvalidator`，测试通过内部状态字段表达调用结果和副作用。

该包已经具备 mockgen 生成物：`mock_generate.go` 提供 `UserTokenVersionStore`、`TokenVersionCache`、`RefreshSessionStore` mock，`mock_validators_test.go` 提供 `TokenVersionLocalInvalidator` mock。继续保留手写替身会让测试契约与真实端口定义分离，并增加接口变更后的维护成本。

## Goals / Non-Goals

**Goals:**

- 让 `lifecycle_test.go` 中的 session lifecycle 测试全部依赖已生成 gomock mock 或真实领域值。
- 用 gomock expectation 明确表达 token version 回源、Redis token version 投影读写、refresh session 创建/旋转/撤销和本地缓存失效的调用契约。
- 删除 `sessionUserTestStore`、`authSessionTestStore` 和 `tokenVersionRecordingInvalidator`，不保留兼容路径。
- 保持 `make user-service-generate` 后 mockgen 产物无 drift。

**Non-Goals:**

- 不修改 session lifecycle 生产代码、端口接口、领域模型或对外 HTTP API。
- 不修改 Redis session store 测试、auth command 测试或 token version validator 测试。
- 不新增测试专用生产构造函数、冗余 adapter 或仅为测试存在的生产分支。
- 不变更 OpenAPI、Ent schema、Atlas migration、部署清单、观测资产或安全策略。

## Decisions

1. 使用现有 gomock 生成物替代手写测试替身。

   理由：生成 mock 直接绑定当前端口接口，接口签名变化时编译与测试会立即暴露 drift。相比保留手写 store，gomock expectation 能在测试用例中就地声明调用次数、参数和返回值，避免隐藏状态字段成为第二套行为模型。

   备选方案：保留手写替身并补充断言。该方案改动小，但不能解决测试替身与端口契约分离的问题，也不符合本次验收要求。

2. 将状态记录迁移为 matcher、`DoAndReturn` 和顺序 expectation。

   理由：session lifecycle 测试不仅要验证最终返回值，还要验证 token version 投影刷新、fallback、refresh session rotation 和 delete-all 等关键交互。matcher 适合表达领域值和 UUID/token version 参数，`DoAndReturn` 适合在测试中捕获创建或旋转后的 session 结构，`gomock.InOrder` 适合约束先递增 token version、再刷新投影、再撤销 session 这类安全敏感顺序。

   备选方案：只使用宽松 `Any()` matcher。该方案实现更快，但会降低测试对安全关键参数和调用顺序的保护，不能覆盖验收中“通过 expectation 明确表达”的目标。

3. 将测试夹具收敛到包内测试 helper，而不是生产构造函数。

   理由：本次变更只影响 `application/sessions` 包测试，生产代码已经通过端口注入具备可测试性。新增生产 helper 或 adapter 会扩大 API 面并制造无业务语义代码，违反 auth feature 分层边界。

   备选方案：新增专用生产构造函数简化测试装配。该方案会把测试便利性泄漏到生产 API，且不在本次范围内。

## Risks / Trade-offs

- [Risk] gomock expectation 过度精确可能让测试对无关调用细节敏感。→ Mitigation：只对 token version、session ID、user ID、rotation、delete-all 和 invalidation 等安全语义相关调用使用严格 matcher 或顺序约束，非关键上下文参数保持宽松。
- [Risk] 删除手写替身后，某些测试用例需要更多 setup 代码。→ Mitigation：抽取仅限测试文件内的最小 helper，复用 mock controller 和生命周期 use case 装配，不新增生产代码。
- [Risk] 生成 mock 与接口定义存在历史 drift。→ Mitigation：执行 `make user-service-generate` 并检查无 mockgen drift。
- [Risk] 本 change 只改测试，可能被误解为业务需求变更。→ Mitigation：规格 delta 明确限定为测试契约，不改变 session lifecycle 生产行为。

## Migration Plan

1. 在 `lifecycle_test.go` 中引入 gomock controller 和现有生成 mock，逐个测试用例替换手写 store/invalidator。
2. 将旧状态字段断言改写为 matcher、`DoAndReturn` 或顺序 expectation。
3. 删除 `sessionUserTestStore`、`authSessionTestStore` 和 `tokenVersionRecordingInvalidator` 定义。
4. 执行 `make user-service-generate`，确认生成 mock 无 drift。
5. 执行 `cd user-service && go test ./internal/features/auth/application/sessions` 和 `make user-service-architecture-lint`。

回滚方式：如果改造引入不可接受的测试复杂度，可在提交前恢复 `lifecycle_test.go` 到变更前状态；由于不涉及生产代码、schema 或部署资产，不需要运行时回滚或数据迁移。

## Open Questions

无。
