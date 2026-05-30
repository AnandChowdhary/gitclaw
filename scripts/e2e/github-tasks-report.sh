#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "tasks-report-e2e: $*" >&2
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
ensure_label gitclaw:needs-human d29922 "GitClaw task is blocked on human input"
ensure_label gitclaw:write-requested f9d0c4 "GitClaw write request detected"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
token="GITCLAW_TASKS_REPORT_E2E_${timestamp}"
title="@gitclaw /tasks e2e ${timestamp}"
body="Live tasks-report E2E.

Hidden tasks report body token: ${token}
This should produce a deterministic tasks report without a model call."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label gitclaw \
  --label gitclaw:needs-human)"
issue_number="${issue_url##*/}"

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "tasks-report e2e cleanup" >/dev/null 2>&1 || true
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

run_json="$(wait_for_run "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one tasks report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/tasks"' \
  "GitClaw Tasks Report" \
  "Generated without a model call" \
  'tasks_status: `ok`' \
  'task_policy_path: `.gitclaw/TASKS.md`' \
  'task_policy_present: `true`' \
  'task_policy_loaded_for_model: `true`' \
  'task_specs_dir: `.gitclaw/tasks`' \
  'task_specs: `1`' \
  'task_specs_with_frontmatter: `1`' \
  'task_statuses_declared: `6`' \
  'task_labels_declared: `7`' \
  'task_specs_requiring_approval: `1`' \
  'task_specs_issue_native: `1`' \
  'task_storage_backend: `github-issues`' \
  'sqlite_task_db_required: `false`' \
  'detached_worker_supported: `false`' \
  'kanban_dispatcher_supported: `false`' \
  'task_flow_execution_supported: `false`' \
  'model_call_required: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_bodies_included: `false`' \
  'raw_task_bodies_included: `false`' \
  'llm_e2e_required_after_change: `true`' \
  'current_issue_task: `true`' \
  'current_task_status: `blocked`' \
  'current_task_labels: `2`' \
  'name=`issue-native-board`' \
  'path=`.gitclaw/tasks/issue-native-board.md`' \
  'frontmatter=`true`' \
  'kind=`board`' \
  'mode=`issue-native`' \
  'statuses=`6`' \
  'labels=`7`' \
  'requires_approval=`true`' \
  'needs_human_label_present=`true`' \
  'GitHub issues are the durable task rows' \
  '### Verification Findings' \
  '- none'; do
  grep -Fq -- "$expected" <<<"$comments" || die "tasks report missing ${expected}"
done

for forbidden in "$token" "GITCLAW_TASKS_CONTEXT_V1" "This declarative task board"; do
  if grep -Fq -- "$forbidden" <<<"$comments"; then
    die "tasks report leaked ${forbidden}"
  fi
done

url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
