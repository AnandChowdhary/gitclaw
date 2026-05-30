# OpenClaw and Hermes Landscape Research

Date: 2026-05-29

Purpose: establish the architecture, product, and security patterns behind OpenClaw, Hermes Agent, and smaller "claw" implementations before speccing this `gitclaw` project.

## Source Quality Notes

High-confidence sources:

- OpenClaw official docs and GitHub repository.
- Hermes Agent official docs and NousResearch GitHub repository.
- Primary repositories for NanoClaw, MicroClaw, NullClaw, LightClaw, MiniClaw OS, BunClaw, and Stockade.
- Recent security research and reports on always-on agent systems.

Lower-confidence sources:

- Many comparison and directory sites in this ecosystem are SEO-heavy and sometimes internally inconsistent. I treated them as discovery leads only unless they pointed to a real repository or official docs.

## Executive Read

OpenClaw and Hermes are converging on the same broad idea: a persistent, self-hosted agent process that receives instructions from many places, keeps durable state, uses tools, schedules work, and replies through the user's normal channels.

The main distinction is product center of gravity:

- OpenClaw is a broad gateway/control-plane platform. It emphasizes messaging channel breadth, multi-agent routing, workspace files, skills, plugins, scheduling, nodes, and UI/control surfaces.
- Hermes is an agent runtime and long-horizon work system. It emphasizes a terminal/TUI experience, self-improving skills, curated memory, session search, delegation, persistent goals, cron, and multiple execution backends.
- Nano/Micro/Null/Light variants are reactions against platform sprawl. They usually pick one sharp axis: inspectability, OS isolation, low resource use, or simpler code.

For `gitclaw`, the promising direction is not a full OpenClaw clone. The opportunity is a narrower Git-native agent gateway: repo-scoped, audit-heavy, worktree-isolated, durable tasks, GitHub/Slack/CLI entry points, and strong permission/provenance boundaries from day one.

## OpenClaw

### What It Is

OpenClaw is positioned as an open-source, local-first personal AI assistant and gateway. Its docs describe a self-hosted gateway that connects many chat channels to AI coding agents, with one gateway process on the user's machine or server.

Current public repo metadata seen during research:

- Repository: `openclaw/openclaw`
- Language mix: mostly TypeScript, with Swift, JavaScript, Kotlin, Shell, and CSS.
- High ecosystem gravity: hundreds of thousands of stars and many forks.
- Latest release observed: `openclaw 2026.5.27`, dated 2026-05-28.

### Core Architecture

OpenClaw's fundamental shape:

```text
Channels / clients / nodes
  -> long-lived Gateway
  -> embedded agent runtime
  -> tools, skills, memory, sessions
  -> delivery back to channel
```

Key details:

- A single long-lived Gateway owns messaging surfaces such as WhatsApp, Telegram, Slack, Discord, Signal, iMessage, and WebChat.
- Control clients connect to the Gateway over WebSocket.
- Nodes such as macOS/iOS/Android/headless devices also connect over WebSocket and declare capabilities.
- The Gateway validates protocol frames, manages provider connections, emits events, and owns singleton channel sessions.
- Remote access is expected to happen over Tailscale/VPN or SSH tunnel rather than exposing a raw unauthenticated service.

### Agent Runtime

OpenClaw runs one embedded agent runtime per Gateway, with:

- a configured workspace used as the default working directory,
- bootstrap files injected into session context,
- session transcripts stored as JSONL,
- built-in tools gated by tool policy,
- optional alternate runtimes/harnesses via plugins.

Workspace/bootstrap files matter. OpenClaw expects user-editable files such as:

- `AGENTS.md`: operating instructions and memory-like guidance.
- `SOUL.md`: persona, boundaries, tone.
- `TOOLS.md`: local tool notes.
- `IDENTITY.md`: agent name/vibe.
- `USER.md`: user profile.
- `BOOTSTRAP.md`: first-run ritual.

2026-05-29 follow-up: OpenClaw's skills documentation reinforces two useful
constraints for GitClaw. First, skills are plain directories containing
`SKILL.md`, which makes them easy to keep in git and review like code. Second,
skill loading must be gated because third-party skills are a supply-chain and
prompt-injection surface. GitClaw should therefore start with repo-local,
read-only `.gitclaw/SKILLS/*/SKILL.md` files and should not install remote
skills or let the agent edit skills automatically.

2026-05-29 skill-loading follow-up: OpenClaw's skill format uses optional YAML
frontmatter with fields such as `name`, `description`, and runtime metadata,
including an `always` flag. Hermes' skill docs emphasize progressive
disclosure: surface a searchable/indexed skill list first, inspect or load full
instructions only when needed, and run security scans before installing
third-party skills. GitClaw's repo-native cut should therefore expose a skill
index in prompt context and load full local skill bodies only when the issue
thread selects them or the skill is marked always-on.

### Memory

OpenClaw's default memory model is file-centric:

- `MEMORY.md` for compact long-term facts and preferences.
- `memory/YYYY-MM-DD.md` for daily notes and working context.
- optional `DREAMS.md` for background consolidation summaries.
- indexed recall via memory tools and pluggable backends.

This keeps state auditable and portable, but it also means prompt-injected memory is part of the security boundary. Anything that can write durable memory can affect future behavior.

2026-05-29 backup/state follow-up: OpenClaw's memory docs are explicit that
there is no hidden agent memory; the model remembers only what is saved to disk.
The workspace docs recommend treating the agent workspace as private memory and
putting files such as `AGENTS.md`, `SOUL.md`, `TOOLS.md`, `USER.md`,
`HEARTBEAT.md`, and `memory/` in git for recoverability, while keeping
credentials, auth profiles, and raw session state out of that repo. For
GitClaw, that maps cleanly to git-backed, reviewable `.gitclaw/` state and a
separate backup branch for raw issue transcript snapshots.

2026-05-29 backup-index follow-up: OpenClaw migration applies only after a
reviewed plan and verified backup, while Hermes exposes session export as a
portable artifact. GitClaw should make its backup branch similarly inspectable:
raw issue transcripts stay in per-issue JSON files, and a repo-scoped
`index.json`/`README.md` summarizes coverage without exposing every raw message.

2026-05-29 backup-report follow-up: OpenClaw's migration and migrate CLI docs
emphasize preview, secret redaction, and verified backups before applying
state changes, while Hermes' OpenClaw migration keeps pre-migration restore
points. GitClaw's serverless analogue should make backup destinations visible
from the issue itself: `@gitclaw /backup` reports the expected backup branch,
per-issue JSON path, and repo index paths, then the post-turn backup job writes
the canonical raw transcript copy.

