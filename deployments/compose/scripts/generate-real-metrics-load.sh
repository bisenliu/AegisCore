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
# 调整压测强度：
#   DURATION=120 CONCURRENCY=16 ./deployments/compose/scripts/generate-real-metrics-load.sh
#
# 只快速生成边界指标并跳过 Prometheus 等待：
#   DURATION=0 PROM_SETTLE_SECONDS=0 ./deployments/compose/scripts/generate-real-metrics-load.sh
#
# 常用环境变量：
#   BASE_URL                         user-service HTTP 地址，默认 http://localhost:8080
#   PROM_URL                         Prometheus 地址，默认 http://localhost:9090
#   COMPOSE_FILE                     docker compose 文件路径
#   DURATION                         并发流量持续秒数
#   CONCURRENCY                      并发 worker 数
#   CREATE_RATE/STATUS_RATE          每 N 轮执行创建和启停类写操作
#   AUTH_RATE/BAD_RATE               每 N 轮执行认证和异常请求
#   PROM_SETTLE_SECONDS              请求结束后等待 Prometheus scrape 的秒数
#   RBAC_WATCHER_CHECK_WAIT_SECONDS  触发 RBAC watcher 补偿检查后的等待秒数；设为 0 可跳过
#   ARTIFACT_DIR                     输出目录，默认 /tmp/aegiscore-metrics-load
#
# 输出文件：
#   results-<RUN_ID>.tsv             每次 HTTP 请求的时间戳、状态码、耗时、方法和路径
#   prometheus-samples-<RUN_ID>.txt  关键 Prometheus 查询快照
#   service-metrics-<RUN_ID>.prom    user-service /metrics 原始快照
#
# 注意：
#   - 脚本会创建 metrics-* 测试用户、角色和权限；请只在本地或测试环境执行。
#   - scheduler 指标只有在运行中的服务注册了 scheduler job 时才会出现；脚本会查询并报告是否缺失。
#   - RBAC policy failure 指标通常需要真实 reload/publish 失败才会出现，脚本默认只触发安全的成功与版本补偿路径。
set -Eeuo pipefail

# ---------- 可通过环境变量覆盖的运行参数 ----------

BASE_URL="${BASE_URL:-http://localhost:8080}"
PROM_URL="${PROM_URL:-http://localhost:9090}"
COMPOSE_FILE="${COMPOSE_FILE:-deployments/compose/docker-compose.yml}"
SERVICE_NAME="${SERVICE_NAME:-user-service}"
POSTGRES_SERVICE="${POSTGRES_SERVICE:-postgres}"
REDIS_SERVICE="${REDIS_SERVICE:-redis}"
POSTGRES_USER="${POSTGRES_USER:-aegiscore}"
POSTGRES_DB="${POSTGRES_DB:-aegiscore_user}"

DURATION="${DURATION:-60}"
CONCURRENCY="${CONCURRENCY:-8}"
CREATE_RATE="${CREATE_RATE:-3}"
STATUS_RATE="${STATUS_RATE:-5}"
AUTH_RATE="${AUTH_RATE:-4}"
BAD_RATE="${BAD_RATE:-3}"
PROM_SCRAPE_INTERVAL="${PROM_SCRAPE_INTERVAL:-10}"
PROM_SETTLE_SECONDS="${PROM_SETTLE_SECONDS:-20}"
WARMUP_SECONDS="${WARMUP_SECONDS:-2}"
RBAC_WATCHER_CHECK_WAIT_SECONDS="${RBAC_WATCHER_CHECK_WAIT_SECONDS:-17}"

BOOTSTRAP_USERNAME="${BOOTSTRAP_USERNAME:-metrics-admin}"
BOOTSTRAP_PASSWORD="${BOOTSTRAP_PASSWORD:-AegisCoreLoad123!}"
BOOTSTRAP_NICKNAME="${BOOTSTRAP_NICKNAME:-Metrics Admin}"
BOOTSTRAP_USER_ID="${BOOTSTRAP_USER_ID:-00000000-0000-0000-0000-00000000a001}"
SUPER_ADMIN_ROLE_ID="${SUPER_ADMIN_ROLE_ID:-00000000-0000-0000-0000-000000000001}"
RBAC_POLICY_VERSION_KEY="${RBAC_POLICY_VERSION_KEY:-aegiscore-user-services:rbac:policy:version}"
RBAC_POLICY_CHANNEL="${RBAC_POLICY_CHANNEL:-aegiscore-user-services:rbac:policy:refresh}"

