#!/usr/bin/env bash
#
# AegisCore user-service 真实指标压测脚本。
#
# 这个脚本面向本地 docker compose 环境，通过真实 HTTP 请求、PostgreSQL 写入、
# Redis policy version 操作和 Prometheus 查询生成并采样指标。它不是 mock exporter：
# 除 bootstrap 管理员账号为了可重复登录会直接写入数据库外，其余业务数据和失败路径
# 都尽量走 user-service 的真实 API 与运行时链路。
#
# 典型用法：
#   ./deployments/compose/scripts/generate-real-metrics-load.sh
#
# 输出文件：
#   results-<RUN_ID>.tsv             每次 HTTP 请求的时间戳、状态码、耗时、方法和路径
#   prometheus-samples-<RUN_ID>.txt  关键 Prometheus 查询快照
#   service-metrics-<RUN_ID>.prom    user-service /metrics 原始快照
#
# 注意：
#   - 脚本会创建 metrics-* 测试用户和角色，并复用代码基线权限；请只在本地或测试环境执行。
#   - scheduler 指标只有在运行中的服务注册了 scheduler job 时才会出现；脚本会查询并报告是否缺失。
#   - RBAC policy failure 指标通常需要真实 reload/publish 失败才会出现，脚本默认只触发安全的成功与版本补偿路径。
set -Eeuo pipefail

# ---------- 运行参数 ----------

BASE_URL="http://localhost:8080"
PROM_URL="http://localhost:9090"
COMPOSE_FILE="deployments/compose/docker-compose.yml"
SERVICE_NAME="user-service"
POSTGRES_SERVICE="postgres"
REDIS_SERVICE="redis"
POSTGRES_USER="postgres"
POSTGRES_DB="aegiscore"

DURATION="60"
CONCURRENCY="8"
MIN_REQUESTS="100"
MAX_ERROR_RATE_PERCENT="1.0"
MAX_P95_SECONDS="1.0"
CREATE_RATE="3"
STATUS_RATE="5"
AUTH_RATE="4"
BAD_RATE="3"
PROM_SCRAPE_INTERVAL="10"
PROM_SETTLE_SECONDS="20"
WARMUP_SECONDS="2"
RBAC_WATCHER_CHECK_WAIT_SECONDS="17"

BOOTSTRAP_USERNAME="metrics-admin"
BOOTSTRAP_PASSWORD="AegisCoreLoad123!"
BOOTSTRAP_NICKNAME="Metrics Admin"
BOOTSTRAP_USER_ID="00000000-0000-0000-0000-00000000a001"
SUPER_ADMIN_ROLE_ID="00000000-0000-0000-0000-000000000001"
RBAC_POLICY_CHANNEL="aegiscore-user-service:rbac:policy:refresh"

STATIC_PASSWORD_HASH="\$2y\$12\$0W17hTDYsMOjsS30IsyJ.u1gtJEJ6kZhpI86BpWR9dhj65Tsgy/42"

RUN_ID="$(date +%Y%m%d%H%M%S)"
ARTIFACT_DIR="/tmp/aegiscore-metrics-load"
RESULTS_FILE="$ARTIFACT_DIR/results-$RUN_ID.tsv"
PROM_SAMPLES_FILE="$ARTIFACT_DIR/prometheus-samples-$RUN_ID.txt"
SERVICE_METRICS_FILE="$ARTIFACT_DIR/service-metrics-$RUN_ID.prom"

# ---------- 运行中写入的状态 ----------

ADMIN_ACCESS_TOKEN=""
ADMIN_REFRESH_TOKEN=""
NORMAL_ACCESS_TOKEN=""
NORMAL_USER_ID=""
MUST_CHANGE_USERNAME=""
MUST_CHANGE_PASSWORD=""
DISABLED_USERNAME=""
DISABLED_PASSWORD=""
ROLE_ID=""
PERMISSION_ID=""

