#!/usr/bin/env bash
# gitclaw-doctor-live-issue: skills sources propose creates a review conversation issue.
set -euo pipefail

log() {
  echo "skills-sources-propose-issue-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-skills-sources-propose-issue-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another skills sources propose issue E2E appears to be running: ${lock_dir}"
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
source_name="e2e-source-${timestamp}"
proposal_id="${source_name}-review"
source_ref="github:example/${source_name}/.gitclaw/SKILLS/${source_name}/SKILL.md"
source_title="@gitclaw /skills sources propose ${source_name} --source ${source_ref} --skill-path .gitclaw/SKILLS/${source_name}/SKILL.md --id ${proposal_id}"
proposal_title="GitClaw skill source proposal: ${source_name}"
hidden_token="NOECHO_SKILLS_SOURCES_PROPOSE_BODY_${timestamp}"
duplicate_hidden_token="NOECHO_SKILLS_SOURCES_PROPOSE_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_SKILLS_SOURCES_PROPOSE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_SKILLS_SOURCE_PROPOSAL_CONTEXT_V1"
search_phrase="skills source proposal unique search fixture phrase"
source_issue_number=""
proposal_issue_number=""

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
  local display_title="$3"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json "$event" "$started_at" | jq -c --arg title "$display_title" '[.[] | select(.displayTitle == $title)] | sort_by(.createdAt) | reverse | .[0] // empty')"
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
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