STATIC_PASSWORD_HASH="${STATIC_PASSWORD_HASH:-\$argon2id\$v=19\$m=65536,t=3,p=4\$PysvHcWpamCZAwybuk5j8w\$0whWdpZhQyNuFUNQZ1HEKp9nsByqXIHxsHc1Xh03o20}"

RUN_ID="${RUN_ID:-$(date +%Y%m%d%H%M%S)}"
ARTIFACT_DIR="${ARTIFACT_DIR:-/tmp/aegiscore-metrics-load}"
RESULTS_FILE="${RESULTS_FILE:-$ARTIFACT_DIR/results-$RUN_ID.tsv}"
PROM_SAMPLES_FILE="${PROM_SAMPLES_FILE:-$ARTIFACT_DIR/prometheus-samples-$RUN_ID.txt}"
SERVICE_METRICS_FILE="${SERVICE_METRICS_FILE:-$ARTIFACT_DIR/service-metrics-$RUN_ID.prom}"

# ---------- 运行中写入的状态 ----------

ADMIN_ACCESS_TOKEN=""
ADMIN_REFRESH_TOKEN=""
NORMAL_ACCESS_TOKEN=""
NORMAL_REFRESH_TOKEN=""
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
  CREATE_RATE                      每 N 轮创建用户和权限，默认: $CREATE_RATE
  STATUS_RATE                      每 N 轮启停角色和权限，默认: $STATUS_RATE
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

# assign_super_admin 使用服务容器内的 RBAC CLI，为 bootstrap 用户补齐授权。
assign_super_admin() {
  log "assigning super admin role to bootstrap user"
  compose exec -T "$SERVICE_NAME" /app/user-service/bin/user-services \
    rbac --config /app/user-service/configs/config.yaml \
    assign-super-admin --user-id "$BOOTSTRAP_USER_ID" >/dev/null
}

