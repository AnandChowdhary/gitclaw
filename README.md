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
go run ./cmd/gitclaw prompt context
go run ./cmd/gitclaw prompt cache
go run ./cmd/gitclaw prompt compression
go run ./cmd/gitclaw prompt risk
go run ./cmd/gitclaw diffs risk
go run ./cmd/gitclaw profile catalog
go run ./cmd/gitclaw profile provenance
go run ./cmd/gitclaw profile search repo-reader
go run ./cmd/gitclaw profile diff HEAD~1
go run ./cmd/gitclaw profile snapshot
go run ./cmd/gitclaw profile manifest
go run ./cmd/gitclaw profile risk
go run ./cmd/gitclaw models catalog
go run ./cmd/gitclaw research catalog
go run ./cmd/gitclaw models usage
go run ./cmd/gitclaw models cost
go run ./cmd/gitclaw models risk
go run ./cmd/gitclaw heartbeat risk
go run ./cmd/gitclaw config risk
go run ./cmd/gitclaw security audit
go run ./cmd/gitclaw orders risk
go run ./cmd/gitclaw policy risk
go run ./cmd/gitclaw approvals catalog
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

```text
gitclaw soul catalog
gitclaw soul anchors
gitclaw soul snapshot
gitclaw soul provenance
gitclaw soul verify
gitclaw soul risk
gitclaw soul validate
gitclaw soul list
gitclaw soul edit-plan <path>
@gitclaw /soul propose --target <path> --id <id>
@gitclaw /soul rehearse --target <path> --id <id>
gitclaw soul info <path>
gitclaw soul search <query>
```

`gitclaw soul catalog` is the compact discovery view for high-authority
context. It reports anchor names, authority layers, load modes, reason codes,
counts, short hashes, and disabled mutation/profile-export gates without
printing raw soul, user, memory, tool, prompt, issue, comment, or description
bodies.

`gitclaw soul snapshot` is the body-free fingerprint for repo-stored
high-authority context. It reports each soul/profile/memory/policy anchor's
load state and short hash plus one composite snapshot hash, with registry,
profile-export, mutation, and raw-body gates disabled.

`gitclaw soul edit-plan <path>` is a dry-run planner for high-authority
context changes. It reports normalized target metadata and write-disabled
gates only, and its live harness now proves a real GitHub Models repo-reader
follow-up after the deterministic report.
Trusted issue threads can queue reviewed high-authority context changes with
`@gitclaw /soul propose --target soul --id <id>`: GitClaw opens or reuses a
GitHub proposal issue, records source hashes and current target metadata, and
keeps `.gitclaw/SOUL.md`, `.gitclaw/USER.md`, and related prompt-authority
files untouched until a human-reviewed branch promotes the change.
Add `--notify-route <route>` or `--notify-routes <a,b>` to also queue a
body-safe Slack/Telegram channel notification for that proposal issue through
the existing routebook, channel issue, outbox, and delivery receipt path.
`@gitclaw /soul rehearse --target soul --id <id>` opens or reuses a dedicated
GitHub conversation issue for trying the current high-authority context without
generating candidate edits, writing `.gitclaw/` files, or calling a model in
the source action.

Memory:

```text
gitclaw memory catalog
gitclaw memory snapshot
gitclaw memory provenance
gitclaw memory verify
gitclaw memory risk
gitclaw memory validate
gitclaw memory timeline
gitclaw memory list
gitclaw memory promote-plan [target]
@gitclaw /memory remember --target <target> --id <id>
@gitclaw /memory remember --target <target> --id <id> --notify-route <route>
@gitclaw /memory rehearse --target <target> --id <id>
gitclaw memory info <path>
gitclaw memory search <query>
```

`gitclaw memory catalog` is the compact discovery view for durable memory. It
reports OpenClaw/Hermes-inspired memory layers, prompt visibility, load modes,
reason codes, counts, hashes, validation/risk gates, and disabled mutation
gates without printing raw memory, issue, comment, prompt, session, or
embedding bodies.

`gitclaw memory snapshot` is the durable-memory fingerprint for backups and
restores. It reports per-memory-file metadata and one composite snapshot hash
for `.gitclaw/MEMORY.md` plus dated memory notes, with raw memory, issue,
comment, prompt, session, and embedding bodies excluded and write/provider
gates disabled.

`gitclaw memory provenance` maps repo-local memory files to body-free git
history. It reports tracked/dirty state, last commit IDs/dates, commit-subject
hashes, validation/risk rollups, and disabled provider/write gates without
printing raw memory bodies, issue/comment text, prompts, git subjects, or
author identities.

`gitclaw memory promote-plan [target]` is a dry-run planner for durable memory
promotion. It stays body-free and write-disabled, and its live harness now
proves a real GitHub Models repo-reader follow-up after the deterministic
report.
Trusted issue threads can queue reviewed memory with
`@gitclaw /memory remember --target long-term --id <id>`: GitClaw opens or
reuses a GitHub proposal issue, records source hashes and memory target
metadata, and keeps `.gitclaw/MEMORY.md` plus dated notes untouched until a
human-reviewed branch promotes the change.
Add `--notify-route <route>` or `--notify-routes <a,b>` to also queue a
body-safe Slack/Telegram channel notification for that memory proposal through
the reviewed routebook, channel issue, outbox, and delivery receipt path.
`@gitclaw /memory rehearse --target long-term --id <id>` opens or reuses a
dedicated GitHub conversation issue for trying the current memory context
without generating candidate memory, writing `.gitclaw/` files, or calling a
model in the source action.

Skills and bundles:

```text
gitclaw skills verify
gitclaw skills risk
gitclaw skills validate
gitclaw skills check
gitclaw skills list
gitclaw skills catalog
gitclaw skills snapshot
gitclaw skills provenance
gitclaw skills select-plan <name>
gitclaw skills refresh-plan
gitclaw skills sources
gitclaw skills sources verify
gitclaw skills sources lock
gitclaw skills sources update-plan
gitclaw skills sources provenance
gitclaw skills sources risk
gitclaw skills sources info <name>
gitclaw skills sources search <query>
gitclaw skills runtime
gitclaw skills proposals [risk]
gitclaw skills proposal-plan <name>
@gitclaw /skills sources propose <name> --source <ref>
@gitclaw /skills sources propose <name> --source <ref> --notify-route <route>
@gitclaw /skills propose <name>
@gitclaw /skills propose <name> --notify-route <route>
@gitclaw /skills rehearse <name> --id <id>
gitclaw skills install-plan <target>
gitclaw skills upgrade-plan <target>
gitclaw skills info <name>
gitclaw skills search <query>
gitclaw bundles catalog
gitclaw bundles list
gitclaw bundles risk
gitclaw bundles provenance
gitclaw bundles info <name>
gitclaw bundles search <query>
@gitclaw /bundles rehearse <name> --id <id>
```

`gitclaw skills install-plan <target>` and `gitclaw skills upgrade-plan
<target>` are dry-run, review-first planners for repo-local skill changes.
Trusted issue threads can start the same review queue with
`@gitclaw /skills propose <name>`: GitClaw opens or reuses a GitHub issue for
the proposal, records source/request hashes and review paths, and keeps active
`SKILL.md` files untouched until a human-reviewed branch converts the proposal
into repository files.
Add `--notify-route <route>` or `--notify-routes <a,b>` to also queue a
body-safe Slack/Telegram channel notification for that proposal issue through
the existing routebook, channel issue, outbox, and delivery receipt path.
External skill provenance gets the same issue-native treatment with
`@gitclaw /skills sources propose <name> --source <ref>`: GitClaw opens or
reuses a labeled review conversation issue for a proposed
`.gitclaw/skill-sources/<name>.yaml` pin, records only source-ref/request
hashes and review paths, avoids registry fetches and installs, and requires a
later GitHub Models follow-up on that proposal issue before promotion.
Add `--notify-route <route>` or `--notify-routes <a,b>` to queue a body-safe
Slack/Telegram channel notification for that source-pin proposal without
copying the raw source ref into the source receipt or channel receipt.
Trusted issue threads can also open a live rehearsal lane with
`@gitclaw /skills rehearse <name> --id <id>`: GitClaw creates or reuses a
GitHub issue labeled for normal GitClaw conversation, records only skill/source
metadata and hashes, avoids install/update writes, and requires the next
comment on that rehearsal issue to prove real GitHub Models skill/tool usage.
The install/upgrade planners report target/match hashes and no-fetch,
no-install, no-mutation gates, and their live harnesses prove real GitHub
Models repo-reader follow-ups after the deterministic report.

`gitclaw skills snapshot` is the body-free fingerprint for repo-local skills,
prompt-visible selected skills, skill bundles, and reviewed source pins. It
prints short hashes, counts, validation/risk/source gates, and one composite
snapshot hash without exposing raw skill bodies, descriptions, source refs, or
bundle instructions.

`gitclaw skills catalog` is a compact eligibility index inspired by the
OpenClaw/Hermes `skills_list`/`skill_view` split. It reports prompt eligibility,
load mode, gate reasons, and description/body hashes without printing raw skill
bodies or descriptions.

`gitclaw skills sources provenance` maps reviewed source pins in
`.gitclaw/skill-sources/*.yml` and `.gitclaw/skill-sources/*.yaml` to body-free
git history. It reports source-pin paths, hashes, tracked/dirty state, last
commit metadata, and no-registry/no-fetch/no-install gates without printing raw
source refs, source YAML, skill bodies, commit subjects, issue text, or
credentials.

`gitclaw skills sources verify` treats those pins as a body-free trust
envelope. It reports source-pin hashes, source-ref hashes, current skill
hashes, registry/fetch/install gates, and risk rollups without contacting a
registry, fetching remote sources, running installers, or printing source or
skill bodies.

`gitclaw skills sources lock` projects a reproducibility lock from reviewed
source pins. It reports lock state, aggregate lock hash, expected/current skill
hashes, stale/unpinned/missing counts, and `.clawhub/lock.json` presence/hash
if present, without loading registry state or printing lockfile, source, or
skill bodies.

`gitclaw skills sources update-plan` is the no-fetch companion to
OpenClaw/ClawHub-style skill updates. It reports which reviewed pins would need
manual action because they are stale, unpinned, missing, remote, or risky, while
keeping registry contact, remote fetch, installers, dependency installs, and
repository mutation disabled.

`gitclaw skills sources search <query>` searches reviewed source-pin metadata
only: source name, path, skill path, source kind, trust level, install mode,
hashes, and risk codes. It hashes the raw query and never prints raw query
text, source refs, source YAML, skill bodies, issue text, prompts, or
credentials.

`gitclaw bundles catalog` is the compact orchestration index for Hermes-style
skill bundles. It reports repo-local bundle roles, selected/load state,
skill-ref resolution, instruction hashes, risk rollups, reason codes, and
disabled registry/install/mutation gates without printing raw bundle YAML,
bundle instructions, skill bodies, prompts, issue text, or credentials.

`gitclaw bundles search <query>` searches the same repo-local bundle metadata by
hashing the raw query and reporting only match fields, paths, skill refs, and
instruction hashes. Its live harness proves the deterministic body-free search
report, then posts a normal GitHub Models repo-reader/search follow-up.

Trusted issue threads can open a bundle rehearsal lane with
`@gitclaw /bundles rehearse <name> --id <id>`: GitClaw creates or reuses a
GitHub issue labeled for normal conversation, records only bundle/source
metadata and hashes, suppresses raw bundle YAML, bundle instructions, skill
bodies, and source text, and requires the next comment on that issue to prove
real GitHub Models skill/tool usage.

Migration:

```bash
gitclaw migrate plan <source>
gitclaw migrate risk <source>
```

Hooks:

```bash
gitclaw hooks catalog
gitclaw hooks list
gitclaw hooks risk
gitclaw hooks verify
gitclaw hooks provenance
```

`gitclaw hooks catalog` is the compact event-automation discovery view. It maps
hook commands, policy/spec/event/approval/provenance layers, ignored handler
state, and disabled execution/payload gates without printing raw hook files,
handler files, provider payloads, issue/comment bodies, prompts, tool outputs,
credentials, or secrets.

Tools:

```bash
gitclaw tools catalog
gitclaw tools snapshot
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
gitclaw tools readiness <name>
gitclaw tools map <name>
gitclaw tools run-plan <name>
gitclaw tools info <name>
gitclaw tools search <query>
@gitclaw /tools rehearse <name> --id <id>
```

`gitclaw tools catalog` is the compact progressive-disclosure index for the
tool surface. It reports direct core tools, repo-reviewed toolsets, and
metadata-only MCP allowlist entries with gate state, reason codes, counts, and
hashes, without printing raw schemas, toolset instructions, MCP args, tool
inputs, tool outputs, issue bodies, comments, prompts, credentials, or secrets.
`gitclaw tools snapshot` adds a stable body-free fingerprint over the same
surface plus loaded tool guidance and prompt-visible active-output metadata. It
emits one composite hash for drift checks while keeping registry contact,
runtime MCP launch, structured tools, shell execution, repository mutation, and
raw body gates disabled.
`@gitclaw /tools request-run <name> --id <id>` is the issue-native action
surface for reviewed tool execution requests. It opens or reuses a dedicated
GitHub issue with only hashes, normalized tool metadata, validation gates, and
review decisions; it does not call a model, execute a tool, copy raw source
text, include raw tool inputs/outputs, or mutate the repository.
Add `--notify-route <route>` or `--notify-routes <a,b>` to queue a body-safe
Slack/Telegram channel notification for the review issue through the existing
routebook, outbox, and delivery receipt path.
`gitclaw tools map <name>` and `@gitclaw /tools map <name>` render a
body-safe OpenClaw/Hermes-style tool sequence: list, search, info,
approval-plan, run-plan, then optional reviewed request-run. The map reports
only normalized contract metadata, hashes, validation/risk gates, and review
steps; it does not execute tools, launch MCP servers, create approval,
rehearsal, or run-request issues, call a model, mutate workflows, mutate the
repository, or print raw issue bodies, comments, prompts, tool inputs, or tool
outputs.
`gitclaw tools readiness <name>` and `@gitclaw /tools readiness <name>` render
a body-free prompt-visible readiness checklist for one deterministic tool
contract. It reports config, allowlist, contract mode, validation, risk,
active-output hash-only, model-context, and no-execution gates before the tool
is treated as safe prompt context. It never executes tools, launches MCP
servers, creates approval/rehearsal/run-request issues, calls a model, mutates
workflows, mutates the repository, or prints raw issue bodies, comments,
prompts, tool inputs, or tool outputs.
`@gitclaw /tools cancel-run --id <id>` closes an open reviewed tool-run request
issue after posting a durable `gitclaw:tool-run-cancel` marker on that request.
It does not approve or execute the tool, call a model, copy raw source text, or
mutate repository files; the source receipt reports only the request hash,
target issue/comment ids, closed state, and no-execution gates.
`@gitclaw /tools rehearse <name> --id <id>` opens or reuses a labeled GitHub
conversation issue for trying a tool contract without creating a run request.
The source receipt is body-free and model-free; the rehearsal issue records the
normalized tool, gate state, validation summary, and no-execution flags, then a
normal comment on that issue can exercise GitHub Models and prompt-visible
tool behavior.

Security:

```bash
gitclaw secrets audit
gitclaw secrets scan
gitclaw secrets list
gitclaw secrets risk
```

Backups, sessions, and run provenance:

```bash
gitclaw backup catalog
gitclaw backup verify
gitclaw backup snapshot
gitclaw backup coverage --issue <number>
gitclaw backup drill --issue <number>
gitclaw backup risk
gitclaw backup provenance
gitclaw backup manifest
gitclaw backup list
gitclaw backup timeline
gitclaw backup info --issue <number>
gitclaw backup stats
gitclaw backup freshness
gitclaw backup continuity
gitclaw backup search <query>
gitclaw backup export-jsonl
gitclaw backup restore-plan
gitclaw backup retention-plan
@gitclaw /backup rehearse --id <id>
@gitclaw /backup restore-request --id <id>
gitclaw session catalog
gitclaw session list --backup <issue.json>
gitclaw session provenance --backup <issue.json>
gitclaw session tools --backup <issue.json>
gitclaw session skills --backup <issue.json>
gitclaw session usage --backup <issue.json>
gitclaw session trajectory --backup <issue.json>
gitclaw session compaction --backup <issue.json>
gitclaw session resume --backup <issue.json>
@gitclaw /session handoff --id <id>
gitclaw session status --backup <issue.json>
gitclaw session stats --backup <issue.json>
gitclaw session coverage --backup <issue.json>
gitclaw session risk --backup <issue.json>
gitclaw session search <query> --backup <issue.json>
gitclaw runs current
gitclaw runs verify
gitclaw runs history --backup <issue.json>
```

`gitclaw backup catalog` is the compact recovery-surface map for the backup
system. It lists the issue intents, local commands, fetched-branch gates, and
restore/retention mutation boundaries without printing raw issue bodies,
comments, prompts, backup payloads, credentials, git subjects, or author
identities.

`gitclaw backup snapshot` is the compact lockfile-style fingerprint for a
fetched backup branch. It verifies the backup tree, fingerprints `index.json`,
`README.md`, and every indexed issue payload, then reports only paths, counts,
timestamps, gates, and short hashes. It is useful for proving the archive shape
changed as expected without opening raw conversation JSON.

`gitclaw session catalog` is the compact session-surface map. It lists the
issue intents, local backup commands, recall gates, and GitHub-issue canonical
session boundary without printing raw issue bodies, comments, assistant
replies, prompts, tool outputs, search queries, or credentials.

`gitclaw session provenance --backup <issue.json>` is the named prompt
provenance audit for a backed-up issue session. It reports assistant-turn
marker counts, prompt-context hashes, prompt-visible skills/tools, model names,
and token usage telemetry without printing issue bodies, comments, assistant
replies, prompts, search queries, or tool outputs.

`gitclaw session tools --backup <issue.json>` is the named tool-use ledger for
a backed-up issue session. It aggregates prompt-visible tools across
assistant-turn markers, model-backed tool turns, prompt-context hash counts, and
token usage telemetry without printing issue bodies, comments, assistant
replies, prompts, tool inputs, search queries, or tool outputs.

`gitclaw session skills --backup <issue.json>` is the matching skill-use ledger
for a backed-up issue session. It aggregates prompt-visible skill names,
selected skill markers, model-backed skill turns, prompt-context hash counts,
and token usage telemetry without printing issue bodies, comments, assistant
replies, prompts, skill bodies, search queries, or tool outputs.

`gitclaw session usage --backup <issue.json>` is the token/cache usage ledger
for a backed-up issue session. It aggregates assistant-turn marker telemetry by
model and by turn, including prompt, completion, total, cache-read, and
cache-write token counts, without printing issue bodies, comments, assistant
replies, prompts, provider responses, search queries, or tool outputs.

`gitclaw session trajectory --backup <issue.json>` is the body-free trajectory
manifest for a backed-up issue session. It combines assistant-turn marker
metadata, run/idempotency hashes, prompt-context hashes, prompt-visible
skills/tools, model names, and usage counters without printing issue bodies,
comments, assistant replies, prompts, provider responses, search queries, run
URLs, or tool outputs.

`gitclaw session compaction --backup <issue.json>` is the body-free session
compaction-readiness audit. It models Hermes-style 50% in-loop and 85%
gateway-hygiene thresholds plus OpenClaw-style trajectory pruning, then reports
transcript sizes, bounded-message counts, per-message hashes, provenance,
model/usage telemetry, and disabled mutation gates without summarizing,
splitting, mutating memory, or printing raw bodies.

`gitclaw session resume --backup <issue.json>` is the body-free continuation
readiness audit. It reports the GitHub issue-thread resume key, labels,
latest-message hashes, assistant-marker provenance, model/usage telemetry, and
reentry gates proving the next issue comment can continue the same session
without a server, socket, workflow-dispatch bridge, or hidden external session
database.

`@gitclaw /session handoff --id <id>` creates or reuses a labeled GitHub issue
as a fresh conversation lane for the current session. The source issue gets a
body-free receipt with only hashes, counts, duplicate status, and reentry gates;
the handoff issue carries the raw handoff id and session metadata, then normal
comments there run through the regular GitHub Models workflow.

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

The live backup-catalog harness covers `@gitclaw /backup catalog`: it verifies
the deterministic command/gate catalog, checks the post-turn backup branch, and
then forces a real GitHub Models repo-reader/search follow-up so the catalog
surface keeps LLM/tool coverage.

The live backup-snapshot harness covers `@gitclaw /backup snapshot`: it records
the deferred issue-side intent, verifies the fetched backup branch can produce a
body-free composite snapshot hash, and then forces a real GitHub Models
repo-reader/search follow-up.

The live session-catalog harness covers `@gitclaw /session catalog`: it checks
the deterministic session command/gate map, then posts a real GitHub Models
repo-reader/search follow-up so the session surface proves LLM/tool grounding.

The live session-provenance harness starts with a normal GitHub Models
repo-reader/search turn, then verifies `@gitclaw /session provenance` reports
the model marker, prompt-context hash, selected skill, prompt-visible tools, and
usage telemetry without leaking hidden issue or comment text.

The live session-tools harness follows the same model-first shape, then verifies
`@gitclaw /session tools` reports the session-level tool ledger, model-backed
tool turn, prompt-visible tools, and usage telemetry without leaking hidden
issue or comment text.

The live session-skills harness follows the same model-first shape, then
verifies `@gitclaw /session skills` reports the session-level skill ledger,
selected repo-reader skill, model-backed skill turn, prompt-context evidence,
and usage telemetry without leaking hidden issue or comment text.

The live session-usage harness follows the same model-first shape, then
verifies `@gitclaw /session usage` reports normalized token/cache telemetry,
model-backed usage turns, prompt-context evidence, and raw-provider/body-free
gates without leaking hidden issue or comment text.

The live session-trajectory harness follows the same model-first shape, then
verifies `@gitclaw /session trajectory` reports an export-like assistant-turn
manifest with model, run-hash, prompt-context, skill/tool, and usage evidence
without leaking hidden issue/comment text or raw run URLs.

The live session-compaction harness follows the same model-first shape, then
verifies `@gitclaw /session compaction` reports threshold readiness,
bounded-transcript cards, model-backed provenance, usage telemetry, and
disabled summary/mutation gates without leaking hidden issue/comment text or
raw run URLs.

The live session-resume harness follows the same model-first shape, then
verifies `@gitclaw /session resume` reports GitHub issue-thread continuation
readiness, resume anchors, latest assistant marker provenance, and reentry
gates without leaking hidden issue/comment text or raw run URLs.
The live session-handoff harness starts with a model-backed repo-reader/search
turn, verifies `@gitclaw /session handoff --id <id>` opens or reuses a labeled
body-free handoff issue, checks duplicate suppression, then continues on the
handoff issue with another real GitHub Models repo-reader/search turn.

`gitclaw backup restore-plan` is a dry-run recovery plan for a fetched backup
payload. Its live harness pairs deterministic restore metadata checks with a
real GitHub Models repo-reader follow-up so backup changes keep normal LLM and
tool coverage honest.

`@gitclaw /backup rehearse --id <id>` opens or reuses a dedicated GitHub issue
for a dry-run recovery rehearsal. The source receipt is body-free and
model-free; the rehearsal issue records the expected backup branch paths and
dry-run gates, then normal comments on that issue exercise GitHub Models and
repo-reader tools.

`@gitclaw /backup restore-request --id <id>` opens or reuses a dedicated
GitHub issue for reviewing a possible restore. It records the expected backup
branch paths, approval gates, and dry-run commands, but the action does not
read backup payloads, mutate the repository, replay GitHub API calls, or call a
model. Continue on the generated restore-request issue to discuss the recovery
with GitHub Models after local backup verification.
Add `--notify-route <route>` or `--notify-routes <a,b>` to also queue a
body-safe Slack/Telegram channel notification for that restore request through
the reviewed routebook, channel issue, outbox, and delivery receipt path.

`gitclaw backup retention-plan` is a dry-run cleanup plan for fetched backups.
Its live harness now also proves a real GitHub Models repo-reader follow-up
after the deterministic keep/prune metadata check.

`gitclaw backup continuity` verifies chronological backup history in a fetched
`gitclaw-backups` branch. It reports longest gaps, threshold violations, and
hash-only gap cards without printing raw issue titles, bodies, comments, or
transcripts.

Operational surfaces:

