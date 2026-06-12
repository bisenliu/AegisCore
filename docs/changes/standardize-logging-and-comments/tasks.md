# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和本 change 的 `proposal.md`、`design.md`。
- [x] 确认本次不新增 `openspec/` 或 `docs/opsx/`。
- [x] 生成日志调用清单：

```bash
rg -n "logger\\.(Debug|Info|Warn|Error)|\\.Debug\\(|\\.Info\\(|\\.Warn\\(|\\.Error\\(" common user-service --glob '*.go'
```

- [x] 生成非生成源码英文注释候选清单：

```bash
rg -n "^\\s*//\\s*[A-Za-z]" common user-service --glob '*.go' -g '!user-service/ent/**'
```

- [x] 生成疑似中文日志候选清单，并人工排除非日志字符串：

```bash
rg -n "\"[^\"]*[\\p{Han}][^\"]*\"" common user-service --glob '*.go'
```

## Main Rules

- [x] 更新 `AGENTS.md`，补充“代码注释和函数/方法注释必须使用中文，日志消息必须使用英文”的仓库规则。
- [x] 更新 `docs/ARCHITECTURE.md` Logging And Trace ID 规则，补充日志语言、字段命名、等级语义和 trace-id helper 要求。
- [x] 确认 `AGENTS.md` 与 `docs/ARCHITECTURE.md` 的规则一致。

## Logging Audit

- [x] 审计 `common/runtime/logger`、`common/runtime/datastore`、`common/runtime/workerpool` 的日志消息、字段和级别。
- [x] 审计 `common/http/middleware` 的 request logging、auth、binding、recovery、trace-id 日志。
- [x] 审计 `user-service/internal/bootstrap` 的 HTTP server lifecycle 日志。
- [x] 审计 `user-service/internal/providers` 的 provider、route、Ent、Redis/PostgreSQL lifecycle 日志。
- [x] 审计 `user-service/internal/features/user/application` 的创建、查询、列表日志。
- [x] 审计 `user-service/internal/features/auth/application` 的登录、刷新、改密、登出、session/token version validation 日志。
- [x] 审计 `user-service/internal/features/auth/infrastructure/postgres` 和 `infrastructure/redis` 的存储、缓存、后台任务和 fallback 日志。
- [x] 为异步任务、fallback、外部依赖失败和无法被上层感知的错误补充结构化日志。
- [x] 更正误用日志级别，确保业务预期拒绝为 `Warn` 或 `Info`，系统异常为 `Error`，调试细节为 `Debug`。
- [x] 确认业务日志优先使用 `common/runtime/logger` context helper，保留 trace-id。
- [x] 确认日志消息全部为英文，字段名保持英文 snake_case。

## Comment Standardization

- [x] 翻译 `common/` 非生成源码中的英文注释为中文。
- [x] 翻译 `user-service/` 非生成源码中的英文注释为中文，排除 `user-service/ent/` 生成代码。
- [x] 检查 `user-service/ent/schema/` 中人工维护 schema 注释，必要时改为中文。
- [x] 检查测试文件中的人工维护英文注释，必要时改为中文或删除无价值注释。
- [x] 保持 Go doc 格式，导出函数、方法、类型、常量和变量注释仍以对应 identifier 开头。
- [x] 不修改 identifier、error string、HTTP response message、配置 key、Redis key、数据库字段或 Swagger schema 名称。

## Tests

- [x] 更新受日志消息影响的单元测试断言。
- [x] 更新受注释样例或 helper 文本影响的测试。
- [x] 运行 `gofmt` 格式化所有修改过的 Go 文件。
- [x] 运行 common 模块测试：

```bash
make test-common
```

- [x] 运行 user-service 模块测试：

```bash
make test-user-service
```

## Verification

- [x] 确认非生成源码无英文注释候选残留，所有残留都已人工判定为允许项：

```bash
rg -n "^\\s*//\\s*[A-Za-z]" common user-service --glob '*.go' -g '!user-service/ent/**'
```

- [x] 确认日志消息没有中文内容。可先扫描中文字符串，再人工确认是否属于日志：

```bash
rg -n "\"[^\"]*[\\p{Han}][^\"]*\"" common user-service --glob '*.go'
```

- [x] 确认生成代码未被手写修改：

```bash
git diff -- user-service/ent
```

- [x] 检查变更范围：

```bash
git diff --stat
git diff -- AGENTS.md docs/ARCHITECTURE.md common user-service
```

## Guardrails

- [x] 不新增 OpenSpec/OPSX 工件。
- [x] 不手写 Ent 生成代码。
- [x] 不修改 HTTP API、response envelope、配置 key、数据库 schema、migration 或 Redis key schema。
- [x] 不引入新日志库、metrics、tracing exporter 或告警系统。
- [x] 不把预期客户端输入错误记录为 `Error`。
- [x] 不把外部依赖失败、panic recover 或后台任务失败降级为 `Info`。
