# Cleanup source structure and comments

## What

对 `common/` 与 `user-service/` 的人工维护 Go 源码做一次结构整理和注释补强，保持现有功能不变：

- 查找并删除仅为测试方便而残留在正式代码中的临时代码、无用代码或冗余入口。
- 同步调整相关测试，让测试继续通过真实生产边界或明确的测试替身完成覆盖。
- 检查单个源码文件内的声明顺序，修复 type、const、var、构造函数、公开方法、私有 helper 穿插混乱的问题。
- 补充或精简代码注释，确保关键逻辑、重要结构、对外接口和复杂实现都有中文说明。
- 清理无价值注释，避免用注释重复代码本身。

本变更不改变 HTTP API、业务流程、数据库 schema、Redis key schema、配置 key、OpenAPI 契约或部署资产。

## Why

仓库已经建立 feature-first 分层、shared/kernel 准入规则、中文注释和英文日志规则。随着测试、RBAC、观测和运行时能力持续补齐，一些文件可能出现以下维护风险：

- 正式代码中残留只为测试场景服务的 hook、分支、临时 helper 或过宽的可替换入口。
- 测试依赖生产代码中的测试缝，导致真实运行边界不够干净。
- 文件内声明顺序随迭代变得松散，读者需要在 type、函数、方法之间跳转才能理解主线。
- 关键结构和复杂实现注释不足，或者注释语言、价值和当前行为不一致。

本次整理的目标是降低后续开发认知成本，让源码更贴合 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 的长期结构规则。

## Scope

包括：

- 审计 `common/` 与 `user-service/` 下非生成 Go 源码。
- 排除 `user-service/ent/` 生成代码；`user-service/ent/schema/` 属于人工维护 schema，可审计注释和声明顺序。
- 审计测试文件中为了配合正式代码清理需要调整的 helper、fake、stub 和断言。
- 修复单文件内明显不合理的声明顺序，优先保持仓库现有阅读主线：
  - package/import
  - const/var
  - type 与 Fx 参数结构
  - 构造函数或 provider
  - 公开 handler/use case 方法
  - 私有 helper
- 为关键公开类型、构造函数、use case、controller handler、provider、复杂 helper、并发或一致性逻辑补充中文注释。
- 删除已经失效、重复代码或只解释语法的注释。
- 如发现正式代码中确有测试专用分支或临时入口，删除后把测试改为使用 `_test.go` 内 helper、接口替身、httptest、sqlmock/miniredis/testcontainers 等更合适方式。

不包括：

- 不新增 `openspec/` 或 `docs/opsx/` 工件。
- 不迁移目录结构，不新增 shared 子包，不新增横向 `internal/service`、`internal/repository`、`internal/controller` 等目录。
- 不手写 Ent 生成代码，不新增 Atlas migration。
- 不调整业务行为、HTTP response envelope、错误码、鉴权语义、RBAC policy 同步语义或 OpenAPI schema。
- 不为“可能未来测试需要”新增新的测试 hook。
- 不引入新测试框架、日志库、代码生成器或格式化工具。

## Acceptance Criteria

- 没有新增 `openspec/` 或 `docs/opsx/`。
- 所有被确认只为测试而存在的正式代码冗余已删除，测试改为在测试层表达替身和 fixture。
- 单文件内声明顺序符合仓库阅读主线，尤其不再出现无关 type/const/var 穿插在两个主要函数之间造成主线断裂。
- 关键逻辑、重要结构、对外接口和复杂实现具备准确、简洁的中文注释。
- 注释不复制 `context`、规则文档或历史 change 内容；只解释源码当前行为。
- 日志消息仍保持英文，日志字段名仍为英文 snake_case。
- `gofmt` 后相关测试通过，至少运行 `make test-common`、`make test-user-service` 和 `make architecture-lint`。
