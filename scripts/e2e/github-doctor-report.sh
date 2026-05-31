#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "doctor-report-e2e: $*" >&2
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
token="GITCLAW_DOCTOR_REPORT_E2E_${timestamp}"
followup_hidden_token="GITCLAW_DOCTOR_REPORT_FOLLOWUP_E2E_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw /doctor e2e ${timestamp}"
body="Live doctor-report E2E.

Hidden doctor body token: ${token}
This should produce a deterministic health report without a model call."

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
      gh issue close "$issue_number" --repo "$repo" --comment "doctor-report e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
}
trap cleanup EXIT

log "created issue #${issue_number}: ${issue_url}"

wait_for_run() {
  local event_name="$1"
  local started_at="$2"
  for _ in {1..90}; do
    local run_json
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$workflow_name" \
      --event "$event_name" \
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
        [[ "$conclusion" == "success" ]] || die "${event_name} run failed with conclusion ${conclusion}: ${url}"
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

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
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

run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one doctor report comment"
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
  'e2e_scripts: `173`' \
  'e2e_live_issue_scripts: `166`' \
  'e2e_cleanup_scripts: `173`' \
  'e2e_model_coverage_scripts: `104`' \
  'e2e_model_followup_scripts: `91`' \
  'e2e_session_coverage_scripts: `2`' \
  'e2e_backup_gate_scripts: `24`' \
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
  'skill_validation_errors: `0`' \
  'skill_validation_warnings: `0`' \
  'soul_validation_status: `ok`' \
  'soul_validation_errors: `0`' \
  'soul_validation_warnings: `0`' \
  'memory_validation_status: `ok`' \
  'memory_validation_errors: `0`' \
  'memory_validation_warnings: `0`' \
  'tool_validation_status: `ok`' \
  'tool_validation_errors: `0`' \
  'tool_validation_warnings: `0`' \
  '`config_validation`: `ok`' \
  '`workflow_set`: `ok`' \
  '`identity_context`: `ok`' \
  '`local_skills`: `ok`' \
  '`e2e_harnesses`: `ok`' \
  '`skill_validation`: `ok`' \
  '`soul_validation`: `ok`' \
  '`memory_validation`: `ok`' \
  '`tool_validation`: `ok`' \
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
  grep -Fq "$expected" <<<"$comments" || die "doctor report missing ${expected}"
done

if grep -Fq "$token" <<<"$comments"; then
  die "doctor report leaked issue body token"
fi

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_SEARCH token from the matching repository search result line.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run issue_comment "$comment_started_at")" || die "timed out waiting for issue_comment workflow run"
wait_for_assistant_count 2 || die "expected model-backed follow-up assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant marker did not prove search_files was prompt-visible"

for leaked in "$token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
url="$(jq -r '.url' <<<"$run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${url} (model follow-up: ${model_url})"
