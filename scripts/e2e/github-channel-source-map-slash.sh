#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-source-map slash action queues provider-visible safe skill-source sequence cards and proves model/tool follow-up.
set -euo pipefail

log() {
  echo "channel-source-map-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-source-map-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-source-map slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-source-map-e2e-${timestamp}"
ingest_message_id="source-map-ingest-${timestamp}"
notify_message_id="source-map-notify-${timestamp}"
source_map_id="source-map-${timestamp}"
requested_source="repo-reader"
source_map_note="Keep source pins reviewed"
account_id="telegram-source-map-account-NOECHO_CHANNEL_SOURCE_MAP_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_SOURCE_MAP_INGEST_${timestamp}"
command_hidden_token="NOECHO_CHANNEL_SOURCE_MAP_COMMAND_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_SOURCE_MAP_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_SOURCE_MAP_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_SOURCE_MAP_CONTEXT_V1"
search_phrase="channel source map unique search fixture phrase"
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

source_map_notification_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$notify_message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

cleanup() {
  if [[ -n "${issue_number:-}" && "$issue_number" != "null" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "channel-source-map slash e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel-source-map slash E2E.

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

source_map_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels source-map ${requested_source} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --map-id ${source_map_id}
Note: ${source_map_note}
Do not include this command hidden token in the receipt: ${command_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$source_map_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel source map action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel source map action receipt"
source_map_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Source Map Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels source-map\`" \
  "channel_source_map_status: \`queued\`" \
  "source_map_mode: \`provider-facing-skill-source-sequence\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "source_map_id_sha256_12: \`" \
  "source_map_id_auto: \`false\`" \
  "requested_source_sha256_12: \`" \
  "normalized_source_sha256_12: \`" \
  "requested_source_bytes: \`11\`" \
  "requested_source_terms: \`1\`" \
  "source_map_note_sha256_12: \`" \
  "source_map_note_bytes: \`25\`" \
  "source_map_note_lines: \`1\`" \
  "source_map_note_source: \`trailing-note\`" \
  "source_map_step_count: \`6\`" \
  "skill_source_status: \`ok\`" \
  "available_source_pins: \`1\`" \
  "parsed_source_pins: \`1\`" \
  "matched_source_pins: \`1\`" \
  "missing_skill_matches: \`0\`" \
  "hash_pinned_sources: \`1\`" \
  "hash_matched_sources: \`1\`" \
  "hash_mismatched_sources: \`0\`" \
  "repo_local_source_refs: \`1\`" \
  "remote_source_refs: \`0\`" \
  "sources_requiring_approval: \`1\`" \
  "remote_fetch_allowed_specs: \`0\`" \
  "sources_with_risk_findings: \`0\`" \
  "selected_source_pins: \`1\`" \
  "selected_skill_matched: \`1\`" \
  "selected_hash_pinned: \`1\`" \
  "selected_hash_matched: \`1\`" \
  "selected_hash_mismatched: \`0\`" \
  "selected_requires_approval: \`1\`" \
  "selected_remote_fetch_allowed: \`0\`" \
  "registry_contact_allowed: \`false\`" \
  "remote_fetch_allowed: \`false\`" \
  "skill_install_allowed: \`false\`" \
  "skill_update_allowed: \`false\`" \
  "source_pin_write_allowed: \`false\`" \
  "installer_scripts_run: \`false\`" \
  "dependency_install_allowed: \`false\`" \
  "source_proposal_issue_created: \`false\`" \
  "model_call_performed: \`false\`" \
  "provider_api_call_performed: \`false\`" \
  "workflow_mutation_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "raw_source_map_id_included: \`false\`" \
  "raw_requested_source_included: \`false\`" \
  "raw_source_refs_included: \`false\`" \
  "raw_source_bodies_included: \`false\`" \
  "raw_skill_bodies_included: \`false\`" \
  "llm_e2e_required_after_channel_source_map_action_change: \`true\`"; do
  grep -Fq -- "$expected" <<<"$source_map_receipt" || die "channel source map receipt missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$source_map_id" "$requested_source" "$source_map_note" ".gitclaw/SKILLS/repo-reader/SKILL.md" "$expected_token"; do
  if grep -Fq -- "$leaked" <<<"$source_map_receipt"; then
    die "channel source map receipt leaked ${leaked}"
  fi
done

[[ "$(source_map_notification_count)" == "1" ]] || die "channel source map did not queue exactly one notification"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
notification_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound") and contains("'"${notify_message_id}"'"))] | join("\n")' <<<"$issue_json")"
for expected in \
  "GitClaw channel skill source map." \
  "Requested source: ${requested_source}" \
  "Skill source status: ok" \
  "Available source pins: 1" \
  "Parsed source pins: 1" \
  "Matched source pins: 1" \
  "Hash pinned sources: 1" \
  "Hash matched sources: 1" \
  "Hash mismatched sources: 0" \
  "Remote fetch allowed pins: 0" \
  "Sources requiring approval: 1" \
  "Matched source: ${requested_source}" \
  "Source kind: repo-local" \
  "Source ref present: true" \
  "Skill matched: true" \
  "Trust level: repo-local" \
  "Install mode: manual-review" \
  "Requires approval: true" \
  "Remote fetch allowed: false" \
  "Hash pinned: true" \
  "Hash matched: true" \
  "Hash mismatched: false" \
  "Source sequence:" \
  "\`/skills sources\`" \
  "\`/skills sources info ${requested_source}\`" \
  "\`/skills sources verify\`" \
  "\`/skills sources lock\`" \
  "\`/skills sources update-plan\`" \
  "\`/skills sources propose ${requested_source} --source <ref> --id <proposal-id>\`" \
  "Note: ${source_map_note}" \
  "Source map hash: " \
  "Source step hash: " \
  "Map source: current GitHub Actions checkout skill-source metadata." \
  "Raw source refs: not included." \
  "Full source bodies: not included." \
  "Full skill bodies: not included." \
  "Registry contact: not performed by this action." \
  "Remote fetch: not performed by this action." \
  "Skill install: not performed by this action." \
  "Source pin write: not performed by this action." \
  "Source proposal issue creation: not performed by this action." \
  "Model call: not performed by this action." \
  "Provider API call: not performed by this action." \
  "Workflow mutation: not performed by this action." \
  "Repository mutation: not performed by this action." \
  "Provider delivery: queued through GitHub channel outbox."; do
  grep -Fq -- "$expected" <<<"$notification_bodies" || die "source_map notification missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$source_map_id" ".gitclaw/SKILLS/repo-reader/SKILL.md" "$expected_token"; do
  if grep -Fq -- "$leaked" <<<"$notification_bodies"; then
    die "source_map notification leaked ${leaked}"
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
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing source_map notify hash ${notify_message_hash}"
for leaked in "$account_id" "$ingest_hidden_token" "$command_hidden_token" "$source_map_id" "$expected_token" "$requested_source" "$source_map_note"; do
  if grep -Fq -- "$leaked" <<<"$outbox_output" || grep -Fq -- "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels source-path ${requested_source} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --map-id ${source_map_id}
Note: ${source_map_note}
Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel source map action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel source map receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Source Map Action" \
  "requested_channel_command: \`/channels source-path\`" \
  "channel_source_map_status: \`duplicate\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "registry_contact_allowed: \`false\`" \
  "remote_fetch_allowed: \`false\`" \
  "skill_install_allowed: \`false\`" \
  "source_pin_write_allowed: \`false\`" \
  "source_proposal_issue_created: \`false\`" \
  "model_call_performed: \`false\`" \
  "repository_mutation_performed: \`false\`"; do
  grep -Fq -- "$expected" <<<"$duplicate_receipt" || die "duplicate channel source map receipt missing ${expected}"
done
[[ "$(source_map_notification_count)" == "1" ]] || die "duplicate channel source map queued another notification"
for leaked in "$duplicate_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$source_map_id" "$requested_source" "$source_map_note" "$expected_token"; do
  if grep -Fq -- "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel source map receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel source map thread and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_SOURCE_MAP_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include provider ids, notification ids, thread ids, message ids, account hashes, source map ids, source map notes, requested source names, source refs, source bodies, skill bodies, issue numbers, or previous channel bodies.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$issue_title")" || die "timed out waiting for channel source map model follow-up"
wait_for_assistant_count_for_issue "$issue_number" 4 || die "expected model-backed channel source map follow-up"
model_comment="$(latest_assistant_comment_for_issue "$issue_number")"

grep -Fq -- "$expected_token" <<<"$model_comment" || die "assistant did not include channel source map search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel source map follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel source map follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel source map follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel source map follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel source map follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel source map follow-up marker missing usage token telemetry"
model_visible_body="$(sed '1{/^<!-- gitclaw:assistant-turn /d;}' <<<"$model_comment")"
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$source_map_id" "$source_map_note" "$account_id"; do
  if grep -Fq -- "$leaked" <<<"$model_comment"; then
    die "model channel source map follow-up leaked ${leaked}"
  fi
done
if grep -Fq -- "$requested_source" <<<"$model_visible_body"; then
  die "model channel source map follow-up visible answer leaked ${requested_source}"
fi

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number} (model follow-up: ${model_url})"
