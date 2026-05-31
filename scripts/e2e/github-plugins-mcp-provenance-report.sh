#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "plugins-mcp-provenance-report-e2e: $*" >&2
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
hidden_token="NOECHO_MCP_PROVENANCE_SENTINEL_${timestamp}"
followup_hidden_token="NOECHO_MCP_PROVENANCE_FOLLOWUP_SENTINEL_${timestamp}"
expected_token="GITCLAW_MCP_PROVENANCE_CONTEXT_V1"
search_phrase="mcp provenance unique search fixture phrase"
title="@gitclaw /plugins mcp provenance e2e ${timestamp}"
body="@gitclaw /plugins mcp provenance

Live MCP provenance E2E. Please keep the provenance report body-free.
Do not include this hidden MCP provenance sentinel: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw plugins mcp provenance)"
for expected in \
  "GitClaw MCP Provenance Report" \
  'scope: `local-cli`' \
  'mcp_provenance_status: `ok`' \
  'provenance_scope: `repo-local-mcp-git-history`' \
  'mcp_status: `ok`' \
  'mcp_specs_dir: `.gitclaw/mcp`' \
  'mcp_specs: `1`' \
  'parsed_mcp_specs: `1`' \
  'mcp_specs_with_command: `0`' \
  'mcp_specs_with_url: `0`' \
  'mcp_specs_with_tool_allowlist: `1`' \
  'mcp_tool_allowlist_refs: `3`' \
  'mcp_tool_denylist_refs: `2`' \
  'mcp_required_secret_refs: `1`' \
  'mcp_env_passthrough_refs: `0`' \
  'mcp_specs_with_resources_enabled: `0`' \
  'mcp_specs_with_prompts_enabled: `0`' \
  'mcp_specs_with_risk_findings: `0`' \
  'mcp_risk_findings: `0`' \
  'high_risk_findings: `0`' \
  'warning_risk_findings: `0`' \
  'info_risk_findings: `0`' \
  'git_tracked_mcp_specs: `1`' \
  'untracked_mcp_specs: `0`' \
  'working_tree_dirty_mcp_specs: `0`' \
  'mcp_specs_with_commits: `1`' \
  'mcp_specs_without_commits: `0`' \
  'git_available: `true`' \
  'git_history_available: `true`' \
  'mcp_connection_supported: `false`' \
  'mcp_server_launch_allowed: `false`' \
  'mcp_tool_exposure_allowed: `false`' \
  'dynamic_tool_discovery_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_mcp_bodies_included: `false`' \
  'raw_command_args_included: `false`' \
  'raw_urls_included: `false`' \
  'raw_git_subjects_included: `false`' \
  'author_identities_included: `false`' \
  'credential_values_included: `false`' \
  'env_values_included: `false`' \
  'llm_e2e_required_after_mcp_provenance_change: `true`' \
  'mcp_name=`github-read` path=`.gitclaw/mcp/github-read.yaml`' \
  'transport=`stdio`' \
  'activation=`metadata-only`' \
  'description=`true`' \
  'source_present=`true`' \
  'source_sha256_12=' \
  'command_present=`false`' \
  'command_sha256_12=`none`' \
  'args_count=`0`' \
  'args_sha256_12=`none`' \
  'url_present=`false`' \
  'url_sha256_12=`none`' \
  'tool_allowlist=`contents.read, issues.read, pull_requests.read`' \
  'tool_denylist=`actions.write, contents.write`' \
  'requires_secret_refs=`1`' \
  'requires_secrets_sha256_12=' \
  'env_passthrough_refs=`0`' \
  'env_passthrough_sha256_12=`none`' \
  'resources_enabled=`false`' \
  'prompts_enabled=`false`' \
  'parse_error=`false`' \
  'parse_error_sha256_12=`none`' \
  'risk_findings=`0`' \
  'risk_max_severity=`none`' \
  'risk_codes=`none`' \
  'git_tracked=`true`' \
  'working_tree_dirty=`false`' \
  'commit_available=`true`' \
  'last_commit_sha256_12=' \
  'last_commit_short=' \
  'last_commit_date=' \
  'subject_sha256_12=' \
  '### Provenance Gates' \
  'risk_gate=`pass`' \
  'git_history_gate=`pass`' \
  'connection_gate=`disabled`' \
  'server_launch_gate=`disabled`' \
  'tool_exposure_gate=`disabled`' \
  'dynamic_discovery_gate=`disabled`' \
  'mutation_gate=`disabled`' \
  'raw_body_gate=`hash_only`' \
  'raw_command_args_gate=`hash_only`' \
  'raw_url_gate=`hash_only`' \
  'credential_value_gate=`disabled`' \
  'env_value_gate=`disabled`' \
  '### Findings' \
  "- none"; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local MCP provenance report missing ${expected}"
