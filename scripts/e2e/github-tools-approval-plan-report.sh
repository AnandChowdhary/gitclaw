#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "tools-approval-plan-report-e2e: $*" >&2
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

timestamp_with_run_filter_slack() {
  date -u -v-15S +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "15 seconds ago" +%Y-%m-%dT%H:%M:%SZ
}

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
hidden_token="GITCLAW_TOOLS_APPROVAL_PLAN_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_TOOLS_APPROVAL_PLAN_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_TOOL_APPROVAL_CONTEXT_V1"
search_phrase="tool approval unique search fixture phrase"
title="@gitclaw /tools approval-plan e2e ${timestamp}"
body="@gitclaw /tools approval-plan search_files

Search for \`${search_phrase}\` so this report has active tool-output hashes.
Do not include this hidden issue token: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw tools approval-plan search_files)"
for expected in \
  "GitClaw Tool Approval Plan Report" \
  "Generated without a model call" \
  'scope: `local-cli`' \
  'tool_approval_plan_status: `ok`' \
  'normalized_tool: `gitclaw.search_files`' \
  'matched_tools: `1`' \
  'tool_enabled: `true`' \
  'tool_mode: `read-only`' \
  'approval_required: `false`' \
  'approval_decision: `no_approval_required_read_only`' \
  'approval_store: `github-issue-labels`' \
  'approval_label: `gitclaw:approved`' \
  'needs_human_label: `gitclaw:needs-human`' \
  'write_requested_label: `gitclaw:write-requested`' \
  'approval_timeout_policy: `not_applicable_no_exec_tool`' \
  'run_allowed_now: `true`' \
  'write_actions_enabled: `false`' \
  'model_call_required: `false`' \
  'model_callable_structured_tools: `false`' \
  'shell_execution_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_inputs_included: `false`' \
  'raw_outputs_included: `false`' \
  'raw_approval_payloads_included: `false`' \
  'tool_validation_status: `ok`' \
  'llm_e2e_required_after_tool_approval_plan_change: `true`' \
  'gate=`approval_label` status=`not_required` label=`gitclaw:approved`' \
  'code=`openclaw_exec_approval_boundary_modeled`' \
  'code=`hermes_tool_authorization_boundary_modeled`' \
  'code=`github_issue_approval_store_modeled`' \
  'code=`read_only_or_metadata_only_no_approval_required`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local tools approval-plan report missing ${expected}"
done

issue_started_at="$(timestamp_with_run_filter_slack)"
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
      gh issue close "$issue_number" --repo "$repo" --comment "tools-approval-plan-report e2e cleanup" >/dev/null 2>&1 || true
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

approval_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one tools approval-plan report comment"
approval_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/tools"' \
  "GitClaw Tool Approval Plan Report" \
  "Generated without a model call" \
  'repository: `'"$repo"'`' \
  'issue: `#'"$issue_number"'`' \
  'tool_approval_plan_status: `ok`' \
  'normalized_tool: `gitclaw.search_files`' \
  'matched_tools: `1`' \
  'active_outputs_for_tool: `1`' \
  'tool_enabled: `true`' \
  'disabled_by_config: `false`' \
  'blocked_by_allowlist: `false`' \
  'tool_mode: `read-only`' \
  'tool_trigger: `explicit quoted phrase or identifier`' \
  'mutating_contract: `false`' \
  'approval_required: `false`' \
  'approval_decision: `no_approval_required_read_only`' \
  'approval_store: `github-issue-labels`' \
  'approval_scope: `per-issue`' \
  'approval_label: `gitclaw:approved`' \
  'needs_human_label: `gitclaw:needs-human`' \
  'write_requested_label: `gitclaw:write-requested`' \
  'approval_timeout_policy: `not_applicable_no_exec_tool`' \
  'run_allowed_now: `true`' \
  'write_actions_enabled: `false`' \
  'model_call_required: `false`' \
  'model_callable_structured_tools: `false`' \
  'shell_execution_allowed: `false`' \
  'network_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_tool_name_included: `false`' \
  'raw_inputs_included: `false`' \
  'raw_outputs_included: `false`' \
  'raw_approval_payloads_included: `false`' \
  'tool_validation_status: `ok`' \
  'tool_validation_errors: `0`' \
  'tool_validation_warnings: `0`' \
  'llm_e2e_required_after_tool_approval_plan_change: `true`' \
  'gate=`tool_contract` status=`matched` matched_tools=`1`' \
  'gate=`config_enabled` status=`passed` disabled_by_config=`false`' \
  'gate=`allowlist` status=`passed` blocked_by_allowlist=`false`' \
  'gate=`tool_mode` status=`read_only_or_metadata_only` mutating_contract=`false`' \
  'gate=`approval_label` status=`not_required` label=`gitclaw:approved`' \
  'gate=`write_mode` status=`blocked` detail=`read_only_v1`' \
  'gate=`structured_model_tools` status=`disabled`' \
  'gate=`shell_exec` status=`disabled`' \
  'gate=`repository_mutation` status=`disabled`' \
  'name=`gitclaw.search_files` source=`builtin-gitclaw` enabled=`true`' \
  'contract_known=`true` input_sha256_12=' \
  'output_sha256_12=' \
  'code=`openclaw_exec_approval_boundary_modeled`' \
  'code=`hermes_tool_authorization_boundary_modeled`' \
  'code=`github_issue_approval_store_modeled`' \
  'code=`read_only_or_metadata_only_no_approval_required`'; do
  grep -Fq -- "$expected" <<<"$approval_comment" || die "tools approval-plan report missing ${expected}"
done

for leaked in "$hidden_token" "$expected_token" "$search_phrase" "Search for" "approval unique"; do
  if grep -Fq "$leaked" <<<"$approval_comment"; then
    die "tools approval-plan report leaked ${leaked}"
  fi
done

comment_started_at="$(timestamp_with_run_filter_slack)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_TOOL_APPROVAL token from the matching repository search result line.
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
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant marker missing normalized provider usage"

for leaked in "$hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
approval_url="$(jq -r '.url' <<<"$approval_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${approval_url} (model follow-up: ${model_url})"
