#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel ingest workflow creates the live issue.
set -euo pipefail

log() {
  echo "channel-outbox-workflow-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
outbox_workflow="${GITCLAW_E2E_CHANNEL_OUTBOX_WORKFLOW:-.github/workflows/gitclaw-channel-outbox.yml}"
delivery_workflow="${GITCLAW_E2E_CHANNEL_DELIVERY_WORKFLOW:-.github/workflows/gitclaw-channel-delivery.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-outbox-workflow-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-outbox workflow E2E appears to be running: ${lock_dir}"
fi

if [[ -z "$repo" ]]; then
  repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
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
thread_id="channel-outbox-e2e-${timestamp}"
message_id="message-${timestamp}"
account_id="telegram-outbox-account-NOECHO_CHANNEL_OUTBOX_ACCOUNT_${timestamp}"
external_message_id="telegram-outbox-delivered-NOECHO_CHANNEL_OUTBOX_EXTERNAL_${timestamp}"
gateway_run_id="channel-outbox-workflow-e2e-${timestamp}"
hidden_token="NOECHO_CHANNEL_OUTBOX_MESSAGE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_OUTBOX_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_OUTBOX_CONTEXT_V1"
followup_expected_token="GITCLAW_CHANNEL_OUTBOX_FOLLOWUP_CONTEXT_V1"
search_phrase="channel outbox unique search fixture phrase"
followup_search_phrase="channel outbox followup unique search fixture phrase"
account_hash="$(sha256_12 "$account_id")"
external_hash="$(sha256_12 "$external_message_id")"
issue_number=""
state_issue=""

run_list_json() {
  local workflow="$1"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event workflow_dispatch \
    --limit 20 \
    --json databaseId,status,conclusion,createdAt,url \
    --jq '.'
}

wait_for_workflow_run() {
  local workflow="$1"
  local started_at="$2"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json "$workflow" | jq -c --arg started "$started_at" '[.[] | select(.createdAt >= $started)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${workflow} run failed with conclusion ${conclusion}: ${url}"
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
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$main_workflow" \
      --event issue_comment \
      --created ">=$started_at" \
      --limit 10 \
      --json databaseId,status,conclusion,createdAt,url,displayTitle \
      --jq '. as $runs | $runs | map(select(.displayTitle == "'"${issue_title}"'")) | sort_by(.createdAt) | reverse | .[0] // empty')"
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

find_issue_numbers() {
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:channel \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg thread "$thread_id" '.[] | select((.title | contains($thread)) or (.body | contains($thread))) | .number'
}

wait_for_issue_number() {
  for _ in {1..30}; do
    local number
    number="$(find_issue_numbers | head -n 1)"
    if [[ -n "$number" && "$number" != "null" ]]; then
      echo "$number"
      return 0
    fi
    sleep 2
  done
  return 1
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

assistant_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

latest_assistant_comment_id() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .id] | .[-1] // empty'
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
      die "assistant run posted ${errors} error marker comment(s)"
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

run_log() {
  local run_json="$1"
  local run_id
  run_id="$(jq -r '.databaseId' <<<"$run_json")"
  gh run view "$run_id" --repo "$repo" --log
}

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "channel-outbox workflow e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  local numbers
  numbers="$(find_state_issue_numbers || true)"
  while read -r number; do
    [[ -n "$number" ]] || continue
    gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$number" --repo "$repo" --comment "channel-outbox state e2e cleanup" >/dev/null 2>&1 || true
    fi
  done <<<"$numbers"
  rm -rf "$lock_dir"
}
trap cleanup EXIT

body="@gitclaw Use the repo-reader skill and search the repository for \`${search_phrase}\`.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${search_phrase} => <token>\`.
Reply with only the uppercase fixture token after the arrow from the matching gitclaw.search_files tool output line.
Do not include this hidden channel-outbox token: ${hidden_token}
Keep the answer under 30 words."

ingest_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$ingest_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f thread_id="$thread_id" \
  -f message_id="$message_id" \
  -f author="telegram:e2e" \
  -f body="$body"

ingest_run_json="$(wait_for_workflow_run "$ingest_workflow" "$ingest_started_at")" || die "timed out waiting for channel ingest workflow"
ingest_log="$(run_log "$ingest_run_json")"
for leaked in "$thread_id" "$message_id" "$hidden_token" "$expected_token"; do
  if grep -Fq "$leaked" <<<"$ingest_log"; then
    die "channel ingest log leaked ${leaked}"
  fi
done
issue_number="$(wait_for_issue_number)" || die "timed out finding channel issue for ${thread_id}"
issue_title="GitClaw ${channel} thread ${thread_id}"
log "channel ingest created issue #${issue_number}"
wait_for_workflow_run "$main_workflow" "$ingest_started_at" >/dev/null || die "timed out waiting for channel message model workflow"
wait_for_assistant_count 1 || die "expected model-backed assistant reply"
model_comment="$(latest_assistant_comment)"
source_comment_id="$(latest_assistant_comment_id)"
[[ -n "$source_comment_id" ]] || die "could not find source assistant comment id"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant marker missing usage token telemetry"
if grep -Fq "$hidden_token" <<<"$model_comment"; then
  die "assistant leaked hidden channel token"
fi

outbox_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$outbox_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f account_id="$account_id" \
  -f issue_number="$issue_number" \
  -f include_body="false" \
  -f limit="10"

outbox_run_json="$(wait_for_workflow_run "$outbox_workflow" "$outbox_started_at")" || die "timed out waiting for channel-outbox workflow"
outbox_log="$(run_log "$outbox_run_json")"
for expected in \
  "channel_outbox issue=${issue_number}" \
  "assistant_comments=1" \
  "delivered=0" \
  "pending=1" \
  "returned=1" \
  "body_included=false" \
  "account_sha256_12=${account_hash}"; do
  grep -Fq "$expected" <<<"$outbox_log" || die "channel outbox log missing ${expected}"
done
for leaked in "$account_id" "$external_message_id" "$hidden_token" "$expected_token"; do
  if grep -Fq "$leaked" <<<"$outbox_log"; then
    die "channel outbox log leaked ${leaked}"
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

delivery_run_json="$(wait_for_workflow_run "$delivery_workflow" "$delivery_started_at")" || die "timed out waiting for channel-delivery workflow"
delivery_log="$(run_log "$delivery_run_json")"
for expected in \
  "channel_delivery state_issue=" \
  "issue=${issue_number}" \
  "source_comment=${source_comment_id}" \
  "account_sha256_12=${account_hash}" \
  "external_message_sha256_12=${external_hash}"; do
  grep -Fq "$expected" <<<"$delivery_log" || die "channel delivery log missing ${expected}"
done
for leaked in "$account_id" "$external_message_id" "$hidden_token" "$expected_token"; do
  if grep -Fq "$leaked" <<<"$delivery_log"; then
    die "channel delivery log leaked ${leaked}"
  fi
done
state_issue="$(find_state_issue_numbers | head -n 1)"
[[ -n "$state_issue" ]] || die "timed out finding channel state issue"
state_json="$(gh issue view "$state_issue" --repo "$repo" --json body,comments)"
grep -Fq "account_sha256_12=\"${account_hash}\"" <<<"$(jq -r '.body' <<<"$state_json")" || die "state issue missing account hash"
grep -Fq "source_comment_id=\"${source_comment_id}\"" <<<"$(jq -r '[.comments[].body] | join("\n")' <<<"$state_json")" || die "state issue missing delivery receipt"
grep -Fq "external_message_sha256_12=\"${external_hash}\"" <<<"$(jq -r '[.comments[].body] | join("\n")' <<<"$state_json")" || die "state issue missing external message hash"

second_outbox_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$outbox_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f account_id="$account_id" \
  -f issue_number="$issue_number" \
  -f include_body="false" \
  -f limit="10"

second_outbox_run_json="$(wait_for_workflow_run "$outbox_workflow" "$second_outbox_started_at")" || die "timed out waiting for second channel-outbox workflow"
second_outbox_log="$(run_log "$second_outbox_run_json")"
for expected in \
  "channel_outbox issue=${issue_number}" \
  "assistant_comments=1" \
  "delivered=1" \
  "pending=0" \
  "returned=0" \
  "body_included=false" \
  "account_sha256_12=${account_hash}"; do
  grep -Fq "$expected" <<<"$second_outbox_log" || die "second channel outbox log missing ${expected}"
done
for leaked in "$account_id" "$external_message_id" "$hidden_token" "$expected_token"; do
  if grep -Fq "$leaked" <<<"$second_outbox_log"; then
    die "second channel outbox log leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the channel-outbox workflow and use the repo-reader skill.

Search the repository for \`${followup_search_phrase}\`.
Search for that exact phrase, not shorter words from it.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${followup_search_phrase} => <token>\`.
Reply with only the uppercase fixture token after the arrow from the matching gitclaw.search_files tool output line.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

followup_run_json="$(wait_for_issue_comment_run "$comment_started_at")" || die "timed out waiting for channel-outbox issue_comment follow-up"
wait_for_assistant_count 2 || die "expected model-backed follow-up assistant comment"
followup_comment="$(latest_assistant_comment)"
grep -Fq "$followup_expected_token" <<<"$followup_comment" || die "assistant did not include follow-up search_files token ${followup_expected_token}"
grep -Fq 'prompt_context_sha256_12="' <<<"$followup_comment" || die "assistant follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$followup_comment" || die "assistant follow-up marker missing selected repo-reader skill"
grep -Fq 'gitclaw.search_files' <<<"$followup_comment" || die "assistant follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$followup_comment" || die "assistant follow-up marker missing usage token telemetry"
for leaked in "$account_id" "$external_message_id" "$hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$followup_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

outbox_url="$(jq -r '.url' <<<"$outbox_run_json")"
followup_url="$(jq -r '.url' <<<"$followup_run_json")"
log "passed for issue #${issue_number}: ${outbox_url} (model follow-up: ${followup_url})"
