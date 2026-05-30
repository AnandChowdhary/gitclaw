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

2026-05-30 skill-risk follow-up: current OpenClaw plugin-hook docs make prompt,
model, tool-call, and heartbeat extension points explicit, while Hermes'
toolsets and MCP docs emphasize filtering exposed capabilities per task. Recent
OpenClaw security work also focuses on persistent-state attacks and
skill-driven token/tool amplification. GitClaw should add a body-free
`@gitclaw /skills risk` / `gitclaw skills risk` report that scans repo-local
`SKILL.md` bodies internally for risky instruction categories, but publishes
only counts, categories, finding codes, paths, and line hashes. This gives
maintainers an issue-visible risk envelope without executing skills, contacting
registries, dumping skill bodies, or trusting third-party install metadata.

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

2026-05-30 memory-risk follow-up: Durable memory is high-leverage prompt state.
OpenClaw's editable memory and Hermes' profile/memory layers imply a review
boundary before anything becomes future context. GitClaw should add `@gitclaw
/memory risk` and `gitclaw memory risk`: scan `.gitclaw/MEMORY.md` and dated
notes for prompt-boundary overrides, credential-looking material, hidden
persistence, unbounded automation, and unreviewed host execution, while
publishing only paths, counts, codes, categories, and line hashes.

2026-05-29 backup-index follow-up: OpenClaw migration applies only after a
reviewed plan and verified backup, while Hermes exposes session export as a
portable artifact. GitClaw should make its backup branch similarly inspectable:
raw issue transcripts stay in per-issue JSON files, and a repo-scoped
`index.json`/`README.md` summarizes coverage without exposing every raw message.

2026-05-30 backup-concurrency follow-up: a git-backed backup branch is a shared
state ref, so parallel issue runs can race on push even when each issue's
assistant turn is correctly isolated. GitClaw should serialize the backup job
with a repo-wide concurrency group while keeping the normal handle job
per-issue concurrent.

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

2026-05-30 backup-command follow-up: OpenClaw exposes backup verification as a
local command over an archive, while Hermes exposes session export as a local
JSONL artifact. GitClaw's issue handler runs before the backup branch update,
so issue-visible `/backup verify`, `/backup manifest`, `/backup search`, and
related subcommands should be treated as deterministic command intents: record
the exact branch paths, local command, privacy boundary, and hashes, then let
the post-turn backup job and fetched-branch CLI command perform the real audit.

2026-05-30 backup-risk follow-up: OpenClaw's backup verification treats unsafe
paths and malformed manifests as restore blockers, while Hermes' checkpoint and
session export posture makes rollback review a local operator action. GitClaw
should add `gitclaw backup risk`: verify the fetched `gitclaw-backups` branch,
scan indexed issue payloads for integrity, path-safety, credential-handling,
prompt-boundary, restore-safety, and retention risks, and report only paths,
counts, codes, severities, and hashes. Issue-side `/backup risk` stays a
deferred intent because the raw backup branch is only written after the
assistant turn.

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

2026-05-30 backup-info follow-up: The same landscape also needs a focused
single-session card between list and raw export. GitClaw should add
`gitclaw backup info --issue <n>`: verify the fetched backup tree, locate one
canonical payload, and print payload bytes/hash, backup timestamp/event,
label/comment/transcript/message counts, assistant/error marker counts, and
body hashes only. This matches Hermes' session detail ergonomics while keeping
OpenClaw-style restore/export behind explicit commands.

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

2026-05-30 session-provenance follow-up: OpenClaw transcript artifacts and
Hermes saved sessions are useful because they make execution history
inspectable after the turn. GitClaw should fold its assistant-turn prompt
provenance marker into `@gitclaw /session`: count assistant turns with prompt
evidence, list unique prompt-context hashes, and show body-free skill/tool names
per assistant turn. This lets a later issue comment verify that a previous
model answer actually saw `gitclaw.search_files` or a selected skill without
replaying raw prompts.

2026-05-30 run-ledger follow-up: OpenClaw's gateway/runtime docs make
execution provenance visible through workspace, session, and run surfaces,
while Hermes' checkpoint/session model treats runs as replayable, auditable
units. GitClaw should expose a read-only `@gitclaw /runs` report before adding
any mutable ledger database: issue comments remain the canonical conversation
log, GitHub Actions remains the canonical execution trace, and
`gitclaw-backups` remains the canonical post-turn archive. The report should
show event/run IDs, idempotency key, preflight gates, managed labels, marker
counts, prompt-visible input hashes, and active tool-output hashes without
printing raw bodies or run payloads.

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

2026-05-30 memory-verify follow-up: OpenClaw's memory model is git-portable
Markdown, and Hermes keeps compact memory distinct from session search and
procedural skills. GitClaw should add `@gitclaw /memory verify` and
`gitclaw memory verify` as a body-free trust envelope that reports repo-local
memory provenance, loaded state, canonical dated-note coverage, hashes,
read-only write status, and explicit external-provider/session-index/background
promotion non-goals.

2026-05-30 memory-promote-plan follow-up: OpenClaw now documents memory as a
reviewable promotion pipeline: compact `MEMORY.md`, daily notes, action-
sensitive boundaries, dreaming/backfill review lanes, and no hidden memory
state beyond files on disk. Hermes likewise keeps memory bounded, curated, and
separate from both skills and session search. GitClaw should add
`@gitclaw /memory promote-plan <target>` and `gitclaw memory promote-plan
<target>` as the GitHub-native equivalent of a memory-promotion dry run:
identify whether a thread should become reviewed long-term or daily-note
memory, report target metadata and validation rollups, and explicitly avoid
model calls, transcript bodies, memory bodies, repository mutation, and direct
memory writes. Any real memory promotion should land as a reviewed git diff
and be followed by memory/profile verification plus a live GitHub Models
conversation E2E that performs an actual LLM call.

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

2026-05-30 soul-info follow-up: the latest OpenClaw/Hermes docs reinforce that
SOUL and related context files are both durable behavior sources and a security
boundary. OpenClaw's SOUL.md guide treats `SOUL.md` as git-friendly identity
state, describes identity cascade behavior, and calls out compromised
`SOUL.md`/`MEMORY.md` as persistent attack vectors. Hermes' feature overview
and tips describe `SOUL.md`, `USER.md`, `MEMORY.md`, and project context files
as prompt-shaping context that should stay focused and bounded. GitClaw should
therefore provide a focused `@gitclaw /soul info <path>` /
`gitclaw soul info <path>` card for one high-authority file at a time:
normalized path, category, source, required/canonical/latest flags,
loaded-for-this-turn state, byte/line counts, short hash, and no raw bodies.
This gives maintainers a precise provenance check without turning identity
files into public issue content.