done

for leaked in "Metadata-only placeholder" "github-mcp-read" "GITHUB_TOKEN" "$expected_token" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$local_report"; then
    die "local MCP provenance report leaked ${leaked}"
  fi
done

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
      gh issue close "$issue_number" --repo "$repo" --comment "plugins-mcp-provenance-report e2e cleanup" >/dev/null 2>&1 || true
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
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${event_name} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

assistant_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
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

mcp_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one MCP provenance report comment"
mcp_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/plugins"' \
  "GitClaw MCP Provenance Report" \
  "Generated without a model call" \
  'mcp_provenance_status: `ok`' \
  'provenance_scope: `repo-local-mcp-git-history`' \
  'mcp_status: `ok`' \
  'mcp_specs_dir: `.gitclaw/mcp`' \
  'mcp_specs: `1`' \
  'parsed_mcp_specs: `1`' \
  'mcp_specs_with_command: `0`' \
  'mcp_specs_with_url: `0`' \
  'mcp_specs_with_tool_allowlist: `1`' \
  'mcp_tool_allowlist_refs: `3`' \
  'mcp_tool_denylist_refs: `2`' \
  'mcp_required_secret_refs: `1`' \
  'mcp_env_passthrough_refs: `0`' \
  'mcp_specs_with_resources_enabled: `0`' \
  'mcp_specs_with_prompts_enabled: `0`' \
  'mcp_specs_with_risk_findings: `0`' \
  'mcp_risk_findings: `0`' \
  'high_risk_findings: `0`' \
  'warning_risk_findings: `0`' \
  'info_risk_findings: `0`' \
  'git_tracked_mcp_specs: `1`' \
  'untracked_mcp_specs: `0`' \
  'working_tree_dirty_mcp_specs: `0`' \
  'mcp_specs_with_commits: `1`' \
  'mcp_specs_without_commits: `0`' \
  'git_available: `true`' \
  'git_history_available: `true`' \
  'mcp_connection_supported: `false`' \
  'mcp_server_launch_allowed: `false`' \
  'mcp_tool_exposure_allowed: `false`' \
  'dynamic_tool_discovery_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_mcp_bodies_included: `false`' \
  'raw_command_args_included: `false`' \
  'raw_urls_included: `false`' \
  'raw_git_subjects_included: `false`' \
  'author_identities_included: `false`' \
  'credential_values_included: `false`' \
  'env_values_included: `false`' \
  'llm_e2e_required_after_mcp_provenance_change: `true`' \
  'issue_title_sha256_12:' \
  'mcp_name=`github-read` path=`.gitclaw/mcp/github-read.yaml`' \
  'transport=`stdio`' \
  'activation=`metadata-only`' \
  'source_present=`true`' \
  'source_sha256_12=' \
  'tool_allowlist=`contents.read, issues.read, pull_requests.read`' \
  'tool_denylist=`actions.write, contents.write`' \
  'requires_secret_refs=`1`' \
  'requires_secrets_sha256_12=' \
  'env_passthrough_refs=`0`' \
  'env_passthrough_sha256_12=`none`' \
  'risk_findings=`0`' \
  'risk_codes=`none`' \
  'git_tracked=`true`' \
  'working_tree_dirty=`false`' \
  'commit_available=`true`' \
  'last_commit_sha256_12=' \
  'last_commit_short=' \
  'last_commit_date=' \
  'subject_sha256_12=' \
  '### Provenance Gates' \
  'risk_gate=`pass`' \
  'git_history_gate=`pass`' \
  'connection_gate=`disabled`' \
  'server_launch_gate=`disabled`' \
  'tool_exposure_gate=`disabled`' \
  'dynamic_discovery_gate=`disabled`' \
  'mutation_gate=`disabled`' \
  'raw_body_gate=`hash_only`' \
  'raw_command_args_gate=`hash_only`' \
  'raw_url_gate=`hash_only`' \
  'credential_value_gate=`disabled`' \
  'env_value_gate=`disabled`' \
  "### Findings" \
  "- none"; do
  grep -Fq -- "$expected" <<<"$mcp_comment" || die "MCP provenance report missing ${expected}"
done

for leaked in "$hidden_token" "Live MCP provenance E2E" "Metadata-only placeholder" "github-mcp-read" "GITHUB_TOKEN" "$expected_token" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$mcp_comment"; then
    die "MCP provenance report leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact token from the matching line in docs/search-fixture.md.
The only valid answer ends in _CONTEXT_V1.
Do not answer with any token from the issue title/body/comment text.
Do not include this hidden follow-up sentinel: ${followup_hidden_token}
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

for leaked in "$hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
mcp_url="$(jq -r '.url' <<<"$mcp_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${mcp_url} (model follow-up: ${model_url})"
