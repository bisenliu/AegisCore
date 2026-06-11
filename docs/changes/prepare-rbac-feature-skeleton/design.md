# Design

## Overview

本变更只做 RBAC future feature skeleton：

```text
user-service/internal/features/role/
  README.md

user-service/internal/features/permission/
  README.md
```

`role` 和 `permission` 暂时不是已实现业务能力。它们是后续角色权限变更的归属边界，用来避免 RBAC 相关代码被提前塞入 `auth`、`user`、`internal/shared` 或横向 service/repository 目录。

## Feature Boundary

### `features/role`

未来用于角色相关能力，例如：

- 角色聚合与角色生命周期。
- 角色名称、状态、描述等领域模型。
- 角色与权限或用户关系的应用用例。
- 角色管理 HTTP API 和持久化 adapter。

本 skeleton 不创建上述代码。后续只有出现真实业务需求时，才按 feature 分层新增 `application/`、`domain/`、`transport/http/`、`infrastructure/*` 和 `fx.go`。

### `features/permission`

未来用于权限相关能力，例如：

- 权限定义、权限编码、权限分组和权限说明。
- 权限查询、校验规则和边界约束。
- 权限管理 HTTP API 和持久化 adapter。
- 与 role feature 的最小 application ports 或 domain 值对象协作。

本 skeleton 不创建上述代码。后续只有出现真实业务需求时，才按 feature 分层新增具体包和实现。

## Placeholder Format

首选 README 占位，而不是 Go package doc。README 不会进入 Go package graph，也不会制造空包被误认为已实现 feature。

如果实现者选择 `doc.go`，文件必须只包含 package comment 和 package declaration，例如：

```go
// Package role documents the future role feature boundary.
package role
```

不得在 skeleton 中定义空 interface、空 struct、provider、route registration、service constructor 或占位 use case。

## Relationship To Auth

Auth 仍只负责认证会话能力：

- 登录。
- 刷新。
- 强制改密。
- 退出当前设备。
- 退出全部设备。
- token version/session validation helper。

本变更不把 RBAC policy、role lookup、permission check 或 authorization middleware 加入 auth。后续如果 auth 需要消费 RBAC 能力，应另开变更设计清楚接口归属、调用方向、缓存策略和测试边界。

## Relationship To User

User 仍只负责用户资料创建、查询和分页列表。本变更不修改用户模型，也不向 user feature 添加 role 字段、permission 字段、join table、查询条件或 DTO 字段。

后续如果需要用户角色关系，应通过独立变更明确数据模型、迁移、application ports、adapter 和 API 行为。

## Relationship To Data Model

本变更不新增数据库结构：

- 不新增 Ent schema。
- 不生成 Ent code。
- 不新增 Atlas migration。
- 不新增 join table 或 seed data。

Role/permission 数据模型需要独立设计，因为它会影响数据库约束、查询路径、授权策略和可能的缓存策略。

## Documentation Updates

Update long-lived docs:

- `docs/ARCHITECTURE.md`
  - 在 Feature-First Organization 中把 `role` 和 `permission` 加为 future feature skeleton。
  - 明确它们当前只有边界占位，不属于当前稳定 HTTP/API 能力。
  - 保持现有分层规则：真实实现出现时仍按 `application/`、`domain/`、`transport/http/`、`infrastructure/*` 和 `fx.go` 扩展。
- `AGENTS.md`
  - 如入口规则需要同步，在 Repository Shape 或 Current Feature Areas 中加入相同说明。

不要新增 `openspec/` 或 `docs/opsx/`。

## Verification Strategy

Implementation should verify:

```bash
test -d user-service/internal/features/role
test -d user-service/internal/features/permission
find user-service/internal/features/role user-service/internal/features/permission -maxdepth 2 -type f -print | sort
rg -n "role|permission|RBAC|future feature skeleton" docs/ARCHITECTURE.md AGENTS.md
```

If only Markdown files are added, Go tests are optional because no Go package graph or runtime behavior changes.

If any `doc.go` files are added, run:

```bash
cd user-service
go test ./...
```

Also confirm no implementation surface was accidentally added:

```bash
find user-service/internal/features/role user-service/internal/features/permission -type f \
  ! -name README.md ! -name doc.go -print
rg -n "RegisterRoutes|fx\\.Provide|ent/schema|migration|Authorization|permission check|role check" user-service/internal/features/role user-service/internal/features/permission
```

## Risks And Mitigations

### Premature feature implementation

Risk: skeleton work quietly introduces routes, providers, schema, DTOs, ports or empty service types before requirements exist.

Mitigation: allow only README or package docs in this change. Put all real RBAC behavior behind a later change with explicit acceptance criteria.

### Boundary confusion with auth

Risk: RBAC is treated as part of auth and authorization logic starts mixing with login/session use cases.

Mitigation: document that auth remains authentication/session control. Any future auth-to-RBAC dependency needs a separate design covering ownership and dependency direction.

### Empty Go package drift

Risk: empty Go packages are mistaken for implemented feature modules and become a dumping ground.

Mitigation: prefer README placeholders. If package docs are used, keep them declaration-only and avoid `fx.go`, routes, interfaces, structs or constructors.
