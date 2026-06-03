## 1. Shared Config Naming

- [x] 1.1 Rename `common/config.Config.PostgresConfigs` to `Postgres` while preserving `mapstructure:"postgres"`.
- [x] 1.2 Update the PostgreSQL config helper and all repository references from `PostgresConfigs` to `Postgres`.
- [x] 1.3 Update `common` and `user-services` tests/fixtures that construct or inspect PostgreSQL configuration maps.

## 2. Authentication TTL Constants

- [x] 2.1 Add package-level default TTL constants in `user-services/internal/service` for Access Token, Refresh Token, token version cache, and any remaining auth session TTL fallback.
- [x] 2.2 Replace method-body magic TTL values in `auth_service.go` and `session_store.go` with the named constants without changing `<= 0` fallback logic.
- [x] 2.3 Adjust or add service tests to verify default TTL values and explicit TTL config precedence remain unchanged.

## 3. Validation

- [x] 3.1 Run `gofmt` on modified Go files.
- [x] 3.2 Run `go test ./...` in `common/`.
- [x] 3.3 Run `go test ./...` in `user-services/`.
- [x] 3.4 Confirm `openspec status --change "normalize-config-and-token-ttl"` reports the change artifacts as apply-ready.
