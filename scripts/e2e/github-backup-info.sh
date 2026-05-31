#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "backup-info-e2e: $*" >&2
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
token="NOECHO_BACKUP_INFO_${timestamp}"
followup_hidden_token="NOECHO_BACKUP_INFO_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_BACKUP_INFO_CONTEXT_V1"
search_phrase="backup info unique search fixture phrase"
title="@gitclaw /backup info e2e ${timestamp}"
body="Live backup info E2E.

Hidden backup info token: ${token}
This should produce a deterministic backup intent report, then the fetched backup branch should render a focused body-free backup info report for this issue."

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
      gh issue close "$issue_number" --repo "$repo" --comment "backup-info e2e cleanup" >/dev/null 2>&1 || true
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

run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one backup report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/backup"' \
  "GitClaw Backup Report" \
  "Generated without a model call" \
  'requested_backup_command: `info`' \
  'backup_command_status: `ok`' \
  'issue_side_execution: `deferred_to_post_turn_backup_branch`' \
  'raw_bodies_included: `false`' \
  "requested_local_command: \`gitclaw backup info --root .gitclaw/backups --repo ${repo} --issue ${issue_number}\`" \
  'llm_e2e_required_after_backup_info_change: `true`' \
  'backup_branch: `gitclaw-backups`' \
  'backup_schema_version: `1`'; do
  grep -Fq "$expected" <<<"$comments" || die "backup report missing ${expected}"
done

if grep -Fq "$expected_token" <<<"$comments" || grep -Fq "$search_phrase" <<<"$comments"; then
  die "backup info report leaked follow-up fixture context"
fi

for leaked in "$token" "Live backup info E2E" "Hidden backup info token" "focused body-free backup info report"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "backup report leaked issue-side info input ${leaked}"
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
    issue_json_path="${backup_checkout}/.gitclaw/backups/${repo_key}/${issue_path}"
  else
    sleep 5
    continue
  fi
  if [[ -f "$index_path" && -f "$issue_json_path" ]] &&
    jq -e --argjson number "$issue_number" --arg title "$title" --arg path "$issue_path" '
      any(.issues[]; .number == $number and .title == $title and .path == $path)
    ' "$index_path" >/dev/null; then
    label_count="$(jq -r '.issue.labels | length' "$issue_json_path")"
    comment_count="$(jq -r '.comments | length' "$issue_json_path")"
    transcript_count="$(jq -r '.transcript | length' "$issue_json_path")"
    user_count="$(jq -r '[.transcript[] | select(.Role != "assistant")] | length' "$issue_json_path")"
    assistant_count_json="$(jq -r '[.transcript[] | select(.Role == "assistant")] | length' "$issue_json_path")"
    assistant_turn_count="$(jq -r '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length' "$issue_json_path")"
    error_count_json="$(jq -r '[.comments[] | select(.body | contains("gitclaw:error"))] | length' "$issue_json_path")"

    info_output="$(go run ./cmd/gitclaw backup info --root "${backup_checkout}/.gitclaw/backups" --repo "$repo" --issue "$issue_number")"
    for expected in \
      "GitClaw Backup Info Report" \
      "repository: \`${repo}\`" \
      'backup_info_status: `ok`' \
      'backup_verify_status: `ok`' \
      'verification_failures: `0`' \
      'backup_schema_version: `1`' \
      "issue: \`#${issue_number}\`" \
      "issue_backup_path: \`${issue_path}\`" \
      'backup_event_name: `issues`' \
      'payload_bytes: `' \
      'payload_sha256_12:' \
      "labels: \`${label_count}\`" \
      "comments: \`${comment_count}\`" \
      "transcript_messages: \`${transcript_count}\`" \
      "user_messages: \`${user_count}\`" \
      "assistant_messages: \`${assistant_count_json}\`" \
      "assistant_turn_comments: \`${assistant_turn_count}\`" \
      "error_comments: \`${error_count_json}\`" \
      'issue_title_sha256_12:' \
      'issue_body_sha256_12:' \
      'raw_bodies_included: `false`' \
      'llm_e2e_required_after_backup_info_change: `true`' \
      'comment_1_sha256_12:' \
      'message_1_sha256_12:'; do
      grep -Fq "$expected" <<<"$info_output" || die "backup info missing ${expected}"
    done
    for leaked in "$token" "$title"; do
      if grep -Fq "$leaked" <<<"$info_output"; then
        die "backup info leaked ${leaked}"
      fi
    done
    url="$(jq -r '.url' <<<"$run_json")"
    break
  fi
  sleep 5
done

[[ -n "${url:-}" ]] || die "backup info did not observe issue #${issue_number} in ${backup_branch}"

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
