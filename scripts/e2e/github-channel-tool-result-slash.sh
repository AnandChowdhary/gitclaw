#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-tool-result slash action records an externally observed channel tool result as a GitHub issue.
set -euo pipefail

log() {
  echo "channel-tool-result-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-tool-result-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-tool-result slash E2E appears to be running: ${lock_dir}"
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

sha256_12() {
  printf "%s" "$1" | shasum -a 256 | awk '{print substr($1, 1, 12)}'
}

ensure_label gitclaw 5319e7 "Handled by GitClaw"
ensure_label gitclaw:channel 1d76db "GitClaw mirrored channel thread"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ | tr '[:upper:]' '[:lower:]')"
channel="telegram"
thread_id="channel-tool-result-e2e-${timestamp}"
ingest_message_id="tool-result-ingest-${timestamp}"
tool_result_id="channel-tool-result-${timestamp}"
recorded_at="2037-07-19T10:11:12Z"
tool_name="gitclaw.search_files"
tool_status="success"
tool_exit_code="0"
notify_message_id="tool-result-notify-${timestamp}"
account_id="telegram-tool_result-account-NOECHO_CHANNEL_TOOL_RESULT_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_TOOL_RESULT_INGEST_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_TOOL_RESULT_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_TOOL_RESULT_FOLLOWUP_${timestamp}"
tool_result_detail_token="VISIBLE_CHANNEL_TOOL_RESULT_DETAIL_${timestamp}"
tool_result_summary="Record GitHub-native channel tool result ${timestamp}"
expected_token="GITCLAW_CHANNEL_TOOL_RESULT_CONTEXT_V1"
search_phrase="channel tool result unique search fixture phrase"
notify_message_hash="$(sha256_12 "$notify_message_id")"
issue_number=""
tool_result_issue_number=""
issue_title="GitClaw ${channel} thread ${thread_id}"

run_list_json() {
  local workflow="$1"
  local event="$2"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event "$event" \
    --limit 30 \
    --json databaseId,status,conclusion,createdAt,url,displayTitle \
    --jq '.'
}

