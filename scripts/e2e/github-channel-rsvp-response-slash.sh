#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-rsvp-response slash action records a channel-origin RSVP reply.
set -euo pipefail

log() {
  echo "channel-rsvp-response-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-rsvp-response-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-rsvp-response slash E2E appears to be running: ${lock_dir}"
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
ensure_label gitclaw:channel 1d76db "GitClaw mirrored channel thread"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
route_a="e2e-slack-route"
route_b="e2e-telegram-route"
rsvp_id="rsvp-response-${timestamp}"
invite_message_id="rsvp-response-invite-${timestamp}"
response_message_id="rsvp-response-msg-${timestamp}"
ack_message_id="rsvp-response-ack-${timestamp}"
slack_account_id="slack-rsvp-response-account-NOECHO_CHANNEL_RSVP_RESPONSE_SLACK_${timestamp}"
telegram_account_id="telegram-rsvp-response-account-NOECHO_CHANNEL_RSVP_RESPONSE_TELEGRAM_${timestamp}"
details_token="NOECHO_CHANNEL_RSVP_RESPONSE_DETAILS_${timestamp}"
note_token="NOECHO_CHANNEL_RSVP_RESPONSE_NOTE_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_RSVP_RESPONSE_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_RSVP_RESPONSE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_RSVP_RESPONSE_CONTEXT_V1"
search_phrase="channel rsvp response unique search fixture phrase"
source_title="GitClaw channel RSVP response E2E ${timestamp}"
rsvp_title="Tiny channel RSVP response fixture ${timestamp}"
rsvp_when="2026-06-04T16:00:00Z"
rsvp_where="E2E response room ${timestamp}"
rsvp_host="GitClaw E2E"
rsvp_details="Bring one clear yes/no/maybe answer and ${details_token}."
responder_name="RSVP Response Tester ${timestamp}"
response_note="Please record this channel-origin note: ${note_token}."
source_issue_number=""
rsvp_issue_number=""
target_issue_number=""
target_issue_title=""
target_channel=""
target_account_id=""
target_issues=()

run_list_json() {
  local event="$1"
  local started_at="$2"
  gh run list \
    --repo "$repo" \
    --workflow "$main_workflow" \
    --event "$event" \
    --created ">=$started_at" \
    --limit 30 \
    --json databaseId,status,conclusion,createdAt,url,displayTitle \
    --jq '.'
}

wait_for_main_run() {
  local event="$1"
  local started_at="$2"
  local title="$3"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json "$event" "$started_at" | jq -c --arg title "$title" '[.[] | select(.displayTitle == $title)] | sort_by(.createdAt) | reverse | .[0] // empty')"
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

assistant_count_for_issue() {
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

error_count_for_issue() {
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("<!-- gitclaw:error"))] | length'
}

wait_for_assistant_count_for_issue() {
  local issue_number="$1"
  local want="$2"
  for _ in {1..90}; do
    local errors
    errors="$(error_count_for_issue "$issue_number")"
    if [[ "$errors" != "0" ]]; then
      die "issue #${issue_number} posted ${errors} error marker comment(s)"
    fi
    local got
    got="$(assistant_count_for_issue "$issue_number")"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

latest_assistant_comment_for_issue() {
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

outbound_comment_count() {
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:channel-outbound"))] | length'
}

rsvp_response_record_count() {
  gh issue view "$rsvp_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:channel-rsvp-response"))] | length'
}

latest_rsvp_response_record() {
  gh issue view "$rsvp_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:channel-rsvp-response")) | .body] | .[-1] // ""'
}