2026-05-29 backup-verify follow-up: OpenClaw's `backup verify` command checks
that an archive has exactly one manifest, rejects traversal-style paths, and
confirms every manifest-declared payload exists. GitClaw's git-native analogue
is not a tarball, so the invariant moves to the backup branch: verify
repo-scoped `index.json`, `README.md`, canonical `issues/000000.json` paths,
schema version, counts, timestamps, and absence of unindexed issue backups
before treating the branch as restorable.

2026-05-29 backup-manifest follow-up: OpenClaw's manifest-centered backup
verification and Hermes' portable session export both point to a compact
provenance view. GitClaw should expose a local `backup manifest` command over
the fetched `gitclaw-backups` branch that lists control files, issue payload
paths, byte counts, counts, and hashes without dumping raw transcript bodies.
That gives restore reviews and mirrors a stable checklist without requiring
operators to open every raw issue JSON file.

2026-05-29 backup-export follow-up: Hermes exposes session export as JSONL.
GitClaw can get the same portability without a local SQLite store by exporting
the fetched `gitclaw-backups` branch: read the repo index and canonical issue
JSON files, then emit one JSON object per reconstructed transcript message.
Because this is raw recovery output, keep it as an explicit local CLI command,
not an issue-visible report.

2026-05-29 backup-stats follow-up: Hermes also exposes `sessions stats` for a
quick count of sessions, messages, and source platforms, while OpenClaw's
backup docs rely on a manifest plus verification result. GitClaw should expose
the git-native equivalent as `gitclaw backup stats`: verify the fetched backup
tree, summarize issue/comment/transcript/message counts, latest backup
metadata, payload bytes, and event types, and avoid printing raw titles or
bodies.

2026-05-30 backup-list follow-up: Hermes' session list and OpenClaw's backup
inspection surfaces both point to a compact index-first view before export or
restore. GitClaw should add `gitclaw backup list`: verify the fetched
`gitclaw-backups` branch, sort indexed backups by timestamp, and print issue
numbers, payload paths, timestamps, event names, label/comment/transcript
counts, and title hashes only. This keeps routine backup navigation body-free.

2026-05-29 backup-restore-plan follow-up: OpenClaw's migration/backup posture
emphasizes preview before state-changing recovery. GitClaw should copy that
separation: a local `backup restore-plan` command reads the backup branch and
prints a dry-run restore plan with counts and hashes, but makes no GitHub API
calls and does not dump raw issue/comment bodies. A future mutating restore can
require explicit approval and compare the restored issue against this plan.

2026-05-29 backup-retention follow-up: OpenClaw's backup/restore commands keep
state changes behind explicit previews, while Hermes' session lifecycle docs
make cleanup/archival pressure visible through exported session artifacts.
GitClaw's git-native equivalent should start with a non-mutating
`backup retention-plan`: verify the fetched `gitclaw-backups` branch, sort
backups by timestamp, keep the latest N, list older payloads as prune
candidates, and expose only paths, counts, timestamps, and title hashes.

2026-05-30 backup-search follow-up: OpenClaw's transcript/session CLIs and
Hermes' cross-session search both make old conversations discoverable. GitClaw
can copy the operator value without copying the storage shape: search the
fetched `gitclaw-backups` branch directly, verify it first, and return issue
paths, sources, trust metadata, scores, and body/line hashes only. Raw backup
JSON remains available through explicit local recovery/export commands, but the
default search report should be body-free and query-free.

2026-05-29 session-inspection follow-up: OpenClaw exposes transcript and
session CLIs around JSONL transcript directories, while Hermes automatically
saves conversations as sessions and can export them to JSONL. GitClaw should
keep the GitHub issue as the canonical session but add `@gitclaw /session` as
the safe inspection layer: count reconstructed messages, markers, trust states,
and hashes without copying raw conversation bodies into a new comment.

2026-05-30 session-list follow-up: local session inspection should operate on
GitClaw's canonical backup JSON, not a new session database. Add
`gitclaw session list --backup <issue.json>` as the local mirror of `/session`:
read a backed-up issue transcript, count messages and markers, report sources,
trust states, sizes, and hashes, and avoid dumping issue/comment/assistant
bodies.

2026-05-30 session-search follow-up: OpenClaw's transcript/session tooling is
built around finding prior session artifacts, and Hermes documents
cross-session search backed by FTS5. GitClaw should not add a hidden session
database yet; the issue thread is the session store. The matching primitive is
`@gitclaw /session search <query>`: search the reconstructed issue transcript,
then return only message indexes, roles, sources, trust metadata, scores, and
message/line hashes without raw transcript snippets or raw query text.
Local `gitclaw session search <query> --backup <issue.json>` should use the
same matcher over canonical backup JSON for offline triage.

2026-05-29 memory follow-up: the right GitClaw cut is read-only memory
injection, not self-writing memory. Load compact files such as
`.gitclaw/MEMORY.md`, `.gitclaw/USER.md`, `.gitclaw/IDENTITY.md`, and the
latest bounded `.gitclaw/memory/*.md` notes as context, but require normal git
commits for any memory edits. That preserves OpenClaw's portability while
avoiding Hermes-style self-improvement authority in early versions.

2026-05-29 memory-audit follow-up: OpenClaw documents `MEMORY.md` plus
`memory/YYYY-MM-DD.md` as the durable Markdown memory layer, while Hermes keeps
small prompt memory separate from larger session search. GitClaw should make
this boundary issue-visible with `@gitclaw /memory`: report long-term memory
presence, dated-note counts, loaded/omitted notes, canonical date filenames,
and hashes without dumping memory bodies or allowing hidden memory writes.

2026-05-29 soul-inspection follow-up: because OpenClaw and Hermes treat
`SOUL.md`, `IDENTITY.md`, `USER.md`, `MEMORY.md`, and dated memory notes as
high-authority portable context, GitClaw should make that load set auditable
without exposing the contents in issue comments. The narrow command is
`@gitclaw /soul`: list loaded identity, policy, and memory files with byte
counts, line counts, and short hashes, but never dump file bodies.

2026-05-30 soul-list follow-up: make the high-authority inventory surface
explicit in both channels. `@gitclaw /soul` already lists the loaded portable
context set, but `@gitclaw /soul list` and `gitclaw soul list` should be
documented aliases so operators can inspect context provenance without
confusing inventory with validation or search.

2026-05-29 soul-validation follow-up: the same high-authority files should have
a local safety gate, not just an inventory. GitClaw should treat
`.gitclaw/SOUL.md`, `.gitclaw/IDENTITY.md`, `.gitclaw/USER.md`,
`.gitclaw/TOOLS.md`, `.gitclaw/MEMORY.md`, and `.gitclaw/HEARTBEAT.md` as the
required minimal context set, warn on noncanonical dated memory filenames, and
expose the result in both `/soul` and `gitclaw soul validate` without dumping
file bodies.