2026-05-30 soul-edit-plan follow-up: OpenClaw's git-friendly `SOUL.md` and
Hermes' high-authority profile/memory files should remain human-reviewed
behavior sources, not model-authored side effects. GitClaw should mirror the
skill install-plan safety posture with `@gitclaw /soul edit-plan <path>` and
`gitclaw soul edit-plan <path>`: normalize one supported soul target, report
only metadata and validation rollups, refuse unsupported paths, and explicitly
disable file writes, branch creation, commits, pushes, raw requested-change
echoing, and model self-modification. Any real soul edit should happen as a
reviewed git diff and be followed by both deterministic soul verification and
a live GitHub Models conversation E2E.

2026-05-30 soul-verify follow-up: OpenClaw's `SOUL.md` convention and Hermes'
profile/memory boundary suggest a second audit surface beyond validation:
provenance. GitClaw should add `@gitclaw /soul verify` and
`gitclaw soul verify` as a body-free trust envelope that reports repo-local
source counts, required-file coverage, soul frontmatter/description presence,
identity/policy versus memory-note counts, short hashes, and explicit
`registry_verification=not_configured` /
`profile_export_verification=not_configured` findings until signed registry or
profile-export verification exists.

2026-05-30 soul-risk follow-up: because OpenClaw treats compromised
`SOUL.md`/`MEMORY.md` as persistent attack vectors and Hermes loads
SOUL/USER/MEMORY/profile context as high-authority prompt-shaping state,
GitClaw should add `@gitclaw /soul risk` and `gitclaw soul risk`. The report
should scan only loaded high-authority context and emit body-free risk metadata
for prompt-boundary overrides, secret exfiltration instructions,
persistent-state backdoors, attacker-controlled channels, unbounded automation,
unreviewed host execution, and credential persistence. It should report only
counts, paths, categories, codes, severities, and line hashes, and every change
to that risk surface should be paired with a live GitHub Models conversation
E2E after the deterministic report.

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

2026-05-30 heartbeat-report follow-up: OpenClaw's heartbeat contract and
Hermes' fresh-session cron posture also imply a separate operator visibility
surface. GitClaw should let maintainers ask `@gitclaw /heartbeat` or run
`gitclaw heartbeat status` to inspect heartbeat workflow triggers,
permissions, context-file metadata, labels, idempotency, and the
`HEARTBEAT_OK` quiet contract without calling the model or scanning issue
bodies. That keeps routine heartbeat debugging cheap while still requiring
live GitHub Models E2E for any feature batch that touches heartbeat behavior.

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

2026-05-30 standing-orders follow-up: OpenClaw distinguishes standing orders
from cron, heartbeat, hooks, and task flow: they are durable authority programs
with scope, triggers, approval gates, and escalation rules. Hermes profiles make
the same lesson concrete by scoping SOUL, memories, sessions, skills, cron
jobs, and state per profile. For GitClaw, the useful translation is
`.gitclaw/STANDING_ORDERS.md`: a reviewed repo file loaded into model context,
plus `@gitclaw /orders` and `gitclaw orders list|verify` reports that audit
program clause coverage and proactive enforcement metadata without executing
orders or printing their bodies.

2026-05-30 hooks follow-up: OpenClaw's hook docs split coarse file-based
internal hooks from typed plugin hooks and webhooks. Internal hooks react to
command, session, gateway, and message events and are inspected with
`openclaw hooks list|check|info`, but they are still executable integration
surface. GitClaw should start with declarative hook policy and specs:
`.gitclaw/HOOKS.md` plus `.gitclaw/hooks/*.md`, reported by `@gitclaw /hooks`
and `gitclaw hooks list|verify` without executing handlers. This preserves the
event-driven design lesson while keeping all side effects behind reviewed
GitHub workflows and approval gates.

2026-05-30 plugins/toolsets follow-up: OpenClaw's capabilities docs draw a
clean line between tools, skills, and plugins: plugins add runtime capabilities
such as tools, providers, channels, hooks, and packaged skills, while tool
policy decides what the model can actually see. Its plugin-building docs also
require manifests/contracts so hosts can discover ownership before loading a
runtime. Hermes makes the same boundary explicit through toolsets and MCP:
toolsets configure capability bundles per platform/session/task, and MCP
servers are filtered so only selected external tools are exposed. GitClaw
should therefore add `.gitclaw/PLUGINS.md` plus `.gitclaw/plugins/*.md` as a
declarative audit layer only. The first cut is `@gitclaw /plugins` and
`gitclaw plugins list|verify`, reporting plugin intent and quarantining package
files, installers, MCP connections, and runtime hooks until reviewed workflows
and approval gates exist.

2026-05-30 tasks/kanban follow-up: OpenClaw background tasks give detached
work durable task records and an issue/chat-visible `/tasks` board, while Task
Flow adds durable multi-step orchestration above individual task records.
Hermes Kanban makes the same idea explicit as a board with task statuses,
handoff comments, parent-child links, retries, heartbeats, and named worker
profiles. GitClaw should not copy the SQLite dispatcher or spawn workers in
v1. Its first translation should be issue-native: GitHub issues are task rows,
labels are the state machine, comments are the handoff log, and
`.gitclaw/TASKS.md` plus `.gitclaw/tasks/*.md` declare reviewed task/flow
policy. `@gitclaw /tasks` and `gitclaw tasks list|verify` can expose that
ledger without dumping bodies, starting a dispatcher, or opening a task DB.

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

2026-05-30 proactive-list follow-up: proactive visibility should be available
before opening an issue too. Add `gitclaw proactive list` as the local mirror
of `/proactive`: workflow presence, workflow_dispatch/schedule trigger state,
prompt file metadata, labels, and enqueue contract, with no prompt bodies and
no issue-only metadata.

