## 1. Role Permission Store Optimization

- [x] 1.1 Update `permissionsByExternalIDs` to build a deduplicated `[]uuid.UUID` before querying.
- [x] 1.2 Replace per-ID `getPermissionByExternalID` calls with one Ent query using `entpermission.PermissionIDIn(uniqueIDs...)`.
- [x] 1.3 Build a `map[uuid.UUID]*ent.Permission` from query results and return permissions in deduplicated input order.
- [x] 1.4 Return the existing role-permission not found domain error when any requested permission ID is missing.
- [x] 1.5 Preserve empty input behavior by returning an empty slice without querying.

## 2. Tests

- [x] 2.1 Add or update role PostgreSQL seed adapter tests for successful multi-permission batch resolution.
- [x] 2.2 Add coverage for duplicate permission IDs in seed input.
- [x] 2.3 Add coverage for missing permission IDs and assert the existing not found error semantics.
- [x] 2.4 Ensure existing `EnsureSystemBindings` and `SyncSystemBindings` tests still pass without behavior changes.

## 3. Verification

- [x] 3.1 Run `go test ./...` under `user-service/`.
- [x] 3.2 Run the relevant architecture or lint command if import boundaries change.
- [x] 3.3 Confirm no Ent generated files, migrations, OpenAPI artifacts, `openspec/`, or `docs/opsx/` files were created.
