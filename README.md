# GitClaw

GitClaw is a GitHub-native OpenClaw-style assistant prototype. A conversation is
a GitHub issue, each follow-up is an issue comment, and each assistant turn runs
inside GitHub Actions. There is no always-on server in the core loop.

The current implementation focuses on a conservative, inspectable MVP:

- GitHub Models as the default model provider from Actions.
- GitHub issues and comments as the transcript.
- Deterministic slash-command reports for operational visibility.
- Repo-local `.gitclaw/` identity, memory, skills, tools, proactive, channel,
  backup, plugin, MCP metadata, and policy files.
- Body-free reports: audits expose counts, paths, names, hashes, and findings,
  not raw issue bodies, comments, prompts, tool outputs, skill bodies, or secret
  values.
- Live end-to-end harnesses that create real issues in the main repository and,
  for feature batches, include at least one real LLM-backed GitHub Models turn.

## Quick Start

Install the CLI from the repository checkout:

```bash
go test ./...
go run ./cmd/gitclaw version
```

Run a local deterministic report:

```bash
go run ./cmd/gitclaw commands
go run ./cmd/gitclaw soul verify
go run ./cmd/gitclaw tools risk
go run ./cmd/gitclaw artifacts risk
go run ./cmd/gitclaw context risk
go run ./cmd/gitclaw prompt pack
go run ./cmd/gitclaw prompt cache
go run ./cmd/gitclaw prompt compression
go run ./cmd/gitclaw prompt risk
go run ./cmd/gitclaw diffs risk
go run ./cmd/gitclaw profile manifest
go run ./cmd/gitclaw profile risk
go run ./cmd/gitclaw models usage
go run ./cmd/gitclaw models cost
go run ./cmd/gitclaw models risk
go run ./cmd/gitclaw heartbeat risk
go run ./cmd/gitclaw config risk
go run ./cmd/gitclaw orders risk
go run ./cmd/gitclaw policy risk
go run ./cmd/gitclaw approvals provenance
go run ./cmd/gitclaw approvals risk
```

The GitHub Action handles issue/comment events according to `.gitclaw/config.yml`.
The default `trigger.mode` is `label-or-prefix`, meaning an issue runs GitClaw
when it has the `gitclaw` label or starts with the configured prefix,
currently `@gitclaw`. Dedicated assistant inbox repos can switch to `inbox`;
stricter shared repos can use `label-only` or `prefix-only`.

## Core Commands

High-authority context:

```bash
gitclaw soul anchors
gitclaw soul provenance
gitclaw soul verify
gitclaw soul risk
gitclaw soul validate
gitclaw soul list
gitclaw soul edit-plan <path>
gitclaw soul info <path>
gitclaw soul search <query>
```

`gitclaw soul edit-plan <path>` is a dry-run planner for high-authority
context changes. It reports normalized target metadata and write-disabled
gates only, and its live harness now proves a real GitHub Models repo-reader
follow-up after the deterministic report.

Memory:

```bash
gitclaw memory verify
gitclaw memory risk
gitclaw memory validate
gitclaw memory timeline
gitclaw memory list
gitclaw memory promote-plan [target]
gitclaw memory info <path>
gitclaw memory search <query>
```

`gitclaw memory promote-plan [target]` is a dry-run planner for durable memory
promotion. It stays body-free and write-disabled, and its live harness now
proves a real GitHub Models repo-reader follow-up after the deterministic
report.

Skills and bundles:

```bash
gitclaw skills verify
gitclaw skills risk
gitclaw skills validate
gitclaw skills check
gitclaw skills list
gitclaw skills provenance
gitclaw skills select-plan <name>
gitclaw skills refresh-plan
gitclaw skills sources
gitclaw skills sources risk
gitclaw skills sources info <name>
gitclaw skills runtime
gitclaw skills proposals [risk]
gitclaw skills proposal-plan <name>
gitclaw skills install-plan <target>
gitclaw skills upgrade-plan <target>
gitclaw skills info <name>
gitclaw skills search <query>
gitclaw bundles list
gitclaw bundles risk
gitclaw bundles provenance
gitclaw bundles info <name>
```

