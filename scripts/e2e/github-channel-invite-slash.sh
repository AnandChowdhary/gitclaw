#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-invite slash action shares a GitHub issue to reviewed routes.
set -euo pipefail

log() {
  echo "channel-invite-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-invite-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-invite slash E2E appears to be running: ${lock_dir}"
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
message_id="invite-${timestamp}"
slack_account_id="slack-invite-account-NOECHO_CHANNEL_INVITE_SLACK_${timestamp}"
telegram_account_id="telegram-invite-account-NOECHO_CHANNEL_INVITE_TELEGRAM_${timestamp}"
note_token="NOECHO_CHANNEL_INVITE_NOTE_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_INVITE_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_INVITE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_INVITE_CONTEXT_V1"
search_phrase="channel invite unique search fixture phrase"
source_title="GitClaw channel invite E2E ${timestamp}"
source_issue_number=""
target_issues=()

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
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:channel-outbound"))] | length'
}

cleanup() {
  local numbers=("$source_issue_number" "${target_issues[@]:-}")
  for number in "${numbers[@]}"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel-invite slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

invite_note="Please join this GitHub issue from the routed channel.

Visible invite note token for provider queue inspection: ${note_token}"

opened_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
source_issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$source_title" \
  --body "@gitclaw /channels invite ${route_a},${route_b} --message-id ${message_id}

${invite_note}")"
source_issue_number="${source_issue_url##*/}"

wait_for_main_run "issues" "$opened_started_at" >/dev/null || die "timed out waiting for source issue channel-invite action"
wait_for_assistant_count 1 || die "expected one channel-invite action receipt"
receipt="$(latest_assistant_comment)"

for expected in \
  "GitClaw Channel Invite Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels invite\`" \
  "channel_invite_status: \`queued\`" \
  "invite_routes: \`2\`" \
  "invite_queued: \`2\`" \
  "invite_duplicates: \`0\`" \
  "target_issues_created: \`2\`" \
  "message_id_auto: \`false\`" \
  "raw_route_names_included: \`false\`" \
  "raw_issue_title_included: \`false\`" \
  "raw_invite_note_included: \`false\`" \
  "raw_outbound_body_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox + channel-delivery\`" \
  "llm_e2e_required_after_channel_invite_action_change: \`true\`" \
  "channel=\`slack\`" \
  "channel=\`telegram\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "channel-invite action receipt missing ${expected}"
done
for leaked in "$note_token" "$route_a" "$route_b" "$message_id" "$source_title" "$slack_account_id" "$telegram_account_id"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "channel-invite action receipt leaked ${leaked}"
  fi
done

mapfile -t target_issues < <(sed -n 's/.*target_issue=`#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | awk '!seen[$0]++')
[[ "${#target_issues[@]}" == "2" ]] || die "expected two invite target issues, got ${#target_issues[@]} from receipt"
log "invite queued outbound messages on channel issues #${target_issues[0]} and #${target_issues[1]}"

combined_targets=""
for target_issue_number in "${target_issues[@]}"; do
  target_json="$(gh issue view "$target_issue_number" --repo "$repo" --json body,labels,comments)"
  target_labels="$(jq -r '.labels[].name' <<<"$target_json")"
  grep -Fxq "gitclaw:channel" <<<"$target_labels" || die "target channel issue #${target_issue_number} missing gitclaw:channel label"
  if grep -Fxq "gitclaw" <<<"$target_labels"; then
    die "target channel issue #${target_issue_number} should not carry the model trigger label"
  fi
  target_body="$(jq -r '.body' <<<"$target_json")"
  target_comments="$(jq -r '[.comments[].body] | join("\n")' <<<"$target_json")"
  combined_targets+="${target_body}"$'\n'"${target_comments}"$'\n'
  grep -Fq "gitclaw:channel-thread" <<<"$target_body" || die "target channel issue #${target_issue_number} missing channel-thread marker"
  for expected in \
    "gitclaw:channel-outbound" \
    "GitClaw channel invite" \
    "Issue: #${source_issue_number} ${source_title}" \
    "https://github.com/${repo}/issues/${source_issue_number}" \
    "message_id=\"${message_id}\"" \
    "$note_token"; do
    grep -Fq "$expected" <<<"$target_comments" || die "target channel issue #${target_issue_number} missing ${expected}"
  done

  if grep -Fq 'channel="slack"' <<<"$target_body"; then
    account_id="$slack_account_id"
    channel="slack"
  elif grep -Fq 'channel="telegram"' <<<"$target_body"; then
    account_id="$telegram_account_id"
    channel="telegram"
  else
    die "target channel issue #${target_issue_number} missing expected provider marker"
  fi
  outbox_file="$(mktemp)"
  outbox_output="$(GITCLAW_CHANNEL="$channel" \
    GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
    GITCLAW_CHANNEL_ISSUE_NUMBER="$target_issue_number" \
    go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
  grep -Fq "pending=1" <<<"$outbox_output" || die "channel outbox did not report pending invite for #${target_issue_number}: ${outbox_output}"
  grep -Fq '"pending_messages": 1' "$outbox_file" || die "channel outbox file missing pending invite for #${target_issue_number}"
  if grep -Fq "$note_token" <<<"$outbox_output" || grep -Fq "$note_token" "$outbox_file"; then
    die "channel outbox leaked invite body without --include-body for #${target_issue_number}"
  fi
done
grep -Fq 'channel="slack"' <<<"$combined_targets" || die "invite targets missing slack route output"
grep -Fq 'channel="telegram"' <<<"$combined_targets" || die "invite targets missing telegram route output"

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels invite ${route_a},${route_b} --message-id ${message_id}
${invite_note}

Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_main_run "issue_comment" "$duplicate_started_at" >/dev/null || die "timed out waiting for duplicate channel-invite issue_comment action"
wait_for_assistant_count 2 || die "expected duplicate channel-invite action receipt"
duplicate_receipt="$(latest_assistant_comment)"
for expected in \
  "GitClaw Channel Invite Action" \
  "channel_invite_status: \`duplicate\`" \
  "invite_queued: \`0\`" \
  "invite_duplicates: \`2\`" \
  "raw_invite_note_included: \`false\`" \
  "raw_outbound_body_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel-invite receipt missing ${expected}"
done
for target_issue_number in "${target_issues[@]}"; do
  [[ "$(outbound_comment_count "$target_issue_number")" == "1" ]] || die "duplicate invite created another outbound comment on #${target_issue_number}"
done
for leaked in "$note_token" "$duplicate_hidden_token" "$route_a" "$route_b" "$message_id" "$source_title"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel-invite receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the slash channel-invite action and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line ends with an all-caps token after the arrow.
That token starts with \`GITCLAW_CHANNEL_INVITE_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include route names, target issue numbers, message ids, account hashes, source issue title, or any channel invite note token.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_main_run "issue_comment" "$comment_started_at")" || die "timed out waiting for channel-invite slash model follow-up"
wait_for_assistant_count 3 || die "expected model-backed follow-up assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include invite search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant invite follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant invite follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant invite follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant invite follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant invite follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant invite follow-up marker missing usage token telemetry"

for leaked in "$note_token" "$duplicate_hidden_token" "$followup_hidden_token" "$slack_account_id" "$telegram_account_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model invite follow-up leaked ${leaked}"
  fi
done

url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}: ${url}"
