#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "doctor-list-report-e2e: $*" >&2
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
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
token="GITCLAW_DOCTOR_LIST_REPORT_E2E_${timestamp}"
title="@gitclaw /doctor list e2e ${timestamp}"
body="Live doctor-list-report E2E.

Hidden doctor list body token: ${token}
This should produce a deterministic health report through the explicit list alias without a model call."

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
      gh issue close "$issue_number" --repo "$repo" --comment "doctor-list-report e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one doctor list report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/doctor"' \
  "GitClaw Doctor Report" \
  "Generated without a model call" \
  'health_status: `ok`' \
  'config_source: `defaults+repo+environment`' \
  'config_valid: `true`' \
  'config_file_present: `true`' \
  'model: `openai/gpt-5-nano`' \
  'run_mode: `read-only`' \
  'workflows_present: `7`' \
  'context_files_present: `6`' \
  'memory_notes: `1`' \
  'skill_files: `1`' \
  'e2e_scripts: `156`' \
  'e2e_live_issue_scripts: `149`' \
  'e2e_cleanup_scripts: `156`' \
  'e2e_model_coverage_scripts: `67`' \
  'e2e_model_followup_scripts: `53`' \
  'e2e_session_coverage_scripts: `2`' \
  'e2e_backup_gate_scripts: `23`' \
  'e2e_workflow_dispatch_scripts: `21`' \
  'enabled_skills: `1`' \
  'disabled_skills: `0`' \
  'allowlist_blocked_skills: `0`' \
  'enabled_tools: `5`' \
  'disabled_tools: `0`' \
  'allowlist_blocked_tools: `0`' \
  'proactive_prompt_files: `1`' \
  'managed_labels: `9`' \
  'validation_errors: `0`' \
  'validation_warnings: `0`' \
  'skill_validation_status: `ok`' \
  'soul_validation_status: `ok`' \
  'memory_validation_status: `ok`' \
  'tool_validation_status: `ok`' \
  '`config_validation`: `ok`' \
  '`workflow_set`: `ok`' \
  '`identity_context`: `ok`' \
  '`local_skills`: `ok`' \
  '`e2e_harnesses`: `ok`' \
  '`proactive_prompt`: `ok`' \
  '.gitclaw/config.yml' \
  '.github/workflows/gitclaw.yml' \
  '.gitclaw/SOUL.md' \
  '.gitclaw/SKILLS/repo-reader/SKILL.md' \
  '.gitclaw/proactive/repo-hygiene.md' \
  "### E2E Harnesses" \
  'e2e_coverage_status=`ok`' \
  'path=`scripts/e2e/github-doctor-report.sh`' \
  'model_coverage=`true`' \
  'model_followup=`true`' \
  'sha256_12='; do
  grep -Fq "$expected" <<<"$comments" || die "doctor list report missing ${expected}"
done

if grep -Fq "$token" <<<"$comments"; then
  die "doctor list report leaked issue body token"
fi

wait_for_done_status || die "expected gitclaw:done without running/error"
url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