2026-05-29 soul-search follow-up: OpenClaw's default workspace instructions
require reading `SOUL.md`, `USER.md`, today's/yesterday's memory notes, and
`MEMORY.md` before responding, while Hermes frames `SOUL.md` as stable
character/behavior guidance rather than memory. GitClaw should make that
high-authority context searchable without making it public: `@gitclaw /soul
search <query>` and `gitclaw soul search <query>` should report only query
hashes, paths, categories, line numbers, scores, and file/line hashes.

### Skills And Plugins

OpenClaw uses AgentSkills-compatible skill folders with a `SKILL.md`. Skills can come from:

- workspace,
- project agent,
- personal `~/.agents`,
- managed `~/.openclaw`,
- bundled install,
- extra configured folders,
- plugins.

OpenClaw has ClawHub as a public skill/plugin registry. This is valuable for ecosystem growth but creates a supply-chain and prompt-injection surface. The docs explicitly treat third-party skills as untrusted code.

### Automation

OpenClaw splits automation into several mechanisms:

- Cron for exact scheduled jobs and one-shot reminders.
- Heartbeat for approximate periodic checks with main-session context.
- Background tasks for detached work tracking.
- Task Flow for durable multi-step orchestration.
- Hooks for lifecycle/event-driven automation.
- Standing orders for permanent operating authority.
- Inferred commitments for short-lived follow-ups.

The important design lesson is that "automation" is not one feature. Exact time, approximate awareness, detached work, event hooks, and persistent authority need separate semantics.

2026-05-29 heartbeat follow-up: OpenClaw's heartbeat docs define heartbeat as
"periodic awareness": it reads `HEARTBEAT.md`, runs lightweight checks on a
fixed interval, and expects `HEARTBEAT_OK` when there is nothing useful to say.
For GitClaw, the direct translation is not a local gateway loop. It is a
scheduled GitHub Actions workflow that scans issue sessions explicitly opted in
with a `gitclaw:heartbeat` label, loads `.gitclaw/HEARTBEAT.md`, and posts only
when there is a visible update. The critical safety difference is that each
heartbeat is a fresh Actions run with a hidden idempotency slot, not a
long-lived main session that can silently mutate memory.

2026-05-29 workflow-dispatch follow-up: GitClaw needs a second fresh-run
boundary in addition to heartbeat. A channel poller that mirrors Telegram or
Slack messages using `GITHUB_TOKEN` cannot depend on those generated comments to
fire another `issue_comment` workflow, so the main issue handler needs an
explicit `workflow_dispatch` wakeup path. The useful OpenClaw/Hermes analogue is
not a socket loop; it is an auditable issue-number dispatch with a stable
external event ID used as the idempotency key.

2026-05-29 proactive usefulness follow-up: OpenClaw's automation categories and
Hermes' cron/goals both point to a useful GitClaw feature that is not just
heartbeat. Proactive jobs should be normal scheduled GitHub Actions workflows
that create or reuse GitHub issues, then dispatch the main issue handler. This
preserves the no-daemon architecture while allowing email triage, reminders,
watchers, and reports to initiate their own visible issue threads.

2026-05-29 proactive implementation follow-up: the minimal GitClaw cut is a
generic `proactive enqueue` command and dispatchable workflow. It creates one
issue per job name and slot, stores a `gitclaw:proactive-run` marker in the
issue body, labels the issue, and wakes the normal handler with a deterministic
dispatch ID. This keeps OpenClaw-style scheduled usefulness while preserving
GitHub as the audit and replay surface.

2026-05-29 proactive audit follow-up: OpenClaw cron distinguishes durable job
definitions from runtime state and records cron executions as background tasks,
while Hermes cron emphasizes fresh scheduled sessions, attached skills, and
explicit delivery. GitClaw's GitHub-native analogue should expose the same
operator visibility through issue-visible metadata: `@gitclaw /proactive`
reports the proactive workflow, schedule trigger, prompt files, labels, and
enqueue contract without dumping the prompt bodies.

2026-05-29 proactive-init follow-up: the reviewed-file boundary matters as much
as the scheduler. GitClaw should make new cron-like jobs easy through a local
generator that writes `.gitclaw/proactive/<name>.md` and
`.github/workflows/gitclaw-proactive-<name>.yml`, but the files still land in
git for review. This borrows OpenClaw/Hermes' durable job-definition idea
without allowing an agent turn to silently install new scheduled automation.

2026-05-29 model resilience follow-up: OpenClaw cron records model/provider
failures as job errors instead of treating empty replies as success, and Hermes
cron documents fresh sessions plus provider recovery/fallback behavior for
scheduled runs. GitHub Models is free but rate-limited unless users opt into
paid usage, and GitHub Actions scheduled runs can be delayed or dropped under
load. GitClaw therefore needs bounded model retries, issue-visible provider
configuration, and safe failure comments. `@gitclaw /models` is the GitHub-native
audit surface for provider family, model, token source name, timeout, and retry
budget without exposing tokens or raw provider bodies.

2026-05-30 model-list follow-up: model wiring should be inspectable before an
operator opens an issue, especially because GitHub Models access depends on
job permissions and token source. Add `gitclaw models list` as the local mirror
of `/models`: provider family, model ID, endpoint host, token-source name,
timeout, retry settings, and environment knobs, with no provider call and no
token value.

2026-05-29 workflow-runtime follow-up: GitClaw's serverless runtime is GitHub
Actions itself, so action runtime deprecations are part of product reliability.
Checked-in workflows and spec snippets should track Node 24-compatible
first-party action majors (`actions/checkout@v5`, `actions/setup-go@v6`, and
`actions/upload-artifact@v6`) to avoid noisy or broken assistant turns as
GitHub phases out Node.js 20 action execution.

2026-05-29 config visibility follow-up: OpenClaw's `models status` separates
read-only provider/auth visibility from live probes that may spend tokens, and
Hermes profiles isolate config, API keys, memory, sessions, skills, cron jobs,
and state per agent. GitClaw's no-daemon version should expose equivalent
operator confidence as metadata only: `@gitclaw /config` reports effective
labels, trusted associations, prompt budgets, command surface, and workflow file
hashes without dumping config or workflow bodies and without allowing the agent
to mutate its own configuration.

2026-05-30 config-list follow-up: config inspection should also exist outside
issue chat. Add `gitclaw config list` as the local mirror of `/config`: report
effective config source, labels, trusted associations, model/prompt budgets,
deterministic command surface, and config/workflow file hashes, but omit
issue-only metadata and all file bodies.

