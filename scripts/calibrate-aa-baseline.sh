#!/usr/bin/env bash
# Calibrate the gateway's AA baseline ratio for the current region.
#
# Use this once after enabling AA_API_KEY in production: run a token
# verification task against a token bound to a direct-official upstream
# channel, then read aa_ttft_ratio_avg as the "geo offset" baseline for
# this gateway. Future tokens whose ratio sits within ±15% of this value
# are normal; anything significantly higher warrants investigation.
#
# Requires: bash, curl, jq, awk.

set -euo pipefail

GATEWAY="${GATEWAY:-http://127.0.0.1:3000}"
TOKEN_ID="${TOKEN_ID:-}"
ACCESS_TOKEN="${ACCESS_TOKEN:-}"
USER_ID="${USER_ID:-}"
MODELS="${MODELS:-gpt-4o,gpt-4o-mini,claude-3-5-haiku-latest}"
PROVIDERS="${PROVIDERS:-openai,anthropic}"
POLL_INTERVAL_SEC="${POLL_INTERVAL_SEC:-3}"
MAX_WAIT_SEC="${MAX_WAIT_SEC:-300}"

usage() {
  cat <<EOF
Usage: $0 [OPTIONS]

Required (flag or env var):
  --gateway URL          Gateway base URL (env GATEWAY, default http://127.0.0.1:3000)
  --token-id N           DB id of the token to verify (env TOKEN_ID)
  --access-token TOKEN   User access token from GET /api/user/token (env ACCESS_TOKEN)
  --user-id ID           Numeric user id (env USER_ID)

Optional:
  --models LIST          Comma-separated, default "${MODELS}"
  --providers LIST       Comma-separated, default "${PROVIDERS}"
  --max-wait SEC         Max poll seconds, default ${MAX_WAIT_SEC}
  -h, --help             Show this help

Example:
  $0 --gateway https://api.example.com \\
     --token-id 42 \\
     --access-token aaa-bbb-ccc \\
     --user-id 7
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --gateway)      GATEWAY="$2"; shift 2 ;;
    --token-id)     TOKEN_ID="$2"; shift 2 ;;
    --access-token) ACCESS_TOKEN="$2"; shift 2 ;;
    --user-id)      USER_ID="$2"; shift 2 ;;
    --models)       MODELS="$2"; shift 2 ;;
    --providers)    PROVIDERS="$2"; shift 2 ;;
    --max-wait)     MAX_WAIT_SEC="$2"; shift 2 ;;
    -h|--help)      usage; exit 0 ;;
    *) echo "Unknown arg: $1" >&2; usage; exit 1 ;;
  esac
done

err=0
[[ -z "$TOKEN_ID"     ]] && { echo "Missing --token-id"     >&2; err=1; }
[[ -z "$ACCESS_TOKEN" ]] && { echo "Missing --access-token" >&2; err=1; }
[[ -z "$USER_ID"      ]] && { echo "Missing --user-id"      >&2; err=1; }
[[ $err -ne 0 ]] && { usage; exit 1; }

for cmd in curl jq awk; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "$cmd is required" >&2; exit 1; }
done

models_json=$(printf '%s' "$MODELS"    | jq -Rc 'split(",") | map(gsub("^\\s+|\\s+$"; ""))')
providers_json=$(printf '%s' "$PROVIDERS" | jq -Rc 'split(",") | map(gsub("^\\s+|\\s+$"; ""))')

body=$(jq -nc \
  --argjson token_id "$TOKEN_ID" \
  --argjson models "$models_json" \
  --argjson providers "$providers_json" \
  '{token_id: $token_id, models: $models, providers: $providers}')

echo ">>> Creating verification task..."
echo "    gateway:   $GATEWAY"
echo "    token_id:  $TOKEN_ID"
echo "    models:    $MODELS"
echo "    providers: $PROVIDERS"

create_resp=$(curl -sS -X POST "$GATEWAY/api/token_verification/tasks" \
  -H "Authorization: $ACCESS_TOKEN" \
  -H "HermesToken-User: $USER_ID" \
  -H "Content-Type: application/json" \
  -d "$body")

if [[ "$(jq -r '.success' <<<"$create_resp")" != "true" ]]; then
  echo "Create failed: $(jq -r '.message // "unknown"' <<<"$create_resp")" >&2
  exit 1
