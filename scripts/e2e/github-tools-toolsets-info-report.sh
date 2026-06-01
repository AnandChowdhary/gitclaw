#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "tools-toolsets-info-report-e2e: $*" >&2
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
hidden_token="NOECHO_TOOLSET_INFO_SENTINEL_${timestamp}"
followup_hidden_token="NOECHO_TOOLSET_INFO_FOLLOWUP_SENTINEL_${timestamp}"
expected_token="GITCLAW_TOOLSET_INFO_CONTEXT_V1"
search_phrase="toolset info unique search fixture phrase"
title="@gitclaw /tools toolsets info repo-read e2e ${timestamp}"
body="@gitclaw /tools toolsets info repo-read

Live toolset info E2E. Keep the focused profile report body-free.
Do not include this hidden toolset info sentinel: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw tools toolsets info repo-read)"
for expected in \
  "GitClaw Toolset Info Report" \
  'scope: `local-cli`' \
  'requested_toolset_sha256_12:' \
  'normalized_toolset: `repo-read`' \
  'toolset_info_status: `ok`' \
  'available_toolsets: `1`' \
  'matched_toolsets: `1`' \
  'runtime_toolset_selection: `not_active_in_v1`' \
  'registry_verification: `static_builtin_contracts_only`' \
  'toolset_activation_supported: `false`' \
  'repository_mutation_allowed: `false`' \
  'shell_execution_allowed: `false`' \
  'network_tool_execution_allowed: `false`' \
  'raw_requested_toolset_included: `false`' \
  'raw_toolset_bodies_included: `false`' \
  'raw_toolset_instructions_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'llm_e2e_required_after_toolset_info_change: `true`' \
  'toolset_name=`repo-read` path=`.gitclaw/toolsets/repo-read.yaml`' \
  'mode=`read-only`' \
  'tools=`gitclaw.list_files' \
  'resolved_tools=`gitclaw.list_files' \
  'unknown_tools=`none`' \
  'disabled_tools=`none`' \
  'allowlist_blocked_tools=`none`' \
  'instruction=`true`' \
  'description=`true`' \
  'risk_findings=`0`' \
  'risk_max_severity=`none`' \
  'risk_codes=`none`' \
  '### Risk Findings For Matches' \
  "- none" \
  '### Info Gates' \
  'toolset_info_gate=`ok`' \
  'activation_gate=`disabled`' \
  'mutation_gate=`disabled`' \
  'shell_execution_gate=`disabled`' \
  'network_execution_gate=`disabled`' \
  'raw_body_gate=`hash_only`' \
  'model_e2e_gate=`required`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local toolset info report missing ${expected}"
done

for leaked in "Read-only repository context tools" "Prefer bounded repository search" "$expected_token" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$local_report"; then
    die "local toolset info report leaked ${leaked}"
  fi
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
      gh issue close "$issue_number" --repo "$repo" --comment "tools-toolsets-info-report e2e cleanup" >/dev/null 2>&1 || true
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

info_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one toolset info report comment"
info_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/tools"' \
  "GitClaw Toolset Info Report" \
  "Generated without a model call" \
  'repository: `' \
  'issue: `#' \
  'requested_toolset_sha256_12:' \
  'normalized_toolset: `repo-read`' \
  'toolset_info_status: `ok`' \
  'available_toolsets: `1`' \
  'matched_toolsets: `1`' \
  'runtime_toolset_selection: `not_active_in_v1`' \
  'registry_verification: `static_builtin_contracts_only`' \
  'toolset_activation_supported: `false`' \
  'repository_mutation_allowed: `false`' \
  'shell_execution_allowed: `false`' \
  'network_tool_execution_allowed: `false`' \
  'raw_requested_toolset_included: `false`' \
  'raw_toolset_bodies_included: `false`' \
  'raw_toolset_instructions_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'llm_e2e_required_after_toolset_info_change: `true`' \
  'toolset_name=`repo-read` path=`.gitclaw/toolsets/repo-read.yaml`' \
  'mode=`read-only`' \
  'tools=`gitclaw.list_files' \
  'resolved_tools=`gitclaw.list_files' \
  'unknown_tools=`none`' \
  'disabled_tools=`none`' \
  'allowlist_blocked_tools=`none`' \
  'instruction=`true`' \
  'description=`true`' \
  'risk_findings=`0`' \
  'risk_max_severity=`none`' \
  'risk_codes=`none`' \
  '### Risk Findings For Matches' \
  "- none" \
  '### Info Gates' \
  'toolset_info_gate=`ok`' \
  'activation_gate=`disabled`' \
  'mutation_gate=`disabled`' \
  'shell_execution_gate=`disabled`' \
  'network_execution_gate=`disabled`' \
  'raw_body_gate=`hash_only`' \
  'model_e2e_gate=`required`'; do
  grep -Fq -- "$expected" <<<"$info_comment" || die "toolset info report missing ${expected}"
done

for leaked in "$hidden_token" "Live toolset info E2E" "Read-only repository context tools" "Prefer bounded repository search" "$expected_token" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$info_comment"; then
    die "toolset info report leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Search for that exact phrase, not shorter words from it.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${search_phrase} => <token>\`.
Reply with only the uppercase fixture token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with any token from this issue or its comments.
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
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant marker missing usage token telemetry"

for leaked in "$hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
info_url="$(jq -r '.url' <<<"$info_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${info_url} (model follow-up: ${model_url})"
