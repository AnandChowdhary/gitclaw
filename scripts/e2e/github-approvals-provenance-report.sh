#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "approvals-provenance-report-e2e: $*" >&2
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
ensure_label gitclaw:write-requested d93f0b "GitClaw detected a write request"
ensure_label gitclaw:approved 0e8a16 "Maintainer approved GitClaw write-mode work"
ensure_label gitclaw:needs-human b60205 "GitClaw needs human approval or authorization"
ensure_label "$retention_label" c2e0c6 "GitClaw E2E retention"

timestamp_with_run_filter_slack() {
  date -u -v-15S +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "15 seconds ago" +%Y-%m-%dT%H:%M:%SZ
}

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
hidden_token="GITCLAW_APPROVALS_PROVENANCE_HIDDEN_${timestamp}"
provenance_hidden_token="GITCLAW_APPROVALS_PROVENANCE_COMMENT_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_APPROVALS_PROVENANCE_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_APPROVAL_PROVENANCE_CONTEXT_V1"
search_phrase="approval provenance unique search fixture phrase"
title="@gitclaw approvals provenance seed e2e ${timestamp}"
body="Live approvals-provenance E2E.

Use the repo-reader skill and search the repository for \`${search_phrase}\`.
Reply with the full token immediately after \`=>\` in the matching line, including the \`_CONTEXT_V1\` suffix.
Do not abbreviate it to the GITCLAW_APPROVAL_PROVENANCE prefix.
Do not include this hidden issue token: ${hidden_token}
Keep the answer under 30 words."

local_report="$(go run ./cmd/gitclaw approvals provenance)"
for expected in \
  "GitClaw Approvals Provenance Report" \
  "Generated without a model call" \
  'scope: `local-cli`' \
  'current_issue_labels_available: `false`' \
  'comments_available: `false`' \
  'approval_provenance_status: `static_policy`' \
  'verification_scope: `current-issue-labels-transcript-and-assistant-markers`' \
  'approval_store: `github-issue-labels`' \
  'approval_scope: `per-issue`' \
  'label_source: `current-github-issue-labels`' \
  'write_request_source: `transcript-heuristic-or-label`' \
  'assistant_marker_source: `issue-comments`' \
  'write_actions_enabled: `false`' \
  'repository_mutation_allowed: `false`' \
  'host_exec_allowed: `false`' \
  'raw_bodies_included: `false`' \
  'raw_comments_included: `false`' \
  'raw_prompts_included: `false`' \
  'raw_approval_payloads_included: `false`' \
  'run_urls_included: `false`' \
  'llm_e2e_required_after_approval_provenance_change: `true`' \
  'source=`local-config`' \
  'role=`approved` label=`gitclaw:approved` present=`false`' \
  'code=`local_static_policy_only`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local approvals provenance report missing ${expected}"
done

issue_started_at="$(timestamp_with_run_filter_slack)"
issue_url="$(gh issue create \
  --repo "$repo" \
  --title "$title" \
  --body "$body" \
  --label gitclaw \
  --label gitclaw:approved)"
issue_number="${issue_url##*/}"

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled --add-label "$retention_label" >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "approvals-provenance-report e2e cleanup" >/dev/null 2>&1 || true
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
grep -Fq 'skills="repo-reader"' <<<"$seed_comment" || die "seed assistant marker missing repo-reader skill"
grep -Fq 'tools="' <<<"$seed_comment" || die "seed assistant marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$seed_comment" || die "seed assistant marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$seed_comment" || die "seed assistant marker missing normalized provider usage"

if grep -Fq "$hidden_token" <<<"$seed_comment"; then
  die "seed assistant leaked hidden issue token"
fi

provenance_started_at="$(timestamp_with_run_filter_slack)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw /approvals provenance

Please implement this change and open a PR, but the approvals provenance report must stay read-only and body-free.
Do not include this hidden provenance token: ${provenance_hidden_token}" >/dev/null

provenance_run_json="$(wait_for_run issue_comment "$provenance_started_at")" || die "timed out waiting for approvals provenance issue_comment workflow run"
wait_for_assistant_count 2 || die "expected approvals provenance report as second assistant comment"
provenance_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/approvals"' \
  "GitClaw Approvals Provenance Report" \
  "Generated without a model call" \
  'repository: `'"$repo"'`' \
  'issue: `#'"$issue_number"'`' \
  'event_kind: `issue_comment`' \
  'active_command: `/approvals provenance`' \
  'preflight_allowed: `true`' \
  'preflight_code: `allowed`' \
  'actor_association: `OWNER`' \
  'actor_trusted: `true`' \
  'triggered: `true`' \
  'current_issue_labels_available: `true`' \
  'write_request_detected: `true`' \
  'write_requested_label_present: `false`' \
  'write_request_evidence_present: `true`' \
  'approved_label_present: `true`' \
  'comments_available: `true`' \
  'issue_comments: `2`' \
  'transcript_messages: `3`' \
  'user_messages: `2`' \
  'assistant_messages: `1`' \
  'assistant_turn_markers: `1`' \
  'model_backed_assistant_turns: `1`' \
  'deterministic_assistant_turns: `0`' \
  'approval_provenance_status: `ok`' \
  'verification_scope: `current-issue-labels-transcript-and-assistant-markers`' \
  'approval_status: `approved_but_write_mode_disabled`' \
  'approval_decision: `proposal_only_approved_label_seen`' \
  'approval_store: `github-issue-labels`' \
  'approval_scope: `per-issue`' \
  'label_source: `current-github-issue-labels`' \
  'write_request_source: `transcript-heuristic-or-label`' \
  'actor_source: `github-event-author-association`' \
  'preflight_source: `github-event-plus-repo-config`' \
  'assistant_marker_source: `issue-comments`' \
  'write_actions_enabled: `false`' \
  'repository_mutation_allowed: `false`' \
  'host_exec_allowed: `false`' \
  'raw_bodies_included: `false`' \
  'raw_comments_included: `false`' \
  'raw_prompts_included: `false`' \
  'raw_approval_payloads_included: `false`' \
  'run_urls_included: `false`' \
  'llm_e2e_required_after_approval_provenance_change: `true`' \
  "### Provenance Chain" \
  'source=`assistant-markers` assistant_turn_markers=`1` model_backed=`1` deterministic=`0`' \
  "### Managed Label Evidence" \
  'role=`approved` label=`gitclaw:approved` present=`true`' \
  "### Assistant Marker Evidence" \
  'model=`openai/' \
  'deterministic=`false` has_prompt_evidence=`true`' \
  'run_url_sha256_12=' \
  "### Findings" \
  'code=`openclaw_exec_approval_state_separated`' \
  'code=`github_issue_label_approval_store`' \
  'code=`hermes_explicit_tool_boundary_mapped`' \
  'code=`read_only_runtime_boundary`'; do
  grep -Fq -- "$expected" <<<"$provenance_comment" || die "approvals provenance report missing ${expected}"
