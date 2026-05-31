#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "proactive-not-before-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
workflow="${GITCLAW_E2E_PROACTIVE_WORKFLOW:-.github/workflows/gitclaw-proactive.yml}"
main_workflow="${GITCLAW_E2E_WORKFLOW:-.github/workflows/gitclaw.yml}"
lock_dir="/tmp/gitclaw-proactive-not-before-e2e.lock"
cleanup_success=0

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another proactive not-before E2E appears to be running: ${lock_dir}"
fi

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e >/dev/null 2>&1 || true
    if [[ "$cleanup_success" == "1" ]]; then
      gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled >/dev/null 2>&1 || true
      gh issue close "$issue_number" --repo "$repo" --comment "proactive not-before e2e cleanup" >/dev/null 2>&1 || true
    else
      log "leaving issue #${issue_number} open for inspection after unsuccessful run"
    fi
  fi
  rm -rf "$lock_dir"
}
trap cleanup EXIT

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
ensure_label gitclaw:proactive fbca04 "GitClaw proactive run"
ensure_label gitclaw:running fbca04 "GitClaw run is active"
ensure_label gitclaw:done 0e8a16 "Latest GitClaw run completed"
ensure_label gitclaw:error b60205 "Latest GitClaw run failed"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
normalized_timestamp="$(printf "%s" "$timestamp" | tr '[:upper:]' '[:lower:]')"
name="proactive-not-before-e2e-${normalized_timestamp}"
future_slot="future-${timestamp}"
due_slot="due-${timestamp}"
future_token="NOECHO_PROACTIVE_NOT_BEFORE_FUTURE_${timestamp}"
due_token="NOECHO_PROACTIVE_NOT_BEFORE_DUE_${timestamp}"
followup_hidden_token="NOECHO_PROACTIVE_NOT_BEFORE_FOLLOWUP_${timestamp}"
expected_token="GITCLAW_PROACTIVE_NOT_BEFORE_CONTEXT_V1"
search_phrase="proactive not-before unique search fixture phrase"

run_list_json() {
  gh run list \
    --repo "$repo" \
    --workflow "$workflow" \
    --event workflow_dispatch \
    --limit 10 \
    --json databaseId,status,conclusion,createdAt,url \
    --jq '.'
}

