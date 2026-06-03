#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel backup-continuity queues archive gap cards and proves model/tool follow-up.
set -euo pipefail

log() {
  echo "channel-backup-continuity-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
backup_branch="${GITCLAW_E2E_BACKUP_BRANCH:-gitclaw-backups}"
lock_dir="/tmp/gitclaw-channel-backup-continuity-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel backup-continuity slash E2E appears to be running: ${lock_dir}"
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
    local errors got
    errors="$(error_count_for_issue "$number")"
    if [[ "$errors" != "0" ]]; then
      die "issue #${number} posted ${errors} error marker comment(s)"
    fi
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

backup_continuity_notification_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$notify_message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

fetch_backup_branch() {
  rm -rf "$tmp_dir"
  tmp_dir="$(mktemp -d)"
  backup_checkout="${tmp_dir}/backup-branch"
  gh repo clone "$repo" "$backup_checkout" -- --depth=1 --branch "$backup_branch" >/dev/null 2>&1
}

cleanup() {
  if [[ -n "${tmp_dir:-}" ]]; then
    rm -rf "$tmp_dir"
  fi
  if [[ -n "${issue_number:-}" && "$issue_number" != "null" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "channel backup-continuity slash e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ensure_label gitclaw 5319e7 "Handled by GitClaw"
ensure_label gitclaw:channel 1d76db "GitClaw mirrored channel thread"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ | tr '[:upper:]' '[:lower:]')"
channel="telegram"
thread_id="channel-backup-continuity-e2e-${timestamp}"
ingest_message_id="backup-continuity-ingest-${timestamp}"
notify_message_id="backup-continuity-notify-${timestamp}"
continuity_id="channel-backup-continuity-${timestamp}"
max_gap_hours="87600"
max_gap_seconds="315360000"
account_id="telegram-backup-continuity-account-NOECHO_CHANNEL_BACKUP_CONTINUITY_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_BACKUP_CONTINUITY_INGEST_${timestamp}"
command_hidden_token="NOECHO_CHANNEL_BACKUP_CONTINUITY_COMMAND_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_BACKUP_CONTINUITY_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_BACKUP_CONTINUITY_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_BACKUP_CONTINUITY_CONTEXT_V1"
search_phrase="channel backup continuity unique search fixture phrase"
notify_message_hash="$(sha256_12 "$notify_message_id")"
issue_number=""
issue_title="GitClaw ${channel} thread ${thread_id}"
tmp_dir=""
backup_checkout=""

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel backup-continuity slash E2E.

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
  if grep -Fq -- "GitClaw Channel Report" <<<"$candidate_report" && grep -Fq -- 'channel_thread_issue: `true`' <<<"$candidate_report"; then
    initial_report="$candidate_report"
    break
  fi
  sleep 5
done
[[ -n "$initial_report" ]] || die "expected initial channel report"

repo_key="${repo//\//__}"
issue_padded="$(printf "%06d" "$issue_number")"
issue_path="issues/${issue_padded}.json"
continuity_output=""
for _ in {1..60}; do
  if fetch_backup_branch; then
    index_path="${backup_checkout}/.gitclaw/backups/${repo_key}/index.json"
  else
    sleep 5
    continue
  fi
  if [[ -f "$index_path" ]] &&
    jq -e --argjson number "$issue_number" --arg title "$issue_title" --arg path "$issue_path" '
      any(.issues[]; .number == $number and .title == $title and .path == $path)
    ' "$index_path" >/dev/null; then
    continuity_output="$(go run ./cmd/gitclaw backup continuity --root "${backup_checkout}/.gitclaw/backups" --repo "$repo" --max-gap-hours "$max_gap_hours")"
    break
  fi
  sleep 5
done

if [[ -z "$continuity_output" ]]; then
  die "channel backup-continuity did not observe source issue #${issue_number} in ${backup_branch}"
fi
for expected in \
  "GitClaw Backup Continuity Report" \
  "repository: \`${repo}\`" \
  'backup_continuity_status: `ok`' \
  'backup_verify_status: `ok`' \
  'verification_failures: `0`' \
  'backup_schema_version: `1`' \
  'issue_count: `' \
  'points_scanned: `' \
  'timeline_order: `chronological`' \
  "max_gap_seconds: \`${max_gap_seconds}\`" \
  'gaps_over_max: `0`' \
  'continuity_gate: `pass`' \
  'raw_bodies_included: `false`' \
  'llm_e2e_required_after_backup_continuity_change: `true`'; do
  grep -Fq -- "$expected" <<<"$continuity_output" || die "local fetched backup-continuity output missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$issue_title"; do
  if grep -Fq -- "$leaked" <<<"$continuity_output"; then
    die "local fetched backup-continuity output leaked ${leaked}"
  fi
done

continuity_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels backup-continuity --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --continuity-id ${continuity_id} --max-gap-hours ${max_gap_hours}
Do not include this command hidden token in the receipt: ${command_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$continuity_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel backup-continuity action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel backup-continuity action receipt"
continuity_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Backup Continuity Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels backup-continuity\`" \
  "channel_backup_continuity_status: \`queued\`" \
  "backup_continuity_status: \`ok\`" \
  "backup_verify_status: \`ok\`" \
  "continuity_gate: \`pass\`" \
  "backup_fetch_status: \`fetched\`" \
  "backup_branch: \`${backup_branch}\`" \
  "continuity_mode: \`gitclaw-backups-continuity-card\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "continuity_id_sha256_12: \`" \
  "continuity_id_auto: \`false\`" \
  "max_gap_hours: \`${max_gap_hours}\`" \
  "max_gap_seconds: \`${max_gap_seconds}\`" \
  "max_gap_source: \`flag\`" \
  "backup_root_sha256_12: \`" \
  "repo_backup_dir_sha256_12: \`" \
  "index_path_sha256_12: \`" \
  "readme_path_sha256_12: \`" \
  "backup_schema_version: \`1\`" \
  "index_generated_at_sha256_12: \`" \
  "verification_failures: \`0\`" \
  "issue_count: \`" \
  "points_scanned: \`" \
  "timeline_order: \`chronological\`" \
  "gaps_over_max: \`0\`" \
  "gaps_reported: \`0\`" \
  "first_issue_sha256_12: \`" \
  "latest_issue_sha256_12: \`" \
  "total_span_seconds: \`" \
  "longest_gap_seconds: \`" \
  "continuity_gaps_sha256_12: \`none\`" \
  "continuity_error_kind: \`none\`" \
  "continuity_error_sha256_12: \`none\`" \
  "notification_body_sha256_12: \`" \
  "notification_body_bytes: \`" \
  "notification_body_lines: \`" \
  "backup_branch_fetch_performed: \`true\`" \
  "raw_backup_payloads_read: \`true\`" \
  "restore_performed: \`false\`" \
  "backup_branch_write_performed: \`false\`" \
  "github_api_replay_performed: \`false\`" \
  "model_call_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox + channel-delivery\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_continuity_id_included: \`false\`" \
  "raw_backup_root_included: \`false\`" \
  "raw_backup_paths_included: \`false\`" \
  "raw_continuity_gaps_included: \`false\`" \
  "raw_backup_payloads_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "raw_issue_titles_included: \`false\`" \
  "raw_issue_bodies_included: \`false\`" \
  "raw_comment_bodies_included: \`false\`" \
  "raw_transcript_bodies_included: \`false\`" \
  "raw_prompts_included: \`false\`" \
  "raw_tool_outputs_included: \`false\`" \
  "llm_e2e_required_after_channel_backup_continuity_change: \`true\`"; do
  grep -Fq -- "$expected" <<<"$continuity_receipt" || die "channel backup-continuity receipt missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$continuity_id" "$expected_token" "$repo_key"; do
  if grep -Fq -- "$leaked" <<<"$continuity_receipt"; then
    die "channel backup-continuity receipt leaked ${leaked}"
  fi
done

[[ "$(backup_continuity_notification_count)" == "1" ]] || die "channel backup-continuity did not queue exactly one notification"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq -- "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
notification_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound") and contains("'"${notify_message_id}"'"))] | join("\n")' <<<"$issue_json")"
for expected in \
  "GitClaw channel backup continuity" \
  "Backup continuity status: ok" \
  "Backup verify status: ok" \
  "Continuity gate: pass" \
  "Backup branch: ${backup_branch}" \
  "Backup fetch status: fetched" \
  "Issue count: " \
  "Points scanned: " \
  "Timeline order: chronological" \
  "Max gap hours: ${max_gap_hours}" \
  "Max gap seconds: ${max_gap_seconds}" \
  "Gaps over max: 0" \
  "Gaps reported: 0" \
  "First issue: #" \
  "Latest issue: #" \
  "Total span seconds: " \
  "Longest gap seconds: " \
  "Continuity id hash: " \
  "Gaps over threshold:" \
  "- none" \
  "Raw backup payloads, backup paths, channel bodies, issue titles, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw continuity ids are not included." \
  "Model call: not performed by this action." \
  "Repository mutation: not performed by this action." \
  "Backup branch write: not performed by this action." \
  "Restore: not performed by this action." \
  "GitHub API replay: not performed by this action." \
  "Provider delivery: queued through GitHub channel outbox."; do
  grep -Fq -- "$expected" <<<"$notification_bodies" || die "backup-continuity notification missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$continuity_id" "$expected_token" "$issue_title"; do
  if grep -Fq -- "$leaked" <<<"$notification_bodies"; then
    die "backup-continuity notification leaked ${leaked}"
  fi
done

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq -- "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq -- "outbound_comments=1" <<<"$outbox_output" || die "channel outbox output missing outbound count: ${outbox_output}"
grep -Fq -- "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing backup-continuity notify hash ${notify_message_hash}"
for leaked in "$account_id" "$ingest_hidden_token" "$command_hidden_token" "$continuity_id" "$expected_token"; do
  if grep -Fq -- "$leaked" <<<"$outbox_output" || grep -Fq -- "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels archive-gaps --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --continuity-id ${continuity_id} --max-gap-hours ${max_gap_hours}
Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel backup-continuity action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel backup-continuity receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Backup Continuity Action" \
  "requested_channel_command: \`/channels archive-gaps\`" \
  "channel_backup_continuity_status: \`duplicate\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "model_call_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "backup_branch_write_performed: \`false\`"; do
  grep -Fq -- "$expected" <<<"$duplicate_receipt" || die "duplicate channel backup-continuity receipt missing ${expected}"
done
[[ "$(backup_continuity_notification_count)" == "1" ]] || die "duplicate channel backup-continuity queued another notification"
for leaked in "$duplicate_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$continuity_id" "$expected_token" "$repo_key"; do
  if grep -Fq -- "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel backup-continuity receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel backup-continuity thread and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_BACKUP_CONTINUITY_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include provider ids, notification ids, thread ids, message ids, account hashes, continuity ids, backup paths, backup branches, gap counts, archive timestamps, issue numbers, or previous channel bodies.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$issue_title")" || die "timed out waiting for channel backup-continuity model follow-up"
wait_for_assistant_count_for_issue "$issue_number" 4 || die "expected model-backed channel backup-continuity follow-up"
model_comment="$(latest_assistant_comment_for_issue "$issue_number")"

grep -Fq -- "$expected_token" <<<"$model_comment" || die "assistant did not include channel backup-continuity search_files token ${expected_token}"
if ! grep -Fq -- 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq -- 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel backup-continuity follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq -- 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel backup-continuity follow-up marker missing prompt context hash"
grep -Fq -- 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel backup-continuity follow-up marker missing selected repo-reader skill"
grep -Fq -- 'tools="' <<<"$model_comment" || die "assistant channel backup-continuity follow-up marker missing prompt-visible tools"
grep -Fq -- 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel backup-continuity follow-up marker did not prove search_files was prompt-visible"
grep -Fq -- 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel backup-continuity follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$continuity_id" "$account_id"; do
  if grep -Fq -- "$leaked" <<<"$model_comment"; then
    die "model channel backup-continuity follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number} (model follow-up: ${model_url})"
