# Design

## Overview

本变更把 `docs/ARCHITECTURE.md` 第 6 节的依赖规则映射为 `golangci-lint` `depguard` 配置。实现重点是使用目录匹配规则约束 import，而不是用运行时代码、测试 helper 或自定义脚本重复实现检查逻辑。

根目录 `.golangci.yml` 已使用 `golangci-lint` v2 配置结构，执行方式是分别进入 `common/` 和 `user-service/` module 运行 lint。因此 depguard 配置继续放在根目录，由模块内命令通过向上查找或显式 `--config ../.golangci.yml` 读取。

## Rule Model

`depguard` 使用 rule-scoped `files` 匹配 Go 文件，再用 `deny` 禁止 import path。每条禁止规则都带 `desc`，让 lint 输出直接说明架构原因。

建议保留现有 `default: standard` 和 `revive` 配置，只新增：

```yaml
linters:
  enable:
    - revive
    - depguard
  settings:
    depguard:
      rules:
        feature-domain:
          files:
            - "**/internal/features/**/domain/**/*.go"
          deny:
            - pkg: "github.com/gin-gonic/gin"
              desc: "domain must not depend on Gin transport"
```

实际落地时应补齐所有分层规则，避免只保护某一个 feature 或某一种 import。

## File Matching

规则按目录职责命名，方便 lint 输出和后续维护：

- `feature-domain`
- `feature-application`
- `feature-transport-http`
- `feature-transport-grpc`
- `feature-infrastructure`
- `integration-boundary`

建议匹配范围：

```yaml
feature-domain:
  files:
    - "**/internal/features/**/domain/**/*.go"

feature-application:
  files:
    - "**/internal/features/**/application/**/*.go"

feature-transport-http:
  files:
    - "**/internal/features/**/transport/http/**/*.go"

feature-transport-grpc:
  files:
    - "**/internal/features/**/transport/grpc/**/*.go"

feature-infrastructure:
  files:
    - "**/internal/features/**/infrastructure/**/*.go"

integration-boundary:
  files:
    - "**/internal/integration/**/*.go"
```

如果 depguard `files` pattern 对当前版本的路径匹配表现与预期不同，实施时应先用一个临时测试 import 或现有代码扫描确认，再调整为当前 `golangci-lint` v2.12.2 支持的 pattern。配置必须通过 `golangci-lint config verify --config .golangci.yml`。

## Deny Lists

### Domain

Domain 层禁止依赖 transport、infrastructure、runtime 和 application ports：

- `github.com/gin-gonic/gin`
- `github.com/aegiscore/user-service/ent`
- `github.com/aegiscore/user-service/ent/...`
- `github.com/redis/go-redis/v9`
- `github.com/aegiscore/common/runtime/config`
- `github.com/aegiscore/common/runtime/logger`
- `github.com/aegiscore/common/contract/response`
- `github.com/aegiscore/common/http/response`
- `github.com/aegiscore/user-service/internal/features/*/application`
- `github.com/aegiscore/user-service/internal/features/*/application/...`
- `github.com/aegiscore/user-service/internal/features/*/infrastructure`
- `github.com/aegiscore/user-service/internal/features/*/infrastructure/...`

Depguard deny entries are import-path patterns rather than Go package wildcards in prose. During implementation, use the syntax supported by the installed depguard version for nested package matching, and verify with `golangci-lint config verify` plus a lint run.

### Application

Application 层可以依赖 domain、消费侧 ports 和 common security primitives，但不能吸收 transport 或 persistence details：

- `github.com/gin-gonic/gin`
- `github.com/aegiscore/user-service/ent`
- `github.com/aegiscore/user-service/ent/...`
- `github.com/redis/go-redis/v9`
- `github.com/aegiscore/common/http/binding`

如果 application 需要通用校验能力，应继续使用 `common/validation` 或 feature-local `application/validators`，而不是导入 HTTP binder。

### Transport HTTP

HTTP transport 可以依赖 Gin、application、feature-local HTTP DTO 和 response envelope，但不能直接访问 storage:

- `github.com/aegiscore/user-service/ent`
- `github.com/aegiscore/user-service/ent/...`
- `github.com/redis/go-redis/v9`
- `database/sql`
- `github.com/jackc/pgx/v5`
- `github.com/jackc/pgx/v5/...`

如 controller 需要数据，应映射为 command/query 并调用 application use case。

### Transport gRPC

未来 feature-local `transport/grpc` 的规则应与 HTTP transport 分离：

