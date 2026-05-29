#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "prompt-budget-e2e: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

need gh
need date

: "${GITCLAW_E2E_REPO:?set GITCLAW_E2E_REPO, e.g. owner/repo}"

workflow_name="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
retention_label="${GITCLAW_E2E_RETENTION_LABEL:-gitclaw:e2e}"
run_deadline_seconds="${GITCLAW_E2E_RUN_DEADLINE_SECONDS:-300}"
comment_deadline_seconds="${GITCLAW_E2E_COMMENT_DEADLINE_SECONDS:-180}"

gh auth status >/dev/null
gh repo view "$GITCLAW_E2E_REPO" >/dev/null
gh workflow view "$workflow_name" --repo "$GITCLAW_E2E_REPO" >/dev/null 2>&1 || die "repo is missing workflow: $workflow_name"

ensure_label() {
  local name="$1"
  local color="$2"
  local description="$3"
  gh label create "$name" --repo "$GITCLAW_E2E_REPO" --color "$color" --description "$description" --force >/dev/null
}

ensure_label gitclaw 0e8a16 "Handled by GitClaw"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:disabled 5319e7 "GitClaw should ignore this issue"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
token="GITCLAW_PROMPT_BUDGET_E2E_${timestamp}"
title="@gitclaw prompt budget e2e ${timestamp}"
noise=""
for _ in $(seq 1 3600); do
  noise+="budget-noise "
done
body="Live prompt budget E2E.

The body intentionally contains a large amount of unimportant text before the actual request.

${noise}

The exact prompt budget token is \`${token}\`.
Reply with that exact token and keep the answer under 30 words."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$GITCLAW_E2E_REPO" \
  --title "$title" \
  --body "$body" \
  --label gitclaw)"
issue_number="${issue_url##*/}"

cleanup() {
  status=$?
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$GITCLAW_E2E_REPO" --add-label "gitclaw:disabled" >/dev/null 2>&1 || true
    gh issue edit "$issue_number" --repo "$GITCLAW_E2E_REPO" --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$GITCLAW_E2E_REPO" >/dev/null 2>&1 || true
    fi
  fi
  exit "$status"
}
trap cleanup EXIT

echo "prompt-budget-e2e: created issue #${issue_number}: ${issue_url}"

wait_for_run() {
  local started_at="$1"
  local deadline=$((SECONDS + run_deadline_seconds))
  while (( SECONDS < deadline )); do
    local run_id
    run_id="$(gh run list \
      --repo "$GITCLAW_E2E_REPO" \
      --workflow "$workflow_name" \
      --event issues \
      --created ">=$started_at" \
      --json databaseId,displayTitle,status,conclusion,createdAt \
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

assistant_comments() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | join("\n---GITCLAW-COMMENT---\n")'
}

wait_for_assistant_comment() {
  local deadline=$((SECONDS + comment_deadline_seconds))
  while (( SECONDS < deadline )); do
    local comments
    comments="$(assistant_comments)"
    if [[ -n "$comments" ]]; then
      echo "$comments"
      return 0
    fi
    sleep 5
  done
  return 1
}

issue_labels() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json labels \
    --jq '.labels[].name'
}

wait_for_run "$issue_started_at" >/dev/null || die "timed out waiting for prompt budget workflow"
comments="$(wait_for_assistant_comment)" || die "expected assistant comment"
grep -Fq "$token" <<<"$comments" || die "assistant comment missing prompt budget tail token ${token}"

labels="$(issue_labels)"
grep -Fxq "gitclaw:done" <<<"$labels" || die "issue missing gitclaw:done"
if grep -Fxq "gitclaw:running" <<<"$labels" || grep -Fxq "gitclaw:error" <<<"$labels"; then
  die "prompt budget issue has incorrect final status labels: ${labels}"
fi

echo "prompt-budget-e2e: bounded prompt response verified"
