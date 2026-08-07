#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COVERAGE_DIR="${COVERAGE_DIR:-$ROOT_DIR/coverage}"
COMMON_MIN_COVERAGE="${COMMON_MIN_COVERAGE:-75.0}"
USER_SERVICE_HANDWRITTEN_MIN_COVERAGE="${USER_SERVICE_HANDWRITTEN_MIN_COVERAGE:-75.0}"
CHANGED_CODE_MIN_COVERAGE="${CHANGED_CODE_MIN_COVERAGE:-80.0}"

mkdir -p "$COVERAGE_DIR"

coverage_total() {
  local report_file="$1"
  awk '/^total:/ { sub(/%$/, "", $3); print $3 }' "$report_file"
}

check_threshold() {
  local name="$1"
  local actual="$2"
  local minimum="$3"

  awk -v name="$name" -v actual="$actual" -v minimum="$minimum" '
    BEGIN {
      if (actual + 0 < minimum + 0) {
        printf "ERROR: %s coverage %.1f%% is below baseline %.1f%%\n", name, actual, minimum > "/dev/stderr"
        exit 1
      }
      printf "%s coverage %.1f%% meets baseline %.1f%%\n", name, actual, minimum
    }
  '
}

filter_user_service_handwritten_profile() {
  local source_profile="$1"
  local target_profile="$2"

  awk '
    NR == 1 { print; next }
    /\/docs\/openapi\.go:/ { next }
    /\/internal\/persistence\/ent\// && !/\/internal\/persistence\/ent\/schema\// { next }
    { print }
  ' "$source_profile" > "$target_profile"
}

is_handwritten_go_file() {
  local file="$1"

  [[ "$file" == *.go ]] || return 1
  [[ "$file" != *_test.go ]] || return 1
  [[ "$file" != user-service/docs/openapi.go ]] || return 1
  if [[ "$file" == user-service/internal/persistence/ent/* && "$file" != user-service/internal/persistence/ent/schema/* ]]; then
    return 1
  fi
  [[ "$file" == common/* || "$file" == user-service/* ]]
}

changed_coverage_base_ref() {
  if [[ -n "${CHANGED_COVERAGE_BASE:-}" ]]; then
    printf '%s\n' "$CHANGED_COVERAGE_BASE"
    return 0
  fi
  if [[ -n "${GITHUB_BASE_REF:-}" ]]; then
    printf 'origin/%s\n' "$GITHUB_BASE_REF"
    return 0
  fi
  return 1
}

generate_changed_lines() {
  local base_ref="$1"
  local output_file="$2"
  local diff_file
  diff_file="$(mktemp)"

  git -C "$ROOT_DIR" diff --unified=0 --diff-filter=ACMR "$base_ref"...HEAD -- common user-service > "$diff_file"
  perl -ne '
    if (/^\+\+\+ b\/(.+)$/) { $file = $1; next }
    if (/^@@ .*\+(\d+)(?:,(\d+))? /) { $line = $1; $remaining = defined($2) ? $2 : 1; next }
    if (defined($file) && $remaining > 0 && /^\+/ && !/^\+\+\+/) { print "$file $line\n"; $line++; $remaining--; next }
    if (defined($file) && $remaining > 0 && !/^-/) { $line++; $remaining-- }
  ' "$diff_file" | while read -r file line; do
    if is_handwritten_go_file "$file"; then
      printf '%s %s\n' "$file" "$line"
    fi
  done > "$output_file"
  rm -f "$diff_file"
}

check_changed_code_coverage() {
  local base_ref changed_lines_file profile_file changed_total changed_covered changed_total_file coverage
  if ! base_ref="$(changed_coverage_base_ref)"; then
    printf 'changed-code coverage skipped because CHANGED_COVERAGE_BASE or GITHUB_BASE_REF is not set.\n'
    return 0
  fi
  if ! git -C "$ROOT_DIR" rev-parse --verify "$base_ref" >/dev/null 2>&1; then
    printf 'changed-code coverage skipped because base ref %s is unavailable.\n' "$base_ref"
    return 0
  fi

  changed_lines_file="$(mktemp)"
  profile_file="$(mktemp)"
  generate_changed_lines "$base_ref" "$changed_lines_file"
  cat "$common_profile" "$user_service_handwritten_profile" > "$profile_file"

  changed_total_file="$(mktemp)"
  awk '
    FNR == NR {
      changed[$1, $2] = 1
      next
    }
    /^mode:/ { next }
    {
      split($1, location, ":")
      file = location[1]
      sub(/^github\.com\/aegiscore\//, "", file)
      split(location[2], span, ",")
      split(span[1], start, ".")
      split(span[2], finish, ".")
      count = $3 + 0
      for (line = start[1] + 0; line <= finish[1] + 0; line++) {
        key = file SUBSEP line
        if (key in changed) {
          executable[key] = 1
          if (count > 0) {
            covered[key] = 1
          }
        }
      }
    }
    END {
      for (key in executable) {
        total++
        if (key in covered) {
          hit++
        }
      }
      printf "%d %d\n", total, hit
    }
  ' "$changed_lines_file" "$profile_file" > "$changed_total_file"
  read -r changed_total changed_covered < "$changed_total_file"
  rm -f "$changed_lines_file" "$profile_file" "$changed_total_file"

  if [[ "$changed_total" == "0" ]]; then
    printf 'changed-code coverage skipped because no changed executable handwritten Go lines were found.\n'
    return 0
  fi
  coverage="$(awk -v covered="$changed_covered" -v total="$changed_total" 'BEGIN { printf "%.1f", covered * 100 / total }')"
  check_threshold changed-code "$coverage" "$CHANGED_CODE_MIN_COVERAGE"
}

run_module_coverage() {
  local module_dir="$1"
  local profile_file="$2"
  local report_file="$3"

  (
    cd "$ROOT_DIR/$module_dir"
    go test -coverprofile="$profile_file" ./...
    go tool cover -func="$profile_file" > "$report_file"
  )
}

common_profile="$COVERAGE_DIR/common.out"
common_report="$COVERAGE_DIR/common.txt"
user_service_profile="$COVERAGE_DIR/user-service.out"
user_service_report="$COVERAGE_DIR/user-service.txt"
user_service_handwritten_profile="$COVERAGE_DIR/user-service-handwritten.out"
user_service_handwritten_report="$COVERAGE_DIR/user-service-handwritten.txt"

run_module_coverage common "$common_profile" "$common_report"
run_module_coverage user-service "$user_service_profile" "$user_service_report"
filter_user_service_handwritten_profile "$user_service_profile" "$user_service_handwritten_profile"
(
  cd "$ROOT_DIR/user-service"
  go tool cover -func="$user_service_handwritten_profile" > "$user_service_handwritten_report"
)

common_total="$(coverage_total "$common_report")"
user_service_total="$(coverage_total "$user_service_report")"
user_service_handwritten_total="$(coverage_total "$user_service_handwritten_report")"

printf 'user-service raw coverage %.1f%% is reported for visibility only; generated Ent/OpenAPI code is excluded from the handwritten gate.\n' "$user_service_total"
check_threshold common "$common_total" "$COMMON_MIN_COVERAGE"
check_threshold user-service-handwritten "$user_service_handwritten_total" "$USER_SERVICE_HANDWRITTEN_MIN_COVERAGE"
check_changed_code_coverage