2026-05-30 proactive-info follow-up: OpenClaw/Hermes-style scheduled jobs need
operator visibility at the individual-job level, not only a global list. Add a
focused `proactive info <name>` report that names the prompt file, generic
workflow, generated workflow candidate, trigger metadata, hashes, and enqueue
contract while keeping prompt/workflow/issue bodies out of comments.

2026-05-30 proactive-risk follow-up: cron-like agents are high-leverage because
a reviewed prompt plus a scheduled workflow can wake itself repeatedly. Borrow
OpenClaw's separation between durable job definitions and runtime state, plus
Hermes' fresh scheduled-session boundary, by adding a body-free
`proactive risk` report. It should scan proactive prompt files and the generic
GitHub Actions workflow internally for prompt-boundary overrides, credential
material, raw prompt logging, host execution of prompt bodies, missing
workflow-dispatch/permission boundaries, and unbounded loops, while reporting
only paths, counts, risk codes, severities, and line hashes. E2E acceptance
must include both the deterministic audit and a real GitHub Models follow-up
conversation that proves model inference and tool-visible repo search.

2026-05-30 hook-risk follow-up: OpenClaw's file-based hooks are powerful
because they bind user-visible events to automation, but that also makes them a
prompt-injection and host-execution choke point. GitClaw should keep hook v1
metadata-only while adding a `hooks risk` report that scans hook policy/spec
files and ignored executable-looking handlers for prompt-boundary overrides,
credential material, raw payload logging, untrusted issue-body execution,
external webhook bridges, repository mutation, missing approval/audit-only
boundaries, and unbounded loops. As with proactive risk, live acceptance must
pair the deterministic report with a GitHub Models follow-up that proves real
inference and prompt-visible repo search.

2026-05-30 plugin-risk follow-up: OpenClaw plugins can arrive through
managed sources such as ClawHub, npm, git, local paths, or hook packs, while
Hermes-style tool/MCP integrations can add new runtime channels and
credential-bearing servers. GitClaw's v1 answer should remain deliberately
boring: plugin specs are repo-reviewed metadata only. Add a `plugins risk`
report that scans plugin policy/spec files and ignored package/runtime files
for supply-chain install instructions, MCP/runtime connections, prompt
boundary overrides, credential material, raw payload logging, untrusted
issue-body execution, external webhooks, repository mutation, missing
approval/metadata-only gates, and unbounded loops. Acceptance should mirror the
other risk reports: deterministic body-free audit plus a real GitHub Models
follow-up proving inference and repo-search tool exposure.

2026-05-30 task-risk follow-up: OpenClaw Task Flow composes work by creating
background tasks and advancing the flow as tasks complete, while Hermes Kanban
uses durable task boards, dispatchers, workers, and a dedicated kanban toolset.
GitClaw intentionally keeps the first task primitive simpler: GitHub issues
are the durable task rows, labels are the state machine, and comments are the
handoff log. Add a `tasks risk` report that scans task policy/spec files for
detached-worker or subagent spawn instructions, external task DBs, untrusted
issue-body execution, raw payload logging, prompt-boundary overrides,
credentials, webhook bridges, repository mutation, missing approval or
issue-native mode, and unbounded loops. Acceptance should include the
deterministic body-free audit plus a real GitHub Models follow-up proving
inference and repo-search tool exposure.

2026-05-30 skill-backed proactive follow-up: Hermes cron jobs run in fresh
sessions and can attach skills in execution order, while OpenClaw standing
orders keep durable authority in reviewed workspace files. GitClaw should keep
the no-daemon boundary by storing skill hints inside the reviewed proactive
prompt file, then letting the normal issue transcript trigger progressive
repo-local skill loading when the scheduled issue is handled.

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

2026-05-30 model-default follow-up: the GitHub-native default should track the
smallest OpenAI model available through the GitHub Models catalog, not the
latest OpenAI API name in isolation. GitHub's catalog API exposes
`https://models.github.ai/catalog/models`; the live catalog for this repo
currently includes `openai/gpt-5-nano` and does not include
`openai/gpt-5.4-mini`, so the default should move to `openai/gpt-5-nano` while
keeping `GITCLAW_MODEL` overrides for repos with different catalog access.

2026-05-30 GPT-5 parameter follow-up: the first live `openai/gpt-5-nano`
conversation failed because GitHub Models rejected `max_tokens` and requested
`max_completion_tokens`. GitClaw should select the output-token request
parameter from the model family and include that choice in `/models` diagnostics.

2026-05-30 fallback follow-up: repeated live GitHub Models conversation E2Es
started failing with `429` responses for `openai/gpt-5-nano`, while direct
local probes with the same GitHub identity returned successful tiny completions
from `openai/gpt-4.1-nano` and `openai/gpt-4o-mini`. GitHub's own billing docs
describe free GitHub Models access as rate-limited, and the REST catalog docs
make the authenticated catalog the source of available model IDs. GitClaw should
therefore keep `openai/gpt-5-nano` as the primary smallest GPT-5-family default,
record the actual selected model in the assistant marker, and support explicit
repo-configured fallback models for retryable provider statuses. Invalid-model
negative tests should disable fallback so they still verify the safe failure
path.

2026-05-30 tool-grounding follow-up: the first model-backed conversation after
the parameter fix proved provider access but exposed prompt ambiguity: the model
echoed the issue nonce where the harness wanted the repository search-result
token. GitClaw should keep tool-output token requests explicit, document that
`gitclaw.search_files` is authoritative for search-result tokens, and use
distinct token prefixes plus redacted prompt artifacts in live E2E checks.
The underlying search tool also needs per-query match limits because broad
queries like `go.mod` can otherwise consume the total match budget before the
explicit fixture phrase is searched.

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

2026-05-30 profile-visibility follow-up: Hermes profiles are the unit of
agent isolation, while OpenClaw workspace files are the portable profile
surface. GitClaw's repo-native equivalent should be a body-free
`@gitclaw /profile` report: one repository equals one reviewed git profile,
with `.gitclaw/` identity, user, soul, memory, skills, tools, model, and
validation state summarized by counts and hashes only. This gives operators a
single "which agent am I talking to?" view without adding profile mutation,
export, installer, or multi-profile switching semantics.

