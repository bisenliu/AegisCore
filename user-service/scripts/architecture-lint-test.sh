#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture_root="$(mktemp -d)"
trap 'rm -rf "${fixture_root}"' EXIT

mkdir -p \
  "${fixture_root}/.github/workflows" \
  "${fixture_root}/common" \
  "${fixture_root}/deployments/docker" \
  "${fixture_root}/deployments/compose" \
  "${fixture_root}/docs/opsx" \
  "${fixture_root}/openspec/changes" \
  "${fixture_root}/openspec/specs" \
  "${fixture_root}/tools/openapi-convert" \
  "${fixture_root}/user-service/ent/schema" \
  "${fixture_root}/user-service/internal/features/auth/application" \
  "${fixture_root}/user-service/internal/features/role" \
  "${fixture_root}/user-service/internal/shared" \
  "${fixture_root}/user-service/migrations" \
  "${fixture_root}/user-service/scripts"

cat > "${fixture_root}/common/go.mod" <<'EOF'
module github.com/aegiscore/common

go 1.26.5
EOF

cat > "${fixture_root}/go.work" <<'EOF'
go 1.26.5

toolchain go1.26.5

use (
	./common
	./tools/openapi-convert
	./user-service
)
EOF

cat > "${fixture_root}/tools/openapi-convert/go.mod" <<'EOF'
module github.com/aegiscore/tools/openapi-convert

go 1.26.5
EOF

cat > "${fixture_root}/user-service/go.mod" <<'EOF'
module github.com/aegiscore/user-service

go 1.26.5
EOF

cat > "${fixture_root}/.github/workflows/ci.yml" <<'EOF'
env:
  GO_VERSION: '1.26.5'
  GOTOOLCHAIN: go1.26.5
EOF

cat > "${fixture_root}/.github/workflows/lint.yml" <<'EOF'
env:
  GO_VERSION: '1.26.5'
  GOTOOLCHAIN: go1.26.5
EOF

cat > "${fixture_root}/deployments/docker/atlas-postgres-pgtrgm.Dockerfile" <<'EOF'
FROM postgres:latest
EOF

cat > "${fixture_root}/user-service/scripts/migrate-diff.sh" <<'EOF'
ATLAS_DEV_IMAGE="aegiscore-atlas-postgres-pgtrgm:latest"
EOF

cat > "${fixture_root}/user-service/migrations/atlas.hcl" <<'EOF'
  default = "docker+postgres://_/aegiscore-atlas-postgres-pgtrgm:latest/dev?search_path=public"
EOF

cat > "${fixture_root}/deployments/compose/docker-compose.yml" <<'EOF'
services:
  postgres:
    image: postgres:latest
EOF

cat > "${fixture_root}/user-service/internal/features/auth/application/violating_import.go" <<'EOF'
package application

import _ "github.com/aegiscore/user-service/internal/features/auth/transport/http"
EOF

git -C "${fixture_root}" init -q

set +e
output="$(ARCHITECTURE_LINT_REPO_ROOT="${fixture_root}" "${script_dir}/architecture-lint.sh" 2>&1)"
status=$?
set -e

if [[ "${status}" -eq 0 ]]; then
  printf 'architecture-lint-test: expected fixture violation to fail\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"application/domain/infrastructure must not import feature HTTP transport DTO/controller packages"* ]]; then
  printf 'architecture-lint-test: expected layering violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" == *"rg execution failed"* ]]; then
  printf 'architecture-lint-test: unexpected rg execution failure\n%s\n' "${output}" >&2
  exit 1
fi

printf 'architecture-lint-test: ok\n'
