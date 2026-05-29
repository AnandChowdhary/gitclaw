#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "backup-report-e2e: $*" >&2
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
token="GITCLAW_BACKUP_REPORT_E2E_${timestamp}"
title="@gitclaw /backup e2e ${timestamp}"
body="Live backup-report E2E.

Hidden backup body token: ${token}
This should produce a deterministic backup report, then the backup job should update the backup branch."

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
      gh issue close "$issue_number" --repo "$repo" --comment "backup-report e2e cleanup" >/dev/null 2>&1 || true
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

read_branch_file() {
  local path="$1"
  gh api "repos/${repo}/contents/${path}?ref=${backup_branch}" \
    --jq '.content' \
    | python3 -c 'import base64, sys; print(base64.b64decode(sys.stdin.read()).decode(), end="")'
}

repo_key="${repo//\//__}"
issue_padded="$(printf "%06d" "$issue_number")"
issue_backup_path=".gitclaw/backups/${repo_key}/issues/${issue_padded}.json"
index_path=".gitclaw/backups/${repo_key}/index.json"
readme_path=".gitclaw/backups/${repo_key}/README.md"

wait_for_run "$issue_started_at" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one backup report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/backup"' \
  "GitClaw Backup Report" \
  "Generated without a model call" \
  'backup_branch: `gitclaw-backups`' \
  "$issue_backup_path" \
  "$index_path" \
  "$readme_path" \
  'backup_schema_version: `1`' \
  'transcript_messages_now: `1`'; do
  grep -Fq "$expected" <<<"$comments" || die "backup report missing ${expected}"
done

if grep -Fq "$token" <<<"$comments"; then
  die "backup report leaked issue body token"
fi

tmp_index="$(mktemp)"
tmp_readme="$(mktemp)"
cleanup_tmp() {
  rm -f "$tmp_index" "$tmp_readme"
}
trap 'cleanup_tmp; cleanup' EXIT

for _ in {1..60}; do
  if read_branch_file "$index_path" >"$tmp_index" 2>/dev/null &&
    read_branch_file "$readme_path" >"$tmp_readme" 2>/dev/null &&
    read_branch_file "$issue_backup_path" >/dev/null 2>&1; then
    if jq -e --argjson number "$issue_number" --arg title "$title" --arg path "issues/${issue_padded}.json" '
      .version == 1
      and .repo == "'"${repo}"'"
      and (.count >= 1)
      and any(.issues[]; .number == $number and .title == $title and .path == $path and .comment_count >= 1 and .transcript_messages >= 1)
    ' "$tmp_index" >/dev/null &&
      grep -Fq "#${issue_number}" "$tmp_readme" &&
      grep -Fq "issues/${issue_padded}.json" "$tmp_readme"; then
      wait_for_done_status || die "expected gitclaw:done without running/error"
      log "passed for issue #${issue_number}"
      exit 0
    fi
  fi
  sleep 5
done

die "backup branch did not include report issue #${issue_number}"
