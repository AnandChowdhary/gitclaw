#!/usr/bin/env bash
# gitclaw-doctor-live-issue: tools cancel-run closes a reviewed tool-run request and then proves normal LLM/tool chat.
set -euo pipefail

log() {
  echo "tools-run-cancel-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-tools-run-cancel-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another tools run-cancel E2E appears to be running: ${lock_dir}"
fi

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
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ | tr '[:upper:]' '[:lower:]')"
request_id="e2e-tool-run-cancel-${timestamp}"
source_title="@gitclaw /tools request-run search_files --id ${request_id}"
hidden_token="NOECHO_TOOLS_RUN_CANCEL_BODY_${timestamp}"
cancel_hidden_token="NOECHO_TOOLS_RUN_CANCEL_COMMAND_${timestamp}"
duplicate_hidden_token="NOECHO_TOOLS_RUN_CANCEL_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_TOOLS_RUN_CANCEL_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_TOOL_RUN_CANCEL_CONTEXT_V1"
search_phrase="tool run cancel unique search fixture phrase"
source_issue_number=""
request_issue_number=""

run_list_json() {
  local event="$1"
  local started_at="$2"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event "$event" \
    --created ">=$started_at" \
    --limit 20 \
    --json databaseId,status,conclusion,createdAt,url,displayTitle \
    --jq '.'
}

