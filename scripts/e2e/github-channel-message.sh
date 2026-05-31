#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "channel-message-e2e: $*" >&2
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
token="NOECHO_CHANNEL_MESSAGE_${timestamp}"
expected_token="GITCLAW_CHANNEL_MESSAGE_CONTEXT_V1"
search_phrase="channel message unique search fixture phrase"
message_id="telegram-update-${timestamp}"
title="GitClaw channel message e2e ${timestamp}"
body="Live channel bridge E2E.

This issue starts untriggered. A mirrored channel message comment contains the actual request.
"

issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body")"
issue_number="${issue_url##*/}"

cleanup() {
  gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled >/dev/null 2>&1 || true
  gh issue close "$issue_number" --repo "$repo" --comment "channel message e2e cleanup" >/dev/null 2>&1 || true
}
trap cleanup EXIT

gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "<!-- gitclaw:channel-message channel=\"telegram\" message_id=\"${message_id}\" author=\"telegram:e2e\" -->
Mirrored Telegram message.

Use the repo-reader skill and search the repository for \`${search_phrase}\`.
Reply with the exact token from the matching gitclaw.search_files result line.
Also include the exact word \`telegram\`.
Do not include this hidden channel token: ${token}."

gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw --add-label gitclaw:e2e >/dev/null
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

assistant_comments() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | join("\n---GITCLAW-COMMENT---\n")'
}

assistant_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

wait_for_assistant_count() {
  local want="$1"
  for _ in {1..90}; do
    local got
    got="$(assistant_count)"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

dispatch_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow_name" \
  --repo "$repo" \
  -f issue_number="$issue_number" \
  -f dispatch_id="$message_id" \
  -f reason="channel-message-e2e"

wait_for_dispatch_run "$dispatch_started_at" >/dev/null || die "timed out waiting for channel-message workflow_dispatch run"
wait_for_assistant_count 1 || die "expected one assistant comment after channel-message dispatch"
comments="$(assistant_comments)"
grep -Fq "$expected_token" <<<"$comments" || die "assistant comment missing search token ${expected_token}"
grep -Fiq "telegram" <<<"$comments" || die "assistant comment missing channel word telegram"
grep -Fq "dispatch-${message_id}" <<<"$comments" || die "assistant marker missing dispatch event id"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$comments" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$comments"; then
  die "assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$comments" || die "assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$comments" || die "assistant marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$comments" || die "assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$comments" || die "assistant marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$comments" || die "assistant marker missing usage token telemetry"
if grep -Fq "$token" <<<"$comments"; then
  die "assistant leaked hidden channel token"
fi

log "channel marker dispatch verified"
log "passed for issue #${issue_number}"
