#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-deliverable slash action uses a channel-ingested live issue.
set -euo pipefail

log() {
  echo "channel-deliverable-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
delivery_workflow="${GITCLAW_E2E_CHANNEL_DELIVERY_WORKFLOW:-.github/workflows/gitclaw-channel-delivery.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-deliverable-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-deliverable slash E2E appears to be running: ${lock_dir}"
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

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
channel="telegram"
thread_id="channel-deliverable-e2e-${timestamp}"
ingest_message_id="deliverable-ingest-${timestamp}"
deliverable_id="channel-deliverable-${timestamp}"
deliverable_message_id="deliverable-slash-${timestamp}"
account_id="telegram-deliverable-account-NOECHO_CHANNEL_DELIVERABLE_ACCOUNT_${timestamp}"
external_message_id="telegram-deliverable-delivered-NOECHO_CHANNEL_DELIVERABLE_EXTERNAL_${timestamp}"
gateway_run_id="channel-deliverable-slash-e2e-${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_DELIVERABLE_INGEST_${timestamp}"
url_hidden_token="NOECHO_CHANNEL_DELIVERABLE_URL_${timestamp}"
caption_token="VISIBLE_CHANNEL_DELIVERABLE_CAPTION_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_DELIVERABLE_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_DELIVERABLE_FOLLOWUP_${timestamp}"
filename="launch-deliverable-${timestamp}.pdf"
media_type="application/pdf"
deliverable_bytes="4242"
file_checksum="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
deliverable_url="https://example.invalid/${url_hidden_token}.pdf"
expected_token="GITCLAW_CHANNEL_DELIVERABLE_CONTEXT_V1"
search_phrase="channel deliverable unique search fixture phrase"
account_hash="$(sha256_12 "$account_id")"
external_hash="$(sha256_12 "$external_message_id")"
deliverable_message_hash="$(sha256_12 "${deliverable_id}|${deliverable_message_id}")"
issue_number=""
state_issue=""
issue_title="GitClaw ${channel} thread ${thread_id}"

run_list_json() {
  local workflow="$1"
  local event="$2"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event "$event" \
    --limit 20 \
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

wait_for_issue_comment_run() {
  local started_at="$1"
  local run_json
  for _ in {1..90}; do
    run_json="$(run_list_json "$main_workflow" "issue_comment" | jq -c --arg started "$started_at" --arg title "$issue_title" '[.[] | select(.createdAt >= $started and .displayTitle == $title)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "issue_comment run failed with conclusion ${conclusion}: ${url}"
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

wait_for_assistant_count() {
  local want="$1"
  for _ in {1..90}; do
    local errors
    errors="$(error_count)"
    if [[ "$errors" != "0" ]]; then
      die "issue posted ${errors} error marker comment(s)"
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
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

deliverable_comment_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg id "$deliverable_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-deliverable") and contains($id))] | length'
}

find_state_issue_number() {
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:channel \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg account "$account_hash" '.[] | select((.title | contains($account)) or (.body | contains($account))) | .number' \
    | head -n 1
}

