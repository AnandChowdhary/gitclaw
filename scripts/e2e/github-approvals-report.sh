#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "approvals-report-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow_name="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
retention_label="${GITCLAW_E2E_RETENTION_LABEL:-gitclaw:e2e}"

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
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"
ensure_label gitclaw:write-requested d93f0b "GitClaw detected a write request"
ensure_label gitclaw:approved 0e8a16 "Maintainer approved GitClaw write-mode work"
ensure_label gitclaw:needs-human b60205 "GitClaw needs human approval or authorization"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
token="GITCLAW_APPROVALS_REPORT_E2E_${timestamp}"
title="@gitclaw /approvals e2e ${timestamp}"
body="Live approvals-report E2E.

Hidden approvals report body token: ${token}
Please implement this change and open a PR. The approval report must stay read-only and body-free."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label gitclaw \
  --label gitclaw:approved)"
issue_number="${issue_url##*/}"

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "approvals-report e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
}
trap cleanup EXIT

log "created issue #${issue_number}: ${issue_url}"

wait_for_run() {
  local started_at="$1"
  for _ in {1..90}; do
    local run_json
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$workflow_name" \
      --event issues \
      --created ">=$started_at" \
      --limit 10 \
      --json databaseId,status,conclusion,url,createdAt,displayTitle \
      --jq '. as $runs | $runs | map(select(.displayTitle == "'"${title}"'")) | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local run_status conclusion url
      run_status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$run_status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "issues run failed with conclusion ${conclusion}: ${url}"
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

error_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("<!-- gitclaw:error"))] | length'
}

issue_label_names() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json labels \
    --jq '.labels[].name'
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

wait_for_done_status() {
  for _ in {1..60}; do
    local labels
    labels="$(issue_label_names)"
    if grep -Fxq "gitclaw:done" <<<"$labels" &&
      ! grep -Fxq "gitclaw:running" <<<"$labels" &&
      ! grep -Fxq "gitclaw:error" <<<"$labels"; then
      return 0
    fi
    sleep 5
  done
  return 1
}

run_json="$(wait_for_run "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one approvals report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/approvals"' \
  "GitClaw Approvals Report" \
  "Generated without a model call" \
  'repository: `'"$repo"'`' \
  'issue: `#'"$issue_number"'`' \
  'event_kind: `issues`' \
  'preflight_allowed: `true`' \
  'preflight_code: `ok`' \
  'actor_association: `OWNER`' \
  'actor_trusted: `true`' \
  'triggered: `true`' \
  'disabled_label_present: `false`' \
  'write_request_detected: `true`' \
  'write_requested_label_present: `true`' \
  'approved_label_present: `true`' \
  'needs_human_label_present: `false`' \
  'transcript_messages: `1`' \
  'issue_title_sha256_12: `' \
  'approval_status: `approved_but_write_mode_disabled`' \
  'approval_decision: `proposal_only_approved_label_seen`' \
  'approval_store: `github-issue-labels`' \
  'approval_scope: `per-issue`' \
  'approval_label: `gitclaw:approved`' \
  'needs_human_label: `gitclaw:needs-human`' \
  'write_requested_label: `gitclaw:write-requested`' \
  'write_actions_enabled: `false`' \
  'run_mode: `read-only`' \
  'raw_bodies_included: `false`' \
  'raw_approval_payloads_included: `false`' \
  "### Approval Gates" \
  'gate=`trusted_actor` status=`passed`' \
  'gate=`write_request` status=`detected`' \
  'gate=`approval_label` status=`present`' \
  'gate=`write_mode` status=`blocked`' \
  "### Trusted Associations" \
  "### Approval Labels"; do
  grep -Fq -- "$expected" <<<"$comments" || die "approvals report missing ${expected}"
done

for leaked in \
  "$token" \
  "Hidden approvals report body token" \
  "Please implement this change and open a PR"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "approvals report leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
labels="$(issue_label_names)"
grep -Fxq "gitclaw:write-requested" <<<"$labels" || die "write-requested label missing"
grep -Fxq "gitclaw:approved" <<<"$labels" || die "approved label missing"

url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
