#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "proactive-not-before-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow="${GITCLAW_E2E_PROACTIVE_WORKFLOW:-.github/workflows/gitclaw-proactive.yml}"
lock_dir="/tmp/gitclaw-proactive-not-before-e2e.lock"
cleanup_success=0

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another proactive not-before E2E appears to be running: ${lock_dir}"
fi

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e >/dev/null 2>&1 || true
    if [[ "$cleanup_success" == "1" ]]; then
      gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled >/dev/null 2>&1 || true
      gh issue close "$issue_number" --repo "$repo" --comment "proactive not-before e2e cleanup" >/dev/null 2>&1 || true
    else
      log "leaving issue #${issue_number} open for inspection after unsuccessful run"
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

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
ensure_label gitclaw:proactive fbca04 "GitClaw proactive run"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
normalized_timestamp="$(printf "%s" "$timestamp" | tr '[:upper:]' '[:lower:]')"
name="proactive-not-before-e2e-${normalized_timestamp}"
future_slot="future-${timestamp}"
due_slot="due-${timestamp}"
future_token="GITCLAW_PROACTIVE_NOT_BEFORE_FUTURE_${timestamp}"
due_token="GITCLAW_PROACTIVE_NOT_BEFORE_DUE_${timestamp}"

run_list_json() {
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event workflow_dispatch \
    --limit 10 \
    --json databaseId,status,conclusion,createdAt,url \
    --jq '.'
}

wait_for_run() {
  local started_at="$1"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json | jq -c --arg started "$started_at" '[.[] | select(.createdAt >= $started)] | sort_by(.createdAt) | reverse | .[0] // empty')"
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

find_issue_numbers() {
  local slot="$1"
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:proactive \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg name "$name" --arg slot "$slot" '.[] | select((.title | contains($name)) and (.title | contains($slot)) or ((.body | contains($name)) and (.body | contains($slot)))) | .number'
}

wait_for_issue_number() {
  local slot="$1"
  for _ in {1..30}; do
    local numbers
    numbers="$(find_issue_numbers "$slot")"
    if [[ -n "$numbers" ]]; then
      echo "$numbers" | head -n 1
      return 0
    fi
    sleep 2
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

error_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("<!-- gitclaw:error"))] | length'
}

wait_for_assistant_count() {
  local want="$1"
  for _ in {1..120}; do
    local got
    got="$(assistant_count 2>/dev/null || true)"
    if [[ "$got" =~ ^[0-9]+$ && "$got" == "$want" ]]; then
      return 0
    fi
    local errors
    errors="$(error_count 2>/dev/null || true)"
    if [[ "$errors" =~ ^[0-9]+$ && "$errors" != "0" ]]; then
      die "assistant run posted ${errors} error comment(s)"
    fi
    sleep 5
  done
  return 1
}

future_prompt="Proactive not-before future E2E instruction.

@gitclaw /proactive

Hidden future prompt token: ${future_token}
This issue must not be created before the due gate."

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow" \
  --repo "$repo" \
  -f name="$name" \
  -f slot="$future_slot" \
  -f prompt="$future_prompt" \
  -f not_before="2099-01-01T00:00:00Z"
wait_for_run "$started_at" >/dev/null || die "timed out waiting for future not-before proactive workflow"

sleep 5
if [[ -n "$(find_issue_numbers "$future_slot")" ]]; then
  die "future not-before gate unexpectedly created an issue"
fi
log "future not-before gate skipped issue creation"

due_prompt="Proactive not-before due E2E instruction.

@gitclaw /proactive

Hidden due prompt token: ${due_token}
The deterministic proactive report must not leak this token."

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow" \
  --repo "$repo" \
  -f name="$name" \
  -f slot="$due_slot" \
  -f prompt="$due_prompt" \
  -f not_before="2000-01-01T00:00:00Z"
wait_for_run "$started_at" >/dev/null || die "timed out waiting for due proactive workflow"

issue_number="$(wait_for_issue_number "$due_slot")" || die "timed out finding due proactive issue for ${name}/${due_slot}"
log "due proactive workflow created issue #${issue_number}"
wait_for_assistant_count 1 || die "timed out waiting for due proactive assistant response"

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,labels,comments)"
grep -Fq "gitclaw:proactive-run" <<<"$(jq -r '.body' <<<"$issue_json")" || die "issue body missing proactive marker"
grep -Fq "$due_token" <<<"$(jq -r '.body' <<<"$issue_json")" || die "issue body missing due prompt token"

comments="$(assistant_comments)"
grep -Fq 'model="gitclaw/proactive"' <<<"$comments" || die "assistant marker missing proactive report model"
grep -Fq "GitClaw Proactive Report" <<<"$comments" || die "assistant comment missing proactive report"
grep -Fq 'proactive_run_issue: `true`' <<<"$comments" || die "assistant comment did not detect proactive issue"
if grep -Fq "$due_token" <<<"$comments"; then
  die "assistant proactive report leaked due prompt token ${due_token}"
fi
grep -Fq "dispatch-proactive-${name}-${due_slot}" <<<"$comments" || die "assistant marker missing proactive dispatch event id"

labels="$(jq -r '.labels[].name' <<<"$issue_json")"
grep -Fxq "gitclaw:proactive" <<<"$labels" || die "issue missing gitclaw:proactive label"
grep -Fxq "gitclaw:done" <<<"$labels" || die "issue missing gitclaw:done label"

log "passed for issue #${issue_number}"
cleanup_success=1