cleanup() {
  local numbers=("$source_issue_number" "$rsvp_issue_number" "${target_issues[@]:-}")
  for number in "${numbers[@]}"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel-rsvp-response slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

opened_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
source_issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$source_title" \
  --body "@gitclaw /channels rsvp ${route_a},${route_b} --rsvp-id ${rsvp_id} --message-id ${invite_message_id}

Title: ${rsvp_title}
When: ${rsvp_when}
Where: ${rsvp_where}
Host: ${rsvp_host}
Details:
${rsvp_details}")"
source_issue_number="${source_issue_url##*/}"

wait_for_main_run "issues" "$opened_started_at" "$source_title" >/dev/null || die "timed out waiting for source issue channel-rsvp action"
wait_for_assistant_count_for_issue "$source_issue_number" 1 || die "expected one channel-rsvp action receipt"
receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"

grep -Fq "GitClaw Channel RSVP Action" <<<"$receipt" || die "source receipt missing RSVP action"
grep -Fq "channel_rsvp_status: \`queued\`" <<<"$receipt" || die "source RSVP did not queue routes"
grep -Fq "rsvp_invites_queued: \`2\`" <<<"$receipt" || die "source RSVP did not queue two invites"
for leaked in "$details_token" "$route_a" "$route_b" "$invite_message_id" "$rsvp_id" "$rsvp_title"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "channel-rsvp source receipt leaked ${leaked}"
  fi
done

rsvp_issue_number="$(sed -n 's/.*rsvp_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -1)"
[[ -n "$rsvp_issue_number" ]] || die "could not parse RSVP issue number from receipt"
mapfile -t target_issues < <(sed -n 's/.*target_issue=`#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | awk '!seen[$0]++')
[[ "${#target_issues[@]}" == "2" ]] || die "expected two RSVP target issues, got ${#target_issues[@]} from receipt"

rsvp_json="$(gh issue view "$rsvp_issue_number" --repo "$repo" --json title,body,labels)"
actual_rsvp_title="$(jq -r '.title' <<<"$rsvp_json")"
rsvp_body="$(jq -r '.body' <<<"$rsvp_json")"
grep -Fq "gitclaw:channel-rsvp" <<<"$rsvp_body" || die "RSVP issue missing RSVP marker"
grep -Fq "$rsvp_details" <<<"$rsvp_body" || die "RSVP issue missing visible details"

target_issue_number="${target_issues[0]}"
target_json="$(gh issue view "$target_issue_number" --repo "$repo" --json title,body,labels,comments)"
target_issue_title="$(jq -r '.title' <<<"$target_json")"
target_body="$(jq -r '.body' <<<"$target_json")"
target_labels="$(jq -r '.labels[].name' <<<"$target_json")"
grep -Fxq "gitclaw:channel" <<<"$target_labels" || die "target channel issue #${target_issue_number} missing gitclaw:channel label"
if grep -Fq 'channel="slack"' <<<"$target_body"; then
  target_channel="slack"
  target_account_id="$slack_account_id"
elif grep -Fq 'channel="telegram"' <<<"$target_body"; then
  target_channel="telegram"
  target_account_id="$telegram_account_id"
else
  die "target channel issue #${target_issue_number} missing expected provider marker"
fi
[[ "$(outbound_comment_count "$target_issue_number")" == "1" ]] || die "target issue should start with one RSVP invite outbound comment"

response_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$target_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels rsvp-response --rsvp-id ${rsvp_id} --message-id ${response_message_id} --notify-message-id ${ack_message_id} --response yes
Responder: ${responder_name}
Note:
${response_note}" >/dev/null

wait_for_main_run "issue_comment" "$response_started_at" "$target_issue_title" >/dev/null || die "timed out waiting for channel-rsvp-response issue_comment action"
wait_for_assistant_count_for_issue "$target_issue_number" 1 || die "expected one channel-rsvp-response action receipt"
response_receipt="$(latest_assistant_comment_for_issue "$target_issue_number")"

for expected in \
  "GitClaw Channel RSVP Response Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels rsvp-response\`" \
  "channel_rsvp_response_status: \`recorded\`" \
  "rsvp_issue: \`#${rsvp_issue_number}\`" \
  "response_recorded: \`true\`" \
  "response_duplicate_suppressed: \`false\`" \
  "notification_target_issue: \`#${target_issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "channel: \`${target_channel}\`" \
  "response_id_auto: \`false\`" \
  "notify_message_id_auto: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "raw_rsvp_id_included: \`false\`" \
  "raw_response_id_included: \`false\`" \
  "raw_response_included: \`false\`" \
  "raw_responder_included: \`false\`" \
  "raw_note_included: \`false\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox + channel-delivery\`" \
  "llm_e2e_required_after_channel_rsvp_response_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$response_receipt" || die "channel-rsvp-response receipt missing ${expected}"
done
for leaked in "$note_token" "$responder_name" "$response_message_id" "$ack_message_id" "$rsvp_id" "$response_note"; do
  if grep -Fq "$leaked" <<<"$response_receipt"; then
    die "channel-rsvp-response receipt leaked ${leaked}"
  fi
done

[[ "$(rsvp_response_record_count)" == "1" ]] || die "expected exactly one RSVP response record"
response_record="$(latest_rsvp_response_record)"
for expected in \
  "gitclaw:channel-rsvp-response" \
  "rsvp_id=\"${rsvp_id}\"" \
  "response_id=\"${response_message_id}\"" \
  "- response: yes" \
  "- source_channel: ${target_channel}" \
  "- source_issue: #${target_issue_number}" \
  "${responder_name}" \
  "${response_note}" \
  "raw_thread_id_included: false" \
  "raw_source_message_id_included: false"; do
  grep -Fq -- "$expected" <<<"$response_record" || die "RSVP response record missing ${expected}"
done

target_after_response="$(gh issue view "$target_issue_number" --repo "$repo" --json comments --jq '[.comments[].body] | join("\n")')"
[[ "$(outbound_comment_count "$target_issue_number")" == "2" ]] || die "RSVP response did not queue acknowledgement outbound comment"
for expected in \
  "message_id=\"${ack_message_id}\"" \
  "GitClaw RSVP response recorded" \
  "RSVP: #${rsvp_issue_number}" \
  "Response: yes" \
  "Participant: ${responder_name}"; do
  grep -Fq "$expected" <<<"$target_after_response" || die "RSVP response acknowledgement missing ${expected}"
done
if grep -Fq "$note_token" <<<"$target_after_response"; then
  die "RSVP response acknowledgement leaked note token"
fi

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$target_channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$target_account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$target_issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "pending=2" <<<"$outbox_output" || die "channel outbox did not report two pending messages after RSVP response: ${outbox_output}"
grep -Fq '"pending_messages": 2' "$outbox_file" || die "channel outbox file missing two pending messages after RSVP response"
for leaked in "$details_token" "$note_token" "$responder_name"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "channel outbox leaked ${leaked} without --include-body"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$target_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels rsvp-response --rsvp-id ${rsvp_id} --message-id ${response_message_id} --notify-message-id ${ack_message_id} --response yes
Note: duplicate should not leak ${duplicate_hidden_token}" >/dev/null

wait_for_main_run "issue_comment" "$duplicate_started_at" "$target_issue_title" >/dev/null || die "timed out waiting for duplicate channel-rsvp-response action"
wait_for_assistant_count_for_issue "$target_issue_number" 2 || die "expected duplicate channel-rsvp-response receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$target_issue_number")"
for expected in \
  "GitClaw Channel RSVP Response Action" \
  "channel_rsvp_response_status: \`duplicate\`" \
  "response_recorded: \`false\`" \
  "response_duplicate_suppressed: \`true\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "raw_note_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel-rsvp-response receipt missing ${expected}"
done
[[ "$(rsvp_response_record_count)" == "1" ]] || die "duplicate RSVP response posted another response record"
[[ "$(outbound_comment_count "$target_issue_number")" == "2" ]] || die "duplicate RSVP response queued another acknowledgement"
for leaked in "$duplicate_hidden_token" "$response_message_id" "$ack_message_id" "$rsvp_id"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel-rsvp-response receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$rsvp_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel RSVP response thread and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line ends with an all-caps token after the arrow.
That token starts with \`GITCLAW_CHANNEL_RSVP_RESPONSE_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include route names, target issue numbers, message ids, account hashes, source issue title, RSVP id, event title, event time, location, host, details, responder name, or response note.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_main_run "issue_comment" "$comment_started_at" "$actual_rsvp_title")" || die "timed out waiting for channel-rsvp-response model follow-up"
wait_for_assistant_count_for_issue "$rsvp_issue_number" 1 || die "expected model-backed RSVP response follow-up"
model_comment="$(latest_assistant_comment_for_issue "$rsvp_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include RSVP response search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant RSVP response follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant RSVP response follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant RSVP response follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant RSVP response follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant RSVP response follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant RSVP response follow-up marker missing usage token telemetry"

for leaked in "$details_token" "$note_token" "$duplicate_hidden_token" "$followup_hidden_token" "$slack_account_id" "$telegram_account_id" "$rsvp_id" "$responder_name"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model RSVP response follow-up leaked ${leaked}"
  fi
done

url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}, RSVP issue #${rsvp_issue_number}, target issue #${target_issue_number}: ${url}"
