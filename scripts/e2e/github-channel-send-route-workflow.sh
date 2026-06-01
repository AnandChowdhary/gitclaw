#!/usr/bin/env bash
# gitclaw-doctor-live-issue: channel-send route workflow creates the live issue.
set -euo pipefail

log() {
  echo "channel-send-route-workflow-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
send_workflow="${GITCLAW_E2E_CHANNEL_SEND_WORKFLOW:-.github/workflows/gitclaw-channel-send.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-channel-send-route-workflow-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-send route workflow E2E appears to be running: ${lock_dir}"
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

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
route="e2e-slack-route"
message_id="route-${timestamp}"
thread_id="gitclaw-e2e-route-${message_id}"
account_id="slack-route-account-NOECHO_CHANNEL_ROUTE_ACCOUNT_${timestamp}"
body_token="NOECHO_CHANNEL_SEND_ROUTE_BODY_${timestamp}"
followup_hidden_token="NOECHO_CHANNEL_SEND_ROUTE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_CHANNEL_SEND_ROUTE_CONTEXT_V1"
search_phrase="channel send route unique search fixture phrase"
issue_title="GitClaw slack thread ${thread_id}"
issue_number=""

run_list_json() {
  local workflow="$1"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event workflow_dispatch \
    --limit 20 \
    --json databaseId,status,conclusion,createdAt,url \
    --jq '.'
}

wait_for_workflow_run() {
  local workflow="$1"
  local started_at="$2"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json "$workflow" | jq -c --arg started "$started_at" '[.[] | select(.createdAt >= $started)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${workflow} run failed with conclusion ${conclusion}: ${url}"
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
      --workflow "$main_workflow" \
      --event issue_comment \
      --created ">=$started_at" \
      --limit 10 \
      --json databaseId,status,conclusion,createdAt,url,displayTitle \
      --jq '. as $runs | $runs | map(select(.displayTitle == "'"${issue_title}"'")) | sort_by(.createdAt) | reverse | .[0] // empty')"
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

find_issue_numbers() {
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:channel \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg thread "$thread_id" '.[] | select((.title | contains($thread)) or (.body | contains($thread))) | .number'
}

wait_for_issue_number() {
  for _ in {1..30}; do
    local number
    number="$(find_issue_numbers | head -n 1)"
    if [[ -n "$number" && "$number" != "null" ]]; then
      echo "$number"
      return 0
    fi
    sleep 2
  done
  return 1
}

run_log() {
  local run_json="$1"
  local run_id
  run_id="$(jq -r '.databaseId' <<<"$run_json")"
  gh run view "$run_id" --repo "$repo" --log
}

outbound_comment_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:channel-outbound"))] | length'
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

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
}

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "channel-send route workflow e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

outbound_body="GitClaw routed outbound E2E message.

Visible outbound token for the provider queue: ${body_token}"

send_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$send_workflow" \
  --repo "$repo" \
  -f route="$route" \
  -f message_id="$message_id" \
  -f body="$outbound_body"

send_run_json="$(wait_for_workflow_run "$send_workflow" "$send_started_at")" || die "timed out waiting for channel-send route workflow"
send_log="$(run_log "$send_run_json")"
issue_number="$(wait_for_issue_number)" || die "timed out finding routed channel issue for ${thread_id}"
log "channel-send route queued outbound message on issue #${issue_number}"

for expected in \
  "channel_send issue=${issue_number}" \
  "created=true" \
  "duplicate=false" \
  "route_resolved=true" \
  "route_sha256_12="; do
  grep -Fq "$expected" <<<"$send_log" || die "channel-send route log missing ${expected}"
done
for leaked in "$body_token" "$account_id"; do
  if grep -Fq "$leaked" <<<"$send_log"; then
    die "channel-send route log leaked ${leaked}"
  fi
done

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,labels,comments)"
labels="$(jq -r '.labels[].name' <<<"$issue_json")"
grep -Fxq "gitclaw:channel" <<<"$labels" || die "routed channel issue missing gitclaw:channel label"
if grep -Fxq "gitclaw" <<<"$labels"; then
  die "routed channel-send issue should not carry the model trigger label"
fi
grep -Fq "gitclaw:channel-thread" <<<"$(jq -r '.body' <<<"$issue_json")" || die "routed issue missing channel-thread marker"
comments="$(jq -r '[.comments[].body] | join("\n")' <<<"$issue_json")"
for expected in \
  "gitclaw:channel-outbound" \
  'channel="slack"' \
  "thread_id=\"${thread_id}\"" \
  "message_id=\"${message_id}\"" \
  "$body_token"; do
  grep -Fq "$expected" <<<"$comments" || die "routed channel issue missing ${expected}"
done

duplicate_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$send_workflow" \
  --repo "$repo" \
  -f route="$route" \
  -f message_id="$message_id" \
  -f body="$outbound_body"
wait_for_workflow_run "$send_workflow" "$duplicate_started_at" >/dev/null || die "timed out waiting for duplicate routed channel-send workflow"
[[ "$(outbound_comment_count)" == "1" ]] || die "duplicate routed channel-send workflow created another outbound comment"

outbox_file="$(mktemp)"
outbox_output="$(GITCLAW_CHANNEL=slack \
  GITCLAW_CHANNEL_ACCOUNT_ID="$account_id" \
  GITCLAW_CHANNEL_ISSUE_NUMBER="$issue_number" \
  go run ./cmd/gitclaw channel-outbox --repo "$repo" --out "$outbox_file")"
grep -Fq "pending=1" <<<"$outbox_output" || die "channel outbox did not report pending routed message: ${outbox_output}"
grep -Fq '"pending_messages": 1' "$outbox_file" || die "channel outbox file missing pending routed message"
if grep -Fq "$body_token" <<<"$outbox_output" || grep -Fq "$body_token" "$outbox_file"; then
  die "channel outbox leaked routed body without --include-body"
fi

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the named channel route workflow and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
The matching repository search result line has the form \`${search_phrase} => <token>\`.
Reply with only the token after the arrow from the matching gitclaw.search_files tool output line.
Do not include the channel route name, thread id, message id, account hash, or any issue/comment token.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run "$comment_started_at")" || die "timed out waiting for channel-send route issue_comment follow-up"
wait_for_assistant_count 1 || die "expected model-backed follow-up assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "assistant did not include route search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "assistant route follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "assistant route follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "assistant route follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "assistant route follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "assistant route follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant route follow-up marker missing usage token telemetry"

for leaked in "$body_token" "$followup_hidden_token" "$account_id"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model route follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number} (model follow-up: ${model_url})"
