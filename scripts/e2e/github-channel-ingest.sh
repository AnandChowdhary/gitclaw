#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "channel-ingest-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"

if [[ -z "$repo" ]]; then
  repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
fi

ensure_label() {
  local name="$1"
  local color="$2"
  local description="$3"
  gh label create "$name" --repo "$repo" --color "$color" --description "$description" --force >/dev/null
}

ensure_label gitclaw 5319e7 "Handled by GitClaw"
ensure_label gitclaw:channel 1d76db "GitClaw mirrored channel thread"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
channel="telegram"
thread_id="channel-ingest-e2e-${timestamp}"
message_id="message-${timestamp}"
dispatch_id="${channel}-${message_id}"
token="GITCLAW_CHANNEL_INGEST_E2E_${timestamp}"
body="Mirrored Telegram ingest message.

When this channel ingest is dispatched, reply with exact token \`${token}\`.
Also include the exact word \`ingest\`."

run_list_json() {
  local workflow="$1"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event workflow_dispatch \
    --limit 10 \
    --json databaseId,status,conclusion,createdAt,url \
    --jq '.'
}

wait_for_run() {
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

assistant_comments() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | join("\n---GITCLAW-COMMENT---\n")'
}

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled >/dev/null 2>&1 || true
    gh issue close "$issue_number" --repo "$repo" --comment "channel ingest e2e cleanup" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

ingest_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$ingest_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f thread_id="$thread_id" \
  -f message_id="$message_id" \
  -f author="telegram:e2e" \
  -f body="$body"

wait_for_run "$ingest_workflow" "$ingest_started_at" >/dev/null || die "timed out waiting for channel ingest workflow"
issue_number="$(wait_for_issue_number)" || die "timed out finding channel issue for ${thread_id}"
log "channel ingest created issue #${issue_number}"

wait_for_run "$main_workflow" "$ingest_started_at" >/dev/null || die "timed out waiting for dispatched main workflow"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,labels,comments)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "issue body missing channel-thread marker"
grep -Fq "gitclaw:channel-message" <<<"$(jq -r '[.comments[].body] | join("\n")' <<<"$issue_json")" || die "comments missing channel-message marker"
comments="$(assistant_comments)"
grep -Fq "$token" <<<"$comments" || die "assistant comment missing token ${token}"
grep -Fiq "ingest" <<<"$comments" || die "assistant comment missing word ingest"
grep -Fq "dispatch-${dispatch_id}" <<<"$comments" || die "assistant marker missing dispatch event id"
labels="$(jq -r '.labels[].name' <<<"$issue_json")"
grep -Fxq "gitclaw:channel" <<<"$labels" || die "issue missing gitclaw:channel label"
grep -Fxq "gitclaw:done" <<<"$labels" || die "issue missing gitclaw:done label"

log "passed for issue #${issue_number}"
