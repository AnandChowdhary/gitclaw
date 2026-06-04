#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel arcade queues provider-visible play menus and proves model/tool follow-up.
set -euo pipefail

log() {
  echo "channel-arcade-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-arcade-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-arcade slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-arcade-e2e-${timestamp}"
ingest_message_id="arcade-ingest-${timestamp}"
notify_message_id="arcade-notify-${timestamp}"
arcade_id="arcade-${timestamp}"
arcade_mode="fun"
arcade_note="Pick the room move"
account_id="telegram-arcade-account-NOECHO_CHANNEL_ARCADE_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_ARCADE_INGEST_${timestamp}"
command_hidden_token="NOECHO_CHANNEL_ARCADE_COMMAND_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_ARCADE_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_ARCADE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_ARCADE_CONTEXT_V1"
search_phrase="channel arcade unique search fixture phrase"
notify_message_hash="$(sha256_12 "$notify_message_id")"
issue_number=""
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
    local errors got
    errors="$(error_count_for_issue "$number")"
    if [[ "$errors" != "0" ]]; then
      die "issue #${number} posted ${errors} error marker comment(s)"
    fi
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

arcade_notification_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$notify_message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

cleanup() {
  if [[ -n "${issue_number:-}" && "$issue_number" != "null" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "channel-arcade slash e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel-arcade slash E2E.

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

initial_report=""
for _ in {1..90}; do
  errors="$(error_count_for_issue "$issue_number")"
  if [[ "$errors" != "0" ]]; then
    die "issue #${issue_number} posted ${errors} error marker comment(s)"
  fi
  candidate_report="$(latest_assistant_comment_for_issue "$issue_number")"
  if grep -Fq -- "$ingest_hidden_token" <<<"$candidate_report"; then
    die "initial channel report leaked ingest hidden token"
  fi
  if grep -Fq -- "GitClaw Channel Report" <<<"$candidate_report" && grep -Fq -- 'channel_thread_issue: `true`' <<<"$candidate_report"; then
    initial_report="$candidate_report"
    break
  fi
  sleep 5
done
[[ -n "$initial_report" ]] || die "expected initial channel report"

arcade_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels arcade ${arcade_mode} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --arcade-id ${arcade_id}
Note: ${arcade_note}
Do not include this command hidden token in the receipt: ${command_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$arcade_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel arcade action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel arcade action receipt"
arcade_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Arcade Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels arcade\`" \
  "channel_arcade_status: \`queued\`" \
  "arcade_card_mode: \`bounded-channel-play-menu\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "arcade_id_sha256_12: \`" \
  "arcade_id_auto: \`false\`" \
  "arcade_mode_sha256_12: \`" \
  "arcade_mode_bytes: \`3\`" \
  "arcade_move_count: \`4\`" \
  "arcade_move_sha256_12: \`" \
  "arcade_note_sha256_12: \`" \
  "arcade_note_bytes: \`18\`" \
  "arcade_note_lines: \`1\`" \
  "arcade_note_source: \`trailing-note\`" \
  "notification_body_sha256_12: \`" \
  "model_call_performed: \`false\`" \
  "dynamic_play_generation_performed: \`false\`" \
  "external_randomness_used: \`false\`" \
  "game_state_persisted: \`false\`" \
  "score_tracking_performed: \`false\`" \
  "command_execution_performed: \`false\`" \
  "skill_install_performed: \`false\`" \
  "tool_execution_performed: \`false\`" \
  "backup_payload_read: \`false\`" \
  "soul_body_read: \`false\`" \
  "memory_write_performed: \`false\`" \
  "provider_api_call_performed: \`false\`" \
  "workflow_mutation_performed: \`false\`" \
  "policy_mutation_performed: \`false\`" \
  "schedule_created: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox + channel-delivery\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_arcade_id_included: \`false\`" \
  "raw_arcade_mode_included: \`false\`" \
  "raw_arcade_note_included: \`false\`" \
  "raw_arcade_moves_included: \`false\`" \
  "raw_arcade_commands_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "raw_issue_bodies_included: \`false\`" \
  "raw_comment_bodies_included: \`false\`" \
  "raw_prompts_included: \`false\`" \
  "raw_tool_outputs_included: \`false\`" \
  "llm_e2e_required_after_channel_arcade_action_change: \`true\`"; do
  grep -Fq -- "$expected" <<<"$arcade_receipt" || die "channel arcade receipt missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$arcade_id" "$arcade_mode" "$arcade_note" "$expected_token" "Story dice" "start a tiny prompt-game card"; do
  if grep -Fqi -- "$leaked" <<<"$arcade_receipt"; then
    die "channel arcade receipt leaked ${leaked}"
  fi
done

[[ "$(arcade_notification_count)" == "1" ]] || die "channel arcade did not queue exactly one notification"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq -- "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
notification_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound") and contains("'"${notify_message_id}"'"))] | join("\n")' <<<"$issue_json")"
for expected in \
  "GitClaw channel arcade." \
  "Mode: ${arcade_mode}" \
  "Frame: Pick one bounded move; GitHub keeps the receipt." \
  "Moves:" \
  "Story dice [ready] - start a tiny prompt-game card" \
  "Try: \`@gitclaw /channels story-dice fun --story-dice-id <id> --message-id <id> --notify-message-id <id>\`" \
  "Spark [ready] - turn the room toward one experiment" \
  "Try: \`@gitclaw /channels spark --spark-id <id> --message-id <id> --notify-message-id <id>\`" \
  "Postcard [ready] - send a small scene card" \
  "Try: \`@gitclaw /channels postcard launch-ready --postcard-id <id> --message-id <id> --notify-message-id <id>\`" \
  "Cockpit [watch] - switch from play to operator view" \
  "Try: \`@gitclaw /channels cockpit fun --cockpit-id <id> --message-id <id> --notify-message-id <id>\`" \
  "Note: ${arcade_note}" \
  "Arcade hash: " \
  "Move hash: " \
  "Note hash: " \
  "Arcade persistence: advisory only; no score or game state changed." \
  "Arcade source: bounded GitHub channel action deck." \
  "Model call: not performed by this action." \
  "Dynamic play generation: not performed by this action." \
  "External randomness: not used by this action." \
  "Game-state persistence: not performed by this action." \
  "Score tracking: not performed by this action." \
  "Command execution: not performed by this action." \
  "Skill install: not performed by this action." \
  "Tool execution: not performed by this action." \
  "Backup payload read: not performed by this action." \
  "Soul body read: not performed by this action." \
  "Memory write: not performed by this action." \
  "Workflow mutation: not performed by this action." \
  "Policy mutation: not performed by this action." \
  "Schedule creation: not performed by this action." \
  "Repository mutation: not performed by this action." \
  "Provider API call: not performed by this action." \
  "Provider delivery: queued through GitHub channel outbox."; do
  grep -Fq -- "$expected" <<<"$notification_bodies" || die "arcade notification missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$arcade_id" "$expected_token"; do
  if grep -Fq -- "$leaked" <<<"$notification_bodies"; then
    die "arcade notification leaked ${leaked}"
  fi
done

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq -- "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq -- "outbound_comments=1" <<<"$outbox_output" || die "channel outbox output missing outbound count: ${outbox_output}"
grep -Fq -- "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing arcade notify hash ${notify_message_hash}"
for leaked in "$account_id" "$ingest_hidden_token" "$command_hidden_token" "$arcade_id" "$arcade_mode" "$arcade_note" "$expected_token" "Story dice" "start a tiny prompt-game card"; do
  if grep -Fqi -- "$leaked" <<<"$outbox_output" || grep -Fqi -- "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels play-menu ${arcade_mode} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --arcade-id ${arcade_id}
Note: ${arcade_note}
Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel arcade action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel arcade receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Arcade Action" \
  "requested_channel_command: \`/channels play-menu\`" \
  "channel_arcade_status: \`duplicate\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "model_call_performed: \`false\`" \
  "dynamic_play_generation_performed: \`false\`" \
  "external_randomness_used: \`false\`" \
  "game_state_persisted: \`false\`" \
  "score_tracking_performed: \`false\`" \
  "command_execution_performed: \`false\`" \
  "skill_install_performed: \`false\`" \
  "tool_execution_performed: \`false\`" \
  "backup_payload_read: \`false\`" \
  "soul_body_read: \`false\`" \
  "memory_write_performed: \`false\`" \
  "provider_api_call_performed: \`false\`" \
  "workflow_mutation_performed: \`false\`" \
  "policy_mutation_performed: \`false\`" \
  "schedule_created: \`false\`" \
  "repository_mutation_performed: \`false\`"; do
  grep -Fq -- "$expected" <<<"$duplicate_receipt" || die "duplicate channel arcade receipt missing ${expected}"
done
[[ "$(arcade_notification_count)" == "1" ]] || die "duplicate channel arcade queued another notification"
for leaked in "$duplicate_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$arcade_id" "$arcade_mode" "$arcade_note" "$expected_token" "Story dice" "start a tiny prompt-game card"; do
  if grep -Fqi -- "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel arcade receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel arcade thread and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_ARCADE_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include provider ids, notification ids, thread ids, message ids, account hashes, arcade ids, arcade modes, arcade notes, move text, command text, issue numbers, or previous channel bodies.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$issue_title")" || die "timed out waiting for channel arcade model follow-up"
wait_for_assistant_count_for_issue "$issue_number" 4 || die "expected model-backed channel arcade follow-up"
model_comment="$(latest_assistant_comment_for_issue "$issue_number")"

grep -Fq -- "$expected_token" <<<"$model_comment" || die "assistant did not include channel arcade search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel arcade follow-up marker did not use configured GitHub models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel arcade follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel arcade follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel arcade follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel arcade follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel arcade follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$arcade_id" "$arcade_note" "$account_id"; do
  if grep -Fq -- "$leaked" <<<"$model_comment"; then
    die "model channel arcade follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number} (model follow-up: ${model_url})"