wait_for_run() {
  local event="$1"
  local started_at="$2"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json "$event" "$started_at" | jq -c --arg title "$source_title" '[.[] | select(.displayTitle == $title)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${event} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

assistant_count() {
  gh issue view "$source_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

error_count() {
  gh issue view "$source_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("<!-- gitclaw:error"))] | length'
}

wait_for_assistant_count() {
  local want="$1"
  for _ in {1..90}; do
    local errors
    errors="$(error_count)"
    if [[ "$errors" != "0" ]]; then
      die "source issue posted ${errors} error marker comment(s)"
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

latest_assistant_comment() {
  gh issue view "$source_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

cleanup() {
  for number in "$source_issue_number" "$request_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "tools run-cancel e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

source_body="GitClaw live tools run-cancel E2E.

Queue a reviewed search_files tool-run request for this test without copying this raw source request.
Do not include this hidden source token: ${hidden_token}"

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
source_url="$(gh issue create \
  --repo "$repo" \
  --title "$source_title" \
  --body "$source_body" \
  --label gitclaw)"
source_issue_number="${source_url##*/}"

wait_for_run "issues" "$started_at" >/dev/null || die "timed out waiting for tools request-run issues run"
wait_for_assistant_count 1 || die "expected one tools request-run receipt"
receipt="$(latest_assistant_comment)"

for expected in \
  "GitClaw Tool Run Request Issue Action" \
  "Generated without a model call" \
  'model="gitclaw/tools"' \
  "requested_tool_command: \`/tools request-run\`" \
  "tool_run_request_status: \`created\`" \
  "tool_run_request_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "tool_run_request_id: \`${request_id}\`" \
  "normalized_tool: \`gitclaw.search_files\`" \
  "matched_tool: \`gitclaw.search_files\`" \
  "tool_enabled: \`true\`" \
  "tool_mode: \`read-only\`" \
  "mutating_contract: \`false\`" \
  "approval_required: \`false\`" \
  "run_allowed_now: \`true\`" \
  "review_decision: \`review_required_read_only_tool\`" \
  "request_store: \`github-issue-to-reviewed-tool-run\`" \
  "model_call_performed: \`false\`" \
  "tool_execution_performed: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "raw_tool_name_included: \`false\`" \
  "raw_tool_inputs_included: \`false\`" \
  "raw_tool_outputs_included: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "llm_e2e_required_after_tool_run_request_issue_change: \`true\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "tools request-run receipt missing ${expected}"
done
for leaked in "$hidden_token" "Queue a reviewed search_files tool-run request"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "tools request-run receipt leaked ${leaked}"
  fi
done

request_issue_number="$(sed -n 's/.*tool_run_request_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
[[ -n "$request_issue_number" ]] || die "could not parse tool run request issue from receipt"
log "tools request-run created review issue #${request_issue_number}"

request_json="$(gh issue view "$request_issue_number" --repo "$repo" --json title,body,state)"
request_title="$(jq -r '.title' <<<"$request_json")"
request_body="$(jq -r '.body' <<<"$request_json")"
request_state="$(jq -r '.state' <<<"$request_json")"
[[ "$request_title" == "GitClaw tool run request: ${request_id}" ]] || die "unexpected tool run request issue title: ${request_title}"
[[ "$request_state" == "OPEN" ]] || die "tool run request issue should start open, got ${request_state}"
for expected in \
  "gitclaw:tool-run-request-issue" \
  "id=\"${request_id}\"" \
  "normalized_tool=\"gitclaw.search_files\"" \
  "request_id: ${request_id}" \
  "normalized_tool: gitclaw.search_files" \
  "matched_tool: gitclaw.search_files" \
  "tool_enabled: true" \
  "tool_mode: read-only" \
  "mutating_contract: false" \
  "approval_required: false" \
  "run_allowed_now: true" \
  "review_decision: review_required_read_only_tool" \
  "tool_execution_performed: false" \
  "model_call_performed: false" \
  "raw_source_body_included: false" \
  "raw_tool_inputs_included: false" \
  "raw_tool_outputs_included: false" \
  "repository_mutation_performed: false"; do
  grep -Fq "$expected" <<<"$request_body" || die "tool run request issue body missing ${expected}"
done
for leaked in "$hidden_token" "Queue a reviewed search_files tool-run request"; do
  if grep -Fq "$leaked" <<<"$request_body"; then
    die "tool run request issue body leaked ${leaked}"
  fi
done

cancel_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /tools cancel-run --id ${request_id}

Cancel the reviewed tool-run request.
Do not include this hidden cancel token: ${cancel_hidden_token}" >/dev/null

wait_for_run "issue_comment" "$cancel_started_at" >/dev/null || die "timed out waiting for tools cancel-run run"
wait_for_assistant_count 2 || die "expected tools cancel-run receipt"
cancel_receipt="$(latest_assistant_comment)"
for expected in \
  "GitClaw Tool Run Cancel Action" \
  "Generated without a model call" \
  'model="gitclaw/tools"' \
  "requested_tool_command: \`/tools cancel-run\`" \
  "tool_run_cancel_status: \`cancelled\`" \
  "tool_run_request_issue: \`#${request_issue_number}\`" \
  "cancel_comment_id: \`" \
  "cancellation_comment_posted: \`true\`" \
  "issue_closed: \`true\`" \
  "cancellation_duplicate_suppressed: \`false\`" \
  "request_not_found_or_closed: \`false\`" \
  "request_id_sha256_12: \`" \
  "cancellation_store: \`github-issue-comment-plus-closed-state\`" \
  "model_call_performed: \`false\`" \
  "tool_execution_performed: \`false\`" \
  "approval_granted: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "raw_request_id_included: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "raw_tool_inputs_included: \`false\`" \
  "raw_tool_outputs_included: \`false\`" \
  "llm_e2e_required_after_tool_run_cancel_change: \`true\`"; do
  grep -Fq "$expected" <<<"$cancel_receipt" || die "tools cancel-run receipt missing ${expected}"
done
for leaked in "$hidden_token" "$cancel_hidden_token" "$request_id" "Cancel the reviewed tool-run request"; do
  if grep -Fq "$leaked" <<<"$cancel_receipt"; then
    die "tools cancel-run receipt leaked ${leaked}"
  fi
done

request_after_cancel_json="$(gh issue view "$request_issue_number" --repo "$repo" --json state,comments)"
request_state="$(jq -r '.state' <<<"$request_after_cancel_json")"
[[ "$request_state" == "CLOSED" ]] || die "tool run request issue should be closed after cancel, got ${request_state}"
cancel_marker_count="$(jq -r '[.comments[] | select(.body | contains("gitclaw:tool-run-cancel"))] | length' <<<"$request_after_cancel_json")"
[[ "$cancel_marker_count" == "1" ]] || die "expected exactly one cancellation marker, got ${cancel_marker_count}"
cancel_marker_body="$(jq -r '[.comments[].body | select(contains("gitclaw:tool-run-cancel"))] | .[-1] // ""' <<<"$request_after_cancel_json")"
for expected in \
  "gitclaw:tool-run-cancel" \
  "id=\"${request_id}\"" \
  "source_issue=\"${source_issue_number}\"" \
  "tool_execution_performed: false" \
  "approval_granted: false" \
  "repository_mutation_performed: false" \
  "raw_source_body_included: false"; do
  grep -Fq "$expected" <<<"$cancel_marker_body" || die "tool run cancel marker missing ${expected}"
done
for leaked in "$hidden_token" "$cancel_hidden_token" "Cancel the reviewed tool-run request"; do
  if grep -Fq "$leaked" <<<"$cancel_marker_body"; then
    die "tool run cancel marker leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /tools cancel-run --id ${request_id}

Repeat the same tool-run cancellation.
Do not include this hidden duplicate token: ${duplicate_hidden_token}" >/dev/null

wait_for_run "issue_comment" "$duplicate_started_at" >/dev/null || die "timed out waiting for duplicate tools cancel-run run"
wait_for_assistant_count 3 || die "expected duplicate tools cancel-run receipt"
duplicate_receipt="$(latest_assistant_comment)"
for expected in \
  "GitClaw Tool Run Cancel Action" \
  "tool_run_cancel_status: \`not_found_or_closed\`" \
  "tool_run_request_issue: \`#0\`" \
  "cancellation_comment_posted: \`false\`" \
  "issue_closed: \`false\`" \
  "request_not_found_or_closed: \`true\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate tools cancel-run receipt missing ${expected}"
done
for leaked in "$hidden_token" "$duplicate_hidden_token" "$request_id" "Repeat the same tool-run cancellation"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate tools cancel-run receipt leaked ${leaked}"
  fi
done
request_after_duplicate_json="$(gh issue view "$request_issue_number" --repo "$repo" --json state,comments)"
[[ "$(jq -r '.state' <<<"$request_after_duplicate_json")" == "CLOSED" ]] || die "duplicate cancel reopened request issue"
cancel_marker_count="$(jq -r '[.comments[] | select(.body | contains("gitclaw:tool-run-cancel"))] | length' <<<"$request_after_duplicate_json")"
[[ "$cancel_marker_count" == "1" ]] || die "duplicate cancel changed cancellation marker count to ${cancel_marker_count}"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the tool run cancel issue action and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line ends with an all-caps token after the arrow.
That token starts with \`GITCLAW_TOOL_RUN_CANCEL_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include the request issue number, tool run cancel id, source body, requested tool text, or hidden sentinels.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run "issue_comment" "$comment_started_at")" || die "timed out waiting for tools cancel-run model follow-up"
wait_for_assistant_count 4 || die "expected model-backed tools cancel-run follow-up"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include tools cancel-run search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant tools cancel-run follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant tools cancel-run follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant tools cancel-run follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant tools cancel-run follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant tools cancel-run follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant tools cancel-run follow-up marker missing usage token telemetry"

for leaked in "$hidden_token" "$cancel_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$request_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model tools cancel-run follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}, request issue #${request_issue_number} (model follow-up: ${model_url})"