2026-05-30 profile-risk follow-up: Hermes profiles make isolation concrete by
separating config, `.env`, memory, sessions, skills, cron jobs, and state
databases into per-profile homes; OpenClaw's workspace docs make
`SOUL.md`/`USER.md`/`IDENTITY.md`/`TOOLS.md`/`HEARTBEAT.md`/memory the agent's
durable identity surface. GitClaw should keep the convenience but audit the
dangerous edges: profile export/import, named-profile switching, profile
installers, profile mutation, credential storage, external profile state, and
confusing profile-as-sandbox claims. Add `@gitclaw /profile risk` and
`gitclaw profile risk` as body-free metadata reports with finding codes and line
hashes only, followed by a live GitHub Models E2E that proves normal
conversation/tool behavior still works.

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

2026-05-30 skill-bundles follow-up: Hermes' current skills documentation adds
"Skill Bundles": small YAML files under `~/.hermes/skill-bundles/` that map a
single slash command to several already-installed skills plus optional
instruction text. The useful GitClaw analogue is repo-local and reviewable:
`.gitclaw/skill-bundles/<name>.yaml` should group existing
`.gitclaw/SKILLS/*/SKILL.md` files, expose `@gitclaw /bundles` and
`gitclaw bundles` body-free metadata reports, and select referenced skills when
the bundle slash command is invoked. This preserves Hermes' ergonomic workflow
packs while keeping OpenClaw-style supply-chain caution: bundles do not install
skills, execute scripts, contact registries, or mutate the system prompt.

2026-05-30 context-references follow-up: Hermes' context reference docs expose
`@file:path`, `@file:path:10-25`, and `@folder:path` as inline context
attachments, alongside broader `@diff`, `@staged`, `@git:N`, and `@url:`
references. The same docs emphasize 1-indexed inclusive line ranges, bounded
folder listings, trailing punctuation stripping, path traversal protection,
binary rejection, and sensitive credential-path blocking. GitClaw should copy
the ergonomic local-file subset first: expand `@file:` and `@folder:` from
GitHub issue text against the checked-out repo, keep folder references as
metadata listings only, block credential-ish paths, and surface body-free
reference metadata in `@gitclaw /context` reports. Leave `@url` for later
because fetching web content expands the network and prompt-injection surface.

2026-05-30 git-context follow-up: Hermes' reference surface also makes current
git state first-class through `@diff`, `@staged`, and `@git:N`; OpenClaw's
diffs plugin frames patches as read-only artifacts that agents can inspect or
render without mutating the workspace. GitClaw can safely adopt the Git-native
subset now because GitHub Actions already checks out a repo: `@diff` and
`@staged` should run bounded read-only `git diff` commands, and `@git:N`
should expose a bounded recent commit log with patches, clamped to 10 commits.
The issue-visible `/context` report should remain body-free and show only kind,
count, status, sizes, and hashes.

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

2026-05-30 memory-info follow-up: OpenClaw/Hermes inspection surfaces make
focused provenance cards useful when an operator wants one durable context
file, not a full inventory. GitClaw should add `@gitclaw /memory info <path>`
and `gitclaw memory info <path>` for one body-free memory file card with
normalized path, source/kind, canonical/latest/loaded state, byte/line counts,
and hash metadata.

2026-05-30 memory-verify follow-up: add `@gitclaw /memory verify` as the
issue-side mirror of `gitclaw memory verify`, returning memory-file trust
cards, repo-local provenance, loaded/omitted state, hashes, hygiene rollup, and
external memory-provider/indexing non-goals without exposing raw memory text.

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

2026-05-30 doctor-list follow-up: health visibility should be usable before an
issue exists. Add `gitclaw doctor list` as the local mirror of `/doctor`: the
same config/workflow/context/skill/memory/proactive/validation rollup, but with
`scope: local-cli` and no repository, issue, or issue-title metadata.

2026-05-30 tools-validate follow-up: validation should be addressable as its
own issue command, not only embedded inside `/tools`. Add
`@gitclaw /tools validate` as the issue-side mirror of
`gitclaw tools validate`, returning only contract/output counts and body-free
findings so maintainers can audit the tool surface without the full inventory.

2026-05-30 skills-validate follow-up: local skills are code-like authority, so
their validation deserves a dedicated issue command. Add
`@gitclaw /skills validate` as the issue-side mirror of
`gitclaw skills validate`, returning only validation counts and findings
without dumping full `SKILL.md` bodies or inventory sections.

2026-05-30 skills-check compatibility follow-up: OpenClaw's CLI uses
`skills check` language for local skill health. GitClaw should accept
`gitclaw skills check` and `@gitclaw /skills check` as aliases for the same
validation-only report as `skills validate`, preserving body-free output while
making migration muscle memory work.

2026-05-30 soul-validate follow-up: high-authority context validation should
also be separately addressable from inventory. Add `@gitclaw /soul validate`
as the issue-side mirror of `gitclaw soul validate`, returning required-file,
memory-note, status, and body-free finding counts without listing context
files when there are no findings.

2026-05-30 soul-verify follow-up: add `@gitclaw /soul verify` as the
issue-side mirror of `gitclaw soul verify`, returning the repo-local trust
envelope for high-authority context with trust cards and hashes while keeping
raw soul, user, memory, tools, heartbeat, issue, and comment bodies out of the
GitHub comment.

2026-05-30 soul-risk follow-up: add `@gitclaw /soul risk` as the issue-side
mirror of `gitclaw soul risk`, returning only persistent-state risk counts,
codes, paths, severities, and line hashes for high-authority context. This
surface must include a real GitHub Models follow-up E2E in the same batch so
deterministic leakage checks are paired with actual inference and tool-grounded
conversation testing.

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

2026-05-30 channel-verify follow-up: channel support needs a positive health
check, not only inventory. Add `/channels verify` and `gitclaw channels verify`
as a body-free bridge verifier for the GitHub-native equivalent of a gateway
connection check: channel-ingest workflow present, `workflow_dispatch` enabled,
`actions: write` and `issues: write` permissions, normalized channel inputs,
and Telegram/Slack/generic provider keys visible before real pollers depend on
them.

2026-05-30 channel-info follow-up: provider-specific bridges need a narrow
contract card before pollers land. Add `/channels info <provider>` and local
`gitclaw channels info <provider>` to expose Telegram/Slack/generic secret
names, offset/thread/message keys, workflow metadata, gateway strategy, and
command shapes without leaking bodies or credential values.