wait_for_workflow_run() {
  local workflow="$1"
  local event="$2"
  local started_at="$3"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json "$workflow" "$event" | jq -c --arg started "$started_at" '[.[] | select(.createdAt >= $started)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${workflow} ${event} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

wait_for_issue_comment_run_for_title() {
  local started_at="$1"
  local title="$2"
  local run_json
  for _ in {1..90}; do
    run_json="$(run_list_json "$main_workflow" "issue_comment" | jq -c --arg started "$started_at" --arg title "$title" '[.[] | select(.createdAt >= $started and .displayTitle == $title)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "issue_comment run for ${title} failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

find_issue_number() {
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:channel \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg thread "$thread_id" '.[] | select((.title | contains($thread)) or (.body | contains($thread))) | .number' \
    | head -n 1
}

wait_for_issue_number() {
  for _ in {1..30}; do
    local number
    number="$(find_issue_number)"
    if [[ -n "$number" && "$number" != "null" ]]; then
      echo "$number"
      return 0
    fi
    sleep 2
  done
  return 1
}

assistant_count_for_issue() {
  local number="$1"
  gh issue view "$number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

error_count_for_issue() {
  local number="$1"
  gh issue view "$number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("<!-- gitclaw:error"))] | length'
}

wait_for_assistant_count_for_issue() {
  local number="$1"
  local want="$2"
  for _ in {1..90}; do
    local errors
    errors="$(error_count_for_issue "$number")"
    if [[ "$errors" != "0" ]]; then
      die "issue #${number} posted ${errors} error marker comment(s)"
    fi
    local got
    got="$(assistant_count_for_issue "$number")"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

latest_assistant_comment_for_issue() {
  local number="$1"
  gh issue view "$number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

tool_result_notification_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$notify_message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

cleanup() {
  for number in "$issue_number" "$tool_result_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel-tool-result slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel-tool-result slash E2E.

Hidden ingest token: ${ingest_hidden_token}"

ingest_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$ingest_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f thread_id="$thread_id" \
  -f message_id="$ingest_message_id" \
  -f author="telegram:e2e" \
  -f body="$ingest_body"

wait_for_workflow_run "$ingest_workflow" "workflow_dispatch" "$ingest_started_at" >/dev/null || die "timed out waiting for channel-ingest workflow"
issue_number="$(wait_for_issue_number)" || die "timed out finding channel issue for ${thread_id}"
log "channel ingest created issue #${issue_number}"

wait_for_workflow_run "$main_workflow" "workflow_dispatch" "$ingest_started_at" >/dev/null || die "timed out waiting for initial channel report workflow"
wait_for_assistant_count_for_issue "$issue_number" 1 || die "expected initial channel report"
initial_report="$(latest_assistant_comment_for_issue "$issue_number")"
grep -Fq "GitClaw Channel Report" <<<"$initial_report" || die "initial assistant comment missing channel report"
grep -Fq 'channel_thread_issue: `true`' <<<"$initial_report" || die "initial channel report missing channel thread status"
if grep -Fq "$ingest_hidden_token" <<<"$initial_report"; then
  die "initial channel report leaked ingest hidden token"
fi

tool_result_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels tool-result --result-id ${tool_result_id} --tool ${tool_name} --status ${tool_status} --exit-code ${tool_exit_code} --recorded-at ${recorded_at} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id}
Summary: ${tool_result_summary}
Result:
Visible tool-result detail token: ${tool_result_detail_token}" >/dev/null

wait_for_issue_comment_run_for_title "$tool_result_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel tool-result action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel tool-result action receipt"
tool_result_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Tool Result Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels tool-result\`" \
  "channel_tool_result_status: \`recorded\`" \
  "tool_result_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "raw_tool_result_id_included: \`false\`" \
  "tool_name_sha256_12:" \
  "tool_status_sha256_12:" \
  "tool_exit_code_sha256_12:" \
  "raw_tool_name_included: \`false\`" \
  "raw_tool_status_included: \`false\`" \
  "raw_tool_exit_code_included: \`false\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "recorded_at_sha256_12:" \
  "recorded_at_bytes: \`20\`" \
  "recorded_at_lines: \`1\`" \
  "raw_recorded_at_included: \`false\`" \
  "raw_tool_result_summary_included: \`false\`" \
  "raw_tool_result_details_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "tool_execution_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "llm_e2e_required_after_channel_tool_result_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$tool_result_receipt" || die "channel tool-result receipt missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$tool_result_detail_token" "$tool_result_summary" "$recorded_at" "$tool_result_id" "$tool_name" "$tool_status" "$thread_id" "$ingest_message_id" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$tool_result_receipt"; then
    die "channel tool-result receipt leaked ${leaked}"
  fi
done

tool_result_issue_number="$(sed -n 's/.*tool_result_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$tool_result_receipt" | head -n 1)"
[[ -n "$tool_result_issue_number" && "$tool_result_issue_number" != "null" ]] || die "could not resolve tool-result issue number"
log "channel tool-result created tool-result issue #${tool_result_issue_number}"

[[ "$(tool_result_notification_count)" == "1" ]] || die "channel tool-result did not queue exactly one tool-result-link outbound comment"
tool_result_json="$(gh issue view "$tool_result_issue_number" --repo "$repo" --json title,body,labels)"
tool_result_issue_title="$(jq -r '.title' <<<"$tool_result_json")"
tool_result_body="$(jq -r '.body' <<<"$tool_result_json")"
tool_result_labels="$(jq -r '.labels[].name' <<<"$tool_result_json")"
grep -Fxq "gitclaw" <<<"$tool_result_labels" || die "tool-result issue missing gitclaw label"
for expected in \
  "gitclaw:channel-tool-result" \
  "result_id: ${tool_result_id}" \
  "tool_name: ${tool_name}" \
  "status: ${tool_status}" \
  "exit_code: ${tool_exit_code}" \
  "recorded_at: ${recorded_at}" \
  "source_channel: ${channel}" \
  "source_issue: #${issue_number}" \
  "source_message_id_sha256_12:" \
  "tool_result_mode: github-issue-tool-result" \
  "tool_execution_performed: false" \
  "repository_mutation_performed: false" \
  "raw_thread_id_included: false" \
  "raw_source_message_id_included: false" \
  "## Tool" \
  "## Status" \
  "## Recorded At" \
  "## Summary" \
  "## Result Details" \
  "${tool_result_summary}" \
  "${tool_result_detail_token}"; do
  grep -Fq "$expected" <<<"$tool_result_body" || die "tool-result issue body missing ${expected}"
done
for leaked in "$thread_id" "$ingest_message_id" "$ingest_hidden_token"; do
  if grep -Fq "$leaked" <<<"$tool_result_body"; then
    die "tool-result issue body leaked ${leaked}"
  fi
done

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
outbound_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound"))] | join("\n")' <<<"$issue_json")"
grep -Fq "gitclaw:channel-outbound" <<<"$outbound_bodies" || die "channel issue missing tool-result-link outbound marker"
grep -Fq "Tool: ${tool_name}" <<<"$outbound_bodies" || die "channel issue missing visible tool notification"
grep -Fq "Status: ${tool_status}" <<<"$outbound_bodies" || die "channel issue missing visible tool status notification"
grep -Fq "Summary: ${tool_result_summary}" <<<"$outbound_bodies" || die "channel issue missing visible tool-result summary notification"
if grep -Fq "$tool_result_detail_token" <<<"$outbound_bodies"; then
  die "channel tool-result notification leaked detail token"
fi

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing tool-result-link outbound hash ${notify_message_hash}"
for leaked in "$tool_result_summary" "$recorded_at" "$tool_name" "$tool_status" "$tool_result_detail_token" "$account_id" "$ingest_hidden_token"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels tool-output ${tool_name} --result-id ${tool_result_id} --status ${tool_status} --exit-code ${tool_exit_code} --recorded-at ${recorded_at} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id}
Summary: ${tool_result_summary}
Result:
Do not leak duplicate hidden token: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel tool-result action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel tool-result receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Tool Result Action" \
  "requested_channel_command: \`/channels tool-output\`" \
  "channel_tool_result_status: \`duplicate\`" \
  "tool_result_issue: \`#${tool_result_issue_number}\`" \
  "tool_result_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "raw_tool_result_details_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel tool-result receipt missing ${expected}"
done
[[ "$(tool_result_notification_count)" == "1" ]] || die "duplicate channel tool-result queued another tool-result-link notification"
for leaked in "$duplicate_hidden_token" "$tool_result_id" "$thread_id" "$ingest_message_id" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel tool-result receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$tool_result_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel-created tool result and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_TOOL_RESULT_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include tool result ids, recorded timestamps, statuses, message ids, thread ids, account hashes, issue numbers, tool result summaries, or result details.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$tool_result_issue_title")" || die "timed out waiting for channel tool-result model follow-up"
wait_for_assistant_count_for_issue "$tool_result_issue_number" 1 || die "expected model-backed channel tool-result follow-up"
model_comment="$(latest_assistant_comment_for_issue "$tool_result_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel tool-result search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel tool-result follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel tool-result follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel tool-result follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel tool-result follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel tool-result follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel tool-result follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$account_id" "$tool_result_id" "$recorded_at" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel tool-result follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number}, tool-result issue #${tool_result_issue_number} (model follow-up: ${model_url})"
