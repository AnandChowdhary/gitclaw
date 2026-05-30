#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "runs-report-e2e: $*" >&2
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
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
token="GITCLAW_RUNS_REPORT_E2E_${timestamp}"
title="@gitclaw /runs e2e ${timestamp}"
body="Live runs-report E2E.

Hidden runs report body token: ${token}
Show the deterministic current-turn ledger report without a model call or raw body leakage."

issue_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label gitclaw)"
issue_number="${issue_url##*/}"

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "runs-report e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
}
trap cleanup EXIT

log "created issue #${issue_number}: ${issue_url}"

wait_for_run() {
  local started_at="$1"
  for _ in {1..90}; do
    local run_json
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$workflow_name" \
      --event issues \
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
        [[ "$conclusion" == "success" ]] || die "issues run failed with conclusion ${conclusion}: ${url}"
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

run_json="$(wait_for_run "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one runs report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/runs"' \
  "GitClaw Run Ledger Report" \
  "Generated without a model call" \
  'repository: `'"$repo"'`' \
  'issue: `#'"$issue_number"'`' \
  'event_kind: `issue_opened`' \
  'event_name: `issues`' \
  'event_action: `opened`' \
  'event_id: `issue-'"$issue_number"'`' \
  'active_command: `/runs`' \
  'idempotency_key: `' \
  'run_id: `' \
  'run_attempt: `' \
  'run_environment_sha256_12: `' \
  'run_url_present: `true`' \
  'run_url_sha256_12: `' \
  'event_sha256_12: `' \
  'preflight_allowed: `true`' \
  'preflight_code: `allowed`' \
  'actor_association: `OWNER`' \
  'actor_trusted: `true`' \
  'triggered: `true`' \
  'disabled_label_present: `false`' \
  'write_request_detected: `false`' \
  'raw_comments_before_turn: `0`' \
  'transcript_messages: `1`' \
  'user_messages: `1`' \
  'assistant_messages: `0`' \
  'assistant_turn_comments_before_turn: `0`' \
  'heartbeat_comments_before_turn: `0`' \
  'error_marker_comments_before_turn: `0`' \
  'channel_message_comments_before_turn: `0`' \
  'context_documents: `7`' \
  'selected_skills: `0`' \
  'available_skills: `1`' \
  'skill_bundles: `1`' \
  'active_tool_outputs: `' \
  'run_ledger_store: `github-issue-comments+actions-run`' \
  'backup_branch: `gitclaw-backups`' \
  'run_ledger_writes_allowed: `false`' \
  'raw_bodies_included: `false`' \
  'raw_run_payloads_included: `false`' \
  'issue_title_sha256_12: `' \
  "### Label State" \
  '`gitclaw` present=`true`' \
  '`gitclaw:disabled` present=`false`' \
  "### Prompt-Visible Inputs" \
  'kind=`context` path=`.gitclaw/SOUL.md`' \
  'kind=`context` path=`.gitclaw/IDENTITY.md`' \
  'kind=`context` path=`.gitclaw/MEMORY.md`' \
  'kind=`context` path=`.gitclaw/memory/2026-05-29.md`' \
  "### Tool Outputs" \
  'name=`gitclaw.list_files` input_sha256_12=`' \
  'output_sha256_12=`' \
  "### Ledger Notes" \
  "issue comments remain the canonical conversation log" \
  "GitHub Actions remains the canonical execution trace" \
  "gitclaw-backups remains the canonical post-turn backup branch when enabled"; do
  grep -Fq -- "$expected" <<<"$comments" || die "runs report missing ${expected}"
done

for leaked in \
  "$token" \
  "Hidden runs report body token" \
  "Show the deterministic current-turn ledger report" \
  "GitClaw is a repo-native GitHub issue assistant" \
  "GITCLAW_MEMORY_CONTEXT_V1"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "runs report leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
