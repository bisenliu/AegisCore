## 1. Directory Migration

- [x] 1.1 Create target directories under `common/contract`, `common/runtime`, `common/http`, and `common/security`.
- [x] 1.2 Move `common/response` to `common/contract/response` while preserving package name `response` and exported API names.
- [x] 1.3 Move `common/config`, `common/logger`, `common/infrastructure`, and `common/timezone` under `common/runtime` while preserving package names and runtime behavior.
- [x] 1.4 Move `common/middleware` and `common/ginvalidation` under `common/http` while preserving package names and HTTP behavior.
- [x] 1.5 Move `common/auth` and `common/password` under `common/security` while preserving package names and credential behavior.
- [x] 1.6 Keep `common/validation` in place and verify it remains independent from Gin HTTP adapter code.

## 2. Import Synchronization

- [x] 2.1 Update all `common` module imports to use the new categorized paths.
- [x] 2.2 Update all `user-services` imports to use the new categorized paths.
- [x] 2.3 Update test imports in both modules and remove references to obsolete package paths.
- [x] 2.4 Search the workspace for old import paths such as `github.com/aegiscore/common/response`, `common/config`, `common/logger`, `common/infrastructure`, `common/middleware`, `common/ginvalidation`, `common/auth`, `common/password`, and `common/timezone`; resolve any remaining references.

## 3. Contract Preservation

- [x] 3.1 Verify response helpers still emit the same `success`, `code`, `message`, `data`, and `errors` JSON fields and existing business codes.
- [x] 3.2 Verify `config.Load` still uses the same YAML keys, `AEGISCORE_` environment overrides, Redis/PostgreSQL named instances, and no required/range validation behavior.
- [x] 3.3 Verify logger and middleware still use `X-Trace-ID`, Gin trace context, Go context trace propagation, and log field `trace-id` consistently.
- [x] 3.4 Verify Redis/PostgreSQL providers still create only explicitly declared named instances and preserve `cache_redis`, `user_db`, and `common_db` Fx names.
- [x] 3.5 Verify auth and password helpers keep JWT claim semantics, Bearer constants, auth context helpers, Argon2id hash format, and constant-time verification behavior.
- [x] 3.6 Verify Gin validation adapter still returns compatible validation failure envelopes and `common/validation` remains reusable without Gin context.

## 4. Documentation And Specs

- [x] 4.1 Update `docs/ARCHITECTURE.md` with the categorized `common` structure and shared capability boundaries.
- [x] 4.2 Update `docs/DEVELOPMENT.md` with new import paths and rules for adding shared code to `common`.
- [x] 4.3 Update `docs/opsx/CAPABILITY_MAP.md` paths for `shared-infrastructure`, `api-response-contract`, `request-validation`, `common-credentials`, and the new `common-module-organization` capability.
- [x] 4.4 Update existing `openspec/specs/*/spec.md` path references affected by the directory migration.
- [x] 4.5 Ensure docs explicitly state that `common` is for stable shared contracts and infrastructure, not service-specific convenience helpers.

## 5. Validation

- [x] 5.1 Run `gofmt` on moved or import-updated Go files.
- [x] 5.2 Run `go test ./...` in `common/` and fix compile or behavior regressions.
- [x] 5.3 Run `go test ./...` in `user-services/` and fix compile or behavior regressions.
- [x] 5.4 Confirm no Ent schema, generated Ent code, Atlas migration files, YAML config keys, HTTP route paths, JSON response fields, or database schema artifacts were changed.
- [x] 5.5 Run `openspec status --change refactor-common-directory-structure` and confirm the change remains apply-ready or complete after implementation artifacts are updated.
