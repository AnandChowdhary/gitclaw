#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "workspace-report-e2e: $*" >&2
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
token="GITCLAW_WORKSPACE_REPORT_E2E_${timestamp}"
title="@gitclaw /workspace e2e ${timestamp}"
body="Live workspace-report E2E.

Hidden workspace report body token: ${token}
This should produce a deterministic workspace report without a model call."

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
      gh issue close "$issue_number" --repo "$repo" --comment "workspace-report e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one workspace report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/workspace"' \
  "GitClaw Workspace Report" \
  "Generated without a model call" \
  'workspace_status: `ok`' \
  'workspace_policy_path: `.gitclaw/WORKSPACE.md`' \
  'workspace_policy_present: `true`' \
  'workspace_policy_loaded_for_model: `true`' \
  'workspace_specs_dir: `.gitclaw/workspaces`' \
  'workspace_specs: `1`' \
  'workspace_specs_with_frontmatter: `1`' \
  'workspace_specs_requiring_approval: `1`' \
  'git_available: `true`' \
  'git_repository: `true`' \
  'worktree_root: `.`' \
  'repo_file_list_limit: `240`' \
  'context_documents_loaded:' \
  'context_allowlist_entries: `18`' \
  'workspace_context_policy_loaded: `true`' \
  'workflow_files_present: `7`' \
  'checkout_workflows: `7`' \
  'checkout_steps: `9`' \
  'checkout_action_versions: `actions/checkout@v5`' \
  'setup_go_action_versions: `actions/setup-go@v6`' \
  'fetch_depth_configured: `true`' \
  'sandbox_backend: `github-actions`' \
  'durable_state_backend: `git-tracked-files-and-backup-branch`' \
  'private_workspace_memory_supported: `false`' \
  'external_workspace_mount_supported: `false`' \
  'workspace_mutation_allowed: `false`' \
  'raw_bodies_included: `false`' \
  'raw_file_bodies_included: `false`' \
  'model_call_required: `false`' \
  'repository_mutation_allowed: `false`' \
  'llm_e2e_required_after_change: `true`' \
  'name=`repository-checkout`' \
  'path=`.gitclaw/workspaces/repository.md`' \
  'frontmatter=`true`' \
  'kind=`git-workspace`' \
  'runtime=`github-actions`' \
  'storage=`repository-checkout`' \
  'mode=`metadata-only`' \
  'root=`.`' \
  'isolation=`ephemeral-actions-runner`' \
  'durable_state=`git-tracked-files-and-backup-branch`' \
  'requires_approval=`true`' \
  '### Workflow Workspace Setup' \
  '### Repository Inventory' \
  'raw_paths_included=`false`' \
  'raw_context_bodies_included=`false`' \
  '`/workspace` is inspect-only' \
  'future private workspace memory or external mounts require reviewed specs' \
  '### Verification Findings' \
  '- none'; do
  grep -Fq -- "$expected" <<<"$comments" || die "workspace report missing ${expected}"
done

for forbidden in "$token" "GITCLAW_WORKSPACE_CONTEXT_V1" "This declarative workspace record"; do
  if grep -Fq -- "$forbidden" <<<"$comments"; then
    die "workspace report leaked ${forbidden}"
  fi
done

url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