fi
task_id=$(jq -r '.data.id' <<<"$create_resp")
echo ">>> Task created: id=$task_id"

elapsed=0
detail=""
while :; do
  detail=$(curl -sS "$GATEWAY/api/token_verification/tasks/$task_id" \
    -H "Authorization: $ACCESS_TOKEN" \
    -H "HermesToken-User: $USER_ID")
  if [[ "$(jq -r '.success' <<<"$detail")" != "true" ]]; then
    echo "Poll failed: $(jq -r '.message // "unknown"' <<<"$detail")" >&2
    exit 1
  fi
  status=$(jq -r '.data.task.status' <<<"$detail")
  case "$status" in
    success) break ;;
    failed)
      reason=$(jq -r '.data.task.fail_reason // "(no reason)"' <<<"$detail")
      echo "Task failed: $reason" >&2
      exit 1
      ;;
    pending|running)
      echo "    [${elapsed}s] status=$status, waiting..."
      sleep "$POLL_INTERVAL_SEC"
      elapsed=$((elapsed + POLL_INTERVAL_SEC))
      if (( elapsed >= MAX_WAIT_SEC )); then
        echo "Timeout after ${MAX_WAIT_SEC}s" >&2
        exit 1
      fi
      ;;
    *)
      echo "Unexpected status: $status" >&2
      exit 1
      ;;
  esac
done

echo ""
echo "Per-model ratios:"
jq -r '
  .data.report.models[]?
  | select(.baseline != null and .baseline.source == "artificial_analysis")
  | "  \(.provider)/\(.model_name):"
    + "\n    TTFT measured=\(.stream_ttft_ms // 0)ms  baseline=\(.baseline.baseline_ttft_ms)ms  ratio=\((.baseline.ttft_ratio // 0) * 1000 | round / 1000)"
    + "\n    TPS  measured=\(((.stream_tokens_ps // 0) * 100 | round) / 100)  baseline=\((.baseline.baseline_tokens_ps * 100 | round) / 100)  ratio=\((.baseline.tps_ratio // 0) * 1000 | round / 1000)"
' <<<"$detail"

baseline_source=$(jq -r '.data.report.baseline_source' <<<"$detail")
ttft_avg=$(jq -r '.data.report.metrics.aa_ttft_ratio_avg // empty' <<<"$detail")
tps_avg=$(jq -r '.data.report.metrics.aa_tps_ratio_avg // empty'  <<<"$detail")
score=$(jq -r '.data.report.score' <<<"$detail")
grade=$(jq -r '.data.report.grade' <<<"$detail")

echo ""
echo "================================"
echo " Calibration Result"
echo "================================"
echo " task_id:               $task_id"
echo " score:                 $score ($grade)"
echo " baseline_source:       $baseline_source"
[[ -n "$ttft_avg" ]] && printf " aa_ttft_ratio_avg:     %.3f\n" "$ttft_avg"
[[ -n "$tps_avg"  ]] && printf " aa_tps_ratio_avg:      %.3f\n" "$tps_avg"
echo "================================"

if [[ "$baseline_source" != "artificial_analysis" && "$baseline_source" != "mixed" ]]; then
  cat <<EOF >&2

WARNING: baseline_source is "$baseline_source" — calibration NOT meaningful.
Possible causes:
  - AA_API_KEY not configured on the gateway
  - Refresh task has not run yet (look for "AA baseline refreshed: models=N" in logs)
  - All tested models are missing from AA's catalog
EOF
  exit 2
fi

if [[ -n "$ttft_avg" ]]; then
  low=$(awk -v r="$ttft_avg" 'BEGIN{printf "%.2f", r*0.85}')
  high=$(awk -v r="$ttft_avg" 'BEGIN{printf "%.2f", r*1.15}')
  alert=$(awk -v r="$ttft_avg" 'BEGIN{printf "%.2f", r*1.60}')
  cat <<EOF

Suggested calibration baseline for this gateway:
  Your region's TTFT offset relative to AA us-central1 ≈ ${ttft_avg}x.

  Future tokens with aa_ttft_ratio_avg in:
    [${low} .. ${high}]    --> normal (offset ±15%)
    > ${alert}             --> investigate (likely upstream issue, not geo)

Record this number per gateway region. If you later spin up a new region,
re-run this script there.
EOF
fi