wait_for_run() {
  local started_at="$1"
  local run_json
  for _ in {1..120}; do
    run_json="$(run_list_json | jq -c --arg started "$started_at" '[.[] | select(.createdAt >= $started)] | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "${workflow} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

run_log() {
  local run_id="$1"
  gh run view "$run_id" --repo "$repo" --log
}

wait_for_issue_comment_run() {
  local started_at="$1"
  local run_json
  for _ in {1..90}; do
    run_json="$(gh run list \
      --repo "$repo" \
      --workflow "$main_workflow" \
      --event issue_comment \
      --created ">=$started_at" \
      --limit 10 \
      --json databaseId,status,conclusion,createdAt,url,displayTitle \
      --jq '. as $runs | $runs | map(select(.displayTitle == "'"${issue_title}"'")) | sort_by(.createdAt) | reverse | .[0] // empty')"
    if [[ -n "$run_json" && "$run_json" != "null" ]]; then
      local status conclusion url
      status="$(jq -r '.status' <<<"$run_json")"
      conclusion="$(jq -r '.conclusion // ""' <<<"$run_json")"
      url="$(jq -r '.url' <<<"$run_json")"
      if [[ "$status" == "completed" ]]; then
        [[ "$conclusion" == "success" ]] || die "issue_comment run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

find_issue_numbers() {
  local slot="$1"
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:proactive \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg name "$name" --arg slot "$slot" '.[] | select((.title | contains($name)) and (.title | contains($slot)) or ((.body | contains($name)) and (.body | contains($slot)))) | .number'
}

wait_for_issue_number() {
  local slot="$1"
  for _ in {1..30}; do
    local numbers
    numbers="$(find_issue_numbers "$slot")"
    if [[ -n "$numbers" ]]; then
      echo "$numbers" | head -n 1
      return 0
    fi
    sleep 2
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
  for _ in {1..120}; do
    local got
    got="$(assistant_count 2>/dev/null || true)"
    if [[ "$got" =~ ^[0-9]+$ && "$got" == "$want" ]]; then
      return 0
    fi
    local errors
    errors="$(error_count 2>/dev/null || true)"
    if [[ "$errors" =~ ^[0-9]+$ && "$errors" != "0" ]]; then
      die "assistant run posted ${errors} error comment(s)"
    fi
    sleep 5
  done
  return 1
}

future_prompt="Proactive not-before future E2E instruction.

@gitclaw /proactive

Hidden future prompt token: ${future_token}
This issue must not be created before the due gate."

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow" \
  --repo "$repo" \
  -f name="$name" \
  -f slot="$future_slot" \
  -f prompt="$future_prompt" \
  -f not_before="2099-01-01T00:00:00Z"
future_run_json="$(wait_for_run "$started_at")" || die "timed out waiting for future not-before proactive workflow"
future_run_id="$(jq -r '.databaseId' <<<"$future_run_json")"
future_log="$(run_log "$future_run_id")"
for expected in \
  "proactive_enqueue issue=0" \
  "name=${name}" \
  "slot=${future_slot}" \
  "created=false" \
  "due=false" \
  "skipped=true" \
  "not_before=2099-01-01T00:00:00Z" \
  "llm_e2e_required_after_proactive_not_before_change=true"; do
  grep -Fq "$expected" <<<"$future_log" || die "future not-before workflow log missing ${expected}"
done

sleep 5
if [[ -n "$(find_issue_numbers "$future_slot")" ]]; then
  die "future not-before gate unexpectedly created an issue"
fi
log "future not-before gate skipped issue creation"

due_prompt="Proactive not-before due E2E instruction.

@gitclaw /proactive

Hidden due prompt token: ${due_token}
The deterministic proactive report must not leak this token."

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$workflow" \
  --repo "$repo" \
  -f name="$name" \
  -f slot="$due_slot" \
  -f prompt="$due_prompt" \
  -f not_before="2000-01-01T00:00:00Z"
due_run_json="$(wait_for_run "$started_at")" || die "timed out waiting for due proactive workflow"
due_run_id="$(jq -r '.databaseId' <<<"$due_run_json")"
due_log="$(run_log "$due_run_id")"
for expected in \
  "proactive_enqueue issue=" \
  "name=${name}" \
  "slot=${due_slot}" \
  "created=true" \
  "due=true" \
  "skipped=false" \
  "not_before=2000-01-01T00:00:00Z" \
  "llm_e2e_required_after_proactive_not_before_change=true"; do
  grep -Fq "$expected" <<<"$due_log" || die "due not-before workflow log missing ${expected}"
done

issue_number="$(wait_for_issue_number "$due_slot")" || die "timed out finding due proactive issue for ${name}/${due_slot}"
log "due proactive workflow created issue #${issue_number}"
issue_title="GitClaw proactive ${name} ${due_slot}"
wait_for_assistant_count 1 || die "timed out waiting for due proactive assistant response"

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,labels,comments)"
grep -Fq "gitclaw:proactive-run" <<<"$(jq -r '.body' <<<"$issue_json")" || die "issue body missing proactive marker"
grep -Fq "$due_token" <<<"$(jq -r '.body' <<<"$issue_json")" || die "issue body missing due prompt token"

comments="$(assistant_comments)"
grep -Fq 'model="gitclaw/proactive"' <<<"$comments" || die "assistant marker missing proactive report model"
grep -Fq "GitClaw Proactive Report" <<<"$comments" || die "assistant comment missing proactive report"
grep -Fq 'proactive_run_issue: `true`' <<<"$comments" || die "assistant comment did not detect proactive issue"
grep -Fq 'llm_e2e_required_after_proactive_report_change: `true`' <<<"$comments" || die "assistant proactive report missing live E2E marker"
if grep -Fq "$due_token" <<<"$comments"; then
  die "assistant proactive report leaked due prompt token ${due_token}"
fi
if grep -Fq "$future_token" <<<"$comments"; then
  die "assistant proactive report leaked future prompt token ${future_token}"
fi
if grep -Fq "$expected_token" <<<"$comments" || grep -Fq "$search_phrase" <<<"$comments"; then
  die "assistant proactive report leaked follow-up fixture context"
fi
grep -Fq "dispatch-proactive-${name}-${due_slot}" <<<"$comments" || die "assistant marker missing proactive dispatch event id"

labels="$(jq -r '.labels[].name' <<<"$issue_json")"
grep -Fxq "gitclaw:proactive" <<<"$labels" || die "issue missing gitclaw:proactive label"
grep -Fxq "gitclaw:done" <<<"$labels" || die "issue missing gitclaw:done label"

comment_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh issue comment "$issue_number" \
  --repo "$repo" \
  --body "@gitclaw Continue after the proactive not-before due gate and use the repo-reader skill.

Search the repository for \`${search_phrase}\`.
Search for that exact phrase, not shorter words from it.
The matching repository search result line in \`docs/search-fixture.md\` has the form \`${search_phrase} => <token>\`.
Reply with only the uppercase fixture token after the arrow from the matching gitclaw.search_files tool output line.
Do not answer with the proactive job name, slot, dispatch id, issue title, or any token from this issue body/comments.
Do not include this hidden follow-up token: ${followup_hidden_token}
Keep the answer under 30 words." >/dev/null

model_run_json="$(wait_for_issue_comment_run "$comment_started_at")" || die "timed out waiting for proactive not-before issue_comment follow-up"
wait_for_assistant_count 2 || die "expected model-backed proactive not-before follow-up assistant comment"
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

for leaked in "$future_token" "$due_token" "$followup_hidden_token"; do
  if grep -Fq "$leaked" <<<"$model_comment"; then
    die "model follow-up leaked ${leaked}"
  fi
done

model_url="$(jq -r '.url' <<<"$model_run_json")"
log "passed for issue #${issue_number} (future run: ${future_run_id}; due run: ${due_run_id}; model follow-up: ${model_url})"
cleanup_success=1