`gitclaw skills install-plan <target>` and `gitclaw skills upgrade-plan
<target>` are dry-run, review-first planners for repo-local skill changes.
They report target/match hashes and no-fetch, no-install, no-mutation gates,
and their live harnesses prove real GitHub Models repo-reader follow-ups after
the deterministic report.

Migration:

```bash
gitclaw migrate plan <source>
gitclaw migrate risk <source>
```

Tools:

```bash
gitclaw tools verify
gitclaw tools risk
gitclaw tools validate
gitclaw tools list
gitclaw tools exposure
gitclaw tools exposure risk
gitclaw tools defer-plan
gitclaw tools boundary [query]
gitclaw tools provenance [query]
gitclaw tools toolsets
gitclaw tools toolsets risk
gitclaw tools toolsets provenance
gitclaw tools toolsets info <name>
gitclaw tools approval-plan <name>
gitclaw tools run-plan <name>
gitclaw tools info <name>
gitclaw tools search <query>
```

Security:

```bash
gitclaw secrets audit
gitclaw secrets scan
gitclaw secrets list
gitclaw secrets risk
```

Backups, sessions, and run provenance:

```bash
gitclaw backup verify
gitclaw backup coverage --issue <number>
gitclaw backup drill --issue <number>
gitclaw backup risk
gitclaw backup provenance
gitclaw backup manifest
gitclaw backup list
gitclaw backup timeline
gitclaw backup info --issue <number>
gitclaw backup stats
gitclaw backup search <query>
gitclaw backup export-jsonl
gitclaw backup restore-plan
gitclaw backup retention-plan
gitclaw session list --backup <issue.json>
gitclaw session status --backup <issue.json>
gitclaw session stats --backup <issue.json>
gitclaw session coverage --backup <issue.json>
gitclaw session risk --backup <issue.json>
gitclaw session search <query> --backup <issue.json>
gitclaw runs current
gitclaw runs verify
gitclaw runs history --backup <issue.json>
```

`gitclaw backup provenance` is the body-free git-history audit for fetched
`gitclaw-backups` branches. It verifies the backup tree, then reports whether
the index, README, and issue payload files are tracked, clean, and backed by
git commits without printing raw backup bodies, commit subjects, or author
identities.

The live backup-index harness proves every normal assistant turn updates the
dedicated backup branch with the issue JSON, repo index, and README, then posts
a normal GitHub Models repo-reader follow-up so index changes keep LLM/tool
coverage in the loop too.

The live backup-report harness does the same for `@gitclaw /backup`: it checks
the issue-visible, body-free backup paths and the fetched backup branch, then
requires a normal GitHub Models repo-reader/search follow-up.

`gitclaw backup restore-plan` is a dry-run recovery plan for a fetched backup
payload. Its live harness pairs deterministic restore metadata checks with a
real GitHub Models repo-reader follow-up so backup changes keep normal LLM and
tool coverage honest.

`gitclaw backup retention-plan` is a dry-run cleanup plan for fetched backups.
Its live harness now also proves a real GitHub Models repo-reader follow-up
after the deterministic keep/prune metadata check.

Operational surfaces:

