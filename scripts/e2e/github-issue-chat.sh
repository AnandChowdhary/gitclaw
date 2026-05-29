#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "e2e: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

need gh
need date

: "${GITCLAW_E2E_REPO:?set GITCLAW_E2E_REPO, e.g. owner/gitclaw-e2e-sandbox}"

workflow_name="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
retention_label="${GITCLAW_E2E_RETENTION_LABEL:-gitclaw:e2e}"
backup_branch="${GITCLAW_E2E_BACKUP_BRANCH:-gitclaw-backups}"
expect_backup="${GITCLAW_E2E_EXPECT_BACKUP:-0}"
run_deadline_seconds="${GITCLAW_E2E_RUN_DEADLINE_SECONDS:-300}"
comment_deadline_seconds="${GITCLAW_E2E_COMMENT_DEADLINE_SECONDS:-180}"

if [[ "$expect_backup" == "1" ]]; then
  need python3
fi

gh auth status >/dev/null
gh repo view "$GITCLAW_E2E_REPO" >/dev/null

if ! gh label list --repo "$GITCLAW_E2E_REPO" --limit 1000 --json name --jq '.[].name' | grep -Fxq gitclaw; then
  die "sandbox repo is missing required label: gitclaw"
fi

if ! gh workflow view "$workflow_name" --repo "$GITCLAW_E2E_REPO" >/dev/null 2>&1; then
  die "sandbox repo is missing workflow: $workflow_name"
fi

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
title="@gitclaw e2e ${timestamp}"
token_a="GITCLAW_E2E_${timestamp}_A"
token_b="GITCLAW_E2E_${timestamp}_B"
module_path="github.com/AnandChowdhary/gitclaw"
memory_token="GITCLAW_MEMORY_CONTEXT_V1"
skill_token="GITCLAW_SKILL_CONTEXT_V1"
search_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
body="Live E2E conversation check.

