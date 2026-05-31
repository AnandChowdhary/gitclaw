# GitClaw

GitClaw is a GitHub-native OpenClaw-style assistant prototype. A conversation is
a GitHub issue, each follow-up is an issue comment, and each assistant turn runs
inside GitHub Actions. There is no always-on server in the core loop.

The current implementation focuses on a conservative, inspectable MVP:

- GitHub Models as the default model provider from Actions.
- GitHub issues and comments as the transcript.
- Deterministic slash-command reports for operational visibility.
- Repo-local `.gitclaw/` identity, memory, skills, tools, proactive, channel,
  backup, and policy files.
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
go run ./cmd/gitclaw prompt risk
go run ./cmd/gitclaw diffs risk
go run ./cmd/gitclaw profile risk
go run ./cmd/gitclaw models risk
go run ./cmd/gitclaw heartbeat risk
go run ./cmd/gitclaw config risk
go run ./cmd/gitclaw orders risk
go run ./cmd/gitclaw policy risk
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
gitclaw soul verify
gitclaw soul risk
gitclaw soul validate
gitclaw soul list
gitclaw soul edit-plan <path>
gitclaw soul info <path>
gitclaw soul search <query>
```

Memory:

```bash
gitclaw memory verify
gitclaw memory risk
gitclaw memory validate
gitclaw memory list
gitclaw memory promote-plan [target]
gitclaw memory info <path>
gitclaw memory search <query>
```

Skills and bundles:

```bash
gitclaw skills verify
gitclaw skills risk
gitclaw skills validate
gitclaw skills check
gitclaw skills list
gitclaw skills select-plan <name>
gitclaw skills refresh-plan
gitclaw skills install-plan <target>
gitclaw skills upgrade-plan <target>
gitclaw skills info <name>
gitclaw skills search <query>
gitclaw bundles list
gitclaw bundles risk
gitclaw bundles info <name>
```

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
gitclaw backup risk
gitclaw backup manifest
gitclaw backup list
gitclaw backup info --issue <number>
gitclaw backup stats
gitclaw backup search <query>
gitclaw backup export-jsonl
gitclaw backup restore-plan
gitclaw backup retention-plan
gitclaw session list --backup <issue.json>
gitclaw session stats --backup <issue.json>
gitclaw session coverage --backup <issue.json>
gitclaw session risk --backup <issue.json>
gitclaw session search <query> --backup <issue.json>
gitclaw runs current
gitclaw runs verify
```

Operational surfaces:

```bash
gitclaw models list
gitclaw models risk
gitclaw heartbeat risk
gitclaw config list
gitclaw config risk
gitclaw doctor
gitclaw doctor list
gitclaw policy verify
gitclaw policy risk
gitclaw approvals risk
gitclaw artifacts list
gitclaw artifacts risk
gitclaw artifacts verify
gitclaw checkpoints risk
gitclaw rollback risk
gitclaw context risk
gitclaw prompt list
gitclaw prompt risk
gitclaw diffs summary
gitclaw diffs risk
gitclaw diffs verify
gitclaw agents risk
gitclaw nodes risk
gitclaw hooks risk
gitclaw plugins risk
gitclaw tasks risk
gitclaw orders risk
gitclaw channels verify
gitclaw channels risk
gitclaw proactive list
gitclaw proactive risk
gitclaw workspace risk
gitclaw workspace verify
gitclaw profile risk
gitclaw sandbox verify
gitclaw sandbox risk
```

Use `gitclaw commands` for the full catalog.

## Testing

Run local tests:

```bash
go test ./...
```

Run a live E2E harness against the current GitHub repository:

```bash
scripts/e2e/github-backup-risk-report.sh
scripts/e2e/github-backup-coverage.sh
scripts/e2e/github-agents-risk-report.sh
scripts/e2e/github-nodes-risk-report.sh
scripts/e2e/github-artifacts-risk-report.sh
scripts/e2e/github-checkpoints-risk-report.sh
scripts/e2e/github-context-risk-report.sh
scripts/e2e/github-prompt-risk-report.sh
scripts/e2e/github-diffs-risk-report.sh
scripts/e2e/github-heartbeat-risk-report.sh
scripts/e2e/github-hooks-risk-report.sh
scripts/e2e/github-memory-risk-report.sh
scripts/e2e/github-migration-risk-report.sh
scripts/e2e/github-model-risk-report.sh
scripts/e2e/github-skills-refresh-plan-report.sh
scripts/e2e/github-bundles-risk-report.sh
scripts/e2e/github-orders-risk-report.sh
scripts/e2e/github-policy-risk-report.sh
scripts/e2e/github-approvals-risk-report.sh
scripts/e2e/github-secrets-risk-report.sh
scripts/e2e/github-plugins-risk-report.sh
scripts/e2e/github-profile-risk-report.sh
scripts/e2e/github-proactive-risk-report.sh
scripts/e2e/github-session-risk-report.sh
scripts/e2e/github-session-stats-report.sh
scripts/e2e/github-session-coverage.sh
scripts/e2e/github-sandbox-risk-report.sh
scripts/e2e/github-tasks-risk-report.sh
scripts/e2e/github-tools-risk-report.sh
scripts/e2e/github-workspace-risk-report.sh
scripts/e2e/github-channels-risk-report.sh
scripts/e2e/github-config-risk-report.sh
scripts/e2e/github-doctor-report.sh
scripts/e2e/github-doctor-list-report.sh
```

Live E2E scripts create a real GitHub issue, wait for the GitHub Actions run,
assert the assistant marker and body-free report contract, then close or label
the issue for retention. Feature batches should include a deterministic
feature-specific E2E plus a normal GitHub Models conversation E2E that proves
inference, prompt context, selected skills, and prompt-visible tools.
`gitclaw doctor list` also inventories checked-in E2E harnesses by count,
cleanup coverage, live issue coverage, model marker coverage, real model
follow-up coverage, session coverage, backup gates, and workflow-dispatch
coverage.

## Design Docs

- [GitHub-native GitClaw spec](docs/spec-github-native-gitclaw.md)
- [OpenClaw and Hermes research notes](docs/research-openclaw-hermes-landscape.md)

These docs are part of the product surface. When adding features, update the
implementation, focused tests, live E2E harnesses, and docs together.
