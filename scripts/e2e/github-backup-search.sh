#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "backup-search-e2e: $*" >&2
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
token="GITCLAW_BACKUP_SEARCH_E2E_${timestamp}"
title="@gitclaw /backup search e2e ${timestamp}"
body="Live backup search E2E.

Hidden backup search body token: ${token}
This should produce a deterministic backup report, write the raw backup branch, then let local backup search find the hidden token without printing it."

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
      gh issue close "$issue_number" --repo "$repo" --comment "backup-search e2e cleanup" >/dev/null 2>&1 || true
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
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
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
wait_for_assistant_count 1 || die "expected one backup report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/backup"' \
  "GitClaw Backup Report" \
  "Generated without a model call" \
  'requested_backup_command: `search`' \
  'backup_command_status: `ok`' \
  'issue_side_execution: `deferred_to_post_turn_backup_branch`' \
  'raw_bodies_included: `false`' \
  "requested_local_command: \`gitclaw backup search --root .gitclaw/backups --repo ${repo} <query>\`" \
  'query_sha256_12:' \
  'query_terms: `2`' \
  'raw search query is not printed' \
  'backup_branch: `gitclaw-backups`' \
  'backup_schema_version: `1`'; do
  grep -Fq "$expected" <<<"$comments" || die "backup report missing ${expected}"
done

for leaked in "$token" "$timestamp" "$title"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "backup report leaked issue-side search input ${leaked}"
  fi
done

repo_key="${repo//\//__}"
issue_padded="$(printf "%06d" "$issue_number")"
issue_path="issues/${issue_padded}.json"

fetch_backup_branch() {
  rm -rf "$tmp_dir"
  tmp_dir="$(mktemp -d)"
  backup_checkout="${tmp_dir}/backup-branch"
  gh repo clone "$repo" "$backup_checkout" -- --depth=1 --branch "$backup_branch" >/dev/null 2>&1
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
    search_output="$(go run ./cmd/gitclaw backup search --root "${backup_checkout}/.gitclaw/backups" --repo "$repo" --query "$token" --max-results 3)"
    for expected in \
      "GitClaw Backup Search Report" \
      "repository: \`${repo}\`" \
      'backup_search_status: `ok`' \
      'backup_verify_status: `ok`' \
      'verification_failures: `0`' \
      'query_sha256_12:' \
      'query_terms: `1`' \
      'max_results: `3`' \
      "issue=\`#${issue_number}\` path=\`${issue_path}\`" \
      'matched_issues: `1`' \
      'matched_lines: `2`' \
      'results_returned: `2`' \
      'raw_bodies_included: `false`' \
      'source=`issue.body`' \
      'source=`transcript:01`' \
      'line_sha256_12='; do
      grep -Fq "$expected" <<<"$search_output" || die "backup search missing ${expected}"
    done
    for leaked in "$token" "$title" "Hidden backup search body token" "This should produce a deterministic backup report"; do
      if grep -Fq "$leaked" <<<"$search_output"; then
        die "backup search leaked ${leaked}"
      fi
    done
    url="$(jq -r '.url' <<<"$run_json")"
    log "passed for issue #${issue_number}: ${url}"
    exit 0
  fi
  sleep 5
done

die "backup search did not observe issue #${issue_number} in ${backup_branch}"
