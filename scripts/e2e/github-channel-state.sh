#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "channel-state-e2e: $*" >&2
}

die() {
  log "$*"
  exit 1
}

repo="${GITCLAW_E2E_REPO:-}"
lock_dir="/tmp/gitclaw-channel-state-e2e.lock"

if ! mkdir "$lock_dir" 2>/dev/null; then
  die "another channel-state E2E appears to be running: ${lock_dir}"
fi

cleanup_lock() {
  rm -rf "$lock_dir"
}
trap cleanup_lock EXIT

if [[ -z "$repo" ]]; then
  repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
fi

if [[ -z "${GH_TOKEN:-}" && -z "${GITHUB_TOKEN:-}" ]]; then
  export GITHUB_TOKEN="$(gh auth token)"
fi

ensure_label() {
  local name="$1"
  local color="$2"
  local description="$3"
  gh label create "$name" --repo "$repo" --color "$color" --description "$description" --force >/dev/null
}

ensure_label gitclaw:channel 1d76db "GitClaw mirrored channel thread"
ensure_label gitclaw:e2e 0e8a16 "Temporary GitClaw end-to-end test"
ensure_label gitclaw:disabled 6a737d "Disable GitClaw on this issue"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
channel="telegram"
account_id="telegram-account-GITCLAW_CHANNEL_STATE_E2E_${timestamp}"
offset="telegram-offset-GITCLAW_CHANNEL_STATE_OFFSET_${timestamp}"
lease_run_id="channel-state-e2e-${timestamp}"

issue_number=""
cleanup() {
  if [[ -n "${issue_number:-}" ]]; then
    gh issue edit "$issue_number" --repo "$repo" --add-label gitclaw:e2e --add-label gitclaw:disabled >/dev/null 2>&1 || true
    if [[ "${GITCLAW_E2E_KEEP_ISSUE:-0}" != "1" ]]; then
      gh issue close "$issue_number" --repo "$repo" --comment "channel-state e2e cleanup" >/dev/null 2>&1 || true
    fi
  fi
  cleanup_lock
}
trap cleanup EXIT

extract_field() {
  local field="$1"
  local text="$2"
  sed -n "s/.*${field}=\\([^ ]*\\).*/\\1/p" <<<"$text"
}

output="$(go run ./cmd/gitclaw channel-state \
  --repo "$repo" \
  --channel "$channel" \
  --account-id "$account_id" \
  --offset "$offset" \
  --lease-run-id "$lease_run_id")"
log "$output"

grep -Fq "channel_state " <<<"$output" || die "missing channel_state output"
grep -Fq "created=true" <<<"$output" || die "initial run did not create state issue"
grep -Fq "updated=true" <<<"$output" || die "initial run did not post state update"
grep -Fq "duplicate=false" <<<"$output" || die "initial run unexpectedly marked duplicate"
if grep -Fq "$account_id" <<<"$output" || grep -Fq "$offset" <<<"$output"; then
  die "CLI output leaked raw account or offset"
fi

issue_number="$(extract_field issue "$output")"
account_hash="$(extract_field account_sha256_12 "$output")"
offset_hash="$(extract_field offset_sha256_12 "$output")"
[[ -n "$issue_number" && "$issue_number" != "0" ]] || die "could not parse issue number from output: ${output}"
[[ -n "$account_hash" && "$account_hash" != "$account_id" ]] || die "could not parse account hash from output: ${output}"
[[ -n "$offset_hash" && "$offset_hash" != "$offset" ]] || die "could not parse offset hash from output: ${output}"

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json title,body,labels,comments)"
labels="$(jq -r '.labels[].name' <<<"$issue_json")"
grep -Fxq "gitclaw:channel" <<<"$labels" || die "state issue missing gitclaw:channel label"

body="$(jq -r '.body' <<<"$issue_json")"
grep -Fq "gitclaw:channel-state" <<<"$body" || die "state issue missing channel-state marker"
grep -Fq "account_sha256_12=\"${account_hash}\"" <<<"$body" || die "state issue missing account hash marker"

comments="$(jq -r '[.comments[].body] | join("\n---GITCLAW-COMMENT---\n")' <<<"$issue_json")"
grep -Fq "gitclaw:channel-state-update" <<<"$comments" || die "state issue missing state update comment"
grep -Fq "offset_sha256_12=\"${offset_hash}\"" <<<"$comments" || die "state update missing offset hash marker"
state_update_count="$(jq -r '[.comments[] | select(.body | contains("gitclaw:channel-state-update"))] | length' <<<"$issue_json")"
[[ "$state_update_count" == "1" ]] || die "expected one state update comment, got ${state_update_count}"

visible="$(jq -r '[.title, .body, (.comments[].body)] | join("\n")' <<<"$issue_json")"
if grep -Fq "$account_id" <<<"$visible" || grep -Fq "$offset" <<<"$visible"; then
  die "state issue leaked raw account or offset"
fi

duplicate_output="$(go run ./cmd/gitclaw channel-state \
  --repo "$repo" \
  --channel "$channel" \
  --account-id "$account_id" \
  --offset "$offset" \
  --lease-run-id "$lease_run_id")"
log "$duplicate_output"

grep -Fq "created=false" <<<"$duplicate_output" || die "duplicate run did not reuse issue"
grep -Fq "updated=false" <<<"$duplicate_output" || die "duplicate run unexpectedly posted update"
grep -Fq "duplicate=true" <<<"$duplicate_output" || die "duplicate run did not report duplicate"
if grep -Fq "$account_id" <<<"$duplicate_output" || grep -Fq "$offset" <<<"$duplicate_output"; then
  die "duplicate CLI output leaked raw account or offset"
fi

issue_json="$(gh issue view "$issue_number" --repo "$repo" --json comments)"
state_update_count="$(jq -r '[.comments[] | select(.body | contains("gitclaw:channel-state-update"))] | length' <<<"$issue_json")"
[[ "$state_update_count" == "1" ]] || die "duplicate run produced ${state_update_count} state update comments"

log "passed for issue #${issue_number}"
