#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "context-risk-report-e2e: $*" >&2
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
hidden_token="GITCLAW_CONTEXT_RISK_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_CONTEXT_RISK_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw /context risk e2e ${timestamp}"
body="@gitclaw /context risk @file:docs/search-fixture.md:1-1

Live context-risk E2E.
Use the repo-reader skill and search for ${search_phrase}, but the deterministic context risk report must stay body-free.
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
      gh issue close "$issue_number" --repo "$repo" --comment "context-risk-report e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one context risk report comment"
risk_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/context"' \
  "GitClaw Context Risk Report" \
  "Generated without a model call" \
  'repository: `'"$repo"'`' \
  'issue: `#'"$issue_number"'`' \
  'event_kind: `issue_opened`' \
  'context_risk_status: `ok`' \
  'verification_scope: `context-files-references-skills-tools-and-prompt-boundary`' \
  'run_mode: `read-only`' \
  'model: `openai/gpt-5-nano`' \
  'context_files_loaded: `17`' \
  'context_references: `1`' \
  'loaded_context_references: `1`' \
  'blocked_context_references: `0`' \
  'failed_context_references: `0`' \
  'file_context_references: `1`' \
  'folder_context_references: `0`' \
  'git_context_references: `0`' \
  'unsupported_url_references: `0`' \
  'selected_skills: `1`' \
  'skill_summaries: `1`' \
  'skill_bundles: `1`' \
  'active_tool_outputs: `4`' \
  'max_prompt_bytes: `60000`' \
  'max_transcript_messages: `40`' \
  'max_transcript_message_bytes: `8000`' \
  'max_context_reference_bytes: `12000`' \
  'max_context_folder_entries: `120`' \
  'max_context_git_commits: `10`' \
  'max_tool_read_bytes: `8000`' \
  'max_repo_files_listed: `240`' \
  'max_search_queries: `5`' \
  'max_search_matches: `20`' \
  'max_search_matches_per_query: `5`' \
  'surfaces_with_risk_findings: `0`' \
  'context_risk_findings: `0`' \
  'high_risk_findings: `0`' \
  'warning_risk_findings: `0`' \
  'info_risk_findings: `0`' \
  'context_file_bodies_included: `false`' \
  'context_reference_bodies_included: `false`' \
  'skill_bodies_included: `false`' \
  'tool_output_bodies_included: `false`' \
  'raw_issue_bodies_included: `false`' \
  'raw_comment_bodies_included: `false`' \
  'raw_inputs_included: `false`' \
  'external_url_fetch_supported: `false`' \
  'repository_mutation_allowed: `false`' \
  'host_exec_allowed: `false`' \
  'llm_e2e_required_after_context_risk_change: `true`' \
  "### Context Budget Risk Card" \
  'kind=`context-budget` prompt_visible_context_bytes=' \
  "### Context File Risk Cards" \
  'kind=`context-file` path=`.gitclaw/SOUL.md`' \
  'kind=`context-file` path=`docs/search-fixture.md:1`' \
  "### Context Reference Risk Cards" \
  'ref_kind=`file` path=`docs/search-fixture.md` range=`1` count=`0` status=`ok`' \
  "### Selected Skill Risk Cards" \
  'kind=`selected-skill` path=`.gitclaw/SKILLS/repo-reader/SKILL.md`' \
  "### Tool Output Risk Cards" \
  'kind=`tool-output` name=`gitclaw.search_files`' \
  'kind=`tool-output` name=`gitclaw.read_file`' \
  'input_included=`false` output_body_included=`false`' \
  "### Runtime Boundary Risk Card" \
  'kind=`runtime-boundary` external_url_fetch_supported=`false`' \
  "### Risk Findings" \
  "- none"; do
  grep -Fq -- "$expected" <<<"$risk_comment" || die "context risk report missing ${expected}"
done

for leaked in \
  "$hidden_token" \
  "$expected_token" \
  "$search_phrase" \
  "Live context-risk E2E" \
  "Use the repo-reader skill"; do
  if grep -Fq "$leaked" <<<"$risk_comment"; then
    die "context risk report leaked ${leaked}"
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
