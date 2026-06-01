#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "proactive-chain-report-e2e: $*" >&2
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
token="NOECHO_PROACTIVE_CHAIN_${timestamp}"
followup_hidden_token="NOECHO_PROACTIVE_CHAIN_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_PROACTIVE_CHAIN_CONTEXT_V1"
search_phrase="proactive chain unique search fixture phrase"
title="@gitclaw /proactive chain e2e ${timestamp}"
body="Live proactive-chain-report E2E.

Hidden proactive chain report body token: ${token}
This should produce a deterministic proactive chain report without a model call."

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
      gh issue close "$issue_number" --repo "$repo" --comment "proactive-chain-report e2e cleanup" >/dev/null 2>&1 || true
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
      local run_status conclusion url
      run_status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$run_status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${event_name} run failed with conclusion ${conclusion}: ${url}"
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

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
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

run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one proactive chain report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/proactive"' \
  "GitClaw Proactive Chain Report" \
  "Generated without a model call" \
  'requested_proactive_command: `chain`' \
  'proactive_command_status: `ok`' \
  'proactive_chain_status: `no_chains`' \
  'chain_strategy: `github-actions-issue-thread-context-from`' \
  'upstream_pattern: `hermes-cron-context-from-openclaw-skill-backed-jobs`' \
  'prompt_files: `1`' \
  'prompt_files_with_context_from: `0`' \
  'chain_edges: `0`' \
  'missing_context_sources: `0`' \
  'self_references: `0`' \
  'cycle_nodes: `0`' \
  'skill_backed_prompt_files: `1`' \
  'prompt_skill_hints: `1`' \
  'fresh_issue_thread_per_name_slot: `true`' \
  'context_from_metadata_only: `true`' \
  'recursive_schedule_creation_allowed: `false`' \
  'no_agent_mode_supported: `false`' \
  'raw_prompt_bodies_included: `false`' \
  'raw_workflow_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_tool_outputs_included: `false`' \
  'credential_values_included: `false`' \
  'llm_e2e_required_after_proactive_chain_change: `true`' \
  'kind=`prompt-chain` name=`repo-hygiene` path=`.gitclaw/proactive/repo-hygiene.md`' \
  'skill_hints=`repo-reader`' \
  'context_from_refs=`0`' \
  'resolved_context_sources=`none`' \
  'missing_context_source_hashes=`none`' \
  '### Chain Edges' \
  '- none' \
  'context_from_gate=`no-chains`' \
  'prompt_body_gate=`metadata-hashes-and-context-refs-only`' \
  'skill_hint_gate=`metadata-only`' \
  'issue_thread_gate=`one-proactive-run-issue-per-name-slot`' \
  'scheduler_gate=`github-actions-workflow-dispatch`' \
  'recursive_schedule_gate=`disabled-inside-proactive-run`' \
  'model_e2e_gate=`required`' \
  '### Findings' \
  '- none'; do
  grep -Fq -- "$expected" <<<"$comments" || die "proactive chain report missing ${expected}"
done

for leaked in "$token" "$expected_token" "$search_phrase" "Run a short, read-only repository hygiene check" "Suggested GitClaw skills"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "proactive chain report leaked ${leaked}"
  fi
done

local_report="$(go run ./cmd/gitclaw proactive chain)"
grep -Fq 'scope: `local-cli`' <<<"$local_report" || die "local proactive chain report missing CLI scope"
grep -Fq 'proactive_chain_status: `no_chains`' <<<"$local_report" || die "local proactive chain report missing no_chains status"
grep -Fq 'raw_prompt_bodies_included: `false`' <<<"$local_report" || die "local proactive chain report missing prompt body gate"
if grep -Fq "$token" <<<"$local_report"; then
  die "local proactive chain report leaked issue token"
fi

url="$(jq -r '.url' <<<"$run_json")"
log "proactive chain report verified for issue #${issue_number}: ${url}"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Search for that exact phrase, not shorter words from it.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${search_phrase} => <token>\`.
Reply with only the uppercase fixture token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with any token from this issue or its comments.
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
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "assistant marker missing usage token telemetry"

for leaked in "$token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${url} (model follow-up: ${model_url})"
