#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "artifacts-report-e2e: $*" >&2
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
token="GITCLAW_ARTIFACTS_REPORT_E2E_${timestamp}"
title="@gitclaw /artifacts e2e ${timestamp}"
body="Live artifacts-report E2E.

Hidden artifacts report body token: ${token}
This should produce a deterministic artifacts report without a model call."

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
      gh issue close "$issue_number" --repo "$repo" --comment "artifacts-report e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
}
trap cleanup EXIT

log "created issue #${issue_number}: ${issue_url}"

wait_for_run() {
  local started_at="$1"
  for _ in {1..90}; do
    local run_json
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$workflow_name" \
      --event issues \
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
        [[ "$conclusion" == "success" ]] || die "issues run failed with conclusion ${conclusion}: ${url}"
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

run_json="$(wait_for_run "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one artifacts report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/artifacts"' \
  "GitClaw Artifacts Report" \
  "Generated without a model call" \
  'artifacts_status: `ok`' \
  'artifact_policy_path: `.gitclaw/ARTIFACTS.md`' \
  'artifact_policy_present: `true`' \
  'artifact_policy_loaded_for_model: `true`' \
  'artifact_specs_dir: `.gitclaw/artifacts`' \
  'artifact_specs: `1`' \
  'artifact_specs_with_frontmatter: `1`' \
  'artifact_specs_requiring_approval: `1`' \
  'artifact_specs_requiring_redaction: `1`' \
  'artifact_retention_days_declared: `7`' \
  'github_actions_artifact_uploaders: `1`' \
  'upload_artifact_versions: `actions/upload-artifact@v6`' \
  'prompt_artifact_default_enabled: `false`' \
  'prompt_artifact_label: `gitclaw:e2e-prompt-artifact`' \
  'prompt_artifact_env_path_configured: `false`' \
  'artifact_storage_backend: `github-actions-artifacts`' \
  'durable_backup_backend: `git-backup-branch`' \
  'artifact_body_printing_allowed: `false`' \
  'model_call_required: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_bodies_included: `false`' \
  'raw_artifact_bodies_included: `false`' \
  'llm_e2e_required_after_change: `true`' \
  'name=`prompt-artifact`' \
  'path=`.gitclaw/artifacts/prompt-artifact.md`' \
  'frontmatter=`true`' \
  'kind=`prompt`' \
  'storage=`github-actions-artifact`' \
  'filename=`prompt.md`' \
  'workflow=`.github/workflows/gitclaw.yml`' \
  'label=`gitclaw:e2e-prompt-artifact`' \
  'retention_days=`7`' \
  'redaction_required=`true`' \
  'requires_approval=`true`' \
  'path=`.github/workflows/gitclaw.yml`' \
  'upload_artifact_actions=`actions/upload-artifact@v6`' \
  'if_no_files_found_error=`true`' \
  'prompt_artifact_label_gate=`true`' \
  'prompt_artifact_path_env=`true`' \
  'GitHub Actions artifacts are short-lived evidence bundles' \
  'future artifact types require body-free audit cards' \
  '### Verification Findings' \
  '- none'; do
  grep -Fq -- "$expected" <<<"$comments" || die "artifacts report missing ${expected}"
done

for forbidden in "$token" "GITCLAW_ARTIFACTS_CONTEXT_V1" "This declarative artifact record"; do
  if grep -Fq -- "$forbidden" <<<"$comments"; then
    die "artifacts report leaked ${forbidden}"
  fi
done

url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
