#!/usr/bin/env bash
# gitclaw-doctor-live-issue: checkpoint rehearsal action creates a rollback conversation issue.
set -euo pipefail

log() {
  echo "checkpoints-rehearse-issue-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-checkpoints-rehearse-issue-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another checkpoint rehearsal issue E2E appears to be running: ${lock_dir}"
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
rehearsal_id="checkpoint-rehearsal-${timestamp}"
target_ref="HEAD~1"
source_title="GitClaw checkpoint rehearsal E2E ${timestamp}"
source_hidden_token="NOECHO_CHECKPOINT_REHEARSAL_SOURCE_${timestamp}"
duplicate_hidden_token="NOECHO_CHECKPOINT_REHEARSAL_DUPLICATE_${timestamp}"
followup_hidden_token="NOECHO_CHECKPOINT_REHEARSAL_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHECKPOINT_REHEARSAL_CONTEXT_V1"
search_phrase="checkpoint rehearsal unique search fixture phrase"
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
        gh issue close "$number" --repo "$repo" --comment "checkpoint rehearsal issue e2e cleanup" >/dev/null 2>&1 || true
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
  --body "@gitclaw /checkpoints rehearse --id ${rehearsal_id} --target ${target_ref}

Create a checkpoint rollback rehearsal lane without copying raw source text.
Do not include this hidden source token: ${source_hidden_token}")"
source_issue_number="${source_issue_url##*/}"
log "created source issue #${source_issue_number}: ${source_issue_url}"

wait_for_run "issues" "$opened_started_at" "$source_title" >/dev/null || die "timed out waiting for checkpoint rehearsal issues run"
wait_for_assistant_count_for_issue "$source_issue_number" 1 || die "expected one checkpoint rehearsal receipt"
receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"

for expected in \
  "GitClaw Checkpoint Rehearsal Issue Action" \
  "Generated without a model call" \
  'model="gitclaw/checkpoints"' \
  "requested_checkpoints_command: \`/checkpoints rehearse\`" \
  "checkpoint_rehearsal_status: \`created\`" \
  "rehearsal_issue_created: \`true\`" \
  "duplicate_suppressed: \`false\`" \
  "target_ref_sha256_12:" \
  "target_allowed: \`true\`" \
  "rehearsal_issue_labeled_for_gitclaw: \`true\`" \
  "model_call_performed: \`false\`" \
  "restore_mode: \`rehearsal-only\`" \
  "rollback_mode: \`inspect-only\`" \
  "repository_mutation_allowed: \`false\`" \
  "git_reset_allowed: \`false\`" \
  "git_clean_allowed: \`false\`" \
  "checkout_mutation_allowed: \`false\`" \
  "raw_source_body_included: \`false\`" \
  "raw_target_ref_included: \`false\`" \
  "raw_diffs_included: \`false\`" \
  "raw_file_bodies_included: \`false\`" \
  "llm_e2e_required_after_checkpoint_rehearsal_issue_change: \`true\`"; do
  grep -Fq "$expected" <<<"$receipt" || die "checkpoint rehearsal receipt missing ${expected}"
done
for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$rehearsal_id" "$target_ref" "Create a checkpoint rollback rehearsal lane"; do
  if grep -Fq "$leaked" <<<"$receipt"; then
    die "checkpoint rehearsal receipt leaked ${leaked}"
  fi
done

rehearsal_issue_number="$(sed -n 's/.*rehearsal_issue: `#\([0-9][0-9]*\)`.*/\1/p' <<<"$receipt" | head -n 1)"
[[ -n "$rehearsal_issue_number" ]] || die "could not parse rehearsal issue from receipt"
log "checkpoint rehearsal created conversation issue #${rehearsal_issue_number}"

rehearsal_json="$(gh issue view "$rehearsal_issue_number" --repo "$repo" --json title,body,labels)"
rehearsal_title="$(jq -r '.title' <<<"$rehearsal_json")"
rehearsal_body="$(jq -r '.body' <<<"$rehearsal_json")"
rehearsal_labels="$(jq -r '.labels[].name' <<<"$rehearsal_json")"
grep -Fxq "gitclaw" <<<"$rehearsal_labels" || die "rehearsal issue missing gitclaw label"
for expected in \
  "gitclaw:checkpoint-rehearsal-issue" \
  "rehearsal_id: ${rehearsal_id}" \
  "target_ref: ${target_ref}" \
  "target_ref_sha256_12:" \
  "target_allowed: true" \
  "rehearsal_mode: rollback-conversation" \
  "restore_mode: rehearsal-only" \
  "rollback_mode: inspect-only" \
  "repository_mutation_allowed: false" \
  "git_reset_allowed: false" \
  "git_clean_allowed: false" \
  "checkout_mutation_allowed: false" \
  "raw_source_body_included: false" \
  "raw_diffs_included: false" \
  "raw_file_bodies_included: false" \
  "gitclaw checkpoints status" \
  "gitclaw checkpoints preview ${target_ref}" \
  "gitclaw checkpoints risk" \
  "gitclaw rollback diff ${target_ref}" \
  "gitclaw rollback risk"; do
  grep -Fq "$expected" <<<"$rehearsal_body" || die "rehearsal issue body missing ${expected}"
done
for leaked in "$source_hidden_token" "Create a checkpoint rollback rehearsal lane"; do
  if grep -Fq "$leaked" <<<"$rehearsal_body"; then
    die "rehearsal issue body leaked ${leaked}"
  fi
done

status_output="$(go run ./cmd/gitclaw checkpoints status)"
preview_output="$(go run ./cmd/gitclaw checkpoints preview "$target_ref")"
risk_output="$(go run ./cmd/gitclaw checkpoints risk)"
rollback_output="$(go run ./cmd/gitclaw rollback diff "$target_ref")"
for expected in \
  "GitClaw Checkpoints Report" \
  "Generated without a model call" \
  "rollback_mode: \`inspect-only\`" \
  "restore_operations_enabled: \`false\`" \
  "raw_diffs_included: \`false\`" \
  "raw_file_bodies_included: \`false\`"; do
  grep -Fq "$expected" <<<"$status_output" || die "checkpoint status output missing ${expected}"
done
for expected in \
  "GitClaw Rollback Preview Report" \
  "target_ref: \`${target_ref}\`" \
  "rollback_mode: \`preview-only\`" \
  "path_names_included: \`false\`" \
  "path_hashes_included: \`true\`" \
  "restore_operations_enabled: \`false\`" \
  "git_reset_allowed: \`false\`" \
  "raw_diffs_included: \`false\`" \
  "raw_file_bodies_included: \`false\`"; do
  grep -Fq "$expected" <<<"$preview_output" || die "checkpoint preview output missing ${expected}"
  grep -Fq "$expected" <<<"$rollback_output" || die "rollback diff output missing ${expected}"
done
for expected in \
  "GitClaw Checkpoint Risk Report" \
  "rollback_mode: \`inspect-only\`" \
  "restore_operations_enabled: \`false\`" \
  "raw_diffs_included: \`false\`" \
  "raw_file_bodies_included: \`false\`"; do
  grep -Fq "$expected" <<<"$risk_output" || die "checkpoint risk output missing ${expected}"
done
for leaked in "$source_hidden_token" "$source_title"; do
  if grep -Fq "$leaked" <<<"$status_output" || grep -Fq "$leaked" <<<"$preview_output" || grep -Fq "$leaked" <<<"$risk_output" || grep -Fq "$leaked" <<<"$rollback_output"; then
    die "checkpoint dry-run output leaked ${leaked}"
  fi
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$source_issue_number" \
  --repo "$repo" \
  --body "@gitclaw /rollback rehearsal --id ${rehearsal_id} --target ${target_ref}

Repeat the same checkpoint rehearsal.
Do not include this hidden duplicate token: ${duplicate_hidden_token}" >/dev/null

wait_for_run "issue_comment" "$duplicate_started_at" "$source_title" >/dev/null || die "timed out waiting for duplicate checkpoint rehearsal run"
wait_for_assistant_count_for_issue "$source_issue_number" 2 || die "expected duplicate checkpoint rehearsal receipt"
duplicate_receipt="$(latest_assistant_comment_for_issue "$source_issue_number")"
for expected in \
  "GitClaw Checkpoint Rehearsal Issue Action" \
  "checkpoint_rehearsal_status: \`existing\`" \
  "rehearsal_issue_created: \`false\`" \
  "duplicate_suppressed: \`true\`" \
  "rehearsal_issue: \`#${rehearsal_issue_number}\`" \
  "raw_source_body_included: \`false\`" \
  "raw_target_ref_included: \`false\`" \
  "raw_diffs_included: \`false\`"; do
  grep -Fq "$expected" <<<"$duplicate_receipt" || die "duplicate checkpoint rehearsal receipt missing ${expected}"
done
for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$rehearsal_id" "$target_ref" "Repeat the same checkpoint rehearsal"; do
  if grep -Fq "$leaked" <<<"$duplicate_receipt"; then
    die "duplicate checkpoint rehearsal receipt leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$rehearsal_issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue this checkpoint rollback rehearsal and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line ends with an all-caps token after the arrow.
That token starts with \`GITCLAW_CHECKPOINT_REHEARSAL_\`.
Reply with only the exact all-caps token after the arrow from the matching gitclaw.search_files tool output line.
Do not reply with a placeholder like \`<token>\` or the word \`token\`.
Do not include source issue numbers, rehearsal issue numbers, rehearsal ids, target refs, branch names, source body, or hidden sentinels.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run "issue_comment" "$comment_started_at" "$rehearsal_title")" || die "timed out waiting for checkpoint rehearsal model follow-up"
wait_for_assistant_count 1 || die "expected model-backed checkpoint rehearsal follow-up"
model_comment="$(latest_assistant_comment_for_issue "$rehearsal_issue_number")"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include checkpoint rehearsal search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant checkpoint rehearsal follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant checkpoint rehearsal follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant checkpoint rehearsal follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant checkpoint rehearsal follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant checkpoint rehearsal follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant checkpoint rehearsal follow-up marker missing usage token telemetry"

for leaked in "$source_hidden_token" "$duplicate_hidden_token" "$followup_hidden_token" "$rehearsal_id" "$target_ref"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model checkpoint rehearsal follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for source issue #${source_issue_number}, rehearsal issue #${rehearsal_issue_number} (model follow-up: ${model_url})"
