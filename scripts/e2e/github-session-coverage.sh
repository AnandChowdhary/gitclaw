#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "session-coverage-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow_name="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
backup_branch="${GITCLAW_E2E_BACKUP_BRANCH:-gitclaw-backups}"
retention_label="${GITCLAW_E2E_RETENTION_LABEL:-gitclaw:e2e}"

if [[ -z "$repo" ]]; then
  repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
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
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
hidden_token="GITCLAW_SESSION_COVERAGE_HIDDEN_${timestamp}"
comment_token="GITCLAW_SESSION_COVERAGE_COMMENT_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw session coverage e2e ${timestamp}"
body="Live session coverage E2E.

Use the repo-reader skill and search the repository for \`${search_phrase}\`.
Reply with the exact GITCLAW_SEARCH token from the matching repository search result line.
Do not include this hidden issue token: ${hidden_token}
Keep the answer under 30 words."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label gitclaw)"
issue_number="${issue_url##*/}"
tmp_dir=""

cleanup() {
  if [[ -n "${tmp_dir:-}" ]]; then
    rm -rf "$tmp_dir"
  fi
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "session-coverage e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
}
trap cleanup EXIT

log "created issue #${issue_number}: ${issue_url}"

wait_for_run() {
  local event_name="$1"
  local started_at="$2"
  for _ in {1..90}; do
    local run_json
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$workflow_name" \
      --event "$event_name" \
      --created ">=$started_at" \
      --limit 10 \
      --json databaseId,status,conclusion,url,createdAt,displayTitle \
      --jq '. as $runs | $runs | map(select(.displayTitle == "'"${title}"'")) | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${event_name} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

assistant_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

error_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("<!-- gitclaw:error"))] | length'
}

issue_label_names() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json labels \
    --jq '.labels[].name'
}

wait_for_assistant_count() {
  local want="$1"
  for _ in {1..90}; do
    local errors
    errors="$(error_count)"
    if [[ "$errors" != "0" ]]; then
      die "assistant run posted ${errors} error marker comment(s)"
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

wait_for_done_status() {
  for _ in {1..60}; do
    local labels
    labels="$(issue_label_names)"
    if grep -Fxq "gitclaw:done" <<<"$labels" &&
      ! grep -Fxq "gitclaw:running" <<<"$labels" &&
      ! grep -Fxq "gitclaw:error" <<<"$labels"; then
      return 0
    fi
    sleep 5
  done
  return 1
}

model_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one model-backed assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant marker missing repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant marker did not prove search_files was prompt-visible"

if grep -Fq "$hidden_token" <<<"$model_comment"; then
  die "model-backed assistant leaked hidden issue token"
fi

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /session coverage

Please prove this issue has model-backed prompt provenance and tool coverage.
Hidden comment token: ${comment_token}" >/dev/null

coverage_run_json="$(wait_for_run issue_comment "$comment_started_at")" || die "timed out waiting for session coverage workflow run"
wait_for_assistant_count 2 || die "expected session coverage report as second assistant comment"
coverage_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/session"' \
  "GitClaw Session Coverage Report" \
  "Generated without a model call" \
  'scope: `issue-thread`' \
  'session_coverage_status: `ok`' \
  'required_assistant_turns: `1`' \
  'required_prompt_provenance_turns: `1`' \
  'required_model_backed_turns: `1`' \
  'assistant_turn_comments: `1`' \
  'assistant_turns_with_prompt_provenance: `1`' \
  'model_backed_assistant_turns: `1`' \
  'prompt_visible_skill_names: `repo-reader`' \
  'gitclaw.search_files' \
  'raw_bodies_included: `false`' \
  'raw_prompts_included: `false`' \
  'llm_e2e_required_after_session_coverage_change: `true`' \
  'assistant_turns_met=`true`' \
  'prompt_provenance_met=`true`' \
  'model_backed_turns_met=`true`'; do
  grep -Fq "$expected" <<<"$coverage_comment" || die "session coverage report missing ${expected}"
done

for leaked in "$hidden_token" "$comment_token" "$search_phrase" "Please prove this issue has model-backed prompt provenance"; do
  if grep -Fq "$leaked" <<<"$coverage_comment"; then
    die "session coverage report leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"

repo_key="${repo//\//__}"
issue_padded="$(printf "%06d" "$issue_number")"
issue_path=".gitclaw/backups/${repo_key}/issues/${issue_padded}.json"

fetch_backup_branch() {
  rm -rf "$tmp_dir"
  tmp_dir="$(mktemp -d)"
  backup_checkout="${tmp_dir}/backup-branch"
  gh repo clone "$repo" "$backup_checkout" -- --depth=1 --branch "$backup_branch" >/dev/null 2>&1
}

for _ in {1..60}; do
  if ! fetch_backup_branch; then
    sleep 5
    continue
  fi
  backup_file="${backup_checkout}/${issue_path}"
  if [[ -f "$backup_file" ]] &&
    jq -e '
      any(.comments[]; (.body | contains("model=\"gitclaw/session\""))) and
      any(.comments[]; (.body | contains("model=\"openai/")))
    ' "$backup_file" >/dev/null; then
    coverage_output="$(go run ./cmd/gitclaw session coverage \
      --backup "$backup_file" \
      --require-skill repo-reader \
      --require-tool gitclaw.search_files)"
    for expected in \
      "GitClaw Session Coverage Report" \
      'scope: `local-backup`' \
      "repository: \`${repo}\`" \
      "issue: \`#${issue_number}\`" \
      'session_coverage_status: `ok`' \
      'required_skill_names: `repo-reader`' \
      'required_tool_names: `gitclaw.search_files`' \
      'model_backed_assistant_turns: `1`' \
      'prompt_visible_skill_names: `repo-reader`' \
      'gitclaw.search_files' \
      'missing_required_skill_names: `none`' \
      'missing_required_tool_names: `none`' \
      'raw_bodies_included: `false`' \
      'raw_prompts_included: `false`' \
      'required_tools_met=`true`'; do
      grep -Fq "$expected" <<<"$coverage_output" || die "local session coverage output missing ${expected}"
    done
    if grep -Fq "$hidden_token" <<<"$coverage_output" ||
      grep -Fq "$comment_token" <<<"$coverage_output" ||
      grep -Fq "$search_phrase" <<<"$coverage_output" ||
      grep -Fq "$title" <<<"$coverage_output"; then
      die "local session coverage output leaked raw issue data"
    fi
    break
  fi
  sleep 5
done

if [[ -z "${coverage_output:-}" ]]; then
  die "session coverage did not observe finalized issue #${issue_number} in ${backup_branch}"
fi

model_url="$(jq -r '.url' <<<"$model_run_json")"
coverage_url="$(jq -r '.url' <<<"$coverage_run_json")"
log "passed for issue #${issue_number}: ${coverage_url} (initial model run: ${model_url})"
