#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-poll-vote slash action records a channel-origin poll vote.
set -euo pipefail

log() {
  echo "channel-poll-vote-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-poll-vote-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-poll-vote slash E2E appears to be running: ${lock_dir}"
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
poll_id="poll-vote-${timestamp}"
invite_message_id="poll-vote-invite-${timestamp}"
vote_message_id="poll-vote-msg-${timestamp}"
ack_message_id="poll-vote-ack-${timestamp}"
poll_id_marker="$(tr '[:upper:]' '[:lower:]' <<<"$poll_id")"
vote_id_marker="$(tr '[:upper:]' '[:lower:]' <<<"$vote_message_id")"
slack_account_id="slack-poll-vote-account-NOECHO_CHANNEL_POLL_VOTE_SLACK_${timestamp}"
telegram_account_id="telegram-poll-vote-account-NOECHO_CHANNEL_POLL_VOTE_TELEGRAM_${timestamp}"
question_token="NOECHO_CHANNEL_POLL_VOTE_QUESTION_${timestamp}"
note_token="NOECHO_CHANNEL_POLL_VOTE_NOTE_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_POLL_VOTE_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_POLL_VOTE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_POLL_VOTE_CONTEXT_V1"
search_phrase="channel poll vote unique search fixture phrase"
source_title="GitClaw channel poll vote E2E ${timestamp}"
poll_question="Which channel poll vote path should ship for ${question_token}?"
option_one="Ship the poll vote path"
option_two="Keep the poll invite only"
voter_name="Poll Vote Tester ${timestamp}"
vote_note="Please record this channel-origin vote note: ${note_token}."
source_issue_number=""
poll_issue_number=""
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

poll_vote_record_count() {
  gh issue view "$poll_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:channel-poll-vote"))] | length'
}

latest_poll_vote_record() {
  gh issue view "$poll_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:channel-poll-vote")) | .body] | .[-1] // ""'
}

