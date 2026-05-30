#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "memory-promote-plan-report-e2e: $*" >&2
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
token="GITCLAW_MEMORY_PROMOTE_PLAN_E2E_${timestamp}"
title="@gitclaw /memory promote-plan long-term e2e ${timestamp}"
body="Live memory promote-plan E2E.

Hidden memory promote token: ${token}
This asks for a deterministic dry-run memory promotion plan. It must not write memory, call a model, dump transcript bodies, dump memory bodies, or echo this issue text.
Any memory promotion behavior change must also run a live GitHub Models conversation E2E that performs an actual model call."

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
      gh issue close "$issue_number" --repo "$repo" --comment "memory-promote-plan-report e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one memory promote-plan report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/memory"' \
  "GitClaw Memory Promote Plan Report" \
  "Generated without a model call" \
  'memory_promote_plan_status: `needs_review`' \
  'source_scope: `issue-thread-transcript-metadata`' \
  'normalized_target_kind: `long-term`' \
  'normalized_target_path: `.gitclaw/MEMORY.md`' \
  'supported_target: `true`' \
  'target_present: `true`' \
  'memory_budget_bytes: `12000`' \
  'dated_memory_notes: `1`' \
  'latest_memory_note: `.gitclaw/memory/2026-05-29.md`' \
  'model_call_required: `false`' \
  'repository_mutation_allowed: `false`' \
  'memory_writes_allowed: `false`' \
  'candidate_generation_included: `false`' \
  'manual_review_required: `true`' \
  'llm_e2e_required_after_change: `true`' \
  'raw_candidate_memory_included: `false`' \
  'raw_transcript_bodies_included: `false`' \
  'raw_memory_bodies_included: `false`' \
  'memory_validation_status: `ok`' \
  "### Target Memory File" \
  '.gitclaw/MEMORY.md' \
  "### Promotion Boundaries" \
  "route user-profile or communication-style changes through" \
  "### Review Steps" \
  "actual model call" \
  "### Findings" \
  'code=`durable_memory_is_prompt_authority`' \
  'code=`repository_mutation_disabled`' \
  'code=`body_suppression_enabled`' \
  'code=`manual_review_required`' \
  'code=`compact_memory_required`'; do
  grep -Fq -- "$expected" <<<"$comments" || die "memory promote-plan report missing ${expected}"
done

for leaked in "$token" "$title" "Hidden memory promote token" "deterministic dry-run memory promotion plan"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "memory promote-plan report leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
