#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "channel-delivery-workflow-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
delivery_workflow="${GITCLAW_E2E_CHANNEL_DELIVERY_WORKFLOW:-.github/workflows/gitclaw-channel-delivery.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-delivery-workflow-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-delivery workflow E2E appears to be running: ${lock_dir}"
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
account_id="telegram-delivery-account-NOECHO_CHANNEL_DELIVERY_${timestamp}"
external_message_id="telegram-delivery-message-NOECHO_CHANNEL_DELIVERY_EXTERNAL_${timestamp}"
gateway_run_id="channel-delivery-workflow-e2e-${timestamp}"
source_token="NOECHO_CHANNEL_DELIVERY_SOURCE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_DELIVERY_FOLLOWUP_${timestamp}"
second_followup_hidden_token="NOECHO_CHANNEL_DELIVERY_SECOND_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_DELIVERY_CONTEXT_V1"
second_expected_token="GITCLAW_CHANNEL_DELIVERY_FOLLOWUP_CONTEXT_V1"
search_phrase="channel delivery unique search fixture phrase"
second_search_phrase="channel delivery followup unique search fixture phrase"
account_hash="$(sha256_12 "$account_id")"
external_hash="$(sha256_12 "$external_message_id")"
source_issue=""

run_list_json() {
  gh run list \
    --repo "$repo" \
    --workflow "$delivery_workflow" \
    --event workflow_dispatch \
    --limit 10 \
    --json databaseId,status,conclusion,createdAt,url \
    --jq '.'
}

wait_for_run() {
  local started_at="$1"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json | jq -c --arg started "$started_at" '[.[] | select(.createdAt >= $started)] | sort_by(.createdAt) | reverse | .[0] // empty')"
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

