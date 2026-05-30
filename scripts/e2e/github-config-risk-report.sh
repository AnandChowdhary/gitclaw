#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "config-risk-report-e2e: $*" >&2
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
hidden_token="GITCLAW_CONFIG_RISK_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_CONFIG_RISK_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw /config risk e2e ${timestamp}"
body="@gitclaw /config risk

Live config-risk E2E.
Use repo-reader after the deterministic report when a follow-up comment arrives.
Do not include this hidden issue token: ${hidden_token}"

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
      gh issue close "$issue_number" --repo "$repo" --comment "config-risk-report e2e cleanup" >/dev/null 2>&1 || true
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

risk_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one config risk report comment"
risk_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/config"' \
  "GitClaw Config Risk Report" \
  "Generated without a model call" \
  'config_risk_status: `ok`' \
  'verification_scope: `repo_local_config_control_plane`' \
  'config_source: `defaults+repo+environment`' \
  'config_file_path: `.gitclaw/config.yml`' \
  'config_file_present: `true`' \
  'workflow_files_expected: `7`' \
  'workflow_files_present: `7`' \
  'workflow_files_missing: `0`' \
  'trigger_mode: `label-or-prefix`' \
  'trigger_label: `gitclaw`' \
  'trigger_prefix: `@gitclaw`' \
  'disabled_label: `gitclaw:disabled`' \
  'trusted_associations: `COLLABORATOR, MEMBER, OWNER`' \
  'trusted_associations_configured: `3`' \
  'broad_trusted_associations: `none`' \
  'broad_trusted_associations_configured: `0`' \
  'managed_labels_configured: `9`' \
  'duplicate_managed_labels: `0`' \
  'model_provider: `github-models`' \
  'model: `openai/gpt-5-nano`' \
  'model_fallbacks: `openai/gpt-4.1-nano`' \
  'model_fallbacks_configured: `1`' \
  'run_mode: `read-only`' \
  'max_prompt_bytes: `60000`' \
  'max_output_tokens: `4000`' \
  'max_transcript_messages: `40`' \
  'max_transcript_message_bytes: `8000`' \
  'skills_allowed_configured: `0`' \
  'skills_disabled_configured: `0`' \
  'skill_gate_conflicts: `0`' \
  'tools_allowed_configured: `0`' \
  'tools_disabled_configured: `0`' \
  'tool_gate_conflicts: `0`' \
  'slash_commands: `33`' \
  'surfaces_with_risk_findings: `0`' \
  'config_risk_findings: `0`' \
  'high_risk_findings: `0`' \
  'warning_risk_findings: `0`' \
  'info_risk_findings: `0`' \
  'raw_config_bodies_included: `false`' \
  'raw_workflow_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_provider_error_bodies_included: `false`' \
  'credential_values_included: `false`' \
  'repository_mutation_allowed: `false`' \
  'agent_authored_config_mutation_supported: `false`' \
  'llm_e2e_required_after_config_risk_change: `true`' \
  "### Config File Risk Card" \
  'kind=`config-file` path=`.gitclaw/config.yml` present=`true`' \
  "### Workflow Risk Cards" \
  'kind=`workflow-file` path=`.github/workflows/gitclaw.yml` present=`true`' \
  'kind=`workflow-file` path=`.github/workflows/gitclaw-heartbeat.yml` present=`true`' \
  'kind=`workflow-file` path=`.github/workflows/gitclaw-proactive.yml` present=`true`' \
  'kind=`workflow-file` path=`.github/workflows/gitclaw-channel-ingest.yml` present=`true`' \
  'kind=`workflow-file` path=`.github/workflows/gitclaw-channel-state.yml` present=`true`' \
  'kind=`workflow-file` path=`.github/workflows/gitclaw-channel-gateway.yml` present=`true`' \
  'kind=`workflow-file` path=`.github/workflows/gitclaw-channel-delivery.yml` present=`true`' \
  "### Trigger And Trust Risk Card" \
  'kind=`trigger-trust` trigger_mode=`label-or-prefix` trigger_label=`gitclaw` trigger_prefix=`@gitclaw` disabled_label=`gitclaw:disabled`' \
  "### Model And Budget Risk Card" \
  'kind=`model-budget` model_provider=`github-models` model=`openai/gpt-5-nano`' \
  "### Gate Risk Card" \
  'kind=`gate` skills_allowed_configured=`0` skills_disabled_configured=`0`' \
  'risk_findings=`0`' \
  'risk_codes=`none`' \
  "### Current Config Request Risk Card" \
  'current_issue_config_request=`true`' \
  'issue_body_scanned=`false`' \
  'comment_bodies_scanned=`false`' \
  "### Risk Findings" \
  "- none"; do
  grep -Fq -- "$expected" <<<"$risk_comment" || die "config risk report missing ${expected}"
done

for leaked in "$hidden_token" "Use repo-reader after the deterministic report" "$search_phrase" "permissions:" "contents: read" "workflow_dispatch:"; do
  if grep -Fq "$leaked" <<<"$risk_comment"; then
    die "config risk report leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_SEARCH token from the matching repository search result line.
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

for leaked in "$hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
risk_url="$(jq -r '.url' <<<"$risk_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${risk_url} (model follow-up: ${model_url})"
