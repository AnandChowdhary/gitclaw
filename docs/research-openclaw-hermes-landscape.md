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

2026-05-31 hook-provenance follow-up: OpenClaw splits internal hooks, typed
plugin hooks, and diagnostic events, and its hook CLI can list and inspect
file-based hook surfaces. Hermes has lifecycle hooks around tool calls, LLM
calls, session start/end, and gateway dispatch, with shell-hook consent as an
explicit review boundary. For GitClaw, the useful v1 slice is not executable
hooks; it is a body-free git provenance report for reviewed hook policy/spec
files. The agent should expose hashes, tracked state, approval/audit-only
metadata, and risk codes, while keeping hook bodies, handler bodies, commit
subjects, and author identities out of issue-visible output.

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

2026-05-31 skill-source follow-up: OpenClaw's ClawHub skill format treats
runtime metadata, install specs, and trust verification as review inputs, while
Hermes exposes official/community skill sources, taps, direct GitHub installs,
well-known indexes, and trust levels. GitClaw should keep the provenance
pressure without adding a live registry client: add reviewed
`.gitclaw/skill-sources/*.yaml` source pins and expose
`@gitclaw /skills sources`, `@gitclaw /skills sources risk`, and
`gitclaw skills sources ...`. Reports should compare expected/current skill
hashes, show source kind, trust level, install mode, approval and no-fetch
gates, and publish only metadata, hashes, and risk codes. They must not contact
ClawHub, Hermes Hub, skills.sh, GitHub, or well-known endpoints; fetch remote
skills; run installers; mutate `.gitclaw/SKILLS`; or dump raw source refs,
source YAML, skill bodies, issue bodies, comments, prompts, credentials, or
provider payloads.

2026-05-31 skill-runtime follow-up: OpenClaw's `SKILL.md` frontmatter and
ClawHub format expose runtime metadata such as required env vars, binaries,
`primaryEnv`, and install specs, while Hermes skills/tool docs frame these
declarations as part of the capability and trust boundary. GitClaw should parse
that metadata for review but keep it inert: add `@gitclaw /skills runtime` and
`gitclaw skills runtime` to report env/bin/install declaration counts, hashes,
install kind names, primary-env consistency, and no-registry/no-install gates.
The report should never run installers, install dependencies, contact remote
registries, mutate skills, or print raw env names, install targets, skill
bodies, prompts, issue/comment bodies, credentials, provider payloads, or tool
outputs. Any change to this surface should be paired with a live GitHub Models
conversation E2E so the deterministic audit does not become a substitute for
testing real skill selection and tool usage.

2026-05-31 skill-upgrade-plan follow-up: OpenClaw's current skills CLI
documents `openclaw skills update <slug>` / `update --all` for tracked
ClawHub installs, while Hermes' skills guide tells users to update skills when
they go stale and describes installed skills as persistent, on-demand
procedural knowledge. GitClaw should keep the maintenance pressure but avoid
the risky parts in Actions: `@gitclaw /skills upgrade-plan <target>` and
`gitclaw skills upgrade-plan <target>` should require an existing repo-local
skill match, report only safe target/match metadata and hashes, never fetch
registries or remote URLs, never run installers, never mutate
`.gitclaw/SKILLS`, and require a live GitHub Models repo-reader/tool E2E after
planner changes or accepted skill edits. Sources: OpenClaw skills CLI
reference (`https://docs.openclaw.ai/cli/skills`) and Hermes Working with
Skills guide (`https://hermes-agent.nousresearch.com/docs/guides/work-with-skills/`).

2026-05-31 skill-install-plan E2E follow-up: OpenClaw's skills system allows
ClawHub, Git, and local directory installs, but its own docs also frame
third-party skills as untrusted code and note that installer/dependency paths
are a separate dangerous surface. Hermes similarly supports hub and direct URL
skill installs while relying on progressive disclosure and user-visible skill
lists. For GitClaw's serverless Actions model, the right v1 behavior is a
planner, not an installer: classify the requested target, derive a safe
repo-local path, show hashes and review gates, never fetch remote code or run
installers, and require a real GitHub Models repo-reader/tool follow-up after
planner changes so deterministic safety reports do not replace actual
conversation coverage.

2026-05-31 prompt-pack follow-up: OpenClaw's token-use docs make context
diagnostics a first-class operator surface: system prompt pieces, conversation
history, tool results, attachments, compaction summaries, and provider wrappers
all contribute to the context window, with a practical 4-chars-per-token
heuristic for OpenAI-style models. Hermes' context-compression docs add a
useful two-threshold model: the in-loop context compressor defaults to 50% of
context, while gateway session hygiene uses an 85% pre-agent safety threshold.
GitClaw should adapt this as `@gitclaw /prompt pack` and `gitclaw prompt pack`:
a body-free component map that shows fixed prompt order, byte ranges, hashes,
head/tail truncation projection, and 50%/85% threshold findings without
printing raw prompts, issue/comment bodies, context bodies, skill bodies, tool
outputs, raw tool inputs, or credentials. The feature should be paired with a
live GitHub Models follow-up E2E that proves actual selected-skill and
repository-search tool behavior.

2026-05-31 context-report hardening: diagnostic reports can accidentally leak
the same prompt material they are meant to audit if they print raw tool inputs.
Hermes-style progressive disclosure and OpenClaw-style context visibility point
to a safer rule for GitClaw: deterministic `/context` reports may name active
tools and output hashes, but raw tool inputs stay hashed because search queries
can be derived from issue bodies and comments.

2026-05-31 prompt-cache follow-up: OpenClaw's prompt-caching reference shows
that cache usefulness depends less on a magic toggle and more on stable
prefixes, provider-specific request fields, usage counters, cache-boundary
ordering, context-pruning after TTL windows, and heartbeat intervals that keep
valuable caches warm. Hermes' context compression/caching docs reinforce the
same interaction: compression, tool-result pruning, and prompt caching share a
context-management boundary. GitClaw should not claim cache support that
GitHub Models does not expose to the current client, but it can still make the
boundary inspectable: add `@gitclaw /prompt cache` and `gitclaw prompt cache`
as an observe-only, body-free report for stable same-issue prefix bytes,
dynamic tool/transcript suffixes, missing cache-control request fields, missing
cache read/write telemetry, heartbeat keep-warm workflow presence, and
LLM/tool E2E requirements. Reports must not dump prompts, issue/comment
bodies, context bodies, skill bodies, tool outputs, credentials, or secret
values.

2026-05-31 prompt-compression follow-up: Hermes documents context management
as two independent layers: gateway hygiene around 85% of the context window and
an in-loop compressor around 50%, with pluggable context engines that decide
when and how to compact. OpenClaw's token/cache docs separate provider usage
from live context, explain cache-TTL pruning, and keep cache/usage diagnostics
as explicit operator surfaces. GitClaw should add the same observability
without adding hidden mutable state: `@gitclaw /prompt compression` and
`gitclaw prompt compression` should report the current prompt envelope against
Hermes-style compression thresholds and OpenClaw-style pruning boundaries while
keeping compression disabled in v1. The report should expose only metadata,
hashes, thresholds, segment actions, truncation/omission counts, and runtime
gates; it must not create summaries, split sessions, write memory, depend on
an external session DB, or print prompt/context/tool/transcript bodies. Pair
the deterministic report with a live GitHub Models repo-reader/search
follow-up so compression diagnostics do not replace real inference coverage.

2026-05-31 skill-bundle provenance follow-up: Hermes' skills system treats
bundles as small YAML files that group several existing skills under one slash
command, skip missing skills rather than failing, and intentionally avoid
mutating the system prompt cache. That maps well to GitClaw's repo-native
model, but the bundle YAML itself becomes prompt-influencing state. GitClaw
should therefore add `@gitclaw /bundles provenance` and
`gitclaw bundles provenance`: report bundle counts, skill-ref resolution,
instruction hashes, git tracked/dirty state, commit IDs/dates, and
commit-subject hashes without printing raw bundle YAML, instructions, skill
bodies, issue/comment bodies, prompts, author identities, provider payloads,
credentials, or secret values. This keeps team-wide task profiles reviewable
as ordinary git files while rejecting Hermes-style agent-managed bundle writes
in the v1 GitHub-native runtime.

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

2026-05-31 backup-index E2E hardening: OpenClaw's manifest/backup posture and
Hermes' session-export posture both make the index a trust root for recovery.
GitClaw index changes therefore need two live proofs: the fetched
`gitclaw-backups` branch must contain the issue JSON plus body-free index and
README entries, and a normal GitHub Models follow-up must select repo-reader,
expose `gitclaw.search_files`, recover the backup-index repository-search
fixture token, and avoid hidden issue/comment token echoing.

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

2026-05-31 backup-report E2E hardening: The root backup report is the
operator's first pointer to raw transcript recovery state, so it should not be
weaker than the subcommands it introduces. OpenClaw-style preview/verify and
Hermes-style portable session export imply two proofs after changes: the issue
report must stay body-free while naming the backup branch and paths, and a
normal GitHub Models follow-up must select repo-reader, expose
`gitclaw.search_files`, recover the backup-report repository-search fixture
token, and avoid hidden issue/comment token echoing.

2026-05-29 backup-verify follow-up: OpenClaw's `backup verify` command checks
that an archive has exactly one manifest, rejects traversal-style paths, and
confirms every manifest-declared payload exists. GitClaw's git-native analogue
is not a tarball, so the invariant moves to the backup branch: verify
repo-scoped `index.json`, `README.md`, canonical `issues/000000.json` paths,
schema version, counts, timestamps, and absence of unindexed issue backups
before treating the branch as restorable.

2026-05-31 backup-verify E2E follow-up: the backup verifier is a restore gate,
not just an operator nicety. OpenClaw-style archive validation and Hermes-style
session export both imply that backup reports should prove branch integrity
before downstream coverage, search, restore, retention, or export commands run.
GitClaw now treats backup-verify changes as needing two proofs: a real fetched
`gitclaw-backups` branch audit and a normal GitHub Models repo-reader/search
follow-up so the model path still has tool provenance and usage telemetry.
The follow-up must keep hidden issue/comment sentinels on a distinct prefix
from the repository search fixture token; otherwise the test can accidentally
reward transcript echoing instead of tool-output grounding.

2026-05-30 backup-command follow-up: OpenClaw exposes backup verification as a
local command over an archive, while Hermes exposes session export as a local
JSONL artifact. GitClaw's issue handler runs before the backup branch update,
so issue-visible `/backup verify`, `/backup manifest`, `/backup search`, and
related subcommands should be treated as deterministic command intents: record
the exact branch paths, local command, privacy boundary, and hashes, then let
the post-turn backup job and fetched-branch CLI command perform the real audit.

2026-05-31 backup-export-jsonl follow-up: Hermes-style session export is an
explicit raw recovery path, not an issue-visible assistant report. GitClaw
should keep `@gitclaw /backup export-jsonl` body-free and deferred, then prove
the fetched `gitclaw-backups` branch can emit raw JSONL locally. Because raw
export is a high-trust backup surface, changes should also run a normal
GitHub Models repo-reader/search follow-up so model provenance, tool grounding,
and usage telemetry remain covered without leaking hidden issue sentinels.

2026-05-31 backup-search E2E follow-up: backup search is another raw-archive
recovery surface, even when the issue-visible report prints only a query hash
and term count. GitClaw should prove two things for search changes: the fetched
backup branch can find a planted raw token without leaking it, and a normal
GitHub Models follow-up still uses repo-reader search with prompt provenance,
selected tool metadata, and usage telemetry.