2026-05-29 command-catalog follow-up: OpenClaw and Hermes both lean on command
help as a discoverability and operations primitive. GitClaw's issue-native
version should expose `@gitclaw /help` and `@gitclaw /commands` as a
body-free, deterministic command catalog with aliases, model marker names,
categories, summaries, and local CLI helpers, so maintainers can inspect the
available control-plane surface before invoking a more specific report.

2026-05-29 skill-info follow-up: OpenClaw's skills CLI includes local
`list`/`info`/`check` commands, while Hermes documents a compact `/skills` or
`hermes skills list` inventory plus progressive `skill_view(name)` loading.
GitClaw should map that to `@gitclaw /skills info <name>` and
`gitclaw skills info <name>`: report one skill's path, hash, requirements,
selection state, and validation findings without dumping full `SKILL.md`
bodies into issues or logs.

2026-05-30 skill-list follow-up: the inventory side should be just as explicit
as focused lookup. `@gitclaw /skills` already behaves like OpenClaw/Hermes'
skills list surface, but local operators should not have to remember that
`validate` is not inventory. Add `gitclaw skills list` and treat
`@gitclaw /skills list` as a documented alias for the same body-free skill
inventory report.

2026-05-29 skill-search follow-up: OpenClaw's `skills search [query...]`
searches the ClawHub skill feed, while Hermes' skill surface keeps a compact
searchable/listable skill atlas separate from full `skill_view` body loading.
GitClaw should keep search local and git-native: `@gitclaw /skills search
<query>` and `gitclaw skills search <query>` search only skill metadata
already present in the repo, report match fields and hashes, and represent the
raw query by hash/term count because issue text may contain secrets.

2026-05-29 memory-validation follow-up: OpenClaw's memory CLI exposes
status/index/search/promotion surfaces and treats deep writes to `MEMORY.md` as
special, while Hermes' memory guidance separates durable facts from procedural
skills and warns against storing secrets or stale task logs. GitClaw's
git-native equivalent should keep memory read-only in assistant turns but add
`@gitclaw /memory validate` and `gitclaw memory validate` for body-free checks:
long-term memory presence, canonical dated-note paths, empty files, context
size limits, and obvious secret-like token patterns.

2026-05-30 memory-list follow-up: the memory inventory path should be explicit,
not just the fallback behavior of `/memory`. Add `@gitclaw /memory list` and
`gitclaw memory list` as documented aliases for the body-free memory surface:
long-term memory, dated note counts, loaded/omitted notes, latest note,
hashes, and validation rollups.

2026-05-29 memory-search follow-up: OpenClaw's memory search accepts positional
or `--query` input plus `--max-results`, and its memory-search concept combines
semantic/vector recall with BM25-style lexical fallback. Hermes likewise treats
session search as the recall layer beyond compact `MEMORY.md`/`USER.md`.
GitClaw's no-server cut should start with local lexical search over
git-backed memory files: report paths, line numbers, scores, and hashes, but
never echo raw queries or memory body snippets into issue comments.

2026-05-29 repo config follow-up: OpenClaw's `config` CLI exposes schema,
validation, dry-run patching, and guarded writes, while `configure` preserves
existing model defaults unless the operator explicitly changes them. Hermes
profiles make `config.yaml` one of the isolated per-agent artifacts alongside
`.env`, `SOUL.md`, memories, sessions, skills, cron jobs, and state. GitClaw's
matching primitive is a checked-in `.gitclaw/config.yml` loaded read-only with
unknown fields rejected and environment overrides applied last.

2026-05-29 doctor follow-up: OpenClaw's doctor command combines config
normalization, health checks, skills status, model auth health, state
integrity, channel warnings, and security warnings; its config docs also
recommend `config validate` before runtime use. GitClaw should take the cold
read-only subset first: `@gitclaw /doctor` reports config validation, workflow
presence, context file metadata, skill counts, memory note counts, and proactive
prompt counts without auto-repair or body dumps.

2026-05-29 doctor-validation follow-up: after adding dedicated skill, soul, and
tool validators, the doctor command should become the top-level rollup. This
matches OpenClaw's health-diagnostics posture and Hermes' platform/toolset
status surfaces: `/doctor` should report validation error/warning totals plus
skill, soul, and tool validation statuses without listing body-level findings
or exposing private context.

2026-05-29 channel visibility follow-up: Slack's Events API expects either a
server HTTP endpoint or Socket Mode, Socket Mode uses a stateful WebSocket
instead of a static public URL, Telegram's `getUpdates` provides long polling
with offsets, and GitHub's workflow dispatch endpoint requires an
Actions-write-capable authenticated caller. GitClaw should expose these bridge
constraints inside the issue thread: `@gitclaw /channels` reports the generic
channel-ingest workflow, provider keys, dispatch contract, and marker counts
without dumping mirrored channel messages.

2026-05-30 channel-list follow-up: the bridge contract also needs an operator
CLI surface before provider-specific Telegram or Slack code lands. Add
`gitclaw channels list` as a local mirror of the body-free `/channels` report:
workflow-dispatch trigger, workflow permissions, normalized inputs, supported
provider keys, labels, and dispatch-id contract, with issue-only marker counts
omitted in CLI mode.

### Multi-Agent Routing

OpenClaw's multi-agent model treats each agent as a full isolated persona scope:

- separate workspace,
- separate state directory,
- separate session store,
- separate auth profiles,
- channel/account bindings.

This lets one Gateway serve several people/personas, but also makes routing, auth, and memory boundaries load-bearing.

### Sandboxing

OpenClaw can run tool execution in sandbox backends. The Gateway remains on the host; tools can run in Docker, SSH, or OpenShell-backed sandboxes. Sandboxing is optional and configurable by mode and scope.

Important caveat: OpenClaw docs frame sandboxing as blast-radius reduction, not a perfect security boundary.

### Migration Surface

OpenClaw has a Hermes migration provider. It can import:

- Hermes model configuration,
- MCP server definitions,
- `SOUL.md` and `AGENTS.md`,
- memory files,
- skills,
- supported credentials after explicit handling.

It archives but does not automatically trust Hermes plugins, sessions, logs, cron jobs, MCP tokens, or state DB.

That is a good migration principle: import declarative state carefully, archive executable/ambiguous state for manual review.

## Hermes Agent

### What It Is

Hermes Agent is NousResearch's self-improving AI agent. It presents itself as a terminal-native autonomous coding/task agent with persistent memory, agent-created skills, a messaging gateway, multiple terminal backends, and provider-agnostic model routing.

Current public repo metadata seen during research:

