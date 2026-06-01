#!/usr/bin/env bash
# gitclaw-doctor-live-issue: proactive enqueue can notify reviewed channel routes and then prove normal LLM/tool chat.
set -euo pipefail

log() {
  echo "proactive-channel-notify-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
proactive_workflow="${GITCLAW_E2E_PROACTIVE_WORKFLOW:-.github/workflows/gitclaw-proactive.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-proactive-channel-notify-e2e.lock"
cleanup_success=0

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another proactive channel notify E2E appears to be running: ${lock_dir}"
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
ensure_label gitclaw:proactive fbca04 "GitClaw proactive run"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%d%H%M%S)"
name="proactive-notify-e2e-${timestamp}"
slot="notify-${timestamp}"
notify_route="e2e-telegram-route"
message_id="gitclaw-proactive-${name}-${slot}"
thread_id="gitclaw-e2e-telegram-${notify_route}-${message_id}"
hidden_token="NOECHO_PROACTIVE_NOTIFY_PROMPT_${timestamp}"
duplicate_hidden_token="NOECHO_PROACTIVE_NOTIFY_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_PROACTIVE_NOTIFY_FOLLOWUP_${timestamp}"
account_id="telegram-proactive-notify-account-NOECHO_${timestamp}"
expected_token="GITCLAW_PROACTIVE_NOTIFY_CONTEXT_V1"
search_phrase="proactive channel notify unique search fixture phrase"
issue_number=""
channel_issue_number=""
issue_title=""
outbox_file="$(mktemp -t gitclaw-proactive-channel-notify-outbox.XXXXXX.json)"

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

wait_for_run() {
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

run_log() {
  local run_id="$1"
  gh run view "$run_id" --repo "$repo" --log
}

wait_for_issue_comment_run() {
  local started_at="$1"
  local run_json
  for _ in {1..120}; do
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

find_proactive_issue_numbers() {
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:proactive \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg name "$name" --arg slot "$slot" '.[] | select((.title | contains($name)) or ((.body | contains($name)) and (.body | contains($slot)))) | .number'
}

wait_for_proactive_issue_number() {
  for _ in {1..30}; do
    local numbers
    numbers="$(find_proactive_issue_numbers)"
    if [[ -n "$numbers" ]]; then
      echo "$numbers" | head -n 1
      return 0
    fi
    sleep 2
  done
  return 1
}

find_channel_issue_number() {
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:channel \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg thread "$thread_id" '.[] | select((.title | contains($thread)) or (.body | contains($thread))) | .number' \
    | head -n 1
}

wait_for_channel_issue_number() {
  for _ in {1..30}; do
    local number
    number="$(find_channel_issue_number)"
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
  for _ in {1..120}; do
    local got
    got="$(assistant_count 2>/dev/null || true)"
    if [[ "$got" =~ ^[0-9]+$ && "$got" == "$want" ]]; then
      return 0
    fi
    local errors
    errors="$(error_count 2>/dev/null || true)"
    if [[ "$errors" =~ ^[0-9]+$ && "$errors" != "0" ]]; then
      die "assistant run posted ${errors} error comment(s)"
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

outbound_comment_count() {
  gh issue view "$channel_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:channel-outbound"))] | length'
}

