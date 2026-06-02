#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel backup-search recalls from the real backup branch and proves model/tool follow-up.
set -euo pipefail

log() {
  echo "channel-backup-search-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
backup_branch="${GITCLAW_E2E_BACKUP_BRANCH:-gitclaw-backups}"
lock_dir="/tmp/gitclaw-channel-backup-search-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel backup-search slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-backup-search-e2e-${timestamp}"
ingest_message_id="backup-search-ingest-${timestamp}"
notify_message_id="backup-search-notify-${timestamp}"
backup_search_id="channel-backup-search-${timestamp}"
backup_anchor="channel-backup-search-anchor-${timestamp}"
account_id="telegram-backup-search-account-NOECHO_CHANNEL_BACKUP_SEARCH_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_BACKUP_SEARCH_INGEST_${timestamp}"
command_hidden_token="NOECHO_CHANNEL_BACKUP_SEARCH_COMMAND_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_BACKUP_SEARCH_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_BACKUP_SEARCH_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_BACKUP_SEARCH_CONTEXT_V1"
search_phrase="channel backup search unique search fixture phrase"
notify_message_hash="$(sha256_12 "$notify_message_id")"
issue_number=""
issue_title="GitClaw ${channel} thread ${thread_id}"
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

backup_search_notification_count() {
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
      gh issue close "$issue_number" --repo "$repo" --comment "channel backup-search slash e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel backup-search slash E2E.
Archive recall seed: ${backup_anchor}

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
  if grep -Fq "$ingest_hidden_token" <<<"$candidate_report" || grep -Fq "$backup_anchor" <<<"$candidate_report"; then
    die "initial channel report leaked ingest body material"
  fi
  if grep -Fq "GitClaw Channel Report" <<<"$candidate_report" && grep -Fq 'channel_thread_issue: `true`' <<<"$candidate_report"; then
    initial_report="$candidate_report"
    break
  fi
  sleep 5
done
[[ -n "$initial_report" ]] || die "expected initial channel report"

repo_key="${repo//\//__}"
issue_padded="$(printf "%06d" "$issue_number")"
issue_path="issues/${issue_padded}.json"
index_path=""
backup_search_output=""
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
    backup_search_output="$(go run ./cmd/gitclaw backup search --root "${backup_checkout}/.gitclaw/backups" --repo "$repo" --query "$backup_anchor" --max-results 5)"
    break
  fi
  sleep 5
done

if [[ -z "$backup_search_output" ]]; then
  die "channel backup-search did not observe source issue #${issue_number} in ${backup_branch}"
fi
for expected in \
  "GitClaw Backup Search Report" \
  "repository: \`${repo}\`" \
  'backup_search_status: `ok`' \
  'backup_verify_status: `ok`' \
  'query_sha256_12:' \
  'results_returned: `' \
  'raw_bodies_included: `false`' \
  "issue=\`#${issue_number}\`" \
  'line_sha256_12='; do
  grep -Fq "$expected" <<<"$backup_search_output" || die "local fetched backup-search output missing ${expected}"
done
grep -Eq 'results_returned: `([1-9][0-9]*)`' <<<"$backup_search_output" || die "local fetched backup-search output did not report at least one result"
for leaked in "$ingest_hidden_token" "$backup_anchor" "$issue_title"; do
  if grep -Fq "$leaked" <<<"$backup_search_output"; then
    die "local fetched backup-search output leaked ${leaked}"
  fi
done

search_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels backup-search ${backup_anchor} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --search-id ${backup_search_id} --max-results 5
Do not include this command hidden token in the receipt: ${command_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$search_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel backup-search action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel backup-search action receipt"
search_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Backup Search Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels backup-search\`" \
  "channel_backup_search_status: \`queued\`" \
  "backup_search_status: \`ok\`" \
  "backup_verify_status: \`ok\`" \
  "backup_fetch_status: \`fetched\`" \
  "backup_branch: \`${backup_branch}\`" \
  "search_mode: \`gitclaw-backups-local-lexical\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "search_id_sha256_12: \`" \
  "query_sha256_12: \`" \
  "query_terms: \`" \
  "query_bytes: \`" \
  "query_source: \`positional\`" \
  "max_results: \`5\`" \
  "issue_count: \`" \
  "matched_issues: \`" \
  "matched_lines: \`" \
  "results_returned: \`" \
  "notification_body_sha256_12: \`" \
  "model_call_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "backup_branch_write_performed: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox + channel-delivery\`" \
  "raw_query_included: \`false\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_search_id_included: \`false\`" \
  "raw_backup_root_included: \`false\`" \
  "raw_backup_paths_included: \`false\`" \
  "raw_backup_payloads_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "raw_issue_bodies_included: \`false\`" \
  "raw_comment_bodies_included: \`false\`" \
  "raw_transcript_bodies_included: \`false\`" \
  "raw_prompts_included: \`false\`" \
  "raw_tool_outputs_included: \`false\`" \
  "llm_e2e_required_after_channel_backup_search_change: \`true\`"; do
  grep -Fq "$expected" <<<"$search_receipt" || die "channel backup-search receipt missing ${expected}"
done
grep -Eq 'results_returned: `([1-9][0-9]*)`' <<<"$search_receipt" || die "channel backup-search receipt did not report at least one result"
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$backup_anchor" "$thread_id" "$ingest_message_id" "$notify_message_id" "$backup_search_id" "$expected_token" "$repo_key"; do
  if grep -Fq "$leaked" <<<"$search_receipt"; then
    die "channel backup-search receipt leaked ${leaked}"
  fi
done

[[ "$(backup_search_notification_count)" == "1" ]] || die "channel backup-search did not queue exactly one notification"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
notification_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound") and contains("'"${notify_message_id}"'"))] | join("\n")' <<<"$issue_json")"
for expected in \
  "GitClaw channel backup search" \
  "Backup search status: ok" \
  "Backup verify status: ok" \
  "Backup branch: ${backup_branch}" \
  "Backup fetch status: fetched" \
  "Query hash: " \
  "Query terms: " \
  "Max results: 5" \
  "Issue count: " \
  "Matched issues: " \
  "Matched lines: " \
  "Results returned: " \
  "Search id hash: " \
  "issue=#${issue_number}" \
  "line_sha256_12=" \
  "Raw backup payloads, channel bodies, issue bodies, comment bodies, transcript messages, prompts, tool outputs, and raw search queries are not included." \
  "Model call: not performed by this action." \
  "Repository mutation: not performed by this action." \
  "Backup branch write: not performed by this action." \
  "Provider delivery: queued through GitHub channel outbox."; do
  grep -Fq "$expected" <<<"$notification_bodies" || die "backup-search notification missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$backup_anchor" "$backup_search_id" "$expected_token" "$issue_title"; do
  if grep -Fq "$leaked" <<<"$notification_bodies"; then
    die "backup-search notification leaked ${leaked}"
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
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing backup-search notify hash ${notify_message_hash}"
for leaked in "$account_id" "$ingest_hidden_token" "$command_hidden_token" "$backup_anchor" "$backup_search_id" "$expected_token"; do
  if grep -Fq "$leaked" <<<"$outbox_output" || grep -Fq "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels search-backups ${backup_anchor} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --search-id ${backup_search_id} --max-results 5
Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel backup-search action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel backup-search receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Backup Search Action" \
  "requested_channel_command: \`/channels search-backups\`" \
  "channel_backup_search_status: \`duplicate\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "model_call_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "backup_branch_write_performed: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate channel backup-search receipt missing ${expected}"
done
[[ "$(backup_search_notification_count)" == "1" ]] || die "duplicate channel backup-search queued another notification"
for leaked in "$duplicate_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$backup_search_id" "$backup_anchor" "$expected_token" "$repo_key"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel backup-search receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel backup-search thread and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_BACKUP_SEARCH_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include provider ids, notification ids, thread ids, message ids, account hashes, search ids, issue numbers, backup paths, backup branches, archive recall anchors, or previous channel bodies.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$issue_title")" || die "timed out waiting for channel backup-search model follow-up"
wait_for_assistant_count_for_issue "$issue_number" 4 || die "expected model-backed channel backup-search follow-up"
model_comment="$(latest_assistant_comment_for_issue "$issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include channel backup-search search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel backup-search follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel backup-search follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel backup-search follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel backup-search follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel backup-search follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel backup-search follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$backup_search_id" "$account_id" "$backup_anchor"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model channel backup-search follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number} (model follow-up: ${model_url})"
