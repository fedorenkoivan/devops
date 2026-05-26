#!/usr/bin/env bash
# verify.sh
# Runs on the runner after deployment.
# Usage: bash verify.sh <base_url>
#   e.g. bash verify.sh http://192.168.1.100
set -euo pipefail

BASE_URL="${1:?Usage: verify.sh <base_url>}"
BASE_URL="${BASE_URL%/}"  # strip trailing slash

FAILURES=0

check_status() {
  local desc="$1"
  local url="$2"
  local expected="$3"

  local actual
  actual=$(curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 10 "$url")
  if [[ "$actual" == "$expected" ]]; then
    echo "  OK  [$actual] $desc ($url)"
  else
    echo "FAIL  [got $actual, want $expected] $desc ($url)" >&2
    FAILURES=$(( FAILURES + 1 ))
  fi
}

echo "=== Verifying deployment at $BASE_URL ==="

check_status "GET / returns 200"            "${BASE_URL}/"             "200"

check_status "GET /items returns 200"       "${BASE_URL}/items"        "200"

check_status "GET /health/alive returns 404" "${BASE_URL}/health/alive" "404"
check_status "GET /health/ready returns 404" "${BASE_URL}/health/ready" "404"

CT=$(curl -s -o /dev/null -w '%{content_type}' --connect-timeout 5 "${BASE_URL}/items")
if echo "$CT" | grep -q "application/json"; then
  echo "  OK  Content-Type on /items is JSON ($CT)"
else
  echo "FAIL  Content-Type on /items is not JSON: $CT" >&2
  FAILURES=$(( FAILURES + 1 ))
fi

echo ""
if [[ "$FAILURES" -eq 0 ]]; then
  echo "All checks passed."
  exit 0
else
  echo "${FAILURES} check(s) FAILED." >&2
  exit 1
fi
