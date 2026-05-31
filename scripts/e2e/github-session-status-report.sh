#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "session-status-report-e2e: $*" >&2
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
hidden_token="GITCLAW_SESSION_STATUS_HIDDEN_${timestamp}"
status_hidden_token="GITCLAW_SESSION_STATUS_COMMENT_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_SESSION_STATUS_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw session status seed e2e ${timestamp}"
body="Live session-status E2E.

Use the repo-reader skill and search the repository for \`${search_phrase}\`.
Reply with the exact GITCLAW_SEARCH token from the matching repository search result line.
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
      gh issue close "$issue_number" --repo "$repo" --comment "session-status-report e2e cleanup" >/dev/null 2>&1 || true
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

seed_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for initial issues workflow run"
wait_for_assistant_count 1 || die "expected one model-backed assistant comment"
seed_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$seed_comment" || die "seed assistant did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$seed_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$seed_comment"; then
  die "seed assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$seed_comment" || die "seed assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$seed_comment" || die "seed assistant marker missing repo-reader skill"
grep -Fq 'tools="' <<<"$seed_comment" || die "seed assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$seed_comment" || die "seed assistant marker did not prove search_files was prompt-visible"

if grep -Fq "$hidden_token" <<<"$seed_comment"; then
  die "seed assistant leaked hidden issue token"
fi

status_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /session status

Please show the body-free session status.
Hidden status request token: ${status_hidden_token}" >/dev/null

status_run_json="$(wait_for_run issue_comment "$status_started_at")" || die "timed out waiting for session status issue_comment workflow run"
wait_for_assistant_count 2 || die "expected session status report as second assistant comment"
status_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/session"' \
  "GitClaw Session Status Report" \
  "Generated without a model call" \
  'scope: `issue-thread`' \
  'repository: `'"$repo"'`' \
  'issue: `#'"$issue_number"'`' \
  'event_kind: `issue_comment`' \
  'active_command: `/session status`' \
  'session_status: `ok`' \
  'raw_comments: `2`' \
  'transcript_messages: `3`' \
  'user_messages: `2`' \
  'assistant_messages: `1`' \
  'assistant_turn_comments: `1`' \
  'model_backed_assistant_turns: `1`' \
  'deterministic_assistant_turns: `0`' \
  'model_names: `openai/' \
  'prompt_visible_skill_names: `repo-reader`' \
  'gitclaw.search_files' \
  'latest_user_message_present: `true`' \
  'latest_assistant_message_present: `true`' \
  'latest_assistant_model: `openai/' \
  'latest_assistant_prompt_context_sha256_12: `' \
  'raw_bodies_included: `false`' \
  'raw_prompts_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'llm_e2e_required_after_session_status_change: `true`' \
  "### Latest Message Hashes" \
  'kind=`user` present=`true` source=`comment:' \
  'kind=`assistant` present=`true` source=`comment:' \
  'sha256_12=`' \
  "### Latest Assistant Marker" \
  'prompt_context_sha256_12=`' \
  'skills=`repo-reader`' \
  'kind=`skill` name=`repo-reader` turns=`1`' \
  'kind=`tool` name=`gitclaw.search_files` turns=`1`' \
  "### Status Notes" \
  "backup JSON can replay the same body-free status locally"; do
  grep -Fq -- "$expected" <<<"$status_comment" || die "session status report missing ${expected}"
done

for leaked in "$hidden_token" "$status_hidden_token" "$expected_token" "$search_phrase" "Please show the body-free session status" "Reply with the exact GITCLAW_SEARCH token"; do
  if grep -Fq "$leaked" <<<"$status_comment"; then
    die "session status report leaked ${leaked}"
  fi
done

followup_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_SEARCH token from the matching repository search result line.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

followup_run_json="$(wait_for_run issue_comment "$followup_started_at")" || die "timed out waiting for model follow-up issue_comment workflow run"
wait_for_assistant_count 3 || die "expected model-backed follow-up assistant comment"
followup_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$followup_comment" || die "model follow-up did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$followup_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$followup_comment"; then
  die "model follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$followup_comment" || die "model follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$followup_comment" || die "model follow-up marker missing repo-reader skill"
grep -Fq 'gitclaw.search_files' <<<"$followup_comment" || die "model follow-up marker did not prove search_files was prompt-visible"

if grep -Fq "$followup_hidden_token" <<<"$followup_comment"; then
  die "model follow-up leaked hidden follow-up token"
fi

wait_for_done_status || die "expected gitclaw:done without running/error"

seed_url="$(jq -r '.url' <<<"$seed_run_json")"
status_url="$(jq -r '.url' <<<"$status_run_json")"
followup_url="$(jq -r '.url' <<<"$followup_run_json")"
log "passed for issue #${issue_number}: seed=${seed_url} status=${status_url} followup=${followup_url}"
