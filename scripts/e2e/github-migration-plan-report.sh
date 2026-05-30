#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "migration-plan-report-e2e: $*" >&2
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
token="GITCLAW_MIGRATION_PLAN_E2E_${timestamp}"
title="@gitclaw /migrate plan hermes e2e ${timestamp}"
body="Live migration-plan E2E.

Hidden migration plan token: ${token}
This should produce a deterministic dry-run migration plan for Hermes without scanning source homes, importing secrets, mutating the repository, or dumping issue bodies.
Any migration behavior change must also run a live GitHub Models conversation E2E."

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
      gh issue close "$issue_number" --repo "$repo" --comment "migration-plan-report e2e cleanup" >/dev/null 2>&1 || true
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
      die "assistant run posted ${errors} error comment(s)"
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
wait_for_assistant_count 1 || die "expected one migration plan report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/migration"' \
  "GitClaw Migration Plan Report" \
  "Generated without a model call" \
  'migration_plan_status: `needs_review`' \
  'normalized_source: `hermes`' \
  'supported_source: `true`' \
  'plan_scope: `repo-local-declarative-state`' \
  'source_scan_allowed: `false`' \
  'apply_supported: `false`' \
  'model_call_required: `false`' \
  'repository_mutation_allowed: `false`' \
  'backup_required_before_apply: `true`' \
  'credentials_import_allowed: `false`' \
  'executable_state_import_allowed: `false`' \
  'raw_source_body_included: `false`' \
  'raw_secret_values_included: `false`' \
  'llm_e2e_required_after_change: `true`' \
  'backup_branch: `gitclaw-backups`' \
  'backup_schema_version: `1`' \
  'soul_validation_status: `ok`' \
  'skill_validation_status: `ok`' \
  'tool_validation_status: `ok`' \
  "### Source Import Map" \
  'source_kind=`config.yaml providers` target=`.gitclaw/config.yml` action=`reviewed-merge`' \
  'source_kind=`SOUL.md` target=`.gitclaw/SOUL.md` action=`manual-copy`' \
  'source_kind=`memories/MEMORY.md` target=`.gitclaw/MEMORY.md` action=`reviewed-append`' \
  'source_kind=`skills/<name>/SKILL.md` target=`.gitclaw/SKILLS/<name>/SKILL.md` action=`manual-copy`' \
  'source_kind=`auth.json/.env` target=`manual secret setup` action=`skip`' \
  "### Current GitClaw Target Inventory" \
  'kind=`context` path=`.gitclaw/SOUL.md`' \
  'kind=`skill` name=`repo-reader`' \
  "### Review Steps" \
  "Verify the current GitClaw backup branch" \
  "### Findings" \
  'code=`preview_first`' \
  'code=`backup_first`' \
  'code=`credentials_manual`' \
  'code=`executable_state_quarantined`' \
  'code=`manual_review_required`'; do
  grep -Fq -- "$expected" <<<"$comments" || die "migration plan report missing ${expected}"
done

for leaked in "$token" "$title" "Hidden migration plan token" "deterministic dry-run migration plan"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "migration plan report leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
