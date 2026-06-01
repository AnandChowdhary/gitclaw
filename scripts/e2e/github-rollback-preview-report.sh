#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "rollback-preview-e2e: $*" >&2
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
token="NOECHO_ROLLBACK_PREVIEW_${timestamp}"
followup_hidden_token="NOECHO_ROLLBACK_PREVIEW_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_ROLLBACK_PREVIEW_CONTEXT_V1"
search_phrase="rollback preview unique search fixture phrase"
title="@gitclaw /rollback diff HEAD~1 e2e ${timestamp}"
body="Live rollback-preview E2E.

Hidden rollback preview body token: ${token}
This should produce a deterministic rollback diff-stat preview without raw patch hunks, file bodies, restore, reset, or checkout."

local_preview="$(go run ./cmd/gitclaw rollback diff HEAD~1)"
for expected in \
  "GitClaw Rollback Preview Report" \
  'scope: `local-cli`' \
  'rollback_preview_status: `' \
  'preview_strategy: `git-diff-stat-inspect-only`' \
  'rollback_mode: `preview-only`' \
  'target_ref: `HEAD~1`' \
  'changed_files: `' \
  'preview_files_returned: `' \
  'raw_diffs_included: `false`' \
  'raw_file_bodies_included: `false`' \
  'path_names_included: `false`' \
  'path_hashes_included: `true`' \
  'restore_operations_enabled: `false`' \
  'llm_e2e_required_after_rollback_preview_change: `true`' \
  'kind=`rollback-preview`' \
  'raw_diff_gate=`numstat-name-status-and-path-hashes-only`'; do
  grep -Fq "$expected" <<<"$local_preview" || die "local rollback preview missing ${expected}"
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
      gh issue close "$issue_number" --repo "$repo" --comment "rollback-preview e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one rollback preview comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/checkpoints"' \
  "GitClaw Rollback Preview Report" \
  "Generated without a model call" \
  'requested_checkpoints_command: `preview`' \
  'checkpoints_command_status: `ok`' \
  'rollback_preview_status: `ok`' \
  'preview_strategy: `git-diff-stat-inspect-only`' \
  'rollback_mode: `preview-only`' \
  'target_ref: `HEAD~1`' \
  'target_commit: `' \
  'head_commit: `' \
  'comparison_range_sha256_12: `' \
  'git_available: `true`' \
  'git_repository: `true`' \
  'worktree_clean: `true`' \
  'changed_files: `' \
  'preview_files_returned: `' \
  'preview_file_limit: `20`' \
  'raw_diffs_included: `false`' \
  'raw_file_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'credential_values_included: `false`' \
  'path_names_included: `false`' \
  'path_hashes_included: `true`' \
  'restore_operations_enabled: `false`' \
  'git_reset_allowed: `false`' \
  'git_clean_allowed: `false`' \
  'checkout_mutation_allowed: `false`' \
  'pre_restore_snapshot_required: `true`' \
  'backup_manifest_required_for_restore: `true`' \
  'llm_e2e_required_after_rollback_preview_change: `true`' \
  'kind=`rollback-preview`' \
  'path_sha256_12=`' \
  'raw_path_included=`false`' \
  'rollback_preview_gate=`ok`' \
  'target_ref_gate=`resolved-before-preview`' \
  'worktree_gate=`clean-required-before-future-restore`' \
  'restore_gate=`disabled-preview-only`' \
  'destructive_git_gate=`reset-clean-checkout-disabled`' \
  'raw_diff_gate=`numstat-name-status-and-path-hashes-only`' \
  'model_e2e_gate=`required`'; do
  grep -Fq "$expected" <<<"$comments" || die "rollback preview report missing ${expected}"
done

for leaked in \
  "$token" \
  "$search_phrase" \
  "Hidden rollback preview body token" \
  "This should produce a deterministic rollback diff-stat preview"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "rollback preview report leaked ${leaked}"
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