wait_for_issue_comment_run() {
  local started_at="$1"
  for _ in {1..90}; do
    local run_json
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$main_workflow" \
      --event issue_comment \
      --created ">=$started_at" \
      --limit 10 \
      --json databaseId,status,conclusion,createdAt,url,displayTitle \
      --jq '. as $runs | $runs | map(select(.displayTitle == "'"${state_issue_title}"'")) | sort_by(.createdAt) | reverse | .[0] // empty')"
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

assistant_count() {
  gh issue view "$state_issue" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

latest_assistant_comment() {
  gh issue view "$state_issue" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

error_count() {
  gh issue view "$state_issue" \
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

cleanup() {
  local numbers
  numbers="$(find_state_issue_numbers || true)"
  while read -r number; do
    [[ -n "$number" ]] || continue
    gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$number" --repo "$repo" --comment "channel-delivery workflow e2e cleanup" >/dev/null 2>&1 || true
    fi
  done <<<"$numbers"
  if [[ -n "${source_issue:-}" ]]; then
    gh issue edit "$source_issue" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$source_issue" --repo "$repo" --comment "channel-delivery source e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

source_url="$(gh issue create \
  --repo "$repo" \
  --title "channel delivery source e2e ${timestamp}" \
  --body "Temporary source issue for channel delivery E2E. Hidden source issue token: ${source_token}" \
  --label gitclaw:e2e \
  --label gitclaw:disabled)"
source_issue="${source_url##*/}"
assistant_body="<!-- gitclaw:assistant-turn run_id=\"delivery-e2e-${timestamp}\" event_id=\"issue-${source_issue}\" model=\"gitclaw/e2e\" idempotency_key=\"delivery-${timestamp}\" -->
Assistant reply source for channel delivery E2E.

Hidden assistant source token: ${source_token}"
source_comment_id="$(gh api "repos/${repo}/issues/${source_issue}/comments" -f body="$assistant_body" --jq .id)"
log "created source issue #${source_issue} comment ${source_comment_id}"

first_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$delivery_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f account_id="$account_id" \
  -f issue_number="$source_issue" \
  -f comment_id="$source_comment_id" \
  -f external_message_id="$external_message_id" \
  -f gateway_run_id="$gateway_run_id"

run_json="$(wait_for_run "$first_started_at")" || die "timed out waiting for channel-delivery workflow"
state_issue="$(wait_for_state_issue_number)" || die "timed out finding channel delivery state issue"
state_issue_title="GitClaw ${channel} channel state ${account_hash}"
log "delivery workflow created state issue #${state_issue}"

issue_json="$(gh issue view "$state_issue" --repo "$repo" --json title,body,labels,comments)"
labels="$(jq -r '.labels[].name' <<<"$issue_json")"
grep -Fxq "gitclaw:channel" <<<"$labels" || die "delivery state issue missing gitclaw:channel label"
body="$(jq -r '.body' <<<"$issue_json")"
grep -Fq "gitclaw:channel-state" <<<"$body" || die "delivery state issue missing state marker"
grep -Fq "account_sha256_12=\"${account_hash}\"" <<<"$body" || die "delivery state issue missing account hash marker"
comments="$(jq -r '[.comments[].body] | join("\n---GITCLAW-COMMENT---\n")' <<<"$issue_json")"
grep -Fq "gitclaw:channel-delivery" <<<"$comments" || die "delivery state issue missing delivery receipt marker"
grep -Fq "source_comment_id=\"${source_comment_id}\"" <<<"$comments" || die "delivery receipt missing source comment id"
grep -Fq "external_message_sha256_12=\"${external_hash}\"" <<<"$comments" || die "delivery receipt missing external message hash marker"
visible="$(jq -r '[.title, .body, (.comments[].body)] | join("\n")' <<<"$issue_json")"
if grep -Fq "$account_id" <<<"$visible" || grep -Fq "$external_message_id" <<<"$visible" || grep -Fq "$source_token" <<<"$visible"; then
  die "delivery workflow leaked raw account, external message, or assistant source body"
fi
if grep -Fq "$expected_token" <<<"$visible" || grep -Fq "$search_phrase" <<<"$visible"; then
  die "delivery workflow leaked follow-up fixture context"
fi
if grep -Fq "$second_expected_token" <<<"$visible" || grep -Fq "$second_search_phrase" <<<"$visible"; then
  die "delivery workflow leaked second follow-up fixture context"
fi

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$delivery_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f account_id="$account_id" \
  -f issue_number="$source_issue" \
  -f comment_id="$source_comment_id" \
  -f external_message_id="$external_message_id" \
  -f gateway_run_id="$gateway_run_id"

wait_for_run "$duplicate_started_at" >/dev/null || die "timed out waiting for duplicate channel-delivery workflow"
issue_count="$(find_state_issue_numbers | wc -l | tr -d ' ')"
[[ "$issue_count" == "1" ]] || die "duplicate delivery workflow created ${issue_count} state issues"
issue_json="$(gh issue view "$state_issue" --repo "$repo" --json comments)"
delivery_count="$(jq -r '[.comments[] | select(.body | contains("gitclaw:channel-delivery"))] | length' <<<"$issue_json")"
[[ "$delivery_count" == "1" ]] || die "duplicate delivery workflow produced ${delivery_count} delivery receipts"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$state_issue" \
  --repo "$repo" \
  --body "@gitclaw Continue after the channel-delivery workflow and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
Search for that exact phrase, not shorter words from it.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${search_phrase} => <token>\`.
Reply with only the uppercase fixture token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with an issue title, issue number, source comment id, gateway run id, account hash, external message hash, or any 12-character hash from this issue.
Do not answer with any token from this issue or its comments.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run "$comment_started_at")" || die "timed out waiting for channel-delivery issue_comment follow-up"
wait_for_assistant_count 1 || die "expected model-backed follow-up assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant marker missing usage token telemetry"

for leaked in "$account_id" "$external_message_id" "$source_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

second_comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$state_issue" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel-delivery receipt issue conversation.

Use the repo-reader skill and search the repository for \`${second_search_phrase}\`.
Search for that exact phrase, not shorter words from it.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${second_search_phrase} => <token>\`.
Reply with only the uppercase fixture token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with an issue title, issue number, source comment id, gateway run id, account hash, external message hash, or any 12-character hash from this issue.
Do not answer with any token from this issue or its comments.
Do not include this hidden follow-up token: ${second_followup_hidden_token}
Keep the answer under 30 words." >/dev/null

second_model_run_json="$(wait_for_issue_comment_run "$second_comment_started_at")" || die "timed out waiting for channel-delivery second issue_comment follow-up"
wait_for_assistant_count 2 || die "expected second model-backed follow-up assistant comment"
second_model_comment="$(latest_assistant_comment)"

grep -Fq "$second_expected_token" <<<"$second_model_comment" || die "assistant did not include second search_files token ${second_expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$second_model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$second_model_comment"; then
  die "assistant second marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$second_model_comment" || die "assistant second marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$second_model_comment" || die "assistant second marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$second_model_comment" || die "assistant second marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$second_model_comment" || die "assistant second marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$second_model_comment" || die "assistant second marker missing usage token telemetry"

for leaked in "$account_id" "$external_message_id" "$source_token" "$followup_hidden_token" "$second_followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$second_model_comment"; then
    die "second model follow-up leaked ${leaked}"
  fi
done

url="$(jq -r '.url' <<<"$run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
second_model_url="$(jq -r '.url' <<<"$second_model_run_json")"
log "passed for state issue #${state_issue}: ${url} (model follow-up: ${model_url}; second follow-up: ${second_model_url})"