- Repository: `NousResearch/hermes-agent`
- Primary language: Python.
- High ecosystem gravity: over 170k GitHub stars observed.
- Entry points include CLI, TUI, messaging gateway, ACP, API server, batch runner, and Python library.

### Core Architecture

Hermes centers around `AIAgent`:

```text
CLI / Gateway / ACP / API / Batch
  -> AIAgent
  -> prompt builder
  -> provider resolver
  -> tool registry
  -> session storage
  -> tool backends
```

Important subsystems:

- Prompt builder: assembles SOUL/personality, memory, skills, context files, tool guidance, and model-specific instructions.
- Provider resolver: maps provider/model to API mode, credentials, and base URL.
- Tool registry: central schema/dispatch layer for many tools and toolsets.
- Session storage: SQLite with FTS5 search.
- Gateway: platform adapters, authorization, slash commands, cron ticking, and delivery.
- Plugins: user/project/pip entry points, plus memory-provider and context-engine plugin types.

### Memory

Hermes uses bounded curated memory plus searchable sessions:

- `~/.hermes/memories/MEMORY.md` for agent notes.
- `~/.hermes/memories/USER.md` for user profile.
- character limits keep prompt memory compact.
- all sessions are stored in SQLite with FTS5 search.
- external memory providers can run alongside built-in memory.

This is a stronger distinction than OpenClaw's default docs suggest: memory is the tiny always-in-context layer, while session search is the large on-demand recall layer.

2026-05-29 follow-up: Hermes' feature docs explicitly group tools into
toolsets that can be enabled per platform and describe project context files
such as `AGENTS.md`, `SOUL.md`, and other repo-local instruction files as part
of every conversation. The GitClaw adaptation should keep this same separation:
context files and skills are prompt inputs, while tools are bounded read-only
operations whose outputs are visible in the prompt and testable in E2E runs.

2026-05-29 search-tool follow-up: Hermes' file toolset includes both file read
and search operations. GitClaw should preserve that shape but keep it pre-model
and non-interactive: derive a few explicit search queries from the issue
thread, scan only bounded text files from the checkout, and insert matched
lines as `gitclaw.search_files` context. That gives the model grounded recall
without granting shell access or a general command runner.

2026-05-29 tools-inspection follow-up: OpenClaw's tool docs emphasize the
difference between tools, skills, and plugins, and note that effective policy
can remove tools before the model sees them. Hermes exposes toolsets and an
authoritative tool registry. GitClaw's no-server version should make the same
surface inspectable with `@gitclaw /tools`: list deterministic tool contracts,
show which tool outputs were produced for the current turn, and publish
input/size/hash metadata without dumping output bodies.

2026-05-30 tools-list follow-up: make the tool inventory surface explicit in
both channels. `@gitclaw /tools` already lists deterministic contracts and
active output metadata, but `@gitclaw /tools list` and `gitclaw tools list`
should be documented aliases so operators can inspect the tool registry without
confusing it with validation or search.

2026-05-29 tool-validation follow-up: OpenClaw's exec approval docs treat tool
execution as a stacked policy/allowlist/approval decision, and Hermes separates
toolsets such as terminal, file, web, cron, memory, and messaging by platform
availability. GitClaw should keep v1 narrower: validate that declared contracts
are only `read-only` or `metadata-only`, active outputs are declared and
bounded, `.gitclaw/TOOLS.md` is loaded, and `/tools` plus
`gitclaw tools validate` expose the result without dumping output bodies.

2026-05-29 tools-search follow-up: OpenClaw's tools docs distinguish
tool-policy visibility from skill/plugin instructions, and Hermes' tool
reference keeps tool names and schemas inspectable as first-class registry
metadata. GitClaw should add the same operator affordance without exposing
prompt internals: `@gitclaw /tools search <query>` and `gitclaw tools search
<query>` should search contract names/modes/triggers and active-output
names/inputs, but report only match fields, counts, hashes, and sizes.

2026-05-29 prompt-budget follow-up: OpenClaw's context docs expose per-file and
total prompt caps plus visible truncation markers, while Hermes' memory/context
docs treat character limits as a core defense against context bloat. GitClaw
should use the same principle before adding semantic compaction: cap the final
prompt, cap transcript message count, preserve the original issue plus the
recent tail, and include explicit truncation markers so failures are auditable.

2026-05-29 prompt-inspection follow-up: the GitHub-native version should make
that budget visible with `@gitclaw /prompt`: report final prompt size, prompt
hash, transcript omission/truncation counts, context contributors, and active
tool output metadata without dumping the prompt body. That gives OpenClaw-style
context debugging while preserving GitHub issue privacy boundaries.

2026-05-29 context-inspection follow-up: OpenClaw's `/context` diagnostics
make context contributors visible before debugging model behavior. GitClaw's
serverless equivalent should be a deterministic issue command, `@gitclaw
/context`, that posts selected context files, selected skills, and read-only
tool output names/sizes without dumping full prompt contents or calling a
model.

2026-05-30 context-list follow-up: context visibility should also exist before
an issue turn. Add `gitclaw context list` as the local mirror of `/context`:
body-free context file metadata, selected always-on skills, deterministic tool
output input/size/hash metadata, and prompt budget settings, with no file,
skill, prompt, or tool-output bodies.

2026-05-30 prompt-list follow-up: prompt-budget visibility should also be
available before opening an issue. Add `gitclaw prompt list` as the local
mirror of `/prompt`: provider/model, prompt size/hash, configured budgets,
transcript counts, context files, selected always-on skills, and deterministic
tool-output input/size/hash metadata, with no prompt, file, skill, issue, or
tool-output bodies.

### Skills

Hermes skills are on-demand knowledge documents, also compatible with AgentSkills. The default local source of truth is `~/.hermes/skills/`, with optional external directories.

Notable difference from conservative systems: Hermes explicitly allows the agent to create, modify, or delete skills. That supports self-improvement, but it increases the need for review, quarantine, provenance, and rollback.

2026-05-29 skill-inspection follow-up: OpenClaw exposes skill inventory through
`openclaw skills` commands, and Hermes separates `skills_list` metadata from
`skill_view` full-body loading. GitClaw should keep the same progressive
disclosure in issue form: `@gitclaw /skills` lists local git-tracked skill
metadata and selected paths without dumping `SKILL.md` bodies or allowing
agent-authored skill mutation.

2026-05-29 skill-audit follow-up: OpenClaw skill metadata includes runtime
requirements such as env vars and binaries, while Hermes' progressive skill
tools keep compact metadata separate from full instruction bodies. GitClaw
should therefore make skill provenance explicit in the repo-local index:
frontmatter/description presence, byte and line counts, hashes, `always`
activation, declared requirement counts, and missing requirement counts.