error_count() {
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
    errors="$(error_count "$issue_number")"
    if [[ "$errors" != "0" ]]; then
      die "issue #${issue_number} posted ${errors} error marker comment(s)"
    fi
    local got
    got="$(assistant_count "$issue_number")"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

latest_assistant_comment() {
  local issue_number="$1"
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

cleanup() {
  for number in "$source_issue_number" "$proposal_issue_number"; do
    if [[ -n "${number:-}" && "$number" != "null" ]]; then
      gh issue edit "$number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
      if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
        gh issue close "$number" --repo "$repo" --comment "skills sources propose issue e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

source_body="GitClaw live skills sources propose issue E2E.

Review this external source pin without copying this raw source request.
Do not include this hidden source token: ${hidden_token}"

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
source_url="$(gh issue create \
  --repo "$repo" \
  --title "$source_title" \
  --body "$source_body")"
source_issue_number="${source_url##*/}"

wait_for_run "issues" "$started_at" "$source_title" >/dev/null || die "timed out waiting for skills sources propose issues run"
wait_for_assistant_count_for_issue "$source_issue_number" 1 || die "expected one skills sources propose receipt"
receipt="$(latest_assistant_comment "$source_issue_number")"
source_ref_sha="$(printf "%s" "$source_ref" | shasum -a 256 | awk '{print substr($1,1,12)}')"

for expected in \
  "GitClaw Skill Source Proposal Issue Action" \
  "Generated without a model call" \
  'model="gitclaw/skills"' \
  "requested_skill_command: \`/skills sources propose\`" \
  "skill_source_proposal_status: \`created\`" \
  "skill_source_proposal_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "source_name: \`${source_name}\`" \
  "source_pin_path: \`.gitclaw/skill-sources/${source_name}.yaml\`" \
  "proposed_skill_path: \`.gitclaw/SKILLS/${source_name}/SKILL.md\`" \
  "source_kind: \`github\`" \
  "source_ref_sha256_12: \`${source_ref_sha}\`" \
  "trust_level: \`review-pending\`" \
  "install_mode: \`proposal-only\`" \
  "proposal_store: \`github-issue-to-git-reviewed-skill-source-pin\`" \
  "proposal_issue_labeled_for_gitclaw: \`true\`" \
  "model_call_performed: \`false\`" \
  "raw_source_ref_included: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "source_pin_file_written: \`false\`" \
  "active_skill_write_performed: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "llm_e2e_required_after_skill_source_proposal_issue_change: \`true\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "skills sources propose receipt missing ${expected}"
done
for leaked in "$source_ref" "$hidden_token" "Review this external source pin"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "skills sources propose receipt leaked ${leaked}"
  fi
done

proposal_issue_number="$(sed -n 's/.*skill_source_proposal_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
[[ -n "$proposal_issue_number" ]] || die "could not parse proposal issue from receipt"
log "skills sources propose created proposal issue #${proposal_issue_number}"

proposal_json="$(gh issue view "$proposal_issue_number" --repo "$repo" --json title,body,state,labels)"
proposal_body="$(jq -r '.body' <<<"$proposal_json")"
actual_proposal_title="$(jq -r '.title' <<<"$proposal_json")"
[[ "$actual_proposal_title" == "$proposal_title" ]] || die "unexpected proposal issue title: ${actual_proposal_title}"
jq -e '.labels[].name | select(. == "gitclaw")' <<<"$proposal_json" >/dev/null || die "proposal issue missing gitclaw label"
for expected in \
  "gitclaw:skill-source-proposal-issue" \
  "proposal_id: ${proposal_id}" \
  "source_name: ${source_name}" \
  "source_pin_path: .gitclaw/skill-sources/${source_name}.yaml" \
  "proposed_skill_path: .gitclaw/SKILLS/${source_name}/SKILL.md" \
  "source_kind: github" \
  "source_ref_sha256_12: ${source_ref_sha}" \
  "trust_level: review-pending" \
  "install_mode: proposal-only" \
  "requires_approval: true" \
  "remote_fetch_allowed: false" \
  "source_issue: #${source_issue_number}" \
  "raw_source_ref_included: false" \
  "raw_source_body_included: false" \
  "source_pin_file_written: false" \
  "active_skill_write_performed: false"; do
  grep -Fq "$expected" <<<"$proposal_body" || die "proposal issue body missing ${expected}"
done
for leaked in "$source_ref" "$hidden_token" "Review this external source pin"; do
  if grep -Fq "$leaked" <<<"$proposal_body"; then
    die "proposal issue body leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /skills sources propose ${source_name} --source ${source_ref} --id ${proposal_id}

Repeat the same source proposal.
Do not include this hidden duplicate token: ${duplicate_hidden_token}" >/dev/null

wait_for_run "issue_comment" "$duplicate_started_at" "$source_title" >/dev/null || die "timed out waiting for duplicate skills sources propose run"
wait_for_assistant_count_for_issue "$source_issue_number" 2 || die "expected duplicate skills sources propose receipt"
duplicate_receipt="$(latest_assistant_comment "$source_issue_number")"
for expected in \
  "GitClaw Skill Source Proposal Issue Action" \
  "skill_source_proposal_status: \`existing\`" \
  "skill_source_proposal_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "skill_source_proposal_issue: \`#${proposal_issue_number}\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate skills sources propose receipt missing ${expected}"
done
for leaked in "$source_ref" "$hidden_token" "$duplicate_hidden_token" "Repeat the same source proposal"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate skills sources propose receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$proposal_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this skill source proposal review and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
Reply with only the token after the arrow from the matching gitclaw.search_files tool output line.
Do not include the source ref, proposal id, source issue body, or hidden sentinels.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run "issue_comment" "$comment_started_at" "$proposal_title")" || die "timed out waiting for skills sources propose model follow-up"
wait_for_assistant_count_for_issue "$proposal_issue_number" 1 || die "expected model-backed skills sources propose follow-up"
model_comment="$(latest_assistant_comment "$proposal_issue_number")"
model_run_id="$(jq -r '.databaseId' <<<"$model_run_json")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include skills source proposal search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant skills source proposal follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant skills source proposal follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant skills source proposal follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant skills source proposal follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant skills source proposal follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant skills source proposal follow-up marker missing usage token telemetry"
for leaked in "$source_ref" "$hidden_token" "$duplicate_hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "skills source proposal model follow-up leaked ${leaked}"
  fi
done

log "skills sources propose issue E2E passed with model run ${model_run_id}"