cleanup() {
  for number in "$issue_number" "$state_issue"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel-deliverable slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel-deliverable slash E2E.

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
wait_for_assistant_count 1 || die "expected initial channel report"
initial_report="$(latest_assistant_comment)"
grep -Fq "GitClaw Channel Report" <<<"$initial_report" || die "initial assistant comment missing channel report"
grep -Fq 'channel_thread_issue: `true`' <<<"$initial_report" || die "initial channel report missing channel thread status"
if grep -Fq "$ingest_hidden_token" <<<"$initial_report"; then
  die "initial channel report leaked ingest hidden token"
fi

deliverable_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels deliverable --deliverable-id ${deliverable_id} --message-id ${deliverable_message_id} --filename ${filename} --media-type ${media_type} --bytes ${deliverable_bytes} --sha256 ${file_checksum} --url ${deliverable_url}
Caption:
Visible provider caption token: ${caption_token}" >/dev/null

wait_for_issue_comment_run "$deliverable_started_at" >/dev/null || die "timed out waiting for channel deliverable action"
wait_for_assistant_count 2 || die "expected channel deliverable action receipt"
deliverable_receipt="$(latest_assistant_comment)"
for expected in \
  "GitClaw Channel Deliverable Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels deliverable\`" \
  "channel_deliverable_status: \`queued\`" \
  "deliverable_target_issue: \`#${issue_number}\`" \
  "deliverable_queued: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "message_id_auto: \`false\`" \
  "deliverable_mode: \`channel-outbox-native-deliverable\`" \
  "provider_upload_performed: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "raw_deliverable_id_included: \`false\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_message_id_included: \`false\`" \
  "raw_deliverable_filename_included: \`false\`" \
  "raw_deliverable_caption_included: \`false\`" \
  "raw_deliverable_url_included: \`false\`" \
  "raw_file_checksum_included: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox --include-body + channel-delivery\`" \
  "llm_e2e_required_after_channel_deliverable_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$deliverable_receipt" || die "channel deliverable receipt missing ${expected}"
done
for leaked in "$caption_token" "$deliverable_id" "$deliverable_message_id" "$thread_id" "$ingest_hidden_token" "$filename" "$deliverable_url" "$url_hidden_token" "$file_checksum"; do
  if grep -Fq "$leaked" <<<"$deliverable_receipt"; then
    die "channel deliverable receipt leaked ${leaked}"
  fi
done

[[ "$(deliverable_comment_count)" == "1" ]] || die "channel deliverable did not queue exactly one deliverable comment"
source_comment_id="$(sed -n 's/.*deliverable_comment_id: `\([0-9][0-9]*\)`.*/\1/p' <<<"$deliverable_receipt" | head -n 1)"
[[ -n "$source_comment_id" && "$source_comment_id" != "null" ]] || die "could not resolve deliverable source comment id"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
deliverable_comments="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-deliverable"))] | join("\n")' <<<"$issue_json")"
for expected in \
  "gitclaw:channel-deliverable" \
  "$filename" \
  "$media_type" \
  "$deliverable_url" \
  "$file_checksum" \
  "$caption_token" \
  "Provider upload performed: false" \
  "Provider delivery performed: false"; do
  grep -Fq "$expected" <<<"$deliverable_comments" || die "channel issue missing visible deliverable field ${expected}"
done
if grep -Fq "$ingest_hidden_token" <<<"$deliverable_comments"; then
  die "channel deliverable comment leaked ingest hidden token"
fi

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$deliverable_message_hash" '.messages[] | select(.kind == "channel-deliverable" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing deliverable hash ${deliverable_message_hash}"
for leaked in "$caption_token" "$deliverable_url" "$url_hidden_token" "$filename" "$account_id" "$external_message_id" "$ingest_hidden_token"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

body_outbox_file="$(mktemp)"
body_outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --include-body --out "$body_outbox_file")"
grep -Fq "body_included=true" <<<"$body_outbox_output" || die "include-body outbox output missing body flag: ${body_outbox_output}"
jq -e --arg hash "$deliverable_message_hash" --arg url "$deliverable_url" --arg filename "$filename" --arg caption "$caption_token" '.messages[] | select(.kind == "channel-deliverable" and .outbound_message_sha256_12 == $hash and (.body | contains($url)) and (.body | contains($filename)) and (.body | contains($caption)))' "$body_outbox_file" >/dev/null || die "include-body outbox missing visible deliverable payload"
if grep -Fq "gitclaw:channel-deliverable" "$body_outbox_file" || grep -Fq "$account_id" "$body_outbox_file"; then
  die "include-body outbox leaked marker or account id"
fi

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels deliverable --deliverable-id ${deliverable_id} --message-id ${deliverable_message_id} --filename ${filename} --media-type ${media_type} --bytes ${deliverable_bytes} --url https://example.invalid/${duplicate_hidden_token}.pdf
Caption:
Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run "$duplicate_started_at" >/dev/null || die "timed out waiting for duplicate channel deliverable action"
wait_for_assistant_count 3 || die "expected duplicate channel deliverable receipt"
duplicate_receipt="$(latest_assistant_comment)"
for expected in \
  "GitClaw Channel Deliverable Action" \
  "requested_channel_command: \`/channels deliverable\`" \
  "channel_deliverable_status: \`duplicate\`" \
  "deliverable_target_issue: \`#${issue_number}\`" \
  "deliverable_queued: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "target_from_current_channel_issue: \`true\`" \
  "raw_deliverable_caption_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel deliverable receipt missing ${expected}"
done
[[ "$(deliverable_comment_count)" == "1" ]] || die "duplicate channel deliverable queued another deliverable comment"
for leaked in "$caption_token" "$duplicate_hidden_token" "$deliverable_id" "$deliverable_message_id" "$thread_id"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel deliverable receipt leaked ${leaked}"
  fi
done

delivery_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$delivery_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f account_id="$account_id" \
  -f issue_number="$issue_number" \
  -f comment_id="$source_comment_id" \
  -f external_message_id="$external_message_id" \
  -f gateway_run_id="$gateway_run_id"

wait_for_workflow_run "$delivery_workflow" "workflow_dispatch" "$delivery_started_at" >/dev/null || die "timed out waiting for channel-delivery workflow"
for _ in {1..30}; do
  state_issue="$(find_state_issue_number)"
  if [[ -n "$state_issue" && "$state_issue" != "null" ]]; then
    break
  fi
  sleep 2
done
[[ -n "$state_issue" && "$state_issue" != "null" ]] || die "could not find delivery state issue"
state_json="$(gh issue view "$state_issue" --repo "$repo" --json body,comments)"
grep -Fq "account_sha256_12=\"${account_hash}\"" <<<"$(jq -r '.body' <<<"$state_json")" || die "state issue missing account hash"
grep -Fq "source_comment_id=\"${source_comment_id}\"" <<<"$(jq -r '[.comments[].body] | join("\n")' <<<"$state_json")" || die "state issue missing delivery receipt"
grep -Fq "external_message_sha256_12=\"${external_hash}\"" <<<"$(jq -r '[.comments[].body] | join("\n")' <<<"$state_json")" || die "state issue missing external message hash"
if grep -Fq "$caption_token" <<<"$(jq -r '[.body, (.comments[].body)] | join("\n")' <<<"$state_json")" || grep -Fq "$url_hidden_token" <<<"$(jq -r '[.body, (.comments[].body)] | join("\n")' <<<"$state_json")"; then
  die "delivery state leaked deliverable payload"
fi

second_outbox_file="$(mktemp)"
GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$second_outbox_file" >/dev/null
if jq -e --arg id "$source_comment_id" '.messages[] | select((.source_comment_id | tostring) == $id and .kind == "channel-deliverable")' "$second_outbox_file" >/dev/null; then
  die "delivered channel deliverable still appeared as pending deliverable work"
fi

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the channel deliverable action and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line ends with an all-caps token after the arrow.
That token starts with \`GITCLAW_CHANNEL_DELIVERABLE_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include the channel thread id, deliverable id, deliverable message id, filename, URL, caption token, account hash, external message hash, or delivery state issue.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run "$comment_started_at")" || die "timed out waiting for channel deliverable model follow-up"
wait_for_assistant_count 4 || die "expected model-backed channel deliverable follow-up"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel deliverable search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel deliverable follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel deliverable follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel deliverable follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel deliverable follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel deliverable follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel deliverable follow-up marker missing usage token telemetry"

for leaked in "$ingest_hidden_token" "$caption_token" "$url_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$account_id" "$external_message_id" "$deliverable_id" "$filename"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel deliverable follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number}, state issue #${state_issue} (model follow-up: ${model_url})"
