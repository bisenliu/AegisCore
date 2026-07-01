#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
service_dir="${repo_root}/user-service"

failures=0

report() {
  printf 'architecture-lint: %s\n' "$*" >&2
  failures=$((failures + 1))
}

run_rg() {
  local description="$1"
  local pattern="$2"
  shift 2
  local output
  if output="$(rg -n --glob '*.go' "$pattern" "$@" 2>/dev/null)"; then
    report "${description}"$'\n'"${output}"
  fi
}

run_rg_any() {
  local description="$1"
  local pattern="$2"
  shift 2
  local output
  if output="$(rg -n "$pattern" "$@" 2>/dev/null)"; then
    report "${description}"$'\n'"${output}"
  fi
}

if [[ -d "${service_dir}/internal/features/permission/application/rbacbaseline" ]]; then
  report "old permission/application/rbacbaseline package still exists"
fi

for forbidden_mock_dir in \
  "${repo_root}/mocks" \
  "${repo_root}/testmocks" \
  "${repo_root}/common/mocks" \
  "${repo_root}/common/testmocks" \
  "${service_dir}/mocks" \
  "${service_dir}/testmocks"; do
  if [[ -d "${forbidden_mock_dir}" ]]; then
    report "central mock repository is forbidden: ${forbidden_mock_dir}; put generated mocks next to the consuming feature tests"
  fi
done

run_rg "old RBAC baseline import remains" \
  'github\.com/aegiscore/user-service/internal/features/permission/application/rbacbaseline' \
  "${service_dir}/internal"

run_rg "old user-domain status reference remains" \
  'userdomain\.UserStatus|github\.com/aegiscore/user-service/internal/features/user/domain.*UserStatus' \
  "${service_dir}/internal/features" "${service_dir}/ent/schema"

if [[ -e "${service_dir}/internal/features/user/domain/user_status.go" ]]; then
  report "old user/domain/user_status.go still exists; use internal/shared/identity"
fi

run_rg "auth feature must not import user domain" \
  'github\.com/aegiscore/user-service/internal/features/user/domain' \
  "${service_dir}/internal/features/auth"

run_rg "role feature must not import user domain; use shared/identity for user identity errors" \
  'github\.com/aegiscore/user-service/internal/features/user/domain' \
  "${service_dir}/internal/features/role"

run_rg "shared packages must not import feature packages" \
  'github\.com/aegiscore/user-service/internal/features/' \
  "${service_dir}/internal/shared"

for forbidden_shared_dir in errors enums types utils helpers; do
  if [[ -d "${service_dir}/internal/shared/${forbidden_shared_dir}" ]]; then
    report "internal/shared/${forbidden_shared_dir} is a forbidden root-level catch-all package; put shared errors/enums/types in the owning shared kernel package"
  fi
done

if [[ -e "${service_dir}/internal/shared/identity/status.go" ]]; then
  report "internal/shared/identity/status.go should be named user_status.go to keep shared enum files subject-specific"
fi

run_rg "shared packages must not import forbidden runtime or transport dependencies" \
  'github\.com/gin-gonic/gin|github\.com/aegiscore/user-service/ent(/|")|github\.com/redis/go-redis|database/sql|github\.com/jackc/pgx|go\.uber\.org/fx|github\.com/aegiscore/common/http/response|github\.com/aegiscore/common/contract/response|github\.com/aegiscore/common/runtime/(config|logger|datastore)' \
  "${service_dir}/internal/shared"

run_rg "application/domain/infrastructure must not import feature HTTP transport DTO/controller packages" \
  'github\.com/aegiscore/user-service/internal/features/.*/transport/http' \
  "${service_dir}/internal/features/*/application" \
  "${service_dir}/internal/features/*/domain" \
  "${service_dir}/internal/features/*/infrastructure"

run_rg "gRPC transport must not import feature HTTP transport packages" \
  'github\.com/aegiscore/user-service/internal/features/.*/transport/http' \
  "${service_dir}/internal/features/*/transport/grpc"

run_rg_any "OpenAPI generated files have uncommitted drift" \
  '^user-service/docs/(openapi\.go|openapi\.json|openapi\.yaml)$' \
  <(cd "${repo_root}" && git diff --name-only -- user-service/docs/openapi.go user-service/docs/openapi.json user-service/docs/openapi.yaml)

run_rg_any "OpenSpec/OPSX markdown must use Simplified Chinese instead of default English templates" \
  '(<!--|-->|What problem does this solve|Describe what will change|brief description|condition|expected outcome|Overview|Summary|Acceptance Criteria|Migration Notes|Implementation Plan|Rollout Plan|Out of Scope)' \
  "${repo_root}/openspec/specs" \
  "${repo_root}/openspec/changes" \
  "${repo_root}/docs/opsx"

run_rg_any "Ent generated files have uncommitted drift; run make generate and commit generated output" \
  '^user-service/ent/(client|ent|mutation|runtime|tx|user|user_create|user_delete|user_query|user_update|permission|permission_create|permission_delete|permission_query|permission_update|role|role_create|role_delete|role_query|role_update|rolepermission|rolepermission_create|rolepermission_delete|rolepermission_query|rolepermission_update|userrole|userrole_create|userrole_delete|userrole_query|userrole_update)\.go$|^user-service/ent/(enttest|hook|migrate|predicate|runtime|user|permission|role|rolepermission|userrole)/' \
  <(cd "${repo_root}" && git diff --name-only -- user-service/ent)

if [[ "${failures}" -gt 0 ]]; then
  printf 'architecture-lint: failed with %d issue(s)\n' "${failures}" >&2
  exit 1
fi

printf 'architecture-lint: ok\n'
