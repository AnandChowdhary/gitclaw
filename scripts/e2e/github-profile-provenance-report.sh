#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "profile-provenance-report-e2e: $*" >&2
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
hidden_token="NOECHO_PROFILE_PROVENANCE_${timestamp}"
followup_hidden_token="NOECHO_PROFILE_PROVENANCE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_PROFILE_PROVENANCE_CONTEXT_V1"
search_phrase="profile provenance unique search fixture phrase"
title="@gitclaw /profile provenance e2e ${timestamp}"
body="@gitclaw /profile provenance

Live profile-provenance E2E. Mention repo-reader so the provenance report proves selected profile surfaces without exposing raw profile bodies.
Do not include this hidden profile provenance token: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw profile provenance)"
for expected in \
  "GitClaw Profile Provenance Report" \
  "Generated without a model call" \
  'scope: `local-cli`' \
  'profile_provenance_status: `ok`' \
  'provenance_scope: `repo-local-profile-git-history`' \
  'provenance_sha256_12:' \
  'profile_strategy: `repo-local-git-profile`' \
  'profile_store: `.gitclaw/`' \
  'profile_scope: `repository`' \
  'profile_documents_loaded:' \
  'manifest_entries:' \
  'profile_surfaces:' \
  'repo_local_surfaces:' \
  'portable_surfaces:' \
  'selected_surfaces:' \
  'enabled_surfaces:' \
  'git_tracked_surfaces:' \
  'untracked_surfaces: `0`' \
  'working_tree_dirty_surfaces: `0`' \
  'surfaces_with_commits:' \
  'surfaces_without_commits: `0`' \
  'available_skills: `1`' \
  'skill_bundles: `1`' \
  'available_tools: `5`' \
  'manifest_sha256_12:' \
  'profile_snapshot_sha256_12:' \
  'git_available: `true`' \
  'git_history_available: `true`' \
  'external_profile_home_accessed: `false`' \
  'profile_export_supported: `false`' \
  'profile_import_supported: `false`' \
  'profile_switching_supported: `false`' \
  'profile_distribution_install_supported: `false`' \
  'profile_mutation_allowed: `false`' \
  'credentials_included: `false`' \
  'sessions_included: `false`' \
  'backup_payloads_included: `false`' \
  'raw_profile_bodies_included: `false`' \
  'raw_skill_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_git_subjects_included: `false`' \
  'author_identities_included: `false`' \
  'llm_e2e_required_after_profile_provenance_change: `true`' \
  '### Profile Provenance Cards' \
  'kind=`profile-config` name=`config`' \
  'kind=`profile-document` name=`soul`' \
  'kind=`skill` name=`repo-reader`' \
  'kind=`skill-bundle` name=`repo-context`' \
  'kind=`toolset-spec` name=`repo-read`' \
  'git_tracked=`true`' \
  'working_tree_dirty=`false`' \
  'commit_available=`true`' \
  'last_commit_sha256_12=' \
  'subject_sha256_12=' \
  '### Provenance Gates' \
  'manifest_gate=`pass`' \
  'snapshot_gate=`pass`' \
  'git_history_gate=`pass`' \
  'profile_export_gate=`disabled`' \
  'profile_import_gate=`disabled`' \
  'profile_switching_gate=`disabled`' \
  'mutation_gate=`disabled`' \
  'external_profile_home_gate=`not_accessed`' \
  'session_payload_gate=`excluded`' \
  'backup_payload_gate=`excluded`' \
  'raw_body_gate=`hash_only`' \
  'git_subject_gate=`sha256_12_only`' \
  '### Findings' \
  '- none'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local profile provenance report missing ${expected}"
done

for leaked in "$expected_token" "$search_phrase" "GitClaw is a repo-native GitHub issue assistant" "Use GitClaw's read-only repository context" "Prefer repository context and deterministic tool outputs"; do
  if grep -Fq "$leaked" <<<"$local_report"; then
    die "local profile provenance report leaked ${leaked}"
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
      gh issue close "$issue_number" --repo "$repo" --comment "profile-provenance-report e2e cleanup" >/dev/null 2>&1 || true
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

provenance_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one profile provenance report comment"
provenance_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/profile"' \
  "GitClaw Profile Provenance Report" \
  "Generated without a model call" \
  'repository:' \
  'issue:' \
  'profile_provenance_status: `ok`' \
  'provenance_scope: `repo-local-profile-git-history`' \
  'provenance_sha256_12:' \
  'profile_documents_loaded:' \
  'manifest_entries:' \
  'profile_surfaces:' \
  'repo_local_surfaces:' \
  'git_tracked_surfaces:' \
  'untracked_surfaces: `0`' \
  'working_tree_dirty_surfaces: `0`' \
  'surfaces_with_commits:' \
  'surfaces_without_commits: `0`' \
  'available_skills: `1`' \
  'skill_bundles: `1`' \
  'available_tools: `5`' \
  'manifest_sha256_12:' \
  'profile_snapshot_sha256_12:' \
  'git_available: `true`' \
  'git_history_available: `true`' \
  'raw_profile_bodies_included: `false`' \
  'raw_skill_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_git_subjects_included: `false`' \
  'author_identities_included: `false`' \
  'llm_e2e_required_after_profile_provenance_change: `true`' \
  'issue_title_sha256_12:' \
  'kind=`profile-config` name=`config`' \
  'kind=`profile-document` name=`soul`' \
  'kind=`skill` name=`repo-reader`' \
  'kind=`skill-bundle` name=`repo-context`' \
  'kind=`toolset-spec` name=`repo-read`' \
  'git_tracked=`true`' \
  'working_tree_dirty=`false`' \
  'commit_available=`true`' \
  'last_commit_sha256_12=' \
  'subject_sha256_12=' \
  'manifest_gate=`pass`' \
  'snapshot_gate=`pass`' \
  'git_history_gate=`pass`' \
  'profile_export_gate=`disabled`' \
  'session_payload_gate=`excluded`' \
  'backup_payload_gate=`excluded`' \
  'raw_body_gate=`hash_only`' \
  'git_subject_gate=`sha256_12_only`'; do
  grep -Fq -- "$expected" <<<"$provenance_comment" || die "profile provenance report missing ${expected}"
done

for leaked in "$hidden_token" "Live profile-provenance E2E" "$expected_token" "$search_phrase" "GitClaw is a repo-native GitHub issue assistant" "Use GitClaw's read-only repository context" "Prefer repository context and deterministic tool outputs"; do
  if grep -Fq "$leaked" <<<"$provenance_comment"; then
    die "profile provenance report leaked ${leaked}"
  fi
done

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

for leaked in "$hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
provenance_url="$(jq -r '.url' <<<"$provenance_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${provenance_url} (model follow-up: ${model_url})"
