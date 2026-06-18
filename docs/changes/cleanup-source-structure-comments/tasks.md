# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`。
- [x] 确认本次不新增 `openspec/` 或 `docs/opsx/`。
- [x] 检查当前 git 状态，避免覆盖用户已有改动。
- [x] 生成源码清单，排除生成代码、本地工具缓存和运行产物。

## Audit

- [x] 扫描疑似测试专用或临时代码：

```bash
rg -n "\b(TODO|FIXME|HACK|TEMP|temporary|only for test|test only|for tests?|mock|stub|fake|fixture)\b" common user-service --glob '*.go' -g '!user-service/ent/**' -g '!user-service/docs/openapi.go'
```

- [x] 扫描疑似正式代码测试 hook 或可替换入口：

```bash
rg -n "ForTest|testHook|override|inject|mock|stub|fake" common user-service --glob '*.go' -g '!user-service/ent/**' -g '!user-service/docs/openapi.go'
```

- [x] 人工区分合法运行时依赖注入、测试文件 helper 和应清理的正式代码冗余。
- [x] 扫描非生成源码注释候选，找出英文注释、失效注释和缺失关键注释的文件。
- [x] 审查 controller、command/query、provider、store adapter、runtime primitive 等关键文件的声明顺序。

## Cleanup Production Code

- [x] 删除确认只为测试服务的正式代码冗余、临时分支或无用 helper。
- [x] 将仍需测试替换的逻辑迁移到 `_test.go` helper、application port fake 或已有测试基础设施。
- [x] 同步修改相关测试，保持原有覆盖意图不变。
- [x] 确认没有新增横向目录、shared 子包、OpenSpec/OPSX 工件或生成代码手改。

## Reorder Source Files

- [x] 按声明顺序模型整理 `common/` 中明显混乱的人工维护文件。
- [x] 按声明顺序模型整理 `user-service/internal/providers`、`bootstrap`、`router` 中明显混乱的文件。
- [x] 按路由注册或 use case 主线整理各 feature 的 `transport/http` controller 和 input 文件。
- [x] 按构造函数、公开方法、私有 helper 主线整理 application command/query 和 infrastructure adapter 文件。
- [x] 整理测试文件中的 helper 位置，避免 helper 打断主要测试场景。

## Comments

- [x] 为关键公开类型、接口、构造函数、provider、controller handler 和 use case 补充准确中文 Go doc。
- [x] 为复杂逻辑补充简洁中文注释，重点关注 session 生命周期、RBAC policy 同步、缓存 fallback、worker pool、scheduler、health/readiness、OpenAPI 转换和 tracing/logger 关联。
- [x] 删除或改写重复代码本身、已经失效或容易误导的注释。
- [x] 保持日志消息英文，日志字段名英文 snake_case。
- [x] 不修改 Go identifier、错误字符串、HTTP response message、配置 key、数据库字段、Redis key 或 OpenAPI schema 名称。

## Formatting And Tests

- [x] 对修改过的 Go 文件运行 `gofmt`。
- [x] 运行被修改包的针对性测试。
- [x] 运行共享模块测试：

```bash
make test-common
```

- [x] 运行用户服务测试：

```bash
make test-user-service
```

- [x] 运行架构边界检查：

```bash
make architecture-lint
```

## Verification

- [x] 检查疑似临时代码残留并人工解释合理残留：

```bash
rg -n "\b(TODO|FIXME|HACK|TEMP|temporary|only for test|test only|for tests?)\b" common user-service --glob '*.go' -g '!user-service/ent/**' -g '!user-service/docs/openapi.go'
```

- [x] 检查英文注释候选并人工解释合理残留：

```bash
rg -n "^\\s*//\\s*[A-Za-z]" common user-service --glob '*.go' -g '!user-service/ent/**' -g '!user-service/docs/openapi.go'
```

- [x] 确认没有手写修改 Ent 生成代码：

```bash
git diff -- user-service/ent
```

- [x] 确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 检查最终变更范围：

```bash
git diff --stat
git diff -- docs/changes/cleanup-source-structure-comments common user-service
```

## Ready To Apply

- [x] `proposal.md` 已说明 what/why/scope/acceptance criteria。
- [x] `design.md` 已说明审计、排序、注释、实现与验证策略。
- [x] `tasks.md` 已列出可执行实现步骤。
