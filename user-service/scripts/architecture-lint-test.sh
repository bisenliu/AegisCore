#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture_root="$(mktemp -d)"
trap 'rm -rf "${fixture_root}"' EXIT

mkdir -p \
  "${fixture_root}/.github/workflows" \
  "${fixture_root}/common" \
  "${fixture_root}/common/runtime/example" \
  "${fixture_root}/common/testing/example" \
  "${fixture_root}/deployments/docker" \
  "${fixture_root}/deployments/compose" \
  "${fixture_root}/docs/opsx" \
  "${fixture_root}/openspec/changes" \
  "${fixture_root}/openspec/specs" \
  "${fixture_root}/tools/openapi-convert" \
  "${fixture_root}/user-service/ent/schema" \
  "${fixture_root}/user-service/docs" \
  "${fixture_root}/user-service/internal/features/auth/application" \
  "${fixture_root}/user-service/internal/features/auth/infrastructure" \
  "${fixture_root}/user-service/internal/features/role" \
  "${fixture_root}/user-service/internal/features/role/infrastructure" \
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

cat > "${fixture_root}/common/runtime/example/mock_generate.go" <<'EOF'
package example

//go:generate go tool mockgen -destination=mock_test.go -package=example example.org/project Port
EOF

cat > "${fixture_root}/user-service/internal/features/auth/application/test_hook.go" <<'EOF'
package application

func setClockForTest() {}
EOF

cat > "${fixture_root}/user-service/internal/features/auth/application/fx_metadata.go" <<'EOF'
package application

import "go.uber.org/fx"

type Deps struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

cat > "${fixture_root}/user-service/internal/features/role/fx.go" <<'EOF'
package role

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

cat > "${fixture_root}/user-service/internal/features/role/infrastructure/store.go" <<'EOF'
package infrastructure

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

cat > "${fixture_root}/user-service/internal/features/auth/infrastructure/default_logger.go" <<'EOF'
package infrastructure

import (
	"context"

	"github.com/aegiscore/common/runtime/logger"
)

func useDefaultLogger() {
	logger.Info(context.Background(), "default logger dependency")
}
EOF

cat > "${fixture_root}/user-service/internal/features/auth/application/allowed_test.go" <<'EOF'
package application

func setRequestForTest() {}
EOF

cat > "${fixture_root}/common/testing/example/helper.go" <<'EOF'
package example

func NewStoreForTest() {}
EOF

cat > "${fixture_root}/user-service/ent/schema/generated.go" <<'EOF'
package schema

func testHookGenerated() {}
EOF

cat > "${fixture_root}/user-service/docs/openapi.go" <<'EOF'
package docs

func NewDocumentForTest() {}
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

if [[ "${output}" != *"mock_generate.go must use //go:build generate"* ]]; then
  printf 'architecture-lint-test: expected mock generate build tag violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"test-only symbol must not enter production Go files"* ]]; then
  printf 'architecture-lint-test: expected test-only production symbol violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"feature production code must not use package-level default logger as main-path dependency"* ]]; then
  printf 'architecture-lint-test: expected default logger dependency violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" != *"feature application/domain production code must not carry Fx DI metadata"* ]]; then
  printf 'architecture-lint-test: expected application/domain Fx metadata violation report\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" == *"allowed_test.go"* || "${output}" == *"common/testing/example/helper.go"* || "${output}" == *"user-service/ent/schema/generated.go"* || "${output}" == *"user-service/docs/openapi.go"* || "${output}" == *"user-service/internal/features/role/fx.go"* || "${output}" == *"user-service/internal/features/role/infrastructure/store.go"* ]]; then
  printf 'architecture-lint-test: excluded test or generated file produced a false positive\n%s\n' "${output}" >&2
  exit 1
fi

if [[ "${output}" == *"rg execution failed"* ]]; then
  printf 'architecture-lint-test: unexpected rg execution failure\n%s\n' "${output}" >&2
  exit 1
fi

printf 'architecture-lint-test: ok\n'