2026-05-31 backup-catalog follow-up: OpenClaw-style backup verification and
Hermes-style session export both work best when operators can first discover
the recovery surface without opening raw archives. GitClaw should expose a
body-free `@gitclaw /backup catalog` and `gitclaw backup catalog` that list
backup commands, branch paths, fetched-branch gates, and restore/retention
no-mutation boundaries. Catalog changes should prove deterministic issue
metadata, the post-turn `gitclaw-backups` update, and a normal GitHub Models
repo-reader/search follow-up.

2026-05-30 backup-risk follow-up: OpenClaw's backup verification treats unsafe
paths and malformed manifests as restore blockers, while Hermes' checkpoint and
session export posture makes rollback review a local operator action. GitClaw
should add `gitclaw backup risk`: verify the fetched `gitclaw-backups` branch,
scan indexed issue payloads for integrity, path-safety, credential-handling,
prompt-boundary, restore-safety, and retention risks, and report only paths,
counts, codes, severities, and hashes. Issue-side `/backup risk` stays a
deferred intent because the raw backup branch is only written after the
assistant turn.

2026-05-31 backup-provenance follow-up: OpenClaw's backup verify surface and
Hermes' checkpoint/session export docs both assume operators can prove state
before recovery. GitClaw's git-native backup store can add a stronger branch
provenance check: `gitclaw backup provenance` verifies the fetched
`gitclaw-backups` tree, then reports whether the index, README, and issue
payload files are tracked, clean, and backed by git commits. The report should
hash file contents and commit subjects, include short commit IDs and commit
dates, and explicitly omit raw backup bodies, raw commit subject text, and
author identities. Issue-side `/backup provenance` remains a deferred command
intent, and its live E2E should include a second GitHub Models/tool follow-up.

2026-05-29 backup-manifest follow-up: OpenClaw's manifest-centered backup
verification and Hermes' portable session export both point to a compact
provenance view. GitClaw should expose a local `backup manifest` command over
the fetched `gitclaw-backups` branch that lists control files, issue payload
paths, byte counts, counts, and hashes without dumping raw transcript bodies.
That gives restore reviews and mirrors a stable checklist without requiring
operators to open every raw issue JSON file.

2026-05-31 backup-manifest E2E hardening: Current OpenClaw backup docs still
center `manifest.json` plus payload validation, while Hermes' current skills
docs emphasize progressive disclosure and portable workflow context. GitClaw's
git-native manifest surface should therefore require two proofs after changes:
a live fetched-branch manifest report, and a normal GitHub Models follow-up
that selects repo-reader, exposes `gitclaw.search_files`, recovers the
backup-manifest repository-search fixture token, and avoids hidden issue/comment
token echoing.

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

2026-05-31 backup-stats E2E hardening: Repo-wide stats can guide backup-health
decisions, so changes must be proven beyond deterministic rendering. The live
harness should verify the fetched backup tree stats report, then post a normal
issue-comment follow-up that makes a GitHub Models call, selects repo-reader,
exposes `gitclaw.search_files`, recovers the backup-stats repository-search
fixture token, and avoids echoing hidden issue/comment tokens.

2026-05-31 backup-freshness follow-up: OpenClaw's verify-before-restore habit
and Hermes' session export/checkpoint posture both imply an operator should be
able to ask whether the backup branch is fresh enough without opening raw
payloads. GitClaw should expose `gitclaw backup freshness`: verify the fetched
`gitclaw-backups` tree, find the latest backup timestamp, compare its age to a
configurable max age, and report status, gate, timestamps, payload hash, and
title hash only. Changes need a live fetched-branch freshness proof plus a
normal GitHub Models repo-reader/search follow-up.

2026-05-31 backup-continuity follow-up: OpenClaw's cron CLI now exposes run
history, skipped/error state, and inspection commands for scheduled work, while
Hermes stores session timestamps and source metadata for resume/search/export.
GitClaw should expose the git-native backup equivalent as
`gitclaw backup continuity`: verify the fetched `gitclaw-backups` tree, sort
indexed backups chronologically, compute longest gaps and threshold violations,
and report only timestamps, issue numbers, paths, event names, and title hashes.
Changes need a live fetched-branch continuity proof plus a normal GitHub Models
repo-reader/search follow-up.

2026-05-30 backup-list follow-up: Hermes' session list and OpenClaw's backup
inspection surfaces both point to a compact index-first view before export or
restore. GitClaw should add `gitclaw backup list`: verify the fetched
`gitclaw-backups` branch, sort indexed backups by timestamp, and print issue
numbers, payload paths, timestamps, event names, label/comment/transcript
counts, and title hashes only. This keeps routine backup navigation body-free.

2026-05-31 backup-list E2E hardening: Treat indexed backup navigation as part
of recovery posture. Changes now need a live fetched-branch list proof plus a
normal GitHub Models follow-up that selects repo-reader, exposes
`gitclaw.search_files`, recovers the backup-list repository-search fixture
token, and avoids hidden issue/comment token echoing.

2026-05-31 backup-timeline follow-up: the latest OpenClaw release notes keep
emphasizing bounded release/QA/E2E proof lanes and a status surface for active
work, while OpenClaw's session CLI frames sessions as manageable/exportable
conversation state and its backup CLI centers manifest-backed verification.
Hermes' current docs still emphasize cross-session recall, scheduled
automations, portable skills, and progressive skill disclosure. GitClaw's
git-native version should therefore expose a body-free `gitclaw backup
timeline` over the fetched backup branch: verify first, select the most recent
backups, render them chronologically with gap seconds, counts, payload hashes,
assistant-turn counts, and title hashes, and never print raw issue, comment,
transcript, prompt, search, or tool bodies. Sources checked:
https://github.com/openclaw/openclaw/releases,
https://openclawlab.com/en/docs/cli/backup/,
https://openclaw.cc/en/cli/sessions,
https://hermes-agent.nousresearch.com/docs/, and
https://hermes-agent.nousresearch.com/docs/guides/work-with-skills/.

2026-05-30 backup-info follow-up: The same landscape also needs a focused
single-session card between list and raw export. GitClaw should add
`gitclaw backup info --issue <n>`: verify the fetched backup tree, locate one
canonical payload, and print payload bytes/hash, backup timestamp/event,
label/comment/transcript/message counts, assistant/error marker counts, and
body hashes only. This matches Hermes' session detail ergonomics while keeping
OpenClaw-style restore/export behind explicit commands.

2026-05-31 backup-info E2E hardening: Treat backup info as a recovery surface,
not just a metadata card. Changes now require two live proofs: the fetched
`gitclaw-backups` branch must render the focused body-free info report, and a
normal issue-comment follow-up must make a GitHub Models call, select the
repo-reader skill, expose `gitclaw.search_files`, recover the backup-info
repository-search fixture token, and avoid echoing hidden issue/comment tokens.

2026-05-29 backup-restore-plan follow-up: OpenClaw's migration/backup posture
emphasizes preview before state-changing recovery. GitClaw should copy that
separation: a local `backup restore-plan` command reads the backup branch and
prints a dry-run restore plan with counts and hashes, but makes no GitHub API
calls and does not dump raw issue/comment bodies. A future mutating restore can
require explicit approval and compare the restored issue against this plan.

2026-05-31 backup-restore-plan E2E follow-up: OpenClaw's current backup docs
still frame verification as an explicit operator action over a manifest-backed
archive, OpenClaw's transcript CLI frames stored transcripts as read-only
inspection artifacts, and Hermes' session docs frame saved conversations as
resume/search/export state rather than context that must be copied into every
turn. GitClaw's restore-plan surface should therefore advertise a mandatory
live LLM/tool follow-up: the deterministic restore plan proves backup recovery
metadata, then a normal GitHub Models issue-comment turn proves the assistant
can still use repo-reader search with prompt provenance, selected-skill
metadata, and usage markers after the backup inspection.

2026-05-29 backup-retention follow-up: OpenClaw's backup/restore commands keep
state changes behind explicit previews, while Hermes' session lifecycle docs
make cleanup/archival pressure visible through exported session artifacts.
GitClaw's git-native equivalent should start with a non-mutating
`backup retention-plan`: verify the fetched `gitclaw-backups` branch, sort
backups by timestamp, keep the latest N, list older payloads as prune
candidates, and expose only paths, counts, timestamps, and title hashes.

2026-05-31 backup-retention E2E follow-up: OpenClaw migration/backup docs keep
preview/verify steps before destructive state changes, while Hermes sessions
docs include expiry, cleanup, deletion, and JSONL export surfaces. GitClaw's
retention plan should therefore remain a dry-run delete-free report, but every
change to it should be paired with a normal GitHub Models repo-reader follow-up
that proves the deterministic backup cleanup planning path did not replace
live model/tool coverage.

2026-05-30 backup-search follow-up: OpenClaw's transcript/session CLIs and
Hermes' cross-session search both make old conversations discoverable. GitClaw
can copy the operator value without copying the storage shape: search the
fetched `gitclaw-backups` branch directly, verify it first, and return issue
paths, sources, trust metadata, scores, and body/line hashes only. Raw backup
JSON remains available through explicit local recovery/export commands, but the
default search report should be body-free and query-free.

2026-05-31 backup-coverage follow-up: OpenClaw's official ecosystem exposes
backup/verification as operator-facing control-plane work, while Hermes'
session-memory posture makes cross-session state useful only if the operator
can prove it exists. GitClaw should add a narrower git-native check:
`gitclaw backup coverage --issue <number>` verifies one conversation's
indexed, canonical, readable backup payload in a fetched `gitclaw-backups`
branch and emits only metadata, counts, timestamps, and hashes. The issue-side
`@gitclaw /backup coverage` command should record that local command before
the post-turn backup job writes the raw payload.

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

2026-05-31 session-catalog follow-up: OpenClaw's current session docs emphasize
bounded session lists, redacted trajectory tails, and cleanup/export gates,
while Hermes' session docs emphasize automatic saved sessions, deterministic
messaging-platform session keys, and cross-session search. GitClaw should add a
body-free `@gitclaw /session catalog` and `gitclaw session catalog` that list
the available session reports, backup-backed local commands, recall gates, and
no-export/no-delete boundaries before adding any separate session database.
Changes should prove deterministic issue metadata plus a normal GitHub Models
repo-reader/search follow-up.

2026-05-30 session-provenance follow-up: OpenClaw transcript artifacts and
Hermes saved sessions are useful because they make execution history
inspectable after the turn. GitClaw should fold its assistant-turn prompt
provenance marker into `@gitclaw /session`: count assistant turns with prompt
evidence, list unique prompt-context hashes, and show body-free skill/tool names
per assistant turn. This lets a later issue comment verify that a previous
model answer actually saw `gitclaw.search_files` or a selected skill without
replaying raw prompts.

2026-05-31 session-coverage follow-up: OpenClaw and Hermes both separate
operator-facing session inspection from the raw transcript store, but live E2E
needs a sharper gate than "a deterministic report ran." GitClaw should add
`@gitclaw /session coverage` plus local
`gitclaw session coverage --backup <issue.json>` so test harnesses can fail
unless the backed-up conversation contains a real model-backed turn, prompt
provenance, and expected prompt-visible skills/tools. This preserves
OpenClaw/Hermes-style replayability while making GitHub-native tests prove LLM
and tool usage without exposing raw prompts or transcript bodies.

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

2026-05-31 runs-report E2E hardening: OpenClaw's current sessions docs make
stored conversation rows bounded by default, with session exports and cleanup
as explicit operator actions, while Hermes-style sessions/checkpoints keep
prior turns inspectable without conflating inspection with mutation. GitClaw's
current-turn ledger should therefore prove two facts after changes: the
deterministic `/runs` report remains body-free and read-only, and a normal
GitHub Models follow-up can still select `repo-reader`, expose bounded
repository search, and recover the runs-report fixture token. That keeps the
ledger from becoming a disconnected diagnostic card and proves ordinary
LLM/tool execution still works after run-provenance changes.

