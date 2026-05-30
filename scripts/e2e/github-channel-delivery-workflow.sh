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

ensure_label gitclaw:channel 1d76db "GitClaw mirrored channel thread"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
channel="telegram"
account_id="telegram-delivery-account-GITCLAW_CHANNEL_DELIVERY_E2E_${timestamp}"
external_message_id="telegram-delivery-message-GITCLAW_CHANNEL_DELIVERY_EXTERNAL_${timestamp}"
gateway_run_id="channel-delivery-workflow-e2e-${timestamp}"
source_token="GITCLAW_CHANNEL_DELIVERY_SOURCE_${timestamp}"
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

url="$(jq -r '.url' <<<"$run_json")"
log "passed for state issue #${state_issue}: ${url}"
