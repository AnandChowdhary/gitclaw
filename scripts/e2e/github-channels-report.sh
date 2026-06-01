#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "channels-report-e2e: $*" >&2
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
ensure_label gitclaw:channel 1d76db "GitClaw mirrored channel thread"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
thread_id="channels-report-e2e-${timestamp}"
message_id="message-${timestamp}"
issue_token="NOECHO_CHANNELS_REPORT_ISSUE_${timestamp}"
message_token="NOECHO_CHANNELS_REPORT_MESSAGE_${timestamp}"
command_token="NOECHO_CHANNELS_REPORT_COMMAND_${timestamp}"
followup_hidden_token="NOECHO_CHANNELS_REPORT_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNELS_REPORT_CONTEXT_V1"
search_phrase="channels report unique search fixture phrase"
title="GitClaw channels report e2e ${timestamp}"
body="<!-- gitclaw:channel-thread channel=\"telegram\" thread_id=\"${thread_id}\" -->
GitClaw channels-report E2E thread.

Hidden issue token: ${issue_token}"

issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label gitclaw:channel)"
issue_number="${issue_url##*/}"

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "channels-report e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
}
trap cleanup EXIT

log "created issue #${issue_number}: ${issue_url}"

gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "<!-- gitclaw:channel-message channel=\"telegram\" thread_id=\"${thread_id}\" message_id=\"${message_id}\" author=\"telegram:e2e\" -->
Hidden mirrored message token: ${message_token}" >/dev/null

wait_for_run_id() {
  local run_id="$1"
  for _ in {1..90}; do
    local run_json
    run_json="$(gh run view "$run_id" \
      --repo "$repo" \
      --json databaseId,status,conclusion,url \
      --jq '.')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local run_status conclusion url
      run_status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$run_status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "channels report run failed with conclusion ${conclusion}: ${url}"
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
  for _ in {1..90}; do
    local run_json
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$workflow_name" \
      --event issue_comment \
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
        [[ "$conclusion" == "success" ]] || die "channels report follow-up run failed with conclusion ${conclusion}: ${url}"
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

gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /channels

Please audit channel bridge config.
Hidden command token: ${command_token}" >/dev/null

wait_for_assistant_count 1 || die "expected one channels report comment"
comments="$(assistant_comments)"
run_id="$(sed -nE 's/.*run_id="([0-9]+)".*/\1/p' <<<"$comments" | tail -n 1)"
[[ -n "$run_id" ]] || die "could not extract channels report run id"
run_json="$(wait_for_run_id "$run_id")" || die "timed out waiting for channels report workflow run"

for expected in \
  'model="gitclaw/channels"' \
  "GitClaw Channel Report" \
  "Generated without a model call" \
  'channel_label: `gitclaw:channel`' \
  'trigger_label: `gitclaw`' \
  'workflow_path: `.github/workflows/gitclaw-channel-ingest.yml`' \
  'workflow_present: `true`' \
  'workflow_dispatch_trigger: `true`' \
  'permissions_actions_write: `true`' \
  'permissions_issues_write: `true`' \
  'workflow_inputs: `5`' \
  'send_workflow_path: `.github/workflows/gitclaw-channel-send.yml`' \
  'send_workflow_present: `true`' \
  'send_workflow_inputs: `6`' \
  'state_workflow_present: `true`' \
  'state_workflow_inputs: `4`' \
  'gateway_workflow_present: `true`' \
  'gateway_workflow_inputs: `6`' \
  'delivery_workflow_present: `true`' \
  'delivery_workflow_inputs: `6`' \
  'outbox_workflow_present: `true`' \
  'outbox_workflow_permissions_issues_read: `true`' \
  'outbox_workflow_inputs: `5`' \
  'channel_thread_issue: `true`' \
  'channel_message_comments_now: `1`' \
  'supported_providers: `telegram, slack, generic`' \
  'wake_strategy: `workflow_dispatch`' \
  'llm_e2e_required_after_channel_report_change: `true`' \
  'telegram' \
  'slack' \
  'generic' \
  'gitclaw channel-ingest' \
  'gitclaw channel-send' \
  'gitclaw channel-gateway' \
  'gitclaw channel-outbox' \
  'gitclaw channel-delivery' \
  'dispatch id: `<channel>-<message_id>`'; do
  grep -Fq "$expected" <<<"$comments" || die "channels report missing ${expected}"
done

if grep -Fq "$issue_token" <<<"$comments" || grep -Fq "$message_token" <<<"$comments" || grep -Fq "$command_token" <<<"$comments"; then
  die "channels report leaked hidden token"
fi
if grep -Fq "$expected_token" <<<"$comments" || grep -Fq "$search_phrase" <<<"$comments"; then
  die "channels report leaked follow-up fixture context"
fi

url="$(jq -r '.url' <<<"$run_json")"
log "channels report verified for issue #${issue_number}: ${url}"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Use the repo-reader skill and search the repository for \`${search_phrase}\`.

The matching repository search result line has the form \`${search_phrase} => <token>\`.
Reply with only the token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with any token from this issue or its comments.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run "$comment_started_at")" || die "timed out waiting for issue_comment workflow run"
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

for leaked in "$issue_token" "$message_token" "$command_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${url} (model follow-up: ${model_url})"
