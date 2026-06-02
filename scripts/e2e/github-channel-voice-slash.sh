#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-voice slash action records a mirrored channel voice as a GitHub issue.
set -euo pipefail

log() {
  echo "channel-voice-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-voice-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-voice slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-voice-e2e-${timestamp}"
ingest_message_id="voice-ingest-${timestamp}"
voice_id="channel-voice-${timestamp}"
notify_message_id="voice-notify-${timestamp}"
account_id="telegram-voice-account-NOECHO_CHANNEL_VOICE_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_VOICE_INGEST_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_VOICE_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_VOICE_FOLLOWUP_${timestamp}"
voice_transcript_token="VISIBLE_CHANNEL_VOICE_TRANSCRIPT_${timestamp}"
voice_title="Shape GitHub-native channel voice ${timestamp}"
voice_duration="47"
voice_media_type="audio/ogg"
voice_audio_url="https://media.example.invalid/${voice_id}.ogg"
expected_token="GITCLAW_CHANNEL_VOICE_CONTEXT_V1"
search_phrase="channel voice unique search fixture phrase"
notify_message_hash="$(sha256_12 "$notify_message_id")"
issue_number=""
voice_issue_number=""
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

voice_notification_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$notify_message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

cleanup() {
  for number in "$issue_number" "$voice_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel-voice slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel-voice slash E2E.

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
  if grep -Fq "$ingest_hidden_token" <<<"$candidate_report"; then
    die "initial channel report leaked ingest hidden token"
  fi
  if grep -Fq "GitClaw Channel Report" <<<"$candidate_report" && grep -Fq 'channel_thread_issue: `true`' <<<"$candidate_report"; then
    initial_report="$candidate_report"
    break
  fi
  sleep 5
done
[[ -n "$initial_report" ]] || die "expected initial channel report"

voice_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels voice --voice-id ${voice_id} --duration ${voice_duration} --media-type ${voice_media_type} --url ${voice_audio_url} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id}
Voice: ${voice_title}
Transcript:
Visible voice transcript token: ${voice_transcript_token}" >/dev/null

wait_for_issue_comment_run_for_title "$voice_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel voice action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel voice action receipt"
voice_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Voice Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels voice\`" \
  "channel_voice_status: \`captured\`" \
  "voice_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "raw_voice_id_included: \`false\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_voice_title_included: \`false\`" \
  "raw_transcript_included: \`false\`" \
  "raw_media_type_included: \`false\`" \
  "raw_audio_url_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "llm_e2e_required_after_channel_voice_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$voice_receipt" || die "channel voice receipt missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$voice_transcript_token" "$voice_title" "$voice_id" "$voice_media_type" "$voice_audio_url" "$thread_id" "$ingest_message_id" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$voice_receipt"; then
    die "channel voice receipt leaked ${leaked}"
  fi
done

voice_issue_number="$(sed -n 's/.*voice_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$voice_receipt" | head -n 1)"
[[ -n "$voice_issue_number" && "$voice_issue_number" != "null" ]] || die "could not resolve voice issue number"
log "channel voice created voice issue #${voice_issue_number}"

[[ "$(voice_notification_count)" == "1" ]] || die "channel voice did not queue exactly one voice-link outbound comment"
voice_json="$(gh issue view "$voice_issue_number" --repo "$repo" --json title,body,labels)"
voice_issue_title="$(jq -r '.title' <<<"$voice_json")"
voice_body="$(jq -r '.body' <<<"$voice_json")"
voice_labels="$(jq -r '.labels[].name' <<<"$voice_json")"
grep -Fxq "gitclaw" <<<"$voice_labels" || die "voice issue missing gitclaw label"
for expected in \
  "gitclaw:channel-voice" \
  "voice_id: ${voice_id}" \
  "source_channel: ${channel}" \
  "source_issue: #${issue_number}" \
  "source_message_id_sha256_12:" \
  "duration_seconds: ${voice_duration}" \
  "media_type_sha256_12:" \
  "audio_url_sha256_12:" \
  "voice_mode: github-issue-transcript" \
  "raw_thread_id_included: false" \
  "raw_source_message_id_included: false" \
  "raw_audio_url_included: false" \
  "${voice_title}" \
  "${voice_transcript_token}"; do
  grep -Fq "$expected" <<<"$voice_body" || die "voice issue body missing ${expected}"
done
for leaked in "$thread_id" "$ingest_message_id" "$ingest_hidden_token" "$voice_media_type" "$voice_audio_url"; do
  if grep -Fq "$leaked" <<<"$voice_body"; then
    die "voice issue body leaked ${leaked}"
  fi
done

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
outbound_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound"))] | join("\n")' <<<"$issue_json")"
grep -Fq "gitclaw:channel-outbound" <<<"$outbound_bodies" || die "channel issue missing voice-link outbound marker"
grep -Fq "Duration: ${voice_duration}s" <<<"$outbound_bodies" || die "channel issue missing visible voice duration notification"
grep -Fq "Title: ${voice_title}" <<<"$outbound_bodies" || die "channel issue missing visible voice title notification"
for leaked in "$voice_transcript_token" "$voice_media_type" "$voice_audio_url"; do
  if grep -Fq "$leaked" <<<"$outbound_bodies"; then
    die "channel voice notification leaked ${leaked}"
  fi
done

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing voice-link outbound hash ${notify_message_hash}"
for leaked in "$voice_title" "$voice_transcript_token" "$voice_media_type" "$voice_audio_url" "$account_id" "$ingest_hidden_token"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels voice --voice-id ${voice_id} --duration ${voice_duration} --media-type ${voice_media_type} --url ${voice_audio_url} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id}
Voice: ${voice_title}
Transcript:
Do not leak duplicate hidden token: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel voice action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel voice receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Voice Action" \
  "requested_channel_command: \`/channels voice\`" \
  "channel_voice_status: \`duplicate\`" \
  "voice_issue: \`#${voice_issue_number}\`" \
  "voice_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "raw_transcript_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel voice receipt missing ${expected}"
done
[[ "$(voice_notification_count)" == "1" ]] || die "duplicate channel voice queued another voice-link notification"
for leaked in "$duplicate_hidden_token" "$voice_id" "$voice_media_type" "$voice_audio_url" "$thread_id" "$ingest_message_id" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel voice receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$voice_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel-created voice and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_VOICE_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include voice ids, durations, media types, audio URLs, message ids, thread ids, account hashes, issue numbers, voice titles, or transcript.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$voice_issue_title")" || die "timed out waiting for channel voice model follow-up"
wait_for_assistant_count_for_issue "$voice_issue_number" 1 || die "expected model-backed channel voice follow-up"
model_comment="$(latest_assistant_comment_for_issue "$voice_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel voice search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel voice follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel voice follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel voice follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel voice follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel voice follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel voice follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$account_id" "$voice_id" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel voice follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number}, voice issue #${voice_issue_number} (model follow-up: ${model_url})"
