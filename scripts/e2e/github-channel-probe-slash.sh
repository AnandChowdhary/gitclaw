#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-probe slash action verifies a reviewed route queue and delivery path.
set -euo pipefail

log() {
  echo "channel-probe-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
delivery_workflow="${GITCLAW_E2E_CHANNEL_DELIVERY_WORKFLOW:-.github/workflows/gitclaw-channel-delivery.yml}"
lock_dir="/tmp/gitclaw-channel-probe-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-probe slash E2E appears to be running: ${lock_dir}"
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
route="e2e-slack-route"
message_id="probe-${timestamp}"
account_id="slack-probe-account-NOECHO_CHANNEL_PROBE_ACCOUNT_${timestamp}"
external_message_id="slack-probe-external-NOECHO_CHANNEL_PROBE_EXTERNAL_${timestamp}"
gateway_run_id="channel-probe-slash-e2e-${timestamp}"
source_hidden_token="NOECHO_CHANNEL_PROBE_SOURCE_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_PROBE_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_PROBE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_PROBE_CONTEXT_V1"
search_phrase="channel probe unique search fixture phrase"
account_hash="$(sha256_12 "$account_id")"
external_hash="$(sha256_12 "$external_message_id")"
source_title="GitClaw channel probe E2E ${timestamp}"
source_issue_number=""
target_issue_number=""
state_issue_number=""
outbound_comment_id=""

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

delivery_run_list_json() {
  local started_at="$1"
  gh run list \
    --repo "$repo" \
    --workflow "$delivery_workflow" \
    --event workflow_dispatch \
    --created ">=$started_at" \
    --limit 20 \
    --json databaseId,status,conclusion,createdAt,url \
    --jq '.'
}

wait_for_delivery_run() {
  local started_at="$1"
  local run_json
  for _ in {1..120}; do
    run_json="$(delivery_run_list_json "$started_at" | jq -c '[.[]] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${delivery_workflow} run failed with conclusion ${conclusion}: ${url}"
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

outbound_comment_count() {
  gh issue view "$target_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:channel-outbound"))] | length'
}

find_state_issue_numbers() {
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:channel \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg account_hash "$account_hash" '.[] | select((.title | contains($account_hash)) or (.body | contains($account_hash))) | .number'
}

wait_for_state_issue_number() {
  for _ in {1..30}; do
    local number
    number="$(find_state_issue_numbers | head -n 1)"
    if [[ -n "$number" && "$number" != "null" ]]; then
      echo "$number"
      return 0
    fi
    sleep 2
  done
  return 1
}