2026-05-31 prompt-report E2E hardening: OpenClaw-style prompt inspection is
most useful when it reports budgets, hashes, and truncation pressure without
printing raw prompt text. Hermes-style bounded context has the same lesson:
diagnostics should make the envelope auditable without becoming a second
conversation channel. GitClaw's `/prompt` and `/prompt list` reports should
therefore ship with live follow-up proof that a normal GitHub Models turn can
still select `repo-reader`, expose `gitclaw.search_files`, and recover a
fixture token after the deterministic prompt report or alias runs.

2026-05-31 run-history follow-up: OpenClaw's trajectory/progress framing is
useful, but GitClaw should keep the first history surface narrower than a full
trace database. `/runs history` should reconstruct a body-free timeline from
prior `gitclaw:assistant-turn` markers only: run IDs, model names, deterministic
versus model-backed counts, prompt-context hashes, skill/tool names, idempotency
hashes, run-URL hashes, and comment hashes. This borrows Hermes' session-list
and session-show ergonomics without adding a daemon or raw transcript store.
The required E2E should include real GitHub Models turns before and after the
deterministic report so run history is proven against actual LLM/tool usage.

2026-05-31 session-status follow-up: Hermes' status/readback pattern is useful
as a compact operator view, but GitClaw should avoid a second transcript display.
`/session status` should report labels, transcript shape, latest user/assistant
message hashes, latest assistant marker provenance, and skill/tool turn counts
without printing the latest request, assistant reply, prompt, search query, or
tool output. The E2E should start from a real GitHub Models turn, run the
deterministic status report, and then perform another model/tool follow-up.

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

2026-05-31 memory-promote-plan E2E follow-up: to keep the GitHub-native design
honest, the dry-run planner now carries a planner-specific LLM-E2E flag and the
live harness exercises both halves of the memory-promotion contract: first the
body-free, non-mutating report, then a normal GitHub Models issue-comment turn
that selects `repo-reader`, exposes bounded search, and recovers only the
fixture token. This mirrors the OpenClaw/Hermes split between reviewed memory
promotion and ordinary model/tool conversation.

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

2026-05-31 soul-edit-plan E2E hardening follow-up: OpenClaw's identity
cascade means `SOUL.md` edits are prompt-shaping changes, and Hermes profiles
also keep `SOUL.md` beside memory/user state as durable agent behavior. The
planner must therefore stay non-mutating and body-free while the E2E proves the
normal runtime still performs a real model turn after the deterministic audit.
GitClaw's soul edit-plan harness should post a second issue comment that uses
repo-reader search and verifies prompt provenance, selected skills,
prompt-visible tools, and usage telemetry before a soul-planner change is
accepted.

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

2026-05-31 proactive-info E2E follow-up: OpenClaw's cron/reminder surfaces and
Hermes' scheduled goals both make scheduled work inspectable before it runs.
GitClaw's equivalent is not a hidden scheduler database: it is reviewed
workflow files plus `.gitclaw/proactive/*.md` prompts. The `proactive info`
surface should therefore stay body-free and metadata-only, but any change to it
should be proven with a real GitHub Models issue-comment follow-up that selects
`repo-reader`, exposes bounded repository search, and confirms scheduled-job
inspection still works on the same path users will use for normal conversation.

