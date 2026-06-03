#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-skill-map slash action queues provider-visible safe skill sequence cards and proves model/tool follow-up.
set -euo pipefail

log() {
  echo "channel-skill-map-slash-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
ingest_workflow="${GITCLAW_E2E_INGEST_WORKFLOW:-.github/workflows/gitclaw-channel-ingest.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-skill-map-slash-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-skill-map slash E2E appears to be running: ${lock_dir}"
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
thread_id="channel-skill-map-e2e-${timestamp}"
ingest_message_id="skill-map-ingest-${timestamp}"
notify_message_id="skill-map-notify-${timestamp}"
skill_map_id="skill-map-${timestamp}"
requested_skill="repo-reader"
skill_map_note="Keep skill changes reviewed"
account_id="telegram-skill-map-account-NOECHO_CHANNEL_SKILL_MAP_ACCOUNT_${timestamp}"
ingest_hidden_token="NOECHO_CHANNEL_SKILL_MAP_INGEST_${timestamp}"
command_hidden_token="NOECHO_CHANNEL_SKILL_MAP_COMMAND_${timestamp}"
duplicate_hidden_token="NOECHO_CHANNEL_SKILL_MAP_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_SKILL_MAP_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_SKILL_MAP_CONTEXT_V1"
search_phrase="channel skill map unique search fixture phrase"
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

skill_map_notification_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    | jq -r --arg msg "$notify_message_id" '[.comments[] | select(.body | contains("<!-- gitclaw:channel-outbound") and contains($msg))] | length'
}

