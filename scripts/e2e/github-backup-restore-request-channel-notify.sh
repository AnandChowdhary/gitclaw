#!/usr/bin/env bash
# gitclaw-doctor-live-issue: backup restore request can notify reviewed channel routes and then prove normal LLM/tool chat.
set -euo pipefail

log() {
  echo "backup-restore-request-channel-notify-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
backup_branch="${GITCLAW_E2E_BACKUP_BRANCH:-gitclaw-backups}"
lock_dir="/tmp/gitclaw-backup-restore-request-channel-notify-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another backup restore request channel notify E2E appears to be running: ${lock_dir}"
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
ensure_label gitclaw:channel 1d76db "GitClaw mirrored channel thread"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ | tr '[:upper:]' '[:lower:]')"
notify_route="e2e-telegram-route"
request_id="backup-restore-notify-${timestamp}"
source_title="GitClaw backup restore request channel notify E2E ${timestamp}"
account_id="telegram-backup-restore-account-NOECHO_BACKUP_RESTORE_NOTIFY_ACCOUNT_${timestamp}"
source_hidden_token="NOECHO_BACKUP_RESTORE_NOTIFY_SOURCE_${timestamp}"
duplicate_hidden_token="NOECHO_BACKUP_RESTORE_NOTIFY_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_BACKUP_RESTORE_NOTIFY_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_BACKUP_RESTORE_NOTIFY_CONTEXT_V1"
search_phrase="backup restore request channel notify unique search fixture phrase"
source_issue_number=""
restore_issue_number=""
channel_issue_number=""

run_list_json() {
  local event="$1"
  local started_at="$2"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event "$event" \
    --created ">=$started_at" \
    --limit 30 \
    --json databaseId,status,conclusion,createdAt,url,displayTitle \
    --jq '.'
}

wait_for_run() {
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

latest_assistant_comment_for_issue() {
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

outbound_comment_count() {
  gh issue view "$channel_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:channel-outbound"))] | length'
}

