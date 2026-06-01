#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-send slash action creates the live issue.
set -euo pipefail

log() {
  echo "channel-send-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-send-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-send slash E2E appears to be running: ${lock_dir}"
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
route="e2e-slack-route"
message_id="slash-${timestamp}"
account_id="slack-slash-account-NOECHO_CHANNEL_SEND_SLASH_ACCOUNT_${timestamp}"
body_token="NOECHO_CHANNEL_SEND_SLASH_BODY_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_SEND_SLASH_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_SEND_SLASH_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_SEND_SLASH_CONTEXT_V1"
search_phrase="channel send slash unique search fixture phrase"
source_title="@gitclaw /channels send --route ${route} --message-id ${message_id}"
source_issue_number=""
target_issue_number=""

run_list_json() {
  local event="$1"
  local started_at="$2"
  gh run list \
    --repo "$repo" \
    --workflow "$main_workflow" \
    --event "$event" \
    --created ">=$started_at" \
    --limit 20 \
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

cleanup() {
  for number in "$source_issue_number" "$target_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel-send slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

outbound_body="GitClaw slash channel-send E2E message.

Visible outbound token for the provider queue: ${body_token}"

opened_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
source_issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$source_title" \
  --body "$outbound_body")"
source_issue_number="${source_issue_url##*/}"

wait_for_main_run "issues" "$opened_started_at" >/dev/null || die "timed out waiting for source issue channel-send action"
wait_for_assistant_count 1 || die "expected one channel-send action receipt"
receipt="$(latest_assistant_comment)"

for expected in \
  "GitClaw Channel Send Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels send\`" \
  "channel_send_status: \`queued\`" \
  "target_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "route_resolved: \`true\`" \
  "message_id_auto: \`false\`" \
  "raw_outbound_body_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "llm_e2e_required_after_channel_send_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "channel-send action receipt missing ${expected}"
done
for leaked in "$body_token" "$account_id" "$message_id"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "channel-send action receipt leaked ${leaked}"
  fi
done

target_issue_number="$(sed -n 's/.*target_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
[[ -n "$target_issue_number" ]] || die "could not parse target issue from receipt"
log "slash action queued outbound message on channel issue #${target_issue_number}"

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
  "$body_token"; do
  grep -Fq "$expected" <<<"$target_comments" || die "target channel issue missing ${expected}"
done

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL=slack \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$target_issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "pending=1" <<<"$outbox_output" || die "channel outbox did not report pending slash message: ${outbox_output}"
grep -Fq '"pending_messages": 1' "$outbox_file" || die "channel outbox file missing pending slash message"
if grep -Fq "$body_token" <<<"$outbox_output" || grep -Fq "$body_token" "$outbox_file"; then
  die "channel outbox leaked slash body without --include-body"
fi

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels send --route ${route} --message-id ${message_id}
${outbound_body}

Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_main_run "issue_comment" "$duplicate_started_at" >/dev/null || die "timed out waiting for duplicate channel-send issue_comment action"
wait_for_assistant_count 2 || die "expected duplicate channel-send action receipt"
duplicate_receipt="$(latest_assistant_comment)"
for expected in \
  "GitClaw Channel Send Action" \
  "channel_send_status: \`duplicate\`" \
  "duplicate_suppressed: \`true\`" \
  "target_issue: \`#${target_issue_number}\`" \
  "raw_outbound_body_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel-send receipt missing ${expected}"
done
[[ "$(outbound_comment_count)" == "1" ]] || die "duplicate slash send created another outbound comment"
for leaked in "$body_token" "$duplicate_hidden_token"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel-send receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the slash channel-send action and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
Reply with only the token after the arrow from the matching gitclaw.search_files tool output line.
Do not include the route name, target issue number, message id, account hash, or any channel body token.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_main_run "issue_comment" "$comment_started_at")" || die "timed out waiting for channel-send slash model follow-up"
wait_for_assistant_count 3 || die "expected model-backed follow-up assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include slash search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant slash follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant slash follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant slash follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant slash follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant slash follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant slash follow-up marker missing usage token telemetry"

for leaked in "$body_token" "$duplicate_hidden_token" "$followup_hidden_token" "$account_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model slash follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}, target issue #${target_issue_number} (model follow-up: ${model_url})"