2026-05-29 skill-validation follow-up: OpenClaw's current skill docs require
`name` and `description`, recommend lower hyphen-case names, and tell authors
to align the leaf folder with frontmatter. The ClawHub format also makes
runtime requirements part of registry/security analysis. GitClaw should expose
the same safety bar without installing or executing skills: `/skills` and
`gitclaw skills validate` report validation status, duplicate names, invalid
names, folder/name mismatches, and missing declared requirements without
dumping full skill bodies.

### Cron And Long-Horizon Work

Hermes cron jobs are first-class agent tasks, not just shell tasks. They can:

- be created through natural language,
- run one-shot or recurring schedules,
- attach skills,
- deliver to chat/platform targets,
- use project `workdir`,
- run in profile-specific contexts,
- run in no-agent/script-only mode.

The useful Hermes lesson for GitClaw heartbeat is the fresh-run boundary:
scheduled work should make its delivery, project directory, toolset, and
idempotency explicit. GitClaw should not try to emulate Hermes' full cron
manager. It should use GitHub's built-in `schedule` trigger for best-effort
periodic checks, `workflow_dispatch` for manual and E2E runs, and visible issue
comments as the delivery/audit surface.

2026-05-29 channel wakeup follow-up: Hermes' gateway can keep a live channel
session open, but GitClaw's no-server version should make channel wakeups
explicit. Channel bridges should write durable issue/comment state first, then
dispatch the canonical issue with the channel message ID as `dispatch_id`. That
keeps replay and dedupe in GitHub instead of introducing a hidden queue.

2026-05-29 channel transcript follow-up: OpenClaw's gateway owns provider
identity and provenance before passing messages to the agent. GitClaw's
equivalent is a hidden `gitclaw:channel-message` comment marker carrying
channel and source message IDs. The marker lets Actions-authored bridge
comments survive transcript reconstruction as user messages while still marking
the message body as untrusted input.

2026-05-29 generic channel ingest follow-up: before provider-specific Telegram
or Slack pollers, GitClaw should expose a generic channel-ingest workflow. It
accepts normalized channel/thread/message fields, writes the canonical issue
state, and dispatches the normal handler. This mirrors the gateway boundary in
OpenClaw while staying serverless and testable with GitHub Actions alone.

Hermes' session docs also expose a practical backup primitive:
`hermes sessions export backup.jsonl` writes conversation metadata and messages
as durable JSONL. GitClaw should preserve the same principle, but use GitHub
issues as the canonical session source and write one canonical JSON file per
issue into git instead of introducing a local SQLite/session database.

Hermes also has:

- `delegate_task` for parallel isolated child agents,
- `/goal` for persistent turn-to-turn objectives with a judge loop,
- Kanban/multi-agent task board features,
- background hooks and batch processing.

This makes Hermes a better reference than OpenClaw for durable coding operations where "keep going until acceptance criteria are met" matters.

### Security

Hermes documents defense in depth:

- user authorization through allowlists and DM pairing,
- dangerous command approvals,
- hardline blocklist for catastrophic commands,
- container/back-end isolation,
- MCP credential filtering,
- context-file scanning,
- cross-session isolation,
- input sanitization.

The approval model is detailed and worth borrowing, especially:

- fail-closed approval timeouts,
- permanent allowlists as explicit config,
- separate headless cron policy,
- an always-on no-override blocklist.

2026-05-29 write-gate follow-up: GitClaw should not wait until write mode
exists to model approval boundaries. Detect write intent in read-only mode,
apply a durable `gitclaw:write-requested` label, and inject policy context that
keeps the assistant in proposal mode. This makes future approval/write
transitions explicit instead of inferring them from free-form comments later.

### Migration Surface

Hermes can migrate from OpenClaw and import:

- `SOUL.md`,
- memories,
- user-created skills,
- command allowlists,
- messaging settings,
- allowlisted API keys,
- workspace instructions.

The bidirectional migration support between OpenClaw and Hermes is a useful signal: `SOUL.md`, `AGENTS.md`, `MEMORY.md`, `USER.md`, skills, provider config, auth profiles, and scheduled jobs are the de facto portability units in this ecosystem.

## Smaller Implementations And Variants

### NanoClaw

Repository observed: `nanocoai/nanoclaw`.

NanoClaw is the clearest minimalist reaction to OpenClaw:

- one host process,
- per-session/per-agent containers,
- Claude Agent SDK / Claude Code as the harness,
- SQLite files as queues between host and container,
- channels and providers installed on demand via skills,
- "fork and customize" instead of a broad plugin/config platform.

Architecture summary from its README:

```text
messaging apps
  -> host router
  -> inbound.db
  -> containerized agent runner
  -> outbound.db
  -> host delivery
  -> messaging apps
```

Lessons for `gitclaw`:

- Small codebases build trust.
- OS isolation is easier to explain than application-level policy.
- "Skills over features" keeps trunk lean.
- File/SQLite IPC is boring but inspectable.
- Tying to one high-quality harness can beat abstracting every model too early.

### LightClaw

Repository observed: `OthmaneBlial/lightclaw`.

LightClaw is a tiny Python implementation. Its short architecture:

```text
Telegram or terminal chat
  -> memory recall (SQLite + semantic search)
  -> provider routing
  -> response + optional file operations
  -> optional delegated local agents
```

Lessons:

- The minimum viable version of this category is small: chat input, memory, provider routing, workspace operations, and optional delegation.
- If `gitclaw` starts narrower than this, it should still keep the same clean loop.

### MicroClaw

Repository observed: `microclaw/microclaw`.

MicroClaw is a Rust chat agent inspired by NanoClaw. It emphasizes:

- shared agent loop across channels,
- file and structured memory,
- resumable session state,
- tool calls,
- compaction,
- sub-agents,
- skills,
- plan/execute,
- scheduled tasks,
- platform adapters.

Lessons:

- Rust makes sense if the project wants a long-lived, lower-overhead daemon with stronger type boundaries.
- A shared loop with platform adapters is a repeat pattern across serious implementations.

### NullClaw

Repository observed: `nullclaw/nullclaw`.

NullClaw is an extreme efficiency implementation in Zig:

- small static binary,
- low memory use,
- fast startup,
- provider/channel/tool/memory subsystems behind pluggable interfaces,
- hardware/edge orientation,
- multi-layer sandbox claims.

Lessons:

- It is useful as a design stress test, not necessarily as an MVP template.
- "Pluggable everything" can be compatible with a small binary if abstractions are compile-time/simple, but that is not free in product complexity.

### MiniClaw OS

Repository observed: `augmentedmike/miniclaw-os`.

