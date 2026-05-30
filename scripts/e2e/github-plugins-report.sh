#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "plugins-report-e2e: $*" >&2
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
token="GITCLAW_PLUGINS_REPORT_E2E_${timestamp}"
title="@gitclaw /plugins e2e ${timestamp}"
body="Live plugins-report E2E.

Hidden plugins report body token: ${token}
This should produce a deterministic plugins report without a model call."

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
      gh issue close "$issue_number" --repo "$repo" --comment "plugins-report e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one plugins report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/plugins"' \
  "GitClaw Plugins Report" \
  "Generated without a model call" \
  'plugins_status: `ok`' \
  'plugin_policy_path: `.gitclaw/PLUGINS.md`' \
  'plugin_policy_present: `true`' \
  'plugin_policy_loaded_for_model: `true`' \
  'plugin_specs_dir: `.gitclaw/plugins`' \
  'plugin_specs: `1`' \
  'plugin_specs_with_frontmatter: `1`' \
  'plugin_capabilities: `2`' \
  'plugin_optional_capabilities: `1`' \
  'plugin_specs_requiring_approval: `1`' \
  'plugin_specs_metadata_only: `1`' \
  'plugin_package_files_present: `0`' \
  'plugin_install_supported: `false`' \
  'plugin_execution_supported: `false`' \
  'plugin_execution_allowed: `false`' \
  'mcp_connection_allowed: `false`' \
  'model_call_required: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_bodies_included: `false`' \
  'raw_plugin_bodies_included: `false`' \
  'llm_e2e_required_after_change: `true`' \
  'name=`github-models-provider`' \
  'path=`.gitclaw/plugins/github-models-provider.md`' \
  'frontmatter=`true`' \
  'kind=`provider`' \
  'source=`repo-reviewed`' \
  'activation=`metadata-only`' \
  'capabilities=`2`' \
  'optional_capabilities=`1`' \
  'requires_approval=`true`' \
  'plugin packages are not installed' \
  '### Verification Findings' \
  '- none'; do
  grep -Fq -- "$expected" <<<"$comments" || die "plugins report missing ${expected}"
done

for forbidden in "$token" "GITCLAW_PLUGINS_CONTEXT_V1" "This declarative plugin record"; do
  if grep -Fq -- "$forbidden" <<<"$comments"; then
    die "plugins report leaked ${forbidden}"
  fi
done

url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
