#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-done slash action closes a channel artifact and acknowledges it.
set -euo pipefail

log() {
  echo "channel-done-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-done-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-done slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-done-e2e-${timestamp}"
ingest_message_id="done-ingest-${timestamp}"
task_id="channel-done-task-${timestamp}"
task_notify_message_id="done-task-notify-${timestamp}"
done_notify_message_id="done-complete-notify-${timestamp}"
account_id="telegram-done-account-NOECHO_CHANNEL_DONE_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_DONE_INGEST_${timestamp}"
done_hidden_token="NOECHO_CHANNEL_DONE_BODY_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_DONE_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_DONE_FOLLOWUP_${timestamp}"
task_notes_token="VISIBLE_CHANNEL_DONE_TASK_NOTES_${timestamp}"
task_title="Complete channel done task ${timestamp}"
expected_token="GITCLAW_CHANNEL_DONE_CONTEXT_V1"
search_phrase="channel done unique search fixture phrase"
done_notify_hash="$(sha256_12 "$done_notify_message_id")"
issue_number=""
task_issue_number=""
issue_title="GitClaw ${channel} thread ${thread_id}"
task_issue_title=""

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
  for _ in {1..120}; do
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

outbound_count_for_message() {
  local message_id="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

cleanup() {
  for number in "$issue_number" "$task_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel-done slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel-done slash E2E.

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
if grep -Fq "$ingest_hidden_token" <<<"$initial_report"; then
  die "initial channel report leaked ingest hidden token"
fi

task_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels task --task-id ${task_id} --message-id ${ingest_message_id} --notify-message-id ${task_notify_message_id}
Title: ${task_title}
Notes:
Visible task note token: ${task_notes_token}" >/dev/null

wait_for_issue_comment_run_for_title "$task_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel task action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel task action receipt"
task_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Task Action" \
  "channel_task_status: \`created\`" \
  "notification_queued: \`true\`" \
  "raw_task_title_included: \`false\`" \
  "raw_task_notes_included: \`false\`"; do
  grep -Fq "$expected" <<<"$task_receipt" || die "channel task receipt missing ${expected}"
done
task_issue_number="$(sed -n 's/.*task_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$task_receipt" | head -n 1)"
[[ -n "$task_issue_number" && "$task_issue_number" != "null" ]] || die "could not resolve task issue number"
task_issue_title="$(gh issue view "$task_issue_number" --repo "$repo" --json title --jq .title)"
log "channel task created task issue #${task_issue_number}"

done_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$task_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels done --message-id ${done_notify_message_id}
Do not leak this done hidden token: ${done_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$done_started_at" "$task_issue_title" >/dev/null || die "timed out waiting for channel done action"
wait_for_assistant_count_for_issue "$task_issue_number" 1 || die "expected channel done action receipt"
done_receipt="$(latest_assistant_comment_for_issue "$task_issue_number")"
for expected in \
  "GitClaw Channel Done Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels done\`" \
  "channel_artifact_kind: \`task\`" \
  "channel_artifact_issue: \`#${task_issue_number}\`" \
  "channel_artifact_closed: \`true\`" \
  "source_channel_issue: \`#${issue_number}\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "raw_artifact_id_included: \`false\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_artifact_title_included: \`false\`" \
  "raw_artifact_body_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "llm_e2e_required_after_channel_done_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$done_receipt" || die "channel done receipt missing ${expected}"
done
for leaked in "$task_id" "$thread_id" "$ingest_message_id" "$done_notify_message_id" "$task_title" "$task_notes_token" "$done_hidden_token"; do
  if grep -Fq "$leaked" <<<"$done_receipt"; then
    die "channel done receipt leaked ${leaked}"
  fi
done
task_state="$(gh issue view "$task_issue_number" --repo "$repo" --json state --jq .state)"
[[ "$task_state" == "CLOSED" ]] || die "channel done did not close task issue; state=${task_state}"
[[ "$(outbound_count_for_message "$done_notify_message_id")" == "1" ]] || die "channel done did not queue exactly one done acknowledgement"

source_json="$(gh issue view "$issue_number" --repo "$repo" --json comments)"
done_outbound="$(jq -r --arg msg "$done_notify_message_id" '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound") and contains($msg))] | join("\n")' <<<"$source_json")"
for expected in \
  "GitClaw channel task completed" \
  "Artifact issue: #${task_issue_number} https://github.com/${repo}/issues/${task_issue_number}" \
  "Kind: task" \
  "State: closed" \
  "Provider delivery performed: false"; do
  grep -Fq "$expected" <<<"$done_outbound" || die "done acknowledgement missing ${expected}"
done
for leaked in "$task_title" "$task_notes_token" "$done_hidden_token"; do
  if grep -Fq "$leaked" <<<"$done_outbound"; then
    die "done acknowledgement leaked ${leaked}"
  fi
done

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$done_notify_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing done acknowledgement hash ${done_notify_hash}"
for leaked in "$task_title" "$task_notes_token" "$done_hidden_token" "$account_id"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$task_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels done --message-id ${done_notify_message_id}
Do not leak duplicate done token: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$task_issue_title" >/dev/null || die "timed out waiting for duplicate channel done action"
wait_for_assistant_count_for_issue "$task_issue_number" 2 || die "expected duplicate channel done receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$task_issue_number")"
for expected in \
  "GitClaw Channel Done Action" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "channel_artifact_closed: \`true\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel done receipt missing ${expected}"
done
[[ "$(outbound_count_for_message "$done_notify_message_id")" == "1" ]] || die "duplicate channel done queued another acknowledgement"
for leaked in "$duplicate_hidden_token" "$task_id" "$thread_id" "$done_notify_message_id"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel done receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the channel done acknowledgement and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_DONE_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include task ids, message ids, thread ids, account hashes, issue numbers, task titles, or notes.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$issue_title")" || die "timed out waiting for channel done model follow-up"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected model-backed channel done follow-up"
model_comment="$(latest_assistant_comment_for_issue "$issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel done search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel done follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel done follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel done follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel done follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel done follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel done follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$done_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$account_id" "$task_id" "$done_notify_message_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel done follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number}, task issue #${task_issue_number} (model follow-up: ${model_url})"