2026-05-31 proactive report/list E2E follow-up: [OpenClaw cron](https://docs.openclaw.ai/cron/)
keeps precise scheduled work distinct from
[OpenClaw heartbeat](https://docs.openclaw.ai/heartbeat), while
[Hermes cron](https://hermes-agent.nousresearch.com/docs/user-guide/features/cron/)
exposes scheduled jobs through an explicit tool and reusable workflow hints.
GitClaw should keep that operator visibility but translate it into GitHub
primitives: `/proactive` and `/proactive list` publish only reviewed workflow
and prompt metadata, then the live harness posts a normal issue-comment turn
that must use GitHub Models, `repo-reader`, and bounded repository search. That
guards against a fake proactive audit that only echoes deterministic metadata.

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
The runtime heartbeat marker should carry the same model id, prompt-context
hash, prompt-visible context counts, and token usage telemetry as ordinary
assistant turns, so scheduled comments can be audited without exposing prompt
or heartbeat file bodies.

2026-05-31 heartbeat-report E2E follow-up: Current [OpenClaw heartbeat docs](https://docs.openclaw.ai/heartbeat)
treat `HEARTBEAT_OK` as a quiet acknowledgement and describe deferral when
cron or session lanes are busy; [Hermes cron guidance](https://hermes-agent.nousresearch.com/docs/guides/automate-with-cron/)
emphasizes fresh scheduled runs rather than resident conversation loops.
GitClaw's heartbeat status report should therefore remain a cheap,
deterministic inventory, but its live acceptance should immediately continue
with a normal issue-comment model/tool turn. That proves the no-daemon
heartbeat surface can still rejoin the ordinary GitHub issue conversation path
after the operator has inspected it.

2026-05-31 heartbeat runtime E2E follow-up: the heartbeat runner itself needs
the same continuation proof, not only the `/heartbeat` report. After a real
workflow-dispatch heartbeat and duplicate-slot suppression, the live harness
should post a normal `@gitclaw` comment in the same heartbeat issue and require
a GitHub Models repo-reader/search answer. That proves scheduled wakeups can
hand the issue back to ordinary conversation without a resident daemon or
socket loop.

2026-05-30 heartbeat-risk follow-up: Current OpenClaw heartbeat docs frame
heartbeat as scheduled proactive awareness, `HEARTBEAT.md` context, and
`HEARTBEAT_OK` quiet suppression, while Hermes cron docs emphasize fresh
scheduled sessions, optional attached skills, no-agent watchdogs, and explicit
delivery suppression. GitHub Actions' `schedule` docs add an important
serverless reliability constraint: scheduled workflows can be delayed or
dropped under load, especially at the top of the hour, and only run from the
default branch. GitClaw should therefore add `@gitclaw /heartbeat risk` and
`gitclaw heartbeat risk`: a body-free audit that checks the heartbeat workflow
for dispatch/schedule presence, off-hour cron timing, minimal permissions,
concurrency/idempotency, self-dispatch loops, raw-input logging, and
credential leakage; checks `.gitclaw/HEARTBEAT.md` for prompt-boundary,
exfiltration, unreviewed persistent-state, and unbounded-scope instructions;
and pairs the deterministic audit with a real GitHub Models follow-up that
proves ordinary repo-reader/tool usage still works.

2026-05-29 workflow-dispatch follow-up: GitClaw needs a second fresh-run
boundary in addition to heartbeat. A channel poller that mirrors Telegram or
Slack messages using `GITHUB_TOKEN` cannot depend on those generated comments to
fire another `issue_comment` workflow, so the main issue handler needs an
explicit `workflow_dispatch` wakeup path. The useful OpenClaw/Hermes analogue is
not a socket loop; it is an auditable issue-number dispatch with a stable
external event ID used as the idempotency key.

2026-05-31 workflow-dispatch E2E hardening: GitHub's Actions docs continue to
make `workflow_dispatch` the explicit manual/API wakeup path and note that
`workflow_dispatch`/`repository_dispatch` are the events designed to create
fresh runs from a workflow-triggering call. OpenClaw's channel docs still frame
Slack and Telegram as gateway-owned adapters that normalize channel messages
before routing them to an agent session, while Hermes cron keeps scheduled work
as fresh agent sessions. GitClaw should keep the GitHub-native version small:
the generic dispatch harness first proves issue-number wakeup and stable
dispatch-id idempotency, then proves conversation continuity with a normal
GitHub Models repo-reader/search follow-up on the same issue. The harness must
also wait for the initial untriggered `issues.opened` run before adding the
trigger label; otherwise the issue-opened run can race with label mutation and
steal the first assistant turn from the manual dispatch proof. That makes the
no-server bridge useful for Slack/Telegram pollers without adding a long-lived
socket process.

2026-05-31 channels-info E2E follow-up: current OpenClaw channel docs continue
to frame Slack and Telegram as gateway-owned channel adapters, while GitClaw's
serverless version keeps GitHub issues as the canonical session and wakes the
handler through `workflow_dispatch`. The provider-info surface should stay
metadata-only: secret names, offset/thread/message keys, workflow bridge
presence, and command shapes. Because channel contracts are easy to break while
still looking fine in a dry run, changes to this surface should also run a real
GitHub Models follow-up that selects `repo-reader`, exposes bounded search, and
recovers only a fixture token.

2026-05-29 proactive usefulness follow-up: OpenClaw's automation categories and
Hermes' cron/goals both point to a useful GitClaw feature that is not just
heartbeat. Proactive jobs should be normal scheduled GitHub Actions workflows
that create or reuse GitHub issues, then dispatch the main issue handler. This
preserves the no-daemon architecture while allowing email triage, reminders,
watchers, and reports to initiate their own visible issue threads.
Because proactive runs are easy to fake with prompt echoing, live proactive
E2E should also prove repo-reader search, selected-skill metadata,
prompt-visible tool names, model provenance, and usage telemetry.
2026-05-31 proactive runtime E2E follow-up: the base proactive harness should
continue the created issue after duplicate-slot idempotency with a normal
`@gitclaw` comment. The follow-up must use a distinct repository-search
fixture so it proves the proactive issue can become an ordinary conversation,
not merely that the initial scheduled prompt produced one model answer.

2026-05-30 standing-orders follow-up: OpenClaw distinguishes standing orders
from cron, heartbeat, hooks, and task flow: they are durable authority programs
with scope, triggers, approval gates, and escalation rules. Hermes profiles make
the same lesson concrete by scoping SOUL, memories, sessions, skills, cron
jobs, and state per profile. For GitClaw, the useful translation is
`.gitclaw/STANDING_ORDERS.md`: a reviewed repo file loaded into model context,
plus `@gitclaw /orders` and `gitclaw orders list|verify` reports that audit
program clause coverage and proactive enforcement metadata without executing
orders or printing their bodies.

2026-05-30 standing-order-risk follow-up: OpenClaw's standing-order docs warn
against broad authority, missing escalation rules, and standing orders without
cron enforcement, while Hermes' persistent goals/cron model keeps scheduled
work scoped, fresh, and observable. GitClaw should add `@gitclaw /orders risk`
and `gitclaw orders risk`: scan `.gitclaw/STANDING_ORDERS.md` internally for
unbounded authority, prompt-boundary overrides, credential transfer, external
delivery, hidden persistence, host execution, unbounded retries, and skipped
verification, while publishing only metadata, finding codes, severities, paths,
program hashes, and line hashes. The E2E should pair the deterministic risk
report with a real GitHub Models follow-up proving repo-reader/tool usage.

2026-05-30 hooks follow-up: OpenClaw's hook docs split coarse file-based
internal hooks from typed plugin hooks and webhooks. Internal hooks react to
command, session, gateway, and message events and are inspected with
`openclaw hooks list|check|info`, but they are still executable integration
surface. GitClaw should start with declarative hook policy and specs:
`.gitclaw/HOOKS.md` plus `.gitclaw/hooks/*.md`, reported by `@gitclaw /hooks`
and `gitclaw hooks list|verify` without executing handlers. This preserves the
event-driven design lesson while keeping all side effects behind reviewed
GitHub workflows and approval gates.

2026-06-01 hooks-catalog follow-up: OpenClaw's current hook docs treat
file-based hooks as discoverable operator automation and distinguish them from
external webhooks and plugin-owned hook surfaces. Hermes' hook docs similarly
make hook visibility and synthetic testing explicit, with hook scripts kept in
auditable directories. GitClaw should add a compact hook catalog before any
runtime hook execution exists: `@gitclaw /hooks catalog` and
`gitclaw hooks catalog` should enumerate hook list/verify/risk/provenance
commands, policy/spec/event/approval/provenance layers, ignored handler files,
provider-payload non-ingest, and disabled execution/mutation gates. Reports
must never print hook bodies, handler bodies, provider payload bodies, issue or
comment bodies, prompts, tool outputs, credentials, or secrets. Acceptance
requires a live deterministic hooks-catalog issue plus a real GitHub Models
follow-up proving repo-reader search and token telemetry.

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

2026-05-31 MCP metadata follow-up: Hermes' MCP support and OpenClaw's plugin
contracts both make external capability discovery tempting, but GitClaw's
serverless issue loop should treat MCP as reviewed metadata before runtime.
Add `.gitclaw/mcp/*.yaml` specs and expose them through
`@gitclaw /plugins mcp`, `@gitclaw /plugins mcp risk`, and
`gitclaw plugins mcp ...`. The report can show spec names, paths, transport,
activation state, tool allowlist/denylist refs, secret-name refs, hashes, and
risk codes, but must not launch servers, connect clients, discover dynamic
tools, expose MCP tools to the model, pass env, mutate repositories, or print
raw command/URL/arg/spec bodies. Acceptance should include the deterministic
MCP audit plus a real GitHub Models follow-up proving ordinary repo-reader
tool behavior still works.

2026-05-31 MCP-provenance follow-up: Hermes' MCP docs emphasize reviewed
catalog entries, per-server filtering, runtime toolsets, dynamic discovery, and
env filtering, while OpenClaw separates tools, skills, and plugins so runtime
capabilities remain policy-visible. GitClaw should add
`@gitclaw /plugins mcp provenance` and `gitclaw plugins mcp provenance` as a
body-free git-history audit over `.gitclaw/mcp/*.yaml`: tracked/dirty state,
commit IDs/dates, subject hashes, launch-surface hashes, filter counts, risk
codes, and no raw commands, args, URLs, env values, credential values, git
subjects, author identities, server launch, MCP connection, dynamic discovery,
or model-visible MCP tools.

2026-05-31 tool-defer-plan follow-up: Hermes' Tool Search docs describe an
auto progressive-disclosure layer that replaces large deferrable MCP/plugin
tool schemas with search/describe/call bridge tools once the schema load is
large enough. OpenClaw's tools overview uses the same conceptual split:
tools are callable actions, skills teach workflows, and plugins add runtime
capabilities, with allow/deny/profile policy controlling visibility. GitClaw
should not expose runtime structured tools yet, but it should make the future
decision inspectable by adding `@gitclaw /tools defer-plan` and
`gitclaw tools defer-plan`: a body-free advisory report that combines built-in
tools, repo-reviewed toolsets, and MCP allowlist entries, estimates direct vs
deferred catalog entries, records bridge non-goals, and proves the change with
live GitHub Models repo-search E2E.

2026-05-31 tool-catalog follow-up: the same OpenClaw/Hermes split points to a
smaller always-available catalog before any bridge exists. GitClaw should add
`@gitclaw /tools catalog` and `gitclaw tools catalog` as a body-free compact
index over built-in deterministic contracts, repo-reviewed toolset profiles,
and MCP allowlist entries. The report should show direct/deferred mode,
schema-visibility mode, activation decision, gate state, reason codes, counts,
and hashes, while excluding raw schemas, toolset instructions, MCP command
args, tool inputs, tool outputs, issue bodies, comments, prompts, credentials,
and secrets. The live proof should pair the deterministic catalog issue with a
GitHub Models repo-reader/search follow-up.

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

2026-05-31 proactive-init E2E follow-up: [OpenClaw scheduled tasks](https://docs.openclaw.ai/cron-jobs)
store durable job definitions and run them as isolated scheduled work, while
[Hermes cron](https://hermes-agent.nousresearch.com/docs/user-guide/features/cron/)
lets a scheduled job attach explicit skills or run in no-agent mode. GitClaw's
generator should keep that distinction legible: local `proactive init` writes
reviewed prompt/workflow files and prints only hashes, then live acceptance
must dispatch a real proactive issue and continue it with a normal GitHub
Models repo-reader/search turn. That proves the generated scheduled job is not
just syntactically valid, but conversationally usable once GitHub Actions wakes
it.

2026-05-31 proactive not-before E2E follow-up: OpenClaw cron exposes skipped
runs as a meaningful `not-due` state, and Hermes cron separates scheduled
definition from each due execution. GitClaw's `--not-before` should therefore
be observable in both directions: a future run must produce `skipped=true`
and `issue_number=0` without creating any issue, while a due run must create
the normal proactive issue and continue into a GitHub Models repo-reader/search
turn. This keeps one-shot reminders cheap and serverless without letting a due
gate become an untested branch around the normal conversation path.

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

2026-05-30 model-risk follow-up: OpenClaw's model CLI separates read-only model
listing/status from live probes, and Hermes keeps provider/model wiring inside
profile config alongside credentials and model roles. GitClaw should add
`@gitclaw /models risk` and `gitclaw models risk` as the body-free version of
that operational check: verify provider family, endpoint host, token-source
name, fallback coverage, retry budget, prompt-artifact state, and config-file
metadata without calling catalog or inference endpoints. The report should
flag non-GitHub endpoints, insecure HTTP, missing budgets, duplicated or
unknown fallbacks, credential material in `.gitclaw/config.yml`, raw prompt
logging, live-probe requirements, and raw provider-error leakage, then pair the
deterministic audit with a real GitHub Models follow-up conversation E2E.

2026-05-31 model-cost follow-up: GitHub's direct GitHub Models billing docs now
publish a token-unit cost model: usage is converted through model-specific
input/cached-input/output multipliers, then multiplied by the fixed token unit
price. The direct-cost table at
`https://docs.github.com/en/billing/reference/costs-for-github-models` does not
list GitClaw's current smallest default, `openai/gpt-5-nano`, so GitClaw should
separate `/models cost` from `/models usage`, keep a reviewed local multiplier
snapshot, estimate only known recorded usage markers, and print `unavailable`
instead of inventing a price for unknown models. The report should also state
that it does not query billing APIs, inspect account paid-usage/budget state, or
perform a live inference probe; live E2E still needs model calls before and
after the deterministic cost audit.

2026-05-31 model-usage follow-up: OpenClaw's token-use reference treats
per-response usage, cached-token counters, and cost estimates as separate
operational surfaces, while Hermes' context engine tracks API-reported token
counts when deciding whether to compress. GitHub Models supports Actions calls
through `models: read` and the workflow `GITHUB_TOKEN`, but its free/API usage
is rate-limited and production billing details live outside an individual
inference response. GitClaw should therefore normalize provider response usage
into assistant-marker attributes when present, add `@gitclaw /models usage`
and `gitclaw models usage` as the deterministic readback, keep raw provider
payloads and prompt bodies out of the report, and route dollar estimation to the
separate reviewed-catalog `/models cost` surface.

2026-05-30 session-risk follow-up: OpenClaw-style assistants treat transcripts
and assistant-turn metadata as operational state, while Hermes emphasizes saved
and searchable sessions. GitClaw should add `@gitclaw /session risk` and
`gitclaw session risk --backup <issue.json>` as a body-free audit of the
GitHub issue session itself: verify transcript reconstruction, trusted versus
untrusted messages, edited messages, assistant-turn prompt provenance, error
markers, heartbeat/channel/proactive origins, and reused prompt-context hashes.
The report should expose only counts, finding codes, sources, and hashes, then
prove the surface with a live E2E that first performs a real GitHub Models
conversation and then audits that session metadata deterministically.

2026-05-30 config-risk follow-up: OpenClaw's config/status surface and Hermes'
profile config model both make configuration a high-authority operational
boundary rather than ordinary prompt text. GitClaw should add `@gitclaw
/config risk` and `gitclaw config risk` as the body-free risk audit for that
boundary: inspect `.gitclaw/config.yml`, workflow presence, trigger labels,
trusted actor scope, model/fallback/budget settings, skill/tool gates, and
workflow risk patterns without printing config or workflow bodies. The report
should flag broad trusted associations, label collisions, missing workflows,
unsafe budgets, missing fallback coverage, credential material, raw prompt
logging, webhook/socket/daemon drift, write-mode config, risky workflow
permissions, `pull_request_target`, raw secret echoing, and unbounded
background loops, then require a live GitHub Models/tool-use E2E.

2026-05-30 policy-risk follow-up: OpenClaw's exec approval model separates
policy and approval review from elevated execution, and Hermes' security
posture keeps dangerous operations behind explicit authorization. GitClaw's
GitHub-native version should make that boundary inspectable in the repo itself:
add `@gitclaw /policy risk` and `gitclaw policy risk` to audit trusted actor
breadth, managed-label collisions, workflow permissions, backup concurrency,
active policy-output hashes, and the hard no-write/no-host-exec runtime gate
without printing workflow, issue, prompt, policy-output, or credential bodies.
The E2E for this surface must include both the deterministic report and a real
GitHub Models follow-up with repo-reader search/tool evidence.

2026-05-30 tool-grounding follow-up: the first model-backed conversation after
the parameter fix proved provider access but exposed prompt ambiguity: the model
echoed the issue nonce where the harness wanted the repository search-result
token. GitClaw should keep tool-output token requests explicit, document that
`gitclaw.search_files` is authoritative for search-result tokens, and use
distinct token prefixes plus redacted prompt artifacts in live E2E checks.
The underlying search tool also needs per-query match limits because broad
queries like `go.mod` can otherwise consume the total match budget before the
explicit fixture phrase is searched.
2026-05-31 search-tool chat hardening: a single search-token answer proves tool
visibility but not conversational continuity. The live search-tool harness
should use two distinct search needles across the issue body and a later
comment, matching the GitClaw design goal that GitHub issues are durable
threads where grounded tool context remains available turn after turn.
2026-05-31 issue-chat hardening: the baseline conversation harness should meet
that same bar. The second issue-comment turn should prove transcript continuity
with earlier tokens and also force a new repo-reader/search result from the
follow-up text, with prompt-context provenance and usage telemetry in the
assistant marker. Since continuous GitHub issue threads may keep earlier
file-reference tools prompt-visible, the harness should require search evidence
for the fresh follow-up fixture without failing solely because `gitclaw.read_file`
also appears from prior context. The follow-up prompt should use fixed labels
and a token-prefix guard because small hosted models may otherwise describe the
search phrase even when the prompt includes the correct search-result token.
This keeps ordinary GitHub-native chat from becoming a thin token echo test
while report-specific harnesses grow more rigorous.

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

2026-05-31 profile-manifest follow-up: Hermes profile docs now make the
portability split explicit: shared profile distributions can package SOUL,
config, skills, cron jobs, and MCP connections, while credentials, memories,
and sessions stay local to the machine. OpenClaw workspace docs make the same
lesson file-shaped by recommending a git-backed workspace of soul, user,
identity, tools, heartbeat, memory, and skills. GitClaw should expose this as a
dry-run profile manifest rather than a packager: list repo-reviewed profile
files, skills, bundles, proactive prompts, toolsets, MCP specs, and tool
contracts by path/hash/counts only, then explicitly exclude credentials,
external homes, sessions, backup payloads, and mutation/install/switch
operations. The harness must include a real GitHub Models repo-reader/search
follow-up so the deterministic manifest does not replace LLM/tool coverage.

2026-05-31 profile-catalog follow-up: before offering any profile
manifest/export-plan surface, GitClaw should give operators a compact profile
command and layer map. Hermes' profile boundary groups config, memory,
sessions, skills, cron jobs, plugins, and state; OpenClaw's workspace boundary
keeps the same idea file-shaped in the repository. GitClaw's catalog should
therefore name the commands and layers for identity, user, soul, memory,
skills, bundles, tools, models, proactive workflows, hooks, channels, backups,
and sessions, but report only counts and gates. It must stay body-free and
prove the deterministic surface with a real GitHub Models repo-reader/search
follow-up.

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

2026-05-31 commands-report E2E hardening: current OpenClaw CLI docs continue
to treat `--help`, channel helpers, doctor/status, and gateway operations as
the operator's navigational surface, while Hermes' CLI reference centralizes
terminal commands, worktree flags, skills, gateway, and doctor commands. GitClaw
should keep `/help` equally useful but safer in GitHub issues: deterministic,
body-free, no model call, no raw issue text, and explicitly paired with a
normal GitHub Models follow-up that selects `repo-reader`, exposes bounded
repository search, and recovers the commands-report fixture token. That keeps
the catalog from becoming a stale checklist that no longer proves the live
LLM/tool path.

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

2026-05-31 bundle-search follow-up: Once bundles become the unit of
task-profile orchestration, operators need the same compact discovery affordance
that OpenClaw and Hermes expose for skills. GitClaw should keep this search
body-free and local: `@gitclaw /bundles search <query>` and
`gitclaw bundles search <query>` should search only bundle names, paths,
descriptions, skill refs, resolved/missing refs, and instruction hashes. Raw
queries should appear only as a hash and term count, and the live acceptance
must pair the deterministic report with a real GitHub Models repo-reader search
turn.

2026-05-30 bundle-risk follow-up: Hermes loads every skill in an invoked bundle
plus the bundle's optional instruction text into the same user-message turn,
while OpenClaw's current skill docs explicitly treat third-party skill content
as untrusted code and require human review/sandboxing before enabling risky
skills. GitClaw should therefore audit bundle YAML and instructions as
prompt-visible control data: `@gitclaw /bundles risk` and
`gitclaw bundles risk` should flag missing refs, duplicate names, malformed
YAML, prompt-boundary overrides, hidden persistence, remote installs, external
delivery, shell-exec language, and credential transfer language, while
reporting only counts, codes, paths, bundle hashes, and line hashes.

2026-05-31 bundle-catalog follow-up: current Hermes material emphasizes skill
bundles as reusable workflow packs over existing skills, while OpenClaw's
skill docs still separate discovery/configuration from raw skill body loading
and tool execution. GitClaw should add `@gitclaw /bundles catalog` and
`gitclaw bundles catalog` as the compact bundle orchestration catalog:
procedural-memory task-profile metadata, selected/load state, instruction
load mode and hashes, skill-ref resolution, risk rollups, reason codes, and
disabled registry/install/agent-authored mutation gates, with a live LLM/tool
E2E proof and no raw bundle YAML, instructions, skill bodies, issue/comment
bodies, prompts, credentials, provider payloads, or secret values.

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

2026-05-31 context-reference chat hardening: explicit `@file:` line ranges
must remain the prompt boundary and should not be widened by an automatic
whole-file `gitclaw.read_file` output. Live E2E should prove the bounded
reference answer first, then continue the same issue with a normal
repo-reader/search turn so context-reference support is tested as conversation,
not as a one-shot fixture echo.

2026-05-30 git-context follow-up: Hermes' reference surface also makes current
git state first-class through `@diff`, `@staged`, and `@git:N`; OpenClaw's
diffs plugin frames patches as read-only artifacts that agents can inspect or
render without mutating the workspace. GitClaw can safely adopt the Git-native
subset now because GitHub Actions already checks out a repo: `@diff` and
`@staged` should run bounded read-only `git diff` commands, and `@git:N`
should expose a bounded recent commit log with patches, clamped to 10 commits.
The issue-visible `/context` report should remain body-free and show only kind,
count, status, sizes, and hashes.
2026-05-31 git-reference chat hardening: the live `@git:1` chat proof should
not stop at copying a commit hash. It should continue in the same issue with a
normal repo-reader/search turn, proving that Git reference context composes
with ordinary conversational model/tool usage instead of being a one-shot
transcript fixture.

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

2026-06-01 memory-provenance follow-up: OpenClaw's memory guidance treats
`MEMORY.md` as durable prompt context and `memory/YYYY-MM-DD.md` as the working
layer, with action-sensitive notes needing source/authority boundaries. Hermes
keeps bounded built-in memory files separate from session search and optional
external providers. GitClaw should therefore add `@gitclaw /memory provenance`
and `gitclaw memory provenance` as a body-free git-history view: report
repo-local memory paths, loaded/prompt-visible state, validation/risk rollups,
tracked/dirty state, last commit IDs/dates, and commit-subject hashes, while
excluding raw memory bodies, git subjects, author identities, issue/comment
text, prompts, provider payloads, and credentials.

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

2026-05-31 doctor-e2e follow-up: OpenClaw-style doctor output is most useful
when it can prove the test harness itself has not drifted. Add an E2E harness
inventory to `/doctor` and `gitclaw doctor list`: script counts, live issue
coverage, cleanup coverage, model-backed coverage, session coverage, backup
gates, workflow-dispatch coverage, and path/hash evidence. The doctor command
must remain body-free and no-model; the live doctor harness should pair that
deterministic report with a normal GitHub Models repo-reader follow-up so the
feature batch still proves inference and prompt-visible tool grounding.

2026-05-31 doctor-model-followup follow-up: OpenClaw transcript/session CLIs
and Hermes' saved-session database both make actual conversation history
auditable, not just test intent. GitClaw's doctor inventory should make the
same distinction for E2E harnesses: marker-only model coverage is useful, but
real model follow-up coverage must require a posted issue comment, an
`issue_comment` Actions run, a second assistant turn, prompt provenance, and
prompt-visible tool evidence. This prevents docs text such as "GitHub Models"
from being counted as proof of a live LLM call.

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

2026-05-31 channel-list E2E hardening: OpenClaw-style channel routing treats
channel inventory as part of the routing contract, while Hermes-style gateways
make the messaging runtime visible before delivery code depends on it. The
explicit `/channels list` alias should therefore prove the same two facts as
the root channel report: a deterministic GitHub workflow-dispatch inventory and
a real GitHub Models repo-reader/search follow-up that recovers the bounded
channels-list fixture token without echoing issue/comment sentinels.

2026-05-30 channel-verify follow-up: channel support needs a positive health
check, not only inventory. Add `/channels verify` and `gitclaw channels verify`
as a body-free bridge verifier for the GitHub-native equivalent of a gateway
connection check: channel-ingest workflow present, `workflow_dispatch` enabled,
`actions: write` and `issues: write` permissions, normalized channel inputs,
and Telegram/Slack/generic provider keys visible before real pollers depend on
them.

2026-05-31 channel-verify E2E hardening: OpenClaw's channel routing docs make
deterministic routing and channel/account bindings explicit, while Hermes'
gateway docs make messaging platforms first-class runtimes. GitClaw's
workflow-dispatch bridge health check should therefore prove both halves:
deterministic bridge readiness over GitHub workflows and a normal GitHub
Models follow-up that selects repo-reader, exposes `gitclaw.search_files`,
recovers the channels-verify repository-search fixture token, and avoids
hidden channel/command token echoing.

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

2026-05-31 agents-catalog follow-up: OpenClaw's current skill and workspace
docs keep agent behavior in reviewed files, while Hermes profiles, toolsets,
and delegation docs separate profile state, tool runtime, and subagent/worker
execution. GitClaw should keep that separation visible with
`@gitclaw /agents catalog` and `gitclaw agents catalog`: list agent commands,
policy/spec stores, the GitHub Actions runtime, GitHub issue/comment
conversation boundary, reviewed tool-name intent, approval frontmatter, and
explicit disabled delegation/profile/gateway gates without printing agent
bodies, issue/comment bodies, prompts, tool outputs, credentials, channel
payloads, or sessions. Acceptance requires a real issue E2E, a local CLI
assertion, and a GitHub Models repo-reader/search follow-up that recovers a
distinct agents-catalog repository-search fixture token.

2026-06-01 agents-provenance follow-up: OpenClaw's multi-agent and workspace
model treats agents as isolated identities with their own workspaces and
sessions, while Hermes profiles isolate state/config and delegation is an
explicit runtime action rather than a passive file read. GitClaw v1 should
borrow the audit shape, not the runtime: `@gitclaw /agents provenance` and
`gitclaw agents provenance` should map `.gitclaw/AGENTS.md` and
`.gitclaw/agents/*.md` to repo-local git history, tracked/dirty state, commit
availability, validation counts, risk metadata, and subject hashes only. The
report must keep delegation, subagents, gateways, shared profile state,
agent-to-agent messaging, and repository mutation disabled, and it must not
print agent bodies, issue/comment bodies, prompts, tool outputs, git subjects,
author identities, channel payloads, or credentials. Acceptance requires
deterministic tests, local CLI assertions, a real issue E2E, and a GitHub
Models repo-reader/search follow-up that recovers a distinct
agents-provenance repository-search fixture token with usage telemetry.

2026-05-31 nodes-catalog follow-up: OpenClaw's current node docs frame nodes as
paired peripherals connected to the Gateway WebSocket with `role: "node"`,
declared command/capability surfaces, gateway policy gates, and local exec
approvals; Hermes' current delegation docs frame child workers around bounded
toolsets, kill switches, interrupt behavior, and non-durable child execution.
GitClaw should expose the analogous surface without implementing it:
`@gitclaw /nodes catalog` and `gitclaw nodes catalog` should list node commands,
`.gitclaw/NODES.md`, `.gitclaw/nodes/*.md`, the GitHub Actions ephemeral-job
runtime, GitHub-native wake paths, issue/comment conversation boundary,
reviewed capability names, approval frontmatter, and explicit no-gateway,
no-pairing, no-RPC, no-browser-proxy, no-media-device, and no-remote-exec gates.
The report must not print node bodies, issue/comment bodies, prompts, tool
outputs, credentials, channel payloads, worker payloads, or sessions. Acceptance
requires a real issue E2E, a local CLI assertion, and a GitHub Models
repo-reader/search follow-up that recovers a distinct nodes-catalog repository
search fixture token. Sources: https://docs.openclaw.ai/nodes,
https://docs.openclaw.ai/cli/gateway, and
https://hermes-agent.nousresearch.com/docs/user-guide/features/delegation.

2026-05-31 artifacts-catalog follow-up: OpenClaw's current assistant setup,
workspace, and FAQ docs treat media/file sends as explicit outbound evidence
with safe document/media type limits, workspace-only path restrictions, and
private git-backed workspace memory. Hermes' session and checkpoint docs split
durable history/checkpoints from exported files: sessions can export full JSONL
transcripts, while checkpoints live in a shadow git store and are pruned by
retention/size gates. GitClaw should expose the same separation with
`@gitclaw /artifacts catalog` and `gitclaw artifacts catalog`: list artifact
commands, `.gitclaw/ARTIFACTS.md`, `.gitclaw/artifacts/*.md`, reviewed
`actions/upload-artifact` workflow steps, GitHub Actions artifact storage,
redaction, explicit short retention, durable backup branch boundaries, and
no-hidden-state/no-external-storage/no-raw-payload gates without printing
artifact payloads, prompt bodies, issue/comment bodies, tool outputs,
credentials, channel payloads, backup payloads, or sessions. Acceptance requires
a real issue E2E, a local CLI assertion, and a GitHub Models repo-reader/search
follow-up that recovers a distinct artifacts-catalog repository-search fixture
token. Sources: https://docs.openclaw.ai/start/openclaw,
https://docs.openclaw.ai/agent-workspace,
https://docs.openclaw.ai/help/faq,
https://hermes-agent.nousresearch.com/docs/user-guide/sessions, and
https://hermes-agent.nousresearch.com/docs/user-guide/checkpoints-and-rollback.

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

2026-05-31 tools-report E2E hardening: OpenClaw's current capabilities docs
still separate tools as typed actions, skills as prompt instructions, and
plugins as runtime capability packages, while Hermes' current tools docs keep
core tools/toolsets explicit and reserve Tool Search-style deferral for large
or non-core catalogs. GitClaw should keep `/tools` and `/tools list`
body-free and GitHub-native: report deterministic contracts, gate state,
validation counts, and active-output hashes, then require a normal GitHub
Models follow-up that selects `repo-reader`, exposes `gitclaw.search_files`,
and recovers a fixture token through actual prompt-visible tool output.

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

2026-05-31 tools-verify E2E hardening: OpenClaw's current capabilities docs
stress that only tools surviving active policy are sent to the model, and
Hermes' current tool-search docs keep deferred catalogs scoped to the
session's enabled/disabled toolsets rather than the whole process registry.
GitClaw's `/tools verify` trust envelope should therefore prove two things
after any change: the deterministic report remains body-free and hash-only,
and a normal GitHub Models follow-up still selects `repo-reader`, exposes
`gitclaw.search_files`, and recovers a fixture token through actual
prompt-visible tool output.

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

2026-05-31 tool-exposure follow-up: OpenClaw's tools docs now make allow/deny
and tool-profile visibility a first-class operator concern, while Hermes' Tool
Search docs frame model-visible tool schemas as a progressive-disclosure and
prompt-cache tradeoff. GitClaw should expose the narrower GitHub-native truth
with `@gitclaw /tools exposure`, `@gitclaw /tools exposure risk`, and
`gitclaw tools exposure ...`: static built-in contracts only, no
model-callable structured tool schemas, no Hermes-style deferred bridge in v1,
bounded pre-model tool outputs, explicit allowlist/denylist gate counts, and a
fail-closed finding when an explicit allowlist leaves zero enabled tool
contracts. Reports should publish only names, modes, counts, hashes, gate
state, and finding codes, never raw tool schemas, inputs, outputs, prompts,
issue/comment bodies, credentials, or secrets.

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

2026-05-31 sandbox-risk follow-up: The same OpenClaw/Hermes boundary split
also needs an operator-facing risk audit, not only a descriptive sandbox
report. GitClaw should add `@gitclaw /sandbox risk` and
`gitclaw sandbox risk` as body-free runtime, tool, workflow, skill, and backup
concurrency cards with stable finding codes. The audit should make raw
issue/comment/prompt/workflow/tool bodies and secrets explicitly absent, and
any future host shell, repository mutation, elevated mode, mutating tool, or
workflow-permission drift must become a high-severity finding. The live E2E
harness should still follow the deterministic report with a normal GitHub
Models conversation that proves prompt provenance, selected skills, and
prompt-visible read-only tool usage.

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

2026-05-31 tools-approval-plan follow-up: OpenClaw's exec approvals separate
tool policy, allowlists, and human approval before command execution, while
Hermes keeps tool availability and dangerous command authorization explicit in
the platform/tool boundary. GitClaw now has enough tool inventory, run-plan, and
approval-label metadata to expose the same decision without adding execution:
`@gitclaw /tools approval-plan <name>` and
`gitclaw tools approval-plan <name>` should report the normalized tool, config
and allowlist gates, contract mode, mutation flag, active-output hashes,
per-issue approval labels, and current decision. In v1, known read-only or
metadata-only tools should say no approval is required; any future mutating
tool must be blocked behind a future write mode, `gitclaw:write-requested`,
`gitclaw:approved`, and live model/tool E2E. The report must never approve,
execute, mutate, call a model, or print raw tool inputs, outputs, approval
payloads, issue/comment bodies, prompts, credentials, or secrets.

2026-05-31 toolsets follow-up: Hermes' toolsets and OpenClaw's tool-policy
surfaces both point to named, task-oriented capability profiles, but GitClaw
should keep v1 stricter than either runtime. Add repo-reviewed
`.gitclaw/toolsets/*.yaml` profiles and expose them through
`@gitclaw /tools toolsets`, `@gitclaw /tools toolsets risk`, and
`gitclaw tools toolsets ...`. These files declare expected deterministic tool
contracts for a task profile and can carry reviewed guidance, but they do not
activate tools, install plugins, expose MCP servers, call provider APIs,
execute shell commands, or change repository permissions. Reports should
publish only profile names, paths, normalized tool refs, resolved/unknown refs,
config-gate state, hashes, risk codes, and line hashes; raw toolset bodies and
instructions stay out of issue comments.

2026-05-31 toolset-provenance follow-up: OpenClaw and Hermes both make tool
selection inspectable, but GitClaw also needs repo-native review history for
those reviewed toolset profiles. Add `@gitclaw /tools toolsets provenance` and
`gitclaw tools toolsets provenance` as a body-free git-history audit over
`.gitclaw/toolsets/*.yaml`: tracked/dirty state, commit IDs/dates, subject
hashes, tool refs, gate counts, and risk codes only. It must not activate
toolsets, run tools, print reviewed instructions, leak git subjects or author
identities, or skip the live GitHub Models follow-up E2E.

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
2026-05-31 migration-plan E2E hardening: the paired model proof should run in
the same issue as the deterministic report, not as a separate generic chat.
That keeps OpenClaw's preview-first migration idea tied to GitClaw's canonical
GitHub-thread conversation model: first prove the import map is body-free and
non-mutating, then prove the same thread can continue with repo-reader and
bounded repository search.

2026-05-31 migration-risk follow-up: OpenClaw's current migrate CLI documents
provider-owned migrations with `--dry-run`, redacted itemized plans,
backup-before-apply, conflict refusal, archive/manual-review state, and opt-in
credential import flags such as `--include-secrets`/`--no-auth-credentials`.
Its Hermes provider imports config/providers, MCP server definitions, `SOUL.md`,
`AGENTS.md`, memory files, skills, skill config, supported auth credentials, and
API keys only when credential migration is accepted; plugins, sessions, logs,
cron, MCP token material, and `state.db` are archive-only/manual-review state.
Hermes' checkpoint docs separately reinforce the preview-before-restore pattern:
`/rollback diff <N>` previews changes, and restore takes a pre-rollback
snapshot. GitClaw's GitHub-native migration risk audit should therefore
classify import-map rows before any migration is implemented: credentials are
skipped, MCP/plugins/hooks/cron are executable-state quarantine, skills and
memory require human review, raw sessions stay archive-only, and the report
must prove no source home reads, no installer execution, no MCP autoload, no
credential import, no mutation, and no raw body/secret leakage.

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

2026-05-30 context-risk follow-up: OpenClaw's `/context` docs emphasize
diagnostic visibility into prompt contributors without dumping the full prompt,
while Hermes context references explicitly bound `@file`, `@folder`, `@diff`,
`@staged`, and `@git:N`, block sensitive paths, reject traversal, and security
scan context files before injection. GitClaw should add `@gitclaw /context
risk` and `gitclaw context risk` as the body-free risk audit for prompt-visible
context: scan loaded context files, explicit references, selected skills, and
tool outputs for prompt-boundary, credential-exfiltration, hidden-instruction,
host-exec, and unbounded-context patterns, then report only counts, paths,
hashes, risk codes, and runtime gates. Pair the deterministic audit with a live
GitHub Models follow-up that proves repo-reader/tool context still works.

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

2026-05-31 skills-select-plan E2E hardening: OpenClaw's current skills docs
frame skills as `SKILL.md` instruction packs loaded from bounded roots with
allowlists and safety gates, while Hermes' current skills docs emphasize
progressive disclosure through compact skill lists and on-demand `skill_view`
loads. GitClaw's skill selection plan should therefore prove both halves after
changes: the deterministic report remains metadata/hash-only and body-free,
and a normal GitHub Models follow-up still selects `repo-reader`, exposes
`gitclaw.search_files`, and recovers a fixture token through actual
prompt-visible tool output. The live failure mode to guard against is fixture
ambiguity: do not mention the fixture file path in the model prompt, use a
high-entropy search needle, and prioritize the newest user turn when extracting
bounded search queries.

2026-05-31 skill-refresh-plan follow-up: OpenClaw's skill surface is built for
a resident gateway that can maintain skill snapshots and refresh through local
runtime control, while Hermes documents explicit skill lifecycle operations
such as update/install/delete in a profile home. GitClaw should not emulate that
with a hidden watcher or self-updating skill registry. Add `@gitclaw /skills
refresh-plan` and `gitclaw skills refresh-plan` as the inspect-only Actions
analogue: each issue/comment/workflow-dispatch turn rebuilds the repo-local
skill index from the current checkout, reports hashes and validation state, and
keeps install/update/mutation/model calls/raw bodies disabled. Pair any refresh
behavior change with a live GitHub Models E2E that proves selected skills and
tool usage still work.

2026-05-31 skill-proposal-plan follow-up: OpenClaw's current skills CLI adds a
Skills Workshop proposal lifecycle where drafts are durable but not active
until applied, and Hermes' skills posture emphasizes reusable procedural
memory while GitClaw explicitly avoids autonomous self-improvement. GitClaw's
matching feature should be a review-first `@gitclaw /skills proposal-plan
<name>` and `gitclaw skills proposal-plan <name>`: hash the request, derive a
safe proposal path, distinguish proposed create versus update, report existing
skill matches and validation rollups, but never fetch sources, run installers,
write proposal files, update active skills, auto-apply, or self-improve. The
E2E harness must pair the deterministic body-free plan with a real GitHub
Models repo-reader/search follow-up.

2026-05-31 skill-proposals-store follow-up: OpenClaw's Skill Workshop exposes
status, pending, quarantine, inspect, apply, and reject actions over gateway
state, while Hermes keeps skills mutable in a profile home and supports hub
audit/quarantine state. GitClaw should keep the same operator visibility but
move the state into git review: `.gitclaw/skill-proposals/*/PROPOSAL.md`.
`@gitclaw /skills proposals risk` and `gitclaw skills proposals [risk]`
should inventory proposal files by lifecycle status, scan proposal bodies for
risk internally, and publish only metadata, counts, finding codes, and line
hashes. It must not activate, apply, reject, quarantine, fetch, write support
files, update `.gitclaw/SKILLS`, or dump proposal bodies. The harness should
pair the deterministic report with a live GitHub Models repo-reader/search
follow-up.

2026-05-30 tool-gating follow-up: Hermes/OpenClaw toolsets are useful partly
because the operator can see and constrain what the agent may call. GitClaw
should mirror that with repo-reviewed `tools.allowed` and `tools.disabled`
config for deterministic built-ins: disabled tools remain visible in reports,
but prompt-visible outputs are not generated.

2026-05-31 tool-provenance follow-up: OpenClaw's workspace/tool model and
Hermes' session/toolset previews both make tool activity inspectable without
requiring a raw prompt dump. GitClaw should expose `@gitclaw /tools
provenance` and `gitclaw tools provenance [query]` as the issue-native
current-turn map: active deterministic tool names, contract modes, enabled
gate state, prompt-visible names, input/output hashes, size counts, and risk
codes only. The report must not print raw tool inputs, raw tool outputs,
search result bodies, file bodies, issue/comment bodies, prompts, credentials,
or secrets, and every change should be paired with a live GitHub Models
follow-up that proves the hashed tool outputs still feed a real model call.

2026-05-31 tool-boundary follow-up: Hermes' prompt/tool boundary guidance and
OpenClaw's tool policy posture point to a narrower audit than provenance:
whether prompt-visible tool output can masquerade as instructions. GitClaw
should expose `@gitclaw /tools boundary` and `gitclaw tools boundary [query]`
as a body-free prompt-boundary report: deterministic `[tool_output ...]`
delimiter strategy, active tool names, known/unknown output counts, read-only
versus metadata-only modes, prompt-injection finding counts, hash-only
input/output gates, disabled model-callable structured tools, disabled shell/
network/mutation gates, and per-output risk codes plus line hashes. The report
must never print raw tool inputs, outputs, search queries, issue/comment bodies,
prompts, credentials, or secrets, and every change should include both the
deterministic issue-command E2E and a real GitHub Models follow-up using
repo-reader/search.

2026-05-31 task-ledger follow-up: OpenClaw's background-task surface and
Hermes' Kanban/session posture both make durable work items inspectable, but
GitClaw should keep the no-server shape: the GitHub issue is the task row,
labels are the current state, and comments plus `gitclaw:assistant-turn`
markers are the handoff log. Add `@gitclaw /tasks ledger` and `gitclaw tasks
ledger --backup <issue.json>` as a body-free current-thread/backup ledger:
current label-derived status, comment/transcript counts, assistant marker
counts, deterministic versus model-backed turn counts, prompt-provenance
counts, channel/proactive marker presence, hashes, and explicit raw-body gates.
Do not claim historical label transitions until GitHub issue events are part of
the runtime input; report `status_history_available=false` instead.

2026-05-30 prompt-list follow-up: prompt-budget visibility should also be
available before opening an issue. Add `gitclaw prompt list` as the local
mirror of `/prompt`: provider/model, prompt size/hash, configured budgets,
transcript counts, context files, selected always-on skills, and deterministic
tool-output input/size/hash metadata, with no prompt, file, skill, issue, or
tool-output bodies.

2026-05-30 prompt-risk follow-up: OpenClaw's token-use docs make the current
context window auditable by separating prompt snapshots from broader provider
usage, and Hermes' prompt guidance keeps secrets, long runbooks, and temporary
task context out of the durable system prompt. GitClaw should make that
boundary testable with `@gitclaw /prompt risk` and `gitclaw prompt risk`:
scan prompt-visible transcript text, context files, selected skills, and tool
outputs for prompt-boundary override, credential exfiltration, hidden
instruction, host-exec, and unbounded-context patterns, but emit only counts,
paths/names, hashes, risk codes, severities, prompt budget metadata, prompt
artifact gates, and no-write runtime gates. Pair the deterministic audit with a
live GitHub Models follow-up that proves repo-reader/tool context still works.

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

2026-05-31 channel-ingest E2E hardening: the generic bridge is the serverless
stand-in for an OpenClaw/Hermes gateway. Its live harness should therefore
prove the whole control path: workflow-dispatch mirroring into a canonical
GitHub issue, duplicate provider-message suppression, and then a normal GitHub
Models repo-reader/search follow-up on that canonical issue without leaking
hidden channel payload tokens.

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
Model-backed channel-message E2E should not stop at transcript reconstruction:
it should force repo-reader search from the mirrored message and assert model,
prompt, tool, and usage markers while keeping hidden channel sentinels out of
the response.

2026-05-31 channel-message follow-up hardening: a GitHub-native channel bridge
is only useful if the user can continue the conversation after the mirrored
Telegram/Slack turn. The live channel-message harness should therefore post a
normal GitHub issue comment after the workflow-dispatch channel turn and prove
another GitHub Models repo-reader/search response with a distinct repository
fixture token, so the bridge validates both gateway ingress and ordinary
thread continuation.

2026-05-31 channels-report E2E hardening: OpenClaw's current channel docs make
Slack/Telegram gateway routing, channel policies, and provider delivery a
first-class operator surface, while Hermes' Slack docs emphasize thread/session
isolation, mention gating, slash commands, and per-channel skill bindings.
GitClaw's serverless channel report should therefore be more than static
workflow metadata: every change needs a deterministic, body-free bridge report
plus a normal GitHub Models follow-up that selects repo-reader, exposes
`gitclaw.search_files`, recovers the channels-report repository-search fixture
token, and avoids hidden channel/command token echoing.

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

2026-05-31 channel-state workflow E2E hardening: OpenClaw's disk-backed channel
routing state and Hermes' long-running messaging gateway both imply durable
provider cursor state. GitClaw's GitHub-native substitute should prove that
state through live Actions: hash-only account/offset records, duplicate offset
suppression, and normal GitHub Models repo-reader/search issue-comment turns on
the state issue without exposing raw provider cursors.

2026-05-30 channel gateway follow-up: OpenClaw's gateway and Slack Socket Mode
loop can be approximated in GitHub Actions as a renewable lease rather than a
server. A `gitclaw channel-gateway` command should first record one
hash-only lease in the channel-state issue; the workflow wrapper can optionally
`workflow_dispatch` its successor with `actions: write`. Provider sockets and
pollers can be added behind that lease once the renewal surface is proven live.

2026-05-31 channel-gateway workflow E2E hardening: Hermes exposes a long-running
messaging gateway process, while OpenClaw treats routing state as part of the
gateway control plane. GitClaw's no-server equivalent is an Actions-renewable
lease, so the live gateway workflow harness should prove hash-only lease state,
duplicate lease suppression, and ordinary GitHub issue-comment continuation on
the lease state issue through real GitHub Models repo-reader/search turns.

2026-05-30 channel delivery follow-up: OpenClaw's gateway owns both inbound
delivery and outbound replies. GitClaw should record outbound reply delivery as
a GitHub-native receipt: verify the source `gitclaw:assistant-turn`, write one
`gitclaw:channel-delivery` marker to the channel-state issue, hash the provider
message id, and dedupe by source issue/comment. This lets Telegram/Slack
gateways retry safely without turning channel state into a plaintext transcript.

2026-05-31 channel-delivery workflow E2E hardening: OpenClaw's message routing
docs describe outbound replies as part of the gateway delivery path, with
duplicate delivery suppression, and Hermes' Slack gateway routes replies through
the workspace-specific client. GitClaw's no-server equivalent should prove the
receipt path live: source assistant verification, hash-only provider receipt
state, duplicate receipt suppression, and normal GitHub Models repo-reader/search
continuation on the receipt state issue without leaking source reply bodies.

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

2026-05-31 workspace-catalog follow-up: OpenClaw's workspace docs make the
workspace file map the agent's durable operating surface, while Hermes keeps
profile state, working directory, and sandbox boundaries separate. GitClaw
should make that same separation visible with `@gitclaw /workspace catalog`
and `gitclaw workspace catalog`: list workspace commands, policy/spec stores,
git/workflow/context/repository-inventory layers, runtime and durable-state
layers, private-memory/external-mount/daemon/socket suppression, and raw-body
gates without printing workspace file bodies, workflow bodies, issue/comment
bodies, prompts, tool outputs, or credentials. Acceptance requires a real issue
E2E, a local CLI assertion, and a GitHub Models repo-reader/search follow-up
that recovers a distinct workspace-catalog repository-search fixture token.

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

2026-05-31 secrets-risk follow-up: OpenClaw's secrets docs separate
`audit --check`, configure/apply, and runtime reload, and explicitly avoid
side-effecting exec SecretRef checks by default; Hermes' secrets docs similarly
load external secrets into the process environment rather than treating secret
stores as prompt-visible memory. GitClaw should add the risk-oriented half now:
`@gitclaw /secrets risk` and `gitclaw secrets risk` reuse the bounded repo scan
but frame results as plaintext-residue, secret-reference, runtime-boundary, and
apply-boundary risk cards. The report must remain body-free, no-model, and
non-mutating, with a live GitHub Models follow-up E2E proving the normal model
path still works.

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

2026-05-31 checkpoint-report E2E hardening: Hermes' current checkpoint docs
describe an opt-in shadow git store, checkpoint creation before file writes or
destructive terminal commands, `/rollback diff` preview, and restore as a
separate explicit action. OpenClaw memory/backup docs keep durable state as
plain workspace files and operator-managed backups. GitClaw's checkpoint report
should therefore prove two things after every change: the deterministic issue
report remains a body-free, inspect-only map of HEAD/worktree/backup-branch
state, and a normal GitHub Models follow-up can still select `repo-reader`,
use bounded repository search, and recover the checkpoints-report fixture token
without echoing issue-body sentinels. That keeps rollback evidence visible
without turning issue chat into an implicit restore channel.

2026-05-30 checkpoint-risk follow-up: Hermes' rollback model treats restore as
dangerous enough to require preview and checkpoint evidence, while OpenClaw's
write approval posture keeps mutation separate from inspection. GitClaw should
add `@gitclaw /checkpoints risk` and `gitclaw checkpoints risk`: scan git
checkpoint metadata for missing auditability, dirty worktrees, raw diff or file
body exposure, restore/reset/clean/checkout authority, shadow-store path
leakage, and missing rollback safety gates while reporting only metadata,
counts, commit hashes, risk codes, and severities. Acceptance requires
deterministic body-free coverage plus a live GitHub Models follow-up E2E.

2026-06-01 checkpoint-catalog follow-up: Hermes' current checkpoint docs make
rollback inspectability a first-class command surface (`/rollback`,
`/rollback diff`, `hermes checkpoints status/list/prune/clear`) and emphasize
the shadow git store, per-project refs, preview-before-restore, worktree
isolation, and no-op behavior when checkpoints are disabled. OpenClaw's backup
docs similarly require explicit manifests and verification before restore-like
recovery. GitClaw should add a compact body-free catalog before any restore
mode exists: `@gitclaw /checkpoints catalog`, `@gitclaw /rollback catalog`,
`gitclaw checkpoints catalog`, and `gitclaw rollback catalog` should enumerate
checkpoint and rollback commands, git-history metadata, worktree counts,
backup-branch evidence, recent-commit hash metadata, future restore-preview
requirements, and disabled reset/clean/checkout gates. It must never print
diffs, file bodies, commit subjects, issue/comment/prompt/tool bodies, shadow
store paths, credentials, or secrets. Acceptance requires a live deterministic
catalog issue plus a real GitHub Models follow-up using repo-reader search.

2026-05-30 approval-readiness follow-up: OpenClaw's exec approvals treat command
execution as a policy decision layered with user approval, while Hermes frames
dangerous commands as an explicit authorization boundary. GitClaw should expose
that boundary before it grows write mode: `@gitclaw /approvals` reports trusted
actor state, write-request detection, per-issue approval labels, and the
read-only write-mode block, but never approves, mutates, executes, or prints raw
issue/comment/prompt text. Local `gitclaw approvals list|verify` should mirror
the static approval shape without issue-only state.

2026-05-30 approval-risk follow-up: OpenClaw's current exec approval docs make
approval a stacked policy, allowlist, and user-decision boundary with timeout
denial, while its approvals CLI exposes host/node approval state as an
operator-inspectable surface. GitClaw should keep the same inspectability
without copying host-exec complexity yet: `@gitclaw /approvals risk` and local
`gitclaw approvals risk` should audit trusted association breadth,
approval-label collisions, managed-label collisions, per-issue approval store
metadata, and hard write/host-exec denial. Acceptance requires a deterministic
body-free report plus a live GitHub Models follow-up that proves the normal
LLM/tool path still works after the risk audit.

2026-05-31 approval-provenance follow-up: OpenClaw's exec approval docs
separate requested policy, host-local approval state, allowlists, UI/manual
approval, and execution result; Hermes similarly keeps tool availability and
tool-call guardrails visible at the tool boundary. GitClaw should add the
missing evidence-chain readback without enabling writes:
`@gitclaw /approvals provenance` and local `gitclaw approvals provenance`
should report current issue-label counts and hashes, actor/preflight source,
write-request detection source, assistant-turn marker counts and hashes, and
the read-only runtime boundary. It should never print raw issue bodies,
comments, prompts, run URLs, approval payloads, credentials, or secrets. Live
E2E must seed a real model-backed turn first, run the deterministic provenance
report, and then run a second real model/tool follow-up so provenance testing
does not replace inference coverage.

2026-06-01 approvals-catalog follow-up: OpenClaw's current approval docs expose
both the policy stack and the operator CLI (`openclaw approvals`) for inspecting
local, gateway, and node approval state, while Hermes' current security docs
frame dangerous-command approval as one layer in a broader defense-in-depth
model. GitClaw should add the equivalent lightweight catalog before enabling
any mutation: `@gitclaw /approvals catalog` and local
`gitclaw approvals catalog` should enumerate the approval commands, trusted
association source, write-request label, per-issue approval labels,
managed-label collision audit, assistant-marker evidence, read-only GitHub
Actions runtime, and body-free payload gate. The report must be metadata-only:
no approvals granted, no command execution, no repository mutation, no model
call, and no raw issue/comment/prompt/tool/approval payloads. Acceptance
requires a live deterministic catalog issue and then a real GitHub Models
follow-up that uses the repo-reader skill and `gitclaw.search_files`.

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

### 2026-05-31 Session Stats Follow-Up

OpenClaw keeps stored session discovery and transcript inspection as explicit
CLI surfaces: `openclaw sessions` lists bounded persisted conversation rows,
while `openclaw transcripts` exposes read-only selectors, metadata, summaries,
and transcript paths. Hermes similarly treats sessions as durable, resumable
conversation state; its resume panel shows compact recent-turn context and
collapses tool calls without printing internal tool results.

GitClaw should mirror the useful parts without adding a private session
database. `@gitclaw /session stats` and
`gitclaw session stats --backup <issue.json>` should summarize the GitHub issue
conversation in place: comment/transcript role counts, trust and edited counts,
assistant-turn marker totals, prompt-provenance totals, model-backed versus
deterministic turns, model names, prompt-visible skill/tool names, channel and
proactive markers, and byte/line totals. It should not list raw messages,
prompts, search queries, or tool outputs. This gives operators a Hermes-style
compact recap and an OpenClaw-style session inspection command while preserving
GitHub issues and git-backed backups as the canonical storage layer.

### 2026-05-31 Backup Drill Follow-Up

OpenClaw's backup command treats recovery as an explicit archive/verify path
with manifest metadata, while Hermes sessions and checkpoint/rollback docs
emphasize resumable conversation history and recoverable state. GitClaw's
GitHub-native equivalent should keep that recovery confidence inside the repo:
`gitclaw backup drill --issue <number>` composes backup verification,
single-issue coverage, and a dry-run restore plan against a fetched
`gitclaw-backups` branch.

Issue-side `@gitclaw /backup drill` should remain metadata-only and deferred
because the backup branch is written after the assistant turn. E2E coverage
must prove both halves: the deterministic body-free drill report and a real
GitHub Models follow-up with repo-reader/tool evidence.

### 2026-05-31 Soul Anchors Follow-Up

OpenClaw's workspace map treats files such as `AGENTS.md`, `SOUL.md`,
`USER.md`, `IDENTITY.md`, `TOOLS.md`, `HEARTBEAT.md`, `MEMORY.md`, and dated
memory notes as the agent's loaded home context, while Hermes profiles isolate
`config.yaml`, `.env`, `SOUL.md`, memories, sessions, skills, cron jobs, and
state database per agent. GitClaw should expose the same concept as a
repo-native authority map rather than a hidden prompt detail.

`gitclaw soul anchors` and `@gitclaw /soul anchors` should report anchor names,
roles, authority layers, required/loaded/prompt-visible/canonical flags, short
hashes, validation gates, risk gates, and mutation-disabled gates without
printing raw soul, user, memory, tool, issue, comment, prompt, or secret
bodies. The live E2E must pair the deterministic anchors report with a normal
GitHub Models follow-up that proves repo-reader and prompt-visible tool
provenance still work.

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

2026-05-31 memory-catalog follow-up: OpenClaw's memory-file convention favors
compact, reviewed Markdown state over hidden mutable memory, and Hermes frames
memory as layered durable facts, procedural skills, and searchable session
recall. GitClaw should add `@gitclaw /memory catalog` and
`gitclaw memory catalog` as a body-free discovery surface that reports durable
memory entries, procedural-memory boundaries, session-search boundaries,
prompt visibility, load modes, reason codes, hashes, validation/risk gates,
and the required live LLM/tool E2E proof without dumping memory, prompt, issue,
comment, session, embedding, credential, or secret bodies.

2026-05-31 memory-timeline follow-up: OpenClaw's memory docs continue to frame
memory as editable, reviewed Markdown context, while the sessions/transcripts
docs keep conversation history as an inspectable surface rather than hidden
agent state. Hermes' memory design keeps prompt memory compact and separates
larger searchable/session recall, which reinforces the GitClaw choice to expose
`.gitclaw/MEMORY.md` and dated `.gitclaw/memory/*.md` notes as a body-free
chronology. Add `@gitclaw /memory timeline` and `gitclaw memory timeline` so a
maintainer can inspect repo-local memory ordering, prompt visibility,
first/latest note, dated-note gaps, validation/risk gates, hashes, and the
LLM-backed E2E requirement without dumping memory bodies.

2026-05-31 soul-provenance follow-up: OpenClaw's Hermes migration path copies
memory and skills into the new workspace rather than treating them as opaque
runtime state, and Hermes profiles isolate config, sessions, skills, and
memory per agent. GitClaw should make that reviewed-state boundary visible by
adding `@gitclaw /soul provenance` and `gitclaw soul provenance`: a body-free
git provenance report for loaded soul/profile files that shows tracked state,
last commit IDs/dates, subject hashes, validation/risk gates, and no raw file
bodies, commit subjects, or author identities.

2026-05-31 soul-catalog follow-up: OpenClaw's workspace-file model and Hermes'
profile isolation both point to a compact discovery surface before raw context
loading. GitClaw should add `@gitclaw /soul catalog` and
`gitclaw soul catalog` as the body-free authority catalog: anchor names,
authority layers, load modes, reason codes, short hashes, validation/risk
gates, and disabled mutation/profile-export gates, with no raw soul,
identity, user, memory, tool, prompt, issue, comment, or description bodies.

2026-05-31 skill-provenance follow-up: OpenClaw's skills CLI can list, check,
install, update, verify, and workshop proposed skills, while its skills
security docs warn that third-party skills are trusted code and should be
read before enabling. Hermes uses progressive disclosure (`skills_list`,
`skill_view`) and distinguishes bundled/official/trusted/community skill
sources with security scanning. GitClaw should keep the GitHub-native variant
review-first by adding `@gitclaw /skills provenance` and
`gitclaw skills provenance`: a body-free report for repo-local `SKILL.md`
files that shows source roots, selected state, requirement counts, tracked
state, dirty state, last commit IDs/dates, subject hashes, validation/risk
gates, disabled installer/mutation gates, and no raw skill bodies, requirement
names, commit subjects, author identities, or installer output.

2026-05-31 skill-catalog follow-up: OpenClaw's current `skills list` surface
includes an eligibility view, while Hermes keeps compact discovery separate
from `skill_view` body loading. GitClaw should preserve that progressive
disclosure boundary with `@gitclaw /skills catalog` and
`gitclaw skills catalog`: a compact body-free eligibility index that reports
enabled/blocked/missing-requirement counts, load modes, reason codes,
selected/always state, validation/risk rollups, description hashes, and body
hashes, while disabling registry/install/update actions and avoiding raw skill
bodies, raw descriptions, env names, issue bodies, prompts, and tool outputs.

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
- OpenClaw prompt caching docs: https://docs.openclaw.ai/reference/prompt-caching
- OpenClaw token use and cost diagnostics: https://docs.openclaw.ai/reference/token-use
- OpenClaw automation hooks docs: https://docs.openclaw.ai/automation/hooks
- OpenClaw plugin hooks docs: https://docs.openclaw.ai/plugins/hooks
- OpenClaw plugins docs: https://docs.openclaw.ai/plugins
- OpenClaw config CLI docs: https://docs.openclaw.ai/cli/config
- OpenClaw configure docs: https://docs.openclaw.ai/cli/configure
- OpenClaw doctor docs: https://docs.openclaw.ai/doctor
- OpenClaw backup docs: https://docs.openclaw.ai/cli/backup
- OpenClaw transcripts CLI docs: https://docs.openclaw.ai/cli/transcripts
- OpenClaw creating skills docs: https://docs.openclaw.ai/tools/creating-skills
- OpenClaw skill format docs: https://docs.openclaw.ai/clawhub/skill-format
- OpenClaw migration guide: https://docs.openclaw.ai/install/migrating
- OpenClaw sandbox vs tool policy vs elevated: https://docs.openclaw.ai/gateway/sandbox-vs-tool-policy-vs-elevated
- OpenClaw exec approvals: https://docs.openclaw.ai/tools/exec-approvals
- OpenClaw approvals CLI docs: https://docs.openclaw.ai/cli/approvals
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
- OpenClaw skills docs: https://docs.openclaw.ai/tools/skills
- OpenClaw skills CLI docs: https://docs.openclaw.ai/cli/skills
- OpenClaw Skill Workshop plugin docs: https://docs.openclaw.ai/plugins/skill-workshop
- GitHub Actions artifact storage docs: https://docs.github.com/en/actions/how-tos/writing-workflows/choosing-what-your-workflow-does/storing-and-sharing-data-from-a-workflow
- `actions/upload-artifact` action: https://github.com/actions/upload-artifact
- GitHub Models quickstart: https://docs.github.com/en/github-models/quickstart
- GitHub Models catalog REST API: https://docs.github.com/en/rest/models/catalog
- GitHub Models REST inference API: https://docs.github.com/en/rest/models/inference
- GitHub Models billing and rate-limit notes: https://docs.github.com/en/billing/concepts/product-billing/github-models
- GitHub Models direct-use costs and multipliers: https://docs.github.com/en/billing/reference/costs-for-github-models
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
- Hermes system prompt guidance: https://hermes-agent.ai/blog/hermes-agent-system-prompt
- Hermes skills docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/skills.md
- Hermes working with skills docs: https://hermes-agent.nousresearch.com/docs/guides/work-with-skills/
- Hermes tools docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/tools/
- Hermes security docs: https://hermes-agent.nousresearch.com/docs/user-guide/security
- Hermes Tool Search docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/tool-search
- Hermes context compression and caching docs: https://hermes-agent.nousresearch.com/docs/developer-guide/context-compression-and-caching/
- Hermes features overview: https://hermes-agent.nousresearch.com/docs/user-guide/features/overview/
- Hermes tools reference: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/reference/tools-reference.md
- Hermes subagent delegation docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/delegation
- Hermes Kanban docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/kanban
- Hermes security overview: https://hermes-agent.nousresearch.com/docs/
- Hermes cron docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/cron.md
- Hermes cron internals docs: https://hermes-agent.nousresearch.com/docs/developer-guide/cron-internals
- GitHub Actions schedule event docs: https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#schedule
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
