#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel rehearse-backup creates a backup rehearsal issue and proves model/tool follow-up.
set -euo pipefail

log() {
  echo "channel-backup-rehearsal-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
backup_branch="${GITCLAW_E2E_BACKUP_BRANCH:-gitclaw-backups}"
lock_dir="/tmp/gitclaw-channel-backup-rehearsal-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel backup rehearsal slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-backup-rehearsal-e2e-${timestamp}"
ingest_message_id="backup-rehearsal-ingest-${timestamp}"
rehearsal_id="channel-backup-rehearsal-${timestamp}"
notify_message_id="backup-rehearsal-notify-${timestamp}"
account_id="telegram-backup-rehearsal-account-NOECHO_CHANNEL_BACKUP_REHEARSAL_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_BACKUP_REHEARSAL_INGEST_${timestamp}"
source_hidden_token="NOECHO_CHANNEL_BACKUP_REHEARSAL_SOURCE_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_BACKUP_REHEARSAL_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_BACKUP_REHEARSAL_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_BACKUP_REHEARSAL_CONTEXT_V1"
search_phrase="channel backup rehearsal unique search fixture phrase"
notify_message_hash="$(sha256_12 "$notify_message_id")"
issue_number=""
rehearsal_issue_number=""
issue_title="GitClaw ${channel} thread ${thread_id}"
rehearsal_issue_title=""
tmp_dir=""
backup_checkout=""

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

backup_rehearsal_notification_count() {
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
  for number in "$issue_number" "$rehearsal_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "channel backup rehearsal slash e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel backup rehearsal slash E2E.

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

request_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels rehearse-backup --id ${rehearsal_id} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id}

Please rehearse channel-origin backup recovery for the mirrored thread.
Do not include this hidden source token: ${source_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$request_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel backup rehearsal action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel backup rehearsal action receipt"
request_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Backup Rehearsal Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels rehearse-backup\`" \
  "channel_backup_rehearsal_status: \`created\`" \
  "rehearsal_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "source_kind: \`channel_comment\`" \
  "backup_issue: \`#${issue_number}\`" \
  "backup_branch: \`${backup_branch}\`" \
  "backup_root: \`.gitclaw/backups\`" \
  "rehearsal_mode: \`recovery-conversation\`" \
  "rehearsal_issue_labeled_for_gitclaw: \`true\`" \
  "model_call_performed: \`false\`" \
  "restore_mode: \`dry-run\`" \
  "repository_mutation_allowed: \`false\`" \
  "backup_branch_write_allowed: \`false\`" \
  "github_api_replay_allowed: \`false\`" \
  "raw_rehearsal_id_included: \`false\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "raw_backup_bodies_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "llm_e2e_required_after_channel_backup_rehearsal_change: \`true\`"; do
  grep -Fq "$expected" <<<"$request_receipt" || die "channel backup rehearsal receipt missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$source_hidden_token" "$rehearsal_id" "$thread_id" "$ingest_message_id" "$notify_message_id" "Please rehearse channel-origin"; do
  if grep -Fq "$leaked" <<<"$request_receipt"; then
    die "channel backup rehearsal receipt leaked ${leaked}"
  fi
done

rehearsal_issue_number="$(sed -n 's/.*rehearsal_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$request_receipt" | head -n 1)"
[[ -n "$rehearsal_issue_number" && "$rehearsal_issue_number" != "null" ]] || die "could not resolve backup rehearsal issue number"
rehearsal_issue_title="GitClaw backup rehearsal: issue #${issue_number} (${rehearsal_id})"
log "channel backup rehearsal created rehearsal issue #${rehearsal_issue_number}"

[[ "$(backup_rehearsal_notification_count)" == "1" ]] || die "channel backup rehearsal did not queue exactly one rehearsal-link outbound comment"
repo_key="${repo//\//__}"
issue_padded="$(printf "%06d" "$issue_number")"
issue_path="issues/${issue_padded}.json"
full_issue_path=".gitclaw/backups/${repo_key}/${issue_path}"

rehearsal_json="$(gh issue view "$rehearsal_issue_number" --repo "$repo" --json title,body,labels,state)"
rehearsal_title="$(jq -r '.title' <<<"$rehearsal_json")"
rehearsal_body="$(jq -r '.body' <<<"$rehearsal_json")"
rehearsal_labels="$(jq -r '.labels[].name' <<<"$rehearsal_json")"
[[ "$rehearsal_title" == "$rehearsal_issue_title" ]] || die "unexpected backup rehearsal issue title: ${rehearsal_title}"
grep -Fxq "gitclaw" <<<"$rehearsal_labels" || die "backup rehearsal issue missing gitclaw label"
for expected in \
  "gitclaw:backup-rehearsal-issue" \
  "id=\"${rehearsal_id}\"" \
  "backup_issue=\"${issue_number}\"" \
  "rehearsal_id: ${rehearsal_id}" \
  "backup_issue: #${issue_number}" \
  "backup_branch: ${backup_branch}" \
  "backup_root: .gitclaw/backups" \
  "issue_backup_path: ${full_issue_path}" \
  "source_issue: #${issue_number}" \
  "source_kind: channel_comment" \
  "rehearsal_mode: recovery-conversation" \
  "restore_mode: dry-run" \
  "repository_mutation_allowed: false" \
  "backup_branch_write_allowed: false" \
  "github_api_replay_allowed: false" \
  "raw_source_body_included: false" \
  "raw_backup_bodies_included: false" \
  "gitclaw backup coverage --root .gitclaw/backups --repo ${repo} --issue ${issue_number}" \
  "gitclaw backup drill --root .gitclaw/backups --repo ${repo} --issue ${issue_number}" \
  "gitclaw backup restore-plan --root .gitclaw/backups --repo ${repo} --issue ${issue_number}"; do
  grep -Fq "$expected" <<<"$rehearsal_body" || die "backup rehearsal issue body missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$source_hidden_token" "Please rehearse channel-origin"; do
  if grep -Fq "$leaked" <<<"$rehearsal_body"; then
    die "backup rehearsal issue body leaked ${leaked}"
  fi
done

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
outbound_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound"))] | join("\n")' <<<"$issue_json")"
grep -Fq "gitclaw:channel-outbound" <<<"$outbound_bodies" || die "channel issue missing backup rehearsal outbound marker"
grep -Fq "GitClaw channel backup rehearsal" <<<"$outbound_bodies" || die "channel issue missing visible backup rehearsal notification"
grep -Fq "Backup issue: #${issue_number}" <<<"$outbound_bodies" || die "channel issue missing backup issue in notification"
if grep -Fq "$source_hidden_token" <<<"$outbound_bodies"; then
  die "channel backup rehearsal notification leaked source token"
fi

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL="$channel" \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "channel_outbox issue=${issue_number}" <<<"$outbox_output" || die "channel outbox output missing issue number: ${outbox_output}"
grep -Fq "body_included=false" <<<"$outbox_output" || die "channel outbox should be metadata-only: ${outbox_output}"
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing backup rehearsal outbound hash ${notify_message_hash}"
for leaked in "$rehearsal_id" "$source_hidden_token" "$account_id" "$ingest_hidden_token"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

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
    coverage_output="$(go run ./cmd/gitclaw backup coverage --root "${backup_checkout}/.gitclaw/backups" --repo "$repo" --issue "$issue_number")"
    drill_output="$(go run ./cmd/gitclaw backup drill --root "${backup_checkout}/.gitclaw/backups" --repo "$repo" --target-repo "$repo" --issue "$issue_number")"
    restore_output="$(go run ./cmd/gitclaw backup restore-plan --root "${backup_checkout}/.gitclaw/backups" --repo "$repo" --target-repo "$repo" --issue "$issue_number")"
    break
  fi
  sleep 5
done

if [[ -z "${coverage_output:-}" || -z "${drill_output:-}" || -z "${restore_output:-}" ]]; then
  die "channel backup rehearsal did not observe source issue #${issue_number} in ${backup_branch}"
fi

for expected in \
  "GitClaw Backup Coverage Report" \
  "repository: \`${repo}\`" \
  "issue: \`#${issue_number}\`" \
  'backup_coverage_status: `ok`' \
  'backup_verify_status: `ok`' \
  'issue_indexed: `true`' \
  "issue_backup_path: \`${issue_path}\`" \
  'raw_bodies_included: `false`' \
  'mutation_performed=`false`'; do
  grep -Fq "$expected" <<<"$coverage_output" || die "backup coverage output missing ${expected}"
done
for expected in \
  "GitClaw Backup Drill Report" \
  "repository: \`${repo}\`" \
  "issue: \`#${issue_number}\`" \
  'backup_drill_status: `ok`' \
  'backup_verify_status: `ok`' \
  'backup_coverage_status: `ok`' \
  'restore_plan_status: `ok`' \
  'restore_mode: `dry-run`' \
  'raw_bodies_included: `false`' \
  'mutation_performed: `false`' \
  'github_api_calls_performed: `false`'; do
  grep -Fq "$expected" <<<"$drill_output" || die "backup drill output missing ${expected}"
done
for expected in \
  "GitClaw Backup Restore Plan" \
  "source_repository: \`${repo}\`" \
  "target_repository: \`${repo}\`" \
  "issue: \`#${issue_number}\`" \
  'restore_mode: `dry-run`' \
  "issue_backup_path: \`${issue_path}\`" \
  'raw_bodies_included: `false`'; do
  grep -Fq "$expected" <<<"$restore_output" || die "backup restore-plan output missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$source_hidden_token" "$issue_title"; do
  if grep -Fq "$leaked" <<<"$coverage_output" || grep -Fq "$leaked" <<<"$drill_output" || grep -Fq "$leaked" <<<"$restore_output"; then
    die "backup dry-run output leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels rehearse-backup --id ${rehearsal_id} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id}

Do not leak duplicate hidden token: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel backup rehearsal action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel backup rehearsal receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Backup Rehearsal Action" \
  "requested_channel_command: \`/channels rehearse-backup\`" \
  "channel_backup_rehearsal_status: \`duplicate\`" \
  "rehearsal_issue: \`#${rehearsal_issue_number}\`" \
  "rehearsal_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel backup rehearsal receipt missing ${expected}"
done
[[ "$(backup_rehearsal_notification_count)" == "1" ]] || die "duplicate channel backup rehearsal queued another rehearsal-link notification"
for leaked in "$duplicate_hidden_token" "$rehearsal_id" "$thread_id" "$ingest_message_id" "$notify_message_id" "$full_issue_path"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel backup rehearsal receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$rehearsal_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel-created backup recovery rehearsal and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_BACKUP_REHEARSAL_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include rehearsal ids, message ids, thread ids, account hashes, issue numbers, backup paths, backup branches, source bodies, backup bodies, or restore commands.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$rehearsal_issue_title")" || die "timed out waiting for channel backup rehearsal model follow-up"
wait_for_assistant_count_for_issue "$rehearsal_issue_number" 1 || die "expected model-backed channel backup rehearsal follow-up"
model_comment="$(latest_assistant_comment_for_issue "$rehearsal_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel backup rehearsal search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel backup rehearsal follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel backup rehearsal follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel backup rehearsal follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel backup rehearsal follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel backup rehearsal follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel backup rehearsal follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$source_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$account_id" "$rehearsal_id" "$notify_message_id" "$full_issue_path"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel backup rehearsal follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number}, rehearsal issue #${rehearsal_issue_number} (model follow-up: ${model_url})"