2026-05-30 channel-risk follow-up: OpenClaw and Hermes both assume a gateway
can see untrusted external chat messages before handing them to an agent.
GitClaw's GitHub-native equivalent should make that boundary inspectable with
`@gitclaw /channels risk` and `gitclaw channels risk`: scan provider contracts,
workflow-dispatch bridge workflows, and prompt-visible `gitclaw:channel-message`
comments for prompt-boundary overrides, secret exfiltration, credential
exposure, raw body logging, channel-body execution, webhook exposure, and
unbounded gateway loops. Keep the report body-free by publishing only provider
names, workflow paths, comment IDs, counts, codes, severities, and hashes, and
pair every implementation batch with a real GitHub Models E2E after the
deterministic report.

### Multi-Agent Routing

OpenClaw's multi-agent model treats each agent as a full isolated persona scope:

- separate workspace,
- separate state directory,
- separate session store,
- separate auth profiles,
- channel/account bindings.

This lets one Gateway serve several people/personas, but also makes routing, auth, and memory boundaries load-bearing.

2026-05-30 agents-surface follow-up: current OpenClaw multi-agent docs frame an
agent as a full isolated workspace, state directory, session store, auth
profile set, and deterministic channel/account binding. Hermes splits the
adjacent idea into short-lived `delegate_task` subagents with fresh isolated
context and durable Kanban workers with named profiles, task rows, comments,
heartbeats, retries, and handoff metadata. GitClaw should not implement either
runtime primitive in v1. The GitHub-native slice is a body-free `/agents`
audit: `.gitclaw/AGENTS.md` plus `.gitclaw/agents/*.md` declare reviewed
single-assistant policy, active runtime is `github-actions`, and reports make
`multi_agent_delegation_supported=false`, `subagent_execution_supported=false`,
`delegate_task_supported=false`, and `remote_agent_process_allowed=false`
explicit. Any future routing/delegation batch must be paired with a live GitHub
Models conversation E2E, not just a deterministic report.

2026-05-30 agents-risk follow-up: OpenClaw sub-agents are background runs
spawned from an existing agent with their own sessions, optional forked
context, `sessions_spawn`, `sessions_yield`, delegation-mode prompt guidance,
and ACP/native runtime options. Hermes exposes the adjacent shape through
delegation patterns, named profiles with their own state directories and
gateways, and Kanban workers. GitClaw should keep v1 narrower: agent files are
reviewed metadata for the single GitHub Actions assistant, not process,
profile, or worker definitions. Add an `agents risk` report that scans
`.gitclaw/AGENTS.md` and `.gitclaw/agents/*.md` for prompt-boundary overrides,
credentials, untrusted issue-body execution, subagent/delegation enablement,
external agent processes, shared credential/session/memory state, raw payload
logging, webhook bridges, repository mutation, missing approval or
single-assistant boundaries, and unbounded loops. Acceptance should include the
deterministic body-free audit plus a real GitHub Models follow-up proving
inference and repo-search tool exposure.

2026-05-30 nodes-runtime follow-up: OpenClaw's node host docs expose a separate
execution plane: a headless node service connects to the Gateway WebSocket,
pairs as `role: node`, advertises capabilities, and can run approved
`system.run`/`system.which` calls on another machine. The broader nodes docs add
camera, screen, location, notification, browser-proxy, Canvas, and platform
permission gates, with command policy split between node-declared commands and
gateway allow/deny policy. Hermes' adjacent durable-worker story is Kanban:
workers are full OS processes with named profiles and tool-based board access,
while `delegate_task` children are fresh-context subagents that are synchronous
and non-durable. GitClaw's GitHub-native v1 should keep runtime nodes narrow:
`.gitclaw/NODES.md` plus `.gitclaw/nodes/*.md` declare reviewed node intent,
`@gitclaw /nodes` reports `active_node_runtime=github-actions-ephemeral-job`,
and reports make `gateway_websocket_required=false`,
`headless_node_host_supported=false`, `node_pairing_supported=false`,
`node_rpc_supported=false`, and `remote_node_exec_supported=false` explicit.
Any future node-host, socket, or remote-exec batch must include a live GitHub
Models conversation E2E in addition to deterministic report coverage.

2026-05-30 nodes-risk follow-up: OpenClaw node hosts connect to the Gateway
WebSocket, pair as `role: node`, can expose `system.run`/`system.which`,
`node.invoke`, browser proxy, and device/media capability surfaces, and can be
installed as long-running services. Hermes' adjacent worker lane model spawns
profile-backed or plugin-backed processes for Kanban cards and records worker
logs, task events, heartbeats, crashes, and timeouts. GitClaw should keep v1
strictly GitHub-native: `.gitclaw/NODES.md` and `.gitclaw/nodes/*.md` are
reviewed metadata for ephemeral GitHub Actions jobs, not WebSocket clients,
paired devices, node hosts, or worker lane definitions. Add a `nodes risk`
report that scans policy/spec files for prompt-boundary overrides,
credentials, untrusted issue-body execution, Gateway WebSocket node hosts,
remote node exec, pairing or auto-approval, browser proxy, media/device
capabilities, external worker lanes, raw payload logging, repository mutation,
missing approval or ephemeral-job boundaries, and unbounded loops. Acceptance
should include the deterministic body-free audit plus a real GitHub Models
follow-up proving inference and repo-search tool exposure.

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

2026-05-30 tools-verify follow-up: OpenClaw treats tools as typed functions
sent to the model and separates them from skills/plugins, while Hermes exposes
toolsets and platform-specific registries. GitClaw should add
`@gitclaw /tools verify` and `gitclaw tools verify` as a body-free trust
envelope that reports built-in contract modes, read-only/metadata-only counts,
repo-local `.gitclaw/TOOLS.md` provenance, known versus unknown active outputs,
input/output hashes, and explicit external-registry/runtime-attestation
non-goals without printing raw tool inputs or output bodies.

2026-05-30 tools-risk follow-up: OpenClaw's current plugin-permission and
hook docs make `before_tool_call` gates, approval elicitations, and runtime
tool interception explicit, while Hermes' tools reference exposes a large
built-in and MCP-loaded tool registry. GitClaw should add `@gitclaw /tools
risk` and `gitclaw tools risk` as the no-server equivalent of a tool/MCP
security audit: scan deterministic contracts, `.gitclaw/TOOLS.md`, and active
prompt-visible tool input/output metadata for prompt-boundary overrides,
secret exfiltration, credential exposure, host execution, repository mutation,
remote exfiltration, unbounded loops, and unknown tool-output provenance. The
report should publish only names, paths, fields, counts, codes, severities, and
hashes, and each implementation batch must pair the deterministic report with a
real GitHub Models conversation E2E.

