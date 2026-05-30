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

ensure_label gitclaw:channel 1d76db "GitClaw mirrored channel thread"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
channel="telegram"
account_id="telegram-gateway-account-GITCLAW_CHANNEL_GATEWAY_E2E_${timestamp}"
gateway_slot="gateway-slot-${timestamp}"
lease_run_id="channel-gateway-workflow-e2e-${timestamp}"
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

url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
