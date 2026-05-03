#!/usr/bin/env bash
# Layer-2 end-to-end calibration runner.
#
# Drives the token verification API with real tokens, evaluates per-case
# expectations against the returned report, and emits a markdown summary.
# Layer-1 (in-process scoring logic) lives in TestCalibrationMatrix; this
# script complements it by exercising the full HTTP path against a real
# gateway with real upstream credentials.
#
# Requires: bash, curl, jq.
#
# Usage:
#   scripts/run-calibration-e2e.sh --config scripts/calibration-cases.json [--out report.md]
#
# Exit codes:
#   0  all enabled cases passed (or none enabled)
#   1  at least one case failed an expectation
#   2  at least one case errored (gateway unreachable, task failed, etc.)

set -euo pipefail

CONFIG=""
OUT=""

usage() {
  cat <<EOF
Usage: $0 --config FILE [--out FILE]

Required:
  --config FILE   JSON config (see scripts/calibration-cases.example.json)

Optional:
  --out FILE      Also write the markdown report to FILE
  -h, --help      Show this help

Exit codes:
  0  all enabled cases passed
  1  one or more expectations failed
  2  one or more cases errored before evaluation
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config) CONFIG="$2"; shift 2 ;;
    --out)    OUT="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown arg: $1" >&2; usage; exit 2 ;;
  esac
done

[[ -z "$CONFIG" ]] && { echo "Missing --config" >&2; usage; exit 2; }
[[ -f "$CONFIG" ]] || { echo "Config not found: $CONFIG" >&2; exit 2; }

for cmd in curl jq; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "$cmd is required" >&2; exit 2; }
done

GATEWAY=$(jq -r '.gateway' "$CONFIG")
ACCESS_TOKEN=$(jq -r '.access_token' "$CONFIG")
USER_ID=$(jq -r '.user_id' "$CONFIG")
POLL_INTERVAL=$(jq -r '.poll_interval_sec // 3' "$CONFIG")
MAX_WAIT=$(jq -r '.max_wait_sec // 300' "$CONFIG")

[[ -z "$GATEWAY" || "$GATEWAY" == "null" ]]            && { echo "config.gateway missing"      >&2; exit 2; }
[[ -z "$ACCESS_TOKEN" || "$ACCESS_TOKEN" == "null" ]]  && { echo "config.access_token missing" >&2; exit 2; }
[[ -z "$USER_ID" || "$USER_ID" == "null" ]]            && { echo "config.user_id missing"      >&2; exit 2; }

# ---------- helpers ----------

# grade_rank maps a letter grade to an ordinal; -1 means unknown.
grade_rank() {
  case "$1" in
    Fail) echo 0 ;;
    D)    echo 1 ;;
    C)    echo 2 ;;
    B)    echo 3 ;;
    A)    echo 4 ;;
    S)    echo 5 ;;
    *)    echo -1 ;;
  esac
}

# Append a failure reason to the FAIL_REASONS array for the current case.
fail_reasons=()
add_fail() { fail_reasons+=("$1"); }

# Resets per-case state.
reset_case() {
  fail_reasons=()
}

# ---------- per-case driver ----------

