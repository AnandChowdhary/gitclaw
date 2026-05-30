#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "nodes-report-e2e: $*" >&2
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
token="GITCLAW_NODES_REPORT_E2E_${timestamp}"
title="@gitclaw /nodes e2e ${timestamp}"
body="Live nodes-report E2E.

Hidden nodes report body token: ${token}
This should produce a deterministic nodes report without a model call."

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
      gh issue close "$issue_number" --repo "$repo" --comment "nodes-report e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one nodes report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/nodes"' \
  "GitClaw Nodes Report" \
  "Generated without a model call" \
  'nodes_status: `ok`' \
  'node_policy_path: `.gitclaw/NODES.md`' \
  'node_policy_present: `true`' \
  'node_policy_loaded_for_model: `true`' \
  'node_specs_dir: `.gitclaw/nodes`' \
  'node_specs: `1`' \
  'node_specs_with_frontmatter: `1`' \
  'node_roles: `1`' \
  'node_capabilities_declared: `3`' \
  'node_specs_requiring_approval: `1`' \
  'node_specs_ephemeral_jobs: `1`' \
  'active_node_runtime: `github-actions-ephemeral-job`' \
  'node_inventory_source: `git-reviewed-metadata`' \
  'gateway_websocket_required: `false`' \
  'headless_node_host_supported: `false`' \
  'node_pairing_supported: `false`' \
  'node_rpc_supported: `false`' \
  'node_command_invocation_supported: `false`' \
  'remote_node_exec_supported: `false`' \
  'browser_proxy_supported: `false`' \
  'media_device_capabilities_supported: `false`' \
  'long_running_node_service_supported: `false`' \
  'model_call_required: `false`' \
  'repository_mutation_allowed: `false`' \
  'raw_bodies_included: `false`' \
  'raw_node_bodies_included: `false`' \
  'llm_e2e_required_after_change: `true`' \
  'name=`github-actions-runner`' \
  'path=`.gitclaw/nodes/github-actions-runner.md`' \
  'frontmatter=`true`' \
  'role=`primary-runtime`' \
  'runtime=`github-actions`' \
  'mode=`ephemeral-job`' \
  'capabilities=`3`' \
  'requires_approval=`true`' \
  'GitHub Actions jobs are the only active execution nodes in v1' \
  'future remote-node execution requires reviewed workflows' \
  '### Verification Findings' \
  '- none'; do
  grep -Fq -- "$expected" <<<"$comments" || die "nodes report missing ${expected}"
done

for forbidden in "$token" "GITCLAW_NODES_CONTEXT_V1" "This declarative node record"; do
  if grep -Fq -- "$forbidden" <<<"$comments"; then
    die "nodes report leaked ${forbidden}"
  fi
done

url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
