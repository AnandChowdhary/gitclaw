#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "session-stats-report-e2e: $*" >&2
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
hidden_token="GITCLAW_SESSION_STATS_HIDDEN_${timestamp}"
comment_token="GITCLAW_SESSION_STATS_COMMENT_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw session stats e2e ${timestamp}"
body="Live session-stats E2E.

Use the repo-reader skill and search the repository for \`${search_phrase}\`.
Reply with only the exact GITCLAW_SEARCH token from the matching repository search result line.
Do not include this hidden issue token: ${hidden_token}
Keep the answer under 30 words."

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
      gh issue close "$issue_number" --repo "$repo" --comment "session-stats-report e2e cleanup" >/dev/null 2>&1 || true
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

model_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one model-backed assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant marker did not prove search_files was prompt-visible"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /session stats

Please summarize assistant-turn session stats.
Hidden comment token: ${comment_token}" >/dev/null

stats_run_json="$(wait_for_run issue_comment "$comment_started_at")" || die "timed out waiting for issue_comment workflow run"
wait_for_assistant_count 2 || die "expected session stats report as second assistant comment"
stats_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/session"' \
  "GitClaw Session Stats Report" \
  "Generated without a model call" \
  'scope: `issue-thread`' \
  'session_stats_status: `ok`' \
  'event_kind: `issue_comment`' \
  'raw_comments: `2`' \
  'transcript_messages: `3`' \
  'user_messages: `2`' \
  'assistant_messages: `1`' \
  'trusted_messages: `3`' \
  'untrusted_messages: `0`' \
  'edited_messages: `0`' \
  'assistant_turn_comments: `1`' \
  'assistant_turns_with_prompt_provenance: `1`' \
  'assistant_turns_missing_prompt_provenance: `0`' \
  'unique_prompt_context_hashes: `1`' \
  'model_backed_assistant_turns: `1`' \
  'deterministic_assistant_turns: `0`' \
  'prompt_visible_skill_count: `1`' \
  'prompt_visible_skill_names: `repo-reader`' \
  'heartbeat_comments: `0`' \
  'error_marker_comments: `0`' \
  'channel_message_comments: `0`' \
  'channel_thread_issue: `false`' \
  'proactive_run_issue: `false`' \
  'raw_bodies_included: `false`' \
  'raw_prompts_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  "### Stats Cards" \
  'kind=`transcript-shape`' \
  'kind=`assistant-provenance`' \
  'kind=`prompt-surface`' \
  'kind=`session-markers`'; do
  grep -Fq -- "$expected" <<<"$stats_comment" || die "session stats report missing ${expected}"
done

if ! grep -Fq 'model_names: `openai/gpt-5-nano`' <<<"$stats_comment" && ! grep -Fq 'model_names: `openai/gpt-4.1-nano`' <<<"$stats_comment"; then
  die "session stats report missing model name"
fi
grep -Fq 'prompt_visible_tool_names: `' <<<"$stats_comment" || die "session stats report missing prompt-visible tool names"
grep -Fq 'gitclaw.search_files' <<<"$stats_comment" || die "session stats report missing search_files provenance"

for leaked in "$hidden_token" "$comment_token" "$search_phrase" "Please summarize assistant-turn session stats"; do
  if grep -Fq "$leaked" <<<"$stats_comment"; then
    die "session stats report leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
model_url="$(jq -r '.url' <<<"$model_run_json")"
stats_url="$(jq -r '.url' <<<"$stats_run_json")"
log "passed for issue #${issue_number}: ${stats_url} (initial model run: ${model_url})"
