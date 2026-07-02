## ADDED Requirements

### Requirement: 认证 command 测试协作者契约

认证 command use case 测试 MUST 使用该包已有 `mockgen` 生成物表达 credential、refresh session、token version、token issuer/verifier、metrics 和 lifecycle 等外部协作者契约。测试 MUST NOT 通过手写 collaborator double 兼容或隐藏这些 port 的调用、失败路径、调用顺序或指标记录。

#### Scenario: 登录测试使用生成 mock

- **WHEN** command 包测试覆盖登录成功、凭证失败、用户状态失败、强制改密或 token 签发失败路径
- **THEN** 测试 MUST 通过生成 mock 的 expectation 表达 credential store、password verifier、token issuer、refresh session store 和 metrics 调用
- **AND** 测试 MUST NOT 使用手写 credential、session、token issuer 或 metrics collaborator double 承载这些外部依赖

#### Scenario: 刷新测试使用生成 mock

- **WHEN** command 包测试覆盖 refresh token 解析、session 查询、token version 校验、rotation 或 rotation 失败路径
- **THEN** 测试 MUST 通过生成 mock 的 expectation、`gomock.InOrder`、matcher 或 `DoAndReturn` 表达 token verifier/issuer、refresh session store、token version store/cache 和 metrics 调用
- **AND** refresh session 与 token claims 一致性、rotation 原子失败和指标失败 reason MUST 由 expectation 或 matcher 明确断言

#### Scenario: 改密与退出测试使用生成 mock

- **WHEN** command 包测试覆盖修改密码、退出当前会话或退出全部会话的成功与失败路径
- **THEN** 测试 MUST 通过生成 mock 表达 credential store、token version store/cache、refresh session store、session lifecycle 和 metrics 调用
- **AND** token version 递增、本地缓存失效、Redis 投影刷新、refresh session 删除和 purge 提交失败等安全相关行为 MUST 通过 expectation 或 `DoAndReturn` 明确断言

#### Scenario: 保留纯构造 helper

- **WHEN** command 包测试需要复用用户凭据、token claims、auth session、密码 verifier 或 use case 构造逻辑
- **THEN** 测试 MAY 保留不实现外部 collaborator port 的纯构造 helper 或真实轻量纯函数依赖
- **AND** 这些 helper MUST NOT 替代 `mockgen` 生成物记录 collaborator 调用或隐藏失败注入
