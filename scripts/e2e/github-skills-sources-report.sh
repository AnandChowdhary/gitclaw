#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "skills-sources-report-e2e: $*" >&2
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
hidden_token="GITCLAW_SKILL_SOURCES_E2E_${timestamp}"
followup_hidden_token="GITCLAW_SKILL_SOURCES_FOLLOWUP_E2E_${timestamp}"
expected_token="GITCLAW_SEARCH_CONTEXT_V1"
expected_hash="2f9e68a57bd6"
search_phrase="bounded repository search fixture phrase"
title="@gitclaw /skills sources risk e2e ${timestamp}"
body="@gitclaw /skills sources risk

Live skills sources E2E.
Do not include this hidden skill source token: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw skills sources risk)"
for expected in \
  "GitClaw Skill Sources Risk Report" \
  'skill_source_status: `ok`' \
  'skill_source_specs_dir: `.gitclaw/skill-sources`' \
  'skill_source_specs: `1`' \
  'matched_skill_sources: `1`' \
  'hash_matched_skill_sources: `1`' \
  'skill_source_risk_findings: `0`' \
  'registry_contact_allowed: `false`' \
  'raw_skill_bodies_included: `false`' \
  'llm_e2e_required_after_skill_source_change: `true`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local skill sources risk report missing ${expected}"
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
      gh issue close "$issue_number" --repo "$repo" --comment "skills-sources-report e2e cleanup" >/dev/null 2>&1 || true
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

sources_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one skills sources report comment"
sources_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/skills"' \
  "GitClaw Skill Sources Risk Report" \
  "Generated without a model call" \
  'skill_source_status: `ok`' \
  'skill_source_specs_dir: `.gitclaw/skill-sources`' \
  'skill_source_specs: `1`' \
  'parsed_skill_source_specs: `1`' \
  'matched_skill_sources: `1`' \
  'missing_skill_source_matches: `0`' \
  'hash_pinned_skill_sources: `1`' \
  'hash_matched_skill_sources: `1`' \
  'hash_mismatched_skill_sources: `0`' \
  'repo_local_source_refs: `1`' \
  'remote_source_refs: `0`' \
  'sources_requiring_approval: `1`' \
  'remote_fetch_allowed_specs: `0`' \
  'sources_with_risk_findings: `0`' \
  'skill_source_risk_findings: `0`' \
  'high_risk_findings: `0`' \
  'warning_risk_findings: `0`' \
  'info_risk_findings: `0`' \
  'registry_contact_allowed: `false`' \
  'installer_scripts_run: `false`' \
  'dependency_install_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_source_bodies_included: `false`' \
  'raw_source_refs_included: `false`' \
  'raw_skill_bodies_included: `false`' \
  'llm_e2e_required_after_skill_source_change: `true`' \
  'source_name=`repo-reader`' \
  'path=`.gitclaw/skill-sources/repo-reader.yaml`' \
  'skill_path=`.gitclaw/SKILLS/repo-reader/SKILL.md`' \
  'skill_matched=`true`' \
  'source_kind=`repo-local`' \
  'source_ref_present=`true`' \
  'trust_level=`repo-local`' \
  'install_mode=`manual-review`' \
  'requires_approval=`true`' \
  'remote_fetch_allowed=`false`' \
  'hash_pinned=`true`' \
  "expected_sha256_12=\`${expected_hash}\`" \
  "current_skill_sha256_12=\`${expected_hash}\`" \
  'hash_matched=`true`' \
  'risk_findings=`0`' \
  'risk_codes=`none`' \
  "### Risk Findings" \
  "- none"; do
  grep -Fq -- "$expected" <<<"$sources_comment" || die "skills sources report missing ${expected}"
done

for leaked in "$hidden_token" "Live skills sources E2E" "GITCLAW_SKILL_CONTEXT_V1" "When a user asks about a repository file" "bounded repository search fixture phrase" "GITCLAW_SEARCH_CONTEXT_V1"; do
  if grep -Fq "$leaked" <<<"$sources_comment"; then
    die "skills sources report leaked ${leaked}"
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
sources_url="$(jq -r '.url' <<<"$sources_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${sources_url} (model follow-up: ${model_url})"