done

for leaked in "$hidden_token" "$provenance_hidden_token" "$expected_token" "$search_phrase" "Please implement this change and open a PR" "Reply with the exact GITCLAW_APPROVAL_PROVENANCE token"; do
  if grep -Fq "$leaked" <<<"$provenance_comment"; then
    die "approvals provenance report leaked ${leaked}"
  fi
done

labels="$(issue_label_names)"
grep -Fxq "gitclaw:write-requested" <<<"$labels" || die "write-requested label missing after approvals provenance report"
grep -Fxq "gitclaw:approved" <<<"$labels" || die "approved label missing after approvals provenance report"

followup_started_at="$(timestamp_with_run_filter_slack)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the full token immediately after \`=>\` in the matching line, including the \`_CONTEXT_V1\` suffix.
Do not abbreviate it to the GITCLAW_APPROVAL_PROVENANCE prefix.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

followup_run_json="$(wait_for_run issue_comment "$followup_started_at")" || die "timed out waiting for model follow-up issue_comment workflow run"
wait_for_assistant_count 3 || die "expected model-backed follow-up assistant comment"
followup_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$followup_comment" || die "model follow-up did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$followup_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$followup_comment"; then
  die "model follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$followup_comment" || die "model follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$followup_comment" || die "model follow-up marker missing repo-reader skill"
grep -Fq 'tools="' <<<"$followup_comment" || die "model follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$followup_comment" || die "model follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$followup_comment" || die "model follow-up marker missing normalized provider usage"

for leaked in "$hidden_token" "$provenance_hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$followup_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
seed_url="$(jq -r '.url' <<<"$seed_run_json")"
provenance_url="$(jq -r '.url' <<<"$provenance_run_json")"
followup_url="$(jq -r '.url' <<<"$followup_run_json")"
log "passed for issue #${issue_number}: ${seed_url} (provenance: ${provenance_url}, model follow-up: ${followup_url})"
