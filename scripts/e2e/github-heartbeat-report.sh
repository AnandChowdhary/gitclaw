#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "heartbeat-report-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow_name="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
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
ensure_label gitclaw:heartbeat 1d76db "GitClaw heartbeat opt-in"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
token="NOECHO_HEARTBEAT_REPORT_${timestamp}"
followup_hidden_token="NOECHO_HEARTBEAT_REPORT_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_HEARTBEAT_REPORT_CONTEXT_V1"
search_phrase="heartbeat report unique search fixture phrase"
title="@gitclaw /heartbeat e2e ${timestamp}"
body="Live heartbeat-report E2E.

Hidden heartbeat report body token: ${token}
This should produce a deterministic heartbeat operator report without a model call.
The report must not print .gitclaw/HEARTBEAT.md contents."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label gitclaw \
  --label gitclaw:heartbeat)"
issue_number="${issue_url##*/}"

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "heartbeat-report e2e cleanup" >/dev/null 2>&1 || true
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
      local run_status conclusion url
      run_status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$run_status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${event_name} run failed with conclusion ${conclusion}: ${url}"
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

run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one heartbeat report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/heartbeat"' \
  "GitClaw Heartbeat Report" \
  "Generated without a model call" \
  'heartbeat_report_status: `ok`' \
  'heartbeat_label: `gitclaw:heartbeat`' \
  'trigger_label: `gitclaw`' \
  'disabled_label: `gitclaw:disabled`' \
  'workflow_path: `.github/workflows/gitclaw-heartbeat.yml`' \
  'workflow_present: `true`' \
  'workflow_dispatch_trigger: `true`' \
  'schedule_trigger: `true`' \
  'schedule_entries: `1`' \
  'permissions_contents_read: `true`' \
  'permissions_issues_write: `true`' \
  'permissions_models_read: `true`' \
  'workflow_inputs: `3`' \
  'heartbeat_context_path: `.gitclaw/HEARTBEAT.md`' \
  'heartbeat_context_present: `true`' \
  'default_limit: `3`' \
  'slot_strategy: `utc-hour-or-explicit`' \
  'idempotency_marker: `gitclaw:heartbeat`' \
  'quiet_response: `HEARTBEAT_OK`' \
  'model_call_required: `false`' \
  'runner_model_call_required: `true`' \
  'repository_mutation_allowed: `false`' \
  'issue_scan_performed: `false`' \
  'raw_bodies_included: `false`' \
  'raw_heartbeat_body_included: `false`' \
  'llm_e2e_required_after_change: `true`' \
  'llm_e2e_required_after_heartbeat_marker_change: `true`' \
  'heartbeat_label_present: `true`' \
  'disabled_label_present: `false`' \
  'heartbeat_comments_now: `0`' \
  'gitclaw heartbeat --repo <owner/repo>' \
  'gitclaw heartbeat status' \
  '### Verification Findings' \
  '- none'; do
  grep -Fq -- "$expected" <<<"$comments" || die "heartbeat report missing ${expected}"
done

for forbidden in "$token" "$expected_token" "$search_phrase" "GITCLAW_HEARTBEAT_CONTEXT_V1"; do
  if grep -Fq -- "$forbidden" <<<"$comments"; then
    die "heartbeat report leaked ${forbidden}"
  fi
done

url="$(jq -r '.url' <<<"$run_json")"
log "heartbeat report verified for issue #${issue_number}: ${url}"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Search for that exact phrase, not shorter words from it.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${search_phrase} => <token>\`.
Reply with only the uppercase fixture token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with the HEARTBEAT.md token, issue title, issue number, or any token from this issue/comments.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run issue_comment "$comment_started_at")" || die "timed out waiting for issue_comment workflow run"
wait_for_assistant_count 2 || die "expected model-backed follow-up assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant marker missing usage token telemetry"

for leaked in "$token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${url} (model follow-up: ${model_url})"