- `github.com/aegiscore/user-service/ent`
- `github.com/aegiscore/user-service/ent/...`
- `github.com/redis/go-redis/v9`
- `database/sql`
- `github.com/jackc/pgx/v5`
- `github.com/jackc/pgx/v5/...`
- `github.com/gin-gonic/gin`
- `github.com/aegiscore/common/http/response`
- `github.com/aegiscore/common/contract/response`
- `github.com/aegiscore/user-service/internal/integration`
- `github.com/aegiscore/user-service/internal/integration/...`

当前没有真实 gRPC API；规则可以先落地为空匹配保护 future boundary，不应新增 gRPC runtime、proto 或 generated code。

### Infrastructure

Feature infrastructure 可以实现 application ports 并访问 Ent/Redis/domain，但不应知道 HTTP transport:

- `github.com/gin-gonic/gin`
- `github.com/aegiscore/common/http/response`
- `github.com/aegiscore/common/contract/response`

这覆盖 `infrastructure/postgres`、`infrastructure/redis` 和未来 `infrastructure/consumers`。如果未来 consumers 需要更细规则，应在单独变更中扩展。

### Integration

`internal/integration/*` 是外部系统防腐层，不应返回 Gin response、直接使用服务持久化 adapter 或导入 Ent：

- `github.com/gin-gonic/gin`
- `github.com/aegiscore/common/http/response`
- `github.com/aegiscore/common/contract/response`
- `github.com/aegiscore/user-service/ent`
- `github.com/aegiscore/user-service/ent/...`
- `github.com/aegiscore/user-service/internal/features/*/infrastructure`
- `github.com/aegiscore/user-service/internal/features/*/infrastructure/...`

Integration adapter 如果需要服务能力，应实现消费侧 feature application port，而不是绕过 application 访问 persistence adapter。

## Historical Violations

本变更不应顺手清理所有既有 lint findings。实施时按以下方式处理：

1. 先运行 `golangci-lint config verify --config .golangci.yml`。
2. 再运行 `make lint-common` 和 `make lint-user-service`，或在 `user-service/` 中运行 `golangci-lint run ./...`。
3. 如果 depguard 发现历史违规，按层记录：
   - domain 违规：列出文件、违规 import、推荐迁移方向。
   - application 违规：列出是否应引入 port、mapper 或 validators。
   - transport 违规：列出应通过 command/query 访问的 use case。
   - infrastructure/integration 违规：列出应返回 domain/application error 而不是 HTTP response 的位置。
4. 只修复为了让新规则本身具备可运行基线所必需的最小问题；其他历史 lint findings 继续保留在分阶段治理清单。

治理清单可以放在 `docs/GO_LINT_AUTOMATION.md` 的存量问题章节，或在 implementation PR 描述中引用。若清单进入文档，应明确它是当前快照，后续清理后需要更新。

## Documentation Updates

`docs/GO_LINT_AUTOMATION.md` 需要新增或更新：

- 示例配置中的 `depguard` enable。
- “架构依赖边界自动化”小节，说明 depguard 和 `docs/ARCHITECTURE.md` dependency table 的对应关系。
- 本地验证命令：
  - `golangci-lint config verify --config .golangci.yml`
  - `make lint-common`
  - `make lint-user-service`
- 历史违规处理策略，强调本变更不一次性清理所有 lint findings。
- 常见排查：如果 depguard 报错，优先移动 import 到正确层，确有例外时必须先更新架构规则再调整 depguard。

不需要更新 `docs/ARCHITECTURE.md`，除非实施时发现现有 dependency table 与实际规则存在语义冲突。结构规则仍以 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 为准。

## Verification Strategy

- `golangci-lint config verify --config .golangci.yml`：确认 YAML schema 和 depguard settings 可读取。
- `golangci-lint linters --config .golangci.yml`：确认 `depguard` 出现在 enabled linters 中。
- `make lint-common`：确认 common module 仍可用根配置执行 lint。
- `make lint-user-service`：确认 user-service module 中 depguard 规则可执行。
- 如存在历史 lint findings，确认输出中能区分 depguard 违规和既有 gofmt/goimports/revive/staticcheck/unused findings。
- 检查 `docs/GO_LINT_AUTOMATION.md`，确认新增说明没有暗示 OpenSpec/OPSX 流程恢复。
- 检查 `git diff`，确认没有业务代码、Ent generated code、migration 或 Swagger 产物的无关改动。

## Risks

- Depguard pattern 写得过宽会误伤 provider、router 或 generated code。缓解方式是只匹配明确分层目录，并保留现有 generated-code exclusions。
- Pattern 写得过窄会漏掉 `domain/` 根部或未来子包。缓解方式是在实施中用 `rg` 对目标目录进行样本检查，并确保 root 和 nested packages 都被覆盖。
- 直接把所有 lint findings 作为 blocking gate 可能放大历史问题。缓解方式是保留当前分阶段治理策略，只把 depguard 作为新增架构边界约束。