2026-05-30 turn-provenance follow-up: OpenClaw/Hermes-style tool registries are
useful only when an operator can prove what the model actually saw. GitClaw
should therefore add body-free provenance to normal assistant-turn markers:
prompt-context hash, context document count, selected skill count/names, and
tool output count/names. This gives live E2E tests a stronger oracle for real
tool/skill visibility without exposing prompt bodies or raw tool outputs.

2026-05-29 tool-validation follow-up: OpenClaw's exec approval docs treat tool
execution as a stacked policy/allowlist/approval decision, and Hermes separates
toolsets such as terminal, file, web, cron, memory, and messaging by platform
availability. GitClaw should keep v1 narrower: validate that declared contracts
are only `read-only` or `metadata-only`, active outputs are declared and
bounded, `.gitclaw/TOOLS.md` is loaded, and `/tools` plus
`gitclaw tools validate` expose the result without dumping output bodies.

2026-05-30 sandbox-report follow-up: OpenClaw's sandboxing docs separate host
gateway execution from Docker/SSH/OpenShell sandboxes and warn that sandboxing
is blast-radius reduction, not a perfect boundary. Its exec approvals docs
also treat command execution as an explicit allowlist/approval decision.
Hermes' security docs similarly separate authorization, environment isolation,
container isolation, and dangerous command approval. GitClaw should expose the
current narrower truth with `@gitclaw /sandbox` and `gitclaw sandbox verify`:
GitHub Actions is the ephemeral runtime, no shell/exec/write/PR tool exists in
v1, deterministic tools are read-only or metadata-only, workflow permissions
are visible as body-free cards, and backup write permission is isolated to the
post-handle backup job.

2026-05-29 tools-search follow-up: OpenClaw's tools docs distinguish
tool-policy visibility from skill/plugin instructions, and Hermes' tool
reference keeps tool names and schemas inspectable as first-class registry
metadata. GitClaw should add the same operator affordance without exposing
prompt internals: `@gitclaw /tools search <query>` and `gitclaw tools search
<query>` should search contract names/modes/triggers and active-output
names/inputs, but report only match fields, counts, hashes, and sizes.

2026-05-30 tools-run-plan follow-up: OpenClaw's exec-approval model makes
tool execution an inspectable policy decision before action, while Hermes'
tool registry makes tool names, modes, and platform constraints explicit.
GitClaw should add `@gitclaw /tools run-plan <name>` and
`gitclaw tools run-plan <name>` as the GitHub-native equivalent of a
body-free dry run: one contract, gate state, trigger, mutation flag,
active-output hashes, validation summary, no shell/network/repository/model
execution, and an explicit requirement that tool-behavior changes also pass a
live GitHub Models conversation E2E.

2026-05-30 migration-plan follow-up: OpenClaw's migration posture is
preview-first, secret-redacted, and backup-backed, while Hermes' profiles keep
agent state isolated by config, `.env`, `SOUL.md`, memories, sessions, skills,
cron jobs, and gateway state. GitClaw should add `@gitclaw /migrate plan
<source>` and `gitclaw migrate plan <source>` as the GitHub-native equivalent:
map `openclaw`, `hermes`, `codex`, and `claude` state into repo-local targets,
but only as a deterministic body-free plan. The command should not scan source
home directories from an issue, import secrets, execute hooks/plugins/MCP
servers/installers, mutate the repository, or call a model. Any implementation
batch for migration behavior must pair the deterministic migration E2E with a
live GitHub Models conversation E2E that performs an actual LLM call.

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

2026-05-30 context-info follow-up: OpenClaw-style context debugging benefits
from focused cards, not just inventories. Add `@gitclaw /context info <path>`
and `gitclaw context info <path>` so maintainers can inspect one loaded context
file, selected skill document, or deterministic read-file output by path/name
with only kind, path/tool metadata, byte/line counts, and short hashes.

2026-05-30 skill-gating follow-up: OpenClaw-style skill systems treat skill
availability as policy, not only discovery. Add repo-reviewed
`skills.allowed` and `skills.disabled` gates so local skills remain inspectable
and hashable, but disabled or allowlist-blocked skill bodies never load into
prompt context even when mentioned or marked always-on.

2026-05-30 skill-select-plan follow-up: Hermes profiles and skill bundles make
task-specific skill loading powerful but also easy to misread. Add
`@gitclaw /skills select-plan <name>` and `gitclaw skills select-plan <name>`
as the body-free dry run for progressive disclosure: one skill, selected state,
gate state, bundle/request/always selection reasons, validation summary, no
model call, no repo mutation, no raw request text, and no `SKILL.md` body dump.

2026-05-30 tool-gating follow-up: Hermes/OpenClaw toolsets are useful partly
because the operator can see and constrain what the agent may call. GitClaw
should mirror that with repo-reviewed `tools.allowed` and `tools.disabled`
config for deterministic built-ins: disabled tools remain visible in reports,
but prompt-visible outputs are not generated.

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

2026-05-30 skill-verify follow-up: current OpenClaw docs expose
`openclaw skills verify` for registry trust envelopes and warn that third-party
skills are untrusted code; Hermes documents skill lifecycle/provenance as a
background system. GitClaw should keep the MVP narrower but still expose an
explicit `@gitclaw /skills verify` and `gitclaw skills verify` report:
repo-local source roots, body hashes, requirement status, validation rollup,
and `registry_verification=not_configured` without installing, executing, or
dumping skill bodies.

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

2026-05-30 channel dedupe follow-up: real Telegram/Slack bridges must tolerate
provider retries. A repeated `channel + message_id` should be visible as a
deduped ingest result, not another mirrored comment or agent wakeup. The
serverless workflow can skip the downstream `workflow_dispatch` when
`gitclaw channel-ingest` reports `duplicate=true`.