cleanup() {
  for number in "$source_issue_number" "$restore_issue_number" "$channel_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "backup restore request channel notify e2e cleanup" >/dev/null 2>&1 || true
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
  --body "@gitclaw /backup restore-request --id ${request_id} --notify-route ${notify_route}

Open a backup restore review lane, then notify the reviewed Telegram route.
Do not include this hidden source token: ${source_hidden_token}")"
source_issue_number="${source_issue_url##*/}"
log "created source issue #${source_issue_number}: ${source_issue_url}"

wait_for_run "issues" "$opened_started_at" "$source_title" >/dev/null || die "timed out waiting for backup restore request notify issues run"
wait_for_assistant_count_for_issue "$source_issue_number" 1 || die "expected one backup restore request notify receipt"
receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"

for expected in \
  "GitClaw Backup Restore Request Issue Action" \
  "Generated without a model call" \
  'model="gitclaw/backup"' \
  "requested_backup_command: \`/backup restore-request\`" \
  "backup_restore_request_status: \`created\`" \
  "restore_request_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "backup_issue: \`#${source_issue_number}\`" \
  "target_repository: \`${repo}\`" \
  "backup_branch: \`${backup_branch}\`" \
  "backup_root: \`.gitclaw/backups\`" \
  "restore_request_issue_labeled_for_gitclaw: \`true\`" \
  "channel_notification_requested: \`true\`" \
  "channel_notification_routes: \`1\`" \
  "channel_notification_queued: \`1\`" \
  "channel_notification_duplicates: \`0\`" \
  "channel_notification_target_issues_created: \`1\`" \
  "raw_channel_routes_included: \`false\`" \
  "raw_channel_notification_body_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "model_call_performed: \`false\`" \
  "approval_required: \`true\`" \
  "restore_pr_required: \`true\`" \
  "restore_mode: \`dry-run-first\`" \
  "repository_mutation_allowed: \`false\`" \
  "backup_branch_write_allowed: \`false\`" \
  "github_api_replay_allowed: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "raw_backup_bodies_included: \`false\`" \
  "llm_e2e_required_after_backup_restore_request_issue_change: \`true\`" \
  "channel=\`telegram\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "backup restore request notify receipt missing ${expected}"
done
for leaked in "$source_hidden_token" "$notify_route" "$request_id" "gitclaw-backup-restore-request-${request_id}" "Open a backup restore review lane"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "backup restore request notify receipt leaked ${leaked}"
  fi
done

restore_issue_number="$(sed -n 's/.*restore_request_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
channel_issue_number="$(sed -n 's/.*destination=`01` target_issue=`#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
[[ -n "$restore_issue_number" ]] || die "could not parse restore request issue from receipt"
[[ -n "$channel_issue_number" ]] || die "could not parse channel notification issue from receipt"
log "created restore request issue #${restore_issue_number} and queued channel notification on #${channel_issue_number}"

repo_key="${repo//\//__}"
issue_padded="$(printf "%06d" "$source_issue_number")"
full_issue_path=".gitclaw/backups/${repo_key}/issues/${issue_padded}.json"

restore_json="$(gh issue view "$restore_issue_number" --repo "$repo" --json title,body,labels)"
restore_title="$(jq -r '.title' <<<"$restore_json")"
restore_body="$(jq -r '.body' <<<"$restore_json")"
restore_labels="$(jq -r '.labels[].name' <<<"$restore_json")"
[[ "$restore_title" == "GitClaw backup restore request: #${source_issue_number}" ]] || die "unexpected restore issue title: ${restore_title}"
grep -Fxq "gitclaw" <<<"$restore_labels" || die "restore request issue missing gitclaw label"
for expected in \
  "gitclaw:backup-restore-request-issue" \
  "request_id: ${request_id}" \
  "restore_scope: issue-thread" \
  "backup_issue: #${source_issue_number}" \
  "target_repository: ${repo}" \
  "backup_branch: ${backup_branch}" \
  "backup_root: .gitclaw/backups" \
  "issue_backup_path: ${full_issue_path}" \
  "approval_required: true" \
  "restore_pr_required: true" \
  "restore_mode: dry-run-first" \
  "repository_mutation_allowed: false" \
  "backup_branch_write_allowed: false" \
  "github_api_replay_allowed: false" \
  "raw_source_body_included: false" \
  "raw_backup_bodies_included: false"; do
  grep -Fq "$expected" <<<"$restore_body" || die "restore request issue body missing ${expected}"
done
for leaked in "$source_hidden_token" "$notify_route" "Open a backup restore review lane"; do
  if grep -Fq "$leaked" <<<"$restore_body"; then
    die "restore request issue body leaked ${leaked}"
  fi
done

channel_json="$(gh issue view "$channel_issue_number" --repo "$repo" --json body,labels,comments)"
channel_labels="$(jq -r '.labels[].name' <<<"$channel_json")"
grep -Fxq "gitclaw:channel" <<<"$channel_labels" || die "channel notification issue missing gitclaw:channel label"
if grep -Fxq "gitclaw" <<<"$channel_labels"; then
  die "channel notification issue should not carry the model trigger label"
fi
channel_body="$(jq -r '.body' <<<"$channel_json")"
channel_comments="$(jq -r '[.comments[].body] | join("\n")' <<<"$channel_json")"
for expected in \
  "gitclaw:channel-thread" \
  'channel="telegram"'; do
  grep -Fq "$expected" <<<"$channel_body" || die "channel notification issue body missing ${expected}"
done
for expected in \
  "gitclaw:channel-outbound" \
  "message_id=\"gitclaw-backup-restore-request-${request_id}\"" \
  "GitClaw backup restore request" \
  "Review issue: #${restore_issue_number} https://github.com/${repo}/issues/${restore_issue_number}" \
  "Source issue: #${source_issue_number} https://github.com/${repo}/issues/${source_issue_number}" \
  "Request id: ${request_id}" \
  "Backup issue: #${source_issue_number}" \
  "Target repository: ${repo}" \
  "Backup branch: ${backup_branch}" \
  "Restore PR required: true" \
  "Restore mode: dry-run-first"; do
  grep -Fq "$expected" <<<"$channel_comments" || die "channel notification comments missing ${expected}"
done
for leaked in "$source_hidden_token" "Open a backup restore review lane"; do
  if grep -Fq "$leaked" <<<"$channel_comments"; then
    die "channel notification leaked ${leaked}"
  fi
done

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL=telegram \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$channel_issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "pending=1" <<<"$outbox_output" || die "channel outbox did not report pending backup restore notification: ${outbox_output}"
grep -Fq '"pending_messages": 1' "$outbox_file" || die "channel outbox file missing pending backup restore notification"
if grep -Fq "$request_id" <<<"$outbox_output" || grep -Fq "$request_id" "$outbox_file"; then
  die "channel outbox leaked backup restore notification body without --include-body"
fi
if grep -Fq "$account_id" <<<"$outbox_output" || grep -Fq "$account_id" "$outbox_file"; then
  die "channel outbox leaked account id"
fi

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /backup restore-request --id ${request_id} --notify-route ${notify_route}

Repeat the same backup restore request notification.
Do not include this hidden duplicate token: ${duplicate_hidden_token}" >/dev/null

wait_for_run "issue_comment" "$duplicate_started_at" "$source_title" >/dev/null || die "timed out waiting for duplicate backup restore request notify run"
wait_for_assistant_count_for_issue "$source_issue_number" 2 || die "expected duplicate backup restore request notify receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"
for expected in \
  "GitClaw Backup Restore Request Issue Action" \
  "backup_restore_request_status: \`existing\`" \
  "restore_request_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "restore_request_issue: \`#${restore_issue_number}\`" \
  "channel_notification_requested: \`true\`" \
  "channel_notification_queued: \`0\`" \
  "channel_notification_duplicates: \`1\`" \
  "raw_channel_notification_body_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate backup restore notify receipt missing ${expected}"
done
[[ "$(outbound_comment_count)" == "1" ]] || die "duplicate backup restore notification created another outbound comment"
for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$notify_route" "$request_id" "Repeat the same backup restore request notification"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate backup restore notify receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$restore_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this backup restore request channel notification review and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line ends with an all-caps token after the arrow.
That token starts with \`GITCLAW_BACKUP_RESTORE_NOTIFY_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
The exact token includes \`BACKUP_RESTORE_NOTIFY\`; do not insert or remove words.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include the source issue number, restore request issue number, channel issue number, route name, account hash, request id, backup path, backup branch, source body, or hidden sentinels.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run "issue_comment" "$comment_started_at" "$restore_title")" || die "timed out waiting for backup restore request notify model follow-up"
wait_for_assistant_count_for_issue "$restore_issue_number" 1 || die "expected model-backed backup restore request notify follow-up"
model_comment="$(latest_assistant_comment_for_issue "$restore_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include backup restore notify search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant backup restore notify follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant backup restore notify follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant backup restore notify follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant backup restore notify follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant backup restore notify follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant backup restore notify follow-up marker missing usage token telemetry"

for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$notify_route" "$request_id" "$account_id" "$full_issue_path"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model backup restore notify follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}, restore request issue #${restore_issue_number}, channel issue #${channel_issue_number} (model follow-up: ${model_url})"
