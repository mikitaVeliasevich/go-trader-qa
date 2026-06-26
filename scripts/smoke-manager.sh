#!/usr/bin/env bash
# Smoke-test Manager proxy endpoints for a single server_id.
# Usage: scripts/smoke-manager.sh [server_id]
# Requires: .env with MANAGER_API_BASE_URL, MANAGER_BEARER_TOKEN

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SERVER_ID="${1:-11}"

if [[ -f "$ROOT/.env" ]]; then
  # shellcheck disable=SC1091
  set -a && source "$ROOT/.env" && set +a
fi

: "${MANAGER_API_BASE_URL:?set MANAGER_API_BASE_URL in .env}"
: "${MANAGER_BEARER_TOKEN:?set MANAGER_BEARER_TOKEN in .env}"

BASE="${MANAGER_API_BASE_URL%/}/provision/servers/${SERVER_ID}"
AUTH="Authorization: Bearer ${MANAGER_BEARER_TOKEN}"

curl_common() {
  curl -sS "$@" \
    -H "$AUTH" \
    -H 'accept: */*' \
    -H 'content-type: application/json' \
    -H 'referer: https://staging.yatrade.org/' \
    -H 'user-agent: go-trader-qa-smoke/1.0'
}

check() {
  local path="$1"
  local out
  out="$(mktemp)"
  local code
  code="$(curl_common -o "$out" -w '%{http_code}' "${BASE}/${path}")"
  echo "=== GET /${path} HTTP ${code} ==="
  if command -v jq >/dev/null 2>&1; then
    jq '.' "$out" 2>/dev/null || head -c 400 "$out"
  else
    head -c 400 "$out"
  fi
  echo
  rm -f "$out"
}

echo "Manager smoke: server_id=${SERVER_ID}"
check status
check config
check 'logs?tail=3'
check debug/vars

echo "Expected: status/config/logs=200; debug/vars=200 with bus_drops, ws_messages_received, ..."
echo "If debug/vars returns 404 from bot, redeploy go-trader image with GET /debug/vars on :3228."
