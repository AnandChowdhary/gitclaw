#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "workflow-dispatch-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow_name="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"

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
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
token="GITCLAW_WORKFLOW_DISPATCH_E2E_${timestamp}"
followup_hidden_token="NOECHO_WORKFLOW_DISPATCH_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_WORKFLOW_DISPATCH_CONTEXT_V1"
search_phrase="workflow dispatch unique search fixture phrase"
dispatch_id="workflow-dispatch-e2e-${timestamp}"
title="GitClaw workflow_dispatch e2e ${timestamp}"
body="Live workflow_dispatch E2E.

When GitClaw is manually dispatched for this issue, reply with exact token \`${token}\`.
"

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body")"
issue_number="${issue_url##*/}"

cleanup() {
  gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled >/dev/null 2>&1 || true
  gh issue close "$issue_number" --repo "$repo" --comment "workflow_dispatch e2e cleanup" >/dev/null 2>&1 || true
}
trap cleanup EXIT

log "created issue #${issue_number}: ${issue_url}"

run_list_json() {
  gh run list \
    --repo "$repo" \
    --workflow "$workflow_name" \
    --event workflow_dispatch \
    --limit 10 \
    --json databaseId,status,conclusion,createdAt,url \
    --jq '.'
}

wait_for_dispatch_run() {
  local started_at="$1"
  local run_json
  for _ in {1..90}; do
    run_json="$(run_list_json | jq -c --arg started "$started_at" '[.[] | select(.createdAt >= $started)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "dispatch run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

wait_for_opened_run() {
  local started_at="$1"
  local run_json
  for _ in {1..90}; do
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$workflow_name" \
      --event issues \
      --created ">=$started_at" \
      --limit 10 \
      --json databaseId,status,conclusion,createdAt,url,displayTitle \
      --jq '. as $runs | $runs | map(select(.displayTitle == "'"${title}"'")) | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "issues.opened run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

wait_for_issue_comment_run() {
  local started_at="$1"
  local run_json
  for _ in {1..90}; do
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$workflow_name" \
      --event issue_comment \
      --created ">=$started_at" \
      --limit 10 \
      --json databaseId,status,conclusion,createdAt,url,displayTitle \
      --jq '. as $runs | $runs | map(select(.displayTitle == "'"${title}"'")) | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "issue_comment run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

assistant_comments() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | join("\n---GITCLAW-COMMENT---\n")'
}

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

assistant_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

error_count() {
  gh issue view "$issue_number" \
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

opened_run_json="$(wait_for_opened_run "$issue_started_at")" || die "timed out waiting for initial issues.opened workflow run"
initial_count="$(assistant_count)"
if [[ "$initial_count" != "0" ]]; then
  opened_url="$(jq -r '.url' <<<"$opened_run_json")"
  die "initial issues.opened run handled the issue before dispatch: ${opened_url}"
fi

gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw --add-label gitclaw:e2e >/dev/null
log "initial issues.opened preflight verified"

dispatch_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow_name" \
  --repo "$repo" \
  -f issue_number="$issue_number" \
  -f dispatch_id="$dispatch_id" \
  -f reason="e2e"

wait_for_dispatch_run "$dispatch_started_at" >/dev/null || die "timed out waiting for first workflow_dispatch run"
wait_for_assistant_count 1 || die "expected one assistant comment after first dispatch"
comments="$(assistant_comments)"
grep -Fq "$token" <<<"$comments" || die "assistant comment missing token ${token}"
grep -Fq "dispatch-${dispatch_id}" <<<"$comments" || die "assistant marker missing dispatch event id"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$comments" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$comments"; then
  die "assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$comments" || die "assistant marker missing prompt context hash"
grep -Fq 'usage_total_tokens="' <<<"$comments" || die "assistant marker missing usage token telemetry"
log "first workflow_dispatch verified"

dispatch_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow_name" \
  --repo "$repo" \
  -f issue_number="$issue_number" \
  -f dispatch_id="$dispatch_id" \
  -f reason="e2e-idempotency"

wait_for_dispatch_run "$dispatch_started_at" >/dev/null || die "timed out waiting for duplicate workflow_dispatch run"
final_count="$(assistant_count)"
if [[ "$final_count" != "1" ]]; then
  die "workflow_dispatch idempotency failed: expected 1 assistant comment, got ${final_count}"
fi

log "idempotency verified"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Search for that exact phrase, not shorter words from it.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${search_phrase} => <token>\`.
Reply with only the uppercase fixture token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with the workflow_dispatch nonce, issue title, dispatch id, issue number, or any token from this issue/comments.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run "$comment_started_at")" || die "timed out waiting for model follow-up issue_comment workflow run"
wait_for_assistant_count 2 || die "expected model-backed follow-up assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "model follow-up did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "model follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "model follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "model follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "model follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "model follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "model follow-up marker missing usage token telemetry"

for leaked in "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number} (model follow-up: ${model_url})"
