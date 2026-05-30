#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "policy-verify-report-e2e: $*" >&2
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
ensure_label gitclaw:write-requested d4c5f9 "User asked GitClaw for write-capable work"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
token="GITCLAW_POLICY_VERIFY_E2E_${timestamp}"
title="@gitclaw /policy verify e2e ${timestamp}"
body="Live policy-verify E2E.

Please implement a small policy verify fixture and open a PR.
Hidden policy verify token: ${token}
This should produce a deterministic policy verify report without calling a model."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label gitclaw)"
issue_number="${issue_url##*/}"

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "policy-verify-report e2e cleanup" >/dev/null 2>&1 || true
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
      die "assistant run posted ${errors} error comment(s)"
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
wait_for_assistant_count 1 || die "expected one policy verify report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/policy"' \
  "GitClaw Policy Verify Report" \
  "Generated without a model call" \
  'policy_verify_status: `ok`' \
  'verification_scope: `workflow-permissions-and-policy-surface`' \
  'workflow_path: `.github/workflows/gitclaw.yml`' \
  'workflow_present: `true`' \
  'workflow_sha256_12:' \
  'expected_jobs: `3`' \
  'jobs_present: `3`' \
  'expected_permissions: `7`' \
  'permissions_present: `7`' \
  'missing_permissions: `0`' \
  'unexpected_write_permissions: `0`' \
  'backup_concurrency_group: `true`' \
  'backup_concurrency_cancel_safe: `true`' \
  'policy_outputs_hashed: `1`' \
  'raw_bodies_included: `false`' \
  'raw_inputs_included: `false`' \
  "### Workflow Permission Cards" \
  'job=`preflight` present=`true`' \
  'job=`handle` present=`true`' \
  'job=`backup` present=`true`' \
  'expected=`contents:read, issues:write, models:read`' \
  'missing=`none`' \
  'unexpected_write=`none`' \
  "### Active Policy Output Trust Cards" \
  'name=`gitclaw.policy` input_sha256_12=' \
  'output_sha256_12=' \
  "### Verification Findings" \
  "- none"; do
  grep -Fq -- "$expected" <<<"$comments" || die "policy verify report missing ${expected}"
done

for leaked in "$token" "$title" "Please implement a small policy verify fixture" 'input=`write-request`' "Current GitClaw mode is read-only"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "policy verify report leaked ${leaked}"
  fi
done

labels="$(issue_label_names)"
grep -Fxq "gitclaw:write-requested" <<<"$labels" || die "issue missing gitclaw:write-requested label"
wait_for_done_status || die "expected gitclaw:done without running/error"
url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