```bash
gitclaw models list
gitclaw models catalog
gitclaw models usage
gitclaw models cost
gitclaw models risk
gitclaw research catalog
gitclaw research sources
gitclaw research coverage
gitclaw research verify
gitclaw heartbeat risk
gitclaw config list
gitclaw config risk
gitclaw doctor
gitclaw doctor list
gitclaw policy verify
gitclaw policy risk
gitclaw approvals catalog
gitclaw approvals provenance
gitclaw approvals risk
gitclaw artifacts catalog
gitclaw artifacts list
gitclaw artifacts risk
gitclaw artifacts verify
gitclaw checkpoints catalog
gitclaw checkpoints preview HEAD~1
gitclaw checkpoints risk
@gitclaw /checkpoints rehearse --id <id> --target HEAD~1
gitclaw rollback catalog
gitclaw rollback diff HEAD~1
gitclaw rollback risk
gitclaw context risk
gitclaw prompt list
gitclaw prompt pack
gitclaw prompt context
gitclaw prompt cache
gitclaw prompt compression
gitclaw prompt risk
gitclaw diffs summary
gitclaw diffs risk
gitclaw diffs verify
gitclaw agents catalog
gitclaw agents provenance
gitclaw agents risk
gitclaw nodes catalog
gitclaw nodes risk
gitclaw hooks catalog
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
gitclaw channel-send --channel slack --thread-id <thread> --message-id <id> --body "hello"
gitclaw channel-send --route e2e-slack-route --message-id <id> --body "hello"
gitclaw channel-status --channel slack --thread-id <thread> --message-id <id> --status-id <id> --state working
gitclaw channel-edit --channel slack --thread-id <thread> --message-id <id> --edit-id <id> --body "updated text"
gitclaw channel-react --channel slack --thread-id <thread> --message-id <id> --reaction eyes
@gitclaw /channels send --route e2e-slack-route --message-id <id>
@gitclaw /channels probe --route e2e-slack-route --message-id <id>
@gitclaw /channels broadcast e2e-slack-route,e2e-telegram-route --message-id <id>
@gitclaw /channels invite e2e-slack-route,e2e-telegram-route --message-id <id>
@gitclaw /channels room e2e-slack-route,e2e-telegram-route --room-id <id> --message-id <id>
@gitclaw /channels huddle e2e-slack-route,e2e-telegram-route --huddle-id <id> --message-id <id>
@gitclaw /channels poll e2e-slack-route,e2e-telegram-route --poll-id <id> --message-id <id>
@gitclaw /channels poll-vote --poll-id <id> --message-id <id> --notify-message-id <id> --choice 1
@gitclaw /channels rollcall e2e-slack-route,e2e-telegram-route --rollcall-id <id> --message-id <id>
@gitclaw /channels roll --dice 2d6+1 --message-id <id> --notify-message-id <id>
@gitclaw /channels choose --message-id <id> --notify-message-id <id>
@gitclaw /channels this-or-that --wyr-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels oracle --choose-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels mood focused --message-id <id> --notify-message-id <id> --intensity 4
@gitclaw /channels room-pulse handoff --pulse-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels quick-replies handoff --reply-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels status-wheel release --wheel-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels sticker confetti --sticker-id <id> --message-id <id> --notify-message-id <id> --scale 4
@gitclaw /channels toast launch-ready --toast-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels postcard launch-ready --postcard-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels timer 25m --timer-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels bingo release --bingo-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels riddle release --riddle-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels haiku launch --haiku-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels soundtrack launch --soundtrack-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels story-dice fun --story-dice-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels arcade fun --arcade-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels coach skills --coach-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels skill-spotlight repo-reader --spotlight-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels research-spotlight openclaw --spotlight-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels research-map openclaw --map-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels nudge release-captain --nudge-id <id> --message-id <id> --notify-message-id <id> --tone gentle
@gitclaw /channels constellation research --constellation-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels mission-control research --mission-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels cockpit research --cockpit-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels palette fun --palette-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels compass all --compass-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels mode tool-review --mode-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels warmup tools --warmup-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels icebreaker --icebreaker-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels spark --spark-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels vibe-check --vibe-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels dock <target-route> --dock-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels session-search <query> --message-id <id> --notify-message-id <id>
@gitclaw /channels memory-search <query> --message-id <id> --notify-message-id <id>
@gitclaw /channels recovery-map issue --map-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels backup-search <query> --message-id <id> --notify-message-id <id>
@gitclaw /channels backup-spotlight <focus> --spotlight-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels backup-timeline --timeline-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels backup-freshness --freshness-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels backup-continuity --continuity-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels backup-info <issue> --message-id <id> --notify-message-id <id>
@gitclaw /channels checkpoint-status --message-id <id> --notify-message-id <id>
@gitclaw /channels availability --message-id <id> --notify-message-id <id>
@gitclaw /channels topic --topic-id <id>
@gitclaw /channels activity typing --activity-id <id>
@gitclaw /channels soul-info <path> --message-id <id> --notify-message-id <id>
@gitclaw /channels soul-spotlight <focus> --spotlight-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels soul-risk --message-id <id> --notify-message-id <id>
@gitclaw /channels soul-search <query> --message-id <id> --notify-message-id <id>
@gitclaw /channels rsvp e2e-slack-route,e2e-telegram-route --rsvp-id <id> --message-id <id>
@gitclaw /channels rsvp-response --rsvp-id <id> --message-id <id> --notify-message-id <id> --response yes
@gitclaw /channels status --message-id <id> --status-id <id> --state working
@gitclaw /channels edit --message-id <id> --edit-id <id>
@gitclaw /channels react --message-id <id> --reaction eyes
@gitclaw /channels pin --message-id <id>
@gitclaw /channels reply --message-id <id>
@gitclaw /channels deliverable --deliverable-id <id> --message-id <id> --filename <name>
@gitclaw /channels task --task-id <id> --message-id <id>
@gitclaw /channels watch --watch-id <id> --cadence <cadence> --message-id <id>
@gitclaw /channels propose-order --id <id> --cadence <cadence> --message-id <id>
@gitclaw /channels clip --clip-id <id> --message-id <id>
@gitclaw /channels open-loop --loop-id <id> --message-id <id>
@gitclaw /channels snippet --snippet-id <id> --language <lang> --message-id <id>
@gitclaw /channels bookmark-message --bookmark-id <id> --message-id <id>
@gitclaw /channels fork --fork-id <id> --new-thread-id <id> --message-id <id>
@gitclaw /channels merge --merge-id <id> --from-thread <id> --message-id <id>
@gitclaw /channels attachment --attachment-id <id> --message-id <id> --filename <name>
@gitclaw /channels decision --decision-id <id> --message-id <id>
@gitclaw /channels digest --digest-id <id> --message-id <id>
@gitclaw /channels journal --journal-id <id> --date <date> --message-id <id>
@gitclaw /channels time-capsule --capsule-id <id> --open-after <hint> --message-id <id>
@gitclaw /channels quote --quote-id <id> --message-id <id>
@gitclaw /channels glossary --glossary-id <id> --message-id <id>
@gitclaw /channels faq --faq-id <id> --message-id <id>
@gitclaw /channels skill-note --note-id <id> --skill <name> --message-id <id>
@gitclaw /channels soul-note --note-id <id> --area <area> --message-id <id>
@gitclaw /channels backup-note --note-id <id> --scope <scope> --message-id <id>
@gitclaw /channels memory-note --note-id <id> --target <target> --message-id <id>
@gitclaw /channels tool-lesson --note-id <id> --tool <tool> --message-id <id>
@gitclaw /channels idea --idea-id <id> --message-id <id>
@gitclaw /channels quest --quest-id <id> --message-id <id>
@gitclaw /channels ritual --ritual-id <id> --cadence <cadence> --message-id <id>
@gitclaw /channels pact --pact-id <id> --message-id <id>
@gitclaw /channels forecast --forecast-id <id> --message-id <id>
@gitclaw /channels lore --lore-id <id> --message-id <id>
@gitclaw /channels boundary --boundary-id <id> --message-id <id>
@gitclaw /channels whiteboard --jam-id <id> --message-id <id>
@gitclaw /channels kudos --kudos-id <id> --message-id <id>
@gitclaw /channels retro --retro-id <id> --message-id <id>
@gitclaw /channels playbook --playbook-id <id> --message-id <id>
@gitclaw /channels insight --insight-id <id> --message-id <id>
@gitclaw /channels board-card --card-id <id> --lane <lane> --message-id <id>
@gitclaw /channels checklist --checklist-id <id> --message-id <id>
@gitclaw /channels agenda --agenda-id <id> --message-id <id>
@gitclaw /channels propose-workspace --workspace-id <id> --target .gitclaw/workspaces/<name>.md --message-id <id>
@gitclaw /channels incident --incident-id <id> --severity <severity> --message-id <id>
@gitclaw /channels voice --voice-id <id> --duration <seconds> --message-id <id>
@gitclaw /channels image --image-id <id> --width <px> --height <px> --message-id <id>
@gitclaw /channels link --link-id <id> --url <url> --message-id <id>
@gitclaw /channels access-request --access-id <id> --scope <scope> --message-id <id>
@gitclaw /channels platform telegram --state running --message-id <id>
@gitclaw /channels browser --message-id <id> --notify-message-id <id>
@gitclaw /channels model --message-id <id>
@gitclaw /channels skills --message-id <id>
@gitclaw /channels skill-search <query> --message-id <id> --notify-message-id <id>
@gitclaw /channels skill-info <skill> --message-id <id> --notify-message-id <id>
@gitclaw /channels skill-map <skill> --map-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels bundle-map <bundle> --map-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels source-map <source> --map-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels tool-search <query> --message-id <id> --notify-message-id <id>
@gitclaw /channels tool-info <tool> --message-id <id> --notify-message-id <id>
@gitclaw /channels tool-spotlight <focus> --spotlight-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels tool-map <tool> --map-id <id> --message-id <id> --notify-message-id <id>
@gitclaw /channels whoami --identity-id <id> --message-id <id>
@gitclaw /channels contact --contact-id <id> --role <role> --message-id <id>
@gitclaw /channels handoff --id <id> --message-id <id>
@gitclaw /channels request-run search_files --id <id> --message-id <id>
@gitclaw /channels approval-plan search_files --id <id> --message-id <id>
@gitclaw /channels rehearse-tool search_files --id <id> --message-id <id>
@gitclaw /channels propose-toolset --toolset-id <id> --message-id <id>
@gitclaw /channels propose-prompt --prompt-id <id> --message-id <id>
@gitclaw /channels propose-bundle --bundle-id <id> --message-id <id>
@gitclaw /channels propose-skill weekly-review --message-id <id>
@gitclaw /channels rehearse-skill repo-reader --id <id> --message-id <id>
@gitclaw /channels propose-soul --target soul --id <id> --message-id <id>
@gitclaw /channels rehearse-soul --target soul --id <id> --message-id <id>
@gitclaw /channels propose-memory --target long-term --id <id> --message-id <id>
@gitclaw /channels rehearse-memory --target long-term --id <id> --message-id <id>
@gitclaw /channels rehearse-backup --id <id> --message-id <id>
@gitclaw /channels restore-request --id <id> --message-id <id>
@gitclaw /channels checkpoint-status --message-id <id> --notify-message-id <id>
@gitclaw /channels availability --message-id <id> --notify-message-id <id>
@gitclaw /channels topic --topic-id <id>
@gitclaw /channels activity typing --activity-id <id>
@gitclaw /channels rehearse-checkpoint --target HEAD~1 --id <id> --message-id <id>
@gitclaw /channels remind --reminder-id <id> --message-id <id> --at <time>
@gitclaw /channels done --message-id <id>
gitclaw proactive list
gitclaw proactive schedule
gitclaw proactive chain
gitclaw proactive risk
gitclaw proactive info repo-hygiene
gitclaw proactive init --name email-triage --cron "17 8 * * 1-5"
gitclaw proactive enqueue --name repo-hygiene --slot "$(date -u +%F)"
gitclaw proactive enqueue --name repo-hygiene --slot "$(date -u +%F)" --notify-route e2e-telegram-route
gitclaw workspace catalog
gitclaw workspace risk
gitclaw workspace verify
gitclaw profile catalog
gitclaw profile show
gitclaw profile provenance
gitclaw profile search <query>
gitclaw profile diff [base-ref]
gitclaw profile snapshot
gitclaw profile manifest
gitclaw profile export-plan
gitclaw profile risk
gitclaw sandbox verify
gitclaw sandbox risk
gitclaw security audit
gitclaw security risk
```

Use `gitclaw commands` for the full catalog.

`gitclaw research catalog` is the body-free OpenClaw/Hermes research map. It
reports reviewed official source IDs/URLs, local research-file hashes, adopted
GitHub-native patterns, rejected v1 surfaces, and no-runtime-fetch gates
without printing raw research notes, source bodies, issue/comment bodies,
prompts, tool outputs, credentials, or secrets.

`gitclaw security audit` is the OpenClaw-style operator security posture card.
It aggregates config, policy, sandbox, channel, tool, skill, plugin, and secret
risk metadata under GitClaw's personal-assistant trust model without printing
issue/comment bodies, prompts, workflow bodies, tool outputs, credentials, or
secret values.

`gitclaw profile catalog` is the compact discovery view for the repo-local
agent profile. It maps profile commands and layers across identity, soul,
memory, skills, tools, models, proactive jobs, channels, backups, and sessions
while keeping raw profile files, issue/comment bodies, prompts, tool outputs,
credentials, sessions, and backup payloads out of the report.

`gitclaw profile provenance` maps the repo-local profile envelope to git
history without printing raw profile bodies. It reports profile path hashes,
tracked/dirty state, last commit IDs/dates, and commit-subject hashes only, so
profile changes stay reviewable in git while author identities, commit
subjects, issue/comment bodies, prompts, sessions, backups, credentials, and
secret values stay out of issue-visible output.

`gitclaw profile search <query>` searches the repo-local `.gitclaw/` profile
envelope with a deterministic lexical matcher. It reports only paths,
categories, line numbers, scores, query hashes, and line hashes, so operators
can find profile facts without posting raw profile files, skill bodies, issue
text, comments, prompts, tool outputs, or raw search queries.

`gitclaw profile diff [base-ref]` compares repo-local `.gitclaw/` profile
files against a git base ref using status and numstat metadata only. It
reports changed profile paths, statuses, counts, and hashes while excluding raw
patches, profile bodies, skill bodies, issue/comment text, prompts, requested
ref text, git subjects, author identities, sessions, backups, credentials, and
secret values.

`gitclaw proactive chain` maps reviewed `.gitclaw/proactive/*.md`
`gitclaw:proactive-context-from` metadata into a body-free dependency report.
It reports prompt paths, hashes, skill hints, resolved job names, missing-source
hashes, and cycle/self-reference gates without printing prompt bodies,
workflow bodies, issue/comment text, tool outputs, credentials, or secret
values.

`gitclaw profile snapshot` is the composite body-free fingerprint for the
profile envelope. It ties the profile manifest, soul snapshot, memory
snapshot, skill snapshot, and tool snapshot together with one profile snapshot
hash while keeping raw profile files, skills, memories, tool outputs,
issue/comment bodies, sessions, backups, credentials, and secret values out of
the report.

`gitclaw workspace catalog` is the compact discovery view for the GitHub
Actions checkout workspace. It maps workspace commands, policy/spec files, git
metadata, workflow setup, repository inventory, runtime, durable-state, and
body-free gates without printing workspace file bodies, issue/comment bodies,
prompts, tool outputs, workflow bodies, or credentials.

`gitclaw agents catalog` is the compact discovery view for the single GitHub
Actions assistant. It maps agent commands, policy/spec files, runtime,
conversation boundary, tool-name intent, approval gates, and explicit
no-delegation/no-subagent boundaries without printing agent files,
issue/comment bodies, prompts, tool outputs, channel payloads, or credentials.

`gitclaw agents provenance` maps `.gitclaw/AGENTS.md` and
`.gitclaw/agents/*.md` to body-free git provenance. It reports tracked state,
dirty state, last commit IDs/dates, risk metadata, validation counts, and
commit-subject hashes while keeping agent files, issue/comment bodies, prompts,
tool outputs, git subjects, author identities, channel payloads, and
credentials out of the report.

`gitclaw nodes catalog` is the compact discovery view for the GitHub Actions
execution node surface. It maps node commands, policy/spec files, runtime, wake
paths, conversation boundary, capability names, approval gates, and explicit
no-gateway/no-pairing/no-remote-exec boundaries without printing node files,
issue/comment bodies, prompts, tool outputs, channel payloads, worker payloads,
or credentials.

`gitclaw artifacts catalog` is the compact discovery view for short-lived
Actions evidence bundles. It maps artifact commands, policy/spec files,
reviewed upload workflow steps, storage, redaction, retention, durable-backup
boundaries, and no-hidden-state gates without printing artifact payloads,
prompts, tool outputs, issue/comment bodies, channel payloads, backup payloads,
sessions, or credentials.

`gitclaw checkpoints catalog` is the compact discovery view for rollback
readiness. It maps checkpoint and rollback commands, git history, worktree
state, backup-branch evidence, recent-commit metadata, rollback diff-stat
preview requirements, and disabled destructive-git gates without printing
diffs, file bodies, commit subjects, issue/comment bodies, prompts, tool
outputs, or credentials.

