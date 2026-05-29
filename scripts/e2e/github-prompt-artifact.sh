#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "prompt-artifact-e2e: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

need gh
need date
need mktemp

: "${GITCLAW_E2E_REPO:?set GITCLAW_E2E_REPO, e.g. owner/repo}"

workflow_name="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
retention_label="${GITCLAW_E2E_RETENTION_LABEL:-gitclaw:e2e}"
artifact_label="${GITCLAW_E2E_PROMPT_ARTIFACT_LABEL:-gitclaw:e2e-prompt-artifact}"
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
ensure_label "$artifact_label" 1d76db "Enable GitClaw prompt artifact E2E"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
public_token="GITCLAW_ARTIFACT_E2E_${timestamp}"
secret_token="GITCLAW_ARTIFACT_SECRET_${timestamp}"
title="@gitclaw prompt artifact e2e ${timestamp}"
body="Live prompt artifact E2E.

Reply with exact public token \`${public_token}\`.
The prompt artifact must redact this secret token: \`${secret_token}\`."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$GITCLAW_E2E_REPO" \
  --title "$title" \
  --body "$body" \
  --label gitclaw \
  --label "$artifact_label")"
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

echo "prompt-artifact-e2e: created issue #${issue_number}: ${issue_url}"

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
      gh run watch "$run_id" --repo "$GITCLAW_E2E_REPO" --exit-status >/dev/null
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

run_id="$(wait_for_run "$issue_started_at")" || die "timed out waiting for prompt artifact workflow"
comments="$(wait_for_assistant_comment)" || die "expected assistant comment"
grep -Fq "$public_token" <<<"$comments" || die "assistant comment missing public token ${public_token}"

tmpdir="$(mktemp -d)"
artifact_name="gitclaw-issue-${issue_number}-run-${run_id}-prompt"
gh run download "$run_id" \
  --repo "$GITCLAW_E2E_REPO" \
  --name "$artifact_name" \
  --dir "$tmpdir" >/dev/null

artifact_file="${tmpdir}/prompt.md"
[[ -f "$artifact_file" ]] || die "downloaded artifact missing prompt.md"
artifact_body="$(cat "$artifact_file")"

grep -Fq "GitClaw Prompt Artifact" <<<"$artifact_body" || die "artifact missing title"
grep -Fq "redaction: \`enabled\`" <<<"$artifact_body" || die "artifact missing redaction metadata"
grep -Fq "untrusted input" <<<"$artifact_body" || die "artifact missing untrusted input warning"
grep -Fq "$public_token" <<<"$artifact_body" || die "artifact missing public prompt token"
grep -Fq "[REDACTED]" <<<"$artifact_body" || die "artifact missing redaction marker"
if grep -Fq "$secret_token" <<<"$artifact_body"; then
  die "artifact leaked secret token"
fi

echo "prompt-artifact-e2e: redacted prompt artifact verified"
