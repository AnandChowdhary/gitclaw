#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "failure-e2e: $*" >&2
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
invalid_model_label="${GITCLAW_E2E_INVALID_MODEL_LABEL:-gitclaw:e2e-invalid-model}"
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
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:disabled 5319e7 "GitClaw should ignore this issue"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"
ensure_label "$invalid_model_label" d93f0b "Force invalid GitClaw model for E2E"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
secret_token="GITCLAW_FAILURE_SECRET_${timestamp}"
title="@gitclaw failure e2e ${timestamp}"
body="Live failure E2E.

This issue intentionally uses the invalid-model E2E label.
Do not leak this token in any failure diagnostic: \`${secret_token}\`."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$GITCLAW_E2E_REPO" \
  --title "$title" \
  --body "$body" \
  --label gitclaw \
  --label "$invalid_model_label")"
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

echo "failure-e2e: created issue #${issue_number}: ${issue_url}"

wait_for_failure_run() {
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
      gh run watch "$run_id" --repo "$GITCLAW_E2E_REPO" --exit-status >/dev/null 2>&1 || true
      local conclusion
      conclusion="$(gh run view "$run_id" --repo "$GITCLAW_E2E_REPO" --json conclusion --jq '.conclusion')"
      [[ "$conclusion" == "failure" ]] || die "expected failure run, got conclusion=${conclusion}"
      echo "$run_id"
      return 0
    fi
    sleep 5
  done
  return 1
}

error_comments() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:error")) | .body] | join("\n---GITCLAW-ERROR---\n")'
}

assistant_comment_count() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

issue_label_names() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json labels \
    --jq '.labels[].name'
}

wait_for_error_comment() {
  local deadline=$((SECONDS + comment_deadline_seconds))
  while (( SECONDS < deadline )); do
    local comments
    comments="$(error_comments)"
    if [[ -n "$comments" ]]; then
      echo "$comments"
      return 0
    fi
    sleep 5
  done
  return 1
}

wait_for_failure_run "$issue_started_at" >/dev/null || die "timed out waiting for failing issues workflow"
comments="$(wait_for_error_comment)" || die "expected safe error comment"
grep -Fq "model provider request failed" <<<"$comments" || die "error comment missing safe diagnostic"
if grep -Fq "$secret_token" <<<"$comments"; then
  die "error comment leaked secret issue token"
fi
if [[ "$(assistant_comment_count)" != "0" ]]; then
  die "failure run should not post a completed assistant-turn comment"
fi

labels="$(issue_label_names)"
grep -Fxq "gitclaw:error" <<<"$labels" || die "issue missing gitclaw:error label"
if grep -Fxq "gitclaw:running" <<<"$labels" || grep -Fxq "gitclaw:done" <<<"$labels"; then
  die "failure issue has incorrect final status labels: ${labels}"
fi

echo "failure-e2e: safe failure comment and labels verified"
