#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel handoff creates a session handoff issue and proves model/tool follow-up.
set -euo pipefail

log() {
  echo "channel-session-handoff-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-session-handoff-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel session handoff slash E2E appears to be running: ${lock_dir}"
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

sha256_12() {
  printf "%s" "$1" | shasum -a 256 | awk '{print substr($1, 1, 12)}'
}

ensure_label gitclaw 5319e7 "Handled by GitClaw"
ensure_label gitclaw:channel 1d76db "GitClaw mirrored channel thread"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ | tr '[:upper:]' '[:lower:]')"
channel="telegram"
thread_id="channel-session-handoff-e2e-${timestamp}"
ingest_message_id="session-handoff-ingest-${timestamp}"
handoff_id="channel-session-handoff-${timestamp}"
notify_message_id="session-handoff-notify-${timestamp}"
account_id="telegram-session-handoff-account-NOECHO_CHANNEL_SESSION_HANDOFF_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_SESSION_HANDOFF_INGEST_${timestamp}"
source_hidden_token="NOECHO_CHANNEL_SESSION_HANDOFF_SOURCE_${timestamp}"
handoff_hidden_token="NOECHO_CHANNEL_SESSION_HANDOFF_COMMAND_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_SESSION_HANDOFF_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_SESSION_HANDOFF_FOLLOWUP_${timestamp}"
source_model_hidden_token="NOECHO_CHANNEL_SESSION_HANDOFF_SOURCE_MODEL_${timestamp}"
expected_source_token="GITCLAW_CHANNEL_SESSION_HANDOFF_SOURCE_CONTEXT_V1"
expected_token="GITCLAW_CHANNEL_SESSION_HANDOFF_CONTEXT_V1"
source_search_phrase="channel session handoff source unique search fixture phrase"
handoff_search_phrase="channel session handoff unique search fixture phrase"
notify_message_hash="$(sha256_12 "$notify_message_id")"
issue_number=""
handoff_issue_number=""
issue_title="GitClaw ${channel} thread ${thread_id}"

run_list_json() {
  local workflow="$1"
  local event="$2"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event "$event" \
    --limit 40 \
    --json databaseId,status,conclusion,createdAt,url,displayTitle \
    --jq '.'
}