cleanup() {
  if [[ -z "${state_issue_number:-}" ]]; then
    state_issue_number="$(find_state_issue_numbers | head -n 1 || true)"
  fi
  for number in "$source_issue_number" "$target_issue_number" "$state_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel-probe slash e2e cleanup" >/dev/null 2>&1 || true
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
  --body "@gitclaw /channels probe --route ${route} --message-id ${message_id}

Probe this reviewed route without copying this raw source text.
Do not include this hidden source token: ${source_hidden_token}")"
source_issue_number="${source_issue_url##*/}"

wait_for_main_run "issues" "$opened_started_at" >/dev/null || die "timed out waiting for source issue channel-probe action"
wait_for_assistant_count 1 || die "expected one channel-probe action receipt"
receipt="$(latest_assistant_comment)"

for expected in \
  "GitClaw Channel Probe Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels probe\`" \
  "channel_probe_status: \`queued\`" \
  "target_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "route_resolved: \`true\`" \
  "channel: \`slack\`" \
  "message_id_auto: \`false\`" \
  "raw_route_names_included: \`false\`" \
  "raw_thread_ids_included: \`false\`" \
  "raw_message_ids_included: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "raw_probe_body_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox + channel-delivery\`" \
  "llm_e2e_required_after_channel_probe_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "channel-probe action receipt missing ${expected}"
done
for leaked in "$source_hidden_token" "$account_id" "$external_message_id" "$route" "$message_id" "Probe this reviewed route"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "channel-probe action receipt leaked ${leaked}"
  fi
done

target_issue_number="$(sed -n 's/.*target_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
outbound_comment_id="$(sed -n 's/.*outbound_comment_id: `\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
[[ -n "$target_issue_number" ]] || die "could not parse target issue from receipt"
[[ -n "$outbound_comment_id" ]] || die "could not parse outbound comment id from receipt"
log "probe queued outbound message ${outbound_comment_id} on channel issue #${target_issue_number}"

target_json="$(gh issue view "$target_issue_number" --repo "$repo" --json body,labels,comments)"
target_labels="$(jq -r '.labels[].name' <<<"$target_json")"
grep -Fxq "gitclaw:channel" <<<"$target_labels" || die "target channel issue missing gitclaw:channel label"
if grep -Fxq "gitclaw" <<<"$target_labels"; then
  die "target channel issue should not carry the model trigger label"
fi
target_body="$(jq -r '.body' <<<"$target_json")"
grep -Fq "gitclaw:channel-thread" <<<"$target_body" || die "target channel issue missing channel-thread marker"
grep -Fq 'channel="slack"' <<<"$target_body" || die "target channel issue missing slack channel marker"
target_comments="$(jq -r '[.comments[].body] | join("\n")' <<<"$target_json")"
for expected in \
  "gitclaw:channel-outbound" \
  'channel="slack"' \
  "message_id=\"${message_id}\"" \
  "GitClaw channel route probe" \
  "source_issue: #${source_issue_number}" \
  "generated_without_model_call: true" \
  "provider_delivery_strategy: channel-outbox + channel-delivery"; do
  grep -Fq "$expected" <<<"$target_comments" || die "target channel issue missing ${expected}"
done
if grep -Fq "$source_hidden_token" <<<"$target_comments"; then
  die "target channel issue leaked source token"
fi

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL=slack \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$target_issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "pending=1" <<<"$outbox_output" || die "channel outbox did not report pending probe message: ${outbox_output}"
grep -Fq '"pending_messages": 1' "$outbox_file" || die "channel outbox file missing pending probe message"
if grep -Fq "GitClaw channel route probe" <<<"$outbox_output" || grep -Fq "GitClaw channel route probe" "$outbox_file"; then
  die "channel outbox leaked probe body without --include-body"
fi

delivery_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$delivery_workflow" \
  --repo "$repo" \
  -f channel=slack \
  -f account_id="$account_id" \
  -f issue_number="$target_issue_number" \
  -f comment_id="$outbound_comment_id" \
  -f external_message_id="$external_message_id" \
  -f gateway_run_id="$gateway_run_id"
delivery_run_json="$(wait_for_delivery_run "$delivery_started_at")" || die "timed out waiting for channel-probe delivery workflow"
state_issue_number="$(wait_for_state_issue_number)" || die "timed out finding channel-probe delivery state issue"
log "delivery workflow recorded probe receipt on state issue #${state_issue_number}"

state_json="$(gh issue view "$state_issue_number" --repo "$repo" --json title,body,labels,comments)"
state_labels="$(jq -r '.labels[].name' <<<"$state_json")"
grep -Fxq "gitclaw:channel" <<<"$state_labels" || die "delivery state issue missing gitclaw:channel label"
state_body="$(jq -r '.body' <<<"$state_json")"
grep -Fq "gitclaw:channel-state" <<<"$state_body" || die "delivery state issue missing state marker"
grep -Fq "account_sha256_12=\"${account_hash}\"" <<<"$state_body" || die "delivery state issue missing account hash"
state_comments="$(jq -r '[.comments[].body] | join("\n")' <<<"$state_json")"
grep -Fq "gitclaw:channel-delivery" <<<"$state_comments" || die "delivery state issue missing delivery marker"
grep -Fq "source_comment_id=\"${outbound_comment_id}\"" <<<"$state_comments" || die "delivery marker missing outbound comment id"
grep -Fq "external_message_sha256_12=\"${external_hash}\"" <<<"$state_comments" || die "delivery marker missing external message hash"
if grep -Fq "$account_id" <<<"$state_body"$'\n'"$state_comments" || grep -Fq "$external_message_id" <<<"$state_body"$'\n'"$state_comments" || grep -Fq "$source_hidden_token" <<<"$state_body"$'\n'"$state_comments"; then
  die "delivery state issue leaked raw account, external message id, or source token"
fi

post_delivery_outbox_file="$(mktemp)"
post_delivery_outbox_output="$(GITCLAW_CHANNEL=slack \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$target_issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$post_delivery_outbox_file")"
grep -Fq "pending=0" <<<"$post_delivery_outbox_output" || die "channel outbox still reports pending probe after delivery: ${post_delivery_outbox_output}"
grep -Fq '"pending_messages": 0' "$post_delivery_outbox_file" || die "channel outbox file still has pending probe after delivery"

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels probe --route ${route} --message-id ${message_id}

Repeat the same route probe.
Do not include this hidden duplicate token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_main_run "issue_comment" "$duplicate_started_at" >/dev/null || die "timed out waiting for duplicate channel-probe issue_comment action"
wait_for_assistant_count 2 || die "expected duplicate channel-probe action receipt"
duplicate_receipt="$(latest_assistant_comment)"
for expected in \
  "GitClaw Channel Probe Action" \
  "channel_probe_status: \`duplicate\`" \
  "target_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "target_issue: \`#${target_issue_number}\`" \
  "raw_route_names_included: \`false\`" \
  "raw_source_body_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel-probe receipt missing ${expected}"
done
[[ "$(outbound_comment_count)" == "1" ]] || die "duplicate probe created another outbound comment"
for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$route" "$message_id" "Repeat the same route probe"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel-probe receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the slash channel-probe action and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
Reply with only the token after the arrow from the matching gitclaw.search_files tool output line.
Do not include the route name, target issue number, state issue number, message id, account hash, external message hash, or any probe body text.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_main_run "issue_comment" "$comment_started_at")" || die "timed out waiting for channel-probe slash model follow-up"
wait_for_assistant_count 3 || die "expected model-backed channel-probe follow-up"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel-probe search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel-probe follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel-probe follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel-probe follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel-probe follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel-probe follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel-probe follow-up marker missing usage token telemetry"

for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$account_id" "$external_message_id" "$route" "$message_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel-probe follow-up leaked ${leaked}"
  fi
done

delivery_url="$(jq -r '.url' <<<"$delivery_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}, target issue #${target_issue_number}, state issue #${state_issue_number} (delivery: ${delivery_url}; model follow-up: ${model_url})"
