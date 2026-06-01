#!/usr/bin/env bash
# gitclaw-doctor-live-issue: soul rehearsal action creates a live GitHub conversation issue.
set -euo pipefail

log() {
  echo "soul-rehearse-issue-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-soul-rehearse-issue-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another soul rehearsal issue E2E appears to be running: ${lock_dir}"
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
rehearsal_id="soul-rehearsal-${timestamp}"
source_title="@gitclaw /soul rehearse --target soul --id ${rehearsal_id}"
source_hidden_token="NOECHO_SOUL_REHEARSAL_SOURCE_${timestamp}"
duplicate_hidden_token="NOECHO_SOUL_REHEARSAL_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_SOUL_REHEARSAL_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_SOUL_REHEARSAL_CONTEXT_V1"
search_phrase="soul rehearsal unique search fixture phrase"
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
        gh issue close "$number" --repo "$repo" --comment "soul rehearsal issue e2e cleanup" >/dev/null 2>&1 || true
      fi
    fi
  done
  rm -rf "$lock_dir"
}
trap cleanup EXIT

source_body="GitClaw live soul rehearsal issue E2E.

Create a current-soul conversation lane without copying this raw source request.
Do not include this hidden source token: ${source_hidden_token}"

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
source_url="$(gh issue create \
  --repo "$repo" \
  --title "$source_title" \
  --body "$source_body")"
source_issue_number="${source_url##*/}"

wait_for_run "issues" "$started_at" "$source_title" >/dev/null || die "timed out waiting for soul rehearsal issues run"
wait_for_assistant_count_for_issue "$source_issue_number" 1 || die "expected one soul rehearsal receipt"
receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"

for expected in \
  "GitClaw Soul Rehearsal Issue Action" \
  "Generated without a model call" \
  'model="gitclaw/soul"' \
  "requested_soul_command: \`/soul rehearse\`" \
  "soul_rehearsal_status: \`created\`" \
  "rehearsal_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "normalized_soul_path: \`.gitclaw/SOUL.md\`" \
  "target_category: \`soul\`" \
  "rehearsal_mode: \`github-issue-conversation\`" \
  "rehearsal_issue_labeled_for_gitclaw: \`true\`" \
  "model_call_performed: \`false\`" \
  "context_target_write_allowed: \`false\`" \
  "candidate_soul_generation_allowed: \`false\`" \
  "soul_file_written: \`false\`" \
  "repository_mutation_performed: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "raw_target_body_included: \`false\`" \
  "raw_candidate_soul_included: \`false\`" \
  "llm_e2e_required_after_soul_rehearsal_issue_change: \`true\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "soul rehearsal receipt missing ${expected}"
done
for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$rehearsal_id" "Create a current-soul conversation lane"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "soul rehearsal receipt leaked ${leaked}"
  fi
done

rehearsal_issue_number="$(sed -n 's/.*rehearsal_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
[[ -n "$rehearsal_issue_number" ]] || die "could not parse soul rehearsal issue from receipt"
log "soul rehearsal created conversation issue #${rehearsal_issue_number}"

rehearsal_json="$(gh issue view "$rehearsal_issue_number" --repo "$repo" --json title,body,labels)"
rehearsal_title="$(jq -r '.title' <<<"$rehearsal_json")"
rehearsal_body="$(jq -r '.body' <<<"$rehearsal_json")"
rehearsal_labels="$(jq -r '.labels[].name' <<<"$rehearsal_json")"
grep -Fxq "gitclaw" <<<"$rehearsal_labels" || die "rehearsal issue missing gitclaw label"
for expected in \
  "gitclaw:soul-rehearsal-issue" \
  "rehearsal_id: ${rehearsal_id}" \
  "target_path: .gitclaw/SOUL.md" \
  "target_category: soul" \
  "source_issue: #${source_issue_number}" \
  "rehearsal_mode: github-issue-conversation" \
  "context_target_write_allowed: false" \
  "candidate_soul_generation_allowed: false" \
  "repository_mutation_allowed: false" \
  "raw_source_body_included: false" \
  "raw_target_body_included: false" \
  "raw_candidate_soul_included: false" \
  "Use this issue to rehearse the current \`.gitclaw/SOUL.md\` behavior"; do
  grep -Fq "$expected" <<<"$rehearsal_body" || die "soul rehearsal issue body missing ${expected}"
done
for leaked in "$source_hidden_token" "Create a current-soul conversation lane"; do
  if grep -Fq "$leaked" <<<"$rehearsal_body"; then
    die "soul rehearsal issue body leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /soul rehearse --target soul --id ${rehearsal_id}

Repeat the same soul rehearsal.
Do not include this hidden duplicate token: ${duplicate_hidden_token}" >/dev/null

wait_for_run "issue_comment" "$duplicate_started_at" "$source_title" >/dev/null || die "timed out waiting for duplicate soul rehearsal run"
wait_for_assistant_count_for_issue "$source_issue_number" 2 || die "expected duplicate soul rehearsal receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"
for expected in \
  "GitClaw Soul Rehearsal Issue Action" \
  "soul_rehearsal_status: \`existing\`" \
  "rehearsal_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "rehearsal_issue: \`#${rehearsal_issue_number}\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate soul rehearsal receipt missing ${expected}"
done
for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$rehearsal_id" "Repeat the same soul rehearsal"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate soul rehearsal receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$rehearsal_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this current-soul rehearsal and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
Reply with only the token after the arrow from the matching gitclaw.search_files tool output line.
Do not include the source issue number, rehearsal issue number, soul rehearsal id, source body, target body, or hidden sentinels.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run "issue_comment" "$comment_started_at" "$rehearsal_title")" || die "timed out waiting for soul rehearsal model follow-up"
wait_for_assistant_count 1 || die "expected model-backed soul rehearsal follow-up"
model_comment="$(latest_assistant_comment_for_issue "$rehearsal_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include soul rehearsal search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant soul rehearsal follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant soul rehearsal follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant soul rehearsal follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant soul rehearsal follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant soul rehearsal follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant soul rehearsal follow-up marker missing usage token telemetry"

for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$rehearsal_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model soul rehearsal follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}, rehearsal issue #${rehearsal_issue_number} (model follow-up: ${model_url})"
