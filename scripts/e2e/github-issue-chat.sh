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

ensure_label() {
  local name="$1"
  local color="$2"
  local description="$3"
  gh label create "$name" --repo "$GITCLAW_E2E_REPO" --color "$color" --description "$description" --force >/dev/null
}

ensure_label gitclaw 0e8a16 "Handled by GitClaw"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:disabled 5319e7 "GitClaw should ignore this issue"
ensure_label gitclaw:e2e-prompt-artifact 1f883d "Upload GitClaw prompt artifact during E2E"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

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
followup_search_token="GITCLAW_ISSUE_CHAT_FOLLOWUP_CONTEXT_V1"
followup_search_phrase="gclaw-issue-chat-followup-needle"
body="Live E2E conversation check.

Please use the repository file \`go.mod\`.
Please use the repo-reader skill.
Search the repository for \`${search_phrase}\` and include the associated token from the matching line.
The repository search token starts with \`GITCLAW_SEARCH_\`; it is different from the conversation tokens below.
Reply with the exact token \`${token_a}\`.
Also state the Go module path from \`go.mod\`.
Also include the exact durable memory token from \`.gitclaw/MEMORY.md\`.
Also include the exact skill verification token from the repo-reader skill.
Use exactly these labels: search, conversation, module, memory, skill.
Keep the answer under 80 words."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$GITCLAW_E2E_REPO" \
  --title "$title" \
  --body "$body" \
  --label gitclaw \
  --label gitclaw:e2e-prompt-artifact)"
issue_number="${issue_url##*/}"

cleanup() {
  status=$?
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$GITCLAW_E2E_REPO" --add-label "gitclaw:disabled" >/dev/null 2>&1 || true
    gh issue edit "$issue_number" --repo "$GITCLAW_E2E_REPO" --add-label "$retention_label" >/dev/null 2>&1 || true
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
      gh run watch "$run_id" --repo "$GITCLAW_E2E_REPO" --exit-status >&2
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

latest_gitclaw_comment_body() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

issue_label_names() {
  gh issue view "$issue_number" \
    --repo "$GITCLAW_E2E_REPO" \
    --json labels \
    --jq '.labels[].name'
}

wait_for_done_status() {
  local deadline=$((SECONDS + comment_deadline_seconds))
  while (( SECONDS < deadline )); do
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
  python3 - "$file" "$token_a" "$token_b" "$module_path" "$memory_token" "$skill_token" "$search_token" "$followup_search_token" <<'PY'
import json
import sys

path, token_a, token_b, module_path, memory_token, skill_token, search_token, followup_search_token = sys.argv[1:9]
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
for value in (token_a, token_b, module_path, memory_token, skill_token, search_token, followup_search_token):
    if value not in body:
        errors.append(f"missing {value}")
if errors:
    raise SystemExit("; ".join(errors))
PY
}

assert_prompt_artifact() {
  local run_id="$1"
  local artifact_name="gitclaw-issue-${issue_number}-run-${run_id}-prompt"
  local tmp
  tmp="$(mktemp -d)"
  gh run download "$run_id" \
    --repo "$GITCLAW_E2E_REPO" \
    --name "$artifact_name" \
    --dir "$tmp" >/dev/null
  grep -Fq "$search_token" "$tmp/prompt.md" ||
    die "prompt artifact did not include search_files token ${search_token}"
  grep -Fq "[tool_output name=gitclaw.search_files" "$tmp/prompt.md" ||
    die "prompt artifact did not include gitclaw.search_files output"
  rm -rf "$tmp"
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

wait_for_assistant_count() {
  wait_for_comment_count "$1"
}

if ! issue_run_id="$(wait_for_run issues "$issue_started_at")"; then
  die "timed out waiting for issues workflow run for #${issue_number}"
fi
wait_for_comment_count 1 || die "expected one GitClaw assistant comment after issue open"
assert_prompt_artifact "$issue_run_id"
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
Also mention the earlier token \`${token_a}\` from this issue thread.
Use the repo-reader skill and search the repository for \`${followup_search_phrase}\`.
Reply with the exact token after the arrow from the matching gitclaw.search_files result line.
Do not use @file or @folder references." >/dev/null

if ! wait_for_run issue_comment "$comment_started_at" >/dev/null; then
  die "timed out waiting for issue_comment workflow run for #${issue_number}"
fi
wait_for_assistant_count 2 || die "expected exactly two GitClaw assistant comments after follow-up"
all_bodies="$(gitclaw_comment_bodies)"
latest_body="$(latest_gitclaw_comment_body)"
grep -Fq "$token_b" <<<"$all_bodies" || die "second assistant comment did not include expected follow-up token ${token_b}"
grep -Fq "$token_a" <<<"$all_bodies" || die "assistant comments do not preserve prior conversation token ${token_a}"
grep -Fq "$followup_search_token" <<<"$latest_body" || die "second assistant comment did not include follow-up search token ${followup_search_token}"
grep -Fq 'prompt_context_sha256_12="' <<<"$latest_body" || die "second assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$latest_body" || die "second assistant marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$latest_body" || die "second assistant marker missing prompt-visible tool list"
grep -Fq 'gitclaw.search_files' <<<"$latest_body" || die "second assistant marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$latest_body" || die "second assistant marker missing usage token telemetry"
if grep -Fq 'gitclaw.read_file' <<<"$latest_body"; then
  die "second assistant should recover the follow-up fixture through search_files, not read_file"
fi
echo "e2e: follow-up response verified"

if [[ "$expect_backup" == "1" ]]; then
  wait_for_backup || die "expected backup JSON on ${backup_branch} with conversation and tool evidence"
  echo "e2e: git-backed backup verified"
fi

wait_for_done_status || die "expected final status label gitclaw:done without running/error"
echo "e2e: status labels verified"

sleep 15
final_count="$(count_gitclaw_comments)"
if [[ "$final_count" != "2" ]]; then
  die "bot loop suspected: expected 2 GitClaw comments after quiet period, got ${final_count}"
fi
echo "e2e: bot-loop prevention verified"
