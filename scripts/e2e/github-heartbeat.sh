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

repo="${GITCLAW_E2E_REPO:-}"
if [[ -z "$repo" ]]; then
  repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
fi

workflow_name="${GITCLAW_E2E_HEARTBEAT_WORKFLOW:-.github/workflows/gitclaw-heartbeat.yml}"
heartbeat_label="${GITCLAW_E2E_HEARTBEAT_LABEL:-gitclaw:heartbeat}"
retention_label="${GITCLAW_E2E_RETENTION_LABEL:-gitclaw:e2e}"
run_deadline_seconds="${GITCLAW_E2E_RUN_DEADLINE_SECONDS:-300}"
comment_deadline_seconds="${GITCLAW_E2E_COMMENT_DEADLINE_SECONDS:-180}"

gh auth status >/dev/null
gh repo view "$repo" >/dev/null
gh workflow view "$workflow_name" --repo "$repo" >/dev/null 2>&1 || die "repo is missing workflow: $workflow_name"

gh label create "$heartbeat_label" --repo "$repo" --color fbca04 --description "Wake GitClaw heartbeat" --force >/dev/null
gh label create "$retention_label" --repo "$repo" --color c2e0c6 --description "GitClaw E2E retention" --force >/dev/null
gh label create gitclaw:disabled --repo "$repo" --color 6a737d --description "Disable GitClaw on this issue" --force >/dev/null

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
slot="e2e-${timestamp}"
token="GITCLAW_HEARTBEAT_E2E_${timestamp}"
heartbeat_context_token="GITCLAW_HEARTBEAT_CONTEXT_V1"
title="GitClaw heartbeat e2e ${timestamp}"
body="Live heartbeat E2E.

When the heartbeat workflow runs, reply with exact token \`${token}\`.
Also include the exact heartbeat context token from \`.gitclaw/HEARTBEAT.md\`.
Keep it short."

issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label "$heartbeat_label")"
issue_number="${issue_url##*/}"

cleanup() {
  status=$?
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label "gitclaw:disabled" >/dev/null 2>&1 || true
    gh issue edit "$issue_number" --repo "$repo" --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" >/dev/null 2>&1 || true
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
      --repo "$repo" \
      --workflow "$workflow_name" \
      --event workflow_dispatch \
      --created ">=$started_at" \
      --json databaseId,status,conclusion,createdAt \
      --jq '.[0].databaseId' \
      | head -n 1)"
    if [[ -n "$run_id" ]]; then
      gh run watch "$run_id" --repo "$repo" --exit-status
      echo "$run_id"
      return 0
    fi
    sleep 5
  done
  return 1
}

heartbeat_comments() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:heartbeat")) | .body] | join("\n---HEARTBEAT-COMMENT---\n")'
}

heartbeat_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
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
  --repo "$repo" \
  -f label="$heartbeat_label" \
  -f slot="$slot" \
  -f limit=5

wait_for_dispatch_run "$dispatch_started_at" >/dev/null || die "timed out waiting for first heartbeat dispatch"
wait_for_heartbeat_count 1 || die "expected one heartbeat comment"
comments="$(heartbeat_comments)"
grep -Fq "$slot" <<<"$comments" || die "heartbeat comment missing slot ${slot}"
grep -Fq "$token" <<<"$comments" || die "heartbeat comment missing issue token ${token}"
grep -Fq "$heartbeat_context_token" <<<"$comments" || die "heartbeat comment missing context token ${heartbeat_context_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$comments" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$comments"; then
  die "heartbeat marker missing GitHub Models model id"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$comments" || die "heartbeat marker missing prompt context hash"
grep -Fq 'context_documents="' <<<"$comments" || die "heartbeat marker missing context document count"
grep -Fq 'selected_skills="' <<<"$comments" || die "heartbeat marker missing selected skill count"
grep -Fq 'tool_outputs="' <<<"$comments" || die "heartbeat marker missing tool output count"
grep -Fq 'usage_total_tokens="' <<<"$comments" || die "heartbeat marker missing token usage telemetry"
echo "heartbeat-e2e: first heartbeat verified"

dispatch_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow_name" \
  --repo "$repo" \
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
