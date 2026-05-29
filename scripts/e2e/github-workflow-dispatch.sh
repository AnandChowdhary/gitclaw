#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "workflow-dispatch-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow_name="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"

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
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
token="GITCLAW_WORKFLOW_DISPATCH_E2E_${timestamp}"
dispatch_id="workflow-dispatch-e2e-${timestamp}"
title="GitClaw workflow_dispatch e2e ${timestamp}"
body="Live workflow_dispatch E2E.

When GitClaw is manually dispatched for this issue, reply with exact token \`${token}\`.
Also include the exact word \`workflow_dispatch\`.
"

issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body")"
issue_number="${issue_url##*/}"

cleanup() {
  gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled >/dev/null 2>&1 || true
  gh issue close "$issue_number" --repo "$repo" --comment "workflow_dispatch e2e cleanup" >/dev/null 2>&1 || true
}
trap cleanup EXIT

gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw --add-label gitclaw:e2e >/dev/null
log "created issue #${issue_number}: ${issue_url}"

run_list_json() {
  gh run list \
    --repo "$repo" \
    --workflow "$workflow_name" \
    --event workflow_dispatch \
    --limit 10 \
    --json databaseId,status,conclusion,createdAt,url \
    --jq '.'
}

wait_for_dispatch_run() {
  local started_at="$1"
  local run_json
  for _ in {1..90}; do
    run_json="$(run_list_json | jq -c --arg started "$started_at" '[.[] | select(.createdAt >= $started)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "dispatch run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

assistant_comments() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | join("\n---GITCLAW-COMMENT---\n")'
}

assistant_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

wait_for_assistant_count() {
  local want="$1"
  for _ in {1..90}; do
    local got
    got="$(assistant_count)"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

dispatch_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow_name" \
  --repo "$repo" \
  -f issue_number="$issue_number" \
  -f dispatch_id="$dispatch_id" \
  -f reason="e2e"

wait_for_dispatch_run "$dispatch_started_at" >/dev/null || die "timed out waiting for first workflow_dispatch run"
wait_for_assistant_count 1 || die "expected one assistant comment after first dispatch"
comments="$(assistant_comments)"
grep -Fq "$token" <<<"$comments" || die "assistant comment missing token ${token}"
grep -Fq "workflow_dispatch" <<<"$comments" || die "assistant comment missing workflow_dispatch"
grep -Fq "dispatch-${dispatch_id}" <<<"$comments" || die "assistant marker missing dispatch event id"
log "first workflow_dispatch verified"

dispatch_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow_name" \
  --repo "$repo" \
  -f issue_number="$issue_number" \
  -f dispatch_id="$dispatch_id" \
  -f reason="e2e-idempotency"

wait_for_dispatch_run "$dispatch_started_at" >/dev/null || die "timed out waiting for duplicate workflow_dispatch run"
final_count="$(assistant_count)"
if [[ "$final_count" != "1" ]]; then
  die "workflow_dispatch idempotency failed: expected 1 assistant comment, got ${final_count}"
fi

log "idempotency verified"
log "passed for issue #${issue_number}"
