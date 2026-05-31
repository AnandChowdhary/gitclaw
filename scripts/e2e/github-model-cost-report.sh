#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "model-cost-report-e2e: $*" >&2
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
seed_hidden_token="GITCLAW_MODEL_COST_SEED_HIDDEN_${timestamp}"
cost_hidden_token="GITCLAW_MODEL_COST_REPORT_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_MODEL_COST_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_MODEL_COST_CONTEXT_V1"
search_phrase="model cost unique search fixture phrase"
title="@gitclaw model cost e2e ${timestamp}"
body="Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_MODEL_COST token from the matching repository search result line.
Do not include this hidden issue token: ${seed_hidden_token}
Keep the answer under 30 words."

local_report="$(go run ./cmd/gitclaw models cost)"
for expected in \
  "GitClaw Model Cost Report" \
  "Generated without a model call" \
  'scope: `local-cli`' \
  'verification_scope: `github_models_direct_cost_catalog`' \
  'pricing_source: `github_models_direct_costs_snapshot`' \
  'pricing_source_url: `https://docs.github.com/en/billing/reference/costs-for-github-models`' \
  'pricing_snapshot_date: `2026-05-31`' \
  'token_unit_price_usd: `0.00001`' \
  'current_model_catalog_entry_present: `false`' \
  'current_model_cost_estimation_supported: `false`' \
  'projected_usd: `unavailable`' \
  'billing_api_probe_performed: `false`' \
  'live_inference_probe_performed: `false`' \
  'raw_provider_usage_included: `false`' \
  'raw_provider_response_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'llm_e2e_required_after_model_cost_change: `true`' \
  "### Cost Cards" \
  'kind=`current-model-cost`' \
  'kind=`recorded-usage-cost`' \
  'code=`github_models_token_unit_pricing_modeled`' \
  'code=`openclaw_usage_cost_surface_modeled`' \
  'code=`hermes_api_token_count_boundary_modeled`' \
  'code=`billing_api_not_queried`' \
  'code=`current_model_multiplier_unknown`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local model cost report missing ${expected}"
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
      gh issue close "$issue_number" --repo "$repo" --comment "model-cost-report e2e cleanup" >/dev/null 2>&1 || true
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

seed_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for seed issues workflow run"
wait_for_assistant_count 1 || die "expected one model-backed seed assistant comment"
seed_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$seed_comment" || die "seed assistant did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$seed_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$seed_comment"; then
  die "seed assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$seed_comment" || die "seed assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$seed_comment" || die "seed assistant marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$seed_comment" || die "seed assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$seed_comment" || die "seed assistant marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$seed_comment" || die "seed assistant marker missing normalized provider usage"

if grep -Fq "$seed_hidden_token" <<<"$seed_comment"; then
  die "seed assistant leaked ${seed_hidden_token}"
fi

cost_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /models cost

Do not include this hidden cost-report token: ${cost_hidden_token}" >/dev/null

