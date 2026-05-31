#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "backup-provenance-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow_name="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
backup_branch="${GITCLAW_E2E_BACKUP_BRANCH:-gitclaw-backups}"
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
token="GITCLAW_BACKUP_PROVENANCE_E2E_${timestamp}"
followup_token="GITCLAW_BACKUP_PROVENANCE_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw /backup provenance e2e ${timestamp}"
body="Live backup provenance E2E.

Hidden backup provenance token: ${token}
This should produce a deterministic backup report, then the fetched backup branch should render body-free git provenance for the backup files."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label gitclaw)"
issue_number="${issue_url##*/}"
tmp_dir=""

cleanup() {
  if [[ -n "${tmp_dir:-}" ]]; then
    rm -rf "$tmp_dir"
  fi
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "backup-provenance e2e cleanup" >/dev/null 2>&1 || true
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

run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one backup provenance report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/backup"' \
  "GitClaw Backup Report" \
  "Generated without a model call" \
  'requested_backup_command: `provenance`' \
  'backup_command_status: `ok`' \
  'issue_side_execution: `deferred_to_post_turn_backup_branch`' \
  'raw_bodies_included: `false`' \
  "requested_local_command: \`gitclaw backup provenance --root .gitclaw/backups --repo ${repo}\`" \
  'backup_provenance_status: `deferred`' \
  'backup_provenance_execution: `local_fetched_backup_branch`' \
  'backup_provenance_gates: `verify, git-history, body-free-output`' \
  'raw_git_subjects_included: `false`' \
  'author_identities_included: `false`' \
  'llm_e2e_required_after_backup_provenance_change: `true`' \
  'backup_branch: `gitclaw-backups`' \
  'backup_schema_version: `1`'; do
  grep -Fq "$expected" <<<"$comments" || die "backup provenance issue report missing ${expected}"
done

for leaked in "$token" "$title"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "backup provenance issue report leaked issue-side input ${leaked}"
  fi
done

repo_key="${repo//\//__}"
issue_padded="$(printf "%06d" "$issue_number")"
issue_path="issues/${issue_padded}.json"

fetch_backup_branch() {
  rm -rf "$tmp_dir"
  tmp_dir="$(mktemp -d)"
  backup_checkout="${tmp_dir}/backup-branch"
  gh repo clone "$repo" "$backup_checkout" -- --branch "$backup_branch" --single-branch >/dev/null 2>&1
}

for _ in {1..60}; do
  if fetch_backup_branch; then
    index_path="${backup_checkout}/.gitclaw/backups/${repo_key}/index.json"
  else
    sleep 5
    continue
  fi
  if [[ -f "$index_path" ]] &&
    jq -e --argjson number "$issue_number" --arg title "$title" --arg path "$issue_path" '
      any(.issues[]; .number == $number and .title == $title and .path == $path)
    ' "$index_path" >/dev/null; then
    issue_count="$(jq -r '.count' "$index_path")"
    provenance_files="$((issue_count + 2))"
    provenance_output="$(go run ./cmd/gitclaw backup provenance --root "${backup_checkout}/.gitclaw/backups" --repo "$repo")"
    for expected in \
      "GitClaw Backup Provenance Report" \
      "repository: \`${repo}\`" \
      'backup_provenance_status: `ok`' \
      'backup_verify_status: `ok`' \
      'verification_failures: `0`' \
      'expected_backup_branch: `gitclaw-backups`' \
      'backup_schema_version: `1`' \
      "issue_count: \`${issue_count}\`" \
      'control_files: `2`' \
      "issue_payload_files: \`${issue_count}\`" \
      "provenance_files: \`${provenance_files}\`" \
      "git_tracked_files: \`${provenance_files}\`" \
      "files_with_commits: \`${provenance_files}\`" \
      'untracked_files: `0`' \
      'working_tree_dirty_files: `0`' \
      'git_available: `true`' \
      'git_history_available: `true`' \
      'raw_backup_bodies_included: `false`' \
      'raw_git_subjects_included: `false`' \
      'author_identities_included: `false`' \
      'repository_mutation_allowed: `false`' \
      'llm_e2e_required_after_backup_provenance_change: `true`' \
      'verify_gate=`pass`' \
      'git_history_gate=`pass`' \
      'mutation_gate=`disabled`' \
      'kind=`index` issue=none path=`index.json`' \
      'kind=`readme` issue=none path=`README.md`' \
      "kind=\`issue-backup\` issue=#${issue_number} path=\`${issue_path}\`" \
      'last_commit_sha256_12=' \
      'last_commit_short=' \
      'subject_sha256_12='; do
      grep -Fq "$expected" <<<"$provenance_output" || die "backup provenance output missing ${expected}"
    done
    for leaked in "$token" "$title" "Hidden backup provenance token"; do
      if grep -Fq "$leaked" <<<"$provenance_output"; then
        die "backup provenance output leaked ${leaked}"
      fi
    done
    break
  fi
  sleep 5
done

if [[ -z "${provenance_output:-}" ]]; then
  die "backup provenance did not observe issue #${issue_number} in ${backup_branch}"
fi

followup_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_SEARCH token from the matching repository search result line.
Do not include this hidden follow-up token: ${followup_token}
Keep the answer under 30 words." >/dev/null

followup_run_json="$(wait_for_run issue_comment "$followup_started_at")" || die "timed out waiting for model follow-up issue_comment workflow run"
wait_for_assistant_count 2 || die "expected model-backed follow-up assistant comment"
followup_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$followup_comment" || die "model follow-up did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$followup_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$followup_comment"; then
  die "model follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$followup_comment" || die "model follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$followup_comment" || die "model follow-up marker missing repo-reader skill"
grep -Fq 'tools="' <<<"$followup_comment" || die "model follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$followup_comment" || die "model follow-up marker did not prove search_files was prompt-visible"

if grep -Fq "$followup_token" <<<"$followup_comment"; then
  die "model follow-up leaked hidden follow-up token"
fi

issue_run_url="$(jq -r '.url' <<<"$run_json")"
followup_run_url="$(jq -r '.url' <<<"$followup_run_json")"
log "passed for issue #${issue_number}: backup=${issue_run_url} followup=${followup_run_url}"