cleanup() {
  local numbers=("$source_issue_number" "$poll_issue_number" "${target_issues[@]:-}")
  for number in "${numbers[@]}"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel-poll-vote slash e2e cleanup" >/dev/null 2>&1 || true
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
  --body "@gitclaw /channels poll ${route_a},${route_b} --poll-id ${poll_id} --message-id ${invite_message_id}

Question: ${poll_question}
Options:
- ${option_one}
- ${option_two}")"
source_issue_number="${source_issue_url##*/}"

wait_for_main_run "issues" "$opened_started_at" "$source_title" >/dev/null || die "timed out waiting for source issue channel-poll action"
wait_for_assistant_count_for_issue "$source_issue_number" 1 || die "expected one channel-poll action receipt"
receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"

for expected in \
  "GitClaw Channel Poll Action" \
  "channel_poll_status: \`queued\`" \
  "poll_issue_created: \`true\`" \
  "poll_invites_queued: \`2\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "source poll receipt missing ${expected}"
done
for leaked in "$question_token" "$route_a" "$route_b" "$invite_message_id" "$poll_id" "$poll_question" "$option_one" "$option_two"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "channel-poll source receipt leaked ${leaked}"
  fi
done

poll_issue_number="$(sed -n 's/.*poll_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -1)"
[[ -n "$poll_issue_number" ]] || die "could not parse poll issue number from receipt"
mapfile -t target_issues < <(sed -n 's/.*target_issue=`#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | awk '!seen[$0]++')
[[ "${#target_issues[@]}" == "2" ]] || die "expected two poll target issues, got ${#target_issues[@]} from receipt"

poll_json="$(gh issue view "$poll_issue_number" --repo "$repo" --json title,body,labels)"
poll_title="$(jq -r '.title' <<<"$poll_json")"
poll_body="$(jq -r '.body' <<<"$poll_json")"
grep -Fq "gitclaw:channel-poll" <<<"$poll_body" || die "poll issue missing poll marker"
grep -Fq "$poll_question" <<<"$poll_body" || die "poll issue missing visible question"
grep -Fq "$option_one" <<<"$poll_body" || die "poll issue missing first visible option"
grep -Fq "$option_two" <<<"$poll_body" || die "poll issue missing second visible option"

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
[[ "$(outbound_comment_count "$target_issue_number")" == "1" ]] || die "target issue should start with one poll invite outbound comment"

vote_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$target_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels poll-vote --poll-id ${poll_id} --message-id ${vote_message_id} --notify-message-id ${ack_message_id} --choice 1
Voter: ${voter_name}
Note:
${vote_note}" >/dev/null

wait_for_main_run "issue_comment" "$vote_started_at" "$target_issue_title" >/dev/null || die "timed out waiting for channel-poll-vote issue_comment action"
wait_for_assistant_count_for_issue "$target_issue_number" 1 || die "expected one channel-poll-vote action receipt"
vote_receipt="$(latest_assistant_comment_for_issue "$target_issue_number")"

for expected in \
  "GitClaw Channel Poll Vote Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels poll-vote\`" \
  "channel_poll_vote_status: \`recorded\`" \
  "poll_issue: \`#${poll_issue_number}\`" \
  "vote_recorded: \`true\`" \
  "vote_duplicate_suppressed: \`false\`" \
  "notification_target_issue: \`#${target_issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "channel: \`${target_channel}\`" \
  "vote_id_auto: \`false\`" \
  "notify_message_id_auto: \`false\`" \
  "choice_resolved_from_poll_options: \`true\`" \
  "choice_index: \`1\`" \
  "target_from_current_channel_issue: \`true\`" \
  "raw_poll_id_included: \`false\`" \
  "raw_vote_id_included: \`false\`" \
  "raw_choice_included: \`false\`" \
  "raw_voter_included: \`false\`" \
  "raw_note_included: \`false\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox + channel-delivery\`" \
  "llm_e2e_required_after_channel_poll_vote_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$vote_receipt" || die "channel-poll-vote receipt missing ${expected}"
done
for leaked in "$note_token" "$voter_name" "$vote_message_id" "$ack_message_id" "$poll_id" "$vote_note" "$option_one"; do
  if grep -Fq "$leaked" <<<"$vote_receipt"; then
    die "channel-poll-vote receipt leaked ${leaked}"
  fi
done

[[ "$(poll_vote_record_count)" == "1" ]] || die "expected exactly one poll vote record"
vote_record="$(latest_poll_vote_record)"
for expected in \
  "gitclaw:channel-poll-vote" \
  "poll_id=\"${poll_id_marker}\"" \
  "vote_id=\"${vote_id_marker}\"" \
  "- choice: ${option_one}" \
  "- choice_index: 1" \
  "- source_channel: ${target_channel}" \
  "- source_issue: #${target_issue_number}" \
  "${voter_name}" \
  "${vote_note}" \
  "raw_thread_id_included: false" \
  "raw_source_message_id_included: false"; do
  grep -Fq -- "$expected" <<<"$vote_record" || die "poll vote record missing ${expected}"
done

[[ "$(outbound_comment_count "$target_issue_number")" == "2" ]] || die "poll vote did not queue acknowledgement outbound comment"
ack_marker="message_id=\"${ack_message_id}\""
ack_body="$(gh issue view "$target_issue_number" --repo "$repo" --json comments | jq -r --arg marker "$ack_marker" '[.comments[] | select(.body | contains($marker)) | .body] | .[-1] // ""')"
[[ -n "$ack_body" ]] || die "could not find poll vote acknowledgement body"
for expected in \
  "message_id=\"${ack_message_id}\"" \
  "GitClaw poll vote recorded" \
  "Poll: #${poll_issue_number}" \
  "Choice: ${option_one}" \
  "Choice index: 1" \
  "Participant: ${voter_name}"; do
  grep -Fq "$expected" <<<"$ack_body" || die "poll vote acknowledgement missing ${expected}"
done
if grep -Fq "$note_token" <<<"$ack_body"; then
  die "poll vote acknowledgement leaked note token"
fi

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$target_channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$target_account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$target_issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "pending=3" <<<"$outbox_output" || die "channel outbox did not report three pending messages after poll vote: ${outbox_output}"
grep -Fq '"pending_messages": 3' "$outbox_file" || die "channel outbox file missing three pending messages after poll vote"
for leaked in "$question_token" "$note_token" "$voter_name" "$option_one"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "channel outbox leaked ${leaked} without --include-body"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$target_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels poll-vote --poll-id ${poll_id} --message-id ${vote_message_id} --notify-message-id ${ack_message_id} --choice 1
Note: duplicate should not leak ${duplicate_hidden_token}" >/dev/null

wait_for_main_run "issue_comment" "$duplicate_started_at" "$target_issue_title" >/dev/null || die "timed out waiting for duplicate channel-poll-vote action"
wait_for_assistant_count_for_issue "$target_issue_number" 2 || die "expected duplicate channel-poll-vote receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$target_issue_number")"
for expected in \
  "GitClaw Channel Poll Vote Action" \
  "channel_poll_vote_status: \`duplicate\`" \
  "vote_recorded: \`false\`" \
  "vote_duplicate_suppressed: \`true\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "raw_note_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel-poll-vote receipt missing ${expected}"
done
[[ "$(poll_vote_record_count)" == "1" ]] || die "duplicate poll vote posted another vote record"
[[ "$(outbound_comment_count "$target_issue_number")" == "2" ]] || die "duplicate poll vote queued another acknowledgement"
for leaked in "$duplicate_hidden_token" "$vote_message_id" "$ack_message_id" "$poll_id" "$option_one"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel-poll-vote receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$poll_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel poll vote thread and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line ends with an all-caps token after the arrow.
That token starts with \`GITCLAW_CHANNEL_POLL_VOTE_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include route names, target issue numbers, message ids, account hashes, source issue title, poll id, poll question, option text, voter name, or vote note.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_main_run "issue_comment" "$comment_started_at" "$poll_title")" || die "timed out waiting for channel-poll-vote model follow-up"
wait_for_assistant_count_for_issue "$poll_issue_number" 1 || die "expected model-backed poll vote follow-up"
model_comment="$(latest_assistant_comment_for_issue "$poll_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include poll vote search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant poll vote follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant poll vote follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant poll vote follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant poll vote follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant poll vote follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant poll vote follow-up marker missing usage token telemetry"

for leaked in "$question_token" "$note_token" "$duplicate_hidden_token" "$followup_hidden_token" "$slack_account_id" "$telegram_account_id" "$poll_id" "$voter_name"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model poll vote follow-up leaked ${leaked}"
  fi
done

url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}, poll issue #${poll_issue_number}, target issue #${target_issue_number}: ${url}"
