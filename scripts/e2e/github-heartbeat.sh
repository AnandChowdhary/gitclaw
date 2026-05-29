#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "heartbeat-e2e: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

need gh
need date

: "${GITCLAW_E2E_REPO:?set GITCLAW_E2E_REPO, e.g. owner/repo}"

workflow_name="${GITCLAW_E2E_HEARTBEAT_WORKFLOW:-.github/workflows/gitclaw-heartbeat.yml}"
heartbeat_label="${GITCLAW_E2E_HEARTBEAT_LABEL:-gitclaw:heartbeat}"
retention_label="${GITCLAW_E2E_RETENTION_LABEL:-gitclaw:e2e}"
run_deadline_seconds="${GITCLAW_E2E_RUN_DEADLINE_SECONDS:-300}"
comment_deadline_seconds="${GITCLAW_E2E_COMMENT_DEADLINE_SECONDS:-180}"

gh auth status >/dev/null
gh repo view "$GITCLAW_E2E_REPO" >/dev/null
gh workflow view "$workflow_name" --repo "$GITCLAW_E2E_REPO" >/dev/null 2>&1 || die "repo is missing workflow: $workflow_name"

for label in gitclaw "$heartbeat_label"; do
  if ! gh label list --repo "$GITCLAW_E2E_REPO" --limit 1000 --json name --jq '.[].name' | grep -Fxq "$label"; then
    die "repo is missing required label: $label"
  fi
done

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
slot="e2e-${timestamp}"
token="GITCLAW_HEARTBEAT_E2E_${timestamp}"
heartbeat_context_token="GITCLAW_HEARTBEAT_CONTEXT_V1"
title="@gitclaw heartbeat e2e ${timestamp}"
body="Live heartbeat E2E.

When the heartbeat workflow runs, reply with exact token \`${token}\`.
Also include the exact heartbeat context token from \`.gitclaw/HEARTBEAT.md\`.
Keep it short."

issue_url="$(gh issue create \
  --repo "$GITCLAW_E2E_REPO" \
  --title "$title" \
  --body "$body" \
  --label gitclaw \
  --label "$heartbeat_label")"
issue_number="${issue_url##*/}"

cleanup() {
  status=$?
  if [[ -n "${issue_number:-}" ]]; then
    if gh label list --repo "$GITCLAW_E2E_REPO" --limit 1000 --json name --jq '.[].name' | grep -Fxq "gitclaw:disabled"; then
      gh issue edit "$issue_number" --repo "$GITCLAW_E2E_REPO" --add-label "gitclaw:disabled" >/dev/null 2>&1 || true
    fi
    if gh label list --repo "$GITCLAW_E2E_REPO" --limit 1000 --json name --jq '.[].name' | grep -Fxq "$retention_label"; then
      gh issue edit "$issue_number" --repo "$GITCLAW_E2E_REPO" --add-label "$retention_label" >/dev/null 2>&1 || true
    fi
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$GITCLAW_E2E_REPO" >/dev/null 2>&1 || true
    fi
  fi
  exit "$status"
}
trap cleanup EXIT

echo "heartbeat-e2e: created issue #${issue_number}: ${issue_url}"

wait_for_dispatch_run() {
  local started_at="$1"
  local deadline=$((SECONDS + run_deadline_seconds))
  while (( SECONDS < deadline )); do
    local run_id
    run_id="$(gh run list \
      --repo "$GITCLAW_E2E_REPO" \
      --workflow "$workflow_name" \
      --event workflow_dispatch \
      --created ">=$started_at" \
      --json databaseId,status,conclusion,createdAt \
      --jq '.[0].databaseId' \
      | head -n 1)"
    if [[ -n "$run_id" ]]; then
      gh run watch "$run_id" --repo "$GITCLAW_E2E_REPO" --exit-status
      echo "$run_id"
      return 0
    fi
    sleep 5
  done
  return 1
}

heartbeat_comments() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:heartbeat")) | .body] | join("\n---HEARTBEAT-COMMENT---\n")'
}

heartbeat_count() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:heartbeat"))] | length'
}

wait_for_heartbeat_count() {
  local want="$1"
  local deadline=$((SECONDS + comment_deadline_seconds))
  while (( SECONDS < deadline )); do
    local got
    got="$(heartbeat_count)"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

dispatch_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow_name" \
  --repo "$GITCLAW_E2E_REPO" \
  -f label="$heartbeat_label" \
  -f slot="$slot" \
  -f limit=5

wait_for_dispatch_run "$dispatch_started_at" >/dev/null || die "timed out waiting for first heartbeat dispatch"
wait_for_heartbeat_count 1 || die "expected one heartbeat comment"
comments="$(heartbeat_comments)"
grep -Fq "$slot" <<<"$comments" || die "heartbeat comment missing slot ${slot}"
grep -Fq "$token" <<<"$comments" || die "heartbeat comment missing issue token ${token}"
grep -Fq "$heartbeat_context_token" <<<"$comments" || die "heartbeat comment missing context token ${heartbeat_context_token}"
echo "heartbeat-e2e: first heartbeat verified"

dispatch_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow_name" \
  --repo "$GITCLAW_E2E_REPO" \
  -f label="$heartbeat_label" \
  -f slot="$slot" \
  -f limit=5

wait_for_dispatch_run "$dispatch_started_at" >/dev/null || die "timed out waiting for second heartbeat dispatch"
sleep 10
final_count="$(heartbeat_count)"
if [[ "$final_count" != "1" ]]; then
  die "heartbeat idempotency failed: expected 1 heartbeat comment, got ${final_count}"
fi
echo "heartbeat-e2e: idempotency verified"
