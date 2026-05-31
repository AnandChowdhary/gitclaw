#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "skills-upgrade-plan-report-e2e: $*" >&2
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
hidden_token="GITCLAW_SKILLS_UPGRADE_PLAN_HIDDEN_${timestamp}"
followup_hidden_token="GITCLAW_SKILLS_UPGRADE_PLAN_FOLLOWUP_HIDDEN_${timestamp}"
expected_token="GITCLAW_SKILLS_UPGRADE_PLAN_CONTEXT_V1"
search_phrase="skills upgrade plan unique search fixture phrase"
title="@gitclaw /skills upgrade-plan repo-reader e2e ${timestamp}"
body="Live skills upgrade-plan E2E.

Hidden skills upgrade plan token: ${hidden_token}
This should produce a deterministic dry-run upgrade plan without fetching remote code, running installers, mutating skills, or dumping the full SKILL.md body."

local_report="$(go run ./cmd/gitclaw skills upgrade-plan repo-reader)"
for expected in \
  "GitClaw Skill Install Plan Report" \
  'scope: `local-cli`' \
  'install_plan_status: `needs_review`' \
  'operation: `upgrade-plan`' \
  'target_type: `registry-name`' \
  'safe_name_candidate: `repo-reader`' \
  'destination_path: `.gitclaw/SKILLS/repo-reader/SKILL.md`' \
  'destination_exists: `true`' \
  'existing_skill_matches: `1`' \
  'existing_skill_hashes:' \
  'upgrade_target_required: `true`' \
  'existing_skill_required: `true`' \
  'available_skills: `1`' \
  'run_mode: `read-only`' \
  'remote_fetch_allowed: `false`' \
  'installer_scripts_run: `false`' \
  'dependency_install_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'manual_review_required: `true`' \
  'llm_e2e_required_after_change: `true`' \
  'llm_e2e_required_after_skill_upgrade_plan_change: `true`' \
  'raw_target_included: `false`' \
  'raw_manifest_included: `false`' \
  'raw_skill_body_included: `false`' \
  'skill_validation_status: `ok`' \
  '### Existing Skill Matches' \
  'skill_name=`repo-reader`' \
  'path=`.gitclaw/SKILLS/repo-reader/SKILL.md`' \
  'selected_for_this_turn=`true`' \
  '### Review Steps' \
  'Run a live GitHub Models conversation E2E' \
  '### Findings' \
  'code=`manual_review_required`' \
  'code=`installer_scripts_disabled`' \
  'code=`repository_mutation_disabled`' \
  'code=`existing_skill_found`' \
  'target_sha256_12:'; do
  grep -Fq -- "$expected" <<<"$local_report" || die "local skills upgrade-plan report missing ${expected}"
done

for leaked in "$hidden_token" "GITCLAW_SKILL_CONTEXT_V1" "$expected_token" "$search_phrase" "When a user asks about a repository file"; do
  if grep -Fq "$leaked" <<<"$local_report"; then
    die "local skills upgrade-plan report leaked ${leaked}"
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
      gh issue close "$issue_number" --repo "$repo" --comment "skills-upgrade-plan-report e2e cleanup" >/dev/null 2>&1 || true
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

upgrade_run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one skills upgrade-plan report comment"
upgrade_comment="$(latest_assistant_comment)"

for expected in \
  'model="gitclaw/skills"' \
  "GitClaw Skill Install Plan Report" \
  "Generated without a model call" \
  'install_plan_status: `needs_review`' \
  'operation: `upgrade-plan`' \
  'target_type: `registry-name`' \
  'safe_name_candidate: `repo-reader`' \
  'destination_path: `.gitclaw/SKILLS/repo-reader/SKILL.md`' \
  'destination_exists: `true`' \
  'existing_skill_matches: `1`' \
  'existing_skill_hashes:' \
  'upgrade_target_required: `true`' \
  'existing_skill_required: `true`' \
  'available_skills: `1`' \
  'run_mode: `read-only`' \
  'remote_fetch_allowed: `false`' \
  'installer_scripts_run: `false`' \
  'dependency_install_allowed: `false`' \
  'repository_mutation_allowed: `false`' \
  'manual_review_required: `true`' \
  'llm_e2e_required_after_change: `true`' \
  'llm_e2e_required_after_skill_upgrade_plan_change: `true`' \
  'raw_target_included: `false`' \
  'raw_manifest_included: `false`' \
  'raw_skill_body_included: `false`' \
  'skill_validation_status: `ok`' \
  'issue_title_sha256_12:' \
  '### Existing Skill Matches' \
  'skill_name=`repo-reader`' \
  'path=`.gitclaw/SKILLS/repo-reader/SKILL.md`' \
  'selected_for_this_turn=`true`' \
  '### Review Steps' \
  'Run a live GitHub Models conversation E2E' \
  '### Findings' \
  'code=`manual_review_required`' \
  'code=`installer_scripts_disabled`' \
  'code=`repository_mutation_disabled`' \
  'code=`existing_skill_found`' \
  'target_sha256_12:'; do
  grep -Fq -- "$expected" <<<"$upgrade_comment" || die "skills upgrade-plan report missing ${expected}"
done

for leaked in "$hidden_token" "Live skills upgrade-plan E2E" "GITCLAW_SKILL_CONTEXT_V1" "When a user asks about a repository file" "$expected_token" "$search_phrase"; do
  if grep -Fq "$leaked" <<<"$upgrade_comment"; then
    die "skills upgrade-plan report leaked ${leaked}"
  fi
done

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Reply with only the exact GITCLAW_SKILLS_UPGRADE_PLAN token from the matching repository search result line.
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
upgrade_url="$(jq -r '.url' <<<"$upgrade_run_json")"
model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${upgrade_url} (model follow-up: ${model_url})"