2026-05-30 channel command follow-up: OpenClaw-style gateways let the channel
message be the active request. GitClaw should do the same for
`workflow_dispatch` bridge wakeups by matching the dispatch ID to the mirrored
`gitclaw:channel-message` marker. That allows model-free slash commands from
Telegram/Slack and gives channel E2E tests a stable path during model
rate-limit spikes.

2026-05-30 channel state follow-up: Telegram long polling and Slack gateway
experiments need durable offset/dedupe state before any runner loop can be
trusted. GitClaw should store this as one GitHub issue per
`channel + account_sha256_12`, with `gitclaw:channel-state-update` comments for
new offsets. Account IDs and offsets should appear only as short hashes so the
state issue stays auditable without becoming a plaintext credential or cursor
store.

2026-05-30 channel state workflow follow-up: the no-server version should expose
channel offset writes through a `workflow_dispatch` wrapper as well as the local
CLI. Provider pollers, manual bridge experiments, and future self-renewing
gateway jobs can then update state with the repository `GITHUB_TOKEN`, keeping
the bridge architecture GitHub-native rather than webhook/socket-hosted.

2026-05-30 channel gateway follow-up: OpenClaw's gateway and Slack Socket Mode
loop can be approximated in GitHub Actions as a renewable lease rather than a
server. A `gitclaw channel-gateway` command should first record one
hash-only lease in the channel-state issue; the workflow wrapper can optionally
`workflow_dispatch` its successor with `actions: write`. Provider sockets and
pollers can be added behind that lease once the renewal surface is proven live.

2026-05-30 channel delivery follow-up: OpenClaw's gateway owns both inbound
delivery and outbound replies. GitClaw should record outbound reply delivery as
a GitHub-native receipt: verify the source `gitclaw:assistant-turn`, write one
`gitclaw:channel-delivery` marker to the channel-state issue, hash the provider
message id, and dedupe by source issue/comment. This lets Telegram/Slack
gateways retry safely without turning channel state into a plaintext transcript.

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

2026-05-30 artifact-governance follow-up: OpenClaw backup/migrate flows and
Hermes sessions/checkpoints make exportable evidence useful, but GitClaw's
issue-native shape needs a body-safe audit surface. `@gitclaw /artifacts`
should inspect `.gitclaw/ARTIFACTS.md`, artifact specs, `actions/upload-artifact`
version, retention, missing-file behavior, label gates, and redaction/approval
metadata without reading or printing uploaded artifact bodies. Each artifact
feature batch should pair deterministic artifact-report E2E coverage with a
real GitHub Models conversation E2E.

2026-05-30 artifact-risk follow-up: OpenClaw's backup/export affordances and
Hermes' session/checkpoint evidence both create pressure to store rich run
artifacts. GitClaw should keep that power narrow: `@gitclaw /artifacts risk`
should scan artifact policy, prompt-artifact specs, and upload workflows for
prompt-boundary overrides, credential material, unredacted prompts, raw payload
logging, hidden state, external storage, retention, label-gate, missing-file,
approval/redaction, repository mutation, and unbounded-collection risks while
only reporting metadata, codes, severities, and line hashes. Acceptance needs a
deterministic body-free report plus a live GitHub Models follow-up E2E.

2026-05-30 diff-governance follow-up: OpenClaw's diffs plugin treats change
content as a read-only diff artifact, while Hermes recommends previewing
checkpoint changes with `/rollback diff` before restoring. GitClaw's issue-safe
version should be `@gitclaw /diffs`: report `.gitclaw/DIFFS.md`, diff specs,
git status, numstat totals, changed paths, raw-patch suppression, and
non-mutating boundaries without printing patch hunks or file bodies. Each diff
feature batch should still run a real GitHub Models conversation E2E.

2026-05-30 diff-risk follow-up: OpenClaw-style diff viewers and Hermes-style
rollback previews are useful precisely because they show what changed before
recovery or write actions. GitClaw should add `@gitclaw /diffs risk` as the
safety twin of `/diffs`: scan diff policy, working-tree specs, and git metadata
for raw patch leakage, destructive git actions, hidden state, untracked-file
body context, external diff storage, missing approval gates, unsafe raw-patch
modes, and unbounded file collection while reporting only metadata, paths,
codes, severities, and line hashes. Acceptance requires deterministic body-free
coverage plus a live GitHub Models follow-up E2E.

2026-05-30 workspace-governance follow-up: OpenClaw treats the workspace as the
agent's home for file tools and context, while Hermes recommends separate git
worktrees so agent sessions get isolated checkouts and rollback history.
GitClaw's serverless version should expose `@gitclaw /workspace`: report
`.gitclaw/WORKSPACE.md`, `.gitclaw/workspaces/*.md`, git repository metadata,
bounded repository inventory counts, context allowlist counts, and workflow
checkout/setup-go/fetch-depth metadata without printing file bodies, workflow
bodies, issue bodies, or treating the Actions checkout as private durable
memory. Each workspace feature batch should pair deterministic workspace-report
E2E coverage with a real GitHub Models conversation E2E.

2026-05-30 workspace-risk follow-up: The OpenClaw workspace/file-tool model and
Hermes worktree isolation docs point to the same risk boundary: workspace state
is powerful context, but it should not silently become private memory, an
external mount, a daemon, or a raw body dump. GitClaw should add
`@gitclaw /workspace risk` and `gitclaw workspace risk`: scan workspace policy,
workspace specs, and workflow metadata for prompt-boundary overrides,
credential material, private workspace memory, external mounts, destructive
mutation, long-running services, raw body leakage, checkout/setup/fetch-depth
drift, missing approval gates, and unbounded repository inventory while
publishing only metadata, paths, counts, codes, severities, and line hashes.
Acceptance requires deterministic body-free coverage plus a live GitHub Models
follow-up E2E that proves actual model inference and tool visibility.

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

2026-05-30 secrets-audit follow-up: OpenClaw's current secrets docs make
`openclaw secrets audit --check` part of the normal operator loop, scanning for
plaintext residues, unresolved refs, precedence drift, legacy residues, and
name-heuristic sensitive provider headers before secrets are considered
migrated. Its gateway docs are explicit that SecretRefs are not a process
isolation boundary if plaintext credentials remain readable by the agent.
Hermes' current security docs similarly treat environment-variable isolation,
container isolation, context-file scanning, and cross-session isolation as
separate layers rather than one magic secret boundary. GitClaw should copy the
safe read-only half first: `@gitclaw /secrets` and `gitclaw secrets audit`
scan the checked-out repo for obvious plaintext residue and GitHub Actions
secret references, but report only paths, line numbers, codes, counts, and
hashes. Do not add configure/apply/reload until there is a reviewed SecretRef
design and a live migration rollback story that never stores historical
plaintext backups.

