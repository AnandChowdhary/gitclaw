#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "commands-report-e2e: $*" >&2
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
token="GITCLAW_COMMANDS_REPORT_E2E_${timestamp}"
title="@gitclaw /help e2e ${timestamp}"
body="Live commands-report E2E.

Hidden commands report body token: ${token}
This should produce a deterministic command catalog report without a model call."

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
      gh issue close "$issue_number" --repo "$repo" --comment "commands-report e2e cleanup" >/dev/null 2>&1 || true
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
wait_for_assistant_count 1 || die "expected one commands report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/commands"' \
  "GitClaw Commands Report" \
  "Generated without a model call" \
  'trigger_prefix: `@gitclaw`' \
  'commands: `33`' \
  'aliases: `31`' \
  'local_cli_helpers: `116`' \
  'run_mode: `read-only`' \
  "### Slash Commands" \
  '/agents' \
  '/agent' \
  '/artifacts' \
  '/artifact' \
  '/approvals' \
  '/approval' \
  '/help' \
  '/commands' \
  '/backup' \
  '/bundles' \
  '/checkpoints' \
  '/checkpoint' \
  '/rollback' \
  '/diffs' \
  '/diff' \
  '/changes' \
  '/workspace' \
  '/workdir' \
  '/repo' \
  '/tools' \
  '/secrets' \
  '/secret' \
  '/migrate' \
  '/migration' \
  '/nodes' \
  '/node' \
  '/orders' \
  '/standing-orders' \
  '/heartbeat' \
  '/hooks' \
  '/hook' \
  '/plugins' \
  '/plugin' \
  '/doctor' \
  '/skills' \
  '/soul' \
  '/profile' \
  '/profiles' \
  '/tasks' \
  '/task' \
  '/runs' \
  '/run' \
  '/ledger' \
  '/sandbox' \
  '/sandboxes' \
  '/exec-policy' \
  '/budget' \
  '/prompt-budget' \
  '/cron' \
  'gitclaw agents list' \
  'gitclaw agents risk' \
  'gitclaw agents verify' \
  'gitclaw artifacts list' \
  'gitclaw artifacts risk' \
  'gitclaw artifacts verify' \
  'gitclaw nodes list' \
  'gitclaw nodes risk' \
  'gitclaw nodes verify' \
  'gitclaw approvals list' \
  'gitclaw approvals verify' \
  'gitclaw bundles list' \
  'gitclaw bundles info <name>' \
  'gitclaw channels verify' \
  'gitclaw channels risk' \
  'gitclaw channels list' \
  'gitclaw channels info <provider>' \
  'gitclaw channel-state' \
  'gitclaw channel-gateway' \
  'gitclaw channel-delivery' \
  'gitclaw checkpoints status' \
  'gitclaw checkpoints list' \
  'gitclaw checkpoints risk' \
  'gitclaw checkpoints verify' \
  'gitclaw rollback list' \
  'gitclaw rollback risk' \
  'gitclaw config list' \
  'gitclaw config risk' \
  'gitclaw context list' \
  'gitclaw context info <path>' \
  'gitclaw diffs summary' \
  'gitclaw diffs risk' \
  'gitclaw diffs verify' \
  'gitclaw doctor' \
  'gitclaw doctor list' \
  'gitclaw heartbeat status' \
  'gitclaw hooks list' \
  'gitclaw hooks risk' \
  'gitclaw hooks verify' \
  'gitclaw plugins list' \
  'gitclaw plugins risk' \
  'gitclaw plugins verify' \
  'gitclaw profile show' \
  'gitclaw profile verify' \
  'gitclaw profile risk' \
  'gitclaw tasks list' \
  'gitclaw tasks risk' \
  'gitclaw tasks verify' \
  'gitclaw runs current' \
  'gitclaw runs verify' \
  'gitclaw sandbox explain' \
  'gitclaw sandbox verify' \
  'gitclaw prompt list' \
  'gitclaw proactive list' \
  'gitclaw proactive risk' \
  'gitclaw proactive info <name>' \
  'gitclaw proactive init' \
  'gitclaw proactive enqueue' \
  'gitclaw session list --backup <issue.json>' \
  'gitclaw session risk --backup <issue.json>' \
  'gitclaw session search <query> --backup <issue.json>' \
  'gitclaw secrets audit' \
  'gitclaw secrets scan' \
  'gitclaw secrets list' \
  'gitclaw models list' \
  'gitclaw models risk' \
  'gitclaw orders list' \
  'gitclaw orders verify' \
  'gitclaw policy list' \
  'gitclaw policy verify' \
  'gitclaw backup verify' \
  'gitclaw backup risk' \
  'gitclaw backup manifest' \
  'gitclaw backup list' \
  'gitclaw backup info --issue <number>' \
  'gitclaw backup stats' \
  'gitclaw backup search <query>' \
  'gitclaw backup export-jsonl' \
  'gitclaw backup restore-plan' \
  'gitclaw backup retention-plan' \
  'gitclaw commands' \
  'gitclaw memory verify' \
  'gitclaw memory risk' \
  'gitclaw memory validate' \
  'gitclaw memory list' \
  'gitclaw memory promote-plan [target]' \
  'gitclaw memory info <path>' \
  'gitclaw memory search <query>' \
  'gitclaw migrate plan <source>' \
  'gitclaw soul verify' \
  'gitclaw soul risk' \
  'gitclaw soul validate' \
  'gitclaw soul list' \
  'gitclaw soul edit-plan <path>' \
  'gitclaw soul info <path>' \
  'gitclaw soul search <query>' \
  'gitclaw skills verify' \
  'gitclaw skills risk' \
  'gitclaw skills validate' \
  'gitclaw skills check' \
  'gitclaw skills list' \
  'gitclaw skills select-plan <name>' \
  'gitclaw skills install-plan <target>' \
  'gitclaw skills upgrade-plan <target>' \
  'gitclaw skills info <name>' \
  'gitclaw skills search <query>' \
  'gitclaw tools verify' \
  'gitclaw tools risk' \
  'gitclaw tools validate' \
  'gitclaw tools list' \
  'gitclaw tools run-plan <name>' \
  'gitclaw tools info <name>' \
  'gitclaw tools search <query>' \
  'gitclaw workspace summary' \
  'gitclaw workspace risk' \
  'gitclaw workspace verify'; do
  grep -Fq "$expected" <<<"$comments" || die "commands report missing ${expected}"
done

if grep -Fq "$token" <<<"$comments"; then
  die "commands report leaked issue body token"
fi

url="$(jq -r '.url' <<<"$run_json")"
log "passed for issue #${issue_number}: ${url}"
