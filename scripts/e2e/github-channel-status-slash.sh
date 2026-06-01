#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-status slash action queues provider-visible progress.
set -euo pipefail

log() {
  echo "channel-status-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
delivery_workflow="${GITCLAW_E2E_CHANNEL_DELIVERY_WORKFLOW:-.github/workflows/gitclaw-channel-delivery.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-status-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-status slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-status-e2e-${timestamp}"
target_message_id="status-ingest-${timestamp}"
status_id="status-update-${timestamp}"
status_state="working"
account_id="telegram-status-account-NOECHO_CHANNEL_STATUS_ACCOUNT_${timestamp}"
external_message_id="telegram-status-delivered-NOECHO_CHANNEL_STATUS_EXTERNAL_${timestamp}"
gateway_run_id="channel-status-slash-e2e-${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_STATUS_INGEST_${timestamp}"
source_hidden_token="NOECHO_CHANNEL_STATUS_SOURCE_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_STATUS_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_STATUS_FOLLOWUP_${timestamp}"
status_body_token="VISIBLE_CHANNEL_STATUS_BODY_${timestamp}"
expected_token="GITCLAW_CHANNEL_STATUS_CONTEXT_V1"
search_phrase="channel status unique search fixture phrase"
account_hash="$(sha256_12 "$account_id")"
external_hash="$(sha256_12 "$external_message_id")"
status_message_hash="$(sha256_12 "${status_id}|${status_state}")"
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

status_comment_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg id "$status_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-status") and contains("status_id=\"" + $id + "\""))] | length'
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
        gh issue close "$number" --repo "$repo" --comment "channel-status slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$issue_title" \
  --label gitclaw:channel \
  --body "<!-- gitclaw:channel-thread channel=\"${channel}\" thread_id=\"${thread_id}\" -->
GitClaw channel bridge thread for channel-status E2E.")"
issue_number="${issue_url##*/}"
log "created channel issue #${issue_number}: ${issue_url}"

wait_for_workflow_run "$main_workflow" "issues" "$issue_started_at" >/dev/null || die "timed out waiting for initial channel issue preflight"

ingest_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "<!-- gitclaw:channel-message channel=\"${channel}\" thread_id=\"${thread_id}\" message_id=\"${target_message_id}\" author=\"telegram\" -->
Mirrored channel message for channel-status E2E.
Do not leak this ingest token: ${ingest_hidden_token}" >/dev/null

wait_for_issue_comment_run "$ingest_started_at" >/dev/null || die "timed out waiting for inert channel ingest comment preflight"
gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw >/dev/null

status_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels status --message-id ${target_message_id} --status-id ${status_id} --state ${status_state}
Working through the channel request.
Visible status update token for provider queue inspection: ${status_body_token}
Do not include this source hidden token in the receipt: ${source_hidden_token}" >/dev/null

wait_for_issue_comment_run "$status_started_at" >/dev/null || die "timed out waiting for channel status action"
wait_for_assistant_count 1 || die "expected channel status action receipt"
status_receipt="$(latest_assistant_comment)"
for expected in \
  "GitClaw Channel Status Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels status\`" \
  "channel_status_status: \`queued\`" \
  "target_issue: \`#${issue_number}\`" \
  "target_issue_created: \`false\`" \
  "duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "target_issue_is_source: \`true\`" \
  "raw_target_message_id_included: \`false\`" \
  "raw_status_id_included: \`false\`" \
  "raw_status_state_included: \`false\`" \
  "raw_status_body_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "llm_e2e_required_after_channel_status_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$status_receipt" || die "channel status receipt missing ${expected}"
done
for leaked in "$source_hidden_token" "$ingest_hidden_token" "$status_body_token" "$target_message_id" "$thread_id" "$status_id" "$status_state"; do
  if grep -Fq "$leaked" <<<"$status_receipt"; then
    die "channel status receipt leaked ${leaked}"
  fi
done

[[ "$(status_comment_count)" == "1" ]] || die "channel status did not queue exactly one status comment"
status_comment_id="$(sed -n 's/.*status_comment_id: `\([0-9][0-9]*\)`.*/\1/p' <<<"$status_receipt" | head -n 1)"
[[ -n "$status_comment_id" && "$status_comment_id" != "null" ]] || die "could not resolve status source comment id"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
status_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-status"))] | join("\n")' <<<"$issue_json")"
grep -Fq "gitclaw:channel-status" <<<"$status_bodies" || die "channel issue missing status marker"
grep -Fq "$status_body_token" <<<"$status_bodies" || die "channel status body missing visible token"

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq "status_comments=1" <<<"$outbox_output" || die "channel outbox output missing status count: ${outbox_output}"
grep -Fq "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$status_message_hash" '.messages[] | select(.kind == "channel-status" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing status hash ${status_message_hash}"
for leaked in "$account_id" "$external_message_id" "$ingest_hidden_token" "$source_hidden_token" "$status_body_token"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels status --message-id ${target_message_id} --status-id ${status_id} --state ${status_state}
Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run "$duplicate_started_at" >/dev/null || die "timed out waiting for duplicate channel status action"
wait_for_assistant_count 2 || die "expected duplicate channel status receipt"
duplicate_receipt="$(latest_assistant_comment)"
for expected in \
  "GitClaw Channel Status Action" \
  "requested_channel_command: \`/channels status\`" \
  "channel_status_status: \`duplicate\`" \
  "target_issue: \`#${issue_number}\`" \
  "duplicate_suppressed: \`true\`" \
  "target_from_current_channel_issue: \`true\`" \
  "target_issue_is_source: \`true\`" \
  "raw_status_body_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel status receipt missing ${expected}"
done
[[ "$(status_comment_count)" == "1" ]] || die "duplicate channel status queued another status comment"
for leaked in "$duplicate_hidden_token" "$target_message_id" "$thread_id" "$status_id" "$status_state"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel status receipt leaked ${leaked}"
  fi
done

delivery_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$delivery_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f account_id="$account_id" \
  -f issue_number="$issue_number" \
  -f comment_id="$status_comment_id" \
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
grep -Fq "source_comment_id=\"${status_comment_id}\"" <<<"$(jq -r '[.comments[].body] | join("\n")' <<<"$state_json")" || die "state issue missing delivery receipt"
grep -Fq "external_message_sha256_12=\"${external_hash}\"" <<<"$(jq -r '[.comments[].body] | join("\n")' <<<"$state_json")" || die "state issue missing external message hash"

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel status thread and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_STATUS_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include target message ids, thread ids, account hashes, delivery hashes, status ids, status states, status body text, or issue numbers.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run "$model_started_at")" || die "timed out waiting for channel status model follow-up"
wait_for_assistant_count 3 || die "expected model-backed channel status follow-up"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel status search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel status follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel status follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel status follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel status follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel status follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel status follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$source_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$status_body_token" "$account_id" "$external_message_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel status follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number}, state issue #${state_issue} (model follow-up: ${model_url})"