cost_run_json="$(wait_for_run issue_comment "$cost_started_at")" || die "timed out waiting for cost issue_comment workflow run"
wait_for_assistant_count 2 || die "expected cost report assistant comment"
cost_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/models"' \
  "GitClaw Model Cost Report" \
  "Generated without a model call" \
  'repository: `'"$repo"'`' \
  'issue: `#'"$issue_number"'`' \
  'event_kind: `issue_comment`' \
  'event_name: `issue_comment`' \
  'model_cost_status: `warn`' \
  'verification_scope: `github_models_direct_cost_catalog`' \
  'provider: `github-models`' \
  'model: `openai/gpt-5-nano`' \
  'fallback_models: `openai/gpt-4.1-nano`' \
  'endpoint_host: `models.github.ai`' \
  'token_source: `GITHUB_TOKEN`' \
  'pricing_source: `github_models_direct_costs_snapshot`' \
  'pricing_source_url: `https://docs.github.com/en/billing/reference/costs-for-github-models`' \
  'pricing_snapshot_date: `2026-05-31`' \
  'token_unit_price_usd: `0.00001`' \
  'current_model_catalog_entry_present: `false`' \
  'current_model_cost_estimation_supported: `false`' \
  'projected_usd: `unavailable`' \
  'usage_bearing_assistant_turns: `1`' \
  'costed_usage_turns: `0`' \
  'uncosted_usage_turns: `1`' \
  'recorded_estimated_usd: `unavailable`' \
  'billing_api_probe_performed: `false`' \
  'live_inference_probe_performed: `false`' \
  'billing_account_state_known: `false`' \
  'paid_usage_opt_in_state_known: `false`' \
  'github_budget_state_known: `false`' \
  'raw_provider_usage_included: `false`' \
  'raw_provider_response_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'credential_values_included: `false`' \
  'llm_e2e_required_after_model_cost_change: `true`' \
  "### Cost Cards" \
  'kind=`current-model-cost` model=`openai/gpt-5-nano` catalog_entry_present=`false`' \
  'kind=`recorded-usage-cost` usage_bearing_assistant_turns=`1` costed_usage_turns=`0` uncosted_usage_turns=`1`' \
  'catalog_entry_present=`false`' \
  'estimated_usd=`unavailable`' \
  'code=`github_models_token_unit_pricing_modeled`' \
  'code=`openclaw_usage_cost_surface_modeled`' \
  'code=`hermes_api_token_count_boundary_modeled`' \
  'code=`billing_api_not_queried`' \
  'code=`current_model_multiplier_unknown`' \
  'code=`uncosted_usage_markers_seen`'; do
  grep -Fq -- "$expected" <<<"$cost_comment" || die "model cost report missing ${expected}"
done

if ! grep -Eq 'recorded_total_tokens: `[1-9][0-9]*`' <<<"$cost_comment"; then
  die "model cost report did not include nonzero recorded_total_tokens"
fi
if ! grep -Eq 'usage_total_tokens="[1-9][0-9]*"' <<<"$seed_comment"; then
  die "seed assistant marker had no nonzero usage_total_tokens"
fi
if ! grep -Eq 'uncosted_model_names: `openai/gpt-5-nano|uncosted_model_names: `openai/gpt-4.1-nano' <<<"$cost_comment"; then
  die "model cost report did not name the uncosted primary or fallback model"
fi

for leaked in "$seed_hidden_token" "$cost_hidden_token" "$expected_token" "$search_phrase" "Reply with only the exact"; do
  if grep -Fq "$leaked" <<<"$cost_comment"; then
    die "model cost report leaked ${leaked}"
  fi
done

followup_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository again for \`${search_phrase}\`.

Reply with only the exact GITCLAW_MODEL_COST token from the matching repository search result line.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

followup_run_json="$(wait_for_run issue_comment "$followup_started_at")" || die "timed out waiting for model follow-up issue_comment workflow run"
wait_for_assistant_count 3 || die "expected model-backed follow-up assistant comment"
followup_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$followup_comment" || die "follow-up assistant did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$followup_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$followup_comment"; then
  die "follow-up assistant marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$followup_comment" || die "follow-up assistant marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$followup_comment" || die "follow-up assistant marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$followup_comment" || die "follow-up assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$followup_comment" || die "follow-up assistant marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$followup_comment" || die "follow-up assistant marker missing normalized provider usage"

if grep -Fq "$followup_hidden_token" <<<"$followup_comment"; then
  die "follow-up assistant leaked ${followup_hidden_token}"
fi

wait_for_done_status || die "expected gitclaw:done without running/error"
seed_url="$(jq -r '.url' <<<"$seed_run_json")"
cost_url="$(jq -r '.url' <<<"$cost_run_json")"
followup_url="$(jq -r '.url' <<<"$followup_run_json")"
log "passed for issue #${issue_number}: ${seed_url} (cost: ${cost_url}, model follow-up: ${followup_url})"
