#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "proactive-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
proactive_workflow="${GITCLAW_E2E_PROACTIVE_WORKFLOW:-.github/workflows/gitclaw-proactive.yml}"
lock_dir="/tmp/gitclaw-proactive-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another proactive E2E appears to be running: ${lock_dir}"
fi
trap 'rm -rf "$lock_dir"' EXIT

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
name="proactive-e2e-${normalized_timestamp}"
slot="slot-${timestamp}"
dispatch_id="proactive-${name}-${slot}"
token="GITCLAW_PROACTIVE_E2E_${timestamp}"
prompt="Proactive E2E instruction.

Reply with exact token \`${token}\`.
Also include the exact word \`proactive\`."

run_list_json() {
  local workflow="$1"
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event workflow_dispatch \
    --limit 10 \
    --json databaseId,status,conclusion,createdAt,url \
    --jq '.'
}

wait_for_run() {
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

find_issue_numbers() {
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:proactive \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg name "$name" --arg slot "$slot" '.[] | select((.title | contains($name)) or ((.body | contains($name)) and (.body | contains($slot)))) | .number'
}

wait_for_issue_number() {
  for _ in {1..30}; do
    local numbers
    numbers="$(find_issue_numbers)"
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

wait_for_assistant_count() {
  local want="$1"
  for _ in {1..120}; do
    local got
    got="$(assistant_count)"
    if [[ "$got" == "$want" ]]; then
      return 0
    fi
    sleep 5
  done
  return 1
}

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label gitclaw:e2e >/dev/null 2>&1 || true
    gh issue close "$issue_number" --repo "$repo" --comment "proactive e2e cleanup" >/dev/null 2>&1 || true
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

dispatch_proactive() {
  local started_at="$1"
  gh workflow run "$proactive_workflow" \
    --repo "$repo" \
    -f name="$name" \
    -f slot="$slot" \
    -f prompt="$prompt"
  wait_for_run "$proactive_workflow" "$started_at" >/dev/null || die "timed out waiting for proactive workflow"
}

first_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
dispatch_proactive "$first_started_at"
issue_number="$(wait_for_issue_number)" || die "timed out finding proactive issue for ${name}/${slot}"
log "proactive workflow created issue #${issue_number}"
wait_for_assistant_count 1 || die "timed out waiting for first proactive assistant response"

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,labels,comments)"
grep -Fq "gitclaw:proactive-run" <<<"$(jq -r '.body' <<<"$issue_json")" || die "issue body missing proactive marker"
grep -Fq "$token" <<<"$(jq -r '.body' <<<"$issue_json")" || die "issue body missing prompt token"
comments="$(assistant_comments)"
grep -Fq "$token" <<<"$comments" || die "assistant comment missing token ${token}"
grep -Fiq "proactive" <<<"$comments" || die "assistant comment missing word proactive"
grep -Fq "dispatch-${dispatch_id}" <<<"$comments" || die "assistant marker missing dispatch event id"
labels="$(jq -r '.labels[].name' <<<"$issue_json")"
grep -Fxq "gitclaw:proactive" <<<"$labels" || die "issue missing gitclaw:proactive label"
grep -Fxq "gitclaw:done" <<<"$labels" || die "issue missing gitclaw:done label"

second_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
dispatch_proactive "$second_started_at"
for _ in {1..6}; do
  issue_count="$(find_issue_numbers | wc -l | tr -d ' ')"
  if [[ "$issue_count" != "1" ]]; then
    die "same proactive slot created ${issue_count} issues"
  fi
  final_count="$(assistant_count)"
  if [[ "$final_count" != "1" ]]; then
    die "same proactive slot created ${final_count} assistant comments"
  fi
  sleep 5
done

log "idempotency verified"
log "passed for issue #${issue_number}"