MiniClaw OS is closer to a cognitive architecture/plugin layer on top of OpenClaw than a ground-up clone. It emphasizes:

- memory,
- planning,
- continuity,
- self-repair,
- plugins,
- a more "AI companion" orientation.

Lessons:

- The market complains about sessions starting from zero.
- Planning, memory, and handoff continuity are often perceived as more valuable than adding another channel.

### BunClaw

Repository observed: `tobalo/bunclaw`.

BunClaw is a NanoClaw fork optimized around Bun and Discord:

- Bun-native SQLite and process APIs,
- far fewer dependencies,
- per-channel Docker isolation,
- scheduled tasks,
- skills for optional integrations,
- Claude Agent SDK as the harness.

Lessons:

- Choosing one runtime and one primary channel can radically simplify the system.
- A "personal fork" model is viable for power users, but not enough for a team/product workflow.

### Stockade

Repository observed: `Dragooon/stockade`.

Stockade is not a broad OpenClaw clone. It is a multi-agent orchestrator for Claude with layered security:

- agents in containers,
- no secrets in containers,
- restricted network,
- per-tool permissions,
- RBAC,
- observability,
- explicit credential injection.

Lessons:

- For a Git/repo agent, Stockade's security posture is more relevant than broad assistant features.
- "No secrets in agent containers" should be a hard requirement for `gitclaw`.

## Cross-System Pattern Map

| Concern | OpenClaw | Hermes | Nano-style variants | Implication for `gitclaw` |
| --- | --- | --- | --- | --- |
| Primary value | Personal AI gateway | Self-improving long-horizon agent | Small, auditable assistant | Narrow to Git/repo ops first |
| Entry points | Many chat channels, UI, nodes | CLI/TUI, gateway, ACP, API | Usually one or few channels | Start GitHub + CLI, add Slack later |
| Runtime | Integrated TS agent runtime | Python `AIAgent` | Claude SDK / small loop | Pick one harness first |
| Memory | Markdown + indexed search/plugins | bounded memory + SQLite FTS5 sessions | SQLite/files | Use repo/project memory + searchable run log |
| Skills | AgentSkills + ClawHub/plugins | agent-managed skills + hub | skills as code transforms | Local skills only in MVP |
| Scheduling | cron, heartbeat, tasks, hooks, flows | cron, goals, delegation, Kanban | scheduled jobs | Add recurring repo audits, not broad reminders |
| Isolation | optional sandbox backends | terminal backends + approvals | container-first | Worktree + container per run by default |
| Trust model | pairing, policies, sandbox optional | approvals, blocklist, pairing | OS isolation | Provenance/approval gates from day one |
| Best lesson | gateway/control plane | durable work loop | inspectable isolation | Combine durable Git tasks with minimal trusted surface |

## Security Research Takeaways

Recent research frames OpenClaw/Hermes-style agents as a new risk class because they combine:

- long-lived process identity,
- messaging ingress,
- memory persistence,
- self-authored skills,
- scheduling,
- shell/filesystem access,
- credentials,
- external content ingestion.

2026-05-29 status-label follow-up: OpenClaw's task/status CLI and Hermes'
Kanban task lifecycle both point to the same product requirement: a durable
agent turn needs a visible state machine, not just a final chat message. The
GitClaw adaptation should stay GitHub-native by using lightweight issue labels
for current status (`gitclaw:running`, `gitclaw:done`, `gitclaw:error`) while
keeping hidden comment markers and Actions run URLs as the provenance record.

2026-05-29 failure-path follow-up: the same audit posture applies to failed
turns. GitClaw should leave a small, machine-readable `gitclaw:error` comment
that points to the Actions run and says which phase failed, but it should not
copy prompt text, user-provided secrets, or raw model-provider response bodies
back into the issue. The full trace belongs in Actions logs; the issue should
only carry a bounded diagnostic.

2026-05-29 prompt-artifact follow-up: OpenClaw/Hermes-style systems need
replayable run evidence, but prompt dumps are sensitive because they combine
issue text, memory, skills, and tool output. GitClaw should therefore make
prompt artifacts explicit opt-in, redact common token shapes before upload, and
store them as GitHub Actions artifacts rather than issue comments or logs.

2026-05-29 policy-inspection follow-up: OpenClaw's security docs separate
sandbox location, tool allow/deny policy, and elevated host execution, and
Hermes' docs frame safety around authorization, command approval, and container
isolation. GitClaw's GitHub-native equivalent should be issue-visible and
serverless: `@gitclaw /policy` reports preflight authorization, trusted actor
state, managed labels, expected workflow permissions, write-intent gating, and
policy-output metadata without exposing issue bodies.

2026-05-30 policy-list follow-up: policy shape should also be inspectable
before an issue event exists. Add `gitclaw policy list` as the local static
policy mirror: trusted associations, managed labels, expected workflow
permissions, model/run mode, and any policy-output metadata, while omitting
event labels, actor state, preflight results, and write-intent state.

Main attack pattern: an untrusted input enters through one surface, persists into memory/skills/cron/filesystem, then fires later through a different surface when the attacker is no longer present.

Design requirements for `gitclaw`:

- Treat memory, skills, schedules, and config changes as privileged writes.
- Record provenance for every durable state mutation: source, author, channel, run, timestamp, and trust level.
- Require owner approval for persistence that changes future behavior.
- Separate "read evidence" from "act on evidence."
- Never put raw secrets inside agent execution sandboxes.
- Use per-run worktrees or disposable clones.
- Default network egress should be denied or allowlisted.
- Require explicit outbound destinations for comments, pushes, PR creation, messages, and webhooks.
- Keep a tamper-evident task/run ledger.
- Keep an always-on hardline blocklist independent of approval mode.
- Make rollback/checkpoints first-class.

## Product Lessons For GitClaw

### Do Not Clone The Whole Assistant

OpenClaw's breadth is impressive, but cloning broad channel support, mobile nodes, voice, browser, marketplace, and general personal-assistant behavior would create a large attack surface before the core Git workflow is proven.

`gitclaw` should be deliberately narrower:

- GitHub/Git first.
- Repo/task/worktree first.
- Durable audit trail first.
- Optional messaging second.

### Use Git As The Audit Substrate

Unlike general assistants, a Git-centered system has a natural source of truth:

- issues,
- PRs,
- commits,
- branches,
- review comments,
- check runs,
- statuses,
- bot comments,
- run artifacts.

The agent should leave evidence in GitHub/Git rather than hiding state in a private app database.

### Suggested MVP Shape

```text
GitHub webhook / CLI command
  -> gitclaw daemon
  -> task record
  -> isolated worktree/container
  -> agent harness
  -> patch/test/result artifacts
  -> approval gate
  -> branch/commit/PR/comment
```

