## Context

审计覆盖 `common`、`user-service`、仓库级工具、部署资产、文档和 OpenSpec。判定同时使用 `rg` 反向引用、Go 正式入口可达性、`golangci-lint`、`go test`、`go vet`、模块依赖整理检查、脚本语法检查和主规格核对。仅凭静态工具报告不可达不足以删除 Ent schema、生成入口、主规格要求的授权白名单或具备独立公共语义的 scheduler API。

本次变更横跨共享实现、服务测试、交付脚本和文档，但目标是删除无消费者实现并修复 drift，不改变稳定业务行为。

## Goals / Non-Goals

**Goals:**

- 删除仓库内没有消费者且已有明确替代入口的共享 helper。
- 把只服务于测试的 panic helper 移出生产文件。
- 删除会产生额外登录副作用的无用脚本状态。
- 让 pprof 和 lint 文档与当前 loader、CI 和主规格一致。
- 减少 user-service 镜像编译层不需要的构建上下文和 cache invalidation。

**Non-Goals:**

- 不删除主规格明确要求的授权白名单、scheduler、workerpool、OpenAPI、Fx graph 或生成器入口。
- 不删除已进入 OpenAPI enum 的预留错误码；该类外部契约需要独立评估。
- 不删除可能被远端 branch protection 引用的 lint workflow，也不删除作为人工入口暴露的 Make verify 目标。
- 不修改业务 API、数据库 schema、认证/RBAC 语义或观测指标契约。

## Decisions

### 1. 只有多重证据一致时删除

删除候选必须满足全仓无真实消费者，并且具有现成替代入口、只为测试便利存在或与当前架构约束冲突。`JSONBinderWithOptions` 的两个布尔模式分别由 `JSONBinder` 和 `StrictJSONBinder` 覆盖；`ValidatePositiveInt` 在 bcrypt 迁移后没有配置调用方；共享 `ConfigPath`/`NewConfig` 没有消费者，且会允许 Fx 再次读取配置文件。

### 2. 测试便利代码只留在测试编译单元

auth 和 permission 的生产构造路径已经使用返回 `error` 的 `NewKeyCatalog`。测试继续需要简洁构造时，在 `_test.go` 中定义小写 `mustKeyCatalog`；panic 语义只存在于测试编译单元，正式构造仍传播配置错误，避免扩大生产 API。

### 3. 脚本清理不得改变目标流量

`NORMAL_REFRESH_TOKEN` 从未读取，但赋值会再次调用登录接口并创建额外 refresh session。删除状态和第二次登录后，脚本仍通过第一次登录获得正常用户 access token，后续 API、RBAC 和指标流量保持可执行，且不再混入无意义的 auth success。

### 4. Docker 只复制编译所需 workspace 内容

workspace 解析需要 `tools/openapi-convert/go.mod`，user-service 编译不导入该工具源码。manifest-first 阶段继续复制该 `go.mod`，编译层删除 `COPY tools ./tools`；`.dockerignore` 只放行父目录和该 manifest。实际 Docker build 和镜像内容校验用于证明构建流程不受影响。

### 5. 保守保留不确定或有契约价值的候选

`Scheduler.RemoveJob` 虽无当前调用，但具有不可由其他 API 替代的动态注销语义；授权白名单由 RBAC 主规格要求；Ent schema 方法由 codegen 消费；错误码已进入 OpenAPI；CI workflow 可能由仓库外 branch protection 引用。这些项目不纳入删除。

## Risks / Trade-offs

- [仓库外 Go 消费者仍调用被删除的导出 helper] -> 在 proposal 中标记 compile-time breaking，并提供现有替代 API；当前仓库和 workspace 无消费者。
- [Docker 构建隐式依赖 tools 源码] -> 保留 workspace 所需 manifest，运行实际 `docker buildx build` 和镜像内容校验。
- [测试 helper 迁移掩盖构造错误] -> helper 使用 `testing.TB` 和 `require.NoError`，不使用未检查错误。
- [文档批量替换误改 legacy 负向测试] -> 只改面向使用者的文档，保留证明旧 `PPROF_*` 被忽略的测试。

## Migration Plan

1. 删除共享无用 helper 和旧 Fx loader，迁移 Redis adapter 测试 helper。
2. 清理指标脚本、忽略规则和 Docker 构建上下文。
3. 修正文档中的环境变量、规格链接和工具版本。
4. 运行定向测试、lint、架构检查、脚本检查、Docker build、镜像校验和全仓 verify。

回滚方式是整体回滚本 change。没有数据迁移、双写、部署顺序或运行时兼容窗口要求。
