#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "session-risk-report-e2e: $*" >&2
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
hidden_token="GITCLAW_SESSION_RISK_HIDDEN_${timestamp}"
comment_token="GITCLAW_SESSION_RISK_COMMENT_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw session risk e2e ${timestamp}"
body="Live session-risk E2E.

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
      gh issue close "$issue_number" --repo "$repo" --comment "session-risk-report e2e cleanup" >/dev/null 2>&1 || true
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
  --body "@gitclaw /session risk

Please audit assistant-turn session risk metadata.
Hidden comment token: ${comment_token}" >/dev/null

session_run_json="$(wait_for_run issue_comment "$comment_started_at")" || die "timed out waiting for issue_comment workflow run"
wait_for_assistant_count 2 || die "expected session risk report as second assistant comment"
session_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/session"' \
  "GitClaw Session Risk Report" \
  "Generated without a model call" \
  'session_risk_status: `ok`' \
  'verification_scope: `github_issue_session_provenance`' \
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
  'prompt_visible_skill_names: `repo-reader`' \
  'heartbeat_comments: `0`' \
  'error_marker_comments: `0`' \
  'channel_message_comments: `0`' \
  'channel_thread_issue: `false`' \
  'proactive_run_issue: `false`' \
  'session_risk_findings: `0`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_assistant_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'raw_search_queries_included: `false`' \
  'credential_values_included: `false`' \
  'llm_e2e_required_after_session_risk_change: `true`' \
  "### Transcript Trust Risk Card" \
  'kind=`session-transcript` transcript_messages=`3`' \
  "### Assistant Provenance Risk Card" \
  'kind=`session-provenance` assistant_turn_comments=`1`' \
  "### Marker Risk Card" \
  'kind=`session-marker` heartbeat_comments=`0` error_marker_comments=`0` channel_message_comments=`0`' \
  "### Current Session Request Risk Card" \
  'current_issue_session_request=`true`' \
  'issue_body_scanned=`false`' \
  'comment_bodies_scanned=`false`' \
  "### Risk Findings" \
  "- none"; do
  grep -Fq -- "$expected" <<<"$session_comment" || die "session risk report missing ${expected}"
done

grep -Fq 'prompt_visible_tool_names: `' <<<"$session_comment" || die "session risk report missing prompt-visible tool names"
grep -Fq 'gitclaw.search_files' <<<"$session_comment" || die "session risk report missing search_files provenance"

for leaked in "$hidden_token" "$comment_token" "$search_phrase" "Please audit assistant-turn session risk metadata"; do
  if grep -Fq "$leaked" <<<"$session_comment"; then
    die "session risk report leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
model_url="$(jq -r '.url' <<<"$model_run_json")"
session_url="$(jq -r '.url' <<<"$session_run_json")"
log "passed for issue #${issue_number}: ${session_url} (initial model run: ${model_url})"
