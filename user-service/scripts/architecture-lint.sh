#!/usr/bin/env bash
set -euo pipefail

repo_root="${ARCHITECTURE_LINT_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
service_dir="${repo_root}/user-service"
shopt -s nullglob

failures=0

report() {
  printf 'architecture-lint: %s\n' "$*" >&2
  failures=$((failures + 1))
}

run_rg() {
  local description="$1"
  local pattern="$2"
  shift 2
  if [[ "$#" -eq 0 ]]; then
    return
  fi

  local output status
  set +e
  output="$(rg -n --glob '*.go' "$pattern" "$@" 2>&1)"
  status=$?
  set -e

  case "${status}" in
    0)
      report "${description}"$'\n'"${output}"
      ;;
    1)
      return
      ;;
    *)
      report "${description}: rg execution failed"$'\n'"${output}"
      ;;
  esac
}

run_rg_any() {
  local description="$1"
  local pattern="$2"
  shift 2
  if [[ "$#" -eq 0 ]]; then
    return
  fi

  local output status
  set +e
  output="$(rg -n "$pattern" "$@" 2>&1)"
  status=$?
  set -e

  case "${status}" in
    0)
      report "${description}"$'\n'"${output}"
      ;;
    1)
      return
      ;;
    *)
      report "${description}: rg execution failed"$'\n'"${output}"
      ;;
  esac
}

go_mod_version() {
  local file="$1"
  awk '$1 == "go" { print $2; exit }' "${file}"
}

toolchain_version() {
  local file="$1"
  awk '$1 == "toolchain" { sub(/^go/, "", $2); print $2; exit }' "${file}"
}

workflow_go_version() {
  local file="$1"
  sed -nE "s/^[[:space:]]*GO_VERSION:[[:space:]]*['\"]?([0-9]+\\.[0-9]+\\.[0-9]+)['\"]?.*/\\1/p" "${file}" | head -n 1
}

workflow_gotoolchain_version() {
  local file="$1"
  sed -nE "s/^[[:space:]]*GOTOOLCHAIN:[[:space:]]*go([0-9]+\\.[0-9]+\\.[0-9]+).*/\\1/p" "${file}" | head -n 1
}

check_go_toolchain_version() {
  local expected
  expected="$(go_mod_version "${repo_root}/common/go.mod")"
  if [[ -z "${expected}" ]]; then
    report "common/go.mod missing go version"
    return
  fi

  local label version
  while IFS=$'\t' read -r label version; do
    if [[ -z "${version}" ]]; then
      report "${label} missing Go toolchain version; expected ${expected}"
    elif [[ "${version}" != "${expected}" ]]; then
      report "${label} has Go version ${version}; expected ${expected}"
    fi
  done < <(
    printf 'go.work go\t%s\n' "$(go_mod_version "${repo_root}/go.work")"
    printf 'go.work toolchain\t%s\n' "$(toolchain_version "${repo_root}/go.work")"
    for mod in common/go.mod user-service/go.mod tools/openapi-convert/go.mod; do
      printf '%s go\t%s\n' "${mod}" "$(go_mod_version "${repo_root}/${mod}")"
    done
    for workflow in .github/workflows/ci.yml .github/workflows/lint.yml; do
      printf '%s GO_VERSION\t%s\n' "${workflow}" "$(workflow_go_version "${repo_root}/${workflow}")"
      printf '%s GOTOOLCHAIN\t%s\n' "${workflow}" "$(workflow_gotoolchain_version "${repo_root}/${workflow}")"
    done
  )

  for mod in common/go.mod user-service/go.mod tools/openapi-convert/go.mod; do
    version="$(toolchain_version "${repo_root}/${mod}")"
    if [[ -n "${version}" && "${version}" != "${expected}" ]]; then
      report "${mod} toolchain has Go version ${version}; expected ${expected}"
    fi
  done
}

check_atlas_postgres_version() {
  # 当前本地交付配置默认跟随最新 PostgreSQL；正式环境建议固定具体版本或镜像 digest。
  local expected="latest"
  local dockerfile="${repo_root}/deployments/docker/atlas-postgres-pgtrgm.Dockerfile"
  local migrate_script="${service_dir}/scripts/migrate-diff.sh"
  local atlas_config="${service_dir}/migrations/atlas.hcl"
  local compose_file="${repo_root}/deployments/compose/docker-compose.yml"

  if ! rg -q "^FROM postgres:${expected}$" "${dockerfile}"; then
    report "Atlas pg_trgm Dockerfile must use postgres:${expected}"
  fi
  if ! rg -q "^ATLAS_DEV_IMAGE=\"aegiscore-atlas-postgres-pgtrgm:${expected}\"$" "${migrate_script}"; then
    report "migration diff script must build aegiscore-atlas-postgres-pgtrgm:${expected}"
  fi
  if ! rg -q "^[[:space:]]+default = \"docker\\+postgres://_/aegiscore-atlas-postgres-pgtrgm:${expected}/dev\\?search_path=public\"$" "${atlas_config}"; then
    report "Atlas dev URL must use aegiscore-atlas-postgres-pgtrgm:${expected}"
  fi
  if ! rg -q "^[[:space:]]+image: postgres:${expected}$" "${compose_file}"; then
    report "Compose PostgreSQL service must use postgres:${expected}"
  fi
}

check_go_toolchain_version
check_atlas_postgres_version

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
  "${service_dir}"/internal/features/*/application \
  "${service_dir}"/internal/features/*/domain \
  "${service_dir}"/internal/features/*/infrastructure

run_rg "gRPC transport must not import feature HTTP transport packages" \
  'github\.com/aegiscore/user-service/internal/features/.*/transport/http' \
  "${service_dir}"/internal/features/*/transport/grpc

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
