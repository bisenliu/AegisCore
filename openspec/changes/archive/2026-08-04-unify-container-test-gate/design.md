## Context

`common/testing/containers/postgres.go` 注册 `-aegiscore.testcontainers`，`StartPostgres` 和 `StartRedis` 在 flag 未启用时 skip，在启用后则把 Docker daemon、镜像、启动与连接失败转为测试失败。代码没有读取 `AEGISCORE_TEST_CONTAINERS`。

`user-service/Makefile` 已有 `test-containers`，但只覆盖 permission/role PostgreSQL adapter 与 HTTP E2E；`common/Makefile` 和根 `Makefile` 没有对应 target。`.github/workflows/ci.yml` 因此只执行 user-service 的三个包，遗漏 `common/testing/containers` 内直接验证 PostgreSQL 和 Redis fixture 的集成测试。`docs/TESTING.md` 与当前主规格同时存在 flag 和环境变量两套说法。

受影响路径包括 `Makefile`、`common/Makefile`、`user-service/Makefile`、`.github/workflows/ci.yml`、架构 lint、测试文档和 OpenSpec。容器 helper 的生产无关实现已经具备所需失败语义，不需要增加环境变量解析或新的 Go 分支。

## Goals / Non-Goals

**Goals:**

- 以根 `make test-containers` 唯一表示 common 与 user-service 的完整真实依赖门禁。
- 所有模块 target 显式传递同一个 Go flag，不依赖测试进程解释环境变量。
- CI 输出实际测试名，禁用结果缓存，并保证 flag 启用后 Docker-backed 测试不能静默 skip。
- 用静态架构门禁防止 CI 调用路径再次漂移。

**Non-Goals:**

- 不让普通 `make test` 启用容器测试，也不在 unit、race、coverage 或 verify job 重复容器负载。
- 不修改 PostgreSQL/Redis fixture 行为、镜像版本、migration 或 E2E 场景集合。
- 不兼容或继续解释 `AEGISCORE_TEST_CONTAINERS`。

## Decisions

### Decision: 根 Make target 负责跨模块编排

新增根 `test-containers`，依赖 `common-test-containers` 与 `user-service-test-containers`；模块 target 各自拥有包列表和 `go test` 参数。CI 和完整本地验收只调用根入口，窄化调试仍可调用模块 target 或显式 `go test ... -args -aegiscore.testcontainers`。

不让根 target 拼接跨 module 的 `go test` 包路径，因为 Go workspace 的模块边界和模块私有包列表应由各自 Makefile 拥有。也不把容器测试并入普通 `make test`，避免开发机或非容器 CI 意外拉起 Docker。

### Decision: Go flag 是唯一启用契约

common 与 user-service target 都直接传递 `-args -aegiscore.testcontainers`。不读取 `AEGISCORE_TEST_CONTAINERS`，因为这会重新形成两套入口，并把 Make/CI 的显式行为隐藏到测试进程初始化中。

模块 target 使用 `-v -count=1`：`-v` 在 CI 日志中记录测试名与耗时，`-count=1` 防止真实依赖验收被 Go test cache 替代。flag 为真后，现有 `requireContainersEnabled` 和 E2E guard 不会执行 `t.Skip`；后续 Docker、连接、migration 或配置失败沿现有 fatal/require 路径使命令失败。

### Decision: 架构 lint 固定 CI 根入口

扩展 `check_ci_quality_workflow`，要求主 CI 与复用质量 workflow 中恰好出现一次独立的 `make test-containers`，并拒绝 `make -C common test-containers`、`make -C user-service test-containers` 等绕过根编排的 CI 调用。fixture 增加错误调用以证明规则可命中。

不解析 GitHub Actions 的完整 YAML 语义；现有 lint 已通过稳定命令扫描固定标准质量入口，本次沿用相同边界。

## Risks / Trade-offs

- [Risk] common 与 user-service 顺序执行会增加容器 job 时长 -> Mitigation：只执行明确包含 Docker-backed 测试的包，不重复 `./...` 普通测试，并保留 35 分钟 job timeout。
- [Risk] verbose 输出增加日志量 -> Mitigation：容器包数量有限，测试名与耗时是门禁可审计性的必要输出。
- [Risk] 静态命令扫描依赖稳定的 YAML 写法 -> Mitigation：fixture 覆盖正确根命令和错误 service-local 命令，变更 CI 编排时必须同步更新规则。

## Migration Plan

先新增并验证 Make targets，再将 CI 和文档切换到根入口，最后运行架构 lint、普通测试及完整 lint/verify。回滚时可同时恢复 CI、Make targets、文档和规格；生产运行时与数据不需要迁移或回滚。
