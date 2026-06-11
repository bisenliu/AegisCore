# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本次只新增 role/permission future feature skeleton。
- [x] 查看 `user-service/internal/features/user` 和 `user-service/internal/features/auth` 的当前分层，确认不需要复制完整 feature 结构。
- [x] 确认当前不存在 `user-service/internal/features/role` 或 `user-service/internal/features/permission`，或若已存在则先审查其内容是否符合 skeleton 范围。

## Skeleton Directories

- [x] 新增 `user-service/internal/features/role/README.md` 或最小 `doc.go`，说明 role 是 future feature skeleton。
- [x] 新增 `user-service/internal/features/permission/README.md` 或最小 `doc.go`，说明 permission 是 future feature skeleton。
- [x] 不新增 `application/`、`domain/`、`transport/http/`、`infrastructure/` 或 `fx.go`，除非后续另有明确业务需求。
- [x] 不新增空 struct、空 interface、空 provider、空 route registration 或占位 use case。
- [x] 不新增横向 `internal/rbac`、`internal/authorization`、`internal/shared`、`internal/service` 或 `internal/repository`。

## Behavior And Data Guardrails

- [x] 不修改 auth 登录、刷新、强制改密、登出、token version 或 session lifecycle 逻辑。
- [x] 不修改 user 创建、查询、分页列表或用户资料 DTO。
- [x] 不注册 role/permission HTTP route。
- [x] 不新增授权中间件、permission check、role check 或 RBAC policy。
- [x] 不新增 Ent schema、Ent generated code、Atlas migration、数据库表、join table 或 seed data。
- [x] 不修改配置 key、响应 envelope、错误码、JWT claim 或 Redis key。

## Documentation

- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization，将 `role` 标注为 future feature skeleton。
- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization，将 `permission` 标注为 future feature skeleton。
- [x] 在 `docs/ARCHITECTURE.md` 中说明 role/permission 当前只有最小边界占位，不属于已实现 HTTP/API 能力。
- [x] 确认架构文档仍要求真实实现按 `application/`、`domain/`、`transport/http/`、`infrastructure/*` 和 `fx.go` 分层扩展。
- [x] 如 `AGENTS.md` 的 Repository Shape、Current Feature Areas 或 Key Entry Points 需要同步，补充 role/permission future skeleton 说明。
- [x] 确认长期规则文档仍声明不新增 `openspec/` 或 `docs/opsx/`。

## Formatting

- [x] 如果新增 Go `doc.go`，运行 `gofmt -w user-service/internal/features/role/doc.go user-service/internal/features/permission/doc.go`。
- [x] 如果只新增 README，无需 gofmt。

## Verification

- [x] 运行目录检查：

```bash
test -d user-service/internal/features/role
test -d user-service/internal/features/permission
find user-service/internal/features/role user-service/internal/features/permission -maxdepth 2 -type f -print | sort
```

- [x] 运行实现范围检查：

```bash
find user-service/internal/features/role user-service/internal/features/permission -type f \
  ! -name README.md ! -name doc.go -print
rg -n "RegisterRoutes|fx\\.Provide|ent/schema|migration|Authorization|permission check|role check" user-service/internal/features/role user-service/internal/features/permission
```

- [x] 运行文档检查：

```bash
rg -n "role|permission|RBAC|future feature skeleton" docs/ARCHITECTURE.md AGENTS.md
```

- [x] 如果新增任何 Go package doc，在 `user-service/` 下运行：

```bash
go test ./...
```

- [x] 如果只新增 Markdown 和长期文档，说明 Go 测试可选，因为没有 Go package graph 或运行时行为变更。
- [x] 检查 `git diff -- user-service/internal/features/role user-service/internal/features/permission docs/ARCHITECTURE.md AGENTS.md docs/changes/prepare-rbac-feature-skeleton`，确认没有 HTTP API、auth/user 行为、Ent schema、migration、generated code 或真实 RBAC 实现变更。

## Review Notes

- [x] 确认 role/permission 只是 future feature skeleton。
- [x] 确认没有注册路由或 Fx module。
- [x] 确认没有新增业务用例、ports、domain model 或 adapter。
- [x] 确认没有修改认证授权逻辑。
- [x] 确认没有新增数据库表或迁移。
- [x] 确认没有新增未使用 Go code。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
