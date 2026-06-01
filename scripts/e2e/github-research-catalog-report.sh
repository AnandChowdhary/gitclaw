#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "research-catalog-report-e2e: $*" >&2
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
hidden_token="GITCLAW_RESEARCH_CATALOG_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_RESEARCH_CATALOG_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_RESEARCH_CATALOG_CONTEXT_V1"
search_phrase="research catalog unique search fixture phrase"
title="@gitclaw /research catalog e2e ${timestamp}"
body="@gitclaw /research catalog

Live research-catalog E2E.
Map the OpenClaw and Hermes research surface without browsing or model inference.
Do not include this hidden issue token: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw research catalog)"
for expected in \
  "GitClaw Research Catalog Report" \
  'scope: `local-cli`' \
  'research_catalog_status: `ok`' \
  'catalog_strategy: `primary-source-to-repo-native-design-map`' \
  'research_scope: `openclaw, hermes-agent, nano-mini-claw-variants`' \
  'source_snapshot_date: `2026-06-01`' \
  'reviewed_sources: `10`' \
  'primary_sources: `10`' \
  'official_docs_sources: `8`' \
  'official_repo_sources: `2`' \
  'local_research_docs: `3`' \
  'local_research_docs_present: `3`' \
  'implemented_patterns: `6`' \
  'adapted_patterns: `5`' \
  'rejected_patterns: `5`' \
  'source_fetch_performed: `false`' \
  'live_source_browse_performed: `false`' \
  'raw_research_bodies_included: `false`' \
  'raw_source_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'credential_values_included: `false`' \
  'llm_e2e_required_after_research_catalog_change: `true`' \
  "### Local Research Files" \
  "docs/research-openclaw-hermes-landscape.md" \
  "docs/spec-github-native-gitclaw.md" \
  "### Reviewed Sources" \
  'source_id=`openclaw-architecture`' \
  'source_id=`openclaw-skills`' \
  'source_id=`hermes-skills`' \
  'source_id=`hermes-checkpoints`' \
  "### Pattern Coverage" \
  'pattern=`serverless-wakeup`' \
  'pattern=`progressive-skills`' \
  'pattern=`checkpoint-readiness`' \
  "### Rejected Patterns" \
  'surface=`long-running-gateway-socket`' \
  'surface=`agent-managed-skill-writes`' \
  'surface=`remote-skill-install`' \
  'runtime_fetch_gate=`disabled-static-reviewed-snapshot`' \
  'model_e2e_gate=`required`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local research catalog report missing ${expected}"
done

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
      gh issue close "$issue_number" --repo "$repo" --comment "research-catalog-report e2e cleanup" >/dev/null 2>&1 || true
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
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${event_name} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

assistant_count() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn"))] | length'
}

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
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

research_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one research catalog report comment"
research_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/research"' \
  "GitClaw Research Catalog Report" \
  "Generated without a model call" \
  'repository: `'"$repo"'`' \
  'issue: `#'"$issue_number"'`' \
  'requested_research_command: `catalog`' \
  'research_command_status: `ok`' \
  'research_catalog_status: `ok`' \
  'catalog_strategy: `primary-source-to-repo-native-design-map`' \
  'research_scope: `openclaw, hermes-agent, nano-mini-claw-variants`' \
  'source_snapshot_date: `2026-06-01`' \
  'reviewed_sources: `10`' \
  'primary_sources: `10`' \
  'official_docs_sources: `8`' \
  'official_repo_sources: `2`' \
  'local_research_docs: `3`' \
  'local_research_docs_present: `3`' \
  'implemented_patterns: `6`' \
  'adapted_patterns: `5`' \
  'rejected_patterns: `5`' \
  'source_fetch_performed: `false`' \
  'live_source_browse_performed: `false`' \
  'raw_research_bodies_included: `false`' \
  'raw_source_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'credential_values_included: `false`' \
  'llm_e2e_required_after_research_catalog_change: `true`' \
  "### Reviewed Sources" \
  'source_id=`openclaw-architecture`' \
  'source_id=`hermes-skills`' \
  'source_id=`hermes-cron`' \
  "### Pattern Coverage" \
  'pattern=`serverless-wakeup`' \
  'pattern=`multi-channel-ingress`' \
  'pattern=`backup-durability`' \
  "### Rejected Patterns" \
  'surface=`long-running-gateway-socket`' \
  'surface=`remote-skill-install`' \
  'runtime_fetch_gate=`disabled-static-reviewed-snapshot`' \
  'model_e2e_gate=`required`'; do
  grep -Fq -- "$expected" <<<"$research_comment" || die "research catalog report missing ${expected}"
done

for leaked in "$hidden_token" "Map the OpenClaw and Hermes research surface" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$research_comment"; then
    die "research catalog report leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_RESEARCH_CATALOG token from the matching repository search result line.
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
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant marker missing usage telemetry"

for leaked in "$hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
research_url="$(jq -r '.url' <<<"$research_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${research_url} (model follow-up: ${model_url})"