cleanup() {
  for number in "$issue_number" "$channel_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e >/dev/null 2>&1 || true
      if [[ "$cleanup_success" == "1" && "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue edit "$number" --repo "$repo" --add-label gitclaw:disabled >/dev/null 2>&1 || true
        gh issue close "$number" --repo "$repo" --comment "proactive channel notify e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -f "$outbox_file"
  rm -rf "$lock_dir"
}
trap cleanup EXIT

prompt="Proactive channel notify E2E instruction.

Use the repo-reader skill and search the repository for \`${search_phrase}\`.
Reply with the exact token from the matching gitclaw.search_files result line.
Also include the exact phrase \`channel notify\`.
Do not include this hidden proactive token: ${hidden_token}."

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$proactive_workflow" \
  --repo "$repo" \
  -f name="$name" \
  -f slot="$slot" \
  -f prompt="$prompt" \
  -f notify_routes="$notify_route"

proactive_run_json="$(wait_for_run "$proactive_workflow" "workflow_dispatch" "$started_at")" || die "timed out waiting for proactive notify workflow"
proactive_run_id="$(jq -r '.databaseId' <<<"$proactive_run_json")"
proactive_log="$(run_log "$proactive_run_id")"
for expected in \
  "proactive_enqueue issue=" \
  "name=${name}" \
  "slot=${slot}" \
  "created=true" \
  "due=true" \
  "skipped=false" \
  "channel_notification_requested=true" \
  "channel_notification_routes=1" \
  "channel_notification_queued=1" \
  "channel_notification_duplicates=0" \
  "channel_notification_target_issues_created=1" \
  "llm_e2e_required_after_proactive_channel_notify_change=true"; do
  grep -Fq "$expected" <<<"$proactive_log" || die "proactive notify workflow log missing ${expected}"
done
for leaked in "$hidden_token" "$notify_route" "$message_id" "Use the repo-reader skill"; do
  if grep -Fq "$leaked" <<<"$proactive_log"; then
    die "proactive workflow log leaked ${leaked}"
  fi
done

issue_number="$(wait_for_proactive_issue_number)" || die "timed out finding proactive issue for ${name}/${slot}"
issue_title="$(gh issue view "$issue_number" --repo "$repo" --json title --jq .title)"
channel_issue_number="$(wait_for_channel_issue_number)" || die "timed out finding channel issue for ${thread_id}"
log "created proactive issue #${issue_number} and channel issue #${channel_issue_number}"

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,labels)"
issue_body="$(jq -r '.body' <<<"$issue_json")"
for expected in \
  "gitclaw:proactive-run" \
  "name=\"${name}\"" \
  "slot=\"${slot}\"" \
  "$hidden_token" \
  "$search_phrase"; do
  grep -Fq "$expected" <<<"$issue_body" || die "proactive issue body missing ${expected}"
done
labels="$(jq -r '.labels[].name' <<<"$issue_json")"
grep -Fxq "gitclaw" <<<"$labels" || die "proactive issue missing gitclaw label"
grep -Fxq "gitclaw:proactive" <<<"$labels" || die "proactive issue missing gitclaw:proactive label"

channel_json="$(gh issue view "$channel_issue_number" --repo "$repo" --json body,labels,comments)"
channel_body="$(jq -r '.body' <<<"$channel_json")"
channel_labels="$(jq -r '.labels[].name' <<<"$channel_json")"
channel_comments="$(jq -r '[.comments[].body] | join("\n")' <<<"$channel_json")"
grep -Fxq "gitclaw:channel" <<<"$channel_labels" || die "channel issue missing gitclaw:channel label"
if grep -Fxq "gitclaw" <<<"$channel_labels"; then
  die "channel issue should not carry the model trigger label"
fi
for expected in \
  "gitclaw:channel-thread" \
  'channel="telegram"' \
  "thread_id=\"${thread_id}\""; do
  grep -Fq "$expected" <<<"$channel_body" || die "channel issue body missing ${expected}"
done
for expected in \
  "gitclaw:channel-outbound" \
  "message_id=\"${message_id}\"" \
  "GitClaw proactive run" \
  "Run issue: #${issue_number} https://github.com/${repo}/issues/${issue_number}" \
  "Name: ${name}" \
  "Slot: ${slot}" \
  "Created: true" \
  "Due: true" \
  "Not before: none"; do
  grep -Fq "$expected" <<<"$channel_comments" || die "channel notification comments missing ${expected}"
done
for leaked in "$hidden_token" "$notify_route" "$search_phrase" "Use the repo-reader skill"; do
  if grep -Fq "$leaked" <<<"$channel_comments"; then
    die "channel notification leaked ${leaked}"
  fi
done

outbox_log="$(go run ./cmd/gitclaw channel-outbox \
  --repo "$repo" \
  --channel telegram \
  --account-id "$account_id" \
  --issue-number "$channel_issue_number" \
  --include-body \
  --out "$outbox_file")"
for expected in \
  "channel_outbox issue=${channel_issue_number}" \
  "outbound_comments=1" \
  "deliverable_comments=1" \
  "pending=1" \
  "returned=1" \
  "body_included=true"; do
  grep -Fq "$expected" <<<"$outbox_log" || die "channel outbox log missing ${expected}: ${outbox_log}"
done
outbox_json="$(cat "$outbox_file")"
for expected in \
  '"channel": "telegram"' \
  '"pending_messages": 1' \
  '"messages_returned": 1' \
  '"kind": "channel-outbound"' \
  "GitClaw proactive run" \
  "Run issue: #${issue_number} https://github.com/${repo}/issues/${issue_number}"; do
  grep -Fq "$expected" <<<"$outbox_json" || die "channel outbox file missing ${expected}"
done
for leaked in "$hidden_token" "$notify_route" "$account_id" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$outbox_json" || grep -Fq "$leaked" <<<"$outbox_log"; then
    die "channel outbox leaked ${leaked}"
  fi
done

wait_for_assistant_count 1 || die "timed out waiting for proactive model response"
model_comment="$(latest_assistant_comment)"
grep -Fq "$expected_token" <<<"$model_comment" || die "model response missing search token ${expected_token}"
grep -Fiq "channel notify" <<<"$model_comment" || die "model response missing phrase channel notify"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "model response marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "model response marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "model response marker missing selected repo-reader skill"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "model response marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "model response marker missing usage token telemetry"
if grep -Fq "$hidden_token" <<<"$model_comment"; then
  die "model response leaked hidden proactive token"
fi

duplicate_prompt="Duplicate proactive notification prompt.

Do not include this hidden duplicate token: ${duplicate_hidden_token}."
duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$proactive_workflow" \
  --repo "$repo" \
  -f name="$name" \
  -f slot="$slot" \
  -f prompt="$duplicate_prompt" \
  -f notify_routes="$notify_route"

duplicate_run_json="$(wait_for_run "$proactive_workflow" "workflow_dispatch" "$duplicate_started_at")" || die "timed out waiting for duplicate proactive notify workflow"
duplicate_run_id="$(jq -r '.databaseId' <<<"$duplicate_run_json")"
duplicate_log="$(run_log "$duplicate_run_id")"
for expected in \
  "created=false" \
  "channel_notification_requested=true" \
  "channel_notification_routes=1" \
  "channel_notification_queued=0" \
  "channel_notification_duplicates=1" \
  "channel_notification_target_issues_created=0"; do
  grep -Fq "$expected" <<<"$duplicate_log" || die "duplicate proactive log missing ${expected}"
done
if grep -Fq "$duplicate_hidden_token" <<<"$duplicate_log"; then
  die "duplicate proactive log leaked duplicate hidden token"
fi
issue_count="$(find_proactive_issue_numbers | wc -l | tr -d ' ')"
[[ "$issue_count" == "1" ]] || die "duplicate enqueue created ${issue_count} proactive issues"
channel_count="$(find_channel_issue_number | wc -l | tr -d ' ')"
[[ "$channel_count" == "1" ]] || die "duplicate enqueue created ${channel_count} channel issues"
outbound_count="$(outbound_comment_count)"
[[ "$outbound_count" == "1" ]] || die "duplicate enqueue created ${outbound_count} outbound comments"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Proactive channel notification follow-up.

Use the repo-reader skill and search the repository for \`${search_phrase}\`.
The matching gitclaw.search_files result line uses this format: phrase => TOKEN.
Copy the exact TOKEN after the arrow.
Reply in exactly these two short labeled lines:
proactive_notify_search: <TOKEN from the matching gitclaw.search_files line>
mode: channel-notify
Do not include this hidden follow-up token: ${followup_hidden_token}.
Do not use @file or @folder references." >/dev/null

wait_for_issue_comment_run "$comment_started_at" >/dev/null || die "timed out waiting for proactive notify follow-up issue_comment workflow run"
wait_for_assistant_count 2 || die "expected model-backed follow-up assistant comment"
followup_comment="$(latest_assistant_comment)"
grep -Fq "$expected_token" <<<"$followup_comment" || die "model follow-up missing search token ${expected_token}"
grep -Fiq "channel-notify" <<<"$followup_comment" || die "model follow-up missing mode channel-notify"
grep -Fq 'skills="repo-reader"' <<<"$followup_comment" || die "model follow-up marker missing selected repo-reader skill"
grep -Fq 'gitclaw.search_files' <<<"$followup_comment" || die "model follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$followup_comment" || die "model follow-up marker missing usage token telemetry"
if grep -Fq "$followup_hidden_token" <<<"$followup_comment"; then
  die "model follow-up leaked hidden follow-up token"
fi

sleep 15
final_count="$(assistant_count)"
[[ "$final_count" == "2" ]] || die "follow-up bot loop suspected: expected 2 assistant comments, got ${final_count}"
log "passed for proactive issue #${issue_number} and channel issue #${channel_issue_number}"
cleanup_success=1
