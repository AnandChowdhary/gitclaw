#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "channel-gateway-workflow-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
gateway_workflow="${GITCLAW_E2E_CHANNEL_GATEWAY_WORKFLOW:-.github/workflows/gitclaw-channel-gateway.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-gateway-workflow-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-gateway workflow E2E appears to be running: ${lock_dir}"
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
account_id="telegram-gateway-account-NOECHO_CHANNEL_GATEWAY_${timestamp}"
gateway_slot="gateway-slot-${timestamp}"
lease_run_id="channel-gateway-workflow-e2e-${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_GATEWAY_FOLLOWUP_${timestamp}"
second_followup_hidden_token="NOECHO_CHANNEL_GATEWAY_SECOND_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_GATEWAY_CONTEXT_V1"
second_expected_token="GITCLAW_CHANNEL_GATEWAY_FOLLOWUP_CONTEXT_V1"
search_phrase="channel gateway unique search fixture phrase"
second_search_phrase="channel gateway followup unique search fixture phrase"
account_hash="$(sha256_12 "$account_id")"
lease_offset="gateway-lease|channel=${channel}|account_id=${account_id}|slot=${gateway_slot}|run_id=${lease_run_id}"
lease_hash="$(sha256_12 "$lease_offset")"

run_list_json() {
  gh run list \
    --repo "$repo" \
    --workflow "$gateway_workflow" \
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
        [[ "$conclusion" == "success" ]] || die "${gateway_workflow} run failed with conclusion ${conclusion}: ${url}"
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
    | jq -r --arg account_hash "$account_hash" '.[] | select((.title | contains($account_hash)) or (.body | contains($account_hash))) | .number'
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

cleanup() {
  local numbers
  numbers="$(find_issue_numbers || true)"
  while read -r number; do
    [[ -n "$number" ]] || continue
    gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$number" --repo "$repo" --comment "channel-gateway workflow e2e cleanup" >/dev/null 2>&1 || true
    fi
  done <<<"$numbers"
  rm -rf "$lock_dir"
}
trap cleanup EXIT

first_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$gateway_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f account_id="$account_id" \
  -f gateway_slot="$gateway_slot" \
  -f lease_run_id="$lease_run_id" \
  -f renew=false

run_json="$(wait_for_run "$first_started_at")" || die "timed out waiting for channel-gateway workflow"
issue_number="$(wait_for_issue_number)" || die "timed out finding channel gateway state issue"
issue_title="GitClaw ${channel} channel state ${account_hash}"
log "gateway workflow created issue #${issue_number}"

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json title,body,labels,comments)"
labels="$(jq -r '.labels[].name' <<<"$issue_json")"
grep -Fxq "gitclaw:channel" <<<"$labels" || die "gateway state issue missing gitclaw:channel label"
body="$(jq -r '.body' <<<"$issue_json")"
grep -Fq "gitclaw:channel-state" <<<"$body" || die "gateway state issue missing state marker"
grep -Fq "account_sha256_12=\"${account_hash}\"" <<<"$body" || die "gateway state issue missing account hash marker"
comments="$(jq -r '[.comments[].body] | join("\n---GITCLAW-COMMENT---\n")' <<<"$issue_json")"
grep -Fq "gitclaw:channel-state-update" <<<"$comments" || die "gateway state issue missing lease update marker"
grep -Fq "offset_sha256_12=\"${lease_hash}\"" <<<"$comments" || die "gateway lease update missing lease hash marker"
visible="$(jq -r '[.title, .body, (.comments[].body)] | join("\n")' <<<"$issue_json")"
if grep -Fq "$account_id" <<<"$visible" || grep -Fq "$lease_offset" <<<"$visible"; then
  die "gateway workflow leaked raw account or lease offset"
fi
if grep -Fq "$expected_token" <<<"$visible" || grep -Fq "$search_phrase" <<<"$visible"; then
  die "gateway workflow leaked follow-up fixture context"
fi
if grep -Fq "$second_expected_token" <<<"$visible" || grep -Fq "$second_search_phrase" <<<"$visible"; then
  die "gateway workflow leaked second follow-up fixture context"
fi

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$gateway_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f account_id="$account_id" \
  -f gateway_slot="$gateway_slot" \
  -f lease_run_id="$lease_run_id" \
  -f renew=false

wait_for_run "$duplicate_started_at" >/dev/null || die "timed out waiting for duplicate channel-gateway workflow"
issue_count="$(find_issue_numbers | wc -l | tr -d ' ')"
[[ "$issue_count" == "1" ]] || die "duplicate gateway workflow created ${issue_count} issues"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json comments)"
state_update_count="$(jq -r '[.comments[] | select(.body | contains("gitclaw:channel-state-update"))] | length' <<<"$issue_json")"
[[ "$state_update_count" == "1" ]] || die "duplicate gateway workflow produced ${state_update_count} state update comments"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the channel-gateway workflow and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
Reply with only the token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with any token from this issue or its comments.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run "$comment_started_at")" || die "timed out waiting for channel-gateway issue_comment follow-up"
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

for leaked in "$account_id" "$lease_offset" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

second_comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel-gateway lease issue conversation.

Use the repo-reader skill and search the repository for \`${second_search_phrase}\`.
The matching repository search result line has the form \`${second_search_phrase} => <token>\`.
Reply with only the token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with any token from this issue or its comments.
Do not include this hidden follow-up token: ${second_followup_hidden_token}
Keep the answer under 30 words." >/dev/null

second_model_run_json="$(wait_for_issue_comment_run "$second_comment_started_at")" || die "timed out waiting for channel-gateway second issue_comment follow-up"
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

for leaked in "$account_id" "$lease_offset" "$followup_hidden_token" "$second_followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$second_model_comment"; then
    die "second model follow-up leaked ${leaked}"
  fi
done

url="$(jq -r '.url' <<<"$run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
second_model_url="$(jq -r '.url' <<<"$second_model_run_json")"
log "passed for issue #${issue_number}: ${url} (model follow-up: ${model_url}; second follow-up: ${second_model_url})"