cleanup() {
  if [[ -n "${issue_number:-}" && "$issue_number" != "null" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "channel-skill-map slash e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

ingest_body="@gitclaw /channels

Mirrored Telegram thread for channel-skill-map slash E2E.

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

skill_map_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels skill-map ${requested_skill} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --map-id ${skill_map_id}
Note: ${skill_map_note}
Do not include this command hidden token in the receipt: ${command_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$skill_map_started_at" "$issue_title" >/dev/null || die "timed out waiting for channel skill map action"
wait_for_assistant_count_for_issue "$issue_number" 2 || die "expected channel skill map action receipt"
skill_map_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Skill Map Action" \
  "Generated without a model call" \
  'model="gitclaw/channels"' \
  "requested_channel_command: \`/channels skill-map\`" \
  "channel_skill_map_status: \`queued\`" \
  "skill_map_mode: \`provider-facing-skill-sequence\`" \
  "notification_target_issue: \`#${issue_number}\`" \
  "notification_queued: \`true\`" \
  "notification_duplicate_suppressed: \`false\`" \
  "target_from_current_channel_issue: \`true\`" \
  "skill_map_id_sha256_12: \`" \
  "skill_map_id_auto: \`false\`" \
  "requested_skill_sha256_12: \`" \
  "normalized_skill_sha256_12: \`" \
  "requested_skill_bytes: \`11\`" \
  "requested_skill_terms: \`1\`" \
  "skill_map_note_sha256_12: \`" \
  "skill_map_note_bytes: \`27\`" \
  "skill_map_note_lines: \`1\`" \
  "skill_map_note_source: \`trailing-note\`" \
  "skill_map_step_count: \`6\`" \
  "skill_map_step_sha256_12: \`" \
  "skill_map_snapshot_sha256_12: \`" \
  "available_skills: \`" \
  "enabled_skills: \`" \
  "disabled_skills: \`" \
  "allowlist_blocked_skills: \`" \
  "selected_skills: \`" \
  "skills_with_frontmatter: \`" \
  "skills_with_descriptions: \`" \
  "skills_missing_requirements: \`" \
  "matched_skills: \`1\`" \
  "validation_status: \`ok\`" \
  "validation_errors: \`0\`" \
  "validation_warnings: \`0\`" \
  "skill_install_allowed: \`false\`" \
  "skill_update_allowed: \`false\`" \
  "registry_contact_allowed: \`false\`" \
  "installer_scripts_run: \`false\`" \
  "skill_proposal_issue_created: \`false\`" \
  "skill_rehearsal_issue_created: \`false\`" \
  "skill_note_issue_created: \`false\`" \
  "model_call_performed: \`false\`" \
  "provider_api_call_performed: \`false\`" \
  "workflow_mutation_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "provider_delivery_strategy: \`channel-outbox + channel-delivery\`" \
  "raw_thread_id_included: \`false\`" \
  "raw_source_message_id_included: \`false\`" \
  "raw_notify_message_id_included: \`false\`" \
  "raw_skill_map_id_included: \`false\`" \
  "raw_requested_skill_included: \`false\`" \
  "raw_skill_map_note_included: \`false\`" \
  "raw_skill_map_steps_included: \`false\`" \
  "raw_skill_names_included: \`false\`" \
  "raw_skill_paths_included: \`false\`" \
  "raw_skill_descriptions_included: \`false\`" \
  "raw_skill_bodies_included: \`false\`" \
  "raw_tool_outputs_included: \`false\`" \
  "raw_channel_message_body_included: \`false\`" \
  "llm_e2e_required_after_channel_skill_map_action_change: \`true\`"; do
  grep -Fq -- "$expected" <<<"$skill_map_receipt" || die "channel skill map receipt missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$skill_map_id" "$requested_skill" "$skill_map_note" "GITCLAW_SKILL_CONTEXT_V1" "Use GitClaw's read-only repository context" "$expected_token"; do
  if grep -Fq -- "$leaked" <<<"$skill_map_receipt"; then
    die "channel skill map receipt leaked ${leaked}"
  fi
done

[[ "$(skill_map_notification_count)" == "1" ]] || die "channel skill map did not queue exactly one notification"
issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,comments,labels)"
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "channel issue lost channel-thread marker"
notification_bodies="$(jq -r '[.comments[].body | select(contains("<!-- gitclaw:channel-outbound") and contains("'"${notify_message_id}"'"))] | join("\n")' <<<"$issue_json")"
for expected in \
  "GitClaw channel skill map." \
  "Requested skill: ${requested_skill}" \
  "Available skills: " \
  "Enabled skills: " \
  "Disabled skills: " \
  "Allowlist blocked skills: " \
  "Selected skills for this turn: " \
  "Matched skills: 1" \
  "Skills with frontmatter: " \
  "Skills with descriptions: " \
  "Skills missing requirements: " \
  "Validation status: ok" \
  "Skill sequence:" \
  "\`/channels skills --message-id <id> --notify-message-id <id>\`" \
  "\`/channels skill-search ${requested_skill} --message-id <id> --notify-message-id <id>\`" \
  "\`/channels skill-info ${requested_skill} --message-id <id> --notify-message-id <id>\`" \
  "\`/channels propose-skill ${requested_skill} --message-id <id> --notify-message-id <id>\`" \
  "\`/channels rehearse-skill ${requested_skill} --id <rehearsal-id> --message-id <id> --notify-message-id <id>\`" \
  "\`/channels skill-note --skill ${requested_skill} --note-id <note-id> --message-id <id> --notify-message-id <id>\`" \
  "Note: ${skill_map_note}" \
  "Note hash: " \
  "Skill map hash: " \
  "Skill step hash: " \
  "Map source: current GitHub Actions checkout skill metadata." \
  "Full skill bodies: not included." \
  "Skill install: not performed by this action." \
  "Skill update: not performed by this action." \
  "Registry contact: not performed by this action." \
  "Installer scripts: not run by this action." \
  "Skill proposal issue creation: not performed by this action." \
  "Skill rehearsal issue creation: not performed by this action." \
  "Skill-note issue creation: not performed by this action." \
  "Model call: not performed by this action." \
  "Provider API call: not performed by this action." \
  "Workflow mutation: not performed by this action." \
  "Repository mutation: not performed by this action." \
  "Provider delivery: queued through GitHub channel outbox."; do
  grep -Fq -- "$expected" <<<"$notification_bodies" || die "skill_map notification missing ${expected}"
done
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$skill_map_id" "GITCLAW_SKILL_CONTEXT_V1" "$expected_token"; do
  if grep -Fq -- "$leaked" <<<"$notification_bodies"; then
    die "skill_map notification leaked ${leaked}"
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
jq -e --arg hash "$notify_message_hash" '.messages[] | select(.kind == "channel-outbound" and .outbound_message_sha256_12 == $hash)' "$outbox_file" >/dev/null || die "outbox file missing skill_map notify hash ${notify_message_hash}"
for leaked in "$account_id" "$ingest_hidden_token" "$command_hidden_token" "$skill_map_id" "$expected_token" "$requested_skill" "$skill_map_note" "GITCLAW_SKILL_CONTEXT_V1"; do
  if grep -Fq -- "$leaked" <<<"$outbox_output" || grep -Fq -- "$leaked" "$outbox_file"; then
    die "metadata-only outbox leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels skill-path ${requested_skill} --message-id ${ingest_message_id} --notify-message-id ${notify_message_id} --map-id ${skill_map_id}
Note: ${skill_map_note}
Do not include this duplicate hidden token in any receipt: ${duplicate_hidden_token}" >/dev/null

wait_for_issue_comment_run_for_title "$duplicate_started_at" "$issue_title" >/dev/null || die "timed out waiting for duplicate channel skill map action"
wait_for_assistant_count_for_issue "$issue_number" 3 || die "expected duplicate channel skill map receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$issue_number")"
for expected in \
  "GitClaw Channel Skill Map Action" \
  "requested_channel_command: \`/channels skill-path\`" \
  "channel_skill_map_status: \`duplicate\`" \
  "notification_queued: \`false\`" \
  "notification_duplicate_suppressed: \`true\`" \
  "skill_install_allowed: \`false\`" \
  "skill_update_allowed: \`false\`" \
  "registry_contact_allowed: \`false\`" \
  "installer_scripts_run: \`false\`" \
  "skill_proposal_issue_created: \`false\`" \
  "skill_rehearsal_issue_created: \`false\`" \
  "skill_note_issue_created: \`false\`" \
  "model_call_performed: \`false\`" \
  "provider_api_call_performed: \`false\`" \
  "workflow_mutation_performed: \`false\`" \
  "repository_mutation_performed: \`false\`"; do
  grep -Fq -- "$expected" <<<"$duplicate_receipt" || die "duplicate channel skill map receipt missing ${expected}"
done
[[ "$(skill_map_notification_count)" == "1" ]] || die "duplicate channel skill map queued another notification"
for leaked in "$duplicate_hidden_token" "$thread_id" "$ingest_message_id" "$notify_message_id" "$skill_map_id" "$requested_skill" "$skill_map_note" "GITCLAW_SKILL_CONTEXT_V1" "$expected_token"; do
  if grep -Fq -- "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate channel skill map receipt leaked ${leaked}"
  fi
done

model_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this channel skill map thread and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
The exact answer starts with \`GITCLAW_CHANNEL_SKILL_MAP_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include provider ids, notification ids, thread ids, message ids, account hashes, skill map ids, skill map notes, requested skill names, skill steps, skill bodies, issue numbers, or previous channel bodies.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run_for_title "$model_started_at" "$issue_title")" || die "timed out waiting for channel skill map model follow-up"
wait_for_assistant_count_for_issue "$issue_number" 4 || die "expected model-backed channel skill map follow-up"
model_comment="$(latest_assistant_comment_for_issue "$issue_number")"

grep -Fq -- "$expected_token" <<<"$model_comment" || die "assistant did not include channel skill map search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant channel skill map follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant channel skill map follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant channel skill map follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant channel skill map follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant channel skill map follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant channel skill map follow-up marker missing usage token telemetry"
for leaked in "$ingest_hidden_token" "$command_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$skill_map_id" "$skill_map_note" "$account_id" "GITCLAW_SKILL_CONTEXT_V1"; do
  if grep -Fq -- "$leaked" <<<"$model_comment"; then
    die "model channel skill map follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for channel issue #${issue_number} (model follow-up: ${model_url})"
