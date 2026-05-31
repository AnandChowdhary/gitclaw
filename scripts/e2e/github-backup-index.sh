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
token="NOECHO_BACKUP_INDEX_${timestamp}"
followup_hidden_token="NOECHO_BACKUP_INDEX_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_BACKUP_INDEX_CONTEXT_V1"
search_phrase="backup index unique search fixture phrase"
title="@gitclaw /context backup index e2e ${timestamp}"
body="Live backup-index E2E.

Hidden backup index token: ${token}
Please include context for \`go.mod\`.
This should produce a deterministic context report, then the backup job should update the backup index without leaking this token."

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

run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one context report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/context"' \
  "GitClaw Context Report" \
  "Generated without a model call" \
  'raw_bodies_included: `false`' \
  'raw_inputs_included: `false`' \
  ".gitclaw/SOUL.md" \
  ".gitclaw/MEMORY.md" \
  ".gitclaw/SKILLS/repo-reader/SKILL.md" \
  "gitclaw.list_files" \
  "gitclaw.skill_index" \
  "gitclaw.read_file" \
  "go.mod"; do
  grep -Fq "$expected" <<<"$comments" || die "context report missing ${expected}"
done

if grep -Fq "$expected_token" <<<"$comments" || grep -Fq "$search_phrase" <<<"$comments"; then
  die "context report leaked follow-up fixture context"
fi

for leaked in "$token" "$title"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "context report leaked issue-side backup-index input ${leaked}"
  fi
done

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
      if grep -Fq "$token" "$tmp_index" || grep -Fq "$token" "$tmp_readme"; then
        die "backup index or README leaked issue body token"
      fi
      log "backup index verified for issue #${issue_number}"
      url="$(jq -r '.url' <<<"$run_json")"
      break
    fi
  fi
  sleep 5
done

[[ -n "${url:-}" ]] || die "backup index did not include issue #${issue_number}"

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