```bash
gitclaw models list
gitclaw models usage
gitclaw models cost
gitclaw models risk
gitclaw heartbeat risk
gitclaw config list
gitclaw config risk
gitclaw doctor
gitclaw doctor list
gitclaw policy verify
gitclaw policy risk
gitclaw approvals provenance
gitclaw approvals risk
gitclaw artifacts list
gitclaw artifacts risk
gitclaw artifacts verify
gitclaw checkpoints risk
gitclaw rollback risk
gitclaw context risk
gitclaw prompt list
gitclaw prompt pack
gitclaw prompt cache
gitclaw prompt compression
gitclaw prompt risk
gitclaw diffs summary
gitclaw diffs risk
gitclaw diffs verify
gitclaw agents risk
gitclaw nodes risk
gitclaw hooks risk
gitclaw hooks provenance
gitclaw tools toolsets provenance
gitclaw plugins risk
gitclaw plugins mcp
gitclaw plugins mcp risk
gitclaw plugins mcp provenance
gitclaw plugins mcp info github-read
gitclaw tasks risk
gitclaw tasks ledger --backup <issue.json>
gitclaw orders risk
gitclaw channels
gitclaw channels list
gitclaw channels verify
gitclaw channels risk
gitclaw channels info telegram
gitclaw proactive list
gitclaw proactive risk
gitclaw proactive info repo-hygiene
gitclaw proactive init --name email-triage --cron "17 8 * * 1-5"
gitclaw proactive enqueue --name repo-hygiene --slot "$(date -u +%F)"
gitclaw workspace risk
gitclaw workspace verify
gitclaw profile show
gitclaw profile manifest
gitclaw profile export-plan
gitclaw profile risk
gitclaw sandbox verify
gitclaw sandbox risk
```

Use `gitclaw commands` for the full catalog.

The live channels-report harness verifies the GitHub-native Slack/Telegram
bridge contract, workflow-dispatch wake strategy, and mirrored message counts
without printing channel bodies, then requires a normal GitHub Models
repo-reader/search follow-up.
The live channels-list harness applies the same two-proof gate to the explicit
inventory alias, so `/channels list` stays equivalent to the bridge report and
still proves real repo-reader search.
The live channels-verify harness applies the same model/tool gate to the
positive bridge health check, so workflow permission/input changes prove both
deterministic readiness and real repo-reader search.
The live proactive-report and proactive-list harnesses use the same two-proof
shape for scheduled work: body-free workflow/prompt metadata first, then a
normal GitHub Models repo-reader/search follow-up.
The live prompt-report and prompt-list harnesses now use that gate for prompt
diagnostics: prompt size, hash, truncation, context, skill, and tool metadata
stay body-free, then a normal GitHub Models repo-reader/search follow-up proves
prompt inspection has not replaced real model/tool execution.
The live tools-report and tools-list harnesses apply the same rule to the tool
surface: tool contracts, gate state, validation, and active-output hashes stay
body-free, then a normal GitHub Models repo-reader/search follow-up proves real
prompt-visible tool usage.
The live tools-verify harness extends that gate to the stricter trust envelope:
contract modes, registry/runtime-attestation non-goals, and input/output hashes
stay body-free, then the model follow-up proves ordinary repo-reader search.
The live skills-select-plan harness now applies the same two-proof rule to
skill selection: selected-for-turn and gate metadata stay body-free, then a
normal GitHub Models repo-reader/search follow-up proves the selected skill
actually reaches inference. Its search proof uses a distinct high-entropy
needle and avoids explicit fixture-file reads, so the answer has to come from
`gitclaw.search_files`.
The live context-reference chat harness now proves both halves of context use:
an `@file:` line-range turn must answer from the bounded reference, and a
second normal issue-comment turn must recover a distinct repository-search
needle without falling back to a whole-file `read_file`.
The live git-reference chat harness applies the same conversational proof to
`@git:1`: first copy the bounded commit hash, then continue with repo-reader
and `gitclaw.search_files` in a second issue-comment turn.
The live search-tool chat harness also uses two distinct search needles now, so
tool grounding has to survive a continued issue conversation and cannot be only
a first-turn fixture recovery.
The live migration-plan harness now follows the same two-proof rule: the
Hermes migration plan stays deterministic, body-free, and non-mutating, then a
normal issue-comment turn proves GitHub Models, repo-reader, and
`gitclaw.search_files` still work in that migration thread.
The core issue-chat harness now applies that standard to ordinary
conversation: the follow-up comment must preserve transcript continuity and
also recover a fresh search fixture with prompt provenance and usage telemetry,
while tolerating earlier prompt-visible tools from the continuous issue thread.

## Testing

Run local tests:

```bash
go test ./...
```

Run a live E2E harness against the current GitHub repository:

