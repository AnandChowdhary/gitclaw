#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "tools-exposure-report-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow_name="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
retention_label="${GITCLAW_E2E_RETENTION_LABEL:-gitclaw:e2e}"

if [[ -z "$repo" ]]; then
  repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
fi

ensure_label() {
  local name="$1"
  local color="$2"
  local description="$3"
  gh label create "$name" --repo "$repo" --color "$color" --description "$description" --force >/dev/null
}

ensure_label gitclaw 5319e7 "Handled by GitClaw"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
hidden_token="GITCLAW_TOOLS_EXPOSURE_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_TOOLS_EXPOSURE_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw /tools exposure risk e2e ${timestamp}"
body="@gitclaw /tools exposure risk

Live tools exposure E2E.
Do not include this hidden issue token: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw tools exposure risk)"
for expected in \
  "GitClaw Tool Exposure Risk Report" \
  'tool_exposure_status: `ok`' \
  'exposure_strategy: `static-pre-model-context`' \
  'bridge_strategy: `not_enabled_in_v1`' \
  'available_tools: `5`' \
  'enabled_tool_contracts: `5`' \
  'model_callable_structured_tools: `false`' \
  'fail_closed_required: `false`' \
  'raw_tool_schemas_included: `false`' \
  'raw_tool_inputs_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'llm_e2e_required_after_tool_exposure_change: `true`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local tools exposure risk report missing ${expected}"
done

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label gitclaw)"
issue_number="${issue_url##*/}"

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "tools-exposure-report e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
}
trap cleanup EXIT

log "created issue #${issue_number}: ${issue_url}"

wait_for_run() {
  local event_name="$1"
  local started_at="$2"
  for _ in {1..90}; do
    local run_json
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$workflow_name" \
      --event "$event_name" \
      --created ">=$started_at" \
      --limit 10 \
      --json databaseId,status,conclusion,url,createdAt,displayTitle \
      --jq '. as $runs | $runs | map(select(.displayTitle == "'"${title}"'")) | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${event_name} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

assistant_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

error_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("<!-- gitclaw:error"))] | length'
}

issue_label_names() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json labels \
    --jq '.labels[].name'
}

wait_for_assistant_count() {
  local want="$1"
  for _ in {1..90}; do
    local errors
    errors="$(error_count)"
    if [[ "$errors" != "0" ]]; then
      die "assistant run posted ${errors} error marker comment(s)"
    fi
    local got
    got="$(assistant_count)"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

wait_for_done_status() {
  for _ in {1..60}; do
    local labels
    labels="$(issue_label_names)"
    if grep -Fxq "gitclaw:done" <<<"$labels" &&
      ! grep -Fxq "gitclaw:running" <<<"$labels" &&
      ! grep -Fxq "gitclaw:error" <<<"$labels"; then
      return 0
    fi
    sleep 5
  done
  return 1
}

exposure_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one tools exposure report comment"
exposure_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/tools"' \
  "GitClaw Tool Exposure Risk Report" \
  "Generated without a model call" \
  'tool_exposure_status: `ok`' \
  'exposure_strategy: `static-pre-model-context`' \
  'bridge_strategy: `not_enabled_in_v1`' \
  'available_tools: `5`' \
  'enabled_tool_contracts: `5`' \
  'disabled_tool_contracts: `0`' \
  'allowlist_blocked_tool_contracts: `0`' \
  'explicit_allowlist_configured: `false`' \
  'allowed_tool_names: `none`' \
  'disabled_tool_names: `none`' \
  'exposed_read_only_contracts: `3`' \
  'exposed_metadata_only_contracts: `2`' \
  'mutating_tool_contracts: `0`' \
  'active_tool_outputs:' \
  'known_active_tool_outputs:' \
  'unknown_active_tool_outputs: `0`' \
  'prompt_visible_tool_outputs:' \
  'model_callable_structured_tools: `false`' \
  'deferred_tool_schemas: `0`' \
  'tool_search_bridge_tools: `0`' \
  'fail_closed_required: `false`' \
  'tool_validation_status: `ok`' \
  'tool_validation_errors: `0`' \
  'tool_validation_warnings: `0`' \
  'high_risk_findings: `0`' \
  'warning_risk_findings: `0`' \
  'info_risk_findings: `3`' \
  'shell_execution_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'network_tool_execution_allowed: `false`' \
  'raw_tool_schemas_included: `false`' \
  'raw_tool_inputs_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'llm_e2e_required_after_tool_exposure_change: `true`' \
  "### Exposure Risk Cards" \
  'tool_name=`gitclaw.list_files`' \
  'tool_name=`gitclaw.search_files`' \
  'tool_name=`gitclaw.read_file`' \
  'tool_name=`gitclaw.skill_index`' \
  'tool_name=`gitclaw.policy`' \
  'enabled=`true`' \
  'exposed_for_prompt=`true`' \
  'risk_codes=`none`' \
  'exposure_codes=`none`' \
  "### Exposure Findings" \
  'code=`static_pre_model_tool_context`' \
  'code=`structured_model_tools_disabled`' \
  'code=`hermes_tool_search_bridge_not_enabled`'; do
  grep -Fq -- "$expected" <<<"$exposure_comment" || die "tools exposure report missing ${expected}"
done

for leaked in "$hidden_token" "Live tools exposure E2E" "module github.com/AnandChowdhary/gitclaw" "GITCLAW_SEARCH_CONTEXT_V1" "bounded repository search fixture phrase"; do
  if grep -Fq "$leaked" <<<"$exposure_comment"; then
    die "tools exposure report leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_SEARCH token from the matching repository search result line.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run issue_comment "$comment_started_at")" || die "timed out waiting for issue_comment workflow run"
wait_for_assistant_count 2 || die "expected model-backed follow-up assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant marker did not prove search_files was prompt-visible"

for leaked in "$hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
exposure_url="$(jq -r '.url' <<<"$exposure_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${exposure_url} (model follow-up: ${model_url})"
