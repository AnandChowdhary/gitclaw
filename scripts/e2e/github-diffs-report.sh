#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "diffs-report-e2e: $*" >&2
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
token="GITCLAW_DIFFS_REPORT_E2E_${timestamp}"
title="@gitclaw /diffs e2e ${timestamp}"
body="Live diffs-report E2E.

Hidden diffs report body token: ${token}
This should produce a deterministic diffs report without a model call."

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
      gh issue close "$issue_number" --repo "$repo" --comment "diffs-report e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one diffs report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/diffs"' \
  "GitClaw Diffs Report" \
  "Generated without a model call" \
  'diff_status: `clean`' \
  'diff_policy_path: `.gitclaw/DIFFS.md`' \
  'diff_policy_present: `true`' \
  'diff_policy_loaded_for_model: `true`' \
  'diff_specs_dir: `.gitclaw/diffs`' \
  'diff_specs: `1`' \
  'diff_specs_with_frontmatter: `1`' \
  'diff_specs_requiring_approval: `1`' \
  'diff_specs_disallowing_raw_patch: `1`' \
  'git_available: `true`' \
  'git_repository: `true`' \
  'worktree_root: `.`' \
  'worktree_clean: `true`' \
  'changed_files: `0`' \
  'staged_files: `0`' \
  'unstaged_files: `0`' \
  'untracked_files: `0`' \
  'renamed_files: `0`' \
  'deleted_files: `0`' \
  'staged_insertions: `0`' \
  'staged_deletions: `0`' \
  'unstaged_insertions: `0`' \
  'unstaged_deletions: `0`' \
  'binary_diff_files: `0`' \
  'diff_file_limit: `200`' \
  'diff_files_returned: `0`' \
  'raw_diffs_included: `false`' \
  'raw_file_bodies_included: `false`' \
  'model_call_required: `false`' \
  'repository_mutation_allowed: `false`' \
  'llm_e2e_required_after_change: `true`' \
  'name=`working-tree`' \
  'path=`.gitclaw/diffs/working-tree.md`' \
  'frontmatter=`true`' \
  'kind=`git-diff`' \
  'source=`git-worktree`' \
  'mode=`metadata-only`' \
  'max_files=`200`' \
  'raw_patch_allowed=`false`' \
  'requires_approval=`true`' \
  '### Changed Files' \
  '- none' \
  '`/diffs` is inspect-only' \
  'future diff rendering needs reviewed workflows' \
  '### Verification Findings'; do
  grep -Fq -- "$expected" <<<"$comments" || die "diffs report missing ${expected}"
done

for forbidden in "$token" "GITCLAW_DIFFS_CONTEXT_V1" "This declarative diff record"; do
  if grep -Fq -- "$forbidden" <<<"$comments"; then
    die "diffs report leaked ${forbidden}"
  fi
done

url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
