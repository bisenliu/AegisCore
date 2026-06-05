## 1. Runtime Module Rename

- [x] 1.1 Rename `UserServiceModule` in `user-services/internal/bootstrap/app.go` to `AppModule`, a symbol that clearly expresses app/runtime composition scope.
- [x] 1.2 Update `NewApp(configPath)` and any direct references to use the renamed runtime module symbol.
- [x] 1.3 Confirm the Fx module string name `aegiscore-user-services`, provider set, invoke set, timezone module, validation module, runtime dependency declarations and HTTP server lifecycle wiring remain unchanged.

## 2. Verification Coverage

- [x] 2.1 Add or update a focused test/static assertion that fails if `UserServiceModule` remains in `user-services/internal/bootstrap/app.go`.
- [x] 2.2 Add or update verification that the replacement symbol uses runtime/composition-root semantics and keeps `NewApp` wired through the renamed module.
- [x] 2.3 Run `gofmt -w` on changed Go files.

## 3. Validation

- [x] 3.1 Run `go test ./...` in `user-services/` and resolve any compile or test failures caused by the rename.
- [x] 3.2 Confirm no Ent schema, generated Ent code, Atlas migration, HTTP route, API response, runtime config or CLI command changes were introduced.
