#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "skills-snapshot-report-e2e: $*" >&2
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
hidden_token="NOECHO_SKILLS_SNAPSHOT_${timestamp}"
followup_hidden_token="NOECHO_SKILLS_SNAPSHOT_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_SKILLS_SNAPSHOT_CONTEXT_V1"
search_phrase="skills snapshot unique search fixture phrase"
title="@gitclaw /skills snapshot e2e ${timestamp}"
body="@gitclaw /skills snapshot

Live skills-snapshot E2E. Mention repo-reader so the report proves prompt-visible skill hashing.
Do not include this hidden skill snapshot token: ${hidden_token}"

local_report="$(go run ./cmd/gitclaw skills snapshot)"
for expected in \
  "GitClaw Skill Snapshot Report" \
  "Generated without a model call" \
  'scope: `local-cli`' \
  'skill_snapshot_status: `ok`' \
  'snapshot_version: `gitclaw-skill-snapshot-v1`' \
  'snapshot_scope: `repo-local-skills-bundles-sources`' \
  'snapshot_sha256_12:' \
  'snapshot_entries: `3`' \
  'skill_entries: `1`' \
  'selected_skill_entries: `0`' \
  'bundle_entries: `1`' \
  'source_pin_entries: `1`' \
  'prompt_visible_entries: `0`' \
  'available_skills: `1`' \
  'enabled_skills: `1`' \
  'disabled_skills: `0`' \
  'allowlist_blocked_skills: `0`' \
  'always_on_skills: `0`' \
  'skills_with_frontmatter: `1`' \
  'skills_with_description: `1`' \
  'skills_with_requirements: `0`' \
  'skills_missing_requirements: `0`' \
  'skill_bundles: `1`' \
  'selected_bundles: `0`' \
  'source_pins_scanned: `1`' \
  'hash_pinned_skill_sources: `1`' \
  'hash_matched_skill_sources: `1`' \
  'hash_mismatched_skill_sources: `0`' \
  'missing_skill_source_matches: `0`' \
  'remote_source_refs: `0`' \
  'remote_fetch_allowed_specs: `0`' \
  'registry_contact_allowed: `false`' \
  'remote_fetch_allowed: `false`' \
  'installer_scripts_run: `false`' \
  'dependency_install_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_skill_bodies_included: `false`' \
  'raw_skill_descriptions_included: `false`' \
  'raw_source_bodies_included: `false`' \
  'raw_source_refs_included: `false`' \
  'raw_bundle_instructions_included: `false`' \
  'llm_e2e_required_after_skill_snapshot_change: `true`' \
  'skill_validation_status: `ok`' \
  'skill_risk_status: `ok`' \
  '### Snapshot Entries' \
  'kind=`skill` name=`repo-reader` source=`repo-local-skill`' \
  'path=`.gitclaw/SKILLS/repo-reader/SKILL.md`' \
  'prompt_visible=`false`' \
  'selected_for_this_turn=`false`' \
  'description_present=`true`' \
  'description_sha256_12=' \
  'kind=`bundle` name=`repo-context` source=`repo-local-skill-bundle`' \
  'bundle_skill_refs=`repo-reader`' \
  'resolved_bundle_skills=`repo-reader`' \
  'instruction_present=`true`' \
  'instruction_sha256_12=' \
  'kind=`source-pin` name=`repo-reader` source=`repo-local-skill-source`' \
  'source_kind=`repo-local`' \
  'source_ref_present=`true`' \
  'source_ref_sha256_12=' \
  'hash_pinned=`true`' \
  'hash_matched=`true`' \
  'risk_findings=`0`' \
  'risk_codes=`none`' \
  '### Snapshot Gates' \
  'validation_gate=`pass`' \
  'risk_gate=`pass`' \
  'source_gate=`pass`' \
  'progressive_disclosure_gate=`enabled`' \
  'registry_gate=`disabled`' \
  'remote_fetch_gate=`disabled`' \
  'installer_gate=`disabled`' \
  'dependency_install_gate=`disabled`' \
  'mutation_gate=`disabled`' \
  'raw_body_gate=`hash_only`' \
  'snapshot_hash_gate=`composite-sha256_12`'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local skill snapshot report missing ${expected}"
done

for leaked in "$expected_token" "$search_phrase" "Use GitClaw's read-only repository context" "When a user asks about a repository file" "Prefer repository context and deterministic tool outputs"; do
  if grep -Fq "$leaked" <<<"$local_report"; then
    die "local skill snapshot report leaked ${leaked}"
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
      gh issue close "$issue_number" --repo "$repo" --comment "skills-snapshot-report e2e cleanup" >/dev/null 2>&1 || true
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

snapshot_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one skills snapshot report comment"
snapshot_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/skills"' \
  "GitClaw Skill Snapshot Report" \
  "Generated without a model call" \
  'skill_snapshot_status: `ok`' \
  'snapshot_version: `gitclaw-skill-snapshot-v1`' \
  'snapshot_scope: `repo-local-skills-bundles-sources`' \
  'snapshot_sha256_12:' \
  'snapshot_entries: `4`' \
  'skill_entries: `1`' \
  'selected_skill_entries: `1`' \
  'bundle_entries: `1`' \
  'source_pin_entries: `1`' \
  'prompt_visible_entries: `2`' \
  'available_skills: `1`' \
  'enabled_skills: `1`' \
  'disabled_skills: `0`' \
  'allowlist_blocked_skills: `0`' \
  'always_on_skills: `0`' \
  'skill_bundles: `1`' \
  'source_pins_scanned: `1`' \
  'hash_matched_skill_sources: `1`' \
  'raw_skill_bodies_included: `false`' \
  'raw_skill_descriptions_included: `false`' \
  'raw_source_refs_included: `false`' \
  'raw_bundle_instructions_included: `false`' \
  'llm_e2e_required_after_skill_snapshot_change: `true`' \
  'skill_validation_status: `ok`' \
  'skill_risk_status: `ok`' \
  'issue_title_sha256_12:' \
  'kind=`skill` name=`repo-reader`' \
  'prompt_visible=`true`' \
  'selected_for_this_turn=`true`' \
  'kind=`selected-skill` name=`.gitclaw/SKILLS/repo-reader/SKILL.md` source=`prompt-context`' \
  'kind=`bundle` name=`repo-context`' \
  'kind=`source-pin` name=`repo-reader`' \
  'source_ref_sha256_12=' \
  '### Snapshot Gates' \
  'validation_gate=`pass`' \
  'risk_gate=`pass`' \
  'source_gate=`pass`' \
  'snapshot_hash_gate=`composite-sha256_12`'; do
  grep -Fq -- "$expected" <<<"$snapshot_comment" || die "skills snapshot report missing ${expected}"
done

for leaked in "$hidden_token" "Live skills-snapshot E2E" "$expected_token" "$search_phrase" "GITCLAW_SKILL_CONTEXT_V1" "When a user asks about a repository file" "Prefer repository context and deterministic tool outputs"; do
  if grep -Fq "$leaked" <<<"$snapshot_comment"; then
    die "skills snapshot report leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_SKILLS_SNAPSHOT token from the matching repository search result line.
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

for leaked in "$hidden_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

wait_for_done_status || die "expected gitclaw:done without running/error"
snapshot_url="$(jq -r '.url' <<<"$snapshot_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${snapshot_url} (model follow-up: ${model_url})"