```bash
scripts/e2e/github-backup-risk-report.sh
scripts/e2e/github-backup-verify.sh
scripts/e2e/github-backup-coverage.sh
scripts/e2e/github-backup-provenance.sh
scripts/e2e/github-backup-manifest.sh
scripts/e2e/github-backup-stats.sh
scripts/e2e/github-backup-list.sh
scripts/e2e/github-backup-timeline.sh
scripts/e2e/github-backup-info.sh
scripts/e2e/github-backup-search.sh
scripts/e2e/github-backup-export-jsonl.sh
scripts/e2e/github-agents-risk-report.sh
scripts/e2e/github-nodes-risk-report.sh
scripts/e2e/github-artifacts-risk-report.sh
scripts/e2e/github-checkpoints-risk-report.sh
scripts/e2e/github-context-risk-report.sh
scripts/e2e/github-prompt-pack-report.sh
scripts/e2e/github-prompt-cache-report.sh
scripts/e2e/github-prompt-compression-report.sh
scripts/e2e/github-prompt-risk-report.sh
scripts/e2e/github-diffs-risk-report.sh
scripts/e2e/github-heartbeat.sh
scripts/e2e/github-heartbeat-report.sh
scripts/e2e/github-heartbeat-risk-report.sh
scripts/e2e/github-hooks-risk-report.sh
scripts/e2e/github-hooks-provenance-report.sh
scripts/e2e/github-memory-timeline-report.sh
scripts/e2e/github-memory-risk-report.sh
scripts/e2e/github-migration-risk-report.sh
scripts/e2e/github-model-usage-report.sh
scripts/e2e/github-model-cost-report.sh
scripts/e2e/github-model-risk-report.sh
scripts/e2e/github-skills-proposal-plan-report.sh
scripts/e2e/github-skills-proposals-report.sh
scripts/e2e/github-skills-refresh-plan-report.sh
scripts/e2e/github-skills-sources-report.sh
scripts/e2e/github-skills-runtime-report.sh
scripts/e2e/github-skills-install-plan-report.sh
scripts/e2e/github-skills-upgrade-plan-report.sh
scripts/e2e/github-bundles-provenance-report.sh
scripts/e2e/github-bundles-risk-report.sh
scripts/e2e/github-orders-risk-report.sh
scripts/e2e/github-policy-risk-report.sh
scripts/e2e/github-approvals-provenance-report.sh
scripts/e2e/github-approvals-risk-report.sh
scripts/e2e/github-secrets-risk-report.sh
scripts/e2e/github-plugins-risk-report.sh
scripts/e2e/github-plugins-mcp-report.sh
scripts/e2e/github-profile-risk-report.sh
scripts/e2e/github-channel-message.sh
scripts/e2e/github-channels-info-report.sh
scripts/e2e/github-proactive.sh
scripts/e2e/github-proactive-init.sh
scripts/e2e/github-proactive-not-before.sh
scripts/e2e/github-proactive-report.sh
scripts/e2e/github-proactive-list-report.sh
scripts/e2e/github-proactive-info-report.sh
scripts/e2e/github-proactive-risk-report.sh
scripts/e2e/github-session-risk-report.sh
scripts/e2e/github-session-status-report.sh
scripts/e2e/github-session-stats-report.sh
scripts/e2e/github-session-coverage.sh
scripts/e2e/github-skills-provenance-report.sh
scripts/e2e/github-soul-provenance-report.sh
scripts/e2e/github-sandbox-risk-report.sh
scripts/e2e/github-tasks-ledger-report.sh
scripts/e2e/github-tasks-risk-report.sh
scripts/e2e/github-tools-toolsets-report.sh
scripts/e2e/github-tools-exposure-report.sh
scripts/e2e/github-tools-defer-plan-report.sh
scripts/e2e/github-tools-boundary-report.sh
scripts/e2e/github-tools-approval-plan-report.sh
scripts/e2e/github-tools-risk-report.sh
scripts/e2e/github-workspace-risk-report.sh
scripts/e2e/github-channels-risk-report.sh
scripts/e2e/github-config-risk-report.sh
scripts/e2e/github-doctor-report.sh
scripts/e2e/github-doctor-list-report.sh
scripts/e2e/github-plugins-mcp-provenance-report.sh
scripts/e2e/github-toolsets-provenance-report.sh
```