# publish_rbac_reload 触发运行中实例刷新 Casbin policy，确保刚分配的角色立即生效。
publish_rbac_reload() {
  log "publishing RBAC policy refresh message for running service"
  local version payload
  version="$(compose exec -T "$REDIS_SERVICE" redis-cli INCR "$RBAC_POLICY_VERSION_KEY" | tr -d '\r')"
  payload="$(jq -nc --argjson version "$version" --arg reason "metrics_bootstrap" \
    '{version:$version,instance_id:"metrics-load-script",reason:$reason,published_at:now|floor}')"
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

# create_user/create_permission/create_role 都通过真实业务 API 创建压测数据。
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

create_permission() {
  local suffix="$1"
  local output status body
  output="$(request POST /api/v1/permissions "$(jq -nc --arg name "Load Permission $suffix" --arg path "/api/v1/load/$suffix" \
    '{name:$name,description:"generated by real metrics load script",module:"load",http_method:"GET",path_template:$path,active:true,system:false}')" "$ADMIN_ACCESS_TOKEN" "create-permission-$suffix")"
  status="$(status_from_request_output <<<"$output")"
  body="$(body_from_request_output <<<"$output")"
  expect_2xx "$status" "create permission"
  json_get "$body" '.data.permission_id'
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
  compose exec -T "$REDIS_SERVICE" redis-cli SET "aegiscore-user-services:auth:user:token_version:{$user_id}" "$token_version" EX 60 >/dev/null
}

# prepare_business_data 准备后续压测需要的正常用户、禁用用户、强制改密用户、角色和权限。
prepare_business_data() {
  local suffix="metrics-$RUN_ID"
  local normal_username="metrics-user-$RUN_ID"
  local normal_password="NormalLoad123!"

  log "creating normal user, permission, and role through real APIs"
  NORMAL_USER_ID="$(create_user "$normal_username" "Metrics User $RUN_ID" "$normal_password" 100)"
  NORMAL_ACCESS_TOKEN="$(login_user "$normal_username" "$normal_password" | jq -r '.data.access_token // empty')"
  NORMAL_REFRESH_TOKEN="$(login_user "$normal_username" "$normal_password" | jq -r '.data.refresh_token // empty')"

  PERMISSION_ID="$(create_permission "$suffix")"
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

# trigger_rbac_watcher_version_check 只递增 Redis policy version，不发布 Pub/Sub 消息。
# 运行中的 watcher 会在定时补偿检查中发现版本差异，生成 watcher_version_check 指标。
trigger_rbac_watcher_version_check() {
  if (( RBAC_WATCHER_CHECK_WAIT_SECONDS <= 0 )); then
    return
  fi
  log "triggering RBAC watcher version-check mismatch"
  compose exec -T "$REDIS_SERVICE" redis-cli INCR "$RBAC_POLICY_VERSION_KEY" >/dev/null
  log "waiting ${RBAC_WATCHER_CHECK_WAIT_SECONDS}s for RBAC watcher version check"
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
    request GET "/api/v1/permissions?page_size=20&module=load&active=true" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-list-permissions" >/dev/null
    request GET "/api/v1/permissions/$PERMISSION_ID" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-get-permission" >/dev/null
    request GET "/api/v1/permissions/users/$NORMAL_USER_ID/effective" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-effective-permissions" >/dev/null

    if (( i % STATUS_RATE == 0 )); then
      request PATCH "/api/v1/roles/$ROLE_ID/status" '{"active":false}' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-role-disable" >/dev/null
      request PATCH "/api/v1/roles/$ROLE_ID/status" '{"active":true}' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-role-enable" >/dev/null
      request POST "/api/v1/permissions/$PERMISSION_ID/disable" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-permission-disable" >/dev/null
      request POST "/api/v1/permissions/$PERMISSION_ID/enable" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-permission-enable" >/dev/null
    fi

    if (( i % AUTH_RATE == 0 )); then
      request POST /api/v1/auth/login "$(jq -nc --arg username "$BOOTSTRAP_USERNAME" --arg password "$BOOTSTRAP_PASSWORD" '{username:$username,password:$password}')" '' "w${worker_id}-login-success" >/dev/null
      request POST /api/v1/auth/login "$(jq -nc --arg username "$BOOTSTRAP_USERNAME" '{username:$username,password:"bad-password"}')" '' "w${worker_id}-login-failure" >/dev/null
      request POST /api/v1/auth/logout '' "$NORMAL_ACCESS_TOKEN" "w${worker_id}-logout-current" >/dev/null
    fi

    if (( i % CREATE_RATE == 0 )); then
      local username="metrics-created-${RUN_ID}-${worker_id}-${i}"
      request POST /api/v1/users "$(jq -nc --arg nickname "Metrics Created $worker_id-$i" --arg username "$username" '{nickname:$nickname,username:$username,password:"CreatedLoad123!",status:100}')" "$ADMIN_ACCESS_TOKEN" "w${worker_id}-create-user" >/dev/null
      request POST /api/v1/permissions "$(jq -nc --arg name "Load Dynamic $worker_id-$i" --arg path "/api/v1/load/${RUN_ID}/${worker_id}/${i}" \
        '{name:$name,description:"generated during load",module:"load",http_method:"GET",path_template:$path,active:true,system:false}')" "$ADMIN_ACCESS_TOKEN" "w${worker_id}-create-permission" >/dev/null
    fi

    if (( i % BAD_RATE == 0 )); then
      request GET "/api/v1/users/$NORMAL_USER_ID" '' '' "w${worker_id}-missing-auth" >/dev/null
      request GET "/api/v1/users/not-a-uuid" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-bad-uri" >/dev/null
      request GET "/api/v1/not-found-$RUN_ID-$worker_id-$i" '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-not-found" >/dev/null
      request GET /api/v1/permissions?active=not-a-bool '' "$ADMIN_ACCESS_TOKEN" "w${worker_id}-bad-query" >/dev/null
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
    aegiscore_user_service_permission_route_diff
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
}

# histogram 在 /metrics 中会以 _bucket/_sum/_count 输出，所以单独兼容 scheduler duration。
service_metric_present() {
  local metric="$1"
  case "$metric" in
    aegiscore_scheduler_job_duration_seconds)
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
    'sum by (source) (aegiscore_user_service_rbac_policy_version_mismatches_total)'
    'sum by (kind) (aegiscore_user_service_permission_route_diff)'
    'sum by (status) (aegiscore_casbin_policy_reloads_total)'
    'aegiscore_redis_up'
    'aegiscore_redis_ping_failures_total'
    'aegiscore_postgres_pool_open_connections'
    'sum by (event) (aegiscore_workerpool_tasks_total)'
    'sum by (exported_job, event, status, reason) (aegiscore_scheduler_jobs_total)'
    'sum by (exported_job, status) (aegiscore_scheduler_job_duration_seconds_count)'
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
  request GET /api/v1/permissions/route-diff '' "$ADMIN_ACCESS_TOKEN" route-diff-initial >/dev/null
  exercise_auth_edges
  trigger_rbac_watcher_version_check
  run_load
  request GET /api/v1/permissions/route-diff '' "$ADMIN_ACCESS_TOKEN" route-diff-final >/dev/null
  save_service_metrics_snapshot
  wait_for_prometheus_scrape
  summarize_results
  log "done"
}

main "$@"
