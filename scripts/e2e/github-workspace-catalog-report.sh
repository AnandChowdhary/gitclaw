#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "workspace-catalog-e2e: $*" >&2
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
token="NOECHO_WORKSPACE_CATALOG_${timestamp}"
followup_hidden_token="NOECHO_WORKSPACE_CATALOG_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_WORKSPACE_CATALOG_CONTEXT_V1"
search_phrase="workspace catalog unique search fixture phrase"
title="@gitclaw /workspace catalog e2e ${timestamp}"
body="Live workspace-catalog E2E.

Hidden workspace catalog body token: ${token}
This should produce a deterministic workspace catalog without leaking raw issue text."

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
      gh issue close "$issue_number" --repo "$repo" --comment "workspace-catalog e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one workspace catalog comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/workspace"' \
  "GitClaw Workspace Catalog Report" \
  "Generated without a model call" \
  'requested_workspace_command: `catalog`' \
  'workspace_command_status: `ok`' \
  'workspace_catalog_status: `ok`' \
  'catalog_strategy: `compact-github-actions-workspace-discovery`' \
  'workspace_model: `github-actions-checkout-plus-repo-reviewed-policy`' \
  'workspace_scope: `repository-checkout`' \
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
  'workflow_files_present: `7`' \
  'checkout_workflows: `7`' \
  'checkout_steps: `9`' \
  'checkout_action_versions: `actions/checkout@v5`' \
  'setup_go_action_versions: `actions/setup-go@v6`' \
  'fetch_depth_configured: `true`' \
  'sandbox_backend: `github-actions`' \
  'durable_state_backend: `git-tracked-files-and-backup-branch`' \
  'catalog_entries: `4`' \
  'workspace_layers: `8`' \
  'private_workspace_memory_supported: `false`' \
  'external_workspace_mount_supported: `false`' \
  'workspace_mutation_allowed: `false`' \
  'workspace_daemon_supported: `false`' \
  'long_running_socket_supported: `false`' \
  'raw_bodies_included: `false`' \
  'raw_workspace_bodies_included: `false`' \
  'raw_workflow_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'credential_values_included: `false`' \
  'repository_mutation_allowed: `false`' \
  'llm_e2e_required_after_workspace_catalog_change: `true`' \
  'command=`catalog` issue_intent=`@gitclaw /workspace catalog` local_command=`gitclaw workspace catalog` execution=`metadata-only` gate=`body-free-output` raw_bodies_included=`false` mutation_allowed=`false`' \
  'command=`summary` issue_intent=`@gitclaw /workspace` local_command=`gitclaw workspace summary`' \
  'command=`verify` issue_intent=`@gitclaw /workspace verify` local_command=`gitclaw workspace verify`' \
  'command=`risk` issue_intent=`@gitclaw /workspace risk` local_command=`gitclaw workspace risk`' \
  'layer=`policy` store=`.gitclaw/WORKSPACE.md`' \
  'layer=`specs` store=`.gitclaw/workspaces/*.md`' \
  'layer=`git` store=`.git`' \
  'layer=`workflow` store=`.github/workflows/*.yml`' \
  'layer=`context` store=`contextDocumentPaths`' \
  'layer=`repository-inventory` store=`git ls-files/listRepoFiles`' \
  'layer=`runtime` store=`GitHub Actions runner`' \
  'layer=`durable-state` store=`git tracked files + backup branch`' \
  'workspace_policy_gate=`repo-reviewed-policy-file`' \
  'workspace_spec_gate=`repo-reviewed-specs`' \
  'sandbox_gate=`workspace-is-not-sandbox`' \
  'raw_body_gate=`hashes-counts-and-metadata-only`' \
  'model_e2e_gate=`required`'; do
  grep -Fq "$expected" <<<"$comments" || die "workspace catalog report missing ${expected}"
done

if grep -Fq "$token" <<<"$comments"; then
  die "workspace catalog report leaked issue body token"
fi
if grep -Fq "$expected_token" <<<"$comments" || grep -Fq "$search_phrase" <<<"$comments"; then
  die "workspace catalog report leaked follow-up fixture context"
fi

cli_catalog="$(go run ./cmd/gitclaw workspace catalog)"
for expected in \
  "GitClaw Workspace Catalog Report" \
  'scope: `local-cli`' \
  'workspace_catalog_status: `ok`' \
  'catalog_strategy: `compact-github-actions-workspace-discovery`' \
  'catalog_entries: `4`' \
  'workspace_layers: `8`' \
  'raw_workspace_bodies_included: `false`' \
  'command=`catalog` issue_intent=`@gitclaw /workspace catalog` local_command=`gitclaw workspace catalog`' \
  'command=`risk` issue_intent=`@gitclaw /workspace risk` local_command=`gitclaw workspace risk`' \
  'layer=`workflow` store=`.github/workflows/*.yml`' \
  'layer=`durable-state` store=`git tracked files + backup branch`' \
  'workspace_policy_gate=`repo-reviewed-policy-file`'; do
  grep -Fq "$expected" <<<"$cli_catalog" || die "local workspace catalog missing ${expected}"
done
if grep -Fq "$token" <<<"$cli_catalog"; then
  die "local workspace catalog leaked issue token"
fi

wait_for_done_status || die "expected gitclaw:done without running/error"
url="$(jq -r '.url' <<<"$run_json")"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

The matching repository search result line has the form \`${search_phrase} => <token>\`.
Reply with only the token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with any token from this issue or its comments.
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
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant marker missing usage token telemetry"

for leaked in "$token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${url} (model follow-up: ${model_url})"