run_case() {
  local case_json="$1"
  local case_id case_desc enabled
  case_id=$(jq -r '.id'           <<<"$case_json")
  case_desc=$(jq -r '.description // ""' <<<"$case_json")
  enabled=$(jq -r '.enabled // false' <<<"$case_json")

  if [[ "$enabled" != "true" ]]; then
    case_result "SKIP" "$case_id" "$case_desc" "" "" "" "disabled"
    return 0
  fi

  reset_case

  local request expect
  request=$(jq -c '.request' <<<"$case_json")
  expect=$(jq -c '.expect // {}'  <<<"$case_json")

  if [[ "$request" == "null" ]]; then
    case_result "ERROR" "$case_id" "$case_desc" "" "" "" "missing .request"
    return 2
  fi

  # Create task
  local create_resp http_status task_id
  create_resp=$(curl -sS -X POST "$GATEWAY/api/token_verification/tasks" \
    -H "Authorization: $ACCESS_TOKEN" \
    -H "HermesToken-User: $USER_ID" \
    -H "Content-Type: application/json" \
    -d "$request" 2>&1) || {
      case_result "ERROR" "$case_id" "$case_desc" "" "" "" "create http error: $create_resp"
      return 2
  }
  if [[ "$(jq -r '.success' <<<"$create_resp")" != "true" ]]; then
    local msg
    msg=$(jq -r '.message // "unknown"' <<<"$create_resp")
    case_result "ERROR" "$case_id" "$case_desc" "" "" "" "create failed: $msg"
    return 2
  fi
  task_id=$(jq -r '.data.id' <<<"$create_resp")

  # Poll until done
  local elapsed=0 detail status
  while :; do
    detail=$(curl -sS "$GATEWAY/api/token_verification/tasks/$task_id" \
      -H "Authorization: $ACCESS_TOKEN" \
      -H "HermesToken-User: $USER_ID" 2>&1) || {
        case_result "ERROR" "$case_id" "$case_desc" "" "" "$task_id" "poll http error: $detail"
        return 2
    }
    if [[ "$(jq -r '.success' <<<"$detail")" != "true" ]]; then
      local msg
      msg=$(jq -r '.message // "unknown"' <<<"$detail")
      case_result "ERROR" "$case_id" "$case_desc" "" "" "$task_id" "poll failed: $msg"
      return 2
    fi
    status=$(jq -r '.data.task.status' <<<"$detail")
    case "$status" in
      success|failed) break ;;
      pending|running)
        sleep "$POLL_INTERVAL"
        elapsed=$((elapsed + POLL_INTERVAL))
        if (( elapsed >= MAX_WAIT )); then
          case_result "ERROR" "$case_id" "$case_desc" "" "" "$task_id" "poll timeout after ${MAX_WAIT}s"
          return 2
        fi
        ;;
      *)
        case_result "ERROR" "$case_id" "$case_desc" "" "" "$task_id" "unexpected task status: $status"
        return 2 ;;
    esac
  done

  # Evaluate expectations
  evaluate_expectations "$detail" "$expect"

  local grade score baseline
  grade=$(jq -r '.data.report.grade // ""' <<<"$detail")
  score=$(jq -r '.data.report.score // 0'  <<<"$detail")
  baseline=$(jq -r '.data.report.baseline_source // ""' <<<"$detail")

  if [[ ${#fail_reasons[@]} -eq 0 ]]; then
    case_result "PASS" "$case_id" "$case_desc" "$grade ($score)" "$baseline" "$task_id" "-"
    return 0
  fi
  local joined
  joined=$(printf "; %s" "${fail_reasons[@]}")
  joined="${joined:2}"
  case_result "FAIL" "$case_id" "$case_desc" "$grade ($score)" "$baseline" "$task_id" "$joined"
  return 1
}

# Each expectation that fails appends a reason via add_fail.
evaluate_expectations() {
  local detail="$1"
  local expect="$2"
  local task_status grade score baseline_source
  task_status=$(jq -r '.data.task.status // ""'        <<<"$detail")
  grade=$(jq -r       '.data.report.grade // ""'       <<<"$detail")
  score=$(jq -r       '.data.report.score // 0'        <<<"$detail")
  baseline_source=$(jq -r '.data.report.baseline_source // ""' <<<"$detail")

  # task_status (e.g. for FAIL-MODEL cases that expect task.success despite checks failing)
  local exp
  exp=$(jq -r '.task_status // empty' <<<"$expect")
  [[ -n "$exp" && "$task_status" != "$exp" ]] && add_fail "task.status=$task_status, want $exp"

  # grade comparisons
  exp=$(jq -r '.grade_equals // empty' <<<"$expect")
  [[ -n "$exp" && "$grade" != "$exp" ]] && add_fail "grade=$grade, want $exp"

  exp=$(jq -r '.grade_at_least // empty' <<<"$expect")
  if [[ -n "$exp" ]]; then
    local g_have g_want
    g_have=$(grade_rank "$grade")
    g_want=$(grade_rank "$exp")
    if (( g_have < 0 || g_want < 0 || g_have < g_want )); then
      add_fail "grade=$grade, want >= $exp"
    fi
  fi

  exp=$(jq -r '.grade_at_most // empty' <<<"$expect")
  if [[ -n "$exp" ]]; then
    local g_have g_want
    g_have=$(grade_rank "$grade")
    g_want=$(grade_rank "$exp")
    if (( g_have < 0 || g_want < 0 || g_have > g_want )); then
      add_fail "grade=$grade, want <= $exp"
    fi
  fi

  exp=$(jq -r '.baseline_source // empty' <<<"$expect")
  [[ -n "$exp" && "$baseline_source" != "$exp" ]] && add_fail "baseline_source=$baseline_source, want $exp"

  # suspected_downgrade flags
  if [[ "$(jq -r '.suspected_downgrade // empty' <<<"$expect")" == "true" ]]; then
    local has
    has=$(jq -r '[.data.report.model_identity[]?.suspected_downgrade] | any' <<<"$detail")
    [[ "$has" != "true" ]] && add_fail "expected suspected_downgrade=true, none found"
  fi
  if [[ "$(jq -r '.no_suspected_downgrade // empty' <<<"$expect")" == "true" ]]; then
    local has
    has=$(jq -r '[.data.report.model_identity[]?.suspected_downgrade] | any' <<<"$detail")
    [[ "$has" == "true" ]] && add_fail "expected no suspected_downgrade, found one"
  fi

  # identity_confidence min/max (compares against ALL identity entries)
  exp=$(jq -r '.identity_confidence_at_most // empty' <<<"$expect")
  if [[ -n "$exp" ]]; then
    local violators
    violators=$(jq -r --argjson max "$exp" \
      '[.data.report.model_identity[]? | select(.identity_confidence > $max) | "\(.claimed_model)=\(.identity_confidence)"] | join(",")' \
      <<<"$detail")
    [[ -n "$violators" ]] && add_fail "identity_confidence > $exp: [$violators]"
  fi
  exp=$(jq -r '.identity_confidence_at_least // empty' <<<"$expect")
  if [[ -n "$exp" ]]; then
    local violators
    violators=$(jq -r --argjson min "$exp" \
      '[.data.report.model_identity[]? | select(.identity_confidence < $min) | "\(.claimed_model)=\(.identity_confidence)"] | join(",")' \
      <<<"$detail")
    [[ -n "$violators" ]] && add_fail "identity_confidence < $exp: [$violators]"
  fi

  # dimension_at_least (object map of dim -> min int)
  while IFS=$'\t' read -r dim min; do
    [[ -z "$dim" ]] && continue
    local actual
    actual=$(jq -r --arg dim "$dim" '.data.report.dimensions[$dim] // 0' <<<"$detail")
    if (( actual < min )); then
      add_fail "dimensions.$dim=$actual, want >= $min"
    fi
  done < <(jq -r '.dimension_at_least // {} | to_entries[]? | "\(.key)\t\(.value)"' <<<"$expect")

  # dimension_at_most
  while IFS=$'\t' read -r dim maxv; do
    [[ -z "$dim" ]] && continue
    local actual
    actual=$(jq -r --arg dim "$dim" '.data.report.dimensions[$dim] // 0' <<<"$detail")
    if (( actual > maxv )); then
      add_fail "dimensions.$dim=$actual, want <= $maxv"
    fi
  done < <(jq -r '.dimension_at_most // {} | to_entries[]? | "\(.key)\t\(.value)"' <<<"$expect")

  # dimension_equals
  while IFS=$'\t' read -r dim want; do
    [[ -z "$dim" ]] && continue
    local actual
    actual=$(jq -r --arg dim "$dim" '.data.report.dimensions[$dim] // 0' <<<"$detail")
    if [[ "$actual" != "$want" ]]; then
      add_fail "dimensions.$dim=$actual, want $want"
    fi
  done < <(jq -r '.dimension_equals // {} | to_entries[]? | "\(.key)\t\(.value)"' <<<"$expect")

  # risk_contains: scalar (single substring) or array (all must match)
  local risk_type
  risk_type=$(jq -r '.risk_contains | type' <<<"$expect")
  if [[ "$risk_type" == "string" ]]; then
    local needle
    needle=$(jq -r '.risk_contains' <<<"$expect")
    local has
    has=$(jq -r --arg n "$needle" '[.data.report.risks[]? | select(contains($n))] | length' <<<"$detail")
    [[ "$has" == "0" ]] && add_fail "no risk contains \"$needle\""
  elif [[ "$risk_type" == "array" ]]; then
    while IFS= read -r needle; do
      [[ -z "$needle" ]] && continue
      local has
      has=$(jq -r --arg n "$needle" '[.data.report.risks[]? | select(contains($n))] | length' <<<"$detail")
      [[ "$has" == "0" ]] && add_fail "no risk contains \"$needle\""
    done < <(jq -r '.risk_contains[]?' <<<"$expect")
  fi
}

# ---------- output ----------

# Storage for all rows (printf-friendly)
rows_status=()
rows_id=()
rows_grade=()
rows_baseline=()
rows_taskid=()
rows_notes=()
rows_desc=()

case_result() {
  rows_status+=("$1")
  rows_id+=("$2")
  rows_desc+=("$3")
  rows_grade+=("$4")
  rows_baseline+=("$5")
  rows_taskid+=("$6")
  rows_notes+=("$7")
}

emit_markdown() {
  local total=${#rows_id[@]}
  local pass=0 fail=0 skip=0 err=0
  for s in "${rows_status[@]}"; do
    case "$s" in
      PASS)  pass=$((pass+1)) ;;
      FAIL)  fail=$((fail+1)) ;;
      SKIP)  skip=$((skip+1)) ;;
      ERROR) err=$((err+1)) ;;
    esac
  done

  echo "# HermesToken Calibration Layer-2 Report"
  echo ""
  echo "- Gateway: \`$GATEWAY\`"
  echo "- Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  echo "- Cases: total=$total, pass=$pass, fail=$fail, skip=$skip, error=$err"
  echo ""
  echo "## Summary"
  echo ""
  echo "| Case | Status | Grade | Baseline | Task | Notes |"
  echo "| ---- | ------ | ----- | -------- | ---- | ----- |"
  for i in "${!rows_id[@]}"; do
    printf "| %s | %s | %s | %s | %s | %s |\n" \
      "${rows_id[$i]}" \
      "${rows_status[$i]}" \
      "${rows_grade[$i]:--}" \
      "${rows_baseline[$i]:--}" \
      "${rows_taskid[$i]:--}" \
      "${rows_notes[$i]}"
  done

  if [[ $fail -gt 0 || $err -gt 0 ]]; then
    echo ""
    echo "## Failures and errors"
    echo ""
    for i in "${!rows_id[@]}"; do
      [[ "${rows_status[$i]}" != "FAIL" && "${rows_status[$i]}" != "ERROR" ]] && continue
      echo "### ${rows_id[$i]} — ${rows_status[$i]}"
      [[ -n "${rows_desc[$i]}" ]] && echo "${rows_desc[$i]}"
      echo ""
      echo "- ${rows_notes[$i]}"
      [[ -n "${rows_taskid[$i]}" ]] && echo "- Task id: ${rows_taskid[$i]}"
      echo ""
    done
  fi
}

# ---------- main loop ----------

overall_exit=0
total_cases=$(jq -r '.cases | length' "$CONFIG")
for ((i=0; i<total_cases; i++)); do
  case_json=$(jq -c ".cases[$i]" "$CONFIG")
  rc=0
  run_case "$case_json" || rc=$?
  if (( rc == 1 )) && (( overall_exit < 1 )); then overall_exit=1; fi
  if (( rc == 2 )); then overall_exit=2; fi
done

if [[ -n "$OUT" ]]; then
  emit_markdown | tee "$OUT"
else
  emit_markdown
fi

exit "$overall_exit"
