#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-profile-status slash action queues provider-visible profile status and proves model/tool follow-up.
set -euo pipefail

log() {
  echo "channel-profile-status-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-profile-status-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-profile-status slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-profile-status-e2e-${timestamp}"
ingest_message_id="profile-status-ingest-${timestamp}"
notify_message_id="profile-status-notify-${timestamp}"
profile_status_id="profile-status-${timestamp}"
account_id="telegram-profile-status-account-NOECHO_CHANNEL_PROFILE_STATUS_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_PROFILE_STATUS_INGEST_${timestamp}"
command_hidden_token="NOECHO_CHANNEL_PROFILE_STATUS_COMMAND_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_PROFILE_STATUS_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_PROFILE_STATUS_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_PROFILE_STATUS_CONTEXT_V1"
search_phrase="channel profile status unique search fixture phrase"
notify_message_hash="$(sha256_12 "$notify_message_id")"
issue_number=""
issue_title="GitClaw ${channel} thread ${thread_id}"

run_list_json() {
  local workflow="$1"
  local event="$2"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event "$event" \
    --limit 30 \
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
  for _ in {1..90}; do
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
  for _ in {1..90}; do
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

profile_status_notification_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$notify_message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

cleanup() {
  if [[ -n "${issue_number:-}" && "$issue_number" != "null" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "channel-profile-status slash e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel-profile-status slash E2E.

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

initial_report=""
for _ in {1..90}; do
  errors="$(error_count_for_issue "$issue_number")"
  if [[ "$errors" != "0" ]]; then
    die "issue #${issue_number} posted ${errors} error marker comment(s)"
  fi
  candidate_report="$(latest_assistant_comment_for_issue "$issue_number")"
  if grep -Fq "$ingest_hidden_token" <<<"$candidate_report"; then
    die "initial channel report leaked ingest hidden token"
  fi
  if grep -Fq "GitClaw Channel Report" <<<"$candidate_report" && grep -Fq 'channel_thread_issue: `true`' <<<"$candidate_report"; then
    initial_report="$candidate_report"
    break
  fi
  sleep 5
done
[[ -n "$initial_report" ]] || die "expected initial channel report"

profile_status_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels profile-status --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --status-id ${profile_status_id}
Do not include this command hidden token in the receipt: ${command_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$profile_status_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel profile-status action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel profile-status action receipt"
profile_status_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Profile Status Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels profile-status\`" \
  "channel_profile_status_status: \`queued\`" \
  "profile_snapshot_mode: \`provider-facing-profile-status\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "profile_status: \`ok\`" \
  "profile_strategy: \`repo-local-git-profile\`" \
  "profile_store: \`.gitclaw/\`" \
  "profile_scope: \`repository\`" \
  "snapshot_version: \`gitclaw-profile-snapshot-v1\`" \
  "snapshot_scope: \`repo-local-profile-soul-memory-skills-tools\`" \
  "snapshot_sha256_12: \`" \
  "snapshot_entries: \`5\`" \
  "profile_documents_loaded: \`" \
  "required_profile_documents: \`6\`" \
  "required_profile_documents_present: \`6\`" \
  "required_profile_documents_missing: \`0\`" \
  "available_skills: \`" \
  "selected_skills: \`" \
  "skill_bundles: \`" \
  "available_tools: \`" \
  "active_tool_outputs: \`" \
  "manifest_entries: \`" \
  "profile_manifest_sha256_12: \`" \
  "soul_snapshot_sha256_12: \`" \
  "memory_snapshot_sha256_12: \`" \
  "skill_snapshot_sha256_12: \`" \
  "tool_snapshot_sha256_12: \`" \
  "soul_status: \`ok\`" \
  "memory_status: \`ok\`" \
  "skill_status: \`ok\`" \
  "tool_status: \`ok\`" \
  "profile_export_supported: \`false\`" \
  "profile_import_supported: \`false\`" \
  "profile_switching_supported: \`false\`" \
  "profile_mutation_allowed: \`false\`" \
  "credentials_included: \`false\`" \
  "sessions_included: \`false\`" \
  "backup_payloads_included: \`false\`" \
  "notification_body_sha256_12: \`" \
  "profile_export_performed: \`false\`" \
  "profile_import_performed: \`false\`" \
  "profile_switch_performed: \`false\`" \
  "profile_mutation_performed: \`false\`" \
  "external_profile_home_accessed: \`false\`" \
  "model_call_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox + channel-delivery\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_profile_status_id_included: \`false\`" \
  "raw_profile_file_paths_included: \`false\`" \
  "raw_profile_bodies_included: \`false\`" \
  "raw_skill_bodies_included: \`false\`" \
  "raw_memory_bodies_included: \`false\`" \
  "raw_tool_outputs_included: \`false\`" \
  "raw_issue_bodies_included: \`false\`" \
  "raw_comment_bodies_included: \`false\`" \
  "raw_session_bodies_included: \`false\`" \
  "raw_backup_payloads_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "llm_e2e_required_after_channel_profile_status_action_change: \`true\`"; do
  grep -Fq "$expected" <<<"$profile_status_receipt" || die "channel profile-status receipt missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$profile_status_id" ".gitclaw/SOUL.md" ".gitclaw/SKILLS/repo-reader/SKILL.md" "GITCLAW_CHANNEL_PROFILE_STATUS_CONTEXT_V1"; do
  if grep -Fq "$leaked" <<<"$profile_status_receipt"; then
    die "channel profile-status receipt leaked ${leaked}"
  fi
done

[[ "$(profile_status_notification_count)" == "1" ]] || die "channel profile-status did not queue exactly one profile-status notification"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
notification_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound") and contains("'"${notify_message_id}"'"))] | join("\n")' <<<"$issue_json")"
for expected in \
  "GitClaw channel profile status." \
  "Profile status: ok" \
  "Profile store: .gitclaw/" \
  "Profile scope: repository" \
  "Snapshot version: gitclaw-profile-snapshot-v1" \
  "Snapshot hash: " \
  "Snapshot entries: 5" \
  "Profile documents loaded: " \
  "Required profile documents: 6/6 present" \
  "Available skills: " \
  "Selected skills: " \
  "Skill bundles: " \
  "Available tools: " \
  "Active tool outputs: " \
  "Components: manifest=ok soul=ok memory=ok skills=ok tools=ok" \
  "Profile export: disabled." \
  "Profile import: disabled." \
  "Profile switching: disabled." \
  "Profile mutation: disabled." \
  "External profile home: not accessed." \
  "Credentials: not included." \
  "Sessions: not included." \
  "Backup payloads: not included." \
  "Raw profile bodies: not included." \
  "Raw skill bodies: not included." \
  "Raw memory bodies: not included." \
  "Raw tool outputs: not included." \
  "Model call: not performed by this action." \
  "Repository mutation: not performed by this action." \
  "Provider delivery: queued through GitHub channel outbox."; do
  grep -Fq "$expected" <<<"$notification_bodies" || die "profile-status notification missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$profile_status_id" "GITCLAW_CHANNEL_PROFILE_STATUS_CONTEXT_V1" ".gitclaw/SOUL.md" ".gitclaw/SKILLS/repo-reader/SKILL.md"; do
  if grep -Fq "$leaked" <<<"$notification_bodies"; then
    die "profile-status notification leaked ${leaked}"
  fi
done

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq "outbound_comments=1" <<<"$outbox_output" || die "channel outbox output missing outbound count: ${outbox_output}"
grep -Fq "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing profile-status notify hash ${notify_message_hash}"
for leaked in "$account_id" "$ingest_hidden_token" "$command_hidden_token" "$profile_status_id" "GITCLAW_CHANNEL_PROFILE_STATUS_CONTEXT_V1" ".gitclaw/SOUL.md" "repo-reader"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels agent-profile --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --status-id ${profile_status_id}
Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel profile-status action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel profile-status receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Profile Status Action" \
  "requested_channel_command: \`/channels agent-profile\`" \
  "channel_profile_status_status: \`duplicate\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "profile_export_performed: \`false\`" \
  "profile_import_performed: \`false\`" \
  "profile_switch_performed: \`false\`" \
  "profile_mutation_performed: \`false\`" \
  "external_profile_home_accessed: \`false\`" \
  "model_call_performed: \`false\`" \
  "repository_mutation_performed: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel profile-status receipt missing ${expected}"
done
[[ "$(profile_status_notification_count)" == "1" ]] || die "duplicate channel profile-status queued another notification"
for leaked in "$duplicate_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$profile_status_id" ".gitclaw/SOUL.md" "GITCLAW_CHANNEL_PROFILE_STATUS_CONTEXT_V1"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel profile-status receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel profile-status thread and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_PROFILE_STATUS_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include provider ids, notification ids, thread ids, message ids, account hashes, profile-status ids, profile file paths, profile bodies, skill bodies, memory bodies, tool outputs, issue numbers, or previous channel bodies.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$issue_title")" || die "timed out waiting for channel profile-status model follow-up"
wait_for_assistant_count_for_issue "$issue_number" 4 || die "expected model-backed channel profile-status follow-up"
model_comment="$(latest_assistant_comment_for_issue "$issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel profile-status search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel profile-status follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel profile-status follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel profile-status follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel profile-status follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel profile-status follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel profile-status follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$profile_status_id" "$account_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel profile-status follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number} (model follow-up: ${model_url})"