wait_for_workflow_run() {
  local workflow="$1"
  local event="$2"
  local started_at="$3"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json "$workflow" "$event" | jq -c --arg started "$started_at" '[.[] | select(.createdAt >= $started)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${workflow} ${event} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

wait_for_issue_comment_run_for_title() {
  local started_at="$1"
  local title="$2"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json "$main_workflow" "issue_comment" | jq -c --arg started "$started_at" --arg title "$title" '[.[] | select(.createdAt >= $started and .displayTitle == $title)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "issue_comment run for ${title} failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

find_issue_number() {
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:channel \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg thread "$thread_id" '.[] | select((.title | contains($thread)) or (.body | contains($thread))) | .number' \
    | head -n 1
}

wait_for_issue_number() {
  for _ in {1..30}; do
    local number
    number="$(find_issue_number)"
    if [[ -n "$number" && "$number" != "null" ]]; then
      echo "$number"
      return 0
    fi
    sleep 2
  done
  return 1
}

assistant_count_for_issue() {
  local number="$1"
  gh issue view "$number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

error_count_for_issue() {
  local number="$1"
  gh issue view "$number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("<!-- gitclaw:error"))] | length'
}

wait_for_assistant_count_for_issue() {
  local number="$1"
  local want="$2"
  for _ in {1..120}; do
    local errors
    errors="$(error_count_for_issue "$number")"
    if [[ "$errors" != "0" ]]; then
      die "issue #${number} posted ${errors} error marker comment(s)"
    fi
    local got
    got="$(assistant_count_for_issue "$number")"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

latest_assistant_comment_for_issue() {
  local number="$1"
  gh issue view "$number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

handoff_notification_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$notify_message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

cleanup() {
  for number in "$issue_number" "$handoff_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel session handoff slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel session handoff slash E2E.

Hidden ingest token: ${ingest_hidden_token}"

ingest_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$ingest_workflow" \
  --repo "$repo" \
  -f channel="$channel" \
  -f thread_id="$thread_id" \
  -f message_id="$ingest_message_id" \
  -f author="telegram:e2e" \
  -f body="$ingest_body"

wait_for_workflow_run "$ingest_workflow" "workflow_dispatch" "$ingest_started_at" >/dev/null || die "timed out waiting for channel-ingest workflow"
issue_number="$(wait_for_issue_number)" || die "timed out finding channel issue for ${thread_id}"
log "channel ingest created issue #${issue_number}"

wait_for_workflow_run "$main_workflow" "workflow_dispatch" "$ingest_started_at" >/dev/null || die "timed out waiting for initial channel report workflow"
wait_for_assistant_count_for_issue "$issue_number" 1 || die "expected initial channel report"
initial_report="$(latest_assistant_comment_for_issue "$issue_number")"
grep -Fq "GitClaw Channel Report" <<<"$initial_report" || die "initial assistant comment missing channel report"
grep -Fq 'channel_thread_issue: `true`' <<<"$initial_report" || die "initial channel report missing channel thread status"
if grep -Fq "$ingest_hidden_token" <<<"$initial_report"; then
  die "initial channel report leaked ingest hidden token"
fi

source_model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Use the repo-reader skill.

Search the repository for \`${source_search_phrase}\`.
The matching repository search result line has the form \`${source_search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_SESSION_HANDOFF_SOURCE_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include thread ids, message ids, account hashes, issue numbers, or source bodies.
Do not include this hidden source model token: ${source_model_hidden_token}
Keep the answer under 30 words." >/dev/null

wait_for_issue_comment_run_for_title "$source_model_started_at" "$issue_title" >/dev/null || die "timed out waiting for source channel model follow-up"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected source model-backed assistant comment"
source_model_comment="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "$expected_source_token" \
  'model="openai/' \
  'prompt_context_sha256_12="' \
  'skills="repo-reader"' \
  'gitclaw.search_files' \
  'usage_total_tokens="'; do
  grep -Fq "$expected" <<<"$source_model_comment" || die "source channel model comment missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$source_model_hidden_token" "$account_id" "$thread_id" "$ingest_message_id"; do
  if grep -Fq "$leaked" <<<"$source_model_comment"; then
    die "source channel model comment leaked ${leaked}"
  fi
done

handoff_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels handoff --id ${handoff_id} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id}

Please hand off this channel-origin session.
Do not include this hidden handoff token: ${handoff_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$handoff_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel session handoff action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected channel session handoff action receipt"
handoff_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Session Handoff Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels handoff\`" \
  "channel_session_handoff_status: \`created\`" \
  "handoff_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "handoff_mode: \`github-issue-conversation\`" \
  "source_session_store: \`github-issue-thread\`" \
  "model_backed_assistant_turns: \`1\`" \
  "deterministic_assistant_turns: \`1\`" \
  "prompt_visible_skill_names: \`repo-reader\`" \
  "gitclaw.search_files" \
  "next_issue_comment_resumes_handoff: \`true\`" \
  "server_required: \`false\`" \
  "socket_required: \`false\`" \
  "external_session_db_required: \`false\`" \
  "model_call_performed: \`false\`" \
  "raw_handoff_id_included: \`false\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "raw_issue_bodies_included: \`false\`" \
  "raw_comment_bodies_included: \`false\`" \
  "raw_assistant_replies_included: \`false\`" \
  "raw_prompts_included: \`false\`" \
  "raw_tool_outputs_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "llm_e2e_required_after_channel_session_handoff_change: \`true\`"; do
  grep -Fq "$expected" <<<"$handoff_receipt" || die "channel session handoff receipt missing ${expected}"
done
for leaked in "$handoff_id" "$ingest_hidden_token" "$source_hidden_token" "$handoff_hidden_token" "$source_model_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$expected_source_token"; do
  if grep -Fq "$leaked" <<<"$handoff_receipt"; then
    die "channel session handoff receipt leaked ${leaked}"
  fi
done

handoff_issue_number="$(sed -n 's/.*handoff_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$handoff_receipt" | head -n 1)"
[[ -n "$handoff_issue_number" && "$handoff_issue_number" != "null" ]] || die "could not resolve session handoff issue number"
log "channel session handoff created handoff issue #${handoff_issue_number}"

[[ "$(handoff_notification_count)" == "1" ]] || die "channel session handoff did not queue exactly one handoff-link outbound comment"
handoff_json="$(gh issue view "$handoff_issue_number" --repo "$repo" --json title,body,labels)"
handoff_title="$(jq -r '.title' <<<"$handoff_json")"
handoff_body="$(jq -r '.body' <<<"$handoff_json")"
handoff_labels="$(jq -r '.labels[].name' <<<"$handoff_json")"
[[ "$handoff_title" == "GitClaw session handoff: issue #${issue_number} (${handoff_id})" ]] || die "unexpected session handoff issue title: ${handoff_title}"
grep -Fxq "gitclaw" <<<"$handoff_labels" || die "handoff issue missing gitclaw label"
for expected in \
  "gitclaw:session-handoff-issue" \
  "id=\"${handoff_id}\"" \
  "GitClaw session handoff issue" \
  "- handoff_id: ${handoff_id}" \
  "- handoff_mode: github-issue-conversation" \
  "- source_issue: #${issue_number}" \
  "- source_issue_url: https://github.com/${repo}/issues/${issue_number}" \
  "- source_kind: channel_comment" \
  "- source_session_store: github-issue-thread" \
  "- assistant_turn_comments: 2" \
  "- assistant_turns_with_prompt_provenance: 2" \
  "- model_backed_assistant_turns: 1" \
  "- deterministic_assistant_turns: 1" \
  "- prompt_visible_skill_names: repo-reader" \
  "gitclaw.search_files" \
  "- usage_total_tokens:" \
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
for leaked in "$ingest_hidden_token" "$source_hidden_token" "$handoff_hidden_token" "$source_model_hidden_token" "$expected_source_token"; do
  if grep -Fq "$leaked" <<<"$handoff_body"; then
    die "handoff issue body leaked ${leaked}"
  fi
done

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
outbound_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound"))] | join("\n")' <<<"$issue_json")"
grep -Fq "gitclaw:channel-outbound" <<<"$outbound_bodies" || die "channel issue missing session handoff outbound marker"
grep -Fq "GitClaw channel session handoff" <<<"$outbound_bodies" || die "channel issue missing visible session handoff notification"
grep -Fq "Handoff issue: #${handoff_issue_number}" <<<"$outbound_bodies" || die "channel issue missing handoff issue number in notification"
grep -Fq "Model-backed turns: 1" <<<"$outbound_bodies" || die "channel handoff notification missing model-backed turn count"
if grep -Fq "$handoff_hidden_token" <<<"$outbound_bodies"; then
  die "channel session handoff notification leaked source token"
fi

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing session handoff outbound hash ${notify_message_hash}"
for leaked in "$handoff_id" "$handoff_hidden_token" "$account_id" "$ingest_hidden_token" "$source_model_hidden_token"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels handoff --id ${handoff_id} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id}

Repeat the same channel session handoff.
Do not leak duplicate hidden token: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel session handoff action"
wait_for_assistant_count_for_issue "$issue_number" 4 || die "expected duplicate channel session handoff receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Session Handoff Action" \
  "requested_channel_command: \`/channels handoff\`" \
  "channel_session_handoff_status: \`duplicate\`" \
  "handoff_issue: \`#${handoff_issue_number}\`" \
  "handoff_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel session handoff receipt missing ${expected}"
done
[[ "$(handoff_notification_count)" == "1" ]] || die "duplicate channel session handoff queued another handoff-link notification"
for leaked in "$duplicate_hidden_token" "$handoff_id" "$thread_id" "$ingest_message_id" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel session handoff receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$handoff_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel-created session handoff and use the repo-reader skill.

Search the repository for \`${handoff_search_phrase}\`.
The matching repository search result line has the form \`${handoff_search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_SESSION_HANDOFF_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include handoff ids, message ids, thread ids, account hashes, issue numbers, or source bodies.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$handoff_title")" || die "timed out waiting for channel session handoff model follow-up"
wait_for_assistant_count_for_issue "$handoff_issue_number" 1 || die "expected model-backed channel session handoff follow-up"
model_comment="$(latest_assistant_comment_for_issue "$handoff_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel session handoff search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel session handoff follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel session handoff follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel session handoff follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel session handoff follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel session handoff follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel session handoff follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$source_hidden_token" "$handoff_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$account_id" "$handoff_id" "$notify_message_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel session handoff follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number}, handoff issue #${handoff_issue_number} (model follow-up: ${model_url})"
