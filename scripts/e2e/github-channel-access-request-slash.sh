#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-access-request slash action records a mirrored channel access request as a GitHub issue.
set -euo pipefail

log() {
  echo "channel-access-request-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-access-request-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-access-request slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-access-request-e2e-${timestamp}"
ingest_message_id="access-ingest-${timestamp}"
access_id="channel-access-request-${timestamp}"
notify_message_id="access-notify-${timestamp}"
account_id="telegram-access-account-NOECHO_CHANNEL_ACCESS_REQUEST_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_ACCESS_REQUEST_INGEST_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_ACCESS_REQUEST_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_ACCESS_REQUEST_FOLLOWUP_${timestamp}"
reason_token="VISIBLE_CHANNEL_ACCESS_REQUEST_REASON_${timestamp}"
requester_name="Channel Access Requester ${timestamp}"
provider_user_id="provider-user-NOECHO_CHANNEL_ACCESS_REQUEST_USER_${timestamp}"
provider_handle="@provider-handle-NOECHO_CHANNEL_ACCESS_REQUEST_HANDLE_${timestamp}"
scope_name="channel-scope-${timestamp}"
requested_role="reviewer-${timestamp}"
expected_token="GITCLAW_CHANNEL_ACCESS_REQUEST_CONTEXT_V1"
search_phrase="channel access request unique search fixture phrase"
notify_message_hash="$(sha256_12 "$notify_message_id")"
issue_number=""
access_issue_number=""
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

access_notification_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$notify_message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

cleanup() {
  for number in "$issue_number" "$access_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel-access-request slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel-access-request slash E2E.

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

access_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels access-request --access-id ${access_id} --provider-user-id ${provider_user_id} --handle ${provider_handle} --scope ${scope_name} --role ${requested_role} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id}
Requester: ${requester_name}
Reason:
Visible access reason token: ${reason_token}" >/dev/null

wait_for_issue_comment_run_for_title "$access_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel access request action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel access request action receipt"
access_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Access Request Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels access-request\`" \
  "channel_access_request_status: \`opened\`" \
  "access_request_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "provider_user_id_sha256_12:" \
  "provider_handle_sha256_12:" \
  "scope_sha256_12:" \
  "requested_role_sha256_12:" \
  "reason_sha256_12:" \
  "target_from_current_channel_issue: \`true\`" \
  "permission_grant_performed: \`false\`" \
  "allowlist_mutation_performed: \`false\`" \
  "pairing_code_issued: \`false\`" \
  "raw_access_id_included: \`false\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_requester_included: \`false\`" \
  "raw_provider_user_id_included: \`false\`" \
  "raw_provider_handle_included: \`false\`" \
  "raw_scope_included: \`false\`" \
  "raw_requested_role_included: \`false\`" \
  "raw_reason_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "llm_e2e_required_after_channel_access_request_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$access_receipt" || die "channel access request receipt missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$reason_token" "$requester_name" "$access_id" "$provider_user_id" "$provider_handle" "$scope_name" "$requested_role" "$thread_id" "$ingest_message_id" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$access_receipt"; then
    die "channel access request receipt leaked ${leaked}"
  fi
done

access_issue_number="$(sed -n 's/.*access_request_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$access_receipt" | head -n 1)"
[[ -n "$access_issue_number" && "$access_issue_number" != "null" ]] || die "could not resolve access issue number"
log "channel access request created access issue #${access_issue_number}"

[[ "$(access_notification_count)" == "1" ]] || die "channel access request did not queue exactly one access-review outbound comment"
access_json="$(gh issue view "$access_issue_number" --repo "$repo" --json title,body,labels)"
access_issue_title="$(jq -r '.title' <<<"$access_json")"
access_body="$(jq -r '.body' <<<"$access_json")"
access_labels="$(jq -r '.labels[].name' <<<"$access_json")"
grep -Fxq "gitclaw" <<<"$access_labels" || die "access issue missing gitclaw label"
for expected in \
  "gitclaw:channel-access-request" \
  "access_id: ${access_id}" \
  "source_channel: ${channel}" \
  "source_issue: #${issue_number}" \
  "source_message_id_sha256_12:" \
  "requester: ${requester_name}" \
  "provider_user_id_sha256_12:" \
  "provider_handle_sha256_12:" \
  "scope: ${scope_name}" \
  "requested_role: ${requested_role}" \
  "access_mode: github-issue-access-review" \
  "permission_grant_performed: false" \
  "allowlist_mutation_performed: false" \
  "pairing_code_issued: false" \
  "raw_thread_id_included: false" \
  "raw_source_message_id_included: false" \
  "raw_provider_user_id_included: false" \
  "raw_provider_handle_included: false" \
  "${reason_token}"; do
  grep -Fq "$expected" <<<"$access_body" || die "access issue body missing ${expected}"
done
for leaked in "$thread_id" "$ingest_message_id" "$ingest_hidden_token" "$provider_user_id" "$provider_handle"; do
  if grep -Fq "$leaked" <<<"$access_body"; then
    die "access issue body leaked ${leaked}"
  fi
done

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
outbound_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound"))] | join("\n")' <<<"$issue_json")"
grep -Fq "gitclaw:channel-outbound" <<<"$outbound_bodies" || die "channel issue missing access-review outbound marker"
grep -Fq "Access review: #${access_issue_number}" <<<"$outbound_bodies" || die "channel issue missing visible access-review notification"
grep -Fq "Requester: ${requester_name}" <<<"$outbound_bodies" || die "channel issue missing visible requester notification"
grep -Fq "Scope: ${scope_name}" <<<"$outbound_bodies" || die "channel issue missing visible scope notification"
grep -Fq "Requested role: ${requested_role}" <<<"$outbound_bodies" || die "channel issue missing visible requested role notification"
grep -Fq "No access was granted by this action." <<<"$outbound_bodies" || die "channel issue missing no-grant notification"
for leaked in "$reason_token" "$provider_user_id" "$provider_handle"; do
  if grep -Fq "$leaked" <<<"$outbound_bodies"; then
    die "channel access request notification leaked ${leaked}"
  fi
done

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing access-review outbound hash ${notify_message_hash}"
for leaked in "$requester_name" "$reason_token" "$provider_user_id" "$provider_handle" "$scope_name" "$requested_role" "$account_id" "$ingest_hidden_token"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels access-request --access-id ${access_id} --provider-user-id ${provider_user_id} --handle ${provider_handle} --scope ${scope_name} --role ${requested_role} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id}
Requester: ${requester_name}
Reason:
Do not leak duplicate hidden token: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel access request action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel access request receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Access Request Action" \
  "requested_channel_command: \`/channels access-request\`" \
  "channel_access_request_status: \`duplicate\`" \
  "access_request_issue: \`#${access_issue_number}\`" \
  "access_request_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "permission_grant_performed: \`false\`" \
  "allowlist_mutation_performed: \`false\`" \
  "pairing_code_issued: \`false\`" \
  "raw_reason_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel access request receipt missing ${expected}"
done
[[ "$(access_notification_count)" == "1" ]] || die "duplicate channel access request queued another access-review notification"
for leaked in "$duplicate_hidden_token" "$reason_token" "$requester_name" "$access_id" "$provider_user_id" "$provider_handle" "$scope_name" "$requested_role" "$thread_id" "$ingest_message_id" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel access request receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$access_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel-created access-review issue and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_ACCESS_REQUEST_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include access ids, provider user ids, provider handles, requester names, scopes, roles, reasons, message ids, thread ids, account hashes, or issue numbers.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$access_issue_title")" || die "timed out waiting for channel access request model follow-up"
wait_for_assistant_count_for_issue "$access_issue_number" 1 || die "expected model-backed channel access request follow-up"
model_comment="$(latest_assistant_comment_for_issue "$access_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel access request search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel access request follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel access request follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel access request follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel access request follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel access request follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel access request follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$reason_token" "$account_id" "$access_id" "$provider_user_id" "$provider_handle" "$requester_name" "$scope_name" "$requested_role" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel access request follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number}, access issue #${access_issue_number} (model follow-up: ${model_url})"