usage() {
  cat <<USAGE
AegisCore user-service 真实指标压测脚本

Usage:
  $(basename "$0") [--help]

Examples:
  ./deployments/compose/scripts/generate-real-metrics-load.sh
  DURATION=120 CONCURRENCY=16 ./deployments/compose/scripts/generate-real-metrics-load.sh
  DURATION=0 PROM_SETTLE_SECONDS=0 ./deployments/compose/scripts/generate-real-metrics-load.sh

Environment:
  BASE_URL                         user-service HTTP 地址，默认: $BASE_URL
  PROM_URL                         Prometheus 地址，默认: $PROM_URL
  COMPOSE_FILE                     docker compose 文件路径，默认: $COMPOSE_FILE
  DURATION                         并发流量持续秒数，默认: $DURATION
  CONCURRENCY                      并发 worker 数，默认: $CONCURRENCY
  MIN_REQUESTS                     最少 HTTP 请求数预算，默认: $MIN_REQUESTS
  MAX_ERROR_RATE_PERCENT           最大非预期 5xx/000 比例，默认: $MAX_ERROR_RATE_PERCENT
  MAX_P95_SECONDS                  最大 HTTP p95 耗时秒数，默认: $MAX_P95_SECONDS
  CREATE_RATE                      每 N 轮创建用户，默认: $CREATE_RATE
  STATUS_RATE                      每 N 轮启停角色，默认: $STATUS_RATE
  AUTH_RATE                        每 N 轮执行认证请求，默认: $AUTH_RATE
  BAD_RATE                         每 N 轮执行异常请求，默认: $BAD_RATE
  PROM_SETTLE_SECONDS              等待 Prometheus scrape 秒数，默认: $PROM_SETTLE_SECONDS
  RBAC_WATCHER_CHECK_WAIT_SECONDS  等待 RBAC watcher 补偿检查秒数，默认: $RBAC_WATCHER_CHECK_WAIT_SECONDS
  ARTIFACT_DIR                     输出目录，默认: $ARTIFACT_DIR

Artifacts:
  $RESULTS_FILE
  $PROM_SAMPLES_FILE
  $SERVICE_METRICS_FILE

Notes:
  - 请先启动本地 docker compose，并确认 user-service /readyz 可用。
  - 脚本会创建 metrics-* 测试数据，只建议在本地或测试环境运行。
  - scheduler 指标依赖服务端真实注册 scheduler job；若没有注册，脚本会报告 no_data/missing。
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if (($# > 0)); then
  printf 'ERROR: unsupported argument: %s\n\n' "$1" >&2
  usage >&2
  exit 2
fi

TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT
trap 'printf "ERROR: line %s failed: %s\n" "$LINENO" "$BASH_COMMAND" >&2' ERR

log() {
  printf '[%s] %s\n' "$(date '+%H:%M:%S')" "$*"
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

json_get() {
  local payload="$1"
  local filter="$2"
  jq -r "$filter // empty" <<<"$payload"
}

urlencode() {
  jq -rn --arg value "$1" '$value|@uri'
}

request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local token="${4:-}"
  local tag="${5:-$method $path}"
  local response_file status elapsed
  response_file="$(mktemp "$TMP_DIR/response.XXXXXX")"

  local args=(-sS -o "$response_file" -w '%{http_code} %{time_total}\n' -X "$method")
  args+=(-H "User-Agent: aegiscore-real-metrics-load/${RUN_ID}")
  if [[ -n "$body" ]]; then
    args+=(-H 'Content-Type: application/json' --data "$body")
  fi
  if [[ -n "$token" ]]; then
    args+=(-H "Authorization: Bearer $token")
  fi

  read -r status elapsed < <(curl "${args[@]}" "$BASE_URL$path" || printf '000 0')
  printf '%s\t%s\t%s\t%s\t%s\n' "$(date +%s)" "$status" "$elapsed" "$method" "$path" >>"$RESULTS_FILE"
  printf '%s\n' "$status"
  cat "$response_file"
  rm -f "$response_file"
  if [[ "$status" == "000" ]]; then
    log "request failed: $tag"
  fi
  return 0
}

# request 会把 HTTP status 写在第一行、响应体写在后续行；这两个 helper 用来拆分结果。
body_from_request_output() {
  tail -n +2
}

status_from_request_output() {
  head -n 1
}

expect_2xx() {
  local status="$1"
  local action="$2"
  [[ "$status" =~ ^2 ]] || fail "$action failed with HTTP $status"
}

preflight() {
  need_cmd curl
  need_cmd jq
  need_cmd docker

  mkdir -p "$(dirname "$RESULTS_FILE")" "$(dirname "$PROM_SAMPLES_FILE")" "$(dirname "$SERVICE_METRICS_FILE")"
  : >"$RESULTS_FILE"
  : >"$PROM_SAMPLES_FILE"
  : >"$SERVICE_METRICS_FILE"

  log "checking service readiness: $BASE_URL/readyz"
  local output status
  output="$(request GET /readyz '' '' preflight-readyz)"
  status="$(status_from_request_output <<<"$output")"
  expect_2xx "$status" "readyz"

  log "checking compose services from $COMPOSE_FILE"
  compose ps >/dev/null
}

# seed_bootstrap_user 直接写入固定管理员，保证任何本地数据状态下都能重新拿到 admin token。
# 密码 hash 对应 BOOTSTRAP_PASSWORD，避免脚本依赖注册接口或人工账号。
seed_bootstrap_user() {
  log "ensuring bootstrap user exists in PostgreSQL"
  compose exec -T "$POSTGRES_SERVICE" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 \
    -v user_id="$BOOTSTRAP_USER_ID" \
    -v nickname="$BOOTSTRAP_NICKNAME" \
    -v username="$BOOTSTRAP_USERNAME" \
    -v password_hash="$STATIC_PASSWORD_HASH" <<'SQL' >/dev/null
INSERT INTO users (user_id, nickname, username, password_hash, token_version, status, created_at, updated_at)
VALUES (:'user_id'::uuid, :'nickname', :'username', :'password_hash', 1, 100, floor(extract(epoch from now()) * 1000)::bigint, floor(extract(epoch from now()) * 1000)::bigint)
ON CONFLICT (username) DO UPDATE
SET nickname = EXCLUDED.nickname,
    password_hash = EXCLUDED.password_hash,
    status = 100,
    deleted_at = NULL,
    updated_at = EXCLUDED.updated_at;
SQL
}

# assign_super_admin 直接写入固定管理员角色绑定，避免压测脚本依赖已删除的旧 RBAC CLI。
assign_super_admin() {
  log "assigning super admin role to bootstrap user"
  compose exec -T "$POSTGRES_SERVICE" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 \
    -v user_id="$BOOTSTRAP_USER_ID" \
    -v role_id="$SUPER_ADMIN_ROLE_ID" <<'SQL' >/dev/null
INSERT INTO user_roles (user_id, role_id, created_at, updated_at)
SELECT u.id, r.id, floor(extract(epoch from now()) * 1000)::bigint, floor(extract(epoch from now()) * 1000)::bigint
FROM users u, roles r
WHERE u.user_id = :'user_id'::uuid AND r.role_id = :'role_id'::uuid
ON CONFLICT (user_id, role_id) DO NOTHING;
SQL
}

# create_rbac_policy_revision 写入数据库权威 revision，并返回新 revision。
create_rbac_policy_revision() {
  local reason="$1"
  compose exec -T "$POSTGRES_SERVICE" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -tA \
    -v reason="$reason" <<'SQL' | tr -d '[:space:]'
INSERT INTO rbac_policy_revisions (reason, created_at)
VALUES (:'reason', floor(extract(epoch from now()) * 1000)::bigint)
RETURNING revision;
SQL
}

# publish_rbac_reload 写入数据库 revision 并发布唤醒 hint，确保刚分配的角色立即生效。
publish_rbac_reload() {
  log "publishing RBAC policy refresh message for running service"
  local version payload
  version="$(create_rbac_policy_revision "metrics_bootstrap")"
  payload="$(jq -nc --argjson version "$version" \
    '{schema_version:1,event_id:"00000000-0000-0000-0000-090000000001",idempotency_key:("metrics-bootstrap:"+($version|tostring)),policy_revision:$version,kind:"policy_changed",reason:"metrics_bootstrap",publisher_instance_id:"metrics-load-script"}')"
  compose exec -T "$REDIS_SERVICE" redis-cli PUBLISH "$RBAC_POLICY_CHANNEL" "$payload" >/dev/null
  sleep "$WARMUP_SECONDS"
}

# login_user 返回登录响应体；调用方从 data.access_token / data.refresh_token 取 token。
login_user() {
  local username="$1"
  local password="$2"
  local output status body
  output="$(request POST /api/v1/auth/login "$(jq -nc --arg username "$username" --arg password "$password" '{username:$username,password:$password}')" '' "login-$username")"
  status="$(status_from_request_output <<<"$output")"
  body="$(body_from_request_output <<<"$output")"
  expect_2xx "$status" "login $username"
  printf '%s\n' "$body"
}

bootstrap_admin_tokens() {
  local body
  log "logging in as bootstrap admin"
  body="$(login_user "$BOOTSTRAP_USERNAME" "$BOOTSTRAP_PASSWORD")"
  ADMIN_ACCESS_TOKEN="$(json_get "$body" '.data.access_token')"
  ADMIN_REFRESH_TOKEN="$(json_get "$body" '.data.refresh_token')"
  [[ -n "$ADMIN_ACCESS_TOKEN" ]] || fail "admin access token missing"
}

# create_user/create_role 通过真实业务 API 创建压测数据，select_permission 从代码基线投影选择权限。
# create_user 遇到用户名冲突时会回查已有用户，便于重复执行同一个 RUN_ID。
create_user() {
  local username="$1"
  local nickname="$2"
  local password="$3"
  local status_value="$4"
  local output status body
  output="$(request POST /api/v1/users "$(jq -nc --arg nickname "$nickname" --arg username "$username" --arg password "$password" --argjson status "$status_value" '{nickname:$nickname,username:$username,password:$password,status:$status}')" "$ADMIN_ACCESS_TOKEN" "create-user-$username")"
  status="$(status_from_request_output <<<"$output")"
  body="$(body_from_request_output <<<"$output")"
  if [[ "$status" == "409" ]]; then
    output="$(request GET "/api/v1/users?username=$(urlencode "$username")&page_size=1" '' "$ADMIN_ACCESS_TOKEN" "find-user-$username")"
    body="$(body_from_request_output <<<"$output")"
    json_get "$body" '.data.items[0].user_id'
    return
  fi
  expect_2xx "$status" "create user $username"
  json_get "$body" '.data.user_id'
}

select_permission() {
  local output status body permission_id
  output="$(request GET "/api/v1/permissions?module=user&http_method=GET" '' "$ADMIN_ACCESS_TOKEN" "select-baseline-permission")"
  status="$(status_from_request_output <<<"$output")"
  body="$(body_from_request_output <<<"$output")"
  expect_2xx "$status" "select baseline permission"
  permission_id="$(json_get "$body" '.data.items[0].permission_id')"
  [[ -n "$permission_id" ]] || fail "baseline permission missing"
  printf '%s\n' "$permission_id"
}

create_role() {
  local suffix="$1"
  local output status body
  output="$(request POST /api/v1/roles "$(jq -nc --arg name "Load Role $suffix" \
    '{name:$name,description:"generated by real metrics load script",active:true,system:false}')" "$ADMIN_ACCESS_TOKEN" "create-role-$suffix")"
  status="$(status_from_request_output <<<"$output")"
  body="$(body_from_request_output <<<"$output")"
  expect_2xx "$status" "create role"
  json_get "$body" '.data.role_id'
}

bump_user_token_version() {
  local user_id="$1"
  compose exec -T "$POSTGRES_SERVICE" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -tA \
    -v user_id="$user_id" <<'SQL' | tr -d '[:space:]'
UPDATE users
SET token_version = token_version + 1,
    updated_at = floor(extract(epoch from now()) * 1000)::bigint
WHERE user_id = :'user_id'::uuid
RETURNING token_version;
SQL
}

cache_user_token_version() {
  local user_id="$1"
  local token_version="$2"
  compose exec -T "$REDIS_SERVICE" redis-cli SET "aegiscore-user-service:auth:user:token_version:{$user_id}" "$token_version" EX 60 >/dev/null
}

# prepare_business_data 准备后续压测需要的用户、角色和基线权限绑定。
prepare_business_data() {
  local suffix="metrics-$RUN_ID"
  local normal_username="metrics-user-$RUN_ID"
  local normal_password="NormalLoad123!"

  log "creating normal user and role, then selecting a baseline permission"
  NORMAL_USER_ID="$(create_user "$normal_username" "Metrics User $RUN_ID" "$normal_password" 100)"
  NORMAL_ACCESS_TOKEN="$(login_user "$normal_username" "$normal_password" | jq -r '.data.access_token // empty')"

  PERMISSION_ID="$(select_permission)"
  ROLE_ID="$(create_role "$suffix")"

  local output status
  output="$(request POST "/api/v1/roles/$ROLE_ID/permissions" "$(jq -nc --arg permission_id "$PERMISSION_ID" '{permission_id:$permission_id}')" "$ADMIN_ACCESS_TOKEN" "bind-role-permission")"
  status="$(status_from_request_output <<<"$output")"
  expect_2xx "$status" "bind role permission"
  output="$(request POST "/api/v1/users/$NORMAL_USER_ID/roles" "$(jq -nc --arg role_id "$ROLE_ID" '{role_id:$role_id}')" "$ADMIN_ACCESS_TOKEN" "bind-user-role")"
  status="$(status_from_request_output <<<"$output")"
  expect_2xx "$status" "bind user role"

  MUST_CHANGE_USERNAME="metrics-change-$RUN_ID"
  MUST_CHANGE_PASSWORD="InitialLoad123!"
  create_user "$MUST_CHANGE_USERNAME" "Metrics Change $RUN_ID" "$MUST_CHANGE_PASSWORD" 300 >/dev/null

  DISABLED_USERNAME="metrics-disabled-$RUN_ID"
  DISABLED_PASSWORD="DisabledLoad123!"
  create_user "$DISABLED_USERNAME" "Metrics Disabled $RUN_ID" "$DISABLED_PASSWORD" 200 >/dev/null
}

# exercise_auth_edges 生成认证链路的成功与失败指标，包括凭据错误、禁用用户、
# refresh token 无效、token_version mismatch、logout-all 后旧 token 失效等场景。
exercise_auth_edges() {
  log "generating auth success/failure and token-version metrics"
  request POST /api/v1/auth/login '{"username":"","password":""}' '' login-validation-failed >/dev/null
  request POST /api/v1/auth/login "$(jq -nc --arg username "$BOOTSTRAP_USERNAME" '{username:$username,password:"wrong-password"}')" '' login-credential-invalid >/dev/null
  request POST /api/v1/auth/login "$(jq -nc --arg username "$DISABLED_USERNAME" --arg password "$DISABLED_PASSWORD" '{username:$username,password:$password}')" '' login-user-status-rejected >/dev/null
  request POST /api/v1/auth/refresh '{"refresh_token":""}' '' refresh-validation-failed >/dev/null
  request POST /api/v1/auth/refresh '{"refresh_token":"not-a-refresh-token"}' '' refresh-invalid >/dev/null

  local change_body change_token new_password
  change_body="$(login_user "$MUST_CHANGE_USERNAME" "$MUST_CHANGE_PASSWORD")"
  change_token="$(json_get "$change_body" '.data.access_token')"
  new_password="ChangedLoad123!"
  request POST /api/v1/auth/change-password "$(jq -nc --arg password "$new_password" '{new_password:$password}')" "$change_token" change-password-success >/dev/null
  request POST /api/v1/auth/login "$(jq -nc --arg username "$MUST_CHANGE_USERNAME" --arg password "$MUST_CHANGE_PASSWORD" '{username:$username,password:$password}')" '' login-old-password-failure >/dev/null

  local mismatch_username mismatch_password mismatch_user_id mismatch_body mismatch_refresh mismatch_version
  mismatch_username="metrics-refresh-mismatch-$RUN_ID"
  mismatch_password="MismatchLoad123!"
  mismatch_user_id="$(create_user "$mismatch_username" "Metrics Refresh Mismatch $RUN_ID" "$mismatch_password" 100)"
  mismatch_body="$(login_user "$mismatch_username" "$mismatch_password")"
  mismatch_refresh="$(json_get "$mismatch_body" '.data.refresh_token')"
  mismatch_version="$(bump_user_token_version "$mismatch_user_id")"
  cache_user_token_version "$mismatch_user_id" "$mismatch_version"
  request POST /api/v1/auth/refresh "$(jq -nc --arg refresh_token "$mismatch_refresh" '{refresh_token:$refresh_token}')" '' refresh-token-version-mismatch >/dev/null

  local old_admin_token old_admin_refresh
  old_admin_token="$ADMIN_ACCESS_TOKEN"
  old_admin_refresh="$ADMIN_REFRESH_TOKEN"
  request POST /api/v1/auth/refresh "$(jq -nc --arg refresh_token "$ADMIN_REFRESH_TOKEN" '{refresh_token:$refresh_token}')" '' refresh-success >/dev/null
  request POST /api/v1/auth/logout-all '' "$ADMIN_ACCESS_TOKEN" logout-all-success >/dev/null
  sleep 1
  request GET /api/v1/users?page_size=1 '' "$old_admin_token" access-token-version-mismatch >/dev/null
  request POST /api/v1/auth/refresh "$(jq -nc --arg refresh_token "$old_admin_refresh" '{refresh_token:$refresh_token}')" '' refresh-session-invalid-after-logout-all >/dev/null

  bootstrap_admin_tokens
}

# trigger_rbac_watcher_revision_check 只提交数据库 policy revision，不发布 Pub/Sub hint。
# 运行中的 watcher 会在定时补偿检查中发现数据库 revision 差异，生成 watcher_revision_check 指标。
trigger_rbac_watcher_revision_check() {
  if (( RBAC_WATCHER_CHECK_WAIT_SECONDS <= 0 )); then
    return
  fi
  log "triggering RBAC watcher database revision-check mismatch"
  create_rbac_policy_revision "metrics_periodic_compensation" >/dev/null
  log "waiting ${RBAC_WATCHER_CHECK_WAIT_SECONDS}s for RBAC watcher revision check"
  sleep "$RBAC_WATCHER_CHECK_WAIT_SECONDS"
}

# traffic_worker 负责持续访问读接口、写接口和异常请求，覆盖 HTTP RED、RBAC、auth 和业务指标。
traffic_worker() {
  local worker_id="$1"
  local end_at="$2"
  local i=0
  while (( $(date +%s) < end_at )); do
    i=$((i + 1))
    request GET "/api/v1/users?page_size=20&nickname=Metrics" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-list-users" >/dev/null
    request GET "/api/v1/users/$NORMAL_USER_ID" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-get-user" >/dev/null
    request GET "/api/v1/roles?page_size=20&active=true" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-list-roles" >/dev/null
    request GET "/api/v1/roles/$ROLE_ID" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-get-role" >/dev/null
    request GET "/api/v1/roles/$ROLE_ID/permissions" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-role-permissions" >/dev/null
    request GET "/api/v1/permissions?module=user&http_method=GET" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-list-permissions" >/dev/null
    request GET "/api/v1/permissions/users/$NORMAL_USER_ID/effective" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-effective-permissions" >/dev/null

    if (( i % STATUS_RATE == 0 )); then
      request PATCH "/api/v1/roles/$ROLE_ID/status" '{"active":false}' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-role-disable" >/dev/null
      request PATCH "/api/v1/roles/$ROLE_ID/status" '{"active":true}' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-role-enable" >/dev/null
    fi

    if (( i % AUTH_RATE == 0 )); then
      request POST /api/v1/auth/login "$(jq -nc --arg username "$BOOTSTRAP_USERNAME" --arg password "$BOOTSTRAP_PASSWORD" '{username:$username,password:$password}')" '' "w${worker_id}-login-success" >/dev/null
      request POST /api/v1/auth/login "$(jq -nc --arg username "$BOOTSTRAP_USERNAME" '{username:$username,password:"bad-password"}')" '' "w${worker_id}-login-failure" >/dev/null
      request POST /api/v1/auth/logout '' "$NORMAL_ACCESS_TOKEN" "w${worker_id}-logout-current" >/dev/null
    fi

    if (( i % CREATE_RATE == 0 )); then
      local username="metrics-created-${RUN_ID}-${worker_id}-${i}"
      request POST /api/v1/users "$(jq -nc --arg nickname "Metrics Created $worker_id-$i" --arg username "$username" '{nickname:$nickname,username:$username,password:"CreatedLoad123!",status:100}')" "$ADMIN_ACCESS_TOKEN" "w${worker_id}-create-user" >/dev/null
    fi

    if (( i % BAD_RATE == 0 )); then
      request GET "/api/v1/users/$NORMAL_USER_ID" '' '' "w${worker_id}-missing-auth" >/dev/null
      request GET "/api/v1/users/not-a-uuid" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-bad-uri" >/dev/null
      request GET "/api/v1/not-found-$RUN_ID-$worker_id-$i" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-not-found" >/dev/null
      request GET /api/v1/permissions?http_method=not-a-method '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-bad-query" >/dev/null
      request GET /api/v1/permissions '' "$NORMAL_ACCESS_TOKEN" "w${worker_id}-rbac-denied" >/dev/null
    fi
  done
}

# scrape_worker 在压测期间主动访问 /metrics 和 Prometheus ready/query，让本地链路尽早产生活跃样本。
scrape_worker() {
  local end_at="$1"
  while (( $(date +%s) < end_at )); do
    curl -fsS "$BASE_URL/metrics" >/dev/null || true
    if curl -fsS "$PROM_URL/-/ready" >/dev/null 2>&1; then
      curl -fsS "$PROM_URL/api/v1/query?query=up" >/dev/null || true
    fi
    sleep "$PROM_SCRAPE_INTERVAL"
  done
}

# run_load 启动 scrape worker 和多个业务 worker，并等待全部压测请求结束。
run_load() {
  local end_at
  end_at=$(( $(date +%s) + DURATION ))
  log "running load for ${DURATION}s with concurrency=${CONCURRENCY}"
  scrape_worker "$end_at" &
  local scrape_pid="$!"

  local pids=()
  for worker in $(seq 1 "$CONCURRENCY"); do
    traffic_worker "$worker" "$end_at" &
    pids+=("$!")
  done

  local pid
  for pid in "${pids[@]}"; do
    wait "$pid"
  done
  wait "$scrape_pid" || true
}

# prom_query 查询 Prometheus instant API，并把前几条结果压缩成便于终端阅读的文本。
prom_query() {
  local query="$1"
  local encoded
  encoded="$(urlencode "$query")"
  curl -fsS "$PROM_URL/api/v1/query?query=$encoded" 2>/dev/null | jq -r '
    if .status != "success" then
      "query_failed"
    elif (.data.result | length) == 0 then
      "no_data"
    else
      [.data.result[] | ((.metric | to_entries | map(.key+"="+.value) | join(",")) + " " + .value[1])] | .[:8] | .[]
    end
  ' || printf 'prometheus_unavailable\n'
}

# wait_for_prometheus_scrape 给 Prometheus 留一个 scrape 周期，避免刚生成的指标还没入库。
wait_for_prometheus_scrape() {
  if (( PROM_SETTLE_SECONDS <= 0 )); then
    return
  fi
  if ! curl -fsS "$PROM_URL/-/ready" >/dev/null 2>&1; then
    return
  fi
  log "waiting ${PROM_SETTLE_SECONDS}s for Prometheus scrape"
  sleep "$PROM_SETTLE_SECONDS"
}

save_service_metrics_snapshot() {
  curl -fsS "$BASE_URL/metrics" >"$SERVICE_METRICS_FILE" || true
}

# report_service_metric_presence 基于服务端原始 /metrics 判断 collector 是否真实暴露。
# 这能区分 Prometheus 暂未 scrape 到数据和服务端根本没有注册该指标。
report_service_metric_presence() {
  log "service /metrics metric presence"
  local metrics=(
    aegiscore_user_service_auth_operations_total
    aegiscore_user_service_auth_token_version_mismatches_total
    aegiscore_user_service_auth_session_purge_submit_failures_total
    aegiscore_user_service_rbac_policy_sync_operations_total
    aegiscore_user_service_rbac_policy_version_mismatches_total
    aegiscore_user_service_rbac_enforce_total
    aegiscore_user_service_rbac_enforce_duration_seconds
    aegiscore_user_service_ent_query_duration_seconds
    aegiscore_user_service_ent_query_errors_total
    aegiscore_localcache_requests_total
    aegiscore_localcache_loads_total
    aegiscore_localcache_capacity_evictions_total
    aegiscore_localcache_capacity
    aegiscore_scheduler_jobs_total
    aegiscore_scheduler_job_duration_seconds
  )

  local metric state
  for metric in "${metrics[@]}"; do
    state="missing"
    if service_metric_present "$metric"; then
      state="present"
    fi
    printf '  %s %s\n' "$metric" "$state"
  done

  if ! grep -Eq '^aegiscore_scheduler_jobs_total(\{| |$)' "$SERVICE_METRICS_FILE"; then
    printf '  note scheduler metrics are absent from the running service; no scheduler jobs appear to be registered.\n'
  fi
  if ! service_metric_present aegiscore_user_service_ent_query_duration_seconds; then
    printf '  note Ent query metrics require ent.plugins.metrics.enabled=true and an enabled metrics provider.\n'
  fi
}

# enforce_performance_budget 使用客户端观测结果做本地可重复预算门禁。
enforce_performance_budget() {
  local summary count errors p95 error_rate
  summary="$(awk -F '\t' '
    {
      count++
      if ($2 == "000" || $2 ~ /^5/) {
        errors++
      }
    }
    END {
      if (count == 0) {
        print "ERROR: no HTTP request records were captured" > "/dev/stderr"
        exit 1
      }
      error_rate = errors * 100 / count
      printf "%d %d %.6f", count, errors, error_rate
    }
  ' "$RESULTS_FILE")"
  read -r count errors error_rate <<<"$summary"
  p95="$(awk -F '\t' '{ print $3 + 0 }' "$RESULTS_FILE" | sort -n | awk -v count="$count" 'BEGIN { p95_index = int(count * 0.95); if (p95_index < 1) p95_index = 1 } NR == p95_index { printf "%.6f", $1; exit }')"

  printf 'performance budget: requests=%d p95=%.3fs error_rate=%.2f%%\n' "$count" "$p95" "$error_rate"
  awk \
    -v count="$count" \
    -v p95="$p95" \
    -v error_rate="$error_rate" \
    -v min_requests="$MIN_REQUESTS" \
    -v max_error_rate="$MAX_ERROR_RATE_PERCENT" \
    -v max_p95="$MAX_P95_SECONDS" '
    BEGIN {
      if (count + 0 < min_requests + 0) {
        printf "ERROR: request count %d is below budget %d\n", count, min_requests > "/dev/stderr"
        exit 1
      }
      if (p95 + 0 > max_p95 + 0) {
        printf "ERROR: p95 %.3fs exceeds budget %.3fs\n", p95, max_p95 > "/dev/stderr"
        exit 1
      }
      if (error_rate + 0 > max_error_rate + 0) {
        printf "ERROR: error rate %.2f%% exceeds budget %.2f%%\n", error_rate, max_error_rate > "/dev/stderr"
        exit 1
      }
    }
  '
}

# histogram 在 /metrics 中会以 _bucket/_sum/_count 输出，所以单独兼容 duration 类指标。
service_metric_present() {
  local metric="$1"
  case "$metric" in
    aegiscore_scheduler_job_duration_seconds|aegiscore_user_service_rbac_enforce_duration_seconds|aegiscore_user_service_ent_query_duration_seconds)
      grep -Eq "^${metric}(_bucket|_sum|_count)(\\{| |$)|^# HELP ${metric}( |$)" "$SERVICE_METRICS_FILE"
      ;;
    *)
      grep -Eq "^${metric}(\\{| |$)" "$SERVICE_METRICS_FILE"
      ;;
  esac
}

# summarize_results 汇总 HTTP 状态码、关键 Prometheus 查询和服务端 metrics 暴露情况。
summarize_results() {
  log "HTTP status summary"
  awk -F '\t' '{count[$2]++} END {for (status in count) printf "  %s %d\n", status, count[status]}' "$RESULTS_FILE" | sort

  log "key Prometheus metric snapshots"
  local queries=(
    'sum by (method, route, status_class) (http_server_requests_total)'
    'sum by (operation, result, reason) (aegiscore_user_service_auth_operations_total)'
    'sum by (operation, reason) (aegiscore_user_service_auth_operations_total{result="failure"})'
    'sum by (source) (aegiscore_user_service_auth_token_version_mismatches_total)'
    'aegiscore_user_service_auth_session_purge_submit_failures_total'
    'sum by (operation, result, reason, source) (aegiscore_user_service_rbac_policy_sync_operations_total)'
    'sum by (operation, reason, source) (aegiscore_user_service_rbac_policy_sync_operations_total{result="failure"})'
    'sum by (method, route_template, result) (aegiscore_user_service_rbac_enforce_total)'
    'sum by (method, route_template, result) (aegiscore_user_service_rbac_enforce_duration_seconds_count)'
    'sum by (source) (aegiscore_user_service_rbac_policy_version_mismatches_total)'
    'sum by (status) (aegiscore_casbin_policy_reloads_total)'
    'sum by (entity, query, result) (aegiscore_user_service_ent_query_duration_seconds_count)'
    'sum by (entity, query, result) (rate(aegiscore_user_service_ent_query_duration_seconds_sum[5m])) / clamp_min(sum by (entity, query, result) (rate(aegiscore_user_service_ent_query_duration_seconds_count[5m])), 0.000001)'
    'histogram_quantile(0.95, sum by (le, entity, query, result) (rate(aegiscore_user_service_ent_query_duration_seconds_bucket[5m])))'
    'sum by (entity, query) (aegiscore_user_service_ent_query_errors_total)'
    'aegiscore_redis_up'
    'aegiscore_redis_ping_failures_total'
    'aegiscore_postgres_pool_open_connections'
    'sum by (cache, result) (aegiscore_localcache_requests_total)'
    'sum by (cache, result) (aegiscore_localcache_loads_total)'
    'sum by (cache) (aegiscore_localcache_capacity_evictions_total)'
    'aegiscore_localcache_capacity'
    'sum by (event) (aegiscore_workerpool_tasks_total)'
    'sum by (scheduler_job, event, status, reason) (aegiscore_scheduler_jobs_total)'
    'sum by (scheduler_job, status) (aegiscore_scheduler_job_duration_seconds_count)'
    'aegiscore_runtime_component_running'
    'go_goroutines'
    'process_resident_memory_bytes'
  )

  local query
  for query in "${queries[@]}"; do
    {
      printf '\n# %s\n' "$query"
      prom_query "$query"
    } | tee -a "$PROM_SAMPLES_FILE"
  done

  report_service_metric_presence
  enforce_performance_budget

  log "raw request records: $RESULTS_FILE"
  log "prometheus query snapshot: $PROM_SAMPLES_FILE"
  log "service /metrics snapshot: $SERVICE_METRICS_FILE"
}

main() {
  preflight
  seed_bootstrap_user
  assign_super_admin
  publish_rbac_reload
  bootstrap_admin_tokens
  prepare_business_data
  exercise_auth_edges
  trigger_rbac_watcher_revision_check
  run_load
  save_service_metrics_snapshot
  wait_for_prometheus_scrape
  summarize_results
  log "done"
}

main "$@"
