#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "sandbox-report-e2e: $*" >&2
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
token="GITCLAW_SANDBOX_REPORT_E2E_${timestamp}"
title="@gitclaw /sandbox e2e ${timestamp}"
body="Live sandbox-report E2E.

Hidden sandbox report body token: ${token}
Show the deterministic execution-boundary report without a model call or raw body leakage."

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
      gh issue close "$issue_number" --repo "$repo" --comment "sandbox-report e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one sandbox report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/sandbox"' \
  "GitClaw Sandbox Report" \
  "Generated without a model call" \
  'repository: `'"$repo"'`' \
  'issue: `#'"$issue_number"'`' \
  'event_kind: `issue_opened`' \
  'event_name: `issues`' \
  'event_action: `opened`' \
  'active_command: `/sandbox`' \
  'sandbox_status: `locked_down`' \
  'runtime_boundary: `github-actions-ephemeral-runner`' \
  'sandbox_backend: `github-actions`' \
  'host_exec_policy: `deny`' \
  'shell_execution_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'write_mode: `read-only`' \
  'approval_mode: `not_applicable_no_exec_tool`' \
  'approval_store: `not_configured`' \
  'elevated_mode_available: `false`' \
  'skill_cli_auto_allow: `false`' \
  'inline_eval_policy: `not_applicable_no_exec_tool`' \
  'network_egress_policy: `github-actions-default`' \
  'available_tools: `5`' \
  'enabled_tools: `5`' \
  'disabled_tools: `0`' \
  'allowlist_blocked_tools: `0`' \
  'read_only_tool_contracts: `3`' \
  'metadata_only_tool_contracts: `2`' \
  'mutating_tool_contracts: `0`' \
  'active_tool_outputs: `' \
  'tool_validation_status: `ok`' \
  'workflow_permission_status: `ok`' \
  'workflow_present: `true`' \
  'unexpected_write_permissions: `0`' \
  'backup_write_permission_scope: `backup-job-only`' \
  'raw_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'raw_workflow_included: `false`' \
  "### Execution Boundary" \
  'shell_tool=`absent`' \
  'file_write_tool=`absent`' \
  'pull_request_tool=`absent`' \
  "### Tool Contracts" \
  'name=`gitclaw.list_files` mode=`read-only` mutating=`false` enabled=`true`' \
  'name=`gitclaw.policy` mode=`metadata-only` mutating=`false` enabled=`true`' \
  "### Active Tool Outputs" \
  'name=`gitclaw.list_files` input_sha256_12=`' \
  'output_sha256_12=`' \
  "### Workflow Permission Boundary" \
  'job=`handle` present=`true`' \
  'models:read' \
  'job=`backup` present=`true`' \
  'contents:write' \
  "### Sandbox Notes" \
  "host exec is denied because no shell/exec tool is exposed in GitClaw v1"; do
  grep -Fq -- "$expected" <<<"$comments" || die "sandbox report missing ${expected}"
done

for leaked in \
  "$token" \
  "Hidden sandbox report body token" \
  "Show the deterministic execution-boundary report" \
  "GitClaw is a repo-native GitHub issue assistant" \
  "GITCLAW_MEMORY_CONTEXT_V1"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "sandbox report leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