Live E2E scripts create a real GitHub issue, wait for the GitHub Actions run,
assert the assistant marker and body-free report contract, then close or label
the issue for retention. Feature batches should include a deterministic
feature-specific E2E plus a normal GitHub Models conversation E2E that proves
inference, prompt context, selected skills, and prompt-visible tools. When a
model E2E asks for a repository-search fixture token, hidden issue/comment
sentinels must use a distinct prefix so the assertion proves tool-output
grounding rather than transcript echoing.
The heartbeat-report harness now checks the body-free scheduled heartbeat
inventory and then posts a normal GitHub Models repo-reader/search follow-up,
so `/heartbeat` changes prove both operator visibility and regular
conversation continuity.
The channel-message harness now proves both sides of the Slack/Telegram bridge:
the mirrored channel comment can wake a model-backed repo-reader/search turn by
`workflow_dispatch`, and the same issue can continue with a normal
issue-comment follow-up that proves repo-reader/search again.
The workflow-dispatch harness now applies that same two-proof gate to the
generic wakeup path: dispatch-id idempotency first, then a normal GitHub Models
repo-reader/search follow-up on the same issue. It also waits for the initial
untriggered issue-opened workflow before labeling, so label timing cannot steal
the first assistant turn from the manual dispatch proof.
The checkpoints-report harness now applies the same rule to rollback readiness:
the issue-visible report stays body-free and inspect-only, then a normal
GitHub Models repo-reader/search follow-up proves ordinary tool-grounded
conversation still works after checkpoint metadata changes.
The commands-report harness does the same for `/help`: the catalog stays a
body-free deterministic capability index, then a model-backed repo-reader/search
follow-up proves the help surface has not replaced ordinary inference and tool
grounding.
The runs-report harness now applies that proof to the current-turn ledger:
issue-visible run provenance stays body-free and read-only, then a normal
GitHub Models repo-reader/search follow-up proves the live run path still
executes with prompt, skill, tool, and usage telemetry.
The channel-ingest harness proves the generic no-server bridge end to end:
workflow-dispatch mirroring, duplicate provider-message suppression, and a
normal model/tool follow-up on the canonical channel issue.
The channel-state workflow harness now proves hash-only provider offset state,
duplicate offset suppression, and two normal model/tool issue-comment turns on
the state issue, keeping gateway cursors auditable without storing raw provider
IDs.
The channel-gateway workflow harness applies the same gate to renewable gateway
leases: hash-only lease state, duplicate lease suppression, and two normal
model/tool turns on the lease state issue.
The channel-delivery workflow harness now proves outbound receipt safety:
source assistant verification, hash-only provider message receipts, duplicate
receipt suppression, and two normal model/tool turns without leaking the source
assistant body.
The proactive-report and proactive-list harnesses now require the deterministic
scheduled-job inventory to be followed by a real issue-comment GitHub Models
turn that selects `repo-reader`, exposes `gitclaw.search_files`, and recovers a
fixture token from repository search.
The proactive-init harness now applies the same gate to generated scheduled
jobs: it verifies body-free prompt/workflow creation, dispatches a real
proactive issue, then continues that issue with a model-backed repo-reader
search turn.
The proactive-not-before harness proves reminder due gates both ways: future
runs log `skipped=true` without creating an issue, while due runs create a
proactive issue and continue with a model-backed repo-reader search turn.
`gitclaw doctor list` also inventories checked-in E2E harnesses by count,
cleanup coverage, live issue coverage, model marker coverage, real model
follow-up coverage, session coverage, backup gates, and workflow-dispatch
coverage.

## Design Docs

- [GitHub-native GitClaw spec](docs/spec-github-native-gitclaw.md)
- [OpenClaw and Hermes research notes](docs/research-openclaw-hermes-landscape.md)

These docs are part of the product surface. When adding features, update the
implementation, focused tests, live E2E harnesses, and docs together.