`gitclaw rollback diff HEAD~1` is the inspect-only version of Hermes
`/rollback diff`: it compares a target git ref to `HEAD`, reports numstat
counts and path hashes, and refuses restore/reset/clean/checkout behavior.
The matching issue command is `@gitclaw /rollback diff HEAD~1`.
`@gitclaw /checkpoints rehearse --id <id> --target HEAD~1` opens or reuses a
dedicated GitHub rehearsal issue for talking through rollback readiness. The
source receipt stays body-free and model-free, while the rehearsal issue records
safe dry-run checkpoint/rollback commands and remains labeled for normal
GitClaw conversation.

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
The channel outbox path makes channels more than reports:
`gitclaw channel-outbox --channel telegram --account-id <id> --issue-number <issue> --out outbox.json`
returns undelivered assistant replies for a provider gateway, while
`gitclaw channel-delivery` records the receipt after Telegram, Slack, or another
sender posts the message.
`gitclaw channel-send` adds the GitHub-originated half: scheduled jobs,
operator commands, or future proactive flows can queue a
`gitclaw:channel-outbound` message onto a channel thread, and the same
outbox/delivery receipt path handles provider delivery without a server.
Named routes in `.gitclaw/channels/routes.yaml` make that usable for
proactive jobs: `gitclaw channel-send --route <name>` resolves a reviewed
Slack/Telegram channel and thread template before queuing the outbound comment.
Trusted GitHub issues and comments can also use the same routebook directly:
`@gitclaw /channels send --route <name> --message-id <id>` queues an outbound
channel comment, then posts a body-free receipt on the source issue while
leaving provider delivery to `channel-outbox` and `channel-delivery`.
`@gitclaw /channels probe --route <name> --message-id <id>` sends a
deterministic route test message through the same queue. The source receipt
keeps route names, thread IDs, message IDs, source text, and probe bodies out
of the issue-visible report, while the channel issue/outbox/delivery path
proves the reviewed route can actually carry a provider-facing message.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels reply
--message-id <id>` infers the current Slack/Telegram thread and queues the
outbound provider message on that same issue. This turns the GitHub issue into
a bridge console while keeping the receipt body-free and delivery delegated to
the outbox/delivery path.
For multi-route announcements, `@gitclaw /channels broadcast <route-a>,<route-b>
--message-id <id>` queues one outbound comment per reviewed route, reports only
route/thread/message/body hashes in the source receipt, suppresses duplicates
per route, and still leaves actual provider delivery to outbox/delivery.
`@gitclaw /channels invite <route-a>,<route-b> --message-id <id>` composes an
issue invitation from the source issue number, URL, title, and optional note,
queues it to each reviewed route, and keeps raw route names, notes, titles, and
outbound invite bodies out of the source receipt.
`@gitclaw /channels room <route-a>,<route-b> --room-id <id> --message-id <id>`
creates or reuses a durable GitHub room issue, labels it for normal GitClaw
conversation, writes the human-readable topic and notes there, and queues
provider-facing room invites through the same routebook/outbox path. The source
receipt stays body-free and reports only hashes, counts, issue numbers, and
duplicate status; the room issue is where the model-backed conversation
continues.
`@gitclaw /channels huddle <route-a>,<route-b> --huddle-id <id> --message-id
<id>` creates or reuses a dedicated GitHub huddle issue, labels it for normal
GitClaw conversation, writes the human-readable topic and agenda there, and
queues provider-facing huddle invites through the same routebook/outbox path.
The source receipt stays body-free and reports only hashes, counts, issue
numbers, and duplicate status.
`@gitclaw /channels poll <route-a>,<route-b> --poll-id <id> --message-id <id>`
does the same for lightweight decisions: it creates or reuses a GitHub poll
issue, writes the question and options there, labels it for normal GitClaw
conversation, and queues provider-facing poll invites through reviewed routes.
The source receipt stays body-free and reports only hashes, counts, issue
numbers, and duplicate status.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
poll-vote --poll-id <id> --message-id <id> --notify-message-id <id> --choice
1` records a channel-origin vote on the GitHub poll issue, resolves option
numbers to poll option text, and queues a provider-facing acknowledgement back
to the same channel thread. Duplicate votes are suppressed by
`poll_id + vote_id`; duplicate acknowledgements are suppressed by
`channel + notify_message_id`.
`@gitclaw /channels rollcall <route-a>,<route-b> --rollcall-id <id>
--message-id <id>` creates or reuses a dedicated GitHub check-in issue, writes
the prompt and instructions there, labels it for normal GitClaw conversation,
and queues provider-facing rollcall invites through the same routebook/outbox
path. It is meant for lightweight standups, attendance, status checks, and
"everyone please respond here" moments without adding a server.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels roll
--dice 2d6+1 --message-id <id> --notify-message-id <id>` queues a
provider-facing dice/coin result back to Slack/Telegram. The result is
deterministic from GitHub/channel metadata, so it needs no model call, no
external randomness, no provider API call, and no repo mutation; the source
receipt only exposes hashes, counts, duplicate status, and delivery metadata.
`@gitclaw /channels choose --message-id <id> --notify-message-id <id>` does
the same for option picking. Put choices in the following body as bullets or
pass simple `--option <value>` flags; GitClaw queues one provider-visible
selected choice while the source receipt keeps raw option text and the selected
choice out of band.
`@gitclaw /channels this-or-that --wyr-id <id> --message-id <id>
--notify-message-id <id> --this <a> --that <b>` with a trailing
`Question: ...`
queues a provider-facing two-option prompt with a deterministic lean and a
low-pressure "pick A or B" nudge. Aliases include `/channels would-you-rather`,
`/channels wyr`, and `/channels either-or`; the receipt keeps raw prompts,
option text, selected lean text, thread ids, and message ids out of band.
`@gitclaw /channels oracle --choose-id <id> --message-id <id>
--notify-message-id <id>` uses the same deterministic picker with a bounded
static answer deck. Add a trailing `Question: ...` line for multi-word
questions. It queues one provider-visible oracle answer without calling a
model, using external randomness, predicting the future, or mutating the
repository; the source receipt keeps the raw question and answer out of band.
`@gitclaw /channels mood <mood> --message-id <id> --notify-message-id <id>
--intensity 1..5` queues a provider-facing presence update back to the current
Slack/Telegram thread. Optional `Note: ...` trailing text is visible in the
provider update, while the source receipt keeps raw mood text, notes, thread
ids, message ids, and mood ids out of band.
`@gitclaw /channels room-pulse <focus> --pulse-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing thread pulse back to the
current Slack/Telegram thread. It counts safe GitClaw issue/comment markers,
reports whether the room looks active, and suggests a next command while the
source receipt keeps raw issue/comment bodies, focus values, notes, step text,
thread ids, message ids, and pulse ids out of band.
`@gitclaw /channels quick-replies <lane> --reply-id <id> --message-id <id>
--notify-message-id <id>` queues provider-facing reply chips back to the current
Slack/Telegram thread. Lanes such as `general`, `handoff`, `skills`, `tools`,
and `fun` expose a small copyable set of next commands, while the source
receipt keeps raw option text, lane values, notes, thread ids, message ids, and
reply ids out of band. The action does not execute any suggested command.
`@gitclaw /channels status-wheel <lane> --wheel-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing deterministic status spin
back to the current Slack/Telegram thread. Lanes such as `focus`, `release`,
`triage`, `tools`, `soul`, and `fun` select a bounded status and micro-action
without external randomness; the source receipt keeps raw lane values, notes,
deck text, selected status text, selected action text, thread ids, message ids,
and wheel ids out of band.
`@gitclaw /channels sticker <sticker> --sticker-id <id> --message-id <id>
--notify-message-id <id> --scale 1..5` queues a provider-facing sticker card
back to the current Slack/Telegram thread. Optional `Note: ...` trailing text
is visible in the provider update, while the source receipt keeps raw sticker
ids, sticker names, notes, thread ids, message ids, and channel bodies out of
band. This does not generate images, fetch media, upload files, call provider
APIs, call a model, or mutate the repository.
`@gitclaw /channels toast <title> --toast-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing celebration toast back to
the current Slack/Telegram thread. Optional `Reason: ...` trailing text is
visible in the provider update, while the source receipt keeps raw toast ids,
titles, reasons, tones, thread ids, message ids, and channel bodies out of
band. This does not open durable kudos issues, call provider APIs, call a
model, edit workflows, or mutate the repository.
`@gitclaw /channels postcard <title> --postcard-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing place/caption scene card
back to the current Slack/Telegram thread. Optional `Caption: ...` trailing
text is visible in the provider update, while the source receipt keeps raw
postcard ids, titles, captions, tones, thread ids, message ids, and channel
bodies out of band. This does not call a model, generate images, fetch media,
call provider APIs, edit workflows, or mutate the repository.
`@gitclaw /channels timer <duration> --timer-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing timebox cue back to the
current Slack/Telegram thread. Optional `Label: ...` and `Note: ...` trailing
text are visible in the provider update, while the source receipt keeps raw
timer ids, durations, labels, notes, thread ids, message ids, and channel
bodies out of band. This does not create reminder issues, schedule workflows,
start provider timers, call provider APIs, call a model, edit workflows, or
mutate the repository.
`@gitclaw /channels bingo <theme> --bingo-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing deterministic mini-game
card from bounded static decks such as `fun`, `release`, `triage`, and
`pairing`. Optional `Note: ...` trailing text is visible in the provider
update, while the source receipt keeps raw bingo ids, themes, notes, card
cells, thread ids, message ids, and channel bodies out of band. This does not
call a model, use external randomness, persist game state, track scores, call
provider APIs, edit workflows, or mutate the repository.
`@gitclaw /channels riddle <theme> --riddle-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing deterministic riddle card
from bounded static decks such as `focus`, `release`, `debug`, `care`, and
`fun`. Optional `Note: ...` trailing text is visible in the provider update,
while the source receipt keeps raw riddle ids, themes, notes, question text,
hint text, answer text, thread ids, message ids, and channel bodies out of
band. This does not call a model, use external randomness, persist game state,
track scores, call provider APIs, edit workflows, or mutate the repository.
`@gitclaw /channels haiku <theme> --haiku-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing three-line poem card from a
bounded static line deck. Optional `Note: ...` trailing text is visible in the
provider update, while the source receipt keeps raw haiku ids, themes, notes,
poem lines, thread ids, message ids, and channel bodies out of band. This does
not call a model, use external randomness, generate media, call provider APIs,
edit workflows, or mutate the repository.
`@gitclaw /channels soundtrack <theme> --soundtrack-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing three-track soundtrack card
from bounded static decks. Optional `Note: ...` trailing text is visible in the
provider update, while the source receipt keeps raw soundtrack ids, themes,
notes, track text, thread ids, message ids, and channel bodies out of band.
This does not call a model, use external randomness, generate media, fetch
audio, call provider APIs, edit workflows, or mutate the repository.
`@gitclaw /channels story-dice <theme> --story-dice-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing prompt-dice card from
bounded static decks. Optional `Note: ...` trailing text is visible in the
provider update, while the source receipt keeps raw story-dice ids, themes,
notes, rolled prompts, thread ids, message ids, and channel bodies out of band.
This does not call a model, use external randomness, generate media, call
provider APIs, edit workflows, or mutate the repository.
`@gitclaw /channels arcade <fun|warmup|story|launch|tools|research|soul|backups|channels>
--arcade-id <id> --message-id <id> --notify-message-id <id>` queues a
provider-facing bounded play-menu card back to the current Slack/Telegram
thread. The card shows a mode, frame, four playful next moves, and copyable
commands for existing channel surfaces such as story-dice, spark, postcard,
cockpit, or tool/research/soul/backup actions. Optional `Note: ...` trailing
text is visible in the provider update, while the source receipt keeps raw
arcade ids, modes, notes, move text, command text, thread ids, message ids, and
channel bodies out of band. This does not call a model, generate play text
dynamically, use external randomness, persist game state, track scores, execute
commands, install skills, execute tools, read backup payloads, read soul bodies,
call provider APIs, mutate workflows, create schedules, or mutate the
repository.
`@gitclaw /channels coach <all|skills|tools|soul|memory|backups|channels|fun>
--coach-id <id> --message-id <id> --notify-message-id <id>` queues a
provider-facing next-move card from repo-local skill, tool, and soul metadata.
The card includes body-safe signals and suggested channel-native follow-up
commands for the selected lane. Optional `Note: ...` trailing text is visible
in the provider update, while the source receipt keeps raw coach ids, lanes,
notes, recommendation text, thread ids, message ids, and channel bodies out of
band. This does not execute commands, install skills, execute tools, read
backup payloads, read soul bodies, call provider APIs, call a model, edit
workflows, or mutate the repository.
`@gitclaw /channels nudge <target> --nudge-id <id> --message-id <id>
--notify-message-id <id> --tone gentle|normal|urgent` queues a provider-facing
attention nudge back to the current Slack/Telegram thread. Optional `Note: ...`
trailing text is visible in the provider update, while the source receipt keeps
raw nudge ids, targets, tones, notes, thread ids, message ids, and channel
bodies out of band. This does not create a task, reminder, watch, scheduled
workflow, provider API call, model call, or repository mutation.
`@gitclaw /channels constellation <all|skills|tools|soul|memory|backups|research|channels|fun>
--constellation-id <id> --message-id <id> --notify-message-id <id>` queues a
provider-facing capability star-map card back to the current Slack/Telegram
thread. The card shows a north star, three bounded capability stars, and
copyable next commands for the selected lane. The `research` lane connects
OpenClaw/Hermes research patterns to GitClaw follow-up commands. Optional
`Note: ...` trailing text is visible in the provider update, while the source
receipt keeps raw constellation ids, lanes, notes, star text, command text,
thread ids, message ids, and channel bodies out of band. This does not
dynamically generate stars, use external randomness, execute commands, install
skills, execute tools, read backup payloads, read soul bodies, write memory,
fetch sources, browse live sources, create schedules, call provider APIs, call
a model, edit workflows, change policy, or mutate the repository.
`@gitclaw /channels mission-control <all|skills|tools|soul|memory|backups|research|channels|launch|fun>
--mission-id <id> --message-id <id> --notify-message-id <id>` queues a
provider-facing bounded operating-loop card back to the current Slack/Telegram
thread. The card shows a lane, a control-loop sentence, four bounded loop
steps, and copyable next commands. The `research` lane connects OpenClaw and
Hermes research to one review-first GitClaw move. Optional `Note: ...`
trailing text is visible in the provider update, while the source receipt keeps
raw mission ids, lanes, notes, step text, command text, thread ids, message
ids, and channel bodies out of band. This does not dynamically generate loops,
use external randomness, execute commands, install skills, execute tools, read
backup payloads, read soul bodies, write memory, fetch sources, browse live
sources, create schedules, call provider APIs, call a model, edit workflows,
change policy, or mutate the repository.
`@gitclaw /channels cockpit <all|skills|tools|soul|memory|backups|research|channels|launch|fun>
--cockpit-id <id> --message-id <id> --notify-message-id <id>` queues a
provider-facing bounded status-board card back to the current Slack/Telegram
thread. The card shows a lane, a board sentence, four gauges, and copyable next
commands. The `research` lane turns OpenClaw/Hermes source and pattern work
into a quick provider-readable cockpit. Optional `Note: ...` trailing text is
visible in the provider update, while the source receipt keeps raw cockpit ids,
lanes, notes, gauge text, command text, thread ids, message ids, and channel
bodies out of band. This does not dynamically generate cockpit content, use
external randomness, execute commands, install skills, execute tools, read
backup payloads, read soul bodies, write memory, fetch sources, browse live
sources, create schedules, call provider APIs, call a model, edit workflows,
change policy, or mutate the repository.
`@gitclaw /channels palette <all|core|skills|tools|soul|backups|fun>
--palette-id <id> --message-id <id> --notify-message-id <id>` queues a
provider-facing command palette back to the current Slack/Telegram thread.
Optional `Note: ...` trailing text is visible in the provider update, while the
source receipt keeps raw palette ids, lanes, notes, shortcut commands, thread
ids, message ids, and channel bodies out of band. This does not execute
commands, install skills, execute tools, read backup payloads, read soul
bodies, call provider APIs, call a model, or mutate the repository.
`@gitclaw /channels compass <all|core|skills|tools|soul|memory|backups|fun>
--compass-id <id> --message-id <id> --notify-message-id <id>` queues a
provider-facing orientation card with safe next steps for the current
Slack/Telegram thread. Optional `Note: ...` trailing text is visible in the
provider update, while the source receipt keeps raw compass ids, focus values,
notes, step text, thread ids, message ids, and channel bodies out of band. This
does not execute commands, install skills, execute tools, read backup payloads,
read soul bodies, call provider APIs, call a model, or mutate the repository.
`@gitclaw /channels mode <focus|pairing|triage|recovery|tool-review|soul-review|backup-review|quiet>
--mode-id <id> --message-id <id> --notify-message-id <id>` queues a
provider-facing advisory mode card back to the current Slack/Telegram thread.
Optional `Note: ...` trailing text is visible in the provider update, while the
source receipt keeps raw mode ids, mode names, notes, suggested steps, thread
ids, message ids, and channel bodies out of band. This does not persist mode
state, execute commands, install skills, execute tools, read backup payloads,
read soul bodies, edit workflows, change policy, create schedules, call
provider APIs, call a model, or mutate the repository.
`@gitclaw /channels warmup <focus|pairing|triage|design|launch|retro|icebreaker|spark|tools|soul|backups|fun>
--warmup-id <id> --message-id <id> --notify-message-id <id>` queues a
provider-facing conversation-starter card back to the current Slack/Telegram
thread. The card includes three deterministic prompts for the selected theme
and optional visible `Note: ...` trailing text. The source receipt keeps raw
warmup ids, theme names, prompt text, notes, thread ids, message ids, and
channel bodies out of band. This does not call a model, execute commands,
install skills, execute tools, read backup payloads, read soul bodies, create
schedules, call provider APIs, or mutate the repository.
`@gitclaw /channels icebreaker --icebreaker-id <id> --message-id <id>
--notify-message-id <id>` is the low-pressure opener alias for the same
provider-facing warmup card, defaulting to the `icebreaker` theme when no theme
is supplied. It gives Slack/Telegram a simple way to start a quiet thread
without creating polls, rollcalls, tasks, schedules, model calls, provider API
calls, workflow edits, or repository mutation.
`@gitclaw /channels spark --spark-id <id> --message-id <id>
--notify-message-id <id>` is the brainstorming alias for the same
provider-facing warmup card, defaulting to the `spark` theme when no theme is
supplied. It helps a loose idea become one concrete experiment without
generating prompt text dynamically, creating a quest, creating a task, opening
a proposal, creating a schedule, calling a model, calling provider APIs, or
mutating the repository.
`@gitclaw /channels vibe-check --vibe-id <id> --message-id <id>
--notify-message-id <id>` is the chat-native alias for the same provider-facing
warmup card, defaulting to the `fun` theme when no theme is supplied. It gives
Slack/Telegram a quick, low-ceremony check-in prompt without creating a poll,
rollcall, task, schedule, model call, provider API call, or repository mutation.
`@gitclaw /channels dock <target-route> --dock-id <id> --message-id <id>
--notify-message-id <id>` captures a channel-origin route-continuity request
as a durable GitHub dock issue and queues a provider-facing review link back
to the current Slack/Telegram thread. The dock issue may include the readable
target route and reason because it is the review surface; the source receipt
keeps raw dock ids, target routes, thread ids, message ids, reasons, and
channel bodies out of band. This does not change provider routes, persist
session routes, edit `.gitclaw/channels/routes.yaml`, mutate workflows, call
provider APIs, call a model, or mutate the repository.
`@gitclaw /channels session-search <query> --message-id <id>
--notify-message-id <id>` searches the current GitHub-backed channel transcript
and queues provider-facing recall metadata back to Slack/Telegram. It reports
query hashes, message indexes, source ids, scores, line hashes, and duplicate
status without printing raw search queries, channel bodies, issue bodies,
assistant replies, prompts, or tool outputs.
`@gitclaw /channels memory-search <query> --message-id <id>
--notify-message-id <id>` searches repo-local durable memory and queues
provider-facing recall metadata back to Slack/Telegram. It reports query
hashes, memory paths, line numbers, scores, file hashes, line hashes, and
duplicate status without printing raw search queries, memory bodies, channel
bodies, issue bodies, comment bodies, prompts, tool outputs, or calling an
external memory provider.
`@gitclaw /channels tool-map <tool> --map-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing safe tool sequence back to
Slack/Telegram. The card shows the reviewed path from tool status, tool search,
and tool info through approval-plan, rehearsal, and run-request issue commands.
It does not execute tools, run shells, launch MCP servers, activate toolsets,
create approval/rehearsal/run-request issues, call provider APIs, call a model,
mutate workflows, or mutate the repository. The source receipt keeps raw map
ids, requested tool names, notes, step text, provider ids, issue bodies,
comments, raw schemas, raw inputs, and raw outputs out of band.
`@gitclaw /channels tool-spotlight <focus> --spotlight-id <id> --message-id
<id> --notify-message-id <id>` queues a deterministic provider-facing safe
tool spotlight card back to Slack/Telegram. The card picks one enabled
read-only or metadata-only built-in tool contract from safe metadata, shows the
tool name, mode, trigger hash, and follow-up commands, and keeps raw focus
text, notes, spotlight ids, tool triggers, schemas, inputs, outputs, channel
bodies, issue bodies, comments, and prompts out of the source receipt. It does
not execute tools, run shells, launch MCP servers, activate toolsets, call a
model, call provider APIs, use external randomness, mutate workflows, or mutate
the repository.
`@gitclaw /channels skill-spotlight <focus> --spotlight-id <id> --message-id
<id> --notify-message-id <id>` queues a deterministic provider-facing skill
spotlight card back to Slack/Telegram. The card picks one enabled, requirement-
complete repo-local skill from safe metadata, shows compact name/path/hash/
requirement counts and follow-up commands, and keeps raw focus text, notes,
spotlight ids, skill descriptions, skill bodies, channel bodies, issue bodies,
comments, prompts, and tool outputs out of the source receipt. It does not
install or update skills, contact registries, run installers, execute tools,
call a model, call provider APIs, use external randomness, or mutate the
repository.
`@gitclaw /channels research-spotlight <focus> --spotlight-id <id>
--message-id <id> --notify-message-id <id>` queues a deterministic
provider-facing research card back to Slack/Telegram. The card draws from the
reviewed static OpenClaw/Hermes source, pattern, and rejected-surface catalog,
can name one selected source or pattern with safe follow-up commands, and keeps
raw focus text, notes, spotlight ids, source ids/URLs, pattern text, surfaces,
channel bodies, issue bodies, comments, prompts, tool outputs, and research
bodies out of the source receipt. It does not fetch sources, browse live docs,
call a model, execute tools, call provider APIs, use external randomness,
mutate workflows, or mutate the repository.
`@gitclaw /channels research-map <focus> --map-id <id> --message-id <id>
--notify-message-id <id>` queues a provider-facing research-to-GitClaw
sequence card back to Slack/Telegram. The card draws one source, pattern, or
rejected surface from the reviewed static catalog, then shows safe follow-up
commands such as `/research catalog`, `research-spotlight`, and domain-specific
channel cards. The source receipt keeps raw focus text, notes, map ids, source
ids/URLs, pattern text, surfaces, step text, channel bodies, issue bodies,
comments, prompts, tool outputs, and research bodies out of band. It does not
fetch sources, browse live docs, call a model, execute tools, call provider
APIs, use external randomness, mutate workflows, or mutate the repository.
`@gitclaw /channels recovery-map <issue|repo|channel|incident> --map-id <id>
--message-id <id> --notify-message-id <id>` queues a provider-facing backup
recovery sequence back to Slack/Telegram. The card shows the safe order:
backup status, backup search, backup info, reviewed rehearsal, then reviewed
restore request. It reads only repo-local backup catalog metadata and local
backup docs presence; it does not fetch backup branches, read backup payloads,
restore files, create rehearsal issues, create restore-request issues, replay
GitHub APIs, call provider APIs, call a model, or mutate the repository. The
source receipt keeps raw map ids, scopes, notes, step text, provider ids, issue
bodies, comments, and backup payloads out of band.
`@gitclaw /channels backup-search <query> --message-id <id>
--notify-message-id <id>` searches the durable `gitclaw-backups` archive and
queues provider-facing recall metadata back to Slack/Telegram. In GitHub
Actions it fetches the backup branch read-only when local backups are absent;
it reports backup/search status, query hashes, issue paths, sources, trust
metadata, scores, body hashes, and line hashes without printing raw search
queries, channel bodies, backup payloads, issue bodies, comment bodies,
transcripts, prompts, or tool outputs.
`@gitclaw /channels backup-spotlight <focus> --spotlight-id <id>
--message-id <id> --notify-message-id <id>` fetches the same durable
`gitclaw-backups` archive read-only when local backups are absent and queues
one deterministic provider-facing backup spotlight card back to Slack/Telegram.
The card draws from lexical matches when the focus finds archive lines and
falls back to recent backup timeline metadata otherwise. It may show the
selected issue number, backup-relative path, source, role, trust flag, hashes,
payload size, comment/transcript counts, and safe follow-up commands; the
source receipt keeps route/thread/message ids, spotlight ids, focus text,
notes, backup roots, paths, issue titles, payload bodies, comments,
transcripts, prompts, and tool outputs out of band and confirms no restore,
branch write, GitHub API replay, provider API call, model call, tool execution,
or repository mutation happened.
`@gitclaw /channels backup-timeline --timeline-id <id> --message-id <id>
--notify-message-id <id>` fetches the same durable `gitclaw-backups` archive
read-only when local backups are absent and queues a provider-facing
chronology card back to Slack/Telegram. The provider card shows backup status,
fetch status, issue count, window/ordering metadata, first/latest issue
numbers, timestamp span, and recent point metadata with hashes; the source
receipt keeps route/thread/message ids, timeline ids, backup roots, paths,
issue numbers, titles, bodies, comments, transcripts, prompts, and tool
outputs out of band and confirms no restore, branch write, GitHub API replay,
provider API call, model call, or repository mutation happened.
`@gitclaw /channels backup-freshness --freshness-id <id> --message-id <id>
--notify-message-id <id> --max-age-hours 24` fetches the same durable
`gitclaw-backups` archive read-only when local backups are absent and queues a
provider-facing freshness gate card back to Slack/Telegram. The provider card
shows backup/freshness status, verify status, fetch status, max-age threshold,
latest issue number, latest backup timestamp, age, payload hash, title hash,
and pass/fail gate. The source receipt keeps route/thread/message ids,
freshness ids, backup roots, paths, issue titles, issue bodies, comments,
transcripts, prompts, and tool outputs out of band and confirms no restore,
branch write, GitHub API replay, provider API call, model call, or repository
mutation happened.
`@gitclaw /channels backup-continuity --continuity-id <id> --message-id <id>
--notify-message-id <id> --max-gap-hours 168` fetches the same durable
`gitclaw-backups` archive read-only when local backups are absent and queues a
provider-facing continuity/gap card back to Slack/Telegram. The provider card
shows backup/verify/gate/fetch status, max-gap threshold, issue count,
first/latest issues and timestamps, total span, longest gap, and reported
gaps over the threshold. The source receipt keeps route/thread/message ids,
continuity ids, backup roots, paths, issue titles, issue bodies, comments,
transcripts, prompts, and tool outputs out of band and confirms no restore,
branch write, GitHub API replay, provider API call, model call, or repository
mutation happened.
`@gitclaw /channels backup-info <issue> --message-id <id>
--notify-message-id <id>` fetches the same durable `gitclaw-backups` archive
read-only when local backups are absent and queues one focused backup metadata
card back to Slack/Telegram. The provider card shows useful recovery metadata
such as backup status, issue number, backup-relative issue path, generated
time, payload size/hash, label/comment/transcript counts, and body hash counts;
the source receipt keeps target issue numbers, route/thread/message ids,
backup info ids, backup roots, and backup paths hashed and confirms no restore,
backup branch write, GitHub API replay, provider delivery, tool execution, or
model call happened in the deterministic action.
`@gitclaw /channels checkpoint-status --message-id <id>
--notify-message-id <id>` queues a provider-facing checkpoint and rollback
readiness card back to Slack/Telegram. The card reports git availability,
branch/head metadata, worktree change counts, backup-branch presence, recent
commit hashes, risk counts, and safe inspect-only follow-up commands. The
source receipt confirms that the deterministic action did not generate raw
diffs, print file bodies or commit subjects, restore, reset, clean, checkout,
call a model, call provider APIs, or mutate repository files.
`@gitclaw /channels rsvp <route-a>,<route-b> --rsvp-id <id> --message-id <id>`
creates or reuses a dedicated GitHub RSVP issue, writes the event title,
when/where/host metadata, and details there, labels it for normal GitClaw
conversation, and queues provider-facing RSVP cards through reviewed routes.
The source receipt stays body-free and reports only hashes, counts, issue
numbers, and duplicate status.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
rsvp-response --rsvp-id <id> --message-id <id> --notify-message-id <id>
--response yes` records a channel-origin yes/no/maybe response as a durable
comment on the GitHub RSVP issue and queues a provider-facing acknowledgement
back to the same channel thread. Duplicate responses are suppressed by
`rsvp_id + response_id`; duplicate acknowledgements are suppressed by
`channel + notify_message_id`.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels status
--message-id <id> --status-id <id> --state working` queues a structured
`gitclaw:channel-status` progress update for provider gateways. The status body
is deliverable through `channel-outbox`, while the source receipt reports only
hashes, duplicate status, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels edit
--message-id <id> --edit-id <id>` queues a structured
`gitclaw:channel-edit` replacement for a provider message. The replacement body
is deliverable through `channel-outbox`, while the source receipt reports only
hashes, duplicate status, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels topic
--topic-id <id>` queues a structured `gitclaw:channel-topic` thread title/topic
update for provider gateways. The topic text is deliverable through
`channel-outbox`, while the source receipt reports only hashes, duplicate
status, delivery gates, and explicit no-provider-API/no-GitHub-issue-rename
flags.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
activity typing --activity-id <id>` queues a transient
`gitclaw:channel-activity` signal for provider-native UI such as typing,
recording, uploading, or thinking indicators. Provider gateways render the
activity after reading it through `channel-outbox`; the source receipt reports
only hashes, TTL, duplicate status, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels react
--message-id <id> --reaction <name>` queues a structured
`gitclaw:channel-reaction` acknowledgement for the provider gateway. Duplicate
reactions are suppressed by channel, target message id, and reaction name; the
issue-visible receipt reports only hashes and delivery gates, while
`channel-outbox` exposes the pending reaction and `channel-delivery` records
the provider receipt.
`@gitclaw /channels pin --message-id <id>` is the short form for the same
provider reaction path: it infers the current mirrored channel thread and
queues a default `pushpin` reaction while keeping message IDs, thread IDs, and
the reaction name out of the receipt.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
deliverable --deliverable-id <id> --message-id <id> --filename <name>` queues
a provider-native file/link deliverable comment on the same channel issue.
Gateways fetch the visible filename, URL, media type, checksum, and caption
through `gitclaw channel-outbox --include-body`, then record the provider
upload with `gitclaw channel-delivery`. The command receipt stays body-free
with only hashes and delivery gates; it does not upload files or call a model.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels task
--task-id <id> --message-id <id>` creates or reuses a normal GitHub task issue
from the channel thread, writes the human-readable title and notes there, and
queues a provider-facing task link back to the mirrored Slack/Telegram thread.
The source receipt stays body-free: it reports task/thread/message/title/notes
hashes, duplicate status, notification queue metadata, and delivery gates
without printing raw provider IDs, channel message bodies, task titles, or
task notes.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels watch
--watch-id <id> --cadence <cadence> --message-id <id>` creates or reuses a
GitHub watch issue for proactive follow-up. The watch issue stores the
human-readable subject, notes, and cadence so scheduled GitHub Actions
workflows or normal issue comments can continue the watch later; the channel
action itself does not open a socket, call a model, or call provider APIs. It
queues a provider-facing watch link back to Slack/Telegram and keeps the source
receipt body-free with only hashes, duplicate state, notification metadata, and
delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
propose-order --id <id> --cadence <cadence> --message-id <id>` creates or
reuses a GitHub standing-order proposal issue for reviewed durable authority.
The proposal issue stores the candidate program text and checklist for scope,
trigger, approval gate, escalation, and later GitHub Actions enforcement; the
channel action itself does not call a model, edit `.gitclaw/STANDING_ORDERS.md`,
create schedules, or mutate the repository. It queues a provider-facing
proposal link back to Slack/Telegram and keeps the source receipt body-free
with only hashes, duplicate state, notification metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels clip
--clip-id <id> --message-id <id>` saves a channel moment as a durable GitHub
clip issue without treating it as work. The clip issue holds the readable title
and notes, a provider-facing clip link is queued back to the Slack/Telegram
thread, and the source receipt stays body-free with only hashes, duplicate
state, notification metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
open-loop --loop-id <id> --message-id <id>` captures an unresolved channel
question or loose end as a durable GitHub issue without treating it as a task,
reminder, or watch yet. The open-loop issue contains the readable title,
context, and next step so normal GitHub conversation can clarify it; the source
receipt stays body-free with only loop/thread/message/title/context/next-step
hashes, byte/line counts, duplicate state, notification metadata, and delivery
gates. It does not call a model, call provider APIs, copy raw mirrored channel
bodies, or print raw loop ids, provider ids, titles, context, or next steps in
the source receipt.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels snippet
--snippet-id <id> --language <lang> --message-id <id>` saves an explicit code
or config block as a durable GitHub snippet issue. The snippet issue contains
the readable title, fenced code/config body, language, and notes so normal
GitHub review can continue there; the source receipt stays body-free with only
snippet/thread/message/title/language/body/note hashes, byte/line counts,
duplicate state, notification metadata, and delivery gates. It does not copy
the raw mirrored channel message body, call a model, call provider APIs, or
print raw snippet ids, provider ids, language values, code bodies, titles, or
notes in the source receipt.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
bookmark-message --bookmark-id <id> --message-id <id>` saves a channel message
pointer as a durable GitHub bookmark issue. This differs from `/channels
bookmark`, which is still a provider reaction alias; the bookmark-message issue
is a normal GitHub conversation surface with readable title/reason notes,
optional reference URL hash, duplicate suppression, and a provider-facing
acknowledgement queued back to Slack/Telegram.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
attachment --attachment-id <id> --message-id <id> --filename <name>` records
channel-origin file/media metadata as a durable GitHub issue. The attachment
issue holds readable filename, media type, size, checksum, and optional caption
metadata; the action does not fetch or copy file bytes, and it queues a
provider-facing attachment link back to Slack/Telegram. The source receipt
stays body-free with only hashes, duplicate state, notification metadata,
source URL hash, and delivery/fetch gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels decision
--decision-id <id> --message-id <id>` records a channel decision as a durable
GitHub issue. The decision issue holds the readable decision and rationale,
queues a provider-facing decision link back to the Slack/Telegram thread, and
keeps the source receipt body-free with hashes, duplicate state, notification
metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels digest
--digest-id <id> --message-id <id>` records a channel digest as a durable
GitHub issue. The digest issue holds the readable summary and highlights,
queues a provider-facing digest link back to the Slack/Telegram thread, and
keeps the source receipt body-free with hashes, duplicate state, notification
metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels journal
--journal-id <id> --date <date> --message-id <id>` records a dated channel
journal/log entry as a durable GitHub issue. The journal issue holds the
readable date, summary, and entry details, queues a provider-facing journal
link back to the Slack/Telegram thread, and keeps the source receipt body-free
with hashes, duplicate state, notification metadata, and delivery gates. It
does not mutate `.gitclaw/MEMORY.md`; promotion to memory stays in the reviewed
memory proposal flow.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels quote
--quote-id <id> --message-id <id>` preserves a channel quote as a durable
GitHub issue without turning it into a task or memory write. The quote issue
holds the readable quote text and context, queues a provider-facing quote link
back to Slack/Telegram, and keeps the source receipt body-free with only
hashes, counts, duplicate state, notification metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
glossary --glossary-id <id> --message-id <id>` preserves a channel term and
definition as a durable GitHub glossary-entry issue. The glossary issue holds
the readable term and definition, queues a provider-facing glossary link with
the issue link and term only, and keeps the source receipt body-free with only
hashes, counts, duplicate state, notification metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels faq
--faq-id <id> --message-id <id>` preserves a channel question and answer as a
durable GitHub FAQ issue. The FAQ issue holds the readable question and answer,
queues a provider-facing FAQ link with the issue link and question only, and
keeps the source receipt body-free with only hashes, counts, duplicate state,
notification metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
skill-note --note-id <id> --skill <name> --message-id <id>` preserves a
channel lesson about a skill as a durable GitHub skill-note issue. The note
issue holds the readable skill, title, and lesson, queues a provider-facing
skill-note link with the issue link, skill, and title only, and keeps the
source receipt body-free. It does not install skills, mutate memory, or edit
repository files; promotion to a skill change stays in the reviewed follow-up
flow.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
soul-note --note-id <id> --area <area> --message-id <id>` preserves a channel
note about high-authority context as a durable GitHub soul-note issue. The
note issue holds the readable area, title, and note, queues a provider-facing
soul-note link with the issue link, area, and title only, and keeps the source
receipt body-free. It does not write `.gitclaw/SOUL.md`, mutate memory, or edit
repository files; promotion to SOUL remains a reviewed follow-up.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
backup-note --note-id <id> --scope <scope> --message-id <id>` preserves a
channel note about backup or recovery context as a durable GitHub backup-note
issue. The note issue holds the readable scope, title, and note, queues a
provider-facing backup-note link with the issue link, scope, and title only,
and keeps the source receipt body-free. It does not fetch backup branches, read
backup payloads, restore files, mutate memory, or edit repository files;
recovery work stays in explicit reviewed follow-ups.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
memory-note --note-id <id> --target <target> --message-id <id>` preserves a
channel durable-memory observation as a GitHub memory-note issue. The note
issue holds the readable target, title, and note, queues a provider-facing
memory-note link with the issue link, target, and title only, and keeps the
source receipt body-free. It does not write `.gitclaw/MEMORY.md`, promote
memory, mutate memory, or edit repository files; memory changes stay in
explicit reviewed follow-ups.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
time-capsule --capsule-id <id> --open-after <hint> --message-id <id>`
preserves a channel future-note as a durable GitHub time-capsule issue. The
capsule issue holds the readable open-after hint, title, and message, queues a
provider-facing time-capsule link with the issue link, open-after hint, and
title only, and keeps the source receipt body-free. It does not schedule
future delivery, create reminders, call a model, or edit repository files;
turning the capsule into a reminder stays an explicit reviewed follow-up.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
tool-lesson --note-id <id> --tool <tool> --message-id <id>` preserves a
channel lesson about tool usage as a durable GitHub tool-lesson issue. The
note issue holds the readable tool, title, and lesson, queues a provider-facing
tool-lesson link with the issue link, tool, and title only, and keeps the
source receipt body-free. It does not execute tools, install tools, update
tool policy, mutate memory, or edit repository files; tool execution and tool
policy changes stay in explicit reviewed follow-ups.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
tool-result --tool <tool> --result-id <id> --status <status> --message-id
<id>` records an externally observed channel tool outcome as a durable GitHub
issue. The tool-result issue holds the readable tool name, status, optional
exit code, recorded timestamp, summary, and details so normal GitHub
conversation can continue there; the channel action itself does not execute
tools, call a model, call provider APIs, or mutate the repository. It queues a
provider-facing tool-result link back to Slack/Telegram and keeps the source
receipt body-free with only hashes, counts, duplicate state, notification
metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels idea
--idea-id <id> --message-id <id>` captures a channel-origin idea as a durable
GitHub issue. The idea issue holds the readable title and notes so the
brainstorm can turn into a task, skill, memory, tool request, or proactive
workflow in GitHub; the channel action queues a provider-facing idea link and
keeps the source receipt body-free with hashes, duplicate state, notification
metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels quest
--quest-id <id> --message-id <id>` captures an exploratory channel challenge
as a durable GitHub quest issue. The quest issue holds the readable title,
objective, first move, and win condition so a playful prompt can become a
reviewable GitHub conversation without pretending it is already a task,
playbook, or scheduled job; the channel action queues a provider-facing quest
link and keeps the source receipt body-free with hashes, duplicate state,
notification metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels ritual
--ritual-id <id> --message-id <id>` captures a recurring channel practice as a
durable GitHub ritual issue. The ritual issue holds the readable title,
cadence, trigger, practice, and review notes so a loose habit or team routine
can be discussed before becoming a standing order, reminder, proactive
workflow, skill, memory, or closed; the channel action queues a provider-facing
ritual link, creates no schedule, and keeps the source receipt body-free with
hashes, duplicate state, notification metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels pact
--pact-id <id> --message-id <id>` captures a channel-origin working agreement
as a durable GitHub pact issue. The pact issue holds the readable title,
participants, agreement, scope, and revisit notes so a norm can be discussed
before becoming a standing order, soul proposal, memory proposal, policy
change, skill, or closed; the channel action queues a provider-facing pact link
with title and participants, writes no soul or memory, mutates no policy, and
keeps the source receipt body-free with hashes, duplicate state, notification
metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels forecast
--forecast-id <id> --message-id <id>` captures a channel-origin prediction as a
durable GitHub forecast issue. The forecast issue holds the readable title,
prediction, evidence, resolution criteria, and due/review timing so the
prediction can be revisited without creating a reminder, schedule, betting
market, money/points ledger, or repo mutation; the channel action queues a
provider-facing forecast link with title and due timing and keeps the source
receipt body-free with hashes, duplicate state, notification metadata, and
delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels lore
--lore-id <id> --message-id <id>` preserves low-authority shared channel
context as a durable GitHub lore issue. The lore issue holds the readable title,
lore body, context, source, and review note so useful ambient context can be
revisited without writing SOUL.md, memory, policy, skills, or repository files;
the channel action queues a provider-facing lore link with title and review
timing and keeps the source receipt body-free with hashes, duplicate state,
notification metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels boundary
--boundary-id <id> --message-id <id>` captures channel-origin boundaries,
guardrails, constraints, or consent notes as durable GitHub boundary issues.
The boundary issue holds the readable title, boundary, scope, reason, and
review note so norms can be revisited without enforcement, allowlist changes,
pairing codes, workflow/provider-setting mutations, SOUL.md writes, memory
writes, policy mutations, skill installs, or repository files; the channel
action queues a provider-facing boundary link with title and review timing and
keeps the source receipt body-free with hashes, duplicate state, notification
metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels whiteboard
--jam-id <id> --message-id <id>` captures a messy channel brainstorm as a
durable GitHub jam issue. The jam issue holds the readable topic and seeds,
queues a provider-facing jam link back to the Slack/Telegram thread, and keeps
the source receipt body-free with hashes, duplicate state, notification
metadata, and delivery gates. The linked jam issue is where normal GitHub
Models conversation can continue with skills and tools.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels kudos
--kudos-id <id> --message-id <id>` captures channel appreciation as a durable
GitHub issue. The kudos issue holds the readable recipient and reason; the
channel action queues a provider-facing acknowledgement that can show the
recipient without leaking the reason, and keeps the source receipt body-free
with hashes, duplicate state, notification metadata, and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels retro
--retro-id <id> --message-id <id>` records a channel retrospective as a
durable GitHub issue. The retro issue holds readable title, went-well notes,
rough edges, and next steps; the channel action queues a provider-facing retro
link that shows the title without leaking the section text, and keeps the
source receipt body-free with hashes, duplicate state, notification metadata,
and delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
playbook --playbook-id <id> --message-id <id>` records a reusable procedure as
a durable GitHub playbook issue. The playbook issue holds readable title,
steps, checks, and rollback guidance; the channel action queues a
provider-facing playbook link that shows the title without leaking procedure
text, and keeps the source receipt body-free with hashes, duplicate state,
notification metadata, and delivery gates. Promotion into a skill, memory,
soul rule, tool approval, or scheduled workflow remains an explicit follow-up
inside GitHub.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels insight
--insight-id <id> --message-id <id>` records a learned observation as a durable
GitHub insight issue. The insight issue holds readable title, observation,
evidence, and recommendation; the channel action queues a provider-facing
insight link that shows the title without leaking section text, and keeps the
source receipt body-free with hashes, duplicate state, notification metadata,
and delivery gates. Promotion into memory, soul, skills, tools, or schedules
remains an explicit GitHub follow-up.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
board-card --card-id <id> --lane <lane> --message-id <id>` records a
channel-origin work card as a durable GitHub issue. The board-card issue holds
readable title, lane, optional owner, and notes; the channel action queues a
provider-facing board-card link that shows the title, lane, and owner, keeps
the source receipt body-free with hashes, and explicitly does not call a
model, mutate the repository, call provider APIs, or move cards outside
GitHub review.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
checklist --checklist-id <id> --message-id <id>` records a channel-origin
checklist as a durable GitHub issue. The checklist issue holds readable title,
checkbox items, and notes; the channel action queues a provider-facing
checklist link that shows only the title and item count, keeps the source
receipt body-free with hashes/counts, and explicitly does not call a model,
mutate the repository, call provider APIs, or change task state outside GitHub
review.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels agenda
--agenda-id <id> --message-id <id>` records a channel-origin meeting or
discussion agenda as a durable GitHub issue. The agenda issue holds readable
title, ordered agenda items, and notes; the channel action queues a
provider-facing agenda link that shows only the title and item count, keeps the
source receipt body-free with hashes/counts, and explicitly does not call a
model, mutate the repository, call provider APIs, or treat agenda items as
completed tasks outside GitHub review.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
propose-workspace --workspace-id <id> --target .gitclaw/workspaces/<name>.md
--message-id <id>` records a channel-origin workspace/context proposal as a
durable GitHub issue. The proposal issue holds readable title, target path,
proposal, and rationale; the channel action queues a provider-facing proposal
link that shows only the title and target path, keeps the source receipt
body-free with hashes, and explicitly does not call a model, mutate the
repository, or write workspace files. Any actual `.gitclaw/workspaces/*.md`
change remains a normal reviewed GitHub follow-up.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels incident
--incident-id <id> --severity <severity> --message-id <id>` captures a
channel-origin incident/escalation as a durable GitHub issue. The incident
issue holds the readable severity, title, and notes for triage; the channel
action queues a provider-facing incident link and keeps the source receipt
body-free with hashes, duplicate state, notification metadata, and delivery
gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels voice
--voice-id <id> --duration <seconds> --message-id <id>` captures a
channel-origin voice/audio note as a durable GitHub transcript issue. The
voice issue holds the readable title and transcript for follow-up; the action
queues a provider-facing voice-note link and keeps the source receipt body-free
with hashes, duration metadata, duplicate state, notification metadata, and
delivery gates. Audio URLs and provider media metadata stay hashed.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels image
--image-id <id> --width <px> --height <px> --message-id <id>` captures a
channel-origin image/photo/screenshot as a durable GitHub visual context issue.
The image issue holds the readable title and description for follow-up; the
action queues a provider-facing image-note link and keeps the source receipt
body-free with hashes, dimensions, duplicate state, notification metadata, and
delivery gates. Image URLs and provider media metadata stay hashed.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels link
--link-id <id> --url <url> --message-id <id>` captures a channel-origin link
as a durable GitHub link-card issue. The link-card issue holds the readable
title and notes plus URL hashes for follow-up; the action queues a
provider-facing link-card issue link and keeps the source receipt body-free
with hashes, duplicate state, notification metadata, and delivery gates. Link
URLs are not fetched, expanded, or echoed into the source receipt.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
access-request --access-id <id> --scope <scope> --message-id <id>` opens or
reuses a GitHub access-review issue for a channel-origin access or pairing
request. The review issue holds the readable requester, scope, requested role,
and reason, while provider user IDs and handles stay hashed. The action queues
a provider-facing review link and explicitly does not grant access, mutate
allowlists, issue pairing codes, call provider APIs, or call a model.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels platform
telegram --state running --message-id <id>` queues a provider-facing bridge
status snapshot back to the current channel thread. This is the serverless
Hermes-style `/platform` primitive: it reports the reviewed provider contract,
workflow-dispatch gateway shape, outbox/delivery path, and adapter state claim.
It explicitly does not pause/resume adapters, mutate breaker state, change home
channels, start gateways, call provider APIs, or call a model; source receipts
hash reasons, home selectors, thread IDs, and message IDs.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels browser
--message-id <id> --notify-message-id <id>` queues a provider-facing browser
readiness card back to the current channel thread. It reports reviewed browser
MCP spec count, total MCP spec count, and channel gateway/outbox workflow
presence, while keeping browser automation as a separate explicit capability.
The action does not open browser sessions, navigate pages, take screenshots,
launch browser MCP servers, execute tools, call a model, edit workflows, call
provider APIs, or mutate the repository; source receipts hash thread IDs,
message IDs, status IDs, and never include MCP spec bodies or channel bodies.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels model
--message-id <id>` queues a provider-facing model-status snapshot back to the
current channel thread. It reports the effective provider, model, fallback
count, endpoint host, read-only run mode, and default model policy so a
Slack/Telegram user can ask "what model are you running?" without leaving the
conversation. The action does not call a model, switch models, write model
configuration, mutate the repository, or call provider APIs; source receipts
hash thread IDs, source/notification message IDs, status IDs, and channel
bodies.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
availability --message-id <id>` queues a provider-facing presence/availability
card back to the current channel thread. It proves the GitHub Actions bridge
received the command and queued an outbox reply, while explicitly avoiding
provider socket probes, provider API calls, session-store liveness guesses,
model calls, workflow edits, or repository mutation; source receipts hash
thread IDs, source/notification message IDs, availability IDs, and channel
bodies.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels skills
--message-id <id>` queues a provider-facing skill-status snapshot back to the
current channel thread. It reports compact repo-local skill availability from
the current GitHub Actions checkout, including counts and enabled skill names,
while preserving progressive disclosure for full `SKILL.md` bodies. The action
does not call a model, install or update skills, contact registries, run
installer scripts, mutate the repository, or call provider APIs; source
receipts hash thread IDs, source/notification message IDs, status IDs, skill
names/paths, and channel bodies.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
skill-search <query> --message-id <id> --notify-message-id <id>` queues
provider-facing skill matches back to the current channel thread. It searches
only repo-local skill metadata and reports skill names, paths, match fields,
hashes, and requirement counts; it does not print raw skill descriptions,
`SKILL.md` bodies, search queries, channel bodies, issue bodies, prompts, or
tool outputs. The action does not call a model, contact registries, install
skills, update skills, run installers, mutate the repository, or call provider
APIs.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
skill-info <skill> --message-id <id> --notify-message-id <id>` queues one
provider-facing focused skill card back to the current channel thread. It
looks up repo-local skill metadata by name or folder and reports the skill
name, path, folder, enablement state, selected-for-turn flag, frontmatter and
description presence, size, hash, and requirement counts without printing the
description or `SKILL.md` body. The source receipt stays stricter with only
route/thread/message/info/skill/result hashes and counts. The action does not
call a model, contact registries, install or update skills, run installers,
mutate the repository, or call provider APIs.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
skill-map <skill> --map-id <id> --message-id <id> --notify-message-id <id>`
queues a provider-facing safe skill-use map back to the current channel
thread. It shows the path from skill status/search/info into reviewed
proposal, rehearsal, and skill-note commands, but it does not load full skill
bodies, install or update skills, contact registries, create proposal,
rehearsal, or note issues, call a model, mutate workflows, mutate the
repository, or call provider APIs; source receipts keep skill names, notes,
step text, provider ids, and channel bodies out of band with hashes.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
bundle-map <bundle> --map-id <id> --message-id <id> --notify-message-id <id>`
queues a provider-facing safe skill-bundle map back to the current channel
thread. It shows compact repo-local bundle metadata, resolved and missing
skill refs, and the reviewed path through bundle info/risk, skill-map,
rehearsal, and bundle proposal commands, but it does not load bundle bodies or
instructions, install skills, enable bundles, write bundle YAML, create
proposal or rehearsal issues, call a model, mutate workflows, mutate the
repository, or call provider APIs; source receipts keep bundle names, notes,
step text, provider ids, and channel bodies out of band with hashes.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
source-map <source> --map-id <id> --message-id <id> --notify-message-id <id>`
queues a provider-facing safe skill-source map back to the current channel
thread. It shows compact source-pin provenance, trust, install-mode, approval,
remote-fetch, hash, and risk metadata plus the reviewed path through source
list/info/verify/lock/update-plan/proposal commands, but it does not contact
registries, fetch remote sources, install or update skills, write source pins,
create proposal issues, call a model, mutate workflows, mutate the repository,
or call provider APIs; source receipts keep source names, refs, notes, step
text, provider ids, and channel bodies out of band with hashes.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels tools
--message-id <id>` queues a provider-facing tool-status snapshot back to the
current channel thread. It reports compact deterministic tool availability,
read-only/metadata/mutating contract counts, enabled tool names, toolset/MCP
metadata counts, prompt-visible entry counts, active output counts, validation
status, and risk status from the current GitHub Actions checkout. The action
does not execute tools, launch MCP servers, activate toolsets, call a model,
mutate the repository, or call provider APIs; source receipts hash thread IDs,
source/notification message IDs, status IDs, tool-name manifests, prompt-visible
tool manifests, active-output manifests, and channel bodies.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels whoami
--identity-id <id> --message-id <id>` queues a provider-facing identity-status
message back to the current channel thread. This is the serverless Hermes-style
`/whoami` primitive: it tells the sender the reviewed display name/role status
GitClaw can safely show, while the source receipt stores only hashes for
identity ids, provider user IDs, handles, display names, roles, notes, thread
ids, and message ids. It does not create a contact card, open an access review,
grant access, mutate allowlists, issue pairing codes, call provider APIs, or
call a model.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels contact
--contact-id <id> --role <role> --message-id <id>` opens or reuses a GitHub
contact-card issue for a channel-origin identity. The contact issue holds the
readable display name, contact role, and notes, while provider user IDs and
handles stay hashed. The action queues a provider-facing contact-card link and
explicitly does not grant access, mutate allowlists, issue pairing codes, call
provider APIs, or call a model.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels handoff
--id <id> --message-id <id>` opens or reuses a normal GitHub session handoff
issue and queues a provider-facing handoff link back to the Slack/Telegram
thread. The channel action does not call a model, copy raw channel text, or
require a server/socket; the linked handoff issue is where a normal GitHub
Models conversation resumes with model, skill, tool, and usage telemetry.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels fork
--fork-id <id> --new-thread-id <id> --message-id <id>` creates or reuses a
second GitHub-backed channel-thread issue for a branch of the same external
conversation and queues a provider-facing fork acknowledgement back to the
source Slack/Telegram thread. The source receipt stays body-free and hash-only
for source/target thread ids, message ids, fork ids, titles, and notes; the new
fork issue carries the readable fork title/notes and the raw target thread id
because it is the new channel address.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels merge
--merge-id <id> --from-thread <id> --message-id <id>` records that a fork or
secondary provider thread should converge back into the current thread. It
opens or reuses a durable `gitclaw:channel-merge` issue, queues a
provider-facing merge acknowledgement to the target thread, and keeps the
source receipt body-free and hash-only for merge ids, source/target thread
ids, message ids, titles, and notes.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
request-run <tool> --id <id> --message-id <id>` opens or reuses a reviewed
GitHub tool-run request issue and queues a provider-facing review link back to
the Slack/Telegram thread. It does not call a model, execute the tool, run
shell commands, or mutate the repository; the source receipt stays body-free
with hashes, review status, duplicate state, notification metadata, and
delivery gates.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
approval-plan <tool> --id <id> --message-id <id>` opens or reuses a normal
GitHub approval-plan issue and queues a provider-facing approval link back to
the Slack/Telegram thread. The channel action does not approve, call a model,
execute the tool, generate tool inputs, or mutate the repository; the linked
approval issue records the dry-run gate state and is where a normal GitHub
Models conversation can continue with prompt-visible tool telemetry.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
rehearse-tool <tool> --id <id> --message-id <id>` opens or reuses a normal
GitHub tool rehearsal issue and queues a provider-facing rehearsal link back to
the Slack/Telegram thread. The channel action does not call a model, execute
the tool, generate tool inputs, create a run request, or mutate the repository;
the linked rehearsal issue is where a normal GitHub Models conversation can
exercise prompt-visible tool behavior with prompt/tool/usage telemetry.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
propose-toolset --toolset-id <id> --message-id <id>` opens or reuses a normal
GitHub toolset proposal issue and queues a provider-facing proposal link back
to the Slack/Telegram thread. The proposal issue stores the readable toolset
name, purpose, proposed tool list, policy, and notes; the source receipt keeps
only hashes/counts and explicitly does not call a model, enable toolsets,
execute tools, write tool configuration, or mutate the repository.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
propose-prompt --prompt-id <id> --message-id <id>` opens or reuses a normal
GitHub prompt-pack proposal issue and queues a provider-facing proposal link
back to the Slack/Telegram thread. The proposal issue stores the readable
prompt name, purpose, draft, inputs, policy, and notes; the source receipt
keeps only hashes/counts and explicitly does not call a model, run prompt
tests, enable prompts, write prompt configuration, or mutate the repository.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
propose-bundle --bundle-id <id> --message-id <id>` opens or reuses a normal
GitHub skill-bundle proposal issue and queues a provider-facing proposal link
back to the Slack/Telegram thread. The proposal issue stores the readable
bundle name, purpose, skill list, bundle instruction, policy, and notes; the
source receipt keeps only hashes/counts and explicitly does not call a model,
install skills, enable bundles, write bundle YAML, or mutate the repository.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
propose-skill <name> --message-id <id>` opens or reuses a normal GitHub skill
proposal issue and queues a provider-facing proposal link back to the
Slack/Telegram thread. The channel action does not call a model, generate a
skill body, write proposal files, edit active `SKILL.md` files, or mutate the
repository; the linked proposal issue is where a normal GitHub Models
conversation can refine the reviewed skill proposal before a code-review
branch applies it.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
propose-soul --target <path> --id <id> --message-id <id>` opens or reuses a
normal GitHub soul proposal issue and queues a provider-facing proposal link
back to the Slack/Telegram thread. The channel action does not call a model,
generate candidate soul text, edit `.gitclaw/` files, or mutate the
repository; the linked proposal issue is where a normal GitHub Models
conversation can review the high-authority context change before a code-review
branch applies it.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
rehearse-skill <skill> --id <id> --message-id <id>` opens or reuses a normal
GitHub skill rehearsal issue and queues a provider-facing rehearsal link back
to the Slack/Telegram thread. The channel action does not call a model, install
skills, edit `SKILL.md`, or mutate the repository; the linked rehearsal issue
is where a normal GitHub Models conversation can exercise the prompt-visible
skill with usage telemetry.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
rehearse-soul --target <path> --id <id> --message-id <id>` opens or reuses a
normal GitHub soul rehearsal issue and queues a provider-facing rehearsal link
back to the Slack/Telegram thread. The channel action does not call a model,
generate candidate soul text, edit `.gitclaw/` files, or mutate the repository;
the linked rehearsal issue is where a normal GitHub Models conversation can
exercise the current high-authority context with prompt/tool/usage telemetry.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
propose-memory --target <target> --id <id> --message-id <id>` opens or reuses
a normal GitHub memory proposal issue and queues a provider-facing proposal
link back to the Slack/Telegram thread. The channel action does not call a
model, generate candidate memory, edit `.gitclaw/` memory files, or mutate the
repository; the linked proposal issue is where a normal GitHub Models
conversation can review the durable memory change before a code-review branch
applies it.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
propose-workspace --workspace-id <id> --target .gitclaw/workspaces/<name>.md
--message-id <id>` opens or reuses a normal GitHub workspace proposal issue
and queues a provider-facing proposal link back to the Slack/Telegram thread.
The channel action does not call a model, generate workspace text, edit
`.gitclaw/workspaces/*.md`, or mutate the repository; the linked proposal issue
is where a normal GitHub Models conversation can review the desired workspace
context before a code-review branch applies it.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
rehearse-memory --target <target> --id <id> --message-id <id>` opens or reuses
a normal GitHub memory rehearsal issue and queues a provider-facing rehearsal
link back to the Slack/Telegram thread. The channel action does not call a
model, generate candidate memory, edit `.gitclaw/` files, or mutate the
repository; the linked rehearsal issue is where a normal GitHub Models
conversation can exercise current prompt-visible memory with prompt/tool/usage
telemetry.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels backup
--message-id <id>` queues a provider-facing backup status snapshot back to the
Slack/Telegram thread. The channel action does not call a model, fetch the
backup branch, read backup payloads, restore files, replay GitHub API calls, or
mutate the repository; it reports the backup branch, root, schema, catalog
counts, and local backup-doc metadata with raw provider ids and repo backup
paths kept out of the source receipt.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
recovery-map --message-id <id>` queues a provider-facing recovery sequence
card back to the Slack/Telegram thread. It is the safe next-step bridge between
backup status and explicit recovery workflow creation: no backup branch fetch,
payload read, restore, rehearsal issue, restore-request issue, GitHub API
replay, model call, provider API call, or repository mutation happens in this
action.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
profile-status --message-id <id>` queues a provider-facing repo-profile
snapshot back to the Slack/Telegram thread. The channel action does not call a
model, export profiles, import profiles, switch profiles, read external agent
homes, expose raw profile/skill/memory/tool bodies, or mutate the repository;
it reports the repo-local `.gitclaw/` profile envelope as component statuses,
counts, and hashes.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
soul-status --message-id <id>` queues a provider-facing high-authority soul
snapshot back to the Slack/Telegram thread. The channel action does not call a
model, contact registries, export profiles, write soul files, expose raw
soul/identity/user/memory/tool/heartbeat bodies, or mutate the repository; it
reports the repo-local authority surface as validation state, risk state,
counts, and hashes.
`@gitclaw /channels soul-info <path> --message-id <id>
--notify-message-id <id>` queues one focused high-authority context card back
to Slack/Telegram. The provider-facing card can identify the normalized
`.gitclaw/` context path, category, source, present/required/canonical/latest
flags, byte/line counts, and file hash. The GitHub source receipt stays
stricter: only requested/normalized path hashes, counts, delivery metadata,
and disabled side-effect gates are printed, with raw context paths and raw
soul/identity/user/memory/tool/heartbeat bodies kept out of the receipt.
`@gitclaw /channels soul-spotlight <focus> --spotlight-id <id> --message-id
<id> --notify-message-id <id>` queues a deterministic provider-facing
high-authority context spotlight card back to Slack/Telegram. The card picks
one present repo-local soul/identity/user/memory/tool/heartbeat metadata
surface from safe hashes, shows the path, category, source, coverage flags,
byte/line counts, hash, and follow-up commands, and keeps raw focus text,
notes, spotlight ids, selected paths, channel bodies, issue bodies, comments,
prompts, and context bodies out of the source receipt. It does not call a
model, execute tools, contact registries, export profiles, write soul or
memory, call provider APIs, use external randomness, mutate workflows, or
mutate the repository.
`@gitclaw /channels soul-risk --message-id <id> --notify-message-id <id>`
queues a provider-facing high-authority persistent-state risk card back to
Slack/Telegram. The provider card includes repo-local risk and validation
counts, per-context metadata, file hashes, risk codes, severities, and line
hashes; the GitHub receipt keeps raw context paths, raw ids, raw channel
bodies, raw file bodies, prompts, and tool outputs out of band, and records
that no model, registry, profile export, soul write, memory write, provider
delivery, or repository mutation happened.
`@gitclaw /channels soul-search <query> --message-id <id>
--notify-message-id <id>` searches the repo-local high-authority context
surface and queues provider-facing recall metadata back to Slack/Telegram. It
reports query hashes, context paths, categories, line numbers, scores, file
hashes, line hashes, and duplicate status without printing raw search queries,
soul/identity/user/memory/tool/heartbeat bodies, channel bodies, issue bodies,
comment bodies, prompts, or tool outputs.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
memory-status --message-id <id>` queues a provider-facing durable-memory
snapshot back to the Slack/Telegram thread. The channel action does not call a
model, write memory, promote memory in the background, access external
providers, include embedding vectors, expose raw memory/issue/comment/prompt/
session bodies, or mutate the repository; it reports long-term and dated
memory counts, chronology, validation, and risk state as provider-safe hashes
and counts.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
rehearse-backup --id <id> --message-id <id>` opens or reuses a normal GitHub
backup recovery rehearsal issue and queues a provider-facing rehearsal link
back to the Slack/Telegram thread. The channel action does not call a model,
read backup payloads, restore files, replay GitHub API calls, or mutate the
repository; the linked rehearsal issue is where a normal GitHub Models
conversation can exercise recovery procedures with prompt/tool/usage telemetry.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
restore-request --id <id> --message-id <id>` opens or reuses a normal GitHub
backup restore request issue and queues a provider-facing restore-review link
back to the Slack/Telegram thread. The channel action does not call a model,
read backup payloads, restore files, replay GitHub API calls, or mutate the
repository; the linked restore request issue records the dry-run
verify/coverage/drill/restore-plan/manifest commands and keeps any real
restore behind explicit human approval.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels
rehearse-checkpoint --target HEAD~1 --id <id> --message-id <id>` opens or
reuses a normal GitHub checkpoint rollback rehearsal issue and queues a
provider-facing rehearsal link back to the Slack/Telegram thread. The channel
action does not call a model, print raw diffs, print file bodies, run
`git reset`, run `git clean`, run checkout mutations, or mutate the repository;
the linked rehearsal issue records inspect-only checkpoint/rollback commands
for normal GitHub Models conversation before any reviewed recovery branch.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels remind
--reminder-id <id> --message-id <id> --at <RFC3339-or-date>` creates or reuses
a normal GitHub reminder issue with a `not_before` due gate, queues a
provider-facing reminder link back to the mirrored thread, and keeps the source
receipt body-free. Scheduled GitHub Actions can later use the reminder issue as
the canonical wake-up lane without a socket or webhook.
Inside a channel-created task, watch, standing-order proposal, backup restore
request, checkpoint rehearsal, clip, attachment, decision, digest, idea, quest,
ritual, pact, forecast, lore, retro, playbook, insight, board card, checklist, agenda, toolset proposal, workspace
proposal, incident, voice, image, link, access request, contact, or reminder
issue, `@gitclaw /channels done --message-id <id>`
closes the GitHub artifact issue and queues a provider-facing acknowledgement
back to the original
mirrored Slack/Telegram thread. The artifact receipt reports hashes, close
status, notification queue metadata, and delivery gates without printing
artifact IDs, thread IDs, message IDs, titles, notes, or channel message
bodies.
The live proactive-report, proactive-list, and proactive-schedule harnesses use
the same two-proof shape for scheduled work: body-free workflow/prompt metadata
first, then a normal GitHub Models repo-reader/search follow-up.
`gitclaw proactive enqueue --notify-route <route>` and
`--notify-routes <a,b>` make proactive jobs channel-useful without adding a
server: after a due proactive issue exists, GitClaw queues one body-safe
Slack/Telegram notification per reviewed route. The workflow/CLI output reports
only counts and short hashes, the channel body points to the GitHub issue, and
provider delivery remains delegated to `gitclaw channel-outbox` plus
`gitclaw channel-delivery`. The live proactive channel-notify harness proves the
enqueue path, duplicate suppression, outbox visibility, prompt non-leakage, and
a real GitHub Models repo-reader/search follow-up on the proactive issue.
The live prompt-report and prompt-list harnesses now use that gate for prompt
diagnostics: prompt size, hash, truncation, context, skill, and tool metadata
stay body-free, then a normal GitHub Models repo-reader/search follow-up proves
prompt inspection has not replaced real model/tool execution.
The live prompt-context harness applies the same rule to the exact context
snapshot: context paths, selected skill paths, tool-output names, hashes, and
the prompt-context identity stay body-free, then a normal GitHub Models
repo-reader/search follow-up proves the snapshot corresponds to real model/tool
execution.
The live tools-report and tools-list harnesses apply the same rule to the tool
surface: tool contracts, gate state, validation, and active-output hashes stay
body-free, then a normal GitHub Models repo-reader/search follow-up proves real
prompt-visible tool usage.
The live security-audit harness aggregates OpenClaw-style operator security
posture across config, policy, sandbox, channels, tools, skills, plugins, and
secrets without a model call, then requires a real GitHub Models
repo-reader/search follow-up so the audit surface does not replace normal
inference and tool grounding.
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
The prompt uses fixed labels and a token-prefix guard so small hosted models
copy the search-result token rather than restating the search phrase.

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
scripts/e2e/github-backup-freshness.sh
scripts/e2e/github-backup-continuity.sh
scripts/e2e/github-backup-list.sh
scripts/e2e/github-backup-timeline.sh
scripts/e2e/github-backup-info.sh
scripts/e2e/github-backup-catalog-report.sh
scripts/e2e/github-backup-snapshot.sh
scripts/e2e/github-backup-search.sh
scripts/e2e/github-backup-export-jsonl.sh
scripts/e2e/github-memory-rehearse-issue.sh
scripts/e2e/github-backup-rehearse-issue.sh
scripts/e2e/github-backup-restore-request-issue.sh
scripts/e2e/github-backup-restore-request-channel-notify.sh
scripts/e2e/github-channel-backup-status-slash.sh
scripts/e2e/github-channel-recovery-map-slash.sh
scripts/e2e/github-channel-tool-map-slash.sh
scripts/e2e/github-channel-backup-search-slash.sh
scripts/e2e/github-channel-backup-spotlight-slash.sh
scripts/e2e/github-channel-backup-timeline-slash.sh
scripts/e2e/github-channel-backup-freshness-slash.sh
scripts/e2e/github-channel-backup-continuity-slash.sh
scripts/e2e/github-channel-backup-info-slash.sh
scripts/e2e/github-channel-checkpoint-status-slash.sh
scripts/e2e/github-channel-availability-slash.sh
scripts/e2e/github-channel-profile-status-slash.sh
scripts/e2e/github-channel-soul-status-slash.sh
scripts/e2e/github-channel-soul-info-slash.sh
scripts/e2e/github-channel-soul-spotlight-slash.sh
scripts/e2e/github-channel-soul-risk-slash.sh
scripts/e2e/github-channel-soul-search-slash.sh
scripts/e2e/github-channel-memory-status-slash.sh
scripts/e2e/github-channel-memory-search-slash.sh
scripts/e2e/github-channel-skill-search-slash.sh
scripts/e2e/github-channel-skill-info-slash.sh
scripts/e2e/github-channel-skill-spotlight-slash.sh
scripts/e2e/github-channel-skill-map-slash.sh
scripts/e2e/github-channel-bundle-map-slash.sh
scripts/e2e/github-channel-source-map-slash.sh
scripts/e2e/github-channel-research-spotlight-slash.sh
scripts/e2e/github-channel-research-map-slash.sh
scripts/e2e/github-channel-tool-search-slash.sh
scripts/e2e/github-channel-tool-info-slash.sh
scripts/e2e/github-channel-tool-spotlight-slash.sh
scripts/e2e/github-channel-backup-rehearsal-slash.sh
scripts/e2e/github-channel-backup-restore-request-slash.sh
scripts/e2e/github-channel-checkpoint-rehearsal-slash.sh
scripts/e2e/github-agents-catalog-report.sh
scripts/e2e/github-agents-provenance-report.sh
scripts/e2e/github-agents-risk-report.sh
scripts/e2e/github-nodes-catalog-report.sh
scripts/e2e/github-nodes-risk-report.sh
scripts/e2e/github-artifacts-catalog-report.sh
scripts/e2e/github-artifacts-risk-report.sh
scripts/e2e/github-checkpoints-catalog-report.sh
scripts/e2e/github-rollback-preview-report.sh
scripts/e2e/github-checkpoints-rehearse-issue.sh
scripts/e2e/github-checkpoints-risk-report.sh
scripts/e2e/github-context-risk-report.sh
scripts/e2e/github-prompt-pack-report.sh
scripts/e2e/github-prompt-context-report.sh
scripts/e2e/github-prompt-cache-report.sh
scripts/e2e/github-prompt-compression-report.sh
scripts/e2e/github-prompt-risk-report.sh
scripts/e2e/github-diffs-risk-report.sh
scripts/e2e/github-heartbeat.sh
scripts/e2e/github-heartbeat-report.sh
scripts/e2e/github-heartbeat-risk-report.sh
scripts/e2e/github-hooks-report.sh
scripts/e2e/github-hooks-catalog-report.sh
scripts/e2e/github-hooks-risk-report.sh
scripts/e2e/github-hooks-provenance-report.sh
scripts/e2e/github-memory-catalog-report.sh
scripts/e2e/github-memory-snapshot-report.sh
scripts/e2e/github-memory-provenance-report.sh
scripts/e2e/github-memory-timeline-report.sh
scripts/e2e/github-memory-risk-report.sh
scripts/e2e/github-memory-remember-issue.sh
scripts/e2e/github-memory-remember-channel-notify.sh
scripts/e2e/github-migration-risk-report.sh
scripts/e2e/github-model-catalog-report.sh
scripts/e2e/github-research-catalog-report.sh
scripts/e2e/github-model-usage-report.sh
scripts/e2e/github-model-cost-report.sh
scripts/e2e/github-model-risk-report.sh
scripts/e2e/github-skills-proposal-plan-report.sh
scripts/e2e/github-skills-propose-issue.sh
scripts/e2e/github-skills-propose-channel-notify.sh
scripts/e2e/github-skills-sources-propose-issue.sh
scripts/e2e/github-skills-sources-propose-channel-notify.sh
scripts/e2e/github-skills-rehearse-issue.sh
scripts/e2e/github-skills-proposals-report.sh
scripts/e2e/github-skills-refresh-plan-report.sh
scripts/e2e/github-skills-snapshot-report.sh
scripts/e2e/github-skills-sources-report.sh
scripts/e2e/github-skills-sources-info-report.sh
scripts/e2e/github-skills-sources-search-report.sh
scripts/e2e/github-skills-sources-lock-report.sh
scripts/e2e/github-skills-sources-update-plan-report.sh
scripts/e2e/github-skills-sources-verify-report.sh
scripts/e2e/github-skills-sources-provenance-report.sh
scripts/e2e/github-skills-runtime-report.sh
scripts/e2e/github-skills-catalog-report.sh
scripts/e2e/github-skills-install-plan-report.sh
scripts/e2e/github-skills-upgrade-plan-report.sh
scripts/e2e/github-bundles-catalog-report.sh
scripts/e2e/github-bundles-search-report.sh
scripts/e2e/github-bundles-provenance-report.sh
scripts/e2e/github-bundles-risk-report.sh
scripts/e2e/github-bundles-rehearse-issue.sh
scripts/e2e/github-channel-bundle-proposal-slash.sh
scripts/e2e/github-channel-bundle-map-slash.sh
scripts/e2e/github-channel-source-map-slash.sh
scripts/e2e/github-orders-risk-report.sh
scripts/e2e/github-policy-risk-report.sh
scripts/e2e/github-approvals-catalog-report.sh
scripts/e2e/github-approvals-provenance-report.sh
scripts/e2e/github-approvals-risk-report.sh
scripts/e2e/github-secrets-risk-report.sh
scripts/e2e/github-plugins-risk-report.sh
scripts/e2e/github-plugins-mcp-report.sh
scripts/e2e/github-profile-catalog-report.sh
scripts/e2e/github-profile-provenance-report.sh
scripts/e2e/github-profile-search-report.sh
scripts/e2e/github-profile-diff-report.sh
scripts/e2e/github-profile-snapshot-report.sh
scripts/e2e/github-profile-risk-report.sh
scripts/e2e/github-channel-message.sh
scripts/e2e/github-channels-info-report.sh
scripts/e2e/github-proactive.sh
scripts/e2e/github-proactive-init.sh
scripts/e2e/github-proactive-not-before.sh
scripts/e2e/github-proactive-channel-notify.sh
scripts/e2e/github-proactive-report.sh
scripts/e2e/github-proactive-list-report.sh
scripts/e2e/github-proactive-schedule-report.sh
scripts/e2e/github-proactive-chain-report.sh
scripts/e2e/github-proactive-info-report.sh
scripts/e2e/github-proactive-risk-report.sh
scripts/e2e/github-session-catalog-report.sh
scripts/e2e/github-session-provenance.sh
scripts/e2e/github-session-tools.sh
scripts/e2e/github-session-skills.sh
scripts/e2e/github-session-usage.sh
scripts/e2e/github-session-trajectory.sh
scripts/e2e/github-session-compaction.sh
scripts/e2e/github-session-resume.sh
scripts/e2e/github-session-handoff-issue.sh
scripts/e2e/github-session-risk-report.sh
scripts/e2e/github-session-status-report.sh
scripts/e2e/github-session-stats-report.sh
scripts/e2e/github-session-coverage.sh
scripts/e2e/github-skills-provenance-report.sh
scripts/e2e/github-soul-catalog-report.sh
scripts/e2e/github-soul-snapshot-report.sh
scripts/e2e/github-soul-provenance-report.sh
scripts/e2e/github-soul-propose-issue.sh
scripts/e2e/github-soul-propose-channel-notify.sh
scripts/e2e/github-soul-rehearse-issue.sh
scripts/e2e/github-sandbox-risk-report.sh
scripts/e2e/github-tasks-ledger-report.sh
scripts/e2e/github-tasks-risk-report.sh
scripts/e2e/github-security-audit-report.sh
scripts/e2e/github-tools-catalog-report.sh
scripts/e2e/github-tools-snapshot-report.sh
scripts/e2e/github-tools-toolsets-report.sh
scripts/e2e/github-tools-toolsets-info-report.sh
scripts/e2e/github-tools-exposure-report.sh
scripts/e2e/github-tools-defer-plan-report.sh
scripts/e2e/github-tools-boundary-report.sh
scripts/e2e/github-tools-approval-plan-report.sh
scripts/e2e/github-tools-map-report.sh
scripts/e2e/github-tools-rehearse-issue.sh
scripts/e2e/github-tools-run-request-issue.sh
scripts/e2e/github-tools-run-request-channel-notify.sh
scripts/e2e/github-tools-risk-report.sh
scripts/e2e/github-workspace-catalog-report.sh
scripts/e2e/github-workspace-risk-report.sh
scripts/e2e/github-channels-risk-report.sh
scripts/e2e/github-channels-report.sh
scripts/e2e/github-channels-list-report.sh
scripts/e2e/github-channels-verify-report.sh
scripts/e2e/github-channel-ingest.sh
scripts/e2e/github-channel-state.sh
scripts/e2e/github-channel-state-workflow.sh
scripts/e2e/github-channel-gateway-workflow.sh
scripts/e2e/github-channel-send-workflow.sh
scripts/e2e/github-channel-send-route-workflow.sh
scripts/e2e/github-channel-send-slash.sh
scripts/e2e/github-channel-probe-slash.sh
scripts/e2e/github-channel-broadcast-slash.sh
scripts/e2e/github-channel-invite-slash.sh
scripts/e2e/github-channel-room-slash.sh
scripts/e2e/github-channel-huddle-slash.sh
scripts/e2e/github-channel-poll-slash.sh
scripts/e2e/github-channel-rollcall-slash.sh
scripts/e2e/github-channel-roll-slash.sh
scripts/e2e/github-channel-choose-slash.sh
scripts/e2e/github-channel-this-or-that-slash.sh
scripts/e2e/github-channel-mood-slash.sh
scripts/e2e/github-channel-room-pulse-slash.sh
scripts/e2e/github-channel-quick-replies-slash.sh
scripts/e2e/github-channel-status-wheel-slash.sh
scripts/e2e/github-channel-sticker-slash.sh
scripts/e2e/github-channel-toast-slash.sh
scripts/e2e/github-channel-postcard-slash.sh
scripts/e2e/github-channel-timer-slash.sh
scripts/e2e/github-channel-bingo-slash.sh
scripts/e2e/github-channel-riddle-slash.sh
scripts/e2e/github-channel-haiku-slash.sh
scripts/e2e/github-channel-soundtrack-slash.sh
scripts/e2e/github-channel-story-dice-slash.sh
scripts/e2e/github-channel-arcade-slash.sh
scripts/e2e/github-channel-coach-slash.sh
scripts/e2e/github-channel-nudge-slash.sh
scripts/e2e/github-channel-constellation-slash.sh
scripts/e2e/github-channel-mission-control-slash.sh
scripts/e2e/github-channel-cockpit-slash.sh
scripts/e2e/github-channel-palette-slash.sh
scripts/e2e/github-channel-compass-slash.sh
scripts/e2e/github-channel-mode-slash.sh
scripts/e2e/github-channel-warmup-slash.sh
scripts/e2e/github-channel-icebreaker-slash.sh
scripts/e2e/github-channel-spark-slash.sh
scripts/e2e/github-channel-dock-slash.sh
scripts/e2e/github-channel-browser-status-slash.sh
scripts/e2e/github-channel-session-search-slash.sh
scripts/e2e/github-channel-status-slash.sh
scripts/e2e/github-channel-edit-slash.sh
scripts/e2e/github-channel-topic-slash.sh
scripts/e2e/github-channel-activity-slash.sh
scripts/e2e/github-channel-reaction-slash.sh
scripts/e2e/github-channel-pin-slash.sh
scripts/e2e/github-channel-reply-slash.sh
scripts/e2e/github-channel-deliverable-slash.sh
scripts/e2e/github-channel-task-slash.sh
scripts/e2e/github-channel-watch-slash.sh
scripts/e2e/github-channel-standing-order-proposal-slash.sh
scripts/e2e/github-channel-clip-slash.sh
scripts/e2e/github-channel-open-loop-slash.sh
scripts/e2e/github-channel-snippet-slash.sh
scripts/e2e/github-channel-attachment-slash.sh
scripts/e2e/github-channel-decision-slash.sh
scripts/e2e/github-channel-digest-slash.sh
scripts/e2e/github-channel-journal-slash.sh
scripts/e2e/github-channel-time-capsule-slash.sh
scripts/e2e/github-channel-quote-slash.sh
scripts/e2e/github-channel-glossary-slash.sh
scripts/e2e/github-channel-faq-slash.sh
scripts/e2e/github-channel-skill-note-slash.sh
scripts/e2e/github-channel-soul-note-slash.sh
scripts/e2e/github-channel-backup-note-slash.sh
scripts/e2e/github-channel-memory-note-slash.sh
scripts/e2e/github-channel-tool-lesson-slash.sh
scripts/e2e/github-channel-tool-result-slash.sh
scripts/e2e/github-channel-idea-slash.sh
scripts/e2e/github-channel-quest-slash.sh
scripts/e2e/github-channel-ritual-slash.sh
scripts/e2e/github-channel-pact-slash.sh
scripts/e2e/github-channel-forecast-slash.sh
scripts/e2e/github-channel-lore-slash.sh
scripts/e2e/github-channel-boundary-slash.sh
scripts/e2e/github-channel-jam-slash.sh
scripts/e2e/github-channel-kudos-slash.sh
scripts/e2e/github-channel-retro-slash.sh
scripts/e2e/github-channel-playbook-slash.sh
scripts/e2e/github-channel-insight-slash.sh
scripts/e2e/github-channel-board-card-slash.sh
scripts/e2e/github-channel-checklist-slash.sh
scripts/e2e/github-channel-agenda-slash.sh
scripts/e2e/github-channel-workspace-proposal-slash.sh
scripts/e2e/github-channel-incident-slash.sh
scripts/e2e/github-channel-voice-slash.sh
scripts/e2e/github-channel-image-slash.sh
scripts/e2e/github-channel-link-slash.sh
scripts/e2e/github-channel-bookmark-slash.sh
scripts/e2e/github-channel-fork-slash.sh
scripts/e2e/github-channel-merge-slash.sh
scripts/e2e/github-channel-access-request-slash.sh
scripts/e2e/github-channel-contact-slash.sh
scripts/e2e/github-channel-session-handoff-slash.sh
scripts/e2e/github-tools-run-cancel.sh
scripts/e2e/github-channel-tool-run-request-slash.sh
scripts/e2e/github-channel-tool-approval-plan-slash.sh
scripts/e2e/github-channel-tool-rehearsal-slash.sh
scripts/e2e/github-channel-toolset-proposal-slash.sh
scripts/e2e/github-channel-prompt-proposal-slash.sh
scripts/e2e/github-channel-skill-proposal-slash.sh
scripts/e2e/github-channel-soul-proposal-slash.sh
scripts/e2e/github-channel-skill-rehearsal-slash.sh
scripts/e2e/github-channel-soul-rehearsal-slash.sh
scripts/e2e/github-channel-memory-proposal-slash.sh
scripts/e2e/github-channel-memory-rehearsal-slash.sh
scripts/e2e/github-channel-checkpoint-rehearsal-slash.sh
scripts/e2e/github-channel-reminder-slash.sh
scripts/e2e/github-channel-done-slash.sh
scripts/e2e/github-channel-delivery-workflow.sh
scripts/e2e/github-channel-outbox-workflow.sh
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
The base proactive harness now verifies the whole proactive lifecycle: generic
workflow dispatch creates or reuses a visible issue, duplicate slots stay
idempotent, and the same issue then continues with a normal GitHub Models
repo-reader/search follow-up.
The heartbeat-report harness now checks the body-free scheduled heartbeat
inventory and then posts a normal GitHub Models repo-reader/search follow-up,
so `/heartbeat` changes prove both operator visibility and regular
conversation continuity.
The heartbeat runtime harness now does the same after a real workflow-dispatch
heartbeat and duplicate-slot check, proving scheduled wakeups can hand the
issue back to ordinary `@gitclaw` conversation with repo-reader/search
grounding.
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
The rollback-preview harness exercises `@gitclaw /rollback diff HEAD~1` against
the real checked-out repository, checks the body-free numstat/path-hash preview
and disabled restore gates, then requires a real GitHub Models repo-reader
follow-up with prompt, skill, tool, and usage telemetry.
The checkpoint-rehearse harness opens a rollback rehearsal issue from
`@gitclaw /checkpoints rehearse`, checks duplicate suppression and disabled
reset/clean/checkout gates, runs the local checkpoint preview commands, and
then continues on the rehearsal issue with a real GitHub Models
repo-reader/search follow-up.
The commands-report harness does the same for `/help`: the catalog stays a
body-free deterministic capability index, then a model-backed repo-reader/search
follow-up proves the help surface has not replaced ordinary inference and tool
grounding.
The runs-report harness now applies that proof to the current-turn ledger:
issue-visible run provenance stays body-free and read-only, then a normal
GitHub Models repo-reader/search follow-up proves the live run path still
executes with prompt, skill, tool, and usage telemetry.
The toolset-info harness focuses that same proof on one repo-reviewed toolset
profile: it verifies activation and mutation gates, keeps reviewed guidance
body-free, then requires a real GitHub Models repo-reader/search follow-up.
The skill-source-info harness mirrors that contract for one reviewed source
pin: it checks no-registry/no-fetch/no-install/no-mutation gates and hash-only
metadata, then requires a real GitHub Models repo-reader/search follow-up.
The skill-source-verify harness checks all reviewed source pins as a
ClawHub/Hermes-inspired trust envelope: source-pin hashes, source-ref hashes,
current skill hashes, registry/fetch/install gates, and then a real GitHub
Models repo-reader/search follow-up.
The skill-source-search harness checks body-free progressive-disclosure search
over reviewed source-pin metadata, then requires the same real GitHub Models
repo-reader/search follow-up so deterministic search cannot mask broken LLM
tool grounding.
The skill-source-lock harness checks the derived reproducibility lock for
reviewed source pins, including stale/unpinned/missing counts and aggregate
hashes, then requires a real GitHub Models repo-reader/search follow-up.
The skill-source-update-plan harness checks the no-fetch/manual-review update
planner for source pins, then requires a real GitHub Models repo-reader/search
follow-up.
The skills-propose harness covers the action side of Skill Workshop: a trusted
`@gitclaw /skills propose <name>` turn opens or reuses a GitHub proposal issue,
keeps source text out of receipts and proposal issue bodies, suppresses
duplicate proposal requests, and then continues with a real GitHub Models
repo-reader/search follow-up.
The skills-propose channel-notify harness proves the same review queue can
also notify a reviewed Slack/Telegram route with `--notify-route`, queue a
metadata-safe outbound channel message, suppress duplicate notifications,
expose the pending provider work through `channel-outbox`, and then continue
with a real GitHub Models repo-reader/search follow-up.
The skills-source-propose harness covers external provenance intake: a trusted
`@gitclaw /skills sources propose <name> --source <ref>` turn opens or reuses a
labeled skill-source proposal issue, hashes the source ref instead of copying
it, suppresses duplicate source-pin requests, and then continues on the
proposal issue itself with a real GitHub Models repo-reader/search follow-up.
The skills-source-propose channel-notify harness extends that intake path with
`--notify-route`: it queues a metadata-safe Slack/Telegram notification for
the source-pin review issue, suppresses duplicate outbound comments, exposes
pending provider work through `channel-outbox`, and still runs the live GitHub
Models repo-reader/search follow-up on the proposal issue itself.
The skills-rehearse harness covers the conversation side of Skill Workshop: a
trusted `@gitclaw /skills rehearse <name> --id <id>` turn opens or reuses a
GitHub rehearsal issue, keeps source text and skill bodies out of receipts,
suppresses duplicate rehearsal requests, and then continues on the rehearsal
issue itself with a real GitHub Models repo-reader/search follow-up.
The bundle-rehearse harness applies that conversation-lane pattern to
Hermes-style task profiles: a trusted `@gitclaw /bundles rehearse <name> --id
<id>` turn opens or reuses a GitHub rehearsal issue, keeps source text, bundle
YAML, bundle instructions, and skill bodies out of receipts, suppresses
duplicate rehearsal requests, and then continues on the rehearsal issue itself
with a real GitHub Models repo-reader/search follow-up.
The memory-remember harness applies the same action pattern to durable memory:
a trusted `@gitclaw /memory remember --target long-term --id <id>` turn opens
or reuses a GitHub memory proposal issue, keeps candidate/source text out of
receipts and proposal bodies, suppresses duplicate proposal requests, and then
continues with a real GitHub Models repo-reader/search follow-up.
The memory-remember channel-notify harness proves the same durable-memory
review queue can notify a reviewed Slack/Telegram route with `--notify-route`,
queue a metadata-safe outbound channel message, suppress duplicate
notifications, expose pending provider work through `channel-outbox`, and then
continue with a real GitHub Models repo-reader/search follow-up.
The memory-rehearse harness covers the conversation side of durable memory: a
trusted `@gitclaw /memory rehearse --target long-term --id <id>` turn opens or
reuses a GitHub rehearsal issue, keeps source text and target memory bodies out
of receipts, suppresses duplicate rehearsal requests, and then continues on the
rehearsal issue itself with a real GitHub Models repo-reader/search follow-up.
The soul-propose harness applies the same review queue to high-authority
identity/profile context: a trusted `@gitclaw /soul propose --target soul --id
<id>` turn opens or reuses a GitHub soul proposal issue, keeps source and
candidate text out of issue-visible receipts, suppresses duplicate proposal
requests, and then continues with a real GitHub Models repo-reader/search
follow-up.
The soul-propose channel-notify harness proves that the same review queue can
also notify a reviewed Slack/Telegram route with `--notify-route`, queue a
metadata-safe outbound channel message, suppress duplicate notifications, expose
the pending provider work through `channel-outbox`, and then continue with a
real GitHub Models repo-reader/search follow-up.
The soul-rehearse harness covers the conversation side of high-authority
context: a trusted `@gitclaw /soul rehearse --target soul --id <id>` turn opens
or reuses a GitHub rehearsal issue, keeps source and target bodies out of
receipts, suppresses duplicate rehearsal requests, and then continues on the
rehearsal issue itself with a real GitHub Models repo-reader/search follow-up.
The tools-run-request harness applies the review-issue pattern to tool
execution requests: `@gitclaw /tools request-run <name> --id <id>` opens or
reuses a dedicated GitHub request issue, keeps source/tool bodies out of
receipts and request bodies, suppresses duplicate requests, and then continues
with a real GitHub Models repo-reader/search follow-up.
The tools-run-cancel harness covers the terminal review-decision path:
`@gitclaw /tools cancel-run --id <id>` posts a durable cancellation marker on
the reviewed request issue, closes it, keeps source/request ids out of the
source receipt, treats repeats after close as not-found-or-closed, and then
continues with a real GitHub Models repo-reader/search follow-up.
The tools-rehearse harness covers the conversation side of tools:
`@gitclaw /tools rehearse <name> --id <id>` opens or reuses a labeled GitHub
rehearsal issue, keeps source/tool inputs and outputs out of receipts,
suppresses duplicate rehearsal requests, and then continues on the rehearsal
issue itself with a real GitHub Models repo-reader/search follow-up.
The tools-run-request channel-notify harness proves the same review issue can
also notify a reviewed Slack/Telegram route with `--notify-route`, queue a
metadata-safe channel outbound message, suppress duplicate notifications, expose
the pending provider work through `channel-outbox`, and then continue with a
real GitHub Models repo-reader/search follow-up.
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
The channel-send workflow harness proves GitHub-originated outbound channel
messages: workflow-dispatch queues a `gitclaw:channel-outbound` comment,
duplicates are suppressed, outbox exposes it as pending provider work, delivery
receipts suppress retries, and a follow-up issue comment still makes a real
GitHub Models/tool call.
The channel-send route workflow harness proves named routes are executable:
workflow-dispatch provides only `route`, `message_id`, and `body`, GitClaw
resolves `.gitclaw/channels/routes.yaml`, queues the outbound comment, suppresses
duplicates, exposes pending outbox work without bodies, and then runs a real
GitHub Models repo-reader/search follow-up on the same issue.
The channel-send slash harness proves channels are usable from ordinary GitHub
conversation too: `@gitclaw /channels send` resolves a named route, queues an
outbound channel comment, posts a body-free receipt, suppresses duplicate
message IDs from a later comment, exposes pending outbox work without bodies,
and then runs a real GitHub Models repo-reader/search follow-up.
The channel-probe slash harness proves reviewed routes can be tested without
copying arbitrary source text: `@gitclaw /channels probe` queues a
deterministic route probe, exposes it through outbox without bodies, records a
delivery receipt through the delivery workflow, suppresses duplicate probes,
keeps route/thread/message/probe data out of receipts, and then runs a real
GitHub Models repo-reader/search follow-up.
The backup-rehearse issue harness proves recovery can become its own GitHub
conversation lane: `@gitclaw /backup rehearse` opens a labeled rehearsal issue,
verifies the real `gitclaw-backups` branch with coverage/drill/restore-plan,
suppresses duplicate rehearsal requests, and then runs a real GitHub Models
repo-reader/search follow-up on the rehearsal issue.
The backup-restore-request harness proves recovery approval can also become its
own GitHub conversation lane: `@gitclaw /backup restore-request` opens a
labeled review issue, verifies the real `gitclaw-backups` branch with
verify/coverage/drill/restore-plan/manifest, suppresses duplicate restore
requests, and then runs a real GitHub Models repo-reader/search follow-up on
the restore-request issue.
The backup-restore-request channel-notify harness proves the same recovery
approval lane can notify a reviewed Telegram/Slack route, queue exactly one
metadata-safe outbound channel message, expose pending provider work through
`channel-outbox`, suppress duplicate notifications, and then run a real GitHub
Models repo-reader/search follow-up on the restore-request issue.
The channel-broadcast slash harness fans one source issue out to multiple
reviewed routes, verifies one outbound queue item per route, checks duplicate
broadcast suppression, keeps route names and outbound bodies out of the source
receipt, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-invite slash harness shares a live GitHub issue to multiple
reviewed routes, verifies the queued provider invite body on each channel issue,
checks duplicate invite suppression, keeps raw routes/notes/titles out of the
source receipt, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-huddle slash harness creates or reuses a dedicated GitHub huddle
issue, labels it for normal GitClaw conversation, invites multiple reviewed
routes through the provider queue, checks duplicate huddle suppression, and then
continues on the huddle issue with a real GitHub Models repo-reader/search
follow-up.
The channel-poll slash harness creates or reuses a dedicated GitHub poll issue,
labels it for normal GitClaw conversation, invites multiple reviewed routes
through the provider queue, checks duplicate poll suppression, keeps question
and option text out of the source receipt, and then continues on the poll issue
with a real GitHub Models repo-reader/search follow-up.
The channel-poll-vote slash harness creates a real GitHub poll, replies from a
routed mirrored channel issue with a channel-origin vote, checks duplicate vote
suppression, keeps choice/voter/note text out of the source receipt, verifies
metadata-only outbox discovery, and then continues on the poll issue with a
real GitHub Models repo-reader/search follow-up.
The channel-rollcall slash harness creates or reuses a dedicated GitHub
rollcall issue, labels it for normal GitClaw conversation, invites multiple
reviewed routes through the provider queue, checks duplicate check-in prompt
suppression, keeps prompt/instruction text out of the source receipt, and then
continues on the rollcall issue with a real GitHub Models repo-reader/search
follow-up.
The channel-roll slash harness proves mirrored channel issues can do tiny
interactive work without a resident socket: a channel-ingested issue receives
`@gitclaw /channels roll`, queues one provider-visible deterministic dice/coin
result, exposes it through metadata-only outbox, suppresses duplicate roll
notifications, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-choose slash harness extends that tiny-interaction path to bounded
oracle answers: a channel-ingested issue receives `@gitclaw /channels oracle`,
queues one provider-visible deterministic answer from the static deck, exposes
it through metadata-only outbox, suppresses duplicate `fortune` notifications,
and then runs a real GitHub Models repo-reader/search follow-up.
The channel-this-or-that slash harness adds a two-option conversation starter:
a channel-ingested issue receives `@gitclaw /channels this-or-that`, queues one
provider-visible A/B prompt with deterministic lean metadata, exposes it through
metadata-only outbox, suppresses duplicate `wyr` notifications, proves no model
call, external randomness, provider API call, workflow edit, or repository
mutation happened, and then runs a real GitHub Models repo-reader/search
follow-up.
The channel-mood slash harness keeps mirrored channel threads more alive than
reports: a channel-ingested issue receives `@gitclaw /channels mood`, queues one
provider-visible presence update with an optional note, exposes it through
metadata-only outbox, suppresses duplicate mood notifications, and then runs a
real GitHub Models repo-reader/search follow-up.
The channel-room-pulse slash harness gives a Slack/Telegram thread a tiny
heartbeat: a channel-ingested issue receives `@gitclaw /channels room-pulse`,
queues one provider-visible marker-count pulse with a suggested next command,
exposes it through metadata-only outbox, suppresses duplicate room-pulse
notifications, proves no raw issue/comment body exposure, task/reminder
creation, provider API call, model call, workflow edit, or repository mutation
happened, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-quick-replies slash harness adds a provider-facing command-chip
launcher: a channel-ingested issue receives `@gitclaw /channels quick-replies`,
queues one provider-visible reply-chip card for a lane, exposes it through
metadata-only outbox, suppresses duplicate reply-chip notifications, proves no
command execution, artifact/task/reminder creation, model call, provider API
call, workflow edit, skill install, tool execution, or repository mutation
happened, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-status-wheel slash harness adds a tiny deterministic spin for team
posture: a channel-ingested issue receives `@gitclaw /channels status-wheel`,
queues one provider-visible status and micro-action for a lane, exposes it
through metadata-only outbox, suppresses duplicate status-wheel notifications,
proves no model call, external randomness, command execution, artifact/task/
reminder creation, provider API call, workflow edit, status persistence, skill
install, tool execution, or repository mutation happened, and then runs a real
GitHub Models repo-reader/search follow-up.
The channel-riddle slash harness adds a small channel-native puzzle lane:
a channel-ingested issue receives `@gitclaw /channels riddle`, queues one
provider-visible riddle, hint, and answer from a bounded static deck, exposes
it through metadata-only outbox, suppresses duplicate riddle notifications,
proves no model call, external randomness, game-state persistence, score
tracking, provider API call, workflow edit, skill install, tool execution, or
repository mutation happened, and then runs a real GitHub Models
repo-reader/search follow-up.
The channel-fortune-cookie slash harness adds a chat-native tiny ritual:
a channel-ingested issue receives `@gitclaw /channels fortune-cookie`, queues
one provider-visible fortune, next prompt, and lucky number from a bounded
static deck, exposes it through metadata-only outbox, suppresses duplicate
cookie notifications, proves no model call, external randomness, command
execution, artifact/task/reminder creation, provider API call, workflow edit,
skill install, tool execution, or repository mutation happened, and then runs a
real GitHub Models repo-reader/search follow-up.
The channel-sticker slash harness adds a provider-facing flourish lane without
media side effects: a channel-ingested issue receives `@gitclaw /channels
sticker`, queues one provider-visible sticker card with an optional note,
exposes it through metadata-only outbox, suppresses duplicate sticker
notifications, proves no model/image/media/upload/provider-API/repository
mutation was performed, and then runs a real GitHub Models repo-reader/search
follow-up.
The channel-toast slash harness adds a lightweight celebration lane that is
more chat-native than a report and less durable than a kudos issue: a
channel-ingested issue receives `@gitclaw /channels toast`, queues one
provider-visible toast with an optional reason, exposes it through
metadata-only outbox, suppresses duplicate toast notifications, proves no
kudos issue/model/provider-API/workflow/repository mutation was performed, and
then runs a real GitHub Models repo-reader/search follow-up.
The channel-postcard slash harness adds a tiny scene-card lane for Slack and
Telegram: a channel-ingested issue receives `@gitclaw /channels postcard`,
queues one provider-visible postcard with an optional caption, exposes it
through metadata-only outbox, suppresses duplicate postcard notifications,
proves no model/image/media/provider-API/workflow/repository mutation was
performed, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-timer slash harness adds a lightweight timebox lane that stays in
chat instead of becoming a scheduled reminder: a channel-ingested issue
receives `@gitclaw /channels timer`, queues one provider-visible timer card
with an optional label/note, exposes it through metadata-only outbox,
suppresses duplicate timer notifications, proves no reminder issue/scheduled
workflow/provider-timer/model/provider-API/repository mutation was performed,
and then runs a real GitHub Models repo-reader/search follow-up.
The channel-soundtrack slash harness adds a tiny chat-native walkout-music lane:
a channel-ingested issue receives `@gitclaw /channels soundtrack`, queues one
provider-visible three-track card with an optional note, exposes it through
metadata-only outbox, suppresses duplicate soundtrack notifications, proves no
model/media/audio/provider-API/workflow/repository mutation was performed, and
then runs a real GitHub Models repo-reader/search follow-up.
The channel-story-dice slash harness adds a tiny channel-native prompt-game
lane: a channel-ingested issue receives `@gitclaw /channels story-dice`, queues
one provider-visible four-die prompt card with an optional note, exposes it
through metadata-only outbox, suppresses duplicate story-dice notifications,
proves no model/randomness/media/provider-API/workflow/repository mutation was
performed, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-arcade slash harness adds a tiny play-menu lane for Slack and
Telegram: a channel-ingested issue receives `@gitclaw /channels arcade`, queues
one provider-visible bounded move menu with an optional note, exposes it
through metadata-only outbox, suppresses duplicate arcade notifications from
the `play-menu` alias, proves no model/dynamic-play/randomness/game-state/
score/provider-API/workflow/policy/schedule/repository mutation or command/
skill/tool execution was performed, and then runs a real GitHub Models
repo-reader/search follow-up.
The channel-coach slash harness adds repo-aware next-move cards to Slack and
Telegram: a channel-ingested issue receives `@gitclaw /channels coach skills`,
queues one provider-visible skill/tool/soul signal card with suggested
channel-native follow-up commands, exposes it through metadata-only outbox,
suppresses duplicate coach notifications, proves no command execution/skill
install/tool execution/backup payload read/soul body read/provider-API/model/
workflow/repository mutation happened, and then runs a real GitHub Models
repo-reader/search follow-up.
The channel-nudge slash harness adds a tiny attention lane that is more chat
than report: a channel-ingested issue receives `@gitclaw /channels nudge`,
queues one provider-visible target/tone/note card, exposes it through
metadata-only outbox, suppresses duplicate nudge notifications, proves no
task/reminder/watch/scheduled-workflow/provider-API/model/repository mutation
was performed, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-constellation slash harness adds a playful capability star-map
lane for Slack/Telegram: a channel-ingested issue receives `@gitclaw
/channels constellation research`, queues one provider-visible bounded
OpenClaw/Hermes-to-GitClaw research star map, exposes it through metadata-only
outbox, suppresses duplicate constellation notifications from the `star-map`
alias, proves no dynamic generation/external randomness/command execution/
skill install/tool execution/backup payload read/soul body read/memory write/
source fetch/live browse/provider-API/model/workflow/policy/schedule/
repository mutation happened, and then runs a real GitHub Models
repo-reader/search follow-up.
The channel-palette slash harness makes channel affordances discoverable inside
chat: a channel-ingested issue receives `@gitclaw /channels palette`, queues
one provider-visible shortcut card for a lane such as skills, tools, soul,
backups, or fun, exposes it through metadata-only outbox, suppresses duplicate
palette notifications, proves no command execution/skill install/tool
execution/backup payload read/soul body read/provider-API/model/repository
mutation was performed, and then runs a real GitHub Models repo-reader/search
follow-up.
The channel-compass slash harness makes channel navigation explicit: a
channel-ingested issue receives `@gitclaw /channels compass`, queues one
provider-visible safe-next-step orientation card for a focus such as skills,
tools, soul, memory, backups, or fun, exposes it through metadata-only outbox,
suppresses duplicate compass notifications, proves no command execution/skill
install/tool execution/backup payload read/soul body read/provider-API/model/
repository mutation was performed, and then runs a real GitHub Models
repo-reader/search follow-up.
The channel-mode slash harness gives Slack/Telegram threads an advisory
posture without hidden state: a channel-ingested issue receives
`@gitclaw /channels mode tool-review`, queues one provider-visible mode card,
exposes it through metadata-only outbox, suppresses duplicate mode
notifications from the `posture` alias, proves no command execution/skill
install/tool execution/backup payload read/soul body read/provider-API/model/
workflow/policy/schedule/repository mutation or durable mode persistence
happened, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-warmup slash harness now exercises the playful `@gitclaw
/channels vibe-check` alias as a chat-native entry point into the same
provider-facing conversation-starter card. It queues the default `fun` theme,
exposes the card through metadata-only outbox, suppresses a duplicate warmup
notification, proves no command execution/skill install/tool execution/backup
payload read/soul body read/provider-API/model/workflow/policy/schedule/
repository mutation happened, and then runs a real GitHub Models
repo-reader/search follow-up.
The channel-icebreaker slash harness exercises `@gitclaw /channels
icebreaker` as the low-pressure opener into the same provider-facing
conversation-starter card. It queues the bounded `icebreaker` theme, exposes
the card through metadata-only outbox, suppresses a duplicate notification
from the `kickoff` alias, proves no poll/rollcall/task/schedule/provider-API/
model/workflow/repository mutation happened, and then runs a real GitHub
Models repo-reader/search follow-up.
The channel-spark slash harness exercises `@gitclaw /channels spark` as the
brainstorming entry point into the same provider-facing conversation-starter
card. It queues the bounded `spark` theme, exposes the card through
metadata-only outbox, suppresses a duplicate spark notification from the
`brainstorm` alias, proves no dynamic prompt generation, quest/task/proposal
creation, command execution, provider API call, model call, workflow edit,
schedule, or repository mutation happened, and then runs a real GitHub Models
repo-reader/search follow-up.
The channel-session-search slash harness makes recall a channel-native action:
a channel-ingested issue receives `@gitclaw /channels session-search`, queues
provider-visible body-free search metadata from the GitHub-backed transcript,
exposes it through metadata-only outbox, suppresses duplicate recall
notifications, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-memory-search slash harness gives durable memory the same channel
shape: a channel-ingested issue receives `@gitclaw /channels memory-search`,
queues provider-visible body-free recall metadata from repo-local memory files,
exposes it through metadata-only outbox, suppresses duplicate recall
notifications, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-backup-search slash harness extends recall to durable archives: a
channel-ingested issue is first observed on the real `gitclaw-backups` branch,
then receives `@gitclaw /channels backup-search`, queues provider-visible
body-free search metadata from the fetched backup archive, exposes it through
metadata-only outbox, suppresses duplicate recall notifications, and then runs
a real GitHub Models repo-reader/search follow-up.
The channel-backup-spotlight slash harness adds a small archive-discovery card:
a channel-ingested issue is first observed on the real `gitclaw-backups` branch,
then receives `@gitclaw /channels backup-spotlight`, queues one deterministic
provider-visible backup candidate from fetched archive metadata, exposes it
through metadata-only outbox, suppresses duplicate spotlight notifications, and
then runs a real GitHub Models repo-reader/search follow-up.
The channel-backup-timeline slash harness turns durable archives into a
channel-native chronology card: a channel-ingested issue is first observed on
the real `gitclaw-backups` branch, then receives `@gitclaw /channels
backup-timeline`, queues provider-visible issue/timestamp/count/hash metadata,
exposes it through metadata-only outbox, suppresses duplicate timeline
notifications, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-backup-freshness slash harness turns the same archive into a
channel-native freshness gate: a channel-ingested issue is first observed on
the real `gitclaw-backups` branch, then receives `@gitclaw /channels
backup-freshness`, queues provider-visible latest-backup age/gate/hash
metadata, exposes it through metadata-only outbox, suppresses duplicate
freshness notifications, and then runs a real GitHub Models repo-reader/search
follow-up.
The channel-backup-continuity slash harness turns the same archive into a
channel-native continuity card: a channel-ingested issue is first observed on
the real `gitclaw-backups` branch, then receives `@gitclaw /channels
backup-continuity`, queues provider-visible issue-span/longest-gap/gate
metadata, exposes it through metadata-only outbox, suppresses duplicate
continuity notifications, and then runs a real GitHub Models repo-reader/search
follow-up.
The channel-backup-info slash harness adds the focused archive-card step: a
channel-ingested issue is first observed on the real `gitclaw-backups` branch,
then receives `@gitclaw /channels backup-info`, queues one provider-visible
backup metadata card from the fetched backup archive, exposes it through
metadata-only outbox, suppresses duplicate backup-info notifications, and then
runs a real GitHub Models repo-reader/search follow-up.
The channel-RSVP slash harness creates or reuses a dedicated GitHub RSVP issue,
labels it for normal GitClaw conversation, invites multiple reviewed routes
through the provider queue, checks duplicate RSVP suppression, keeps event
metadata and details out of the source receipt, and then continues on the RSVP
issue with a real GitHub Models repo-reader/search follow-up.
The channel-reaction slash harness proves mirrored channel issues can do tiny
social acknowledgements without composing a full reply: a channel-ingested issue
receives `@gitclaw /channels react`, queues one structured provider reaction,
exposes it through metadata-only outbox, records delivery, suppresses duplicate
reactions, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-status slash harness proves mirrored channel issues can send
provider-visible progress without a socket: a channel issue receives
`@gitclaw /channels status`, queues one structured status update, exposes it
through metadata-only outbox, records delivery, suppresses duplicate status
ids, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-edit slash harness proves mirrored channel issues can update a
provider message without a resident gateway socket: a channel issue receives
`@gitclaw /channels edit`, queues one structured provider edit, exposes it
through metadata-only outbox, records delivery, suppresses duplicate edit ids,
and then runs a real GitHub Models repo-reader/search follow-up.
The channel-topic slash harness proves mirrored channel issues can queue a
provider thread title/topic update without renaming the GitHub issue or calling
provider APIs: a channel issue receives `@gitclaw /channels topic`, queues one
structured provider topic update, exposes it through metadata-only outbox,
records delivery, suppresses duplicate topic ids, and then runs a real GitHub
Models repo-reader/search follow-up.
The channel-activity slash harness proves mirrored channel issues can queue a
provider-native activity signal without a socket: a channel issue receives
`@gitclaw /channels activity`, queues one structured activity event, exposes it
through metadata-only outbox, records delivery, suppresses duplicate activity
ids, and then runs a real GitHub Models repo-reader/search follow-up.
The channel-pin slash harness proves the same operator-console path has a
one-word shortcut: a channel-ingested issue receives `@gitclaw /channels pin`,
queues a default `pushpin` provider reaction, exposes and delivers it through
the existing reaction outbox, suppresses duplicate pins, and then runs a real
GitHub Models repo-reader/search follow-up.
The channel-reply slash harness proves mirrored channel issues can act as
operator consoles: a channel-ingested issue receives `@gitclaw /channels reply`,
queues an outbound message back onto the same thread, suppresses duplicate
message IDs, records delivery through the channel-delivery workflow, and then
runs a real GitHub Models repo-reader/search follow-up.
The channel-deliverable slash harness turns the operator console into an
outbound file/link surface: a channel-ingested issue receives `@gitclaw
/channels deliverable`, queues one `gitclaw:channel-deliverable` comment,
checks metadata-only and include-body outbox behavior, records delivery through
the channel-delivery workflow, checks duplicate suppression, and then continues
on the channel issue with a real GitHub Models repo-reader/search follow-up.
The channel-task slash harness turns the operator console into a work intake
surface: a channel-ingested issue receives `@gitclaw /channels task`, creates
or reuses a normal GitHub task issue, queues a provider-facing task link back
to the mirrored thread, checks duplicate task and notification suppression,
exposes the task-link notification through metadata-only outbox, and then
continues on the task issue with a real GitHub Models repo-reader/search
follow-up.
The channel-watch slash harness turns the operator console into a proactive
watch intake surface: a channel-ingested issue receives `@gitclaw /channels
watch`, creates or reuses a GitHub watch issue with cadence metadata, queues a
provider-facing watch link back to the mirrored thread, checks duplicate watch
and notification suppression, exposes the watch-link notification through
metadata-only outbox, and then continues on the watch issue with a real GitHub
Models repo-reader/search follow-up.
The channel-standing-order-proposal slash harness turns the operator console
into a reviewed authority-intake surface: a channel-ingested issue receives
`@gitclaw /channels propose-order`, creates or reuses a GitHub standing-order
proposal issue, queues a provider-facing proposal link back to the mirrored
thread, checks duplicate proposal and notification suppression, exposes the
proposal-link notification through metadata-only outbox, and then continues on
the proposal issue with a real GitHub Models repo-reader/search follow-up.
The channel-clip slash harness turns the operator console into a save-for-later
surface: a channel-ingested issue receives `@gitclaw /channels clip`, creates
or reuses a durable GitHub clip issue, queues a provider-facing clip link back
to the mirrored thread, checks duplicate clip and notification suppression,
exposes the clip-link notification through metadata-only outbox, and then
continues on the clip issue with a real GitHub Models repo-reader/search
follow-up.
The channel-open-loop slash harness turns the operator console into a loose-end
capture surface: a channel-ingested issue receives `@gitclaw /channels
open-loop`, creates or reuses a durable GitHub open-loop issue, queues a
provider-facing open-loop link back to the mirrored thread, checks duplicate
loop and notification suppression, proves source receipts/outbox do not copy
context, next steps, or raw provider ids, and then continues on the open-loop
issue with a real GitHub Models repo-reader/search follow-up.
The channel-snippet slash harness turns the operator console into a code/config
capture surface: a channel-ingested issue receives `@gitclaw /channels
snippet` with an explicit fenced snippet, creates or reuses a durable GitHub
snippet issue, queues a provider-facing snippet link back to the mirrored
thread, checks duplicate snippet and notification suppression, proves source
receipts/outbox do not copy snippet bodies or raw provider ids, and then
continues on the snippet issue with a real GitHub Models repo-reader/search
follow-up.
The channel-attachment slash harness turns the operator console into a
metadata-safe file/media intake surface: a channel-ingested issue receives
`@gitclaw /channels attachment`, creates or reuses a durable GitHub attachment
metadata issue, queues a provider-facing attachment link back to the mirrored
thread, checks duplicate attachment and notification suppression, proves source
URLs and file bytes are not copied into receipts/outbox, and then continues on
the attachment issue with a real GitHub Models repo-reader/search follow-up.
The channel-decision slash harness turns the operator console into a decision
log: a channel-ingested issue receives `@gitclaw /channels decision`, creates
or reuses a durable GitHub decision issue, queues a provider-facing decision
link back to the mirrored thread, checks duplicate decision and notification
suppression, exposes the decision-link notification through metadata-only
outbox, and then continues on the decision issue with a real GitHub Models
repo-reader/search follow-up.
The channel-digest slash harness turns the operator console into a digest
surface: a channel-ingested issue receives `@gitclaw /channels digest`, creates
or reuses a durable GitHub digest issue, queues a provider-facing digest link
back to the mirrored thread, checks duplicate digest and notification
suppression, exposes the digest-link notification through metadata-only outbox,
and then continues on the digest issue with a real GitHub Models
repo-reader/search follow-up.
The channel-journal slash harness turns the operator console into a durable
field-note surface: a channel-ingested issue receives `@gitclaw /channels
journal`, creates or reuses a dated GitHub journal issue, queues a
provider-facing journal link back to the mirrored thread, checks duplicate
journal and notification suppression, exposes the journal-link notification
through metadata-only outbox, and then continues on the journal issue with a
real GitHub Models repo-reader/search follow-up.
The channel-quote slash harness turns the operator console into a quote
capture surface: a channel-ingested issue receives `@gitclaw /channels quote`,
creates or reuses a durable GitHub quote issue, queues a provider-facing quote
link back to the mirrored thread, checks duplicate quote and notification
suppression, exposes the quote-link notification through metadata-only outbox,
and then continues on the quote issue with a real GitHub Models
repo-reader/search follow-up.
The channel-glossary slash harness turns the operator console into a glossary
capture surface: a channel-ingested issue receives `@gitclaw /channels
glossary`, creates or reuses a durable GitHub glossary issue, queues a
provider-facing glossary link with only the visible term back to the mirrored
thread, checks duplicate glossary and notification suppression, exposes the
glossary-link notification through metadata-only outbox, and then continues on
the glossary issue with a real GitHub Models repo-reader/search follow-up.
The channel-faq slash harness turns the operator console into a FAQ capture
surface: a channel-ingested issue receives `@gitclaw /channels faq`, creates
or reuses a durable GitHub FAQ issue, queues a provider-facing FAQ link with
only the visible question back to the mirrored thread, checks duplicate FAQ and
notification suppression, exposes the FAQ-link notification through
metadata-only outbox, and then continues on the FAQ issue with a real GitHub
Models repo-reader/search follow-up.
The channel-skill-note slash harness turns the operator console into a skill
lesson capture surface: a channel-ingested issue receives `@gitclaw /channels
skill-note`, creates or reuses a durable GitHub skill-note issue, queues a
provider-facing skill-note link with only the visible skill and title back to
the mirrored thread, checks duplicate skill-note and notification suppression,
exposes the skill-note notification through metadata-only outbox, and then
continues on the skill-note issue with a real GitHub Models repo-reader/search
follow-up.
The channel-soul-note slash harness turns the operator console into a
high-authority context capture surface: a channel-ingested issue receives
`@gitclaw /channels soul-note`, creates or reuses a durable GitHub soul-note
issue, queues a provider-facing soul-note link with only the visible area and
title back to the mirrored thread, checks duplicate soul-note and notification
suppression, exposes the soul-note notification through metadata-only outbox,
and then continues on the soul-note issue with a real GitHub Models
repo-reader/search follow-up.
The channel-backup-note slash harness turns the operator console into a
recovery-context capture surface: a channel-ingested issue receives
`@gitclaw /channels backup-note`, creates or reuses a durable GitHub
backup-note issue, queues a provider-facing backup-note link with only the
visible scope and title back to the mirrored thread, checks duplicate
backup-note and notification suppression, exposes the backup-note notification
through metadata-only outbox, and then continues on the backup-note issue with
a real GitHub Models repo-reader/search follow-up.
The channel-memory-note slash harness turns the operator console into a
durable-memory observation capture surface: a channel-ingested issue receives
`@gitclaw /channels memory-note`, creates or reuses a durable GitHub
memory-note issue, queues a provider-facing memory-note link with only the
visible target and title back to the mirrored thread, checks duplicate
memory-note and notification suppression, exposes the memory-note notification
through metadata-only outbox, and then continues on the memory-note issue with
a real GitHub Models repo-reader/search follow-up.
The channel-time-capsule slash harness turns the operator console into a
future-note capture surface: a channel-ingested issue receives `@gitclaw
/channels time-capsule`, creates or reuses a durable GitHub time-capsule
issue, queues a provider-facing time-capsule link with only the visible
open-after hint and title back to the mirrored thread, checks duplicate capsule
and notification suppression, exposes the time-capsule notification through
metadata-only outbox, and then continues on the time-capsule issue with a real
GitHub Models repo-reader/search follow-up.
The channel-tool-lesson slash harness turns the operator console into a tool
guidance capture surface: a channel-ingested issue receives `@gitclaw
/channels tool-lesson`, creates or reuses a durable GitHub tool-lesson issue,
queues a provider-facing tool-lesson link with only the visible tool and title
back to the mirrored thread, checks duplicate tool-lesson and notification
suppression, exposes the tool-lesson notification through metadata-only outbox,
and then continues on the tool-lesson issue with a real GitHub Models
repo-reader/search follow-up.
The channel-idea slash harness turns the operator console into an idea intake
surface: a channel-ingested issue receives `@gitclaw /channels idea`, creates
or reuses a durable GitHub idea issue, queues a provider-facing idea link back
to the mirrored thread, checks duplicate idea and notification suppression,
exposes the idea-link notification through metadata-only outbox, and then
continues on the idea issue with a real GitHub Models repo-reader/search
follow-up.
The channel-quest slash harness turns the operator console into a lightweight
challenge lane: a channel-ingested issue receives `@gitclaw /channels quest`,
creates or reuses a durable GitHub quest issue with readable title, objective,
first move, and win condition, queues a provider-facing quest link back to the
mirrored thread, checks duplicate quest and notification suppression, exposes
the quest-link notification through metadata-only outbox, and then continues
on the quest issue with a real GitHub Models repo-reader/search follow-up.
The channel-ritual slash harness turns the operator console into a lightweight
practice lane: a channel-ingested issue receives `@gitclaw /channels ritual`,
creates or reuses a durable GitHub ritual issue with readable title, cadence,
trigger, practice, and review notes, queues a provider-facing ritual link with
title and cadence back to the mirrored thread, checks duplicate ritual and
notification suppression, proves no scheduled workflow/reminder/standing order
was created, exposes the ritual-link notification through metadata-only
outbox, and then continues on the ritual issue with a real GitHub Models
repo-reader/search follow-up.
The channel-pact slash harness turns the operator console into a lightweight
agreement lane: a channel-ingested issue receives `@gitclaw /channels pact`,
creates or reuses a durable GitHub pact issue with readable title,
participants, agreement, scope, and revisit notes, queues a provider-facing
pact link with title and participants back to the mirrored thread, checks
duplicate pact and notification suppression, proves no soul/memory/policy/
standing-order mutation was performed, exposes the pact-link notification
through metadata-only outbox, and then continues on the pact issue with a real
GitHub Models repo-reader/search follow-up.
The channel-forecast slash harness turns the operator console into a prediction
lane: a channel-ingested issue receives `@gitclaw /channels forecast`, creates
or reuses a durable GitHub forecast issue with readable title, prediction,
evidence, resolution criteria, and due/review timing, queues a provider-facing
forecast link with title and due timing back to the mirrored thread, checks
duplicate forecast and notification suppression, proves no reminder/schedule/
betting-market/money-or-points/repository mutation was performed, exposes the
forecast-link notification through metadata-only outbox, and then continues on
the forecast issue with a real GitHub Models repo-reader/search follow-up.
The channel-lore slash harness turns the operator console into a shared-context
lane: a channel-ingested issue receives `@gitclaw /channels lore`, creates or
reuses a durable GitHub lore issue with readable title, lore body, context,
source, and review note, queues a provider-facing lore link with title and
review timing back to the mirrored thread, checks duplicate lore and
notification suppression, proves no soul/memory/policy/skill/repository
mutation was performed, exposes the lore-link notification through
metadata-only outbox, and then continues on the lore issue with a real GitHub
Models repo-reader/search follow-up.
The channel-boundary slash harness turns the operator console into a reviewable
norms lane: a channel-ingested issue receives `@gitclaw /channels boundary`,
creates or reuses a durable GitHub boundary issue with readable title,
boundary, scope, reason, and review note, queues a provider-facing boundary
link with title and review timing back to the mirrored thread, checks duplicate
boundary and notification suppression, proves no enforcement, allowlist,
pairing-code, workflow/provider-setting, soul, memory, policy, skill, or
repository mutation was performed, exposes the boundary-link notification
through metadata-only outbox, and then continues on the boundary issue with a
real GitHub Models repo-reader/search follow-up.
The channel-jam slash harness turns the operator console into a brainstorm
lane: a channel-ingested issue receives `@gitclaw /channels whiteboard`, creates or
reuses a durable GitHub jam issue with the readable topic and seeds, queues a
provider-facing jam link back to the mirrored thread, checks duplicate jam and
notification suppression, exposes the jam-link notification through
metadata-only outbox, and then continues on the jam issue with a real GitHub
Models repo-reader/search follow-up.
The channel-kudos slash harness turns the operator console into an
appreciation lane: a channel-ingested issue receives `@gitclaw /channels
kudos`, creates or reuses a durable GitHub kudos issue with the readable
recipient and reason, queues a provider-facing acknowledgement back to the
mirrored thread, checks duplicate kudos and notification suppression, exposes
the acknowledgement through metadata-only outbox, and then continues on the
kudos issue with a real GitHub Models repo-reader/search follow-up.
The channel-retro slash harness turns the operator console into a retrospective
lane: a channel-ingested issue receives `@gitclaw /channels retro`, creates or
reuses a durable GitHub retro issue with readable title, went-well notes, rough
edges, and next steps, queues a provider-facing retro link back to the mirrored
thread, checks duplicate retro and notification suppression, exposes the link
through metadata-only outbox, and then continues on the retro issue with a real
GitHub Models repo-reader/search follow-up.
The channel-playbook slash harness turns the operator console into a reusable
procedure lane: a channel-ingested issue receives `@gitclaw /channels
playbook`, creates or reuses a durable GitHub playbook issue with readable
title, steps, checks, and rollback guidance, queues a provider-facing playbook
link back to the mirrored thread, checks duplicate playbook and notification
suppression, exposes the link through metadata-only outbox, and then continues
on the playbook issue with a real GitHub Models repo-reader/search follow-up.
The channel-insight slash harness turns the operator console into an
observation lane: a channel-ingested issue receives `@gitclaw /channels
insight`, creates or reuses a durable GitHub insight issue with readable title,
observation, evidence, and recommendation, queues a provider-facing insight
link back to the mirrored thread, checks duplicate insight and notification
suppression, exposes the link through metadata-only outbox, and then continues
on the insight issue with a real GitHub Models repo-reader/search follow-up.
The channel-board-card slash harness turns a mirrored Slack/Telegram thread
into a lightweight GitHub-native work board: a channel-ingested issue receives
`@gitclaw /channels board-card`, creates or reuses a durable GitHub board-card
issue with readable title, lane, owner, and notes, queues a provider-facing
board-card link back to the mirrored thread, checks that no repository
mutation happened, checks duplicate card and notification suppression, exposes
the link through metadata-only outbox, and then continues on the board-card
issue with a real GitHub Models repo-reader/search follow-up.
The channel-checklist slash harness turns a mirrored Slack/Telegram thread
into a durable GitHub checklist: a channel-ingested issue receives `@gitclaw
/channels checklist`, creates or reuses a normal GitHub checklist issue with
readable title, checkbox items, and notes, queues a provider-facing checklist
link back to the mirrored thread, checks that no repository mutation happened,
checks duplicate checklist and notification suppression, exposes the link
through metadata-only outbox, and then continues on the checklist issue with a
real GitHub Models repo-reader/search follow-up.
The channel-agenda slash harness turns a mirrored Slack/Telegram thread into a
durable GitHub agenda: a channel-ingested issue receives `@gitclaw /channels
agenda`, creates or reuses a normal GitHub agenda issue with readable title,
ordered agenda items, and notes, queues a provider-facing agenda link back to
the mirrored thread, checks that no repository mutation happened, checks
duplicate agenda and notification suppression, exposes the link through
metadata-only outbox, and then continues on the agenda issue with a real GitHub
Models repo-reader/search follow-up.
The channel-workspace-proposal slash harness turns the operator console into a
workspace-context intake lane: a channel-ingested issue receives `@gitclaw
/channels propose-workspace`, creates or reuses a durable GitHub workspace
proposal issue with readable title, target path, proposal, and rationale,
queues a provider-facing proposal link back to the mirrored thread, checks
that no workspace file or repository mutation happened, checks duplicate
proposal and notification suppression, exposes the link through metadata-only
outbox, and then continues on the proposal issue with a real GitHub Models
repo-reader/search follow-up.
The channel-incident slash harness turns the operator console into an
escalation intake surface: a channel-ingested issue receives `@gitclaw
/channels incident`, creates or reuses a durable GitHub incident issue with a
reviewable severity, queues a provider-facing incident link back to the
mirrored thread, checks duplicate incident and notification suppression,
exposes the incident-link notification through metadata-only outbox, and then
continues on the incident issue with a real GitHub Models repo-reader/search
follow-up.
The channel-voice slash harness turns the operator console into a voice-note
transcript surface: a channel-ingested issue receives `@gitclaw /channels
voice`, creates or reuses a durable GitHub voice transcript issue, queues a
provider-facing voice-note link back to the mirrored thread, checks duplicate
voice-note and notification suppression, exposes the voice-link notification
through metadata-only outbox, and then continues on the voice issue with a real
GitHub Models repo-reader/search follow-up.
The channel-image slash harness turns the operator console into a visual
context surface: a channel-ingested issue receives `@gitclaw /channels image`,
creates or reuses a durable GitHub image issue, queues a provider-facing
image-note link back to the mirrored thread, checks duplicate image and
notification suppression, exposes the image-link notification through
metadata-only outbox, and then continues on the image issue with a real GitHub
Models repo-reader/search follow-up.
The channel-link slash harness turns the operator console into a link-card
surface: a channel-ingested issue receives `@gitclaw /channels link`, creates
or reuses a durable GitHub link-card issue, queues a provider-facing link-card
issue link back to the mirrored thread, checks duplicate link and notification
suppression, exposes the link-card notification through metadata-only outbox,
and then continues on the link issue with a real GitHub Models
repo-reader/search follow-up.
The channel-bookmark slash harness turns a channel message into saved
GitHub-native context: a channel-ingested issue receives `@gitclaw /channels
bookmark-message`, creates or reuses a durable bookmark issue, queues a
provider-facing acknowledgement back to the mirrored thread, checks duplicate
bookmark and notification suppression, exposes the bookmark notification
through metadata-only outbox, and then continues on the bookmark issue with a
real GitHub Models repo-reader/search follow-up.
The channel-fork slash harness turns a mirrored provider thread into a new
GitHub-backed channel lane: a channel-ingested issue receives
`@gitclaw /channels fork`, creates or reuses a second channel-thread issue,
queues a provider-facing fork acknowledgement back to the source thread, checks
duplicate fork and notification suppression, exposes the fork acknowledgement
through metadata-only outbox, and then continues on the forked channel issue
with a real GitHub Models repo-reader/search follow-up.
The channel-merge slash harness records convergence between channel lanes: a
channel-ingested issue receives `@gitclaw /channels merge`, creates or reuses a
durable merge issue, queues a provider-facing merge acknowledgement back to the
target thread, checks duplicate merge and notification suppression, exposes the
merge acknowledgement through metadata-only outbox, and then continues on the
merge issue with a real GitHub Models repo-reader/search follow-up.
The channel-access-request slash harness turns the operator console into an
access review surface: a channel-ingested issue receives `@gitclaw /channels
access-request`, creates or reuses a GitHub access-review issue, queues a
provider-facing review link back to the mirrored thread, checks that no access
grant, allowlist mutation, pairing code, or provider API call happened, checks
duplicate access-request and notification suppression, exposes the review-link
notification through metadata-only outbox, and then continues on the access
review issue with a real GitHub Models repo-reader/search follow-up.
The channel-contact slash harness turns the operator console into an identity
surface: a channel-ingested issue receives `@gitclaw /channels contact`,
creates or reuses a GitHub contact-card issue, queues a provider-facing
contact-card link back to the mirrored thread, checks that no access grant,
allowlist mutation, pairing code, or provider API call happened, checks
duplicate contact and notification suppression, exposes the contact-card
notification through metadata-only outbox, and then continues on the contact
issue with a real GitHub Models repo-reader/search follow-up.
The channel-platform slash harness turns the operator console into a
provider-visible bridge status surface: a channel-ingested issue receives
`@gitclaw /channels platform`, queues a provider-facing platform-status message
back to the mirrored thread, checks that no pause/resume, breaker mutation,
home-channel mutation, gateway start, provider API call, or model call happened,
checks duplicate notification suppression, exposes the platform-status
notification through metadata-only outbox, and then continues on the channel
thread itself with a real GitHub Models repo-reader/search follow-up.
The channel-model-status slash harness turns the operator console into a
provider-visible runtime transparency surface: a channel-ingested issue
receives `@gitclaw /channels model`, queues a provider-facing model-status
message back to the mirrored thread, checks that no model call, model switch,
model-config write, provider API call, or repository mutation happened, checks
duplicate notification suppression, exposes the model-status notification
through metadata-only outbox, and then continues on the channel thread itself
with a real GitHub Models repo-reader/search follow-up.
The channel-availability slash harness turns the operator console into a
provider-visible presence surface: a channel-ingested issue receives
`@gitclaw /channels availability`, queues a provider-facing availability card
back to the mirrored thread, checks that no provider socket probe, provider API
call, model call, workflow mutation, session-store liveness guess, or
repository mutation happened, checks duplicate notification suppression,
exposes the availability notification through metadata-only outbox, and then
continues on the channel thread itself with a real GitHub Models
repo-reader/search follow-up.
The channel-skill-status slash harness turns the operator console into a
provider-visible skill discovery surface: a channel-ingested issue receives
`@gitclaw /channels skills`, queues a provider-facing skill-status message back
to the mirrored thread, checks that no model call, skill install/update,
registry contact, installer run, provider API call, or repository mutation
happened, checks duplicate notification suppression, exposes the skill-status
notification through metadata-only outbox, and then continues on the channel
thread itself with a real GitHub Models repo-reader/search follow-up.
The channel-skill-search slash harness turns that discovery surface into
provider-visible skill recall: a channel-ingested issue receives `@gitclaw
/channels skill-search`, queues body-free skill metadata matches back to the
mirrored thread, checks that no model call, skill install/update, registry
contact, installer run, provider API call, or repository mutation happened,
checks duplicate notification suppression, exposes the skill-search
notification through metadata-only outbox, and then continues on the channel
thread itself with a real GitHub Models repo-reader/search follow-up.
The channel-skill-info slash harness adds the focused skill-card step: a
channel-ingested issue receives `@gitclaw /channels skill-info`, queues one
provider-facing skill metadata card back to the mirrored thread without
printing skill descriptions or `SKILL.md` bodies, checks duplicate notification
suppression, exposes the skill-info notification through metadata-only outbox,
and then continues on the channel thread itself with a real GitHub Models
repo-reader/search follow-up.
The channel-skill-spotlight slash harness adds a chat-native discovery card: a
channel-ingested issue receives `@gitclaw /channels skill-spotlight`, queues one
deterministically selected provider-facing skill card without installing skills,
contacting registries, loading skill bodies, using randomness, or calling a
model, checks duplicate notification suppression, exposes the spotlight through
metadata-only outbox, and then continues on the channel thread itself with a
real GitHub Models repo-reader/search follow-up.
The channel-skill-map slash harness adds the safe sequence card: a
channel-ingested issue receives `@gitclaw /channels skill-map`, queues a
provider-facing path from skill status/search/info to reviewed proposal,
rehearsal, and skill-note commands without installing or updating skills,
contacting registries, loading skill bodies, creating those review issues, or
calling a model, checks duplicate notification suppression, exposes the
skill-map notification through metadata-only outbox, and then continues on the
channel thread itself with a real GitHub Models repo-reader/search follow-up.
The channel-bundle-map slash harness adds the skill-bundle sequence card: a
channel-ingested issue receives `@gitclaw /channels bundle-map`, queues a
provider-facing path from bundle metadata and risk review to skill-map,
rehearsal, and proposal commands without installing skills, enabling bundles,
writing bundle YAML, loading bundle bodies/instructions, creating those review
issues, or calling a model, checks duplicate notification suppression, exposes
the bundle-map notification through metadata-only outbox, and then continues
on the channel thread itself with a real GitHub Models repo-reader/search
follow-up.
The channel-source-map slash harness adds the skill-source sequence card: a
channel-ingested issue receives `@gitclaw /channels source-map`, queues a
provider-facing path from reviewed source-pin provenance to verify, lock,
update-plan, and proposal commands without contacting registries, fetching
remote sources, installing skills, writing source pins, creating proposal
issues, or calling a model, checks duplicate notification suppression, exposes
the source-map notification through metadata-only outbox, and then continues
on the channel thread itself with a real GitHub Models repo-reader/search
follow-up.
The channel-tool-search slash harness turns tool discovery into provider-visible
capability recall: a channel-ingested issue receives `@gitclaw /channels
tool-search`, queues tool contract matches back to the mirrored thread without
executing tools or exposing raw schemas, checks duplicate notification
suppression, exposes the tool-search notification through metadata-only outbox,
and then continues on the channel thread itself with a real GitHub Models
repo-reader/search follow-up.
The channel-tool-info slash harness adds the focused describe step: a
channel-ingested issue receives `@gitclaw /channels tool-info`, queues one
provider-facing tool card back to the mirrored thread without executing tools
or exposing raw schemas, checks duplicate notification suppression, exposes the
tool-info notification through metadata-only outbox, and then continues on the
channel thread itself with a real GitHub Models repo-reader/search follow-up.
The channel-tool-spotlight slash harness adds a chat-native safe discovery
card: a channel-ingested issue receives `@gitclaw /channels tool-spotlight`,
queues one deterministic provider-facing tool spotlight from read-only/
metadata-only built-in contracts without executing tools or exposing raw
triggers/schemas, checks duplicate notification suppression, exposes the
tool-spotlight notification through metadata-only outbox, and then continues on
the channel thread itself with a real GitHub Models repo-reader/search
follow-up.
The channel-research-spotlight slash harness adds a chat-native research
discovery card: a channel-ingested issue receives `@gitclaw /channels
research-spotlight`, queues one deterministic provider-facing card from the
reviewed OpenClaw/Hermes source, pattern, and rejection catalog without source
fetches or live browsing, checks duplicate notification suppression, exposes
the research-spotlight notification through metadata-only outbox, and then
continues on the same channel issue with a real GitHub Models
repo-reader/search follow-up.
The channel-research-map slash harness adds a chat-native research sequence:
a channel-ingested issue receives `@gitclaw /channels research-map`, queues one
provider-facing research-to-GitClaw command path from the same reviewed static
catalog without source fetches, live browsing, model calls, tool execution, or
workflow mutation, checks duplicate notification suppression, exposes the
research-map notification through metadata-only outbox, and then continues on
the same channel issue with a real GitHub Models repo-reader/search follow-up.
The channel-tool-map slash harness adds the safe sequence card: a
channel-ingested issue receives `@gitclaw /channels tool-map`, queues a
provider-facing path from tool status/search/info to reviewed approval-plan,
rehearsal, and request-run commands without executing tools or creating those
review issues, checks duplicate notification suppression, exposes the tool-map
notification through metadata-only outbox, and then continues on the channel
thread itself with a real GitHub Models repo-reader/search follow-up.
The channel-tool-status slash harness turns the operator console into a
provider-visible tool discovery surface: a channel-ingested issue receives
`@gitclaw /channels tools`, queues a provider-facing tool-status message back
to the mirrored thread, checks that no tool execution, shell execution, MCP
server launch, toolset activation, model call, provider API call, or repository
mutation happened, checks duplicate notification suppression, exposes the
tool-status notification through metadata-only outbox, and then continues on the
channel thread itself with a real GitHub Models repo-reader/search follow-up.
The channel-whoami slash harness turns the operator console into a lightweight
identity-status surface: a channel-ingested issue receives `@gitclaw /channels
whoami`, queues a provider-facing identity-status message back to the mirrored
thread, checks that no contact-card issue, access-review issue, access grant,
allowlist mutation, pairing code, or provider API call happened, checks
duplicate notification suppression, exposes the identity-status notification
through metadata-only outbox, and then continues on the channel thread itself
with a real GitHub Models repo-reader/search follow-up.
The channel-session-handoff slash harness turns the operator console into a
conversation handoff surface: a channel-ingested issue receives `@gitclaw
/channels handoff`, creates or reuses a GitHub session handoff issue, queues a
provider-facing handoff link back to the mirrored thread, checks duplicate
handoff and notification suppression, exposes the handoff-link notification
through metadata-only outbox, and then continues on the handoff issue with a
real GitHub Models repo-reader/search follow-up.
The channel-tool-run-request slash harness turns the operator console into a
reviewed tool intake surface: a channel-ingested issue receives `@gitclaw
/channels request-run`, creates or reuses a GitHub tool-run request issue,
queues a provider-facing review link back to the mirrored thread, checks
duplicate request and notification suppression, exposes the review-link
notification through metadata-only outbox, and then continues on the review
issue with a real GitHub Models repo-reader/search follow-up.
The channel-tool-approval-plan slash harness turns the operator console into a
tool approval gate surface: a channel-ingested issue receives `@gitclaw
/channels approval-plan`, creates or reuses a GitHub tool approval-plan issue,
queues a provider-facing approval-plan link back to the mirrored thread,
checks duplicate plan and notification suppression, exposes the approval-link
notification through metadata-only outbox, and then continues on the approval
issue with a real GitHub Models repo-reader/search follow-up.
The channel-tool-rehearsal slash harness turns the operator console into a
tool practice surface: a channel-ingested issue receives `@gitclaw /channels
rehearse-tool`, creates or reuses a GitHub tool rehearsal issue, queues a
provider-facing rehearsal link back to the mirrored thread, checks duplicate
rehearsal and notification suppression, exposes the rehearsal-link notification
through metadata-only outbox, and then continues on the rehearsal issue with a
real GitHub Models repo-reader/search follow-up.
The channel-toolset-proposal slash harness turns the operator console into a
toolset intake surface: a channel-ingested issue receives `@gitclaw /channels
propose-toolset`, creates or reuses a GitHub toolset proposal issue, queues a
provider-facing proposal link back to the mirrored thread, checks duplicate
proposal and notification suppression, exposes the proposal-link notification
through metadata-only outbox, and then continues on the proposal issue with a
real GitHub Models repo-reader/search follow-up.
The channel-prompt-proposal slash harness turns the operator console into a
prompt-pack intake surface: a channel-ingested issue receives `@gitclaw
/channels propose-prompt`, creates or reuses a GitHub prompt proposal issue,
queues a provider-facing proposal link back to the mirrored thread, checks
duplicate proposal and notification suppression, exposes the proposal-link
notification through metadata-only outbox, and then continues on the proposal
issue with a real GitHub Models repo-reader/search follow-up.
The channel-bundle-proposal slash harness turns the operator console into a
skill-bundle intake surface: a channel-ingested issue receives `@gitclaw
/channels propose-bundle`, creates or reuses a GitHub bundle proposal issue,
queues a provider-facing proposal link back to the mirrored thread, checks
duplicate proposal and notification suppression, exposes the proposal-link
notification through metadata-only outbox, and then continues on the proposal
issue with a real GitHub Models repo-reader/search follow-up.
The channel-skill-proposal slash harness turns the operator console into a
skill intake surface: a channel-ingested issue receives `@gitclaw /channels
propose-skill`, creates or reuses a GitHub skill proposal issue, queues a
provider-facing proposal link back to the mirrored thread, checks duplicate
proposal and notification suppression, exposes the proposal-link notification
through metadata-only outbox, and then continues on the proposal issue with a
real GitHub Models repo-reader/search follow-up.
The channel-soul-proposal slash harness turns the operator console into a
high-authority-context intake surface: a channel-ingested issue receives
`@gitclaw /channels propose-soul`, creates or reuses a GitHub soul proposal
issue, queues a provider-facing proposal link back to the mirrored thread,
checks duplicate proposal and notification suppression, exposes the
proposal-link notification through metadata-only outbox, and then continues on
the proposal issue with a real GitHub Models repo-reader/search follow-up.
The channel-skill-rehearsal slash harness turns the operator console into a
skill practice surface: a channel-ingested issue receives `@gitclaw /channels
rehearse-skill`, creates or reuses a GitHub skill rehearsal issue, queues a
provider-facing rehearsal link back to the mirrored thread, checks duplicate
rehearsal and notification suppression, exposes the rehearsal-link notification
through metadata-only outbox, and then continues on the rehearsal issue with a
real GitHub Models repo-reader/search follow-up.
The channel-soul-rehearsal slash harness turns the operator console into a
high-authority-context practice surface: a channel-ingested issue receives
`@gitclaw /channels rehearse-soul`, creates or reuses a GitHub soul rehearsal
issue, queues a provider-facing rehearsal link back to the mirrored thread,
checks duplicate rehearsal and notification suppression, exposes the
rehearsal-link notification through metadata-only outbox, and then continues on
the rehearsal issue with a real GitHub Models repo-reader/search follow-up.
The channel-memory-proposal slash harness turns the operator console into a
durable-memory intake surface: a channel-ingested issue receives `@gitclaw
/channels propose-memory`, creates or reuses a GitHub memory proposal issue,
queues a provider-facing proposal link back to the mirrored thread, checks
duplicate proposal and notification suppression, exposes the proposal-link
notification through metadata-only outbox, and then continues on the proposal
issue with a real GitHub Models repo-reader/search follow-up.
The channel-memory-rehearsal slash harness turns the operator console into a
memory practice surface: a channel-ingested issue receives `@gitclaw /channels
rehearse-memory`, creates or reuses a GitHub memory rehearsal issue, queues a
provider-facing rehearsal link back to the mirrored thread, checks duplicate
rehearsal and notification suppression, exposes the rehearsal-link notification
through metadata-only outbox, and then continues on the rehearsal issue with a
real GitHub Models repo-reader/search follow-up.
The channel-backup-status slash harness turns the operator console into a
backup cockpit: a channel-ingested issue receives `@gitclaw /channels backup`,
queues a provider-facing backup status snapshot back to the mirrored thread,
checks duplicate notification suppression, exposes the backup-status
notification through metadata-only outbox, and then continues on the same
channel issue with a real GitHub Models repo-reader/search follow-up.
The channel-recovery-map slash harness turns that cockpit into a safe recovery
route card: a channel-ingested issue receives `@gitclaw /channels
recovery-map`, queues one provider-visible status/search/info/rehearsal/restore
sequence, checks duplicate notification suppression, exposes the card through
metadata-only outbox, proves no backup fetch/payload read/restore/rehearsal
issue/restore-request issue/GitHub API replay/provider API/model/repository
mutation happened, and then continues on the same channel issue with a real
GitHub Models repo-reader/search follow-up.
The channel-profile-status slash harness turns the operator console into a
repo-profile cockpit: a channel-ingested issue receives `@gitclaw /channels
profile-status`, queues a provider-facing profile snapshot back to the
mirrored thread, checks duplicate notification suppression, exposes the
profile-status notification through metadata-only outbox, and then continues
on the same channel issue with a real GitHub Models repo-reader/search
follow-up.
The channel-soul-status slash harness turns the operator console into an
agent-authority cockpit: a channel-ingested issue receives `@gitclaw /channels
soul-status`, queues a provider-facing high-authority soul snapshot back to
the mirrored thread, checks duplicate notification suppression, exposes the
soul-status notification through metadata-only outbox, and then continues on
the same channel issue with a real GitHub Models repo-reader/search follow-up.
The channel-soul-info slash harness adds focused high-authority context
lookup: a channel-ingested issue receives `@gitclaw /channels soul-info`,
queues one provider-facing `.gitclaw/` context metadata card back to the
mirrored thread, checks duplicate notification suppression, exposes the
soul-info notification through metadata-only outbox, and then continues on the
same channel issue with a real GitHub Models repo-reader/search follow-up.
The channel-soul-spotlight slash harness adds a chat-native high-authority
context discovery card: a channel-ingested issue receives `@gitclaw /channels
soul-spotlight`, queues one deterministic provider-facing context spotlight
from repo-local soul metadata without exposing raw context bodies or source
receipt paths, checks duplicate notification suppression, exposes the
soul-spotlight notification through metadata-only outbox, and then continues
on the same channel issue with a real GitHub Models repo-reader/search
follow-up.
The channel-soul-risk slash harness adds high-authority state risk cards: a
channel-ingested issue receives `@gitclaw /channels soul-risk`, queues one
provider-facing repo-local soul risk card back to the mirrored thread, checks
duplicate notification suppression, exposes the soul-risk notification through
metadata-only outbox, and then continues on the same channel issue with a real
GitHub Models repo-reader/search follow-up.
The channel-soul-search slash harness adds body-free high-authority context
recall: a channel-ingested issue receives `@gitclaw /channels soul-search`,
queues provider-facing soul/context matches back to the mirrored thread,
checks duplicate notification suppression, exposes the soul-search
notification through metadata-only outbox, and then continues on the same
channel issue with a real GitHub Models repo-reader/search follow-up.
The channel-memory-status slash harness turns the operator console into a
durable-memory cockpit: a channel-ingested issue receives `@gitclaw /channels
memory-status`, queues a provider-facing memory snapshot back to the mirrored
thread, checks duplicate notification suppression, exposes the memory-status
notification through metadata-only outbox, and then continues on the same
channel issue with a real GitHub Models repo-reader/search follow-up.
The channel-backup-rehearsal slash harness turns the operator console into a
recovery practice surface: a channel-ingested issue receives `@gitclaw
/channels rehearse-backup`, creates or reuses a GitHub backup rehearsal issue,
queues a provider-facing rehearsal link back to the mirrored thread, checks
duplicate rehearsal and notification suppression, proves the channel issue was
captured on the `gitclaw-backups` branch, exposes the rehearsal-link
notification through metadata-only outbox, and then continues on the rehearsal
issue with a real GitHub Models repo-reader/search follow-up.
The channel-backup-restore-request slash harness turns the operator console
into a recovery approval surface: a channel-ingested issue receives `@gitclaw
/channels restore-request`, creates or reuses a GitHub backup restore request
issue, queues a provider-facing restore-review link back to the mirrored
thread, checks duplicate request and notification suppression, proves the
channel issue was captured on the `gitclaw-backups` branch, exposes the
restore-review notification through metadata-only outbox, and then continues
on the restore request issue with a real GitHub Models repo-reader/search
follow-up.
The channel-checkpoint-status slash harness turns the operator console into a
rollback readiness cockpit: a channel-ingested issue receives `@gitclaw
/channels checkpoint-status`, queues a provider-facing checkpoint/rollback
metadata card back to the mirrored thread, checks duplicate notification
suppression, exposes the checkpoint-status notification through metadata-only
outbox, and then continues on the same channel issue with a real GitHub Models
repo-reader/search follow-up.
The channel-checkpoint-rehearsal slash harness turns the operator console into
a rollback practice surface: a channel-ingested issue receives `@gitclaw
/channels rehearse-checkpoint`, creates or reuses a GitHub checkpoint rollback
rehearsal issue, queues a provider-facing rehearsal link back to the mirrored
thread, checks duplicate rehearsal and notification suppression, runs
inspect-only checkpoint status/preview/risk and rollback diff commands, exposes
the rehearsal-link notification through metadata-only outbox, and then
continues on the rehearsal issue with a real GitHub Models repo-reader/search
follow-up.
The channel-reminder slash harness turns the operator console into a scheduled
nudge surface: a channel-ingested issue receives `@gitclaw /channels remind`,
creates or reuses a GitHub reminder issue with a normalized `not_before` gate,
queues a provider-facing reminder link back to the mirrored thread, checks
duplicate reminder and notification suppression, exposes the reminder-link
notification through metadata-only outbox, and then continues on the reminder
issue with a real GitHub Models repo-reader/search follow-up.
The channel-done slash harness closes the loop: a channel-created task issue
receives `@gitclaw /channels done`, GitClaw closes that artifact issue, queues
a provider-facing done acknowledgement back to the mirrored thread, checks
duplicate acknowledgement suppression, exposes the acknowledgement through
metadata-only outbox, and then continues on the channel issue with a real
GitHub Models repo-reader/search follow-up.
The channel-outbox workflow harness proves the missing outbound half of the
bridge: a real channel-ingested message gets a GitHub Models/tool reply, the
outbox exposes only pending assistant comments for provider delivery, delivery
receipts suppress retries, and a follow-up issue comment still makes a real
model/tool call.
The proactive-report, proactive-list, and proactive-schedule harnesses now
require the deterministic scheduled-job inventory to be followed by a real
issue-comment GitHub Models turn that selects `repo-reader`, exposes
`gitclaw.search_files`, and recovers a fixture token from repository search.
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
