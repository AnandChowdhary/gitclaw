#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "checkpoints-catalog-e2e: $*" >&2
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
token="NOECHO_CHECKPOINTS_CATALOG_${timestamp}"
followup_hidden_token="NOECHO_CHECKPOINTS_CATALOG_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHECKPOINTS_CATALOG_CONTEXT_V1"
search_phrase="checkpoints catalog unique search fixture phrase"
title="@gitclaw /checkpoints catalog e2e ${timestamp}"
body="Live checkpoints-catalog E2E.

Hidden checkpoints catalog body token: ${token}
This should produce a deterministic checkpoint catalog without leaking raw issue text, diffs, or file bodies."

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
      gh issue close "$issue_number" --repo "$repo" --comment "checkpoints-catalog e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one checkpoints catalog comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/checkpoints"' \
  "GitClaw Checkpoints Catalog Report" \
  "Generated without a model call" \
  'requested_checkpoints_command: `catalog`' \
  'checkpoints_command_status: `ok`' \
  'checkpoint_catalog_status: `ok`' \
  'catalog_strategy: `compact-git-history-rollback-discovery`' \
  'checkpoint_strategy: `git-history-plus-backup-branch`' \
  'rollback_model: `github-actions-git-metadata-inspect-only`' \
  'rollback_mode: `inspect-only`' \
  'git_available: `true`' \
  'git_repository: `true`' \
  'head_commit: `' \
  'commits_available: `' \
  'recent_commits_returned: `' \
  'recent_commit_limit: `5`' \
  'worktree_clean: `true`' \
  'staged_changes: `0`' \
  'unstaged_changes: `0`' \
  'untracked_files: `0`' \
  'backup_branch: `' \
  'catalog_entries: `11`' \
  'checkpoint_layers: `7`' \
  'restore_operations_enabled: `false`' \
  'git_reset_allowed: `false`' \
  'git_clean_allowed: `false`' \
  'checkout_mutation_allowed: `false`' \
  'pre_restore_snapshot_required: `true`' \
  'rollback_diff_preview_required: `true`' \
  'backup_manifest_required_for_restore: `true`' \
  'raw_diffs_included: `false`' \
  'raw_file_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'credential_values_included: `false`' \
  'llm_e2e_required_after_checkpoint_catalog_change: `true`' \
  'command=`catalog` issue_intent=`@gitclaw /checkpoints catalog` local_command=`gitclaw checkpoints catalog` execution=`metadata-only` gate=`body-free-checkpoint-command-map` raw_bodies_included=`false` mutation_allowed=`false`' \
  'command=`status` issue_intent=`@gitclaw /checkpoints` local_command=`gitclaw checkpoints status`' \
  'command=`list` issue_intent=`@gitclaw /checkpoints list` local_command=`gitclaw checkpoints list`' \
  'command=`verify` issue_intent=`@gitclaw /checkpoints verify` local_command=`gitclaw checkpoints verify`' \
  'command=`preview` issue_intent=`@gitclaw /checkpoints preview HEAD~1` local_command=`gitclaw checkpoints preview HEAD~1`' \
  'command=`risk` issue_intent=`@gitclaw /checkpoints risk` local_command=`gitclaw checkpoints risk`' \
  'command=`rehearse` issue_intent=`@gitclaw /checkpoints rehearse --id <id>` local_command=`n/a` execution=`issue-native` gate=`rollback-rehearsal-conversation`' \
  'command=`rollback-catalog` issue_intent=`@gitclaw /rollback catalog` local_command=`gitclaw rollback catalog`' \
  'command=`rollback-list` issue_intent=`@gitclaw /rollback` local_command=`gitclaw rollback list`' \
  'command=`rollback-diff` issue_intent=`@gitclaw /rollback diff HEAD~1` local_command=`gitclaw rollback diff HEAD~1`' \
  'command=`rollback-risk` issue_intent=`@gitclaw /rollback risk` local_command=`gitclaw rollback risk`' \
  'layer=`git-history` store=`repository .git metadata`' \
  'layer=`worktree` store=`git status --porcelain`' \
  'layer=`backup-branch` store=`' \
  'layer=`recent-commits` store=`git log metadata`' \
  'layer=`restore-preview` store=`rollback diff stat and path hashes`' \
  'layer=`operation-boundary` store=`unsupported restore/reset/clean/checkout`' \
  'layer=`payloads` store=`unsupported in reports`' \
  'checkpoint_catalog_gate=`ok`' \
  'git_metadata_gate=`available-before-report`' \
  'worktree_gate=`clean-preferred-dirty-is-warning`' \
  'backup_branch_gate=`manifest-required-before-restore`' \
  'rollback_preview_gate=`diff-preview-required-before-restore`' \
  'restore_gate=`disabled-inspect-only-v1`' \
  'destructive_git_gate=`reset-clean-checkout-disabled`' \
  'raw_body_gate=`hashes-counts-and-metadata-only`' \
  'model_e2e_gate=`required`'; do
  grep -Fq "$expected" <<<"$comments" || die "checkpoints catalog report missing ${expected}"
done

if grep -Fq "$token" <<<"$comments"; then
  die "checkpoints catalog report leaked issue body token"
fi
if grep -Fq "$expected_token" <<<"$comments" || grep -Fq "$search_phrase" <<<"$comments"; then
  die "checkpoints catalog report leaked follow-up fixture context"
fi

cli_catalog="$(go run ./cmd/gitclaw checkpoints catalog)"
rollback_catalog="$(go run ./cmd/gitclaw rollback catalog)"
for output_name in cli_catalog rollback_catalog; do
  output="${!output_name}"
  for expected in \
    "GitClaw Checkpoints Catalog Report" \
    'scope: `local-cli`' \
    'catalog_strategy: `compact-git-history-rollback-discovery`' \
    'catalog_entries: `11`' \
    'checkpoint_layers: `7`' \
    'raw_diffs_included: `false`' \
    'raw_file_bodies_included: `false`' \
    'command=`catalog` issue_intent=`@gitclaw /checkpoints catalog` local_command=`gitclaw checkpoints catalog`' \
    'command=`preview` issue_intent=`@gitclaw /checkpoints preview HEAD~1` local_command=`gitclaw checkpoints preview HEAD~1`' \
    'command=`rehearse` issue_intent=`@gitclaw /checkpoints rehearse --id <id>` local_command=`n/a`' \
    'command=`rollback-catalog` issue_intent=`@gitclaw /rollback catalog` local_command=`gitclaw rollback catalog`' \
    'command=`rollback-diff` issue_intent=`@gitclaw /rollback diff HEAD~1` local_command=`gitclaw rollback diff HEAD~1`' \
    'layer=`git-history` store=`repository .git metadata`' \
    'layer=`operation-boundary` store=`unsupported restore/reset/clean/checkout`' \
    'restore_gate=`disabled-inspect-only-v1`'; do
    grep -Fq "$expected" <<<"$output" || die "local ${output_name} missing ${expected}"
  done
  if grep -Fq "$token" <<<"$output"; then
    die "local ${output_name} leaked issue token"
  fi
done

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