MVP components:

- `gitclaw init`: create config and local state.
- `gitclaw daemon`: receive GitHub webhooks and run task queue.
- `gitclaw run <repo> <prompt>`: local one-shot task.
- GitHub app or webhook receiver for issue/PR comments.
- Per-task worktree checkout.
- Agent harness adapter for one backend initially: Codex, Claude Code, or OpenCode.
- Task ledger in SQLite.
- Artifacts directory with logs, patches, summaries, and test output.
- Approval model for push/PR/comment/secret/network actions.
- Local project memory in `.gitclaw/` or repo metadata, plus global user preferences.
- Minimal skills directory, no public marketplace in v1.

### First-Class Objects

`gitclaw` should probably model these explicitly:

- `Repo`: remote, local cache path, default branch, policies.
- `Task`: requested work, source event, requester, trust level, acceptance criteria.
- `Run`: one attempt in one worktree/container with model/tool logs.
- `AgentProfile`: harness, model, tool permissions, memory scope.
- `Worktree`: disposable or persistent checkout linked to a run.
- `Approval`: requested action, diff/command/destination, owner decision.
- `MemoryFact`: durable project/user fact with provenance and expiry.
- `Skill`: local instruction bundle with trust status.
- `Connector`: GitHub, Slack, CLI, webhook, etc.
- `Artifact`: patch, branch, PR, test log, screenshot, summary.

### Hard Product Boundaries

Recommended non-goals for the first spec:

- No all-channel personal assistant.
- No public skill marketplace.
- No agent-written skills without review.
- No raw host shell by default.
- No credentials in task containers.
- No autonomous pushes to protected branches.
- No hidden memory writes from untrusted external content.
- No long-running unsupervised loops without budget and stop conditions.

## Open Questions For Speccing

1. Is `gitclaw` primarily for one user's repos, or for teams/organizations?
2. Should the first control surface be GitHub comments, CLI, Slack, or a local web UI?
3. Which agent harness should be first: Codex, Claude Code, OpenCode, or a direct model/tool loop?
4. Should it run local-first only, or support a VPS/cloud daemon from day one?
5. Should outputs create PRs automatically, or only produce patches until approved?
6. How much memory should be global user memory vs repo-local project memory?
7. Do we want OpenClaw/Hermes migration compatibility, or just borrow their file conventions?
8. Is the product a security-first coding agent operator, or a personal Git assistant with broad convenience features?

## Sources

- OpenClaw official docs: https://docs.openclaw.ai/llms.txt
- OpenClaw default AGENTS.md template: https://docs.openclaw.ai/reference/AGENTS.default
- OpenClaw about page: https://openclawlab.com/en/about/
- OpenClaw GitHub: https://github.com/openclaw/openclaw
- OpenClaw agent runtime docs: https://docs.openclaw.ai/concepts/agent
- OpenClaw gateway architecture docs: https://docs.openclaw.ai/concepts/architecture
- OpenClaw transcript CLI docs: https://docs.openclaw.ai/cli/transcripts
- OpenClaw sessions CLI docs: https://docs.openclaw.ai/cli/sessions
- OpenClaw automation docs: https://docs.openclaw.ai/automation
- OpenClaw scheduled tasks docs: https://docs.openclaw.ai/automation/cron-jobs
- OpenClaw heartbeat docs: https://openclawlab.com/en/docs/agent/heartbeat/
- OpenClaw memory docs: https://docs.openclaw.ai/concepts/memory
- OpenClaw tools overview: https://docs.openclaw.ai/tools
- OpenClaw config CLI docs: https://docs.openclaw.ai/cli/config
- OpenClaw configure docs: https://docs.openclaw.ai/cli/configure
- OpenClaw doctor docs: https://docs.openclaw.ai/doctor
- OpenClaw backup docs: https://docs.openclaw.ai/cli/backup
- OpenClaw creating skills docs: https://docs.openclaw.ai/tools/creating-skills
- OpenClaw skill format docs: https://docs.openclaw.ai/clawhub/skill-format
- OpenClaw migration guide: https://docs.openclaw.ai/install/migrating
- OpenClaw sandbox vs tool policy vs elevated: https://docs.openclaw.ai/gateway/sandbox-vs-tool-policy-vs-elevated
- OpenClaw exec approvals: https://docs.openclaw.ai/tools/exec-approvals
- OpenClaw sandboxing docs: https://docs.openclaw.ai/gateway/sandboxing
- OpenClaw migrating from Hermes: https://docs.openclaw.ai/install/migrating-hermes
- OpenClaw migrate CLI docs: https://docs.openclaw.ai/cli/migrate
- OpenClaw models CLI docs: https://docs.openclaw.ai/cli/models
- Hermes docs index: https://hermes-agent.nousresearch.com/docs/llms.txt
- Hermes GitHub and README: https://github.com/NousResearch/hermes-agent
- Hermes architecture docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/developer-guide/architecture.md
- Hermes sessions docs: https://hermes-agent.nousresearch.com/docs/user-guide/sessions
- Hermes profiles docs: https://hermes-agent.nousresearch.com/docs/user-guide/profiles
- Hermes migrate from OpenClaw docs: https://hermes-agent.nousresearch.com/docs/guides/migrate-from-openclaw
- Hermes memory docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/memory.md
- Hermes three-layer memory explainer: https://hermes-agent.ai/blog/hermes-agent-memory-system
- Hermes skills docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/skills.md
- Hermes tools docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/tools/
- Hermes tools reference: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/reference/tools-reference.md
- Hermes security overview: https://hermes-agent.nousresearch.com/docs/
- Hermes cron docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/cron.md
- Hermes security docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/security.md
- NanoClaw: https://github.com/nanocoai/nanoclaw
- LightClaw: https://github.com/OthmaneBlial/lightclaw
- MicroClaw: https://github.com/microclaw/microclaw
- NullClaw: https://github.com/nullclaw/nullclaw
- MiniClaw OS: https://github.com/augmentedmike/miniclaw-os
- BunClaw: https://github.com/tobalo/bunclaw
- Stockade: https://github.com/Dragooon/stockade
- Sleeper Channels and Provenance Gates: https://arxiv.org/abs/2605.13471
- Your Agent, Their Asset: https://arxiv.org/abs/2604.04759
- OpenClaw PRISM: https://arxiv.org/abs/2603.11853
- Cloud Security Alliance Hermes/OpenClaw research note: https://labs.cloudsecurityalliance.org/wp-content/uploads/2026/05/CSA_research_note_hermes_agent_CVEs_20260504-csa-styled.pdf
