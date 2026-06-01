#!/usr/bin/env bash
# gitclaw-doctor-live-issue: skills rehearsal action creates a live GitHub conversation issue.
set -euo pipefail

log() {
  echo "skills-rehearse-issue-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-skills-rehearse-issue-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another skills rehearsal issue E2E appears to be running: ${lock_dir}"
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
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ | tr '[:upper:]' '[:lower:]')"
skill_name="repo-reader"
rehearsal_id="rehearsal-${timestamp}"
source_title="GitClaw skills rehearsal E2E ${timestamp}"
source_hidden_token="NOECHO_SKILL_REHEARSAL_SOURCE_${timestamp}"
duplicate_hidden_token="NOECHO_SKILL_REHEARSAL_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_SKILL_REHEARSAL_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_SKILLS_REHEARSAL_CONTEXT_V1"
search_phrase="skills rehearsal unique search fixture phrase"
source_issue_number=""
rehearsal_issue_number=""

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

wait_for_assistant_count() {
  local want="$1"
  wait_for_assistant_count_for_issue "$rehearsal_issue_number" "$want"
}

latest_assistant_comment_for_issue() {
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

cleanup() {
  for number in "$source_issue_number" "$rehearsal_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "skills rehearsal issue e2e cleanup" >/dev/null 2>&1 || true
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
  --body "@gitclaw /skills rehearse ${skill_name} --id ${rehearsal_id}

Create a live rehearsal lane for this reviewed skill.
Do not include this hidden source token: ${source_hidden_token}")"
source_issue_number="${source_issue_url##*/}"

wait_for_run "issues" "$opened_started_at" "$source_title" >/dev/null || die "timed out waiting for skills rehearsal issues run"
wait_for_assistant_count_for_issue "$source_issue_number" 1 || die "expected one skills rehearsal receipt"
receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"

for expected in \
  "GitClaw Skill Rehearsal Issue Action" \
  "Generated without a model call" \
  'model="gitclaw/skills"' \
  "requested_skill_command: \`/skills rehearse\`" \
  "skill_rehearsal_status: \`created\`" \
  "rehearsal_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "requested_skill: \`${skill_name}\`" \
  "matched_skills: \`1\`" \
  "rehearsal_issue_labeled_for_gitclaw: \`true\`" \
  "model_call_performed: \`false\`" \
  "skill_install_allowed: \`false\`" \
  "skill_update_allowed: \`false\`" \
  "active_skill_write_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "raw_skill_body_included: \`false\`" \
  "llm_e2e_required_after_skill_rehearsal_issue_change: \`true\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "skills rehearsal receipt missing ${expected}"
done
for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$rehearsal_id" "Create a live rehearsal lane"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "skills rehearsal receipt leaked ${leaked}"
  fi
done

rehearsal_issue_number="$(sed -n 's/.*rehearsal_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
[[ -n "$rehearsal_issue_number" ]] || die "could not parse rehearsal issue from receipt"
log "skills rehearsal created conversation issue #${rehearsal_issue_number}"

rehearsal_json="$(gh issue view "$rehearsal_issue_number" --repo "$repo" --json title,body,labels)"
rehearsal_title="$(jq -r '.title' <<<"$rehearsal_json")"
rehearsal_body="$(jq -r '.body' <<<"$rehearsal_json")"
rehearsal_labels="$(jq -r '.labels[].name' <<<"$rehearsal_json")"
grep -Fxq "gitclaw" <<<"$rehearsal_labels" || die "rehearsal issue missing gitclaw label"
for expected in \
  "gitclaw:skill-rehearsal-issue" \
  "rehearsal_id: ${rehearsal_id}" \
  "requested_skill: ${skill_name}" \
  "source_issue: #${source_issue_number}" \
  "matched_skills: 1" \
  "skill_install_allowed: false" \
  "active_skill_write_performed: false" \
  "raw_source_body_included: false" \
  "raw_skill_body_included: false" \
  "Use the \`${skill_name}\` skill"; do
  grep -Fq "$expected" <<<"$rehearsal_body" || die "rehearsal issue body missing ${expected}"
done
for leaked in "$source_hidden_token" "Create a live rehearsal lane"; do
  if grep -Fq "$leaked" <<<"$rehearsal_body"; then
    die "rehearsal issue body leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /skills rehearse ${skill_name} --id ${rehearsal_id}

Repeat the same rehearsal.
Do not include this hidden duplicate token: ${duplicate_hidden_token}" >/dev/null

wait_for_run "issue_comment" "$duplicate_started_at" "$source_title" >/dev/null || die "timed out waiting for duplicate skills rehearsal run"
wait_for_assistant_count_for_issue "$source_issue_number" 2 || die "expected duplicate skills rehearsal receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"
for expected in \
  "GitClaw Skill Rehearsal Issue Action" \
  "skill_rehearsal_status: \`existing\`" \
  "rehearsal_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "rehearsal_issue: \`#${rehearsal_issue_number}\`" \
  "raw_source_body_included: \`false\`" \
  "raw_skill_body_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate skills rehearsal receipt missing ${expected}"
done
for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$rehearsal_id" "Repeat the same rehearsal"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate skills rehearsal receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$rehearsal_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this skill rehearsal and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line ends with an all-caps token after the arrow.
That token starts with \`GITCLAW_SKILLS_REHEARSAL_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include the source issue number, rehearsal issue number, skill name, rehearsal id, source body, or hidden sentinels.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run "issue_comment" "$comment_started_at" "$rehearsal_title")" || die "timed out waiting for skills rehearsal model follow-up"
wait_for_assistant_count 1 || die "expected model-backed skills rehearsal follow-up"
model_comment="$(latest_assistant_comment_for_issue "$rehearsal_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include skills rehearsal search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant skills rehearsal follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant skills rehearsal follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant skills rehearsal follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant skills rehearsal follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant skills rehearsal follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant skills rehearsal follow-up marker missing usage token telemetry"

for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$rehearsal_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model skills rehearsal follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}, rehearsal issue #${rehearsal_issue_number} (model follow-up: ${model_url})"
