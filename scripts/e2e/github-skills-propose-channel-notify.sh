#!/usr/bin/env bash
# gitclaw-doctor-live-issue: skills propose can notify reviewed channel routes and then prove normal LLM/tool chat.
set -euo pipefail

log() {
  echo "skills-propose-channel-notify-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-skills-propose-channel-notify-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another skills propose channel notify E2E appears to be running: ${lock_dir}"
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
skill_name="e2e-skill-notify-${timestamp}"
source_title="GitClaw skills proposal channel notify E2E ${timestamp}"
account_id="telegram-skills-notify-account-NOECHO_SKILLS_NOTIFY_ACCOUNT_${timestamp}"
hidden_token="NOECHO_SKILLS_NOTIFY_BODY_${timestamp}"
duplicate_hidden_token="NOECHO_SKILLS_NOTIFY_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_SKILLS_NOTIFY_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_SKILLS_NOTIFY_CONTEXT_V1"
search_phrase="skills propose channel notify unique search fixture phrase"
source_issue_number=""
proposal_issue_number=""
channel_issue_number=""

run_list_json() {
  local event="$1"
  local started_at="$2"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event "$event" \
    --created ">=$started_at" \
    --limit 20 \
    --json databaseId,status,conclusion,createdAt,url,displayTitle \
    --jq '.'
}

wait_for_run() {
  local event="$1"
  local started_at="$2"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json "$event" "$started_at" | jq -c --arg title "$source_title" '[.[] | select(.displayTitle == $title)] | sort_by(.createdAt) | reverse | .[0] // empty')"
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

assistant_count() {
  gh issue view "$source_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

error_count() {
  gh issue view "$source_issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("<!-- gitclaw:error"))] | length'
}

wait_for_assistant_count() {
  local want="$1"
  for _ in {1..90}; do
    local errors
    errors="$(error_count)"
    if [[ "$errors" != "0" ]]; then
      die "source issue posted ${errors} error marker comment(s)"
    fi
    local got
    got="$(assistant_count)"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

latest_assistant_comment() {
  gh issue view "$source_issue_number" \
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
  for number in "$source_issue_number" "$proposal_issue_number" "$channel_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "skills propose channel notify e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

source_body="@gitclaw /skills propose ${skill_name} --notify-route ${notify_route}

Capture a reusable skill proposal, then notify the reviewed channel route.
Do not include this hidden source token: ${hidden_token}"

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
source_url="$(gh issue create \
  --repo "$repo" \
  --title "$source_title" \
  --body "$source_body" \
  --label gitclaw)"
source_issue_number="${source_url##*/}"

wait_for_run "issues" "$started_at" >/dev/null || die "timed out waiting for skills propose notify issues run"
wait_for_assistant_count 1 || die "expected one skills propose notify receipt"
receipt="$(latest_assistant_comment)"

for expected in \
  "GitClaw Skill Proposal Issue Action" \
  "Generated without a model call" \
  'model="gitclaw/skills"' \
  "requested_skill_command: \`/skills propose\`" \
  "proposal_issue_status: \`created\`" \
  "proposal_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "planned_proposal_action: \`propose-create\`" \
  "safe_name_candidate: \`${skill_name}\`" \
  "proposal_store: \`github-issue-to-git-reviewed-proposal-file\`" \
  "channel_notification_requested: \`true\`" \
  "channel_notification_routes: \`1\`" \
  "channel_notification_queued: \`1\`" \
  "channel_notification_duplicates: \`0\`" \
  "channel_notification_target_issues_created: \`1\`" \
  "raw_channel_routes_included: \`false\`" \
  "raw_channel_notification_body_included: \`false\`" \
  "provider_delivery_performed: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "raw_proposal_issue_body_included: \`false\`" \
  "raw_skill_body_included: \`false\`" \
  "proposal_file_written: \`false\`" \
  "active_skill_write_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "llm_e2e_required_after_skill_proposal_issue_change: \`true\`" \
  "channel=\`telegram\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "skills propose notify receipt missing ${expected}"
done
for leaked in "$hidden_token" "$notify_route" "gitclaw-skill-proposal-${skill_name}" "Capture a reusable skill proposal"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "skills propose notify receipt leaked ${leaked}"
  fi
done

proposal_issue_number="$(sed -n 's/.*proposal_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
channel_issue_number="$(sed -n 's/.*destination=`01` target_issue=`#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
[[ -n "$proposal_issue_number" ]] || die "could not parse skill proposal issue from receipt"
[[ -n "$channel_issue_number" ]] || die "could not parse channel notification issue from receipt"
log "created proposal issue #${proposal_issue_number} and queued channel notification on #${channel_issue_number}"

proposal_json="$(gh issue view "$proposal_issue_number" --repo "$repo" --json title,body,state)"
proposal_title="$(jq -r '.title' <<<"$proposal_json")"
proposal_body="$(jq -r '.body' <<<"$proposal_json")"
[[ "$proposal_title" == "GitClaw skill proposal: ${skill_name}" ]] || die "unexpected skill proposal issue title: ${proposal_title}"
for expected in \
  "gitclaw:skill-proposal-issue" \
  "proposal_name: ${skill_name}" \
  "planned_action: propose-create" \
  "proposal_path: .gitclaw/skill-proposals/${skill_name}/PROPOSAL.md" \
  "destination_path: .gitclaw/SKILLS/${skill_name}/SKILL.md" \
  "source_issue: #${source_issue_number}" \
  "raw_source_body_included: false" \
  "raw_proposal_body_included: false" \
  "active_skill_write_performed: false"; do
  grep -Fq "$expected" <<<"$proposal_body" || die "skill proposal issue body missing ${expected}"
done
for leaked in "$hidden_token" "$notify_route" "Capture a reusable skill proposal"; do
  if grep -Fq "$leaked" <<<"$proposal_body"; then
    die "skill proposal issue body leaked ${leaked}"
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
  "message_id=\"gitclaw-skill-proposal-${skill_name}\"" \
  "GitClaw skill proposal" \
  "Review issue: #${proposal_issue_number} https://github.com/${repo}/issues/${proposal_issue_number}" \
  "Source issue: #${source_issue_number} https://github.com/${repo}/issues/${source_issue_number}" \
  "Proposal name: ${skill_name}" \
  "Planned action: propose-create" \
  "Proposal path: .gitclaw/skill-proposals/${skill_name}/PROPOSAL.md" \
  "Destination path: .gitclaw/SKILLS/${skill_name}/SKILL.md" \
  "Review PR required: true" \
  "Active skill written: false"; do
  grep -Fq "$expected" <<<"$channel_comments" || die "channel notification comments missing ${expected}"
done
for leaked in "$hidden_token" "Capture a reusable skill proposal"; do
  if grep -Fq "$leaked" <<<"$channel_comments"; then
    die "channel notification leaked ${leaked}"
  fi
done

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL=telegram \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$channel_issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "pending=1" <<<"$outbox_output" || die "channel outbox did not report pending skill notification: ${outbox_output}"
grep -Fq '"pending_messages": 1' "$outbox_file" || die "channel outbox file missing pending skill notification"
if grep -Fq "$skill_name" <<<"$outbox_output" || grep -Fq "$skill_name" "$outbox_file"; then
  die "channel outbox leaked skill notification body without --include-body"
fi
if grep -Fq "$account_id" <<<"$outbox_output" || grep -Fq "$account_id" "$outbox_file"; then
  die "channel outbox leaked account id"
fi

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /skills propose ${skill_name} --notify-route ${notify_route}

Repeat the same skill proposal notification.
Do not include this hidden duplicate token: ${duplicate_hidden_token}" >/dev/null

wait_for_run "issue_comment" "$duplicate_started_at" >/dev/null || die "timed out waiting for duplicate skills propose notify run"
wait_for_assistant_count 2 || die "expected duplicate skills propose notify receipt"
duplicate_receipt="$(latest_assistant_comment)"
for expected in \
  "GitClaw Skill Proposal Issue Action" \
  "proposal_issue_status: \`existing\`" \
  "duplicate_suppressed: \`true\`" \
  "proposal_issue: \`#${proposal_issue_number}\`" \
  "channel_notification_requested: \`true\`" \
  "channel_notification_queued: \`0\`" \
  "channel_notification_duplicates: \`1\`" \
  "raw_channel_notification_body_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate skills propose notify receipt missing ${expected}"
done
[[ "$(outbound_comment_count)" == "1" ]] || die "duplicate skill notification created another outbound comment"
for leaked in "$hidden_token" "$duplicate_hidden_token" "$notify_route" "Repeat the same skill proposal notification"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate skills propose notify receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the skills proposal channel notification and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line ends with an all-caps token after the arrow.
That token starts with \`GITCLAW_SKILLS_NOTIFY_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
The exact token includes \`SKILLS_NOTIFY\`; do not insert or remove words.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include the proposal issue number, channel issue number, route name, account hash, skill name, source body, proposal path, or hidden sentinels.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run "issue_comment" "$comment_started_at")" || die "timed out waiting for skills propose notify model follow-up"
wait_for_assistant_count 3 || die "expected model-backed skills propose notify follow-up"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include skills propose notify search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant skills propose notify follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant skills propose notify follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant skills propose notify follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant skills propose notify follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant skills propose notify follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant skills propose notify follow-up marker missing usage token telemetry"

for leaked in "$hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$notify_route" "$account_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model skills propose notify follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}, proposal issue #${proposal_issue_number}, channel issue #${channel_issue_number} (model follow-up: ${model_url})"
