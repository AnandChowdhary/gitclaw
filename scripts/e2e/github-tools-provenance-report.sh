#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "tools-provenance-report-e2e: $*" >&2
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
hidden_token="GITCLAW_TOOLS_PROVENANCE_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_TOOLS_PROVENANCE_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw /tools provenance e2e ${timestamp}"
body="@gitclaw /tools provenance

Please inspect \`go.mod\` and search for \`${search_phrase}\`.
Do not include this hidden issue token: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw tools provenance "go.mod \"${search_phrase}\"")"
for expected in \
  "GitClaw Tool Provenance Report" \
  'tool_provenance_status: `ok`' \
  'provenance_scope: `pre_model_prompt_context`' \
  'tool_context_strategy: `deterministic-pre-model-outputs`' \
  'available_tools: `5`' \
  'enabled_tools: `5`' \
  'active_tool_outputs: `4`' \
  'known_tool_outputs: `4`' \
  'unknown_tool_outputs: `0`' \
  'prompt_visible_tool_outputs: `4`' \
  'read_only_outputs: `3`' \
  'metadata_only_outputs: `1`' \
  'tool_inputs_hashed: `4`' \
  'tool_outputs_hashed: `4`' \
  'model_callable_structured_tools: `false`' \
  'raw_inputs_included: `false`' \
  'raw_outputs_included: `false`' \
  'raw_bodies_included: `false`' \
  'llm_e2e_required_after_tool_provenance_change: `true`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local tools provenance report missing ${expected}"
done

for leaked in "module github.com/AnandChowdhary/gitclaw" "$search_phrase" "$expected_token"; do
  if grep -Fq "$leaked" <<<"$local_report"; then
    die "local tools provenance report leaked ${leaked}"
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
      gh issue close "$issue_number" --repo "$repo" --comment "tools-provenance-report e2e cleanup" >/dev/null 2>&1 || true
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

provenance_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one tools provenance report comment"
provenance_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/tools"' \
  "GitClaw Tool Provenance Report" \
  "Generated without a model call" \
  'tool_provenance_status: `ok`' \
  'provenance_scope: `pre_model_prompt_context`' \
  'tool_context_strategy: `deterministic-pre-model-outputs`' \
  'available_tools: `5`' \
  'enabled_tools: `5`' \
  'disabled_tools: `0`' \
  'allowlist_blocked_tools: `0`' \
  'active_tool_outputs: `4`' \
  'known_tool_outputs: `4`' \
  'unknown_tool_outputs: `0`' \
  'prompt_visible_tool_outputs: `4`' \
  'prompt_visible_tool_names: `gitclaw.list_files, gitclaw.read_file, gitclaw.search_files, gitclaw.skill_index`' \
  'read_only_outputs: `3`' \
  'metadata_only_outputs: `1`' \
  'tool_inputs_hashed: `4`' \
  'tool_outputs_hashed: `4`' \
  'registry_verification: `not_configured`' \
  'runtime_permission_verification: `static_contracts_only`' \
  'model_callable_structured_tools: `false`' \
  'shell_execution_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_inputs_included: `false`' \
  'raw_outputs_included: `false`' \
  'raw_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'llm_e2e_required_after_tool_provenance_change: `true`' \
  'tool_validation_status: `ok`' \
  'tool_validation_errors: `0`' \
  'tool_validation_warnings: `0`' \
  'tool_risk_status: `ok`' \
  'high_risk_findings: `0`' \
  'warning_risk_findings: `0`' \
  "### Prompt-Visible Tool Outputs" \
  'name=`gitclaw.list_files` contract_known=`true` mode=`read-only`' \
  'name=`gitclaw.read_file` contract_known=`true` mode=`read-only`' \
  'name=`gitclaw.search_files` contract_known=`true` mode=`read-only`' \
  'name=`gitclaw.skill_index` contract_known=`true` mode=`metadata-only`' \
  'input_sha256_12=' \
  'output_sha256_12=' \
  'risk_codes=`none`' \
  "### Provenance Gates" \
  'model_callable_structured_tools=`false`' \
  'raw_input_gate=`hash_only`' \
  'raw_output_gate=`hash_only`' \
  'mutation_gate=`disabled`' \
  'shell_gate=`disabled`'; do
  grep -Fq -- "$expected" <<<"$provenance_comment" || die "tools provenance report missing ${expected}"
done

for leaked in "$hidden_token" "module github.com/AnandChowdhary/gitclaw" "$expected_token" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$provenance_comment"; then
    die "tools provenance report leaked ${leaked}"
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
provenance_url="$(jq -r '.url' <<<"$provenance_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${provenance_url} (model follow-up: ${model_url})"
