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
@gitclaw /skills propose <name>
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
External skill provenance gets the same issue-native treatment with
`@gitclaw /skills sources propose <name> --source <ref>`: GitClaw opens or
reuses a labeled review conversation issue for a proposed
`.gitclaw/skill-sources/<name>.yaml` pin, records only source-ref/request
hashes and review paths, avoids registry fetches and installs, and requires a
later GitHub Models follow-up on that proposal issue before promotion.
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
@gitclaw /channels rollcall e2e-slack-route,e2e-telegram-route --rollcall-id <id> --message-id <id>
@gitclaw /channels status --message-id <id> --status-id <id> --state working
@gitclaw /channels edit --message-id <id> --edit-id <id>
@gitclaw /channels react --message-id <id> --reaction eyes
@gitclaw /channels pin --message-id <id>
@gitclaw /channels reply --message-id <id>
@gitclaw /channels task --task-id <id> --message-id <id>
@gitclaw /channels clip --clip-id <id> --message-id <id>
gitclaw proactive list
gitclaw proactive schedule
gitclaw proactive chain
gitclaw proactive risk
gitclaw proactive info repo-hygiene
gitclaw proactive init --name email-triage --cron "17 8 * * 1-5"
gitclaw proactive enqueue --name repo-hygiene --slot "$(date -u +%F)"
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
`@gitclaw /channels rollcall <route-a>,<route-b> --rollcall-id <id>
--message-id <id>` creates or reuses a dedicated GitHub check-in issue, writes
the prompt and instructions there, labels it for normal GitClaw conversation,
and queues provider-facing rollcall invites through the same routebook/outbox
path. It is meant for lightweight standups, attendance, status checks, and
"everyone please respond here" moments without adding a server.
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
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels task
--task-id <id> --message-id <id>` creates or reuses a normal GitHub task issue
from the channel thread, writes the human-readable title and notes there, and
queues a provider-facing task link back to the mirrored Slack/Telegram thread.
The source receipt stays body-free: it reports task/thread/message/title/notes
hashes, duplicate status, notification queue metadata, and delivery gates
without printing raw provider IDs, channel message bodies, task titles, or
task notes.
Inside a mirrored `gitclaw:channel-thread` issue, `@gitclaw /channels clip
--clip-id <id> --message-id <id>` saves a channel moment as a durable GitHub
clip issue without treating it as work. The clip issue holds the readable title
and notes, a provider-facing clip link is queued back to the Slack/Telegram
thread, and the source receipt stays body-free with only hashes, duplicate
state, notification metadata, and delivery gates.
The live proactive-report, proactive-list, and proactive-schedule harnesses use
the same two-proof shape for scheduled work: body-free workflow/prompt metadata
first, then a normal GitHub Models repo-reader/search follow-up.
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
scripts/e2e/github-migration-risk-report.sh
scripts/e2e/github-model-catalog-report.sh
scripts/e2e/github-research-catalog-report.sh
scripts/e2e/github-model-usage-report.sh
scripts/e2e/github-model-cost-report.sh
scripts/e2e/github-model-risk-report.sh
scripts/e2e/github-skills-proposal-plan-report.sh
scripts/e2e/github-skills-propose-issue.sh
scripts/e2e/github-skills-sources-propose-issue.sh
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
scripts/e2e/github-channel-status-slash.sh
scripts/e2e/github-channel-edit-slash.sh
scripts/e2e/github-channel-reaction-slash.sh
scripts/e2e/github-channel-pin-slash.sh
scripts/e2e/github-channel-reply-slash.sh
scripts/e2e/github-channel-task-slash.sh
scripts/e2e/github-channel-clip-slash.sh
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
The skills-source-propose harness covers external provenance intake: a trusted
`@gitclaw /skills sources propose <name> --source <ref>` turn opens or reuses a
labeled skill-source proposal issue, hashes the source ref instead of copying
it, suppresses duplicate source-pin requests, and then continues on the
proposal issue itself with a real GitHub Models repo-reader/search follow-up.
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
The channel-rollcall slash harness creates or reuses a dedicated GitHub
rollcall issue, labels it for normal GitClaw conversation, invites multiple
reviewed routes through the provider queue, checks duplicate check-in prompt
suppression, keeps prompt/instruction text out of the source receipt, and then
continues on the rollcall issue with a real GitHub Models repo-reader/search
follow-up.
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
The channel-task slash harness turns the operator console into a work intake
surface: a channel-ingested issue receives `@gitclaw /channels task`, creates
or reuses a normal GitHub task issue, queues a provider-facing task link back
to the mirrored thread, checks duplicate task and notification suppression,
exposes the task-link notification through metadata-only outbox, and then
continues on the task issue with a real GitHub Models repo-reader/search
follow-up.
The channel-clip slash harness turns the operator console into a save-for-later
surface: a channel-ingested issue receives `@gitclaw /channels clip`, creates
or reuses a durable GitHub clip issue, queues a provider-facing clip link back
to the mirrored thread, checks duplicate clip and notification suppression,
exposes the clip-link notification through metadata-only outbox, and then
continues on the clip issue with a real GitHub Models repo-reader/search
follow-up.
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
