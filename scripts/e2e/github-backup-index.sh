#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "backup-index-e2e: $*" >&2
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
title="@gitclaw /context backup index e2e ${timestamp}"
body="Live backup-index E2E.

Please include context for \`go.mod\`.
This should produce a deterministic context report, then the backup job should update the backup index."

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
      gh issue close "$issue_number" --repo "$repo" --comment "backup-index e2e cleanup" >/dev/null 2>&1 || true
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

wait_for_run "$issue_started_at" >/dev/null || die "timed out waiting for issues workflow run"

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
      log "backup index verified for issue #${issue_number}"
      exit 0
    fi
  fi
  sleep 5
done

die "backup index did not include issue #${issue_number}"
