#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "agents-provenance-e2e: $*" >&2
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
token="NOECHO_AGENTS_PROVENANCE_${timestamp}"
followup_hidden_token="NOECHO_AGENTS_PROVENANCE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_AGENTS_PROVENANCE_CONTEXT_V1"
search_phrase="agents provenance unique search fixture phrase"
title="@gitclaw /agents provenance e2e ${timestamp}"
body="Live agents-provenance E2E.

Hidden agents provenance body token: ${token}
This should produce a deterministic agents provenance report without leaking raw issue text."

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
      gh issue close "$issue_number" --repo "$repo" --comment "agents-provenance e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one agents provenance comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/agents"' \
  "GitClaw Agent Provenance Report" \
  "Generated without a model call" \
  'agent_provenance_status: `ok`' \
  'provenance_scope: `repo-local-agent-git-history`' \
  'agents_status: `ok`' \
  'agent_validation_status: `ok`' \
  'agent_validation_findings: `0`' \
  'agent_risk_status: `ok`' \
  'agent_risk_findings: `0`' \
  'agent_policy_present: `true`' \
  'agent_policy_loaded_for_model: `true`' \
  'agent_specs: `1`' \
  'agent_specs_with_frontmatter: `1`' \
  'agent_roles: `1`' \
  'agent_tools_requested: `2`' \
  'agent_specs_requiring_approval: `1`' \
  'agent_specs_single_assistant: `1`' \
  'provenance_surfaces: `2`' \
  'repo_local_surfaces: `2`' \
  'unknown_source_surfaces: `0`' \
  'git_tracked_surfaces: `2`' \
  'untracked_surfaces: `0`' \
  'working_tree_dirty_surfaces: `0`' \
  'surfaces_with_commits: `2`' \
  'surfaces_without_commits: `0`' \
  'git_available: `true`' \
  'git_history_available: `true`' \
  'active_agent_runtime: `github-actions`' \
  'multi_agent_routing_supported: `false`' \
  'multi_agent_delegation_supported: `false`' \
  'subagent_execution_supported: `false`' \
  'delegate_task_supported: `false`' \
  'remote_agent_process_allowed: `false`' \
  'agent_to_agent_messaging_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_agent_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'raw_git_subjects_included: `false`' \
  'author_identities_included: `false`' \
  'credential_values_included: `false`' \
  'llm_e2e_required_after_agent_provenance_change: `true`' \
  'kind=`agent-policy` name=`agents-policy` path=`.gitclaw/AGENTS.md` source=`repo-local`' \
  'kind=`agent-spec` name=`repo-assistant` path=`.gitclaw/agents/repo-assistant.md` source=`repo-local`' \
  'risk_findings=`0`' \
  'risk_codes=`none`' \
  'validation_findings=`0`' \
  'git_tracked=`true`' \
  'working_tree_dirty=`false`' \
  'commit_available=`true`' \
  'last_commit_sha256_12=' \
  'last_commit_short=' \
  'last_commit_date=' \
  'subject_sha256_12=' \
  'agent_validation_gate=`pass`' \
  'risk_gate=`pass`' \
  'git_history_gate=`pass`' \
  'runtime_gate=`github-actions-only`' \
  'delegation_gate=`disabled-single-assistant-v1`' \
  'mutation_gate=`disabled`' \
  'raw_body_gate=`hash_only`'; do
  grep -Fq "$expected" <<<"$comments" || die "agents provenance report missing ${expected}"
done

if grep -Fq "$token" <<<"$comments"; then
  die "agents provenance report leaked issue body token"
fi
if grep -Fq "$expected_token" <<<"$comments" || grep -Fq "$search_phrase" <<<"$comments"; then
  die "agents provenance report leaked follow-up fixture context"
fi

cli_report="$(go run ./cmd/gitclaw agents provenance)"
for expected in \
  "GitClaw Agent Provenance Report" \
  'scope: `local-cli`' \
  'agent_provenance_status: `ok`' \
  'provenance_scope: `repo-local-agent-git-history`' \
  'agent_risk_status: `ok`' \
  'provenance_surfaces: `2`' \
  'git_tracked_surfaces: `2`' \
  'surfaces_with_commits: `2`' \
  'raw_agent_bodies_included: `false`' \
  'raw_git_subjects_included: `false`' \
  'agent_validation_gate=`pass`' \
  'git_history_gate=`pass`'; do
  grep -Fq "$expected" <<<"$cli_report" || die "local agents provenance missing ${expected}"
done
if grep -Fq "$token" <<<"$cli_report"; then
  die "local agents provenance leaked issue token"
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
