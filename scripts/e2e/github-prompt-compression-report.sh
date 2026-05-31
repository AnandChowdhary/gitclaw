#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "prompt-compression-report-e2e: $*" >&2
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
hidden_token="GITCLAW_PROMPT_COMPRESSION_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_PROMPT_COMPRESSION_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_PROMPT_COMPRESSION_CONTEXT_V1"
search_phrase="prompt compression unique search fixture phrase"
title="@gitclaw /prompt compression e2e ${timestamp}"
body="@gitclaw /prompt compression @file:docs/search-fixture.md:1-1

Live prompt-compression E2E.
Use the repo-reader skill and search for ${search_phrase}, but the deterministic prompt compression report must stay body-free.
Do not include this hidden issue token: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw prompt compression)"
for expected in \
  "GitClaw Prompt Compression Report" \
  'compression_strategy: `stateless-github-issue-bounded-prompt-audit`' \
  'compression_model: `hermes-dual-thresholds+openclaw-session-pruning`' \
  'compression_engine_configured: `false`' \
  'lossy_summary_supported: `false`' \
  'lossless_session_search_supported: `true`' \
  'issue_thread_canonical_storage: `true`' \
  'backup_branch_replay_preferred: `true`' \
  'prompt_body_included: `false`' \
  'context_file_bodies_included: `false`' \
  'skill_bodies_included: `false`' \
  'tool_output_bodies_included: `false`' \
  'llm_e2e_required_after_prompt_compression_change: `true`' \
  "### Compression Segments" \
  'kind=`system-prompt` name=`gitclaw-system-prompt`' \
  'kind=`context-file` name=`.gitclaw/SOUL.md`' \
  'kind=`tool-output` name=`gitclaw.list_files`' \
  'code=`hermes_dual_compression_thresholds_modeled`' \
  'code=`openclaw_session_pruning_boundary_modeled`' \
  'code=`lossy_compression_engine_disabled`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local prompt compression report missing ${expected}"
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
      gh issue close "$issue_number" --repo "$repo" --comment "prompt-compression-report e2e cleanup" >/dev/null 2>&1 || true
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

compression_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one prompt compression report comment"
compression_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/prompt"' \
  "GitClaw Prompt Compression Report" \
  "Generated without a model call" \
  'repository: `'"$repo"'`' \
  'issue: `#'"$issue_number"'`' \
  'event_kind: `issue_opened`' \
  'event_name: `issues`' \
  'prompt_compression_status: `warn`' \
  'compression_strategy: `stateless-github-issue-bounded-prompt-audit`' \
  'compression_model: `hermes-dual-thresholds+openclaw-session-pruning`' \
  'provider: `github-models`' \
  'model: `openai/gpt-5-nano`' \
  'agent_compression_threshold_percent: `50`' \
  'gateway_hygiene_threshold_percent: `85`' \
  'compression_engine_configured: `false`' \
  'lossy_summary_supported: `false`' \
  'lossless_session_search_supported: `true`' \
  'pre_agent_gateway_hygiene_supported: `false`' \
  'in_loop_context_compression_supported: `false`' \
  'compression_writes_memory_allowed: `false`' \
  'session_split_supported: `false`' \
  'external_session_db_required: `false`' \
  'issue_thread_canonical_storage: `true`' \
  'backup_branch_replay_preferred: `true`' \
  'context_files:' \
  'selected_skills: `1`' \
  'tool_outputs:' \
  'transcript_messages: `1`' \
  'bounded_transcript_messages: `1`' \
  'prompt_body_included: `false`' \
  'context_file_bodies_included: `false`' \
  'skill_bodies_included: `false`' \
  'tool_output_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'credential_values_included: `false`' \
  'repository_mutation_allowed: `false`' \
  'llm_e2e_required_after_prompt_compression_change: `true`' \
  "### Compression Segments" \
  'kind=`system-prompt` name=`gitclaw-system-prompt` compression_region=`stable-system-prefix`' \
  'kind=`context-file` name=`.gitclaw/SOUL.md` compression_region=`stable-context-prefix`' \
  'kind=`context-file` name=`docs/search-fixture.md:1` compression_region=`stable-context-prefix`' \
  'kind=`selected-skill` name=`.gitclaw/SKILLS/repo-reader/SKILL.md` compression_region=`stable-context-prefix`' \
  'kind=`tool-output` name=`gitclaw.search_files` compression_region=`dynamic-tool-context`' \
  'body_included=`false`' \
  "### Findings" \
  'code=`hermes_dual_compression_thresholds_modeled`' \
  'code=`openclaw_session_pruning_boundary_modeled`' \
  'code=`lossy_compression_engine_disabled`' \
  'code=`backup_branch_replay_preferred`'; do
  grep -Fq -- "$expected" <<<"$compression_comment" || die "prompt compression report missing ${expected}"
done

for leaked in \
  "$hidden_token" \
  "$expected_token" \
  "$search_phrase" \
  "Live prompt-compression E2E" \
  "Use the repo-reader skill"; do
  if grep -Fq "$leaked" <<<"$compression_comment"; then
    die "prompt compression report leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_PROMPT_COMPRESSION token from the matching repository search result line.
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
compression_url="$(jq -r '.url' <<<"$compression_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${compression_url} (model follow-up: ${model_url})"
