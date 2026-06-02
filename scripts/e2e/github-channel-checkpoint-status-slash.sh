#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-checkpoint-status queues rollback readiness cards and proves model/tool follow-up.
set -euo pipefail

log() {
  echo "channel-checkpoint-status-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-checkpoint-status-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-checkpoint-status slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-checkpoint-status-e2e-${timestamp}"
ingest_message_id="checkpoint-status-ingest-${timestamp}"
notify_message_id="checkpoint-status-notify-${timestamp}"
checkpoint_status_id="checkpoint-status-${timestamp}"
account_id="telegram-checkpoint-status-account-NOECHO_CHANNEL_CHECKPOINT_STATUS_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_CHECKPOINT_STATUS_INGEST_${timestamp}"
command_hidden_token="NOECHO_CHANNEL_CHECKPOINT_STATUS_COMMAND_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_CHECKPOINT_STATUS_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_CHECKPOINT_STATUS_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_CHECKPOINT_STATUS_CONTEXT_V1"
search_phrase="channel checkpoint status unique search fixture phrase"
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

checkpoint_status_notification_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$notify_message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

cleanup() {
  if [[ -n "${issue_number:-}" && "$issue_number" != "null" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "channel-checkpoint-status slash e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel-checkpoint-status slash E2E.

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
  if grep -Fq -- "$ingest_hidden_token" <<<"$candidate_report"; then
    die "initial channel report leaked ingest hidden token"
  fi
  if grep -Fq "GitClaw Channel Report" <<<"$candidate_report" && grep -Fq 'channel_thread_issue: `true`' <<<"$candidate_report"; then
    initial_report="$candidate_report"
    break
  fi
  sleep 5
done
[[ -n "$initial_report" ]] || die "expected initial channel report"

checkpoint_status_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels checkpoint-status --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --status-id ${checkpoint_status_id}
Do not include this command hidden token in the receipt: ${command_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$checkpoint_status_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel checkpoint-status action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel checkpoint-status action receipt"
checkpoint_status_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Checkpoint Status Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels checkpoint-status\`" \
  "channel_checkpoint_status_status: \`queued\`" \
  "checkpoint_snapshot_mode: \`provider-facing-rollback-readiness\`" \
  "checkpoint_status: \`clean\`" \
  "checkpoint_risk_status: \`ok\`" \
  "checkpoint_strategy: \`git-history-plus-backup-branch\`" \
  "rollback_mode: \`inspect-only\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "checkpoint_status_id_sha256_12: \`" \
  "checkpoint_status_id_auto: \`false\`" \
  "git_available: \`true\`" \
  "git_repository: \`true\`" \
  "worktree_root: \`.\`" \
  "branch: \`main\`" \
  "head_commit: \`" \
  "commits_available: \`" \
  "recent_commits_returned: \`" \
  "recent_commit_limit: \`5\`" \
  "recent_commit_index_sha256_12: \`" \
  "worktree_clean: \`true\`" \
  "staged_changes: \`0\`" \
  "unstaged_changes: \`0\`" \
  "untracked_files: \`0\`" \
  "backup_branch: \`gitclaw-backups\`" \
  "backup_branch_local_ref: \`" \
  "surfaces_with_risk_findings: \`0\`" \
  "checkpoint_risk_findings: \`0\`" \
  "high_risk_findings: \`0\`" \
  "warning_risk_findings: \`0\`" \
  "info_risk_findings: \`0\`" \
  "risk_finding_index_sha256_12: \`none\`" \
  "raw_diffs_included: \`false\`" \
  "raw_file_bodies_included: \`false\`" \
  "restore_operations_enabled: \`false\`" \
  "git_reset_allowed: \`false\`" \
  "git_clean_allowed: \`false\`" \
  "checkout_mutation_allowed: \`false\`" \
  "pre_restore_snapshot_required: \`true\`" \
  "rollback_diff_preview_required: \`true\`" \
  "backup_manifest_required_for_restore: \`true\`" \
  "shadow_store_path_included: \`false\`" \
  "notification_body_sha256_12: \`" \
  "notification_body_bytes: \`" \
  "notification_body_lines: \`" \
  "model_call_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "restore_performed: \`false\`" \
  "git_reset_performed: \`false\`" \
  "git_clean_performed: \`false\`" \
  "checkout_mutation_performed: \`false\`" \
  "raw_diff_generation_performed: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox + channel-delivery\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_checkpoint_status_id_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "raw_issue_bodies_included: \`false\`" \
  "raw_comment_bodies_included: \`false\`" \
  "raw_commit_subjects_included: \`false\`" \
  "raw_diffs_included_in_notification: \`false\`" \
  "raw_file_bodies_included_in_notification: \`false\`" \
  "raw_prompts_included: \`false\`" \
  "raw_tool_outputs_included: \`false\`" \
  "credential_values_included: \`false\`" \
  "llm_e2e_required_after_channel_checkpoint_status_change: \`true\`"; do
  grep -Fq -- "$expected" <<<"$checkpoint_status_receipt" || die "channel checkpoint-status receipt missing ${expected}"
done
grep -Eq 'commits_available: `([1-9][0-9]*)`' <<<"$checkpoint_status_receipt" || die "channel checkpoint-status receipt did not report commits"
grep -Eq 'recent_commits_returned: `([1-9][0-9]*)`' <<<"$checkpoint_status_receipt" || die "channel checkpoint-status receipt did not report recent commits"
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$checkpoint_status_id" "$expected_token" "Add channel" "Harden channel" "diff --git" "CHECKPOINT_STATUS_COMMIT_SECRET" "CHECKPOINT_STATUS_FILE_SECRET"; do
  if grep -Fq -- "$leaked" <<<"$checkpoint_status_receipt"; then
    die "channel checkpoint-status receipt leaked ${leaked}"
  fi
done

[[ "$(checkpoint_status_notification_count)" == "1" ]] || die "channel checkpoint-status did not queue exactly one checkpoint-status notification"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
notification_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound") and contains("'"${notify_message_id}"'"))] | join("\n")' <<<"$issue_json")"
for expected in \
  "GitClaw channel checkpoint status" \
  "Checkpoint status: clean" \
  "Checkpoint risk status: ok" \
  "Checkpoint strategy: git-history-plus-backup-branch" \
  "Rollback mode: inspect-only" \
  "Git available: true" \
  "Git repository: true" \
  "Worktree root: ." \
  "Branch: main" \
  "Head commit: " \
  "Commits available: " \
  "Recent commits returned: " \
  "Worktree clean: true" \
  "Staged changes: 0" \
  "Unstaged changes: 0" \
  "Untracked files: 0" \
  "Backup branch: gitclaw-backups" \
  "Backup branch local ref: " \
  "Surfaces with risk findings: 0" \
  "Risk findings: 0" \
  "High risk findings: 0" \
  "Warning risk findings: 0" \
  "Info risk findings: 0" \
  "Status id hash: " \
  "Recent commits:" \
  "subject_sha256_12=" \
  "raw_subject_included=false" \
  "raw_diff_included=false" \
  "Risk findings:" \
  "- none" \
  "Safe follow-up commands:" \
  "gitclaw checkpoints status" \
  "gitclaw checkpoints preview HEAD~1" \
  "gitclaw checkpoints risk" \
  "gitclaw rollback diff HEAD~1" \
  "@gitclaw /channels rehearse-checkpoint --target HEAD~1 --message-id <id>" \
  "Raw diffs: not generated by this action." \
  "Raw file bodies: not included." \
  "Raw commit subjects: not included." \
  "Restore: not performed by this action." \
  "Git reset: not performed by this action." \
  "Git clean: not performed by this action." \
  "Checkout mutation: not performed by this action." \
  "Model call: not performed by this action." \
  "Repository mutation: not performed by this action." \
  "Provider delivery: queued through GitHub channel outbox."; do
  grep -Fq -- "$expected" <<<"$notification_bodies" || die "checkpoint-status notification missing ${expected}"
done
grep -Eq 'Commits available: ([1-9][0-9]*)' <<<"$notification_bodies" || die "checkpoint-status notification did not report commits"
grep -Eq 'Recent commits returned: ([1-9][0-9]*)' <<<"$notification_bodies" || die "checkpoint-status notification did not report recent commits"
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$checkpoint_status_id" "$expected_token" "Add channel" "Harden channel" "diff --git" "CHECKPOINT_STATUS_COMMIT_SECRET" "CHECKPOINT_STATUS_FILE_SECRET"; do
  if grep -Fq -- "$leaked" <<<"$notification_bodies"; then
    die "checkpoint-status notification leaked ${leaked}"
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
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing checkpoint-status notify hash ${notify_message_hash}"
for leaked in "$account_id" "$ingest_hidden_token" "$command_hidden_token" "$checkpoint_status_id" "$expected_token" "Add channel" "Harden channel" "diff --git" "CHECKPOINT_STATUS_COMMIT_SECRET" "CHECKPOINT_STATUS_FILE_SECRET"; do
  if grep -Fq -- "$leaked" <<<"$outbox_output" || grep -Fq -- "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels rollback-status --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --status-id ${checkpoint_status_id}
Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel checkpoint-status action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel checkpoint-status receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Checkpoint Status Action" \
  "requested_channel_command: \`/channels rollback-status\`" \
  "channel_checkpoint_status_status: \`duplicate\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "model_call_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "restore_performed: \`false\`"; do
  grep -Fq -- "$expected" <<<"$duplicate_receipt" || die "duplicate channel checkpoint-status receipt missing ${expected}"
done
[[ "$(checkpoint_status_notification_count)" == "1" ]] || die "duplicate channel checkpoint-status queued another notification"
for leaked in "$duplicate_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$checkpoint_status_id" "$expected_token"; do
  if grep -Fq -- "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel checkpoint-status receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel checkpoint-status thread and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${search_phrase} => <token>\`.
Reply with only the uppercase fixture token immediately after \`=>\`, including the \`_CONTEXT_V1\` suffix.
Do not abbreviate the token to a prefix.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include provider ids, notification ids, thread ids, message ids, account hashes, status ids, issue numbers, raw diffs, commit subjects, or previous channel bodies.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$issue_title")" || die "timed out waiting for channel checkpoint-status model follow-up"
wait_for_assistant_count_for_issue "$issue_number" 4 || die "expected model-backed channel checkpoint-status follow-up"
model_comment="$(latest_assistant_comment_for_issue "$issue_number")"

grep -Fq -- "$expected_token" <<<"$model_comment" || die "assistant did not include channel checkpoint-status search_files token ${expected_token}"
if ! grep -Fq -- 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq -- 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel checkpoint-status follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq -- 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel checkpoint-status follow-up marker missing prompt context hash"
grep -Fq -- 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel checkpoint-status follow-up marker missing selected repo-reader skill"
grep -Fq -- 'tools="' <<<"$model_comment" || die "assistant channel checkpoint-status follow-up marker missing prompt-visible tools"
grep -Fq -- 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel checkpoint-status follow-up marker did not prove search_files was prompt-visible"
grep -Fq -- 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel checkpoint-status follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$checkpoint_status_id" "$account_id"; do
  if grep -Fq -- "$leaked" <<<"$model_comment"; then
    die "model channel checkpoint-status follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number} (model follow-up: ${model_url})"
