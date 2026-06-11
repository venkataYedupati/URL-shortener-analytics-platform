#!/usr/bin/env bash
set -euo pipefail

API_BASE="${API_BASE:-http://127.0.0.1:8080}"
CODE="smoke-$(date +%s)"

echo "Creating ${CODE}"
CREATE_RESPONSE="$(curl -sS -X POST "${API_BASE}/v1/links" \
  -H "Content-Type: application/json" \
  -d "{\"target_url\":\"https://example.com\",\"title\":\"Smoke test\",\"custom_code\":\"${CODE}\"}")"

SHORT_URL="$(printf '%s' "${CREATE_RESPONSE}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["short_url"])')"
echo "Short URL: ${SHORT_URL}"

echo "Triggering redirect"
curl -sS -o /dev/null -w "redirect_status=%{http_code}\n" \
  -H "X-Geo-Country: US" \
  -H "User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS)" \
  "${API_BASE}/${CODE}"

sleep 5

echo "Fetching analytics"
curl -sS "${API_BASE}/v1/links/${CODE}/analytics?hours=24"
echo
