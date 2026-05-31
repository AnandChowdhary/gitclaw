#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "memory-provenance-report-e2e: $*" >&2
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
hidden_token="NOECHO_MEMORY_PROVENANCE_${timestamp}"
followup_hidden_token="NOECHO_MEMORY_PROVENANCE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_MEMORY_PROVENANCE_CONTEXT_V1"
search_phrase="memory provenance unique search fixture phrase"
title="@gitclaw /memory provenance e2e ${timestamp}"
body="@gitclaw /memory provenance

Live memory-provenance E2E.
Do not include this hidden issue token: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw memory provenance)"
for expected in \
  "GitClaw Memory Provenance Report" \
  'memory_provenance_status: `ok`' \
  'provenance_scope: `repo-local-memory-git-history`' \
  'memory_files: `2`' \
  'long_term_memory_present: `true`' \
  'long_term_memory_loaded: `true`' \
  'dated_memory_notes: `1`' \
  'canonical_dated_memory_notes: `1`' \
  'noncanonical_dated_memory_notes: `0`' \
  'loaded_memory_notes: `1`' \
  'omitted_memory_notes: `0`' \
  'first_memory_note: `.gitclaw/memory/2026-05-29.md`' \
  'latest_memory_note: `.gitclaw/memory/2026-05-29.md`' \
  'repo_local_memory_files: `2`' \
  'unknown_memory_files: `0`' \
  'git_tracked_memory_files: `2`' \
  'untracked_memory_files: `0`' \
  'working_tree_dirty_memory_files: `0`' \
  'memory_files_with_commits: `2`' \
  'memory_files_without_commits: `0`' \
  'git_available: `true`' \
  'git_history_available: `true`' \
  'external_provider_accessed: `false`' \
  'session_search_index_source: `github-issues-and-backups`' \
  'background_promotion_active: `false`' \
  'memory_writes_allowed: `false`' \
  'raw_memory_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_git_subjects_included: `false`' \
  'author_identities_included: `false`' \
  'llm_e2e_required_after_memory_provenance_change: `true`' \
  'memory_validation_status: `ok`' \
  'memory_risk_status: `ok`' \
  "### Memory Provenance Cards" \
  'kind=`long-term` path=`.gitclaw/MEMORY.md` source=`repo-local` role=`stable-summary` date=`long-term`' \
  'kind=`dated-note` path=`.gitclaw/memory/2026-05-29.md` source=`repo-local` role=`latest-daily-note` date=`2026-05-29`' \
  'risk_codes=`none`' \
  'validation_findings=`0`' \
  'git_tracked=`true`' \
  'working_tree_dirty=`false`' \
  'commit_available=`true`' \
  'subject_sha256_12=' \
  "### Provenance Gates" \
  'validation_gate=`pass`' \
  'risk_gate=`pass`' \
  'git_history_gate=`pass`' \
  'memory_write_gate=`disabled`' \
  'external_provider_gate=`not_configured`' \
  'session_search_gate=`github-issues-and-backups`' \
  'raw_body_gate=`hash_only`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local memory provenance report missing ${expected}"
done

for leaked in "$expected_token" "$search_phrase" "GitClaw remembers durable product decisions" "Keep persistent state"; do
  if grep -Fq "$leaked" <<<"$local_report"; then
    die "local memory provenance report leaked ${leaked}"
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
      gh issue close "$issue_number" --repo "$repo" --comment "memory-provenance-report e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one memory provenance report comment"
provenance_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/memory"' \
  "GitClaw Memory Provenance Report" \
  "Generated without a model call" \
  'memory_provenance_status: `ok`' \
  'provenance_scope: `repo-local-memory-git-history`' \
  'memory_files: `2`' \
  'repo_local_memory_files: `2`' \
  'git_tracked_memory_files: `2`' \
  'untracked_memory_files: `0`' \
  'working_tree_dirty_memory_files: `0`' \
  'memory_files_with_commits: `2`' \
  'memory_files_without_commits: `0`' \
  'git_history_available: `true`' \
  'external_provider_accessed: `false`' \
  'session_search_index_source: `github-issues-and-backups`' \
  'background_promotion_active: `false`' \
  'memory_writes_allowed: `false`' \
  'raw_memory_bodies_included: `false`' \
  'raw_git_subjects_included: `false`' \
  'author_identities_included: `false`' \
  'llm_e2e_required_after_memory_provenance_change: `true`' \
  'memory_validation_status: `ok`' \
  'memory_risk_status: `ok`' \
  "### Memory Provenance Cards" \
  'kind=`long-term` path=`.gitclaw/MEMORY.md`' \
  'kind=`dated-note` path=`.gitclaw/memory/2026-05-29.md`' \
  'git_tracked=`true`' \
  'working_tree_dirty=`false`' \
  'commit_available=`true`' \
  'last_commit_sha256_12=' \
  'subject_sha256_12=' \
  "### Provenance Gates" \
  'validation_gate=`pass`' \
  'risk_gate=`pass`' \
  'git_history_gate=`pass`' \
  'memory_write_gate=`disabled`' \
  'external_provider_gate=`not_configured`' \
  'session_search_gate=`github-issues-and-backups`' \
  'raw_body_gate=`hash_only`'; do
  grep -Fq -- "$expected" <<<"$provenance_comment" || die "memory provenance report missing ${expected}"
done

for leaked in "$hidden_token" "$expected_token" "$search_phrase" "Live memory-provenance E2E" "GitClaw remembers durable product decisions" "Keep persistent state"; do
  if grep -Fq "$leaked" <<<"$provenance_comment"; then
    die "memory provenance report leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

The matching repository search result line in \`docs/search-fixture.md\` has the form \`${search_phrase} => <token>\`.
Reply with only the uppercase fixture token immediately after \`=>\`, including the \`_CONTEXT_V1\` suffix.
Do not abbreviate the token to a prefix.
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
