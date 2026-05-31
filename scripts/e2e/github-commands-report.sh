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
token="NOECHO_COMMANDS_REPORT_${timestamp}"
followup_hidden_token="NOECHO_COMMANDS_REPORT_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_COMMANDS_REPORT_CONTEXT_V1"
search_phrase="commands report unique search fixture phrase"
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

assistant_comments() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | join("\n---GITCLAW-COMMENT---\n")'
}

latest_assistant_comment() {
  gh issue view "$issue_number" \
    --repo "$repo" \
    --json comments \
    --jq '[.comments[] | select(.body | contains("gitclaw:assistant-turn")) | .body] | .[-1] // ""'
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

run_json="$(wait_for_run issues "$issue_started_at")" || die "timed out waiting for issues workflow run"
wait_for_assistant_count 1 || die "expected one commands report comment"
comments="$(assistant_comments)"

for expected in \
  'model="gitclaw/commands"' \
  "GitClaw Commands Report" \
  "Generated without a model call" \
  'trigger_prefix: `@gitclaw`' \
  'commands: `33`' \
  'aliases: `31`' \
  'local_cli_helpers: `193`' \
  'run_mode: `read-only`' \
  'llm_e2e_required_after_commands_report_change: `true`' \
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
  'gitclaw agents catalog' \
  'gitclaw agents list' \
  'gitclaw agents provenance' \
  'gitclaw agents risk' \
  'gitclaw agents verify' \
  'gitclaw artifacts catalog' \
  'gitclaw artifacts list' \
  'gitclaw artifacts risk' \
  'gitclaw artifacts verify' \
  'gitclaw nodes catalog' \
  'gitclaw nodes list' \
  'gitclaw nodes risk' \
  'gitclaw nodes verify' \
  'gitclaw approvals catalog' \
  'gitclaw approvals list' \
  'gitclaw approvals verify' \
  'gitclaw approvals provenance' \
  'gitclaw approvals risk' \
  'gitclaw bundles catalog' \
  'gitclaw bundles list' \
  'gitclaw bundles risk' \
  'gitclaw bundles provenance' \
  'gitclaw bundles info <name>' \
  'gitclaw bundles search <query>' \
  'gitclaw channels verify' \
  'gitclaw channels risk' \
  'gitclaw channels list' \
  'gitclaw channels info <provider>' \
  'gitclaw channel-state' \
  'gitclaw channel-gateway' \
  'gitclaw channel-delivery' \
  'gitclaw checkpoints catalog' \
  'gitclaw checkpoints status' \
  'gitclaw checkpoints list' \
  'gitclaw checkpoints risk' \
  'gitclaw checkpoints verify' \
  'gitclaw rollback catalog' \
  'gitclaw rollback list' \
  'gitclaw rollback risk' \
  'gitclaw config list' \
  'gitclaw config risk' \
  'gitclaw context list' \
  'gitclaw context risk' \
  'gitclaw context info <path>' \
  'gitclaw diffs summary' \
  'gitclaw diffs risk' \
  'gitclaw diffs verify' \
  'gitclaw doctor' \
  'gitclaw doctor list' \
  'gitclaw heartbeat status' \
  'gitclaw heartbeat risk' \
  'gitclaw hooks catalog' \
  'gitclaw hooks list' \
  'gitclaw hooks risk' \
  'gitclaw hooks verify' \
  'gitclaw hooks provenance' \
  'gitclaw plugins list' \
  'gitclaw plugins risk' \
  'gitclaw plugins verify' \
  'gitclaw plugins mcp' \
  'gitclaw plugins mcp risk' \
  'gitclaw plugins mcp provenance' \
  'gitclaw plugins mcp info <name>' \
  'gitclaw profile catalog' \
  'gitclaw profile show' \
  'gitclaw profile verify' \
  'gitclaw profile manifest' \
  'gitclaw profile export-plan' \
  'gitclaw profile risk' \
  'gitclaw tasks list' \
  'gitclaw tasks risk' \
  'gitclaw tasks verify' \
  'gitclaw tasks ledger --backup <issue.json>' \
  'gitclaw runs current' \
  'gitclaw runs verify' \
  'gitclaw runs history --backup <issue.json>' \
  'gitclaw sandbox explain' \
  'gitclaw sandbox verify' \
  'gitclaw sandbox risk' \
  'gitclaw prompt list' \
  'gitclaw prompt pack' \
  'gitclaw prompt cache' \
  'gitclaw prompt compression' \
  'gitclaw prompt risk' \
  'gitclaw proactive list' \
  'gitclaw proactive risk' \
  'gitclaw proactive info <name>' \
  'gitclaw proactive init' \
  'gitclaw proactive enqueue' \
  'gitclaw session catalog' \
  'gitclaw session list --backup <issue.json>' \
  'gitclaw session provenance --backup <issue.json>' \
  'gitclaw session status --backup <issue.json>' \
  'gitclaw session stats --backup <issue.json>' \
  'gitclaw session coverage --backup <issue.json>' \
  'gitclaw session risk --backup <issue.json>' \
  'gitclaw session search <query> --backup <issue.json>' \
  'gitclaw secrets audit' \
  'gitclaw secrets scan' \
  'gitclaw secrets list' \
  'gitclaw secrets risk' \
  'gitclaw models list' \
  'gitclaw models usage' \
  'gitclaw models cost' \
  'gitclaw models risk' \
  'gitclaw orders list' \
  'gitclaw orders verify' \
  'gitclaw orders risk' \
  'gitclaw policy list' \
  'gitclaw policy verify' \
  'gitclaw policy risk' \
  'gitclaw backup catalog' \
  'gitclaw backup verify' \
  'gitclaw backup coverage --issue <number>' \
  'gitclaw backup drill --issue <number>' \
  'gitclaw backup risk' \
  'gitclaw backup provenance' \
  'gitclaw backup manifest' \
  'gitclaw backup list' \
  'gitclaw backup timeline' \
  'gitclaw backup info --issue <number>' \
  'gitclaw backup stats' \
  'gitclaw backup freshness' \
  'gitclaw backup continuity' \
  'gitclaw backup search <query>' \
  'gitclaw backup export-jsonl' \
  'gitclaw backup restore-plan' \
  'gitclaw backup retention-plan' \
  'gitclaw commands' \
  'gitclaw bundles catalog' \
  'gitclaw memory catalog' \
  'gitclaw memory provenance' \
  'gitclaw memory verify' \
  'gitclaw memory risk' \
  'gitclaw memory validate' \
  'gitclaw memory timeline' \
  'gitclaw memory list' \
  'gitclaw memory promote-plan [target]' \
  'gitclaw memory info <path>' \
  'gitclaw memory search <query>' \
  'gitclaw migrate plan <source>' \
  'gitclaw migrate risk <source>' \
  'gitclaw soul catalog' \
  'gitclaw soul anchors' \
  'gitclaw soul provenance' \
  'gitclaw soul verify' \
  'gitclaw soul risk' \
  'gitclaw soul validate' \
  'gitclaw soul list' \
  'gitclaw soul edit-plan <path>' \
  'gitclaw soul info <path>' \
  'gitclaw soul search <query>' \
  'gitclaw skills verify' \
  'gitclaw skills risk' \
  'gitclaw skills runtime' \
  'gitclaw skills catalog' \
  'gitclaw skills validate' \
  'gitclaw skills check' \
  'gitclaw skills list' \
  'gitclaw skills provenance' \
  'gitclaw skills select-plan <name>' \
  'gitclaw skills refresh-plan' \
  'gitclaw skills sources' \
  'gitclaw skills sources provenance' \
  'gitclaw skills sources risk' \
  'gitclaw skills sources info <name>' \
  'gitclaw skills proposals [risk]' \
  'gitclaw skills proposal-plan <name>' \
  'gitclaw skills install-plan <target>' \
  'gitclaw skills upgrade-plan <target>' \
  'gitclaw skills info <name>' \
  'gitclaw skills search <query>' \
  'gitclaw tools catalog' \
  'gitclaw tools verify' \
  'gitclaw tools risk' \
  'gitclaw tools validate' \
  'gitclaw tools list' \
  'gitclaw tools exposure' \
  'gitclaw tools exposure risk' \
  'gitclaw tools defer-plan' \
  'gitclaw tools boundary [query]' \
  'gitclaw tools provenance [query]' \
  'gitclaw tools toolsets' \
  'gitclaw tools toolsets risk' \
  'gitclaw tools toolsets provenance' \
  'gitclaw tools toolsets info <name>' \
  'gitclaw tools approval-plan <name>' \
  'gitclaw tools run-plan <name>' \
  'gitclaw tools info <name>' \
  'gitclaw tools search <query>' \
  'gitclaw workspace catalog' \
  'gitclaw workspace summary' \
  'gitclaw workspace risk' \
  'gitclaw workspace verify'; do
  grep -Fq "$expected" <<<"$comments" || die "commands report missing ${expected}"
done

for leaked in \
  "$token" \
  "Hidden commands report body token" \
  "This should produce a deterministic command catalog report"; do
  if grep -Fq "$leaked" <<<"$comments"; then
    die "commands report leaked ${leaked}"
  fi
done

url="$(jq -r '.url' <<<"$run_json")"
log "commands report verified for issue #${issue_number}: ${url}"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Use the repo-reader skill and search the repository for \`${search_phrase}\`.

Search for that exact phrase, not shorter words from it.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${search_phrase} => <token>\`.
Reply with only the uppercase fixture token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with the command catalog nonce, issue title, issue number, slash command, helper name, or any token from this issue/comments.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_run issue_comment "$comment_started_at")" || die "timed out waiting for model follow-up issue_comment workflow run"
wait_for_assistant_count 2 || die "expected model-backed follow-up assistant comment"
model_comment="$(latest_assistant_comment)"

grep -Fq "$expected_token" <<<"$model_comment" || die "model follow-up did not include search_files token ${expected_token}"
if ! grep -Fq 'model="openai/gpt-5-nano"' <<<"$model_comment" && ! grep -Fq 'model="openai/gpt-4.1-nano"' <<<"$model_comment"; then
  die "model follow-up marker did not use configured GitHub Models primary or fallback"
fi
grep -Fq 'prompt_context_sha256_12="' <<<"$model_comment" || die "model follow-up marker missing prompt context hash"
grep -Fq 'skills="repo-reader"' <<<"$model_comment" || die "model follow-up marker missing selected repo-reader skill"
grep -Fq 'tools="' <<<"$model_comment" || die "model follow-up marker missing prompt-visible tools"
grep -Fq 'gitclaw.search_files' <<<"$model_comment" || die "model follow-up marker did not prove search_files was prompt-visible"
grep -Fq 'usage_total_tokens="' <<<"$model_comment" || die "model follow-up marker missing usage token telemetry"

for leaked in "$token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number}: ${url} (model follow-up: ${model_url})"
