# 测试与质量门禁规格

## 需求

### 需求：测试入口
`make test` 必须测试两个 workspace module。模块级测试必须在 `common/` 和 `user-service/` 内执行。

#### 场景：默认测试
Given 开发者运行 `make test`
When 容器测试开关未设置
Then 默认测试不得要求 Docker、PostgreSQL 或 Redis。

### 需求：集成测试开关
真实 PostgreSQL/Redis 测试必须由 `AEGISCORE_TEST_CONTAINERS` 显式启用。完整 user-service HTTP E2E 测试必须由 `AEGISCORE_TEST_E2E` 或容器测试开关启用。

#### 场景：启用容器测试
Given `AEGISCORE_TEST_CONTAINERS=1`
When Docker 或所需镜像不可用
Then 测试必须明确失败，不得静默声称已执行集成覆盖。

### 需求：质量门禁
相关变更必须通过 lint、architecture-lint、build、测试、OpenAPI drift 检查和适用的 migration validation。

#### 场景：分层 import 违规
Given 领域层导入 Gin 或 Ent
When architecture-lint 或 depguard 运行
Then 违规必须导致质量门禁失败。

#### 场景：OpenSpec 文档语言违规
Given `openspec/specs/`、`openspec/changes/` 或 `docs/opsx/` 下的 Markdown 文档保留英文模板标题或英文模板段落
When architecture-lint 运行
Then 违规必须导致质量门禁失败，并要求将说明性内容改为简体中文。

### 需求：生产代码不暴露测试专用 API
正式代码不得暴露 `NewXForTest`、`testHook`、`setNowForTest` 等只为测试服务的 API，除非它们是具有真实运行时职责的抽象。

#### 场景：测试需要控制时间
Given 测试需要确定性时间
When 添加生产代码
Then 必须使用具有运行时语义的 clock dependency，或将 helper 保留在 `_test.go` 中。
