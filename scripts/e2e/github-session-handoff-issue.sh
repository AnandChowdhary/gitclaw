#!/usr/bin/env bash
# gitclaw-doctor-live-issue: session handoff opens a GitHub issue lane, then proves GitHub Models follow-up.
set -euo pipefail

log() {
  echo "session-handoff-issue-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-session-handoff-issue-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another session-handoff issue E2E appears to be running: ${lock_dir}"
fi

if [[ -z "$repo" ]]; then
  repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
fi
if [[ -z "${GH_TOKEN:-}" && -z "${GITHUB_TOKEN:-}" ]]; then
  export GH_TOKEN="$(gh auth token)"
fi
if [[ -z "${GITHUB_TOKEN:-}" && -n "${GH_TOKEN:-}" ]]; then
  export GITHUB_TOKEN="$GH_TOKEN"
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
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
source_hidden_token="NOECHO_SESSION_HANDOFF_SOURCE_${timestamp}"
handoff_command_token="NOECHO_SESSION_HANDOFF_COMMAND_${timestamp}"
duplicate_hidden_token="NOECHO_SESSION_HANDOFF_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_SESSION_HANDOFF_FOLLOWUP_${timestamp}"
source_expected_token="GITCLAW_SESSION_HANDOFF_SOURCE_CONTEXT_V1"
handoff_expected_token="GITCLAW_SESSION_HANDOFF_CONTEXT_V1"
source_search_phrase="session handoff source unique search fixture phrase"
handoff_search_phrase="session handoff unique search fixture phrase"
handoff_id="handoff-${timestamp,,}"
source_title="GitClaw session handoff E2E ${timestamp}"
source_issue_number=""
handoff_issue_number=""

run_list_json() {
  local event="$1"
  local started_at="$2"
  gh run list \
    --repo "$repo" \
    --workflow "$main_workflow" \
    --event "$event" \
    --created ">=$started_at" \
    --limit 30 \
    --json databaseId,status,conclusion,createdAt,url,displayTitle \
    --jq '.'
}

wait_for_main_run() {
  local event="$1"
  local started_at="$2"
  local title="$3"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json "$event" "$started_at" | jq -c --arg title "$title" '[.[] | select(.displayTitle == $title)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${event} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

assistant_count_for_issue() {
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

error_count_for_issue() {
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("<!-- gitclaw:error"))] | length'
}

wait_for_assistant_count_for_issue() {
  local issue_number="$1"
  local want="$2"
  for _ in {1..90}; do
    local errors
    errors="$(error_count_for_issue "$issue_number")"
    if [[ "$errors" != "0" ]]; then
      die "issue #${issue_number} posted ${errors} error marker comment(s)"
    fi
    local got
    got="$(assistant_count_for_issue "$issue_number")"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

wait_for_assistant_count() {
  local want="$1"
  wait_for_assistant_count_for_issue "$handoff_issue_number" "$want"
}

latest_assistant_comment_for_issue() {
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

cleanup() {
  local numbers=("$source_issue_number" "$handoff_issue_number")
  for number in "${numbers[@]}"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "session-handoff issue e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

opened_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
source_issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$source_title" \
  --label gitclaw \
  --body "@gitclaw Use repo-reader skill.

Search the repository for \`${source_search_phrase}\`.
Reply with the exact GITCLAW_SESSION_HANDOFF_SOURCE token from the matching repository search result line.
Do not include this hidden issue token: ${source_hidden_token}
Keep the answer under 30 words.")"
source_issue_number="${source_issue_url##*/}"
log "created source issue #${source_issue_number}: ${source_issue_url}"

wait_for_main_run "issues" "$opened_started_at" "$source_title" >/dev/null || die "timed out waiting for source issue model run"
wait_for_assistant_count_for_issue "$source_issue_number" 1 || die "expected one model-backed assistant comment on source issue"
source_model_comment="$(latest_assistant_comment_for_issue "$source_issue_number")"

for expected in \
  "$source_expected_token" \
  'model="openai/' \
  'prompt_context_sha256_12="' \
  'skills="repo-reader"' \
  'gitclaw.search_files' \
  'usage_total_tokens="'; do
  grep -Fq "$expected" <<<"$source_model_comment" || die "source model comment missing ${expected}"
done
if grep -Fq "$source_hidden_token" <<<"$source_model_comment"; then
  die "source model comment leaked hidden source token"
fi

handoff_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /session handoff --id ${handoff_id}

Open a fresh body-free session lane.
Do not leak this command token: ${handoff_command_token}" >/dev/null

wait_for_main_run "issue_comment" "$handoff_started_at" "$source_title" >/dev/null || die "timed out waiting for session handoff issue run"
wait_for_assistant_count_for_issue "$source_issue_number" 2 || die "expected session handoff receipt as second source assistant comment"
handoff_receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"

for expected in \
  "GitClaw Session Handoff Issue Action" \
  "Generated without a model call" \
  'model="gitclaw/session"' \
  "requested_session_command: \`/session handoff\`" \
  "session_handoff_status: \`created\`" \
  "handoff_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "handoff_issue_labeled_for_gitclaw: \`true\`" \
  "handoff_mode: \`github-issue-conversation\`" \
  "transcript_messages: \`3\`" \
  "assistant_turn_comments: \`1\`" \
  "assistant_turns_with_prompt_provenance: \`1\`" \
  "model_backed_assistant_turns: \`1\`" \
  "prompt_visible_skill_names: \`repo-reader\`" \
  "gitclaw.search_files" \
  "usage_total_tokens:" \
  "next_issue_comment_resumes_handoff: \`true\`" \
  "workflow_event: \`issue_comment\`" \
  "server_required: \`false\`" \
  "socket_required: \`false\`" \
  "external_session_db_required: \`false\`" \
  "model_call_performed: \`false\`" \
  "raw_handoff_id_included: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "raw_issue_bodies_included: \`false\`" \
  "raw_comment_bodies_included: \`false\`" \
  "raw_assistant_replies_included: \`false\`" \
  "raw_prompts_included: \`false\`" \
  "raw_tool_outputs_included: \`false\`" \
  "llm_e2e_required_after_session_handoff_issue_change: \`true\`"; do
  grep -Fq "$expected" <<<"$handoff_receipt" || die "session handoff receipt missing ${expected}"
done
for leaked in "$handoff_id" "$source_hidden_token" "$handoff_command_token" "$source_expected_token"; do
  if grep -Fq "$leaked" <<<"$handoff_receipt"; then
    die "session handoff receipt leaked ${leaked}"
  fi
done

handoff_issue_number="$(sed -n 's/.*handoff_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$handoff_receipt" | head -1)"
[[ -n "$handoff_issue_number" ]] || die "could not parse handoff issue number from receipt"
log "handoff issue #${handoff_issue_number}"

handoff_json="$(gh issue view "$handoff_issue_number" --repo "$repo" --json title,body,labels)"
handoff_title="$(jq -r '.title' <<<"$handoff_json")"
handoff_body="$(jq -r '.body' <<<"$handoff_json")"
handoff_labels="$(jq -r '.labels[].name' <<<"$handoff_json")"

grep -Fxq "gitclaw" <<<"$handoff_labels" || die "handoff issue missing gitclaw label"
for expected in \
  "gitclaw:session-handoff-issue" \
  "id=\"${handoff_id}\"" \
  "GitClaw session handoff issue" \
  "- handoff_id: ${handoff_id}" \
  "- handoff_mode: github-issue-conversation" \
  "- source_issue: #${source_issue_number}" \
  "- source_issue_url: https://github.com/${repo}/issues/${source_issue_number}" \
  "- source_kind: comment" \
  "- source_session_store: github-issue-thread" \
  "- transcript_messages: 3" \
  "- assistant_turn_comments: 1" \
  "- assistant_turns_with_prompt_provenance: 1" \
  "- model_backed_assistant_turns: 1" \
  "- prompt_visible_skill_names: repo-reader" \
  "gitclaw.search_files" \
  "- next_issue_comment_resumes_handoff: true" \
  "- workflow_event: issue_comment" \
  "- server_required: false" \
  "- socket_required: false" \
  "- external_session_db_required: false" \
  "- raw_source_body_included: false" \
  "- raw_comment_bodies_included: false" \
  "- raw_assistant_replies_included: false" \
  "- raw_prompts_included: false" \
  "- raw_tool_outputs_included: false"; do
  grep -Fq -- "$expected" <<<"$handoff_body" || die "handoff issue body missing ${expected}"
done
for leaked in "$source_hidden_token" "$handoff_command_token" "$source_expected_token"; do
  if grep -Fq "$leaked" <<<"$handoff_body"; then
    die "handoff issue body leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /session handoff --id ${handoff_id}

Repeat the same handoff id.
Do not leak this duplicate token: ${duplicate_hidden_token}" >/dev/null

wait_for_main_run "issue_comment" "$duplicate_started_at" "$source_title" >/dev/null || die "timed out waiting for duplicate session handoff run"
wait_for_assistant_count_for_issue "$source_issue_number" 3 || die "expected duplicate handoff receipt as third source assistant comment"
duplicate_receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"

for expected in \
  "session_handoff_status: \`existing\`" \
  "handoff_issue: \`#${handoff_issue_number}\`" \
  "handoff_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "raw_handoff_id_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate session handoff receipt missing ${expected}"
done
for leaked in "$handoff_id" "$duplicate_hidden_token"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate session handoff receipt leaked ${leaked}"
  fi
done

followup_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$handoff_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Use repo-reader skill.

Search the repository for \`${handoff_search_phrase}\`.
Reply with the exact GITCLAW_SESSION_HANDOFF token from the matching repository search result line.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

wait_for_main_run "issue_comment" "$followup_started_at" "$handoff_title" >/dev/null || die "timed out waiting for handoff issue model follow-up"
wait_for_assistant_count 1 || die "expected one model-backed assistant comment on handoff issue"
handoff_model_comment="$(latest_assistant_comment_for_issue "$handoff_issue_number")"

for expected in \
  "$handoff_expected_token" \
  'model="openai/' \
  'prompt_context_sha256_12="' \
  'skills="repo-reader"' \
  'gitclaw.search_files' \
  'usage_total_tokens="'; do
  grep -Fq "$expected" <<<"$handoff_model_comment" || die "handoff model comment missing ${expected}"
done
for leaked in "$followup_hidden_token" "$source_hidden_token" "$handoff_command_token" "$duplicate_hidden_token"; do
  if grep -Fq "$leaked" <<<"$handoff_model_comment"; then
    die "handoff model comment leaked ${leaked}"
  fi
done

log "session handoff issue E2E passed for source #${source_issue_number}, handoff #${handoff_issue_number}"
