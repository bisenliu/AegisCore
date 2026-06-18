# Design

## Overview

本变更是行为保持型整理，按“先审计、再小批量修复、最后验证”的方式推进：

```text
扫描候选
  -> 人工判断是否为真实问题
  -> 删除测试专用正式代码或重排声明顺序
  -> 补充中文注释
  -> gofmt 与测试验证
```

实现时必须避免机械批量改写。测试相关命名、mock/fake/stub 在 `_test.go` 中是合理的；只有进入正式构建产物并且没有运行时职责的测试缝、临时分支或冗余 helper 才应删除。

## Source Scope

审计范围：

- `common/` 人工维护 Go 源码和测试。
- `user-service/` 人工维护 Go 源码和测试。
- `user-service/ent/schema/` 中人工维护 Ent schema。
- `user-service/internal/tools/` 中人工维护工具代码。

排除范围：

- `user-service/ent/` 生成代码，但不包括 `user-service/ent/schema/`。
- `user-service/docs/openapi.go`、`openapi.json`、`openapi.yaml` 等生成产物。
- `go.sum`、migration SQL、Atlas checksum、部署 YAML/JSON、历史 `docs/changes/*`。
- `.kilo/`、`.kilocode/`、`.agent/`、`.codegraph/`、`bin/`、`logs/` 等本地工具或运行产物。

## Test-Only Production Code Audit

候选扫描命令：

```bash
rg -n "\b(TODO|FIXME|HACK|TEMP|temporary|only for test|test only|for tests?|mock|stub|fake|fixture)\b" common user-service --glob '*.go' -g '!user-service/ent/**' -g '!user-service/docs/openapi.go'
rg -n "var .* = .*ForTest|New.*ForTest|testHook|override|inject" common user-service --glob '*.go' -g '!user-service/ent/**' -g '!user-service/docs/openapi.go'
```

判断规则：

- 正式代码中为了运行时解耦而定义的消费侧接口、Fx provider 参数、clock/ID generator 等稳定依赖注入，不因为测试会使用而删除。
- `_test.go` 中的 fake、stub、mock、fixture、helper 是测试代码，不属于本项清理对象。
- 如果正式代码中的替换点只被测试使用，且没有清晰运行时扩展职责，应删除或收窄到测试文件。
- 如果删除测试缝会迫使测试访问真实 PostgreSQL/Redis 等外部依赖，需要优先考虑已有 testing 基础设施、miniredis/sqlmock/testcontainers 或 application port fake，而不是把测试分支放回正式代码。
- 删除代码后必须同步调整测试，确保覆盖意图没有变弱。

## Declaration Ordering Model

单个 Go 文件的推荐顺序：

1. package 与 import。
2. const、var 和包级 sentinel error。
3. 主要 type、接口、Fx params/results、DTO 或输入输出结构。
4. 构造函数、provider 或 factory。
5. 公开方法和公开函数，顺序跟路由注册、use case 主流程或调用主线一致。
6. 私有 helper，优先紧跟主要调用方；若被多处使用，放在文件尾。

特殊情况：

- HTTP controller handler 顺序应尽量与 `routes.go` 注册顺序一致。
- command/query use case 文件优先让构造函数位于业务方法前，业务方法按主流程排列。
- store adapter 文件优先让构造函数位于 CRUD 方法前，helper 和 mapper 位于方法后。
- 测试文件可按被测对象、场景和 helper 分区，但 helper 不应打断主要测试阅读主线。
- 不为了排序手写 Ent/OpenAPI 生成代码。

## Comment Model

注释目标：

- 导出类型、接口、函数和方法保留 Go doc 注释，并使用中文说明当前职责。
- 复杂并发、缓存一致性、RBAC policy 同步、session 生命周期、迁移或健康检查语义，应在关键实现处保留简洁说明。
- 对外接口和 feature 边界注释要说明“消费什么、保证什么”，避免写成实现流水账。
- 必要技术术语可保留英文，例如 HTTP、JWT、Redis、PostgreSQL、Ent、Fx、Gin、OpenAPI、Casbin、trace-id。

删除或改写：

- 删除“设置字段”“调用函数”这类重复代码本身的注释。
- 修正已经和代码行为不一致的历史注释。
- 不把架构规则、change 上下文或大段背景复制到源码注释中。
- 不把日志消息、错误字符串、HTTP response message 改成中文；日志仍必须是英文。

## Implementation Strategy

1. 创建候选清单：测试专用代码、临时标记、英文或缺失注释、声明顺序明显混乱的文件。
2. 按模块分批判断，先处理小范围低风险文件。
3. 删除正式代码中的真实冗余后，立即更新对应测试。
4. 重排声明顺序时只移动代码块，不改变逻辑；移动后运行 `gofmt`。
5. 补注释时优先补关键公开结构和复杂逻辑，避免给简单 getter 或 DTO 字段添加噪声注释。
6. 每批修复后运行针对性 `go test ./...` 或包级测试，最后运行仓库级验证。

## Verification Strategy

基础验证：

```bash
gofmt -w <changed-go-files>
make architecture-lint
make test-common
make test-user-service
```

补充扫描：

```bash
rg -n "\b(TODO|FIXME|HACK|TEMP|temporary|only for test|test only|for tests?)\b" common user-service --glob '*.go' -g '!user-service/ent/**' -g '!user-service/docs/openapi.go'
rg -n "^\\s*//\\s*[A-Za-z]" common user-service --glob '*.go' -g '!user-service/ent/**' -g '!user-service/docs/openapi.go'
git diff -- user-service/ent
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

扫描结果需要人工解释，不要求零输出。例如测试文件中的 fake、OpenAPI 技术术语、identifier 开头的 Go doc 都可能是合理残留。

## Risks And Mitigations

风险：

- 把合法的运行时依赖注入误判为测试专用代码。
- 只做机械排序，反而破坏局部阅读主线。
- 注释补充过多，让源码变得啰嗦。
- 修改测试 helper 时降低真实行为覆盖。

缓解：

- 每个删除都要求能回答“正式运行时为什么不需要它”。
- 重排只处理明显混乱文件，保持局部调用关系清晰。
- 优先删除无价值注释，少量补充关键注释。
- 测试调整后检查覆盖的输入、输出、错误和边界条件是否仍完整。