2026-05-30 checkpoint-readiness follow-up: Hermes' checkpoint/rollback surface
is useful because long-horizon agents need recoverable state transitions, not
just optimistic action. OpenClaw's approval and sandboxing docs reinforce the
same separation: preview evidence, approval, and mutation should be distinct
steps. GitClaw's safe GitHub-native equivalent should start as a read-only
rollback-readiness report: inspect `git status`, HEAD, recent commits, and the
expected backup branch without printing diffs or file bodies and without
running `reset`, `restore`, or checkout commands. This gives future write-mode
turns a visible checkpoint gate while keeping rollback itself a reviewed human
operation.

2026-05-30 checkpoint-risk follow-up: Hermes' rollback model treats restore as
dangerous enough to require preview and checkpoint evidence, while OpenClaw's
write approval posture keeps mutation separate from inspection. GitClaw should
add `@gitclaw /checkpoints risk` and `gitclaw checkpoints risk`: scan git
checkpoint metadata for missing auditability, dirty worktrees, raw diff or file
body exposure, restore/reset/clean/checkout authority, shadow-store path
leakage, and missing rollback safety gates while reporting only metadata,
counts, commit hashes, risk codes, and severities. Acceptance requires
deterministic body-free coverage plus a live GitHub Models follow-up E2E.

2026-05-30 approval-readiness follow-up: OpenClaw's exec approvals treat command
execution as a policy decision layered with user approval, while Hermes frames
dangerous commands as an explicit authorization boundary. GitClaw should expose
that boundary before it grows write mode: `@gitclaw /approvals` reports trusted
actor state, write-request detection, per-issue approval labels, and the
read-only write-mode block, but never approves, mutates, executes, or prints raw
issue/comment/prompt text. Local `gitclaw approvals list|verify` should mirror
the static approval shape without issue-only state.

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
- OpenClaw standing orders docs: https://docs.openclaw.ai/automation/standing-orders
- OpenClaw heartbeat docs: https://openclawlab.com/en/docs/agent/heartbeat/
- OpenClaw memory docs: https://docs.openclaw.ai/concepts/memory
- OpenClaw tools overview: https://docs.openclaw.ai/tools
- OpenClaw automation hooks docs: https://docs.openclaw.ai/automation/hooks
- OpenClaw plugin hooks docs: https://docs.openclaw.ai/plugins/hooks
- OpenClaw plugins docs: https://docs.openclaw.ai/plugins
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
- OpenClaw agent workspace docs: https://docs.openclaw.ai/agent-workspace
- OpenClaw migrating from Hermes: https://docs.openclaw.ai/install/migrating-hermes
- OpenClaw migrate CLI docs: https://docs.openclaw.ai/cli/migrate
- OpenClaw models CLI docs: https://docs.openclaw.ai/cli/models
- OpenClaw node host CLI docs: https://docs.openclaw.ai/cli/node
- OpenClaw multi-agent routing docs: https://docs.openclaw.ai/concepts/multi-agent
- OpenClaw nodes CLI docs: https://docs.openclaw.ai/cli/nodes
- OpenClaw diffs plugin docs: https://docs.openclaw.ai/vi/tools/diffs
- OpenClaw secrets CLI docs: https://docs.openclaw.ai/cli/secrets
- OpenClaw secrets management docs: https://docs.openclaw.ai/gateway/secrets
- GitHub Actions artifact storage docs: https://docs.github.com/en/actions/how-tos/writing-workflows/choosing-what-your-workflow-does/storing-and-sharing-data-from-a-workflow
- `actions/upload-artifact` action: https://github.com/actions/upload-artifact
- GitHub Models quickstart: https://docs.github.com/en/github-models/quickstart
- GitHub Models catalog REST API: https://docs.github.com/en/rest/models/catalog
- GitHub Models REST inference API: https://docs.github.com/en/rest/models/inference
- GitHub Models billing and rate-limit notes: https://docs.github.com/en/billing/concepts/product-billing/github-models
- Hermes docs index: https://hermes-agent.nousresearch.com/docs/llms.txt
- Hermes GitHub and README: https://github.com/NousResearch/hermes-agent
- Hermes architecture docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/developer-guide/architecture.md
- Hermes sessions docs: https://hermes-agent.nousresearch.com/docs/user-guide/sessions
- Hermes profiles docs: https://hermes-agent.nousresearch.com/docs/user-guide/profiles
- Hermes checkpoints and rollback docs: https://hermes-agent.nousresearch.com/docs/user-guide/checkpoints-and-rollback
- Hermes git worktrees docs: https://hermes-agent.nousresearch.com/docs/user-guide/git-worktrees
- Hermes migrate from OpenClaw docs: https://hermes-agent.nousresearch.com/docs/guides/migrate-from-openclaw
- Hermes memory docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/memory.md
- Hermes three-layer memory explainer: https://hermes-agent.ai/blog/hermes-agent-memory-system
- Hermes skills docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/skills.md
- Hermes working with skills docs: https://hermes-agent.nousresearch.com/docs/guides/work-with-skills/
- Hermes tools docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/tools/
- Hermes features overview: https://hermes-agent.nousresearch.com/docs/user-guide/features/overview/
- Hermes tools reference: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/reference/tools-reference.md
- Hermes subagent delegation docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/delegation
- Hermes Kanban docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/kanban
- Hermes security overview: https://hermes-agent.nousresearch.com/docs/
- Hermes cron docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/cron.md
- Hermes cron internals docs: https://hermes-agent.nousresearch.com/docs/developer-guide/cron-internals
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
- Security of OpenClaw Agents: https://arxiv.org/abs/2605.25435
- OpenClawBench: https://arxiv.org/abs/2605.29253
- Cloud Security Alliance Hermes/OpenClaw research note: https://labs.cloudsecurityalliance.org/wp-content/uploads/2026/05/CSA_research_note_hermes_agent_CVEs_20260504-csa-styled.pdf
