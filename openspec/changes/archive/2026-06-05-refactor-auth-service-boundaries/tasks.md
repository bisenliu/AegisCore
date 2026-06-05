## 1. Component Boundary Setup

- [x] 1.1 Add a credential verifier component in `user-services/internal/service` that owns username lookup, password verification and invalid credential mapping.
- [x] 1.2 Add a token issuer/verifier component in `user-services/internal/service` that owns JWT TTL fallback, token signing and Refresh/Password-Change claims parsing.
- [x] 1.3 Add a session manager component in `user-services/internal/service` that owns Redis auth session creation, validation, deletion, rotation support and token version cache invalidation.

## 2. AuthService Refactor

- [x] 2.1 Refactor `AuthService` construction to compose the new components from existing Fx dependencies without changing controller or router injection.
- [x] 2.2 Refactor login flow so `AuthService` orchestrates credential verification, status decision, token issuance and session creation without directly implementing those strategies.
- [x] 2.3 Refactor password change flow so `AuthService` delegates password-change token parsing and token_version validation while keeping user state validation and credential update orchestration.
- [x] 2.4 Refactor refresh flow so `AuthService` delegates Refresh Token claims parsing, session validation, rotation session deletion and new token/session creation.
- [x] 2.5 Refactor logout and logout-all flows so Redis session deletion and token version cache invalidation are delegated while PostgreSQL token_version increment remains in `AuthService` orchestration.

## 3. Compatibility and Tests

- [x] 3.1 Preserve existing HTTP routes, DTOs, response errors, Redis key behavior, token claims and token_version semantics.
- [x] 3.2 Update or add service-layer tests for the refactored login, refresh, password-change, logout and logout-all flows.
- [x] 3.3 Add focused unit coverage for credential verifier, token issuer/verifier or session manager where behavior is no longer covered by `AuthService` tests.
- [x] 3.4 Run `gofmt` on modified Go files.
- [x] 3.5 Run `go test ./...` in `user-services/` and address regressions.
