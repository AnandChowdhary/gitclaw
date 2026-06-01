#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "session-resume-e2e: $*" >&2
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
hidden_token="GITCLAW_SESSION_RESUME_HIDDEN_${timestamp}"
comment_token="GITCLAW_SESSION_RESUME_COMMENT_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw session resume e2e ${timestamp}"
body="Live session resume E2E.

Search the repository for \`${search_phrase}\`.
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
      gh issue close "$issue_number" --repo "$repo" --comment "session-resume e2e cleanup" >/dev/null 2>&1 || true
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

run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one model-backed assistant comment"
first_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$first_comment" || die "assistant did not include search_files token ${expected_token}"
grep -Fq 'model="openai/' <<<"$first_comment" || die "assistant marker did not use an OpenAI GitHub Models model"
grep -Fq 'run_id="' <<<"$first_comment" || die "assistant marker missing run id"
grep -Fq 'event_id="' <<<"$first_comment" || die "assistant marker missing event id"
grep -Fq 'idempotency_key="' <<<"$first_comment" || die "assistant marker missing idempotency key"
grep -Fq 'run_url="https://github.com/' <<<"$first_comment" || die "assistant marker missing run url"
grep -Fq 'prompt_context_sha256_12="' <<<"$first_comment" || die "assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$first_comment" || die "assistant marker missing repo-reader skill"
grep -Fq 'gitclaw.search_files' <<<"$first_comment" || die "assistant marker did not include search_files"
grep -Fq 'usage_total_tokens="' <<<"$first_comment" || die "assistant marker missing total token telemetry"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /session resume

Please show the body-free session resume readiness for this conversation.
Hidden comment token: ${comment_token}" >/dev/null

resume_run_json="$(wait_for_run issue_comment "$comment_started_at")" || die "timed out waiting for issue_comment workflow run"
wait_for_assistant_count 2 || die "expected session resume report as second assistant comment"
resume_comment="$(latest_assistant_comment)"
resume_report_body="$(sed '1{/^<!-- gitclaw:assistant-turn /d;}' <<<"$resume_comment")"

for expected in \
  'model="gitclaw/session"' \
  "GitClaw Session Resume Report" \
  "Generated without a model call" \
  'scope: `issue-thread`' \
  'event_kind: `issue_comment`' \
  'active_command: `/session resume`' \
  'session_resume_status: `ok`' \
  'resume_strategy: `github-issue-comment-continuation`' \
  'resume_key_sha256_12: `' \
  'session_store: `github-issue-thread`' \
  'raw_comments: `2`' \
  'transcript_messages: `3`' \
  'user_messages: `2`' \
  'assistant_messages: `1`' \
  'trigger_label_present: `true`' \
  'error_label_present: `false`' \
  'disabled_label_present: `false`' \
  'latest_user_message_present: `true`' \
  'latest_assistant_message_present: `true`' \
  'assistant_turn_comments: `1`' \
  'assistant_turns_with_prompt_provenance: `1`' \
  'assistant_turns_missing_prompt_provenance: `0`' \
  'unique_prompt_context_hashes: `1`' \
  'model_backed_assistant_turns: `1`' \
  'deterministic_assistant_turns: `0`' \
  'model_names: `openai/' \
  'prompt_visible_skill_names: `repo-reader`' \
  'prompt_visible_tool_names: `gitclaw.list_files, gitclaw.skill_index, gitclaw.search_files`' \
  'latest_assistant_model: `openai/' \
  'latest_assistant_prompt_context_sha256_12: `' \
  'usage_bearing_assistant_turns: `1`' \
  'usage_prompt_tokens:' \
  'usage_completion_tokens:' \
  'usage_total_tokens:' \
  'usage_cache_read_tokens:' \
  'usage_cache_write_tokens:' \
  'next_issue_comment_resumes_session: `true`' \
  'github_actions_reentry_supported: `true`' \
  'workflow_event: `issue_comment`' \
  'workflow_dispatch_required: `false`' \
  'server_required: `false`' \
  'socket_required: `false`' \
  'external_session_db_required: `false`' \
  'backup_branch_replay_preferred: `true`' \
  'issue_thread_canonical_storage: `true`' \
  'channel_thread_issue: `false`' \
  'proactive_run_issue: `false`' \
  'raw_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_assistant_replies_included: `false`' \
  'raw_prompts_included: `false`' \
  'raw_provider_responses_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'raw_search_queries_included: `false`' \
  'repository_mutation_allowed: `false`' \
  'resume_mutation_allowed: `false`' \
  'llm_e2e_required_after_session_resume_change: `true`' \
  "### Resume Anchors" \
  'kind=`session-key` source=`github-issue`' \
  'kind=`latest-user` source=`comment:' \
  'kind=`latest-assistant` source=`comment:' \
  'kind=`latest-model-turn` source=`comment:' \
  'prompt_context_sha256_12=`' \
  'skills=`repo-reader` tools=`gitclaw.list_files, gitclaw.skill_index, gitclaw.search_files` usage_present=`true`' \
  'body_included=`false` identifier_included=`false`' \
  "### Latest Assistant Marker" \
  'model=`openai/' \
  'context_documents=`' \
  'selected_skills=`1`' \
  'tool_outputs=`' \
  "### Resume Gates" \
  'continuation_gate=`pass`' \
  'issue_label_gate=`pass`' \
  'latest_user_gate=`pass`' \
  'latest_assistant_gate=`pass`' \
  'model_backed_gate=`pass`' \
  'prompt_provenance_gate=`pass`' \
  'reentry_gate=`issue-comment-action`' \
  'external_session_db_gate=`disabled`' \
  'raw_body_gate=`hashes-counts-and-marker-attributes-only`' \
  'mutation_gate=`disabled`'; do
  grep -Fq "$expected" <<<"$resume_comment" || die "session resume report missing ${expected}"
done

for leaked in "$hidden_token" "$comment_token" "$search_phrase" "Please show the body-free session resume readiness" "https://github.com/${repo}/actions/runs/"; do
  if grep -Fq "$leaked" <<<"$resume_report_body"; then
    die "session resume report leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
url="$(jq -r '.url' <<<"$resume_run_json")"
first_url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url} (initial model run: ${first_url})"
