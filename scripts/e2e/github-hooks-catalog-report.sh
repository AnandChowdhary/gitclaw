#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "hooks-catalog-report-e2e: $*" >&2
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
hidden_token="NOECHO_HOOK_CATALOG_SENTINEL_${timestamp}"
followup_hidden_token="NOECHO_HOOK_CATALOG_FOLLOWUP_SENTINEL_${timestamp}"
expected_token="GITCLAW_HOOK_CATALOG_CONTEXT_V1"
search_phrase="hook catalog unique search fixture phrase"
title="@gitclaw /hooks catalog e2e ${timestamp}"
body="@gitclaw /hooks catalog

Live hook catalog E2E. Keep the catalog report body-free.
Do not include this hidden hook catalog sentinel: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw hooks catalog)"
for expected in \
  "GitClaw Hooks Catalog Report" \
  'scope: `local-cli`' \
  'hook_catalog_status: `ok`' \
  'catalog_strategy: `compact-event-hook-discovery`' \
  'catalog_scope: `hook-policy-specs-events-provenance`' \
  'hook_surface_model: `repo-reviewed-hooks-plus-github-actions`' \
  'hooks_status: `ok`' \
  'hook_risk_status: `ok`' \
  'hook_provenance_status: `ok`' \
  'hook_policy_path: `.gitclaw/HOOKS.md`' \
  'hook_policy_present: `true`' \
  'hook_policy_loaded_for_model: `true`' \
  'hook_specs_dir: `.gitclaw/hooks`' \
  'hook_specs: `1`' \
  'hook_specs_with_frontmatter: `1`' \
  'hook_events: `2`' \
  'hook_specs_requiring_approval: `1`' \
  'hook_specs_audit_only: `1`' \
  'executable_handlers_present: `0`' \
  'git_tracked_hook_surfaces: `2`' \
  'working_tree_dirty_hook_surfaces: `0`' \
  'catalog_entries: `5`' \
  'hook_layers: `7`' \
  'hook_execution_supported: `false`' \
  'hook_execution_allowed: `false`' \
  'handler_execution_allowed: `false`' \
  'provider_payload_ingest_enabled: `false`' \
  'model_call_required: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_bodies_included: `false`' \
  'raw_hook_bodies_included: `false`' \
  'raw_handler_bodies_included: `false`' \
  'raw_provider_payloads_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'credential_values_included: `false`' \
  'llm_e2e_required_after_hook_catalog_change: `true`' \
  'command=`catalog` issue_intent=`@gitclaw /hooks catalog` local_command=`gitclaw hooks catalog` execution=`metadata-only` gate=`body-free-hook-command-map` raw_bodies_included=`false` mutation_allowed=`false`' \
  'command=`list` issue_intent=`@gitclaw /hooks` local_command=`gitclaw hooks list`' \
  'command=`verify` issue_intent=`@gitclaw /hooks verify` local_command=`gitclaw hooks verify`' \
  'command=`risk` issue_intent=`@gitclaw /hooks risk` local_command=`gitclaw hooks risk`' \
  'command=`provenance` issue_intent=`@gitclaw /hooks provenance` local_command=`gitclaw hooks provenance`' \
  'layer=`policy` store=`.gitclaw/HOOKS.md`' \
  'layer=`specs` store=`.gitclaw/hooks/*.md`' \
  'layer=`events` store=`hook frontmatter events`' \
  'layer=`approval` store=`requires_approval frontmatter`' \
  'layer=`handlers` store=`executable-looking hook files`' \
  'layer=`provenance` store=`git history`' \
  'layer=`provider-payloads` store=`unsupported external payloads`' \
  'validation_gate=`pass`' \
  'risk_gate=`pass`' \
  'provenance_gate=`pass`' \
  'context_gate=`hook-policy-loaded-before-model`' \
  'event_gate=`declarative-events-only`' \
  'approval_gate=`side-effects-require-approval`' \
  'handler_gate=`disabled-not-executed`' \
  'provider_payload_gate=`not-ingested`' \
  'raw_body_gate=`hashes-counts-and-metadata-only`' \
  'model_e2e_gate=`required`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local hook catalog report missing ${expected}"
done

for leaked in "GITCLAW_HOOKS_CONTEXT_V1" "Repo Hygiene Audit" "GitClaw hooks are declarative" "$expected_token" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$local_report"; then
    die "local hook catalog report leaked ${leaked}"
  fi
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
      gh issue close "$issue_number" --repo "$repo" --comment "hooks-catalog-report e2e cleanup" >/dev/null 2>&1 || true
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

catalog_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one hook catalog report comment"
catalog_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/hooks"' \
  "GitClaw Hooks Catalog Report" \
  "Generated without a model call" \
  'requested_hooks_command: `catalog`' \
  'hooks_command_status: `ok`' \
  'hook_catalog_status: `ok`' \
  'catalog_strategy: `compact-event-hook-discovery`' \
  'catalog_scope: `hook-policy-specs-events-provenance`' \
  'hook_surface_model: `repo-reviewed-hooks-plus-github-actions`' \
  'hooks_status: `ok`' \
  'hook_risk_status: `ok`' \
  'hook_provenance_status: `ok`' \
  'hook_policy_present: `true`' \
  'hook_policy_loaded_for_model: `true`' \
  'hook_specs: `1`' \
  'hook_specs_with_frontmatter: `1`' \
  'hook_events: `2`' \
  'hook_specs_requiring_approval: `1`' \
  'hook_specs_audit_only: `1`' \
  'executable_handlers_present: `0`' \
  'git_tracked_hook_surfaces: `2`' \
  'working_tree_dirty_hook_surfaces: `0`' \
  'catalog_entries: `5`' \
  'hook_layers: `7`' \
  'hook_execution_allowed: `false`' \
  'handler_execution_allowed: `false`' \
  'provider_payload_ingest_enabled: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_hook_bodies_included: `false`' \
  'raw_handler_bodies_included: `false`' \
  'raw_provider_payloads_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'credential_values_included: `false`' \
  'llm_e2e_required_after_hook_catalog_change: `true`' \
  'command=`catalog` issue_intent=`@gitclaw /hooks catalog` local_command=`gitclaw hooks catalog`' \
  'command=`provenance` issue_intent=`@gitclaw /hooks provenance` local_command=`gitclaw hooks provenance`' \
  'layer=`policy` store=`.gitclaw/HOOKS.md`' \
  'layer=`provider-payloads` store=`unsupported external payloads`' \
  'validation_gate=`pass`' \
  'risk_gate=`pass`' \
  'provenance_gate=`pass`' \
  'handler_gate=`disabled-not-executed`' \
  'provider_payload_gate=`not-ingested`' \
  'model_e2e_gate=`required`'; do
  grep -Fq -- "$expected" <<<"$catalog_comment" || die "hook catalog report missing ${expected}"
done

for leaked in "$hidden_token" "Live hook catalog E2E" "GITCLAW_HOOKS_CONTEXT_V1" "Repo Hygiene Audit" "GitClaw hooks are declarative" "$expected_token" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$catalog_comment"; then
    die "hook catalog report leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error after hook catalog"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact token from the matching line in docs/search-fixture.md.
The only valid answer ends in _CONTEXT_V1.
Do not answer with any token from the issue title/body/comment text.
Do not include this hidden follow-up sentinel: ${followup_hidden_token}
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

for leaked in "$hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error after model follow-up"
catalog_url="$(jq -r '.url' <<<"$catalog_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${catalog_url} (model follow-up: ${model_url})"
