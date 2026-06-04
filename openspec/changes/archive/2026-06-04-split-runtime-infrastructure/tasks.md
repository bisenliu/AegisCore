## 1. Runtime Package Split

- [x] 1.1 Create `common/runtime/configfx` and move the Fx config provider there while keeping `common/runtime/config` free of Fx dependencies.
- [x] 1.2 Create `common/runtime/loggerfx` and move the Fx logger provider and stop-sync lifecycle there while preserving logger sync error handling.
- [x] 1.3 Create `common/runtime/datastore` and move Redis/PostgreSQL pure construction helpers there without importing Fx.
- [x] 1.4 Create `common/runtime/datastorefx` and move named Redis/PostgreSQL Fx providers plus ping/close lifecycle registration there.
- [x] 1.5 Create `common/runtime/resources` and move `NameUserDB`, `NameCommonDB` and `NameCacheRedis` constants there with the existing values.

## 2. Call Site Migration

- [x] 2.1 Update `user-services/internal/bootstrap` imports and provider calls to use `configfx`, `loggerfx`, `datastorefx` and `resources` instead of `common/runtime/infrastructure`.
- [x] 2.2 Update common tests to target the new package boundaries and preserve coverage for missing config, pool settings, lifecycle ping/close and opt-in named provider behavior.
- [x] 2.3 Search the workspace for remaining `common/runtime/infrastructure` imports or `commoninfra` aliases and migrate or remove them.
- [x] 2.4 Remove the old `common/runtime/infrastructure` package after all internal callers and tests are migrated.

## 3. Documentation And Specs

- [x] 3.1 Update `docs/opsx/CAPABILITY_MAP.md` shared-infrastructure code locations to list the new runtime packages.
- [x] 3.2 Update repository guidance or related docs if they still point maintainers to `common/runtime/infrastructure` for config, logger, datastore provider or resource names.

## 4. Verification

- [x] 4.1 Run `gofmt` on changed Go files.
- [x] 4.2 Run `go test ./...` in `common/` and fix any package split regressions.
- [x] 4.3 Run `go test ./...` in `user-services/` and fix any bootstrap wiring regressions.
- [x] 4.4 Verify `common/runtime/config`, `common/runtime/logger` and `common/runtime/datastore` do not import `go.uber.org/fx`.