Please use the repository file \`go.mod\`.
Please use the repo-reader skill.
Search the repository for \`${search_phrase}\` and include the associated token from the matching line.
Reply with the exact token \`${token_a}\`.
Also state the Go module path from \`go.mod\`.
Also include the exact durable memory token from \`.gitclaw/MEMORY.md\`.
Also include the exact skill verification token from the repo-reader skill.
Keep the answer under 80 words."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$GITCLAW_E2E_REPO" \
  --title "$title" \
  --body "$body" \
  --label gitclaw)"
issue_number="${issue_url##*/}"

cleanup() {
  status=$?
  if [[ -n "${issue_number:-}" ]]; then
    if gh label list --repo "$GITCLAW_E2E_REPO" --limit 1000 --json name --jq '.[].name' | grep -Fxq "gitclaw:disabled"; then
      gh issue edit "$issue_number" --repo "$GITCLAW_E2E_REPO" --add-label "gitclaw:disabled" >/dev/null 2>&1 || true
    fi
    if gh label list --repo "$GITCLAW_E2E_REPO" --limit 1000 --json name --jq '.[].name' | grep -Fxq "$retention_label"; then
      gh issue edit "$issue_number" --repo "$GITCLAW_E2E_REPO" --add-label "$retention_label" >/dev/null 2>&1 || true
    fi
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$GITCLAW_E2E_REPO" >/dev/null 2>&1 || true
    fi
  fi
  exit "$status"
}
trap cleanup EXIT

echo "e2e: created issue #${issue_number}: ${issue_url}"

wait_for_run() {
  local event_name="$1"
  local started_at="$2"
  local deadline=$((SECONDS + run_deadline_seconds))
  while (( SECONDS < deadline )); do
    local run_id
    run_id="$(gh run list \
      --repo "$GITCLAW_E2E_REPO" \
      --workflow "$workflow_name" \
      --event "$event_name" \
      --created ">=$started_at" \
      --json databaseId,displayTitle,status,conclusion,createdAt \
      --jq '.[0].databaseId' \
      | head -n 1)"
    if [[ -n "$run_id" ]]; then
      gh run watch "$run_id" --repo "$GITCLAW_E2E_REPO" --exit-status
      echo "$run_id"
      return 0
    fi
    sleep 5
  done
  return 1
}

count_gitclaw_comments() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

gitclaw_comment_bodies() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | join("\n---GITCLAW-COMMENT---\n")'
}

backup_path_for_issue() {
  local repo_key
  local issue_padded
  repo_key="${GITCLAW_E2E_REPO//\//__}"
  issue_padded="$(printf "%06d" "$issue_number")"
  printf ".gitclaw/backups/%s/issues/%s.json" "$repo_key" "$issue_padded"
}

read_backup_json() {
  local path
  path="$(backup_path_for_issue)"
  gh api "repos/${GITCLAW_E2E_REPO}/contents/${path}?ref=${backup_branch}" \
    --jq '.content' \
    | python3 -c 'import base64, sys; print(base64.b64decode(sys.stdin.read()).decode(), end="")'
}

assert_backup_json() {
  local file="$1"
  python3 - "$file" "$token_a" "$token_b" "$module_path" "$memory_token" "$skill_token" "$search_token" <<'PY'
import json
import sys

path, token_a, token_b, module_path, memory_token, skill_token, search_token = sys.argv[1:8]
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)
body = json.dumps(data, sort_keys=True)
errors = []
if data.get("version") != 1:
    errors.append("version is not 1")
if data.get("issue", {}).get("number") is None:
    errors.append("missing issue number")
if len(data.get("comments", [])) < 3:
    errors.append("expected at least three raw comments")
if len(data.get("transcript", [])) < 4:
    errors.append("expected at least four transcript messages")
for value in (token_a, token_b, module_path, memory_token, skill_token, search_token):
    if value not in body:
        errors.append(f"missing {value}")
if errors:
    raise SystemExit("; ".join(errors))
PY
}

wait_for_backup() {
  local deadline=$((SECONDS + comment_deadline_seconds))
  local tmp
  tmp="$(mktemp)"
  while (( SECONDS < deadline )); do
    if read_backup_json >"$tmp" 2>/dev/null && assert_backup_json "$tmp"; then
      rm -f "$tmp"
      return 0
    fi
    sleep 5
  done
  rm -f "$tmp"
  return 1
}

wait_for_comment_count() {
  local want="$1"
  local deadline=$((SECONDS + comment_deadline_seconds))
  while (( SECONDS < deadline )); do
    local got
    got="$(count_gitclaw_comments)"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

if ! wait_for_run issues "$issue_started_at" >/dev/null; then
  die "timed out waiting for issues workflow run for #${issue_number}"
fi
wait_for_comment_count 1 || die "expected one GitClaw assistant comment after issue open"
first_bodies="$(gitclaw_comment_bodies)"
grep -Fq "$token_a" <<<"$first_bodies" || die "first assistant comment did not include expected conversation token ${token_a}"
grep -Fq "$module_path" <<<"$first_bodies" || die "first assistant comment did not use go.mod module path ${module_path}"
grep -Fq "$memory_token" <<<"$first_bodies" || die "first assistant comment did not use memory context token ${memory_token}"
grep -Fq "$skill_token" <<<"$first_bodies" || die "first assistant comment did not use repo-reader skill token ${skill_token}"
grep -Fq "$search_token" <<<"$first_bodies" || die "first assistant comment did not use search_files token ${search_token}"
echo "e2e: issue-open response verified"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$GITCLAW_E2E_REPO" \
  --body "Follow-up E2E conversation check.

Reply with the exact new token \`${token_b}\`.
Also mention the earlier token \`${token_a}\` from this issue thread." >/dev/null

if ! wait_for_run issue_comment "$comment_started_at" >/dev/null; then
  die "timed out waiting for issue_comment workflow run for #${issue_number}"
fi
wait_for_comment_count 2 || die "expected exactly two GitClaw assistant comments after follow-up"
all_bodies="$(gitclaw_comment_bodies)"
grep -Fq "$token_b" <<<"$all_bodies" || die "second assistant comment did not include expected follow-up token ${token_b}"
grep -Fq "$token_a" <<<"$all_bodies" || die "assistant comments do not preserve prior conversation token ${token_a}"
echo "e2e: follow-up response verified"

if [[ "$expect_backup" == "1" ]]; then
  wait_for_backup || die "expected backup JSON on ${backup_branch} with conversation and tool evidence"
  echo "e2e: git-backed backup verified"
fi

sleep 15
final_count="$(count_gitclaw_comments)"
if [[ "$final_count" != "2" ]]; then
  die "bot loop suspected: expected 2 GitClaw comments after quiet period, got ${final_count}"
fi
echo "e2e: bot-loop prevention verified"
