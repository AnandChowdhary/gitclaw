#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "proactive-init-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
proactive_workflow="${GITCLAW_E2E_PROACTIVE_WORKFLOW:-.github/workflows/gitclaw-proactive.yml}"
lock_dir="/tmp/gitclaw-proactive-init-e2e.lock"
tmp_dir="$(mktemp -d)"
cleanup_success=0

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another proactive-init E2E appears to be running: ${lock_dir}"
fi

cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e >/dev/null 2>&1 || true
    if [[ "$cleanup_success" == "1" ]]; then
      gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:disabled >/dev/null 2>&1 || true
      gh issue close "$issue_number" --repo "$repo" --comment "proactive-init e2e cleanup" >/dev/null 2>&1 || true
    else
      log "leaving issue #${issue_number} open for inspection after unsuccessful run"
    fi
  fi
  rm -rf "$tmp_dir" "$lock_dir"
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
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
normalized_timestamp="$(printf "%s" "$timestamp" | tr '[:upper:]' '[:lower:]')"
name="proactive-init-e2e-${normalized_timestamp}"
slot="slot-${timestamp}"
dispatch_id="proactive-${name}-${slot}"
token="GITCLAW_PROACTIVE_INIT_E2E_${timestamp}"
prompt_body="Proactive init E2E instruction.

Reply with exact token \`${token}\`.
Also include the exact word \`proactive\`."
prompt_path=".gitclaw/proactive/${name}.md"
workflow_path=".github/workflows/gitclaw-proactive-${name}.yml"

init_output="$(go run ./cmd/gitclaw proactive init \
  --root "$tmp_dir" \
  --name "$name" \
  --cron "17 8 * * 1" \
  --prompt-body "$prompt_body")"

for expected in \
  'GitClaw Proactive Init Report' \
  'mode: `apply`' \
  "name: \`${name}\`" \
  "prompt_file: \`${prompt_path}\`" \
  "workflow_file: \`${workflow_path}\`" \
  'prompt_written: `true`' \
  'workflow_written: `true`' \
  'prompt_body_sha256_12:' \
  'workflow_body_sha256_12:'; do
  grep -Fq "$expected" <<<"$init_output" || die "init report missing ${expected}"
done

if grep -Fq "$token" <<<"$init_output"; then
  die "init report leaked prompt body token"
fi

generated_prompt="${tmp_dir}/${prompt_path}"
generated_workflow="${tmp_dir}/${workflow_path}"
grep -Fq "$token" "$generated_prompt" || die "generated prompt missing token"
for expected in \
  "name: GitClaw Proactive Proactive Init E2e ${normalized_timestamp}" \
  "workflow_dispatch:" \
  "- cron: '17 8 * * 1'" \
  "actions/checkout@v5" \
  "actions/setup-go@v6" \
  "go run ./cmd/gitclaw proactive enqueue" \
  "--name '${name}'" \
  "--prompt-file '${prompt_path}'" \
  "gh workflow run .github/workflows/gitclaw.yml"; do
  grep -Fq -- "$expected" "$generated_workflow" || die "generated workflow missing ${expected}"
done

if command -v actionlint >/dev/null 2>&1; then
  actionlint "$generated_workflow"
elif [[ -x /tmp/gitclaw-bin/actionlint ]]; then
  /tmp/gitclaw-bin/actionlint "$generated_workflow"
else
  log "actionlint not found; skipping generated workflow lint"
fi

run_list_json() {
  gh run list \
    --repo "$repo" \
    --workflow "$proactive_workflow" \
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
        [[ "$conclusion" == "success" ]] || die "${proactive_workflow} run failed with conclusion ${conclusion}: ${url}"
        echo "$run_json"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

find_issue_numbers() {
  gh issue list \
    --repo "$repo" \
    --state all \
    --label gitclaw:proactive \
    --limit 100 \
    --json number,title,body \
    | jq -r --arg name "$name" --arg slot "$slot" '.[] | select((.title | contains($name)) or ((.body | contains($name)) and (.body | contains($slot)))) | .number'
}

wait_for_issue_number() {
  for _ in {1..30}; do
    local numbers
    numbers="$(find_issue_numbers)"
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

started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
gh workflow run "$proactive_workflow" \
  --repo "$repo" \
  -f name="$name" \
  -f slot="$slot" \
  -f prompt="$prompt_body"
wait_for_run "$started_at" >/dev/null || die "timed out waiting for proactive workflow"

issue_number="$(wait_for_issue_number)" || die "timed out finding proactive issue for ${name}/${slot}"
log "proactive workflow created issue #${issue_number}"
wait_for_assistant_count 1 || die "timed out waiting for proactive assistant response"

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json body,labels,comments)"
grep -Fq "gitclaw:proactive-run" <<<"$(jq -r '.body' <<<"$issue_json")" || die "issue body missing proactive marker"
grep -Fq "$token" <<<"$(jq -r '.body' <<<"$issue_json")" || die "issue body missing prompt token"
comments="$(assistant_comments)"
grep -Fq "$token" <<<"$comments" || die "assistant comment missing token ${token}"
grep -Fiq "proactive" <<<"$comments" || die "assistant comment missing word proactive"
grep -Fq "dispatch-${dispatch_id}" <<<"$comments" || die "assistant marker missing dispatch event id"
labels="$(jq -r '.labels[].name' <<<"$issue_json")"
grep -Fxq "gitclaw:proactive" <<<"$labels" || die "issue missing gitclaw:proactive label"
grep -Fxq "gitclaw:done" <<<"$labels" || die "issue missing gitclaw:done label"

log "passed for issue #${issue_number}"
cleanup_success=1
