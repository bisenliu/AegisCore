## 1. Initialism Audit

- [x] 1.1 Search `common/`, `user-services/`, `docs/`, `openspec/specs/`, and active `openspec/changes/` for non-canonical initialism spellings such as `UserId`, `Api`, `Http`, `Url`, `Json`, `Uuid`, `Jwt`, `Ttl`, and `Sql`.
- [x] 1.2 Classify each candidate as external contract, public Go API, internal Go API, docs/spec reference, generated code, tooling, or migration history.
- [x] 1.3 Identify protected names that must remain unchanged, including JSON tags, `user_id`, `session_id`, `token_version`, `X-Trace-ID`, config keys, env vars, Redis keys, database fields, module paths, generated Ent files, and migration files.

## 2. Low-risk Go Rename

- [x] 2.1 Rename low-risk internal Go identifiers to canonical initialism spelling, e.g. `UserID`, `API`, `HTTP`, `URL`, `JSON`, `UUID`, `JWT`, `TTL`, and `SQL`.
- [x] 2.2 Update all workspace Go references, imports, test names, table names, and helper names affected by the renames.
- [x] 2.3 Update godoc comment prefixes and nearby comments to match renamed exported identifiers without changing external field names.
- [x] 2.4 Keep Go package names short, lowercase, and import-path compatible; do not introduce uppercase package names for initialisms.
- [x] 2.5 Run `gofmt` on modified Go files.

## 3. Documentation and OpenSpec Synchronization

- [x] 3.1 Update docs and OpenSpec references to renamed internal Go symbols.
- [x] 3.2 Preserve external contract prose using existing external field/path/header spelling, such as `user_id`, `session_id`, `token_version`, and `X-Trace-ID`.
- [x] 3.3 Update related capability specs only where they reference renamed internal Go symbols.
- [x] 3.4 Confirm no manual edits were made under `user-services/ent/`, no Atlas migration files were renamed, and `atlas.sum` was not modified for this non-schema change.

## 4. Verification and Reporting

- [x] 4.1 Run `go test ./...` in `common/` if common Go identifiers changed.
- [x] 4.2 Run `go test ./...` in `user-services/` if user service Go identifiers changed.
- [x] 4.3 Run `golangci-lint run ./...` in affected modules if lint tooling is available and relevant to the modified files.
- [x] 4.4 Search again for old non-canonical spellings and document any retained results with protection reasons.
- [x] 4.5 Report modified symbols/files and confirm no HTTP API, JSON fields, config keys, database schema, generated code, or migration history changed.
