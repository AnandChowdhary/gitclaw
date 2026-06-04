#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "tools-readiness-report-e2e: $*" >&2
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
if [[ -z "${GH_TOKEN:-}" && -z "${GITHUB_TOKEN:-}" ]]; then
  export GH_TOKEN="$(gh auth token)"
fi
if [[ -z "${GITHUB_TOKEN:-}" && -n "${GH_TOKEN:-}" ]]; then
  export GITHUB_TOKEN="$GH_TOKEN"
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
body_token="GITCLAW_TOOLS_READINESS_BODY_${timestamp}"
followup_hidden_token="GITCLAW_TOOLS_READINESS_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_TOOLS_READINESS_CONTEXT_V1"
search_phrase="tools readiness unique search fixture phrase"
title="@gitclaw /tools readiness search_files e2e ${timestamp}"
body="Live tools readiness E2E ${timestamp}.

Search for \"${search_phrase}\" so a search_files output is active.
Hidden tools readiness body token: ${body_token}
The deterministic readiness report must stay body-free and must be paired with a separate live LLM chat E2E for feature changes."

local_report="$(go run ./cmd/gitclaw tools readiness search_files)"
for expected in \
  "GitClaw Tool Readiness Report" \
  "Generated without a model call" \
  "tool_readiness_status: \`ok\`" \
  "tool_readiness_mode: \`body-free-tool-gate-checklist\`" \
  "normalized_tool: \`gitclaw.search_files\`" \
  "matched_tools: \`1\`" \
  "tool_enabled: \`true\`" \
  "prompt_visible_ready: \`true\`" \
  "model_context_allowed: \`true\`" \
  "execution_allowed_now: \`false\`" \
  "readiness_gate_count: \`12\`" \
  "model_call_performed: \`false\`" \
  "tool_execution_performed: \`false\`" \
  "shell_execution_allowed: \`false\`" \
  "mcp_launch_allowed: \`false\`" \
  "repository_mutation_allowed: \`false\`" \
  "workflow_mutation_allowed: \`false\`" \
  "raw_inputs_included: \`false\`" \
  "raw_outputs_included: \`false\`" \
  "llm_e2e_required_after_tool_readiness_change: \`true\`" \
  "### Readiness Gates" \
  "gate=\`tool_contract\` status=\`matched\`" \
  "gate=\`active_outputs\` status=\`hashes_only\`" \
  "gate=\`tool_execution\` status=\`disabled\`" \
  "code=\`openclaw_prompt_tool_exposure_checked\`" \
  "code=\`hermes_tool_boundary_kept_issue_native\`"; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local tools readiness report missing ${expected}"
done
for leaked in "$expected_token" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$local_report"; then
    die "local tools readiness report leaked ${leaked}"
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
      gh issue close "$issue_number" --repo "$repo" --comment "tools-readiness-report e2e cleanup" >/dev/null 2>&1 || true
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
      local run_status conclusion url
      run_status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$run_status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${event_name} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

assistant_comments() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | join("\n---GITCLAW-COMMENT---\n")'
}

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

assistant_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
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

run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one tools readiness report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/tools"' \
  "GitClaw Tool Readiness Report" \
  "Generated without a model call" \
  'tool_readiness_status: `ok`' \
  'tool_readiness_mode: `body-free-tool-gate-checklist`' \
  'normalized_tool: `gitclaw.search_files`' \
  'matched_tools: `1`' \
  'active_outputs_for_tool: `1`' \
  'tool_enabled: `true`' \
  'tool_mode: `read-only`' \
  'tool_trigger: `explicit quoted phrase or identifier`' \
  'mutating_contract: `false`' \
  'prompt_visible_ready: `true`' \
  'model_context_allowed: `true`' \
  'execution_allowed_now: `false`' \
  'approval_required: `false`' \
  'readiness_gate_count: `12`' \
  'model_call_performed: `false`' \
  'tool_execution_performed: `false`' \
  'shell_execution_allowed: `false`' \
  'mcp_launch_allowed: `false`' \
  'network_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'workflow_mutation_allowed: `false`' \
  'raw_tool_name_included: `false`' \
  'raw_inputs_included: `false`' \
  'raw_outputs_included: `false`' \
  'raw_issue_body_included: `false`' \
  'raw_comments_included: `false`' \
  'tool_validation_status: `ok`' \
  'tool_risk_status: `ok`' \
  'llm_e2e_required_after_tool_readiness_change: `true`' \
  "### Matched Tool" \
  'name=`gitclaw.search_files`' \
  "### Active Outputs For Tool" \
  'contract_known=`true`' \
  'input_sha256_12=' \
  'output_sha256_12=' \
  "### Readiness Gates" \
  'gate=`tool_contract` status=`matched`' \
  'gate=`active_outputs` status=`hashes_only`' \
  'gate=`tool_execution` status=`disabled`' \
  "### Findings" \
  'code=`openclaw_prompt_tool_exposure_checked`' \
  'code=`hermes_tool_boundary_kept_issue_native`' \
  'code=`tool_execution_disabled`' \
  'code=`prompt_visible_read_only_or_metadata_only`'; do
  grep -Fq -- "$expected" <<<"$comments" || die "tools readiness report missing ${expected}"
done

for leaked in "$body_token" "$title" "$expected_token" "$search_phrase" "Hidden tools readiness body token"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "tools readiness report leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_TOOLS_READINESS token from the matching repository search result line.
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

for leaked in "$body_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
url="$(jq -r '.url' <<<"$run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${url} (model follow-up: ${model_url})"
