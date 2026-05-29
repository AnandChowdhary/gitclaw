# GitClaw Product and Technical Spec

Date: 2026-05-29

Status: draft for alignment

## One-Line Concept

GitClaw is a GitHub-native OpenClaw-style assistant where every conversation is a GitHub issue, every follow-up is an issue comment, and every agent turn runs as a GitHub Actions job. No always-on server, no chat gateway, no WhatsApp, no self-improving skills.

## Product Thesis

OpenClaw and Hermes prove that persistent agents become useful when they have:

- a durable conversation surface,
- a runtime that can act,
- a memory/context contract,
- visible audit logs,
- a way to resume work later.

GitHub already gives us most of that:

- Issues are durable sessions.
- Issue comments are messages.
- GitHub Actions is the serverless runtime.
- The repository is the workspace.
- Commits, branches, PRs, checks, labels, and comments are the audit log.
- `GITHUB_TOKEN` gives a scoped, short-lived GitHub identity per run.
- GitHub Models can provide LLM inference from inside Actions using that
  same job token when the workflow grants `models: read`, so the MVP does not
  need a separate OpenAI/Anthropic secret.

The product should therefore be much narrower than OpenClaw: a GitHub issue assistant, not a personal assistant platform.

## Goals

- Let users open a GitHub issue and receive an agent response as a comment.
- Let users continue the conversation by commenting on the same issue.
- Run each agent turn inside GitHub Actions with no external daemon.
- Reconstruct all session state from GitHub issue body, comments, labels, repository files, and optional run artifacts.
- Keep the MVP read-only by default: answer questions, inspect repo context, summarize, explain, and propose changes.
- Add write actions only through explicit modes and permission gates.
- Make every action auditable in GitHub.

## Non-Goals

- No Telegram/Slack channel bridge in v0; v0 proves the GitHub issue core first.
- No WhatsApp, Discord, or generic multi-channel gateway in v1. Telegram/Slack
  are planned as explicit GitHub-backed bridges, not as OpenClaw-style direct
  chat runtimes.
- No self-improving skills.
- No agent-written config, skills, workflows, or memory without human review.
- No multi-agent delegation.
- No external always-on scheduler or daemon. GitHub Actions `schedule` is
  allowed for best-effort heartbeat checks.
- No broad local-machine automation.
- No hidden database required for conversation continuity.
- No autonomous push to protected branches.

## MVP User Experience

### New Conversation

User opens an issue:

```md
Title: @gitclaw explain the auth flow

Can you walk through how login tokens are created and refreshed?
```

The workflow runs and GitClaw replies:

```md
<!-- gitclaw:assistant-turn run_id=... -->
I traced this through `src/auth/...`.

Summary:
...
```

### Follow-Up

User comments:

```md
What would need to change if we moved refresh tokens to Redis?
```

The `issue_comment` workflow runs, reconstructs the full issue thread, and GitClaw replies as another comment.

### Triggering

We should support two modes:

- **Inbox repo mode:** every issue in a dedicated repository is a GitClaw conversation.
- **Per-repo assistant mode:** only issues with label `gitclaw` or title/body prefix `@gitclaw` trigger the agent.

Default for public repositories should be per-repo assistant mode with a required trigger label or prefix.
Deterministic slash commands are recognized when the issue title, comment
body, or a line in the issue body starts with the trigger prefix plus command,
such as `@gitclaw /proactive`. Inline mentions inside prose are ignored.
`@gitclaw /help` and `@gitclaw /commands` expose the current deterministic
command catalog without making a model call.

## GitHub Actions Event Model

Use issue/comment events for normal GitHub chat and `workflow_dispatch` for
explicit issue wakeups from manual runs, E2E harnesses, heartbeat/channel
pollers, or another workflow:

```yaml
on:
  issues:
    types: [opened]
  issue_comment:
    types: [created]
  workflow_dispatch:
    inputs:
      issue_number:
        required: true
      dispatch_id:
        required: false
      reason:
        required: false
```

Use a separate workflow for heartbeat:

```yaml
on:
  workflow_dispatch:
  schedule:
    - cron: "17 * * * *"
```

Important details:

- `issues.opened` starts a new session.
- `issue_comment.created` continues a session.
- `workflow_dispatch` on the main workflow wakes a specific issue by number.
  It must fetch the live issue before preflight because the dispatch payload
  carries inputs, not the full issue object.
- `dispatch_id` is the stable idempotency identity for externally mirrored
  work. Channel bridges should use source IDs such as Telegram `update_id` or
  Slack event IDs.
- `workflow_dispatch` on the heartbeat workflow starts a manual or e2e
  heartbeat pass.
- `schedule` starts a best-effort periodic heartbeat pass.
- `issue_comment` fires for both issues and pull requests, so we must ignore PR comments for the issue-chat workflow using `!github.event.issue.pull_request`.
- GitHub requires the workflow file to exist on the default branch for these events to run.
- Scheduled workflows run on GitHub's UTC cron schedule and should not be
  treated as exact timers; they can be delayed and should be idempotent.
- Actions jobs should use explicit `permissions`; never rely on repository defaults.
- Model-running jobs need `models: read` in addition to `issues: write` and
  `contents: read` when using GitHub Models.
- Comments posted with the repository `GITHUB_TOKEN` should not recursively trigger another workflow run, which prevents agent reply loops. Channel pollers that create issue comments with `GITHUB_TOKEN` must call the main workflow through `workflow_dispatch` or run the handler directly in the same job; they should not rely on the created comment to fire `issue_comment`.
- If we later use a GitHub App token or PAT, we must add explicit bot-comment filtering.

## Reference Workflow

```yaml
name: GitClaw

on:
  issues:
    types: [opened]
  issue_comment:
    types: [created]
  workflow_dispatch:
    inputs:
      issue_number:
        required: true
      dispatch_id:
        required: false
      reason:
        required: false

permissions:
  contents: read
  issues: write
  models: read

concurrency:
  group: gitclaw-${{ github.event.issue.number || inputs.issue_number }}
  cancel-in-progress: false

jobs:
  run:
    if: >
      (
        github.event_name == 'issues' &&
        !contains(github.event.issue.labels.*.name, 'gitclaw:disabled') &&
        (
          contains(github.event.issue.labels.*.name, 'gitclaw') ||
          startsWith(github.event.issue.title, '@gitclaw')
        )
      ) ||
      (
        github.event_name == 'issue_comment' &&
        !github.event.issue.pull_request &&
        !contains(github.event.issue.labels.*.name, 'gitclaw:disabled') &&
        (
          contains(github.event.issue.labels.*.name, 'gitclaw') ||
          startsWith(github.event.issue.title, '@gitclaw')
        )
      )
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@v5
        with:
          fetch-depth: 1

      - uses: actions/setup-go@v6
        with:
          go-version: stable

      - run: go run ./cmd/gitclaw handle --event "$GITHUB_EVENT_PATH"
        env:
          GH_TOKEN: ${{ github.token }}
          GITHUB_TOKEN: ${{ github.token }}
          GITCLAW_MODEL: openai/gpt-5-mini
```

Later, when GitClaw is released as a binary, the workflow should download the pinned release binary instead of compiling on every run.

## Reference Heartbeat Workflow

Heartbeat is the GitHub-native replacement for an OpenClaw-style periodic
awareness loop. It does require a scheduled workflow; without `schedule`,
heartbeat only runs when manually dispatched.

```yaml
name: GitClaw Heartbeat

on:
  workflow_dispatch:
    inputs:
      label:
        required: false
        default: gitclaw:heartbeat
      slot:
        required: false
      limit:
        required: false
        default: "3"
  schedule:
    - cron: "17 * * * *"

permissions:
  contents: read
  issues: write
  models: read

concurrency:
  group: gitclaw-heartbeat
  cancel-in-progress: false

jobs:
  heartbeat:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version: stable
      - run: go run ./cmd/gitclaw heartbeat --repo "$GITHUB_REPOSITORY"
        env:
          GH_TOKEN: ${{ github.token }}
          GITHUB_TOKEN: ${{ github.token }}
          GITCLAW_MODEL: openai/gpt-5-mini
```

Heartbeat behavior:

- scan open issues labeled `gitclaw:heartbeat`,
- skip pull requests and issues labeled `gitclaw:disabled`,
- reconstruct the issue transcript the same way normal issue chat does,
- load `.gitclaw/HEARTBEAT.md` and other repo context,
- append a trusted synthetic heartbeat instruction,
- call GitHub Models with the Actions token,
- post a short issue comment only when the model does not return
  `HEARTBEAT_OK`,
- include a hidden `gitclaw:heartbeat` marker with run id, run URL, and
  idempotency slot.

Default idempotency slot: current UTC hour. Manual dispatch and E2E can pass an
explicit slot. Re-running the same slot must not create a second heartbeat
comment.

## Proactive Usefulness

GitClaw supports a first proactive primitive without introducing an always-on
daemon. The primitive is a GitHub Actions workflow that creates or reuses a
GitHub issue, then wakes the normal issue handler with `workflow_dispatch`.
Template workflows can add `schedule` triggers for cron-like behavior.

This is distinct from heartbeat:

- heartbeat checks existing opted-in issues and usually says nothing,
- proactive jobs create their own issue thread when there is work to do,
- each proactive job has an explicit schedule, prompt, permissions, and
  idempotency key,
- every proactive action is visible as an issue, comment, workflow run, label,
  and optional backup.

Examples:

- weekday email triage summary,
- reminders that open an issue at a due time,
- dependency or CI health checks,
- weekly repository hygiene reports,
- "watch this API/product/page" checks,
- personal inbox or notification digest.

Proactive jobs should be easy to create, but not silently self-installed by the
agent. The enqueue primitive is:

```text
gitclaw proactive enqueue \
  --name email-triage \
  --slot 2026-05-29 \
  --prompt-file .gitclaw/proactive/email-triage.md
```

It is exposed through `.github/workflows/gitclaw-proactive.yml` for manual
dispatch, a weekly default scheduled run, and E2E. The checked-in default uses
`.gitclaw/proactive/repo-hygiene.md` so a repository has a working proactive
job without a daemon. GitClaw also ships a safe generator command that creates
additional scheduled workflows plus prompt files as ordinary reviewed repo
files:

```text
gitclaw proactive init \
  --name email-triage \
  --cron "17 8 * * 1-5" \
  --prompt-file .gitclaw/proactive/email-triage.md \
  --prompt-body "Summarize inbox state and open an issue only when action is needed."
```

`--prompt` is accepted as a path alias for `--prompt-file`. If no prompt file
is supplied, the generator defaults to `.gitclaw/proactive/<name>.md`; if no
workflow file is supplied, it defaults to
`.github/workflows/gitclaw-proactive-<name>.yml`. The command refuses to
overwrite differing files unless `--force` is used, supports `--dry-run`, and
prints a body-free `GitClaw Proactive Init Report` with file paths, write
status, byte counts, and hashes. Generated files are:

```text
.github/workflows/gitclaw-proactive-email-triage.yml
.gitclaw/proactive/email-triage.md
```

Reference proactive workflow shape:

```yaml
name: GitClaw Proactive Email Triage

on:
  workflow_dispatch:
  schedule:
    - cron: "17 8 * * 1-5"

permissions:
  actions: write
  contents: read
  issues: write

concurrency:
  group: gitclaw-proactive-email-triage
  cancel-in-progress: false

jobs:
  enqueue:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version: stable
      - id: enqueue
        run: >
          go run ./cmd/gitclaw proactive enqueue
          --name email-triage
          --slot "$(date -u +%Y-%m-%d)"
          --prompt-file .gitclaw/proactive/email-triage.md
      - run: >
          gh workflow run .github/workflows/gitclaw.yml
          -f issue_number="${{ steps.enqueue.outputs.issue_number }}"
          -f dispatch_id="proactive-email-triage-${{ steps.enqueue.outputs.slot }}"
          -f reason="proactive:email-triage"
```

Proactive issue bodies should include a hidden marker:

```md
<!-- gitclaw:proactive-run name="email-triage" slot="2026-05-29" -->
```

### Proactive Inspection Command

GitClaw supports a deterministic proactive audit command:

```text
@gitclaw /proactive
```

The command runs after normal preflight and context loading, but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/proactive"` and summarizes:

- proactive and trigger labels,
- the generic proactive workflow path,
- whether `workflow_dispatch` and `schedule` triggers are present,
- configured `.gitclaw/proactive/*.md` prompt files by path, size, and hash,
- whether the current issue is itself a proactive-run thread,
- the enqueue/idempotency contract.

It never dumps prompt, issue, or comment bodies. The command is for safe
operator visibility before adding or editing scheduled jobs.

Idempotency rules:

- one issue per `name + slot` unless a future job declares rolling-thread mode,
- rerunning the same slot updates or reuses the existing issue,
- the main handler `dispatch_id` is derived from `name + slot`,
- proactive jobs must not loop by reacting to their own assistant comments.

Security rules:

- external integrations such as email require explicit secrets and scopes,
- proactive workflow files are reviewed code, not model-authored side effects,
- job prompts live in `.gitclaw/proactive/` and are treated as repo context,
- generated workflows must use least-privilege permissions,
- write-capable proactive jobs still go through the normal write gates.

## Model Provider Default: GitHub Models

Research update: GitHub Models should be the default inference provider for
the GitHub-native MVP.

Why:

- GitHub-hosted Actions runners automatically receive `GITHUB_TOKEN`.
- GitHub Models can be called from Actions with `GITHUB_TOKEN` when the job
  grants `models: read`.
- The API is chat-completions shaped, so the GitClaw client can stay
  OpenAI-compatible while defaulting to GitHub-hosted inference.
- The public endpoint is:

```text
https://models.github.ai/inference/chat/completions
```

- Organization-governed deployments can use the org-scoped endpoint:

```text
https://models.github.ai/orgs/<org>/inference/chat/completions
```

Default MVP behavior:

- provider: `github-models`
- model: `openai/gpt-5-mini`
- auth token lookup: `GITHUB_TOKEN`, then `GH_TOKEN`, then optional
  `GITCLAW_LLM_API_KEY` for local/manual runs
- base URL override: `GITCLAW_LLM_BASE_URL`
- model override: `GITCLAW_MODEL`

`openai/gpt-5-mini` is the recommended default because it is the smallest
new-generation OpenAI model currently visible in the GitHub Models Marketplace.
The first assistant version is issue-thread chat plus repository context
summarization, where latency and cost matter more than maximum reasoning depth.
Repositories can override to `openai/gpt-5.4-mini`, `openai/gpt-4o`, or another
GitHub Models catalog model when that model is available to the repository.

Fallback provider rule:

- GitHub Models is the hosted default.
- Generic OpenAI-compatible providers are supported by setting
  `GITCLAW_LLM_BASE_URL`, `GITCLAW_LLM_API_KEY`, and `GITCLAW_MODEL`.
- Provider-specific SDKs are not required in v0.

Security and operational notes:

- The model job should grant only `contents: read`, `issues: write`, and
  `models: read` for conversation-only v0.
- Preflight/authorization should run before model inference and should not
  require `models: read`.
- Workflow snippets and checked-in workflows should use Node 24-compatible
  first-party Actions majors: `actions/checkout@v5`, `actions/setup-go@v6`,
  and `actions/upload-artifact@v6`. This keeps the GitHub-native runtime ahead
  of GitHub's 2026 Node.js 20 runner deprecation warnings.
- Do not dump raw prompts into logs by default; if prompt artifacts are enabled,
  redact secrets and mark issue text as untrusted input.
- GitHub Models has free but rate-limited usage and optional paid usage, so
  the model client retries transient `429`, timeout, and `5xx` responses before
  surfacing a safe issue-level provider failure. Individual model HTTP requests
  are also time-bounded so a stuck inference call cannot consume the whole
  Actions job timeout, and provider `Retry-After` values are capped so dense
  E2E runs do not park a workflow for an unbounded cooldown window.
- Retry delays use bounded exponential backoff. The default source build uses
  five attempts, a 60 second request timeout, a 5 second base delay, and a
  60 second maximum delay. The checked-in Actions workflow is more patient for
  live model-backed runs: six attempts, a 75 second request timeout, a
  10 second base delay, and a 90 second maximum delay.

### Model Inspection Command

GitClaw supports a deterministic model/provider audit command:

```text
@gitclaw /models
```

The command runs after normal preflight and context loading, but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/models"` and summarizes:

- provider family,
- selected model,
- endpoint host without URL credentials,
- token source name without token value,
- request timeout,
- retry attempts,
- retry base and maximum delay,
- retryable status categories,
- prompt-artifact enablement.

It never dumps issue/comment bodies, API keys, full prompts, or raw provider
error bodies. This gives operators a safe way to inspect GitHub Models and
OpenAI-compatible provider wiring from the issue thread before burning model
quota on a real assistant turn.

## Runtime Architecture

```text
GitHub issue/comment event
  -> GitHub Actions job
  -> gitclaw handle
  -> event gate and authorization
  -> fetch issue + comments
  -> build transcript
  -> load repo context
  -> run model/tool loop
  -> post assistant comment
  -> update labels/artifacts
```

### Components

`cmd/gitclaw`

- CLI entry point.
- Subcommands: `preflight`, `handle`, `backup`, `backup retention-plan`,
  `heartbeat`,
  `channel-ingest`, `proactive enqueue`, `proactive init`,
  `memory validate`, `skills validate`, `skills info`, `skills search`,
  `soul validate`, `soul search`, `tools validate`, `doctor`, `commands`,
  `version`.

`internal/github`

- Reads event payload.
- Fetches issue, comments, labels, repository metadata.
- Posts comments and manages labels.
- Uses `GH_TOKEN` or `GITHUB_TOKEN`.

`internal/session`

- Converts issue body and comments into a normalized transcript.
- Identifies prior GitClaw assistant comments via hidden HTML markers.
- Drops bot noise and workflow status comments.

`internal/context`

- Loads repo context files.
- Builds the system prompt and run prompt.
- Applies token budgets.
- Loads repo-native soul, tools notes, curated memory, and local skills from
  git-tracked files.
- Builds a skill index from `SKILL.md` frontmatter and loads full skill bodies
  only when selected or marked always-on.
- Runs bounded read-only repository tools before the model turn.
- Supports deterministic `@gitclaw /context` reports so maintainers can inspect
  which context files, skills, and tool outputs were assembled without making a
  model call.

`internal/agent`

- Calls the selected LLM provider.
- In MVP, no autonomous shell execution.
- Optional read-only tools can include file search and file read against the checkout.

`internal/policy`

- Author/association gate.
- Trigger gate.
- Action permission gate.
- Output size and rate limits.

`internal/comment`

- Renders assistant replies.
- Adds provenance marker.
- Splits long replies into multiple comments if needed.

## Conversation Model

One issue equals one session.

Transcript mapping:

- Issue title + body become the first user message.
- Each non-bot issue comment becomes a later user message.
- Each GitClaw-marked bot comment becomes an assistant message.
- Non-GitClaw bot comments are ignored by default.
- Edited comments are read in their latest GitHub state.

No external session DB is required.

Hidden assistant marker:

```html
<!-- gitclaw:assistant-turn run_id=123 event_id=456 model=openai/gpt-5-mini -->
```

Hidden status marker:

```html
<!-- gitclaw:status run_id=123 state=running -->
```

## Session Inspection Command

GitClaw supports a deterministic session audit command inspired by OpenClaw's
transcript/session CLIs and Hermes' saved/searchable sessions:

```text
@gitclaw /session
```

The command runs after normal preflight authorization and transcript
reconstruction, but before model inference. It posts a `gitclaw:assistant-turn`
comment with `model="gitclaw/session"` and summarizes:

- raw comment count and reconstructed transcript message count,
- user/assistant and trusted/untrusted message counts,
- GitClaw assistant, heartbeat, error, and channel-message marker counts,
- whether the issue is a channel-thread or proactive-run issue,
- per-transcript-message source, actor, trust state, size, line count, and
  short hash.

It never dumps issue/comment bodies. The hashes make session reconstruction
debuggable without turning the issue-visible report into another raw transcript
copy.

## Context Contract

Borrow the useful parts of OpenClaw and Hermes, but make them repo-native:

```text
AGENTS.md                    # existing coding-agent instructions, if present
.gitclaw/GITCLAW.md          # GitClaw-specific repo instructions
.gitclaw/POLICY.md           # repo-local permission and behavior policy
.gitclaw/SOUL.md             # persona, boundaries, and tone
.gitclaw/IDENTITY.md         # agent identity and product framing
.gitclaw/USER.md             # maintainer preferences, human-reviewed only
.gitclaw/HEARTBEAT.md        # scheduled heartbeat checklist
.gitclaw/SKILLS/*.md         # optional read-only local skills, v1+
.gitclaw/MEMORY.md           # optional curated repo memory, human-reviewed only
.gitclaw/memory/YYYY-MM-DD.md # dated working memory notes, human-reviewed only
```

MVP loads:

- `AGENTS.md`
- `.github/copilot-instructions.md`, if present
- `.gitclaw/GITCLAW.md`, if present
- `.gitclaw/SOUL.md`, if present
- `.gitclaw/IDENTITY.md`, if present
- `.gitclaw/USER.md`, if present and human-reviewed
- `.gitclaw/TOOLS.md`, if present
- `.gitclaw/MEMORY.md`, if present and human-reviewed
- latest bounded `.gitclaw/memory/*.md` notes, if present and human-reviewed
- `.gitclaw/HEARTBEAT.md`, if present, for heartbeat turns and as optional
  issue context
- `.gitclaw/SKILLS/*/SKILL.md`, if selected by the issue thread or marked
  always-on
- issue thread transcript
- small repository summary from a read-only file listing
- bounded `gitclaw.read_file` output for files explicitly mentioned in the
  issue thread

Do not let the agent write these files in MVP. Skills, soul, tools notes, and
memory are git-backed source files: edits happen through normal human-reviewed
commits, not hidden agent mutation.

## Memory Inspection Command

GitClaw supports a deterministic memory audit command inspired by OpenClaw's
Markdown memory files and Hermes' split between compact prompt memory and
larger session recall:

```text
@gitclaw /memory
@gitclaw /memory validate
@gitclaw /memory search backup branch
```

The command runs after normal context assembly, but before model inference. It
posts a `gitclaw:assistant-turn` comment with `model="gitclaw/memory"` and
summarizes:

- whether `.gitclaw/MEMORY.md` exists and was loaded,
- total `.gitclaw/memory/*.md` notes,
- canonical `YYYY-MM-DD.md` dated note count,
- noncanonical note count,
- loaded dated memory note count,
- max loaded memory note budget,
- omitted note count,
- latest canonical dated note path,
- byte, line, and short hash metadata for memory files,
- memory validation status, error/warning counts, and body-free findings.

It never dumps memory file bodies, issue bodies, or comments. Memory remains
read-only during assistant turns; edits require normal reviewed git changes.

When called as `@gitclaw /memory search <query>`, the command searches
git-backed memory files with a local lexical matcher. It reports query hash,
term count, scanned file count, matched file/line counts, paths, line numbers,
scores, loaded-for-this-turn state, and file/line hashes. It does not echo the
raw query because query text comes from issues and may contain secrets.

When called as `@gitclaw /memory validate`, the command renders only the
memory-hygiene report. Local operators can run the same validation with:

```bash
gitclaw memory validate
gitclaw memory search <query> --max-results 10
gitclaw memory search --query <query> --max-results 10
```

The validator checks for:

- missing or empty `.gitclaw/MEMORY.md`,
- noncanonical `.gitclaw/memory/*.md` filenames,
- empty memory notes,
- memory files at the context byte limit,
- obvious secret-like token patterns.

The output includes paths, counts, hashes, and short finding details only. It
does not print memory bodies or matched secret values.

## Skill Loading

GitClaw skills use the AgentSkills/OpenClaw shape: a skill is a directory with
a `SKILL.md` file and optional YAML frontmatter.

Supported MVP frontmatter:

```yaml
---
name: repo-reader
description: Use read-only repository files.
metadata:
  openclaw:
    always: false
    requires:
      env:
        - GITHUB_TOKEN
      bins:
        - gh
---
```

GitClaw inserts a compact `gitclaw.skill_index` tool output listing all
discovered local skills. The index includes only review metadata: path, byte
and line counts, short hash, frontmatter/description presence, `always`, and
counts of declared/missing runtime requirements. Full skill instructions are
loaded only when:

- the user mentions the skill name, folder, path, or relevant description terms;
- the skill declares `always: true` or `metadata.openclaw.always: true`.

Remote skill installation, skill execution scripts, dependency installation,
and agent-authored skill edits remain out of scope.

### Skill Validation

GitClaw validates discovered skills against the safe subset borrowed from
OpenClaw/AgentSkills:

- `SKILL.md` should start with YAML frontmatter,
- `name` must be lower hyphen-case: `^[a-z0-9][a-z0-9-]*$`,
- `description` should be present,
- leaf folder name should match the effective skill name,
- duplicate effective names are warned about,
- missing declared env/bin requirements are warned about.

Validation is visible in the `/skills` report and locally through:

```bash
gitclaw skills validate
gitclaw skills info <name>
gitclaw skills search <query>
```

The validation output includes only paths, counts, hashes, and short finding
details. It never dumps full `SKILL.md` bodies.

## Skills Inspection Command

GitClaw supports a deterministic skill inventory command inspired by
OpenClaw's `openclaw skills` commands and Hermes' `skills_list` /
`skill_view` split:

```text
@gitclaw /skills
@gitclaw /skills info repo-reader
@gitclaw /skills search repository context
```

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/skills"` and summarizes:

- available local skills from git-tracked `SKILL.md` files,
- selected skills for the current issue/comment,
- `always` activation state and frontmatter descriptions,
- short hashes and size metadata for review,
- declared env/bin requirement counts and whether any are missing.
- validation status, error/warning counts, duplicate-name count, invalid-name
  count, folder/name mismatch count, and body-free findings.

It does not dump full skill bodies. Full `SKILL.md` content remains a prompt
input only when selected by the normal progressive-disclosure rules.

When called as `@gitclaw /skills info <name>`, the same deterministic command
switches from inventory mode to focused skill-info mode. The info report shows
one skill's safe metadata:

- requested name and match count,
- path, folder, byte/line counts, and content hash,
- whether the skill was selected for this turn,
- `always`, frontmatter, and description presence,
- declared and missing env/bin requirement names/counts,
- validation findings for matching skill files only.

This mirrors OpenClaw's `skills info <name>` and Hermes' progressive
`skills_list()` / `skill_view(name)` split while preserving GitClaw's rule that
issue-visible diagnostics never dump full skill bodies or secret values.

When called as `@gitclaw /skills search <query>`, the command switches to
body-safe metadata search. It searches skill names, leaf folders, paths, and
frontmatter descriptions, then reports match counts, match fields, selection
state, hashes, sizes, and requirement counts. The raw search query is
represented only by a short hash and term count because the query itself comes
from issue text and may contain secrets.

## Soul Validation

GitClaw validates the high-authority context surface that OpenClaw/Hermes-style
agents rely on for durable identity and policy:

- `.gitclaw/SOUL.md` should be present and non-empty,
- `.gitclaw/IDENTITY.md` should be present and non-empty,
- `.gitclaw/USER.md` should be present and non-empty,
- `.gitclaw/TOOLS.md` should be present and non-empty,
- `.gitclaw/MEMORY.md` should be present and non-empty,
- `.gitclaw/HEARTBEAT.md` should be present and non-empty,
- dated memory notes should use `.gitclaw/memory/YYYY-MM-DD.md`,
- context files at the prompt loading limit are warned about because their
  bodies may have been truncated before model inference.

Validation is visible in the `/soul` report and locally through:

```bash
gitclaw soul validate
gitclaw soul search <query> --max-results 10
```

The validation output includes only paths, counts, and short finding details.
It never dumps full soul, user, memory, tool, or heartbeat file bodies.

## Soul Inspection Command

GitClaw supports a deterministic high-authority context audit command inspired
by OpenClaw and Hermes' portable workspace files:

```text
@gitclaw /soul
@gitclaw /soul search durable state layer
```

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/soul"` and summarizes:

- loaded identity and policy files such as `AGENTS.md`, `.gitclaw/SOUL.md`,
  `.gitclaw/IDENTITY.md`, `.gitclaw/USER.md`, `.gitclaw/TOOLS.md`,
  `.gitclaw/MEMORY.md`, and `.gitclaw/HEARTBEAT.md`,
- loaded dated memory notes from `.gitclaw/memory/*.md`,
- byte counts, line counts, and short hashes for each file,
- soul validation status, error/warning counts, required-file counts,
  memory-note counts, noncanonical memory-note counts, and body-free findings.

It never dumps full file bodies. The hashes make the issue-visible report
verifiable without exposing private user, memory, or policy text.

When called as `@gitclaw /soul search <query>`, the command searches only the
loaded high-authority context files with a local lexical matcher. It reports
the query hash and term count, scanned and matched file counts, result limits,
paths, categories, line numbers, scores, and file/line hashes. It never emits
raw soul, user, memory, tools, heartbeat, issue, comment, prompt, or raw query
bodies. This is the body-safe equivalent of inspecting OpenClaw/Hermes
workspace context when debugging why the assistant should have remembered a
stable identity, policy, or operating convention.

## Read-Only Tool Context

GitClaw v1 adds a small deterministic tool layer before the model call:

- `gitclaw.list_files`: lists a bounded set of repository files in the checkout.
- `gitclaw.search_files`: searches bounded text files for explicit quoted
  phrases or identifiers from the issue thread and returns matching lines.
- `gitclaw.read_file`: reads a bounded text file only when the issue thread
  explicitly mentions that repository-relative path.

Tool outputs are inserted into the prompt as auditable context blocks. They are
not autonomous shell execution, and they do not mutate the repository.

## Tool Validation

GitClaw validates the deterministic tool surface against the OpenClaw/Hermes
safety split between callable tools, procedural skills, plugins, and toolsets:

- every declared GitClaw tool contract must use the `gitclaw.` namespace,
- every declared contract must be `read-only` or `metadata-only`,
- duplicate contracts are errors,
- `.gitclaw/TOOLS.md` should be loaded as the repo-local tool guidance file,
- every active tool output must have a declared contract,
- active outputs must stay within their configured caps for file listing,
  search matches, bounded file reads, skill index metadata, and policy output.

Validation is visible in the `/tools` report and locally through:

```bash
gitclaw tools validate
```

The validation output includes only names, counts, and short finding details.
It never dumps full tool outputs, file bodies, or search result bodies.

## Tools Inspection Command

GitClaw supports a deterministic tool-surface audit command inspired by
OpenClaw's tool policy visibility and Hermes' toolset inventory:

```text
@gitclaw /tools
```

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/tools"` and summarizes:

- available deterministic GitClaw tool contracts and their trigger conditions,
- `.gitclaw/TOOLS.md` metadata, if present,
- active tool outputs generated for the current issue/comment,
- each active output's input, byte count, line count, and short hash,
- tool validation status, error/warning counts, contract counts, active-output
  counts, unknown-output counts, unsafe-contract counts, over-limit output
  counts, missing-guidance count, duplicate-contract count, and body-free
  findings.

It never dumps full tool output bodies. Tool output bodies remain prompt inputs
only; the issue-visible report exposes enough metadata to debug whether
`gitclaw.list_files`, `gitclaw.search_files`, `gitclaw.read_file`,
`gitclaw.skill_index`, or `gitclaw.policy` ran for the turn.

## Context Inspection Command

GitClaw supports a deterministic context inspection command inspired by
OpenClaw's `/context` diagnostics:

```text
@gitclaw /context
```

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/context"` and summarizes:

- selected context files,
- selected full skill documents,
- read-only tool outputs and their input/size,
- transcript and prompt-budget settings.

It never dumps full file bodies, skill bodies, prompts, or tool output contents
into the issue. This makes context visibility cheap enough for routine E2E
debugging and avoids burning GitHub Models quota for a diagnostic turn.

## Prompt Inspection Command

GitClaw supports a deterministic prompt-budget audit command inspired by
OpenClaw's context diagnostics and Hermes' bounded memory/context posture:

```text
@gitclaw /prompt
```

`@gitclaw /budget` and `@gitclaw /prompt-budget` are accepted aliases, but the
canonical advertised command is `/prompt`.

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/prompt"` and summarizes:

- provider/model and system-prompt hash metadata,
- final prompt byte count, line count, and short hash,
- configured prompt, output-token, transcript-message, and per-message body
  limits,
- transcript messages included/omitted and body truncation counts,
- whether prompt truncation markers are present,
- prompt artifact enablement and redaction-pattern count,
- context file, selected skill, and tool-output metadata.

It never dumps the prompt text, issue/comment bodies, context file bodies,
skill bodies, or tool output bodies into the issue. This gives maintainers a
low-cost way to debug prompt bloat, missing context, and truncation behavior
without leaking the exact prompt into a public or long-lived GitHub comment.

## Labels

Managed labels:

- `gitclaw`: issue should be handled by GitClaw.
- `gitclaw:running`: a run is active; added before model/tool context work.
- `gitclaw:done`: latest turn completed; added after the assistant comment is
  posted.
- `gitclaw:error`: latest turn failed; added when the run fails after it starts.
- `gitclaw:disabled`: ignore future comments.
- `gitclaw:write-requested`: user is asking for code changes; added
  deterministically before model inference when write intent is detected.

Planned labels:

- `gitclaw:needs-human`: blocked on approval or authorization.
- `gitclaw:approved`: maintainer approved a write-capable turn.

Labels are state hints, not the source of truth. The issue thread and run
artifacts remain the source of truth. Label update failures should not prevent
the assistant from answering, but the live E2E harness verifies that configured
repositories end successful turns with `gitclaw:done` and without
`gitclaw:running` or `gitclaw:error`.

## Policy Inspection Command

GitClaw supports a deterministic policy audit command inspired by OpenClaw's
sandbox/tool-policy/elevated split and Hermes' authorization and approval
posture:

```text
@gitclaw /policy
```

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/policy"` and summarizes:

- preflight result, trigger state, actor association, and trust decision,
- configured trusted GitHub author associations,
- managed labels and event labels,
- write-request detection state,
- expected least-privilege workflow permissions for preflight, handle, and
  backup jobs,
- active `gitclaw.policy` output metadata, if a policy output was injected.

It never dumps issue/comment bodies or the `gitclaw.policy` body. The report is
for checking the enforcement shape and provenance without exposing prompt text.

## Authorization And Abuse Controls

Public repos need strict defaults because any GitHub user can open issues or comment.

MVP policy:

- Run the LLM only for trusted authors by default: `OWNER`, `MEMBER`, or `COLLABORATOR`.
- For untrusted users, either ignore or post a cheap non-LLM comment asking a maintainer to add `gitclaw:approved`.
- Allow repo config to add explicit GitHub usernames or teams.
- Enforce a max prompt size and max transcript messages per run.
- Ignore GitClaw's own comments.
- Never execute shell commands based on issue text in MVP.
- Never expose secrets to model-visible logs.

Example `.gitclaw/config.yml`:

```yaml
trigger:
  mode: label-or-prefix
  label: gitclaw
  prefix: "@gitclaw"

authorization:
  allowed_associations:
    - OWNER
    - MEMBER
    - COLLABORATOR
  approved_label: gitclaw:approved
  external_user_mode: request_approval

model:
  provider: github-models
  model: openai/gpt-5-mini
  base_url: https://models.github.ai/inference/chat/completions
  max_input_tokens: 60000
  max_output_tokens: 4000

actions:
  mode: read_only
```

### Repo Config Loading

GitClaw loads `.gitclaw/config.yml` as human-reviewed repository policy before
each preflight, handle, heartbeat, proactive, or channel-ingest command. The
load order is:

1. built-in safe defaults,
2. repo `.gitclaw/config.yml`, if present,
3. environment overrides such as `GITCLAW_MODEL` and `GITCLAW_WORKDIR`.

The first supported schema is deliberately narrow:

- `trigger.label`,
- `trigger.prefix`,
- `trigger.disabled_label`,
- `authorization.allowed_associations`,
- `model.provider`,
- `model.model`,
- `model.base_url`,
- `model.max_prompt_bytes` or legacy alias `model.max_input_tokens`,
- `model.max_output_tokens`,
- `model.max_transcript_messages`,
- `model.max_transcript_message_bytes`,
- `actions.mode`, which must currently be `read_only`.

Unknown YAML fields are rejected. This mirrors OpenClaw's schema/validate
discipline without adding agent-authored config writes. Secrets do not belong
in this file; model auth continues to come from GitHub Actions tokens or
environment variables.

### Config Inspection Command

GitClaw supports a deterministic config/control-plane audit command:

```text
@gitclaw /config
```

The command runs after normal preflight and context loading, but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/config"` and summarizes:

- effective config source,
- expected `.gitclaw/config.yml` path and file presence,
- trigger label and prefix,
- managed status/feature labels,
- trusted author associations,
- selected model and prompt budget settings,
- known deterministic slash commands, including the command catalog command,
- key workflow files by path, size, and hash.

It never dumps config, workflow, issue, or comment bodies. This is the
GitHub-native equivalent of OpenClaw/Hermes config/profile status: enough to
understand the active control plane without exposing secrets or allowing the
agent to rewrite configuration.

### Command Catalog Command

GitClaw supports a deterministic command catalog command:

```text
@gitclaw /help
@gitclaw /commands
```

The command runs after normal preflight and before model inference. It posts a
`gitclaw:assistant-turn` comment with `model="gitclaw/commands"` and
summarizes:

- canonical deterministic slash commands,
- command aliases,
- deterministic model marker names used for body-free reports,
- broad command categories,
- short descriptions,
- related local CLI helpers.

It never dumps issue/comment bodies, prompts, config bodies, workflow bodies,
or backup payloads. This is the GitHub-native equivalent of the
OpenClaw/Hermes help surface: a stable issue-visible capability index that
operators can ask for before using more specific commands.

Local operators can print the same catalog with:

```bash
gitclaw commands
```

### Doctor Command

GitClaw supports a deterministic doctor/health audit command:

```text
@gitclaw /doctor
```

The command runs after preflight and context loading, but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/doctor"` and summarizes:

- whether `.gitclaw/config.yml` exists and validates,
- model provider and endpoint host metadata,
- workflow presence,
- context file presence for soul, identity, user, tools, memory, and heartbeat,
- dated memory note count,
- local skill count,
- proactive prompt count,
- managed label count,
- validation error/warning totals,
- skill, soul, and tool validation statuses plus error/warning counts,
- pass/warn checks for the core control plane and validation rollups.

It never dumps file bodies, issue bodies, comments, prompts, or secrets. This
is the GitHub-native equivalent of `openclaw config validate` plus the cold,
read-only parts of `openclaw doctor`: useful health diagnostics inside the
canonical issue log without introducing an auto-repair mode.

## Prompt Budgeting

GitClaw uses character/byte budgets before the model call rather than relying
on the provider to reject oversized prompts:

- default max prompt size: 60,000 bytes,
- default max transcript messages: 40,
- default max body per transcript message: 8,000 bytes,
- the original issue message is preserved,
- the most recent transcript tail is preserved,
- oversized bodies are middle-truncated with a `gitclaw:truncated` marker,
- omitted middle transcript turns are recorded with
  `gitclaw.prompt_budget omitted_older_messages=<n>`.

This mirrors the OpenClaw/Hermes lesson that context windows need explicit
budgeting and visible truncation. It is intentionally not semantic compaction
yet; GitClaw should first be predictable and auditable.

## Tooling Scope

### v0: Conversation-Only

- Fetch issue thread.
- Load repo instructions.
- Ask LLM.
- Reply as issue comment.

### v1: Read-Only Repo Assistant

- Let the agent search/read files from the checkout.
- Summarize relevant files.
- Explain architecture.
- Propose patches as markdown diffs, but do not apply them.

### v2: PR-Producing Assistant

Write mode requires an explicit approval label or maintainer command.
Until write mode exists, GitClaw detects write intent, applies
`gitclaw:write-requested`, and injects a `gitclaw.policy` context block telling
the model to stay in read-only proposal mode.

Capabilities:

- apply patch in a throwaway branch,
- run configured tests,
- commit changes,
- open draft PR,
- comment back with PR link and test summary.

Required workflow permissions for v2:

```yaml
permissions:
  contents: write
  issues: write
  pull-requests: write
```

Important: if PRs are created with `GITHUB_TOKEN`, follow-on CI behavior has GitHub-specific restrictions. We should support an optional GitHub App token later for repos that want automation-created PRs to trigger full CI without manual approval.

### v3: Channel Bridges

Telegram, Slack, and other chat channels should not replace GitHub issues as
the source of truth. They are bridge surfaces:

```text
Telegram/Slack message
  -> channel bridge
  -> GitHub issue or issue comment
  -> normal GitClaw issue workflow
  -> assistant issue comment
  -> bridge mirrors reply back to Telegram/Slack
```

The GitHub issue remains the canonical session, audit log, authorization unit,
and replay source. Channel messages become issue comments with provenance, not
a separate hidden conversation store.

Mirrored channel comments use a hidden provenance marker before the user-visible
message body:

```md
<!-- gitclaw:channel-message channel="telegram" message_id="123" author="telegram:42" -->
User's mirrored message text.
```

GitClaw reconstructs these comments as user transcript messages even when they
were posted by `github-actions[bot]`, but the message body remains untrusted
input in the prompt. The `message_id` should also be reused as the
`workflow_dispatch` `dispatch_id`.

## Channel Bridge Strategy

The hard constraint: Slack and Telegram cannot directly call
`workflow_dispatch` without some authenticated caller. GitHub's
`workflow_dispatch` REST endpoint requires an authenticated request with Actions
write permission, and Slack/Telegram webhooks cannot safely attach the required
GitHub Authorization header themselves. That leaves four viable tiers.

### Tier 0: GitHub-Only Core

This is v0 and must stay independently useful. GitHub issue/comment support must
not depend on any channel bridge.

### Tier 1: Polling Bridges

Use short-lived GitHub Actions runs on a schedule or manual dispatch to poll
channel APIs and convert new messages into GitHub issues/comments.

Telegram:

- use `getUpdates` long polling,
- store the last acknowledged `update_id`,
- acknowledge an update only after the matching GitHub issue/comment write
  succeeds.

Slack:

- polling is less natural than Telegram,
- possible surfaces include `conversations.history`, app mentions, or DMs, but
  this needs more scopes and careful rate-limit handling,
- use only for low-volume personal/team channels.

Tradeoffs:

- simplest no-server model,
- higher latency,
- scheduled Actions are best-effort and should not be treated as exact timers,
- needs durable offset storage.

After a poller mirrors a channel message into a GitHub issue or comment, it
must wake GitClaw explicitly. The preferred no-server path is to dispatch the
main workflow with:

- `issue_number`: canonical GitHub issue,
- `dispatch_id`: source message/event ID used for idempotency,
- `reason`: channel and bridge name for audit.

The poller may also run `gitclaw handle` directly in the same job with a
synthetic dispatch event. It should not wait for the mirrored GitHub comment to
trigger `issue_comment`, because events created with the repository
`GITHUB_TOKEN` generally do not recursively create new workflow runs.

Recommended use: optional low-latency-insensitive Telegram bridge, not the main
Slack strategy.

### Tier 1.5: Manual Channel Ingest Workflow

Before implementing provider-specific polling, GitClaw should expose a generic
`gitclaw-channel-ingest.yml` workflow. It accepts normalized channel metadata
and a message body, mirrors the message into the canonical GitHub issue, then
dispatches the main GitClaw workflow.

Inputs:

- `channel`: `telegram`, `slack`, or another provider key,
- `thread_id`: external chat/thread/conversation id,
- `message_id`: stable provider message/update/event id,
- `author`: provider-specific author id,
- `body`: text to mirror.

Behavior:

- find an open issue with a matching hidden `gitclaw:channel-thread` marker,
- create one if it does not exist,
- post the inbound message as a `gitclaw:channel-message` comment,
- apply `gitclaw` and `gitclaw:channel` labels,
- dispatch `.github/workflows/gitclaw.yml` with `issue_number` and
  `dispatch_id=<channel>-<message_id>`.

This workflow is useful for E2E, manual bridge experiments, and tiny external
dispatchers. Provider-specific pollers can later call the same CLI path after
they read Telegram/Slack events.

### Channel Inspection Command

GitClaw supports a deterministic channel/control-plane audit command:

```text
@gitclaw /channels
```

The command runs after normal preflight and context loading, but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/channels"` and summarizes:

- the canonical channel label,
- the generic channel-ingest workflow path and metadata,
- whether `workflow_dispatch` is configured,
- whether the ingest workflow has `actions: write` and `issues: write`,
- normalized workflow input count,
- whether the current issue is a channel thread,
- how many existing comments carry `gitclaw:channel-message`,
- supported provider keys and the dispatch-id contract.

It never dumps channel message, issue, command, or workflow bodies. This keeps
Slack/Telegram bridge debugging GitHub-native and auditable without making the
channel bridge itself a privileged hidden conversation store.

### Tier 2: Long-Running Actions Gateway

Run a `channel-gateway.yml` workflow via `workflow_dispatch`. The job opens a
Telegram long-poll loop and/or Slack Socket Mode WebSocket, mirrors channel
messages into GitHub issues/comments, and mirrors GitClaw replies back to the
channel.

This is the "no server, but effectively a temporary runner daemon" option.

Constraints:

- GitHub-hosted runner jobs have a finite execution window, so the gateway must
  self-renew before timeout.
- The gateway must be interrupt-safe; it can die at any point.
- A scheduled watchdog should restart it if renewal fails.
- Concurrency must enforce at most one gateway run per channel identity.
- State must be durable outside the runner filesystem.
- Runner minutes/cost become proportional to uptime.

Renewal shape:

```text
channel-gateway run starts
  -> acquire lease for channel account
  -> load durable offsets/dedupe state
  -> connect to Telegram long poll and/or Slack Socket Mode
  -> mirror inbound messages to GitHub issues/comments
  -> wake the canonical issue via workflow_dispatch using channel event ID
  -> mirror outbound GitClaw comments back to channel
  -> before timeout, workflow_dispatch next channel-gateway run
  -> release or transfer lease
```

Workflow sketch:

```yaml
name: GitClaw Channel Gateway

on:
  workflow_dispatch:
    inputs:
      channel:
        required: true
        type: choice
        options: [telegram, slack]
  schedule:
    - cron: "*/15 * * * *" # watchdog only; not exact timing

permissions:
  actions: write
  contents: read
  issues: write

concurrency:
  group: gitclaw-channel-${{ inputs.channel || 'watchdog' }}
  cancel-in-progress: false

jobs:
  gateway:
    runs-on: ubuntu-latest
    timeout-minutes: 330
    steps:
      - uses: actions/checkout@v5
      - run: gitclaw channel-gateway --channel "${{ inputs.channel }}"
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          TELEGRAM_BOT_TOKEN: ${{ secrets.TELEGRAM_BOT_TOKEN }}
          SLACK_APP_TOKEN: ${{ secrets.SLACK_APP_TOKEN }}
          SLACK_BOT_TOKEN: ${{ secrets.SLACK_BOT_TOKEN }}
```

Important: `GITHUB_TOKEN`-triggered events normally do not recursively trigger
workflows, but GitHub documents `workflow_dispatch` and `repository_dispatch` as
exceptions. If we rely on self-renewal, the workflow must explicitly grant the
token enough permission to dispatch the next run.

Durable bridge state options:

- **Bridge state issue:** one locked issue per channel account containing offset,
  lease owner, heartbeat, and dedupe markers in hidden comments.
- **State branch:** commit a small JSON file under a dedicated
  `gitclaw-state` branch.
- **Repository variables:** acceptable for low-volume offsets, but not for large
  dedupe windows or detailed audit.

Recommended MVP for this tier: bridge state issue. It is visible, auditable,
and aligned with the product's issue-native model.

Required bridge state:

- channel account id,
- current lease holder workflow run id,
- lease expiry,
- Telegram last committed `update_id`,
- Slack last seen event ids / timestamps,
- GitHub issue mapping per channel thread,
- outbound delivery markers,
- dedupe window.

Slack caveat: Socket Mode is a WebSocket-based stateful connection. Running it
inside Actions can work, but it is a best-effort bridge, not production-grade
infrastructure. Reconnect gaps, runner timeout, and ack timing must be tested
live.

### Tier 3: Minimal External Dispatcher

Use a tiny external component only to receive Telegram/Slack webhooks and call
GitHub `workflow_dispatch` or `repository_dispatch`. Examples: Cloudflare
Worker, Fly.io, or a GitHub App endpoint.

Tradeoffs:

- violates the pure "no server" ideal,
- much more reliable for Slack Events API and Telegram webhooks,
- lower latency,
- easier replay/ack control,
- smaller Actions spend.

Recommendation: do not build this first, but be honest that it is the
production-grade path if Slack/Telegram become primary channels.

### Channel Bridge Recommendation

Build in this order:

1. GitHub issue/comment core.
2. Telegram polling bridge as the first non-GitHub experiment.
3. Long-running Actions gateway for Slack Socket Mode as experimental.
4. External dispatcher only if users require reliable low-latency Slack/Telegram.

Do not let bridge channels bypass GitHub:

- every inbound channel message maps to an issue or comment,
- every outbound channel reply maps back to a GitHub assistant comment,
- channel-specific IDs live in provenance markers,
- dedupe is mandatory,
- channel bridge failures must not corrupt the issue conversation.

## State And Artifacts

State sources:

- GitHub issue body and comments: canonical conversation.
- Labels: current lightweight status.
- GitHub Actions logs: execution trace.
- Artifacts: optional prompt dump, model metadata, file list, proposed patch, test output.

Artifact names:

```text
gitclaw-issue-<number>-run-<run_id>-prompt.md
gitclaw-issue-<number>-run-<run_id>-summary.json
gitclaw-issue-<number>-run-<run_id>-patch.diff
```

Prompt artifacts are disabled by default. They can be enabled for a run by
setting `GITCLAW_PROMPT_ARTIFACT_PATH`; the GitHub workflow wires this for the
test-only `gitclaw:e2e-prompt-artifact` label and uploads:

```text
gitclaw-issue-<number>-run-<run_id>-prompt
```

Prompt artifacts must:

- redact known token/secret shapes before upload,
- mark issue text, comments, context files, and tool outputs as untrusted input,
- include basic run metadata and prompt byte count,
- never be printed into workflow logs.

## Git-Backed Backups

GitClaw should be able to export an issue conversation into a canonical JSON
backup file inside the repository:

```text
.gitclaw/backups/<owner>__<repo>/issues/<issue-number>.json
```

The backup includes:

- issue metadata,
- raw issue comments,
- reconstructed transcript with GitClaw assistant markers stripped,
- generation timestamp,
- schema version.

This is intentionally a normal file, not a hidden database. Repositories can
commit backups to the default branch, push them to a dedicated backup branch,
or mirror them privately. The MVP command writes the backup file but does not
auto-commit from the model-running job. Automatic backup commits run in a
separate workflow job after a successful assistant turn, with explicit
`contents: write` and `issues: read` permissions, and push only the canonical
backup file plus a repo-scoped backup index to a dedicated `gitclaw-backups`
branch.

Each backup branch update also refreshes:

```text
.gitclaw/backups/<owner>__<repo>/index.json
.gitclaw/backups/<owner>__<repo>/README.md
```

The index contains only navigational metadata: issue number, title, backup
path, backup timestamp, event name, labels, comment count, and transcript
message count. It intentionally avoids raw issue bodies and comments so humans
and E2E harnesses can verify backup coverage without opening every transcript.

The backup branch is intentionally separate from `main`:

- assistant conversation code keeps least-privilege `contents: read`;
- backup writes do not churn the product branch;
- raw issue transcript snapshots can have different retention/privacy rules;
- recovery remains a normal `git fetch origin gitclaw-backups` operation.

## Backup Inspection Command

GitClaw supports a deterministic backup audit command inspired by OpenClaw's
verified migration backups and Hermes' session export/backup flows:

```text
@gitclaw /backup
```

The command runs after normal preflight authorization and transcript
reconstruction, but before model inference. It posts a `gitclaw:assistant-turn`
comment with `model="gitclaw/backup"` and summarizes:

- dedicated backup branch name,
- expected issue backup JSON path,
- repo-scoped `index.json` and `README.md` paths,
- backup schema version,
- current raw comment, transcript, and assistant-turn counts,
- a short hash of the issue title for path/report correlation.

It never dumps issue/comment bodies. The report is navigational metadata; the
raw transcript copy remains the canonical backup JSON written by the post-turn
backup job.

## Backup Verification Command

GitClaw also supports a local verifier for the dedicated backup branch:

```bash
gitclaw backup verify --root .gitclaw/backups --repo <owner/repo>
```

The verifier is the git-native equivalent of `openclaw backup verify`. Instead
of checking a tarball manifest, it checks the repo-scoped backup tree:

- `index.json` exists, parses, has schema version `1`, matches the repository,
  has a valid timestamp, and has a count that matches its issue entries;
- `README.md` exists beside the index;
- every index entry points to canonical `issues/000000.json` form;
- index paths are relative, traversal-safe, and stay inside the repo backup
  directory;
- every issue payload exists, parses, has schema version `1`, matches the
  repository/issue/title/count metadata from the index, and has a valid backup
  timestamp;
- every `issues/*.json` file is listed in the index.

The command prints a deterministic `GitClaw Backup Verify Report` with status,
counts, paths, and failures. It exits non-zero when verification fails. It does
not print issue bodies, comments, or transcript text.

## Backup Manifest Command

GitClaw supports a local backup manifest command inspired by OpenClaw's
backup verification posture and Hermes' portable session export mindset:

```bash
gitclaw backup manifest --root .gitclaw/backups --repo <owner/repo> --issue 123
```

`--issue` is optional. Without it, the command summarizes every indexed issue
payload in the fetched backup tree. The command prints a deterministic
`GitClaw Backup Manifest` with:

- repository backup root, repo backup directory, index path, and README path,
- backup schema version and index generation time,
- control-file count and hashes for `index.json` and `README.md`,
- indexed issue payload file count, bytes, hashes, event names, timestamps,
  comment counts, and transcript message counts,
- aggregate payload bytes, comments, and transcript messages.

It never prints raw issue, comment, or transcript bodies. The manifest is a
compact provenance view for audits, mirrors, and restore reviews before anyone
uses the explicit raw `export-jsonl` path.

## Backup Stats Command

GitClaw supports a local backup stats command inspired by OpenClaw's
manifest/verify posture and Hermes' `sessions stats` view:

```bash
gitclaw backup stats --root .gitclaw/backups --repo <owner/repo>
```

The command reads a fetched `gitclaw-backups` tree, verifies it, and prints a
deterministic `GitClaw Backup Stats Report` with:

- backup stats and verify status,
- schema version and index generation time,
- issue, comment, transcript, user-message, assistant-message, assistant-turn,
  and error-comment counts,
- total payload bytes,
- event type counts,
- latest backup issue path, timestamp, event name, and title hash.

It never prints raw issue titles, issue bodies, comments, or transcript bodies.
The stats report is meant for routine backup-health monitoring where opening
every raw JSON file would be noisy.

## Backup Retention Plan Command

GitClaw supports a local non-mutating retention plan command inspired by
OpenClaw's verified backup/preview posture and Hermes' practical session
cleanup pressure:

```bash
gitclaw backup retention-plan --root .gitclaw/backups --repo <owner/repo> --keep-latest 50
```

The command reads a fetched `gitclaw-backups` tree, verifies it, sorts indexed
issue backups by backup timestamp descending, keeps the latest N, and lists the
older backups as prune candidates. It prints a deterministic
`GitClaw Backup Retention Plan` with:

- retention and verify status,
- backup root, repo backup directory, index path, README path, schema version,
  and index generation time,
- keep-latest value, total issue count, kept count, and prune-candidate count,
- newest and oldest kept backup boundaries,
- kept backup and prune-candidate paths, timestamps, event names, counts, and
  title hashes.

This is a dry-run report. It does not delete files, delete branches, edit
issues, post comments, or call GitHub APIs. It never prints raw issue titles,
issue bodies, comments, or transcript bodies. A future mutating prune command
must be separately approved and should compare its target set against this
plan before deleting anything from the backup branch.

## Backup JSONL Export Command

GitClaw supports a local JSONL export command inspired by Hermes'
`sessions export` workflow:

```bash
gitclaw backup export-jsonl --root .gitclaw/backups --repo <owner/repo> --issue 123
```

The command reads the repo-scoped backup index and canonical issue JSON files
from a fetched backup tree, then emits one JSON object per reconstructed
transcript message. Each record includes:

- `schema: "gitclaw.backup.transcript.v1"`,
- repository, issue number, issue title, event name, and backup timestamp,
- sequence number and source (`issue` or `comment:<id>`),
- transcript role, actor, author association, trust/edited flags,
- body hash and raw body text.

This is an explicit recovery/export command, so it may print raw issue and
comment bodies to stdout. It is intentionally local CLI output, not an issue
comment or Actions diagnostic. Use it only against a trusted checkout of the
backup branch.

## Backup Restore Plan Command

GitClaw supports a local non-mutating restore plan command inspired by
OpenClaw's backup/migration preview posture:

```bash
gitclaw backup restore-plan --root .gitclaw/backups --repo <owner/repo> --issue 123
```

The command reads the repo-scoped backup index and one canonical issue JSON
file from a fetched backup tree, then prints a deterministic
`GitClaw Backup Restore Plan`. The report includes:

- source repository and target repository,
- backup path, timestamp, event name, and schema version,
- issue title/body hashes,
- label, comment, transcript, assistant-turn, and error-comment counts,
- comment body hashes and transcript body hashes,
- planned restore actions for a future approved mutating restore command.

It does not call GitHub APIs, create issues, post comments, apply labels, or
print raw issue/comment/transcript bodies. A future mutating restore command
must be separately approved and should verify the restored issue against this
plan before considering recovery complete.

## Testing Strategy

End-to-end testing is a core product requirement. Unit tests and event fixtures
are necessary, but they are not sufficient because the most important behavior
lives in GitHub's event, permission, and workflow runtime semantics.

### Test Layers

1. **Fixture tests**

   Fast local Go tests using captured `issues.opened` and
   `issue_comment.created` payloads.

   Required coverage:

   - trigger gate: label, title prefix, disabled label,
   - PR comment exclusion,
   - trusted/untrusted `author_association`,
   - bot-loop filtering,
   - transcript ordering,
   - edited comment metadata,
   - hidden marker parsing,
   - idempotent comment posting,
   - prompt size limits,
   - redaction.

2. **Dry-run integration tests**

   Local tests that run `gitclaw handle --event <fixture>` with a fake GitHub
   client and fake LLM provider. These verify the whole command path without
   hitting GitHub or an LLM.

3. **Live GitHub E2E tests**

   Real tests using the GitHub CLI and GitHub Actions. These create real issues,
   add real comments, wait for real workflow runs, and verify real bot comments.
   Every meaningful implementation step should include at least one live E2E
   check before being considered complete.

### Live E2E Harness

The E2E harness should be a script, not a manual checklist:

```bash
./scripts/e2e/github-issue-chat.sh
```

Preflight:

```bash
command -v gh
gh auth status
gh repo view "$GITCLAW_E2E_REPO"
```

The harness should fail fast if `gh` is missing, not authenticated, or lacks
repo/workflow permissions.

Recommended default for this repository's own development: use the main
`AnandChowdhary/gitclaw` repository because the goal is to test real issue,
comment, workflow, permission, model, context, tool, backup, and heartbeat
behavior in the same repo users will install from. Downstream users can run the
same harness against a dedicated private sandbox repository:

```bash
export GITCLAW_E2E_REPO=AnandChowdhary/gitclaw
```

Why a main-repo E2E is acceptable during GitClaw development:

- issue events only use workflow files from the default branch,
- the real workflow, model permissions, labels, backup branch, and context files
  are the product surface,
- E2E issues are labeled `gitclaw:e2e` and closed automatically,
- failures leave auditable evidence in the same issue/action logs users will
  rely on.

The harness should verify that the target repository's default branch contains
the required workflow files. For this repo, the harness tests committed
workflows directly. For downstream sandbox repositories, a setup helper can
install or update generated workflows that run a pinned GitClaw binary or source
ref.

### Required Live Scenarios

Each scenario should create its own issue, wait for the matching Actions run,
assert the expected comments/labels, and close the issue in cleanup.

1. **New issue happy path**

   - create issue with `@gitclaw` title,
   - wait for workflow run,
   - assert one assistant comment exists,
   - assert comment has `gitclaw:assistant-turn` marker.

2. **Follow-up comment happy path**

   - comment on the same issue,
   - wait for second workflow run,
   - assert exactly one new assistant comment exists,
   - assert transcript includes prior assistant reply.

3. **Bot loop prevention**

   - verify the assistant's own comment does not trigger an additional run.

4. **PR comment ignored**

   - create a test PR or use a fixture PR,
   - comment with `@gitclaw`,
   - assert the issue-chat workflow does not respond.

5. **Untrusted actor gate**

   - simulate with a fixture when a second GitHub identity is unavailable,
   - run live when an external test account is configured,
   - assert no LLM-backed response is posted.

6. **Idempotency/retry**

   - re-run the same workflow for the same event when possible,
   - assert no duplicate assistant comment is created.

7. **Disabled issue**

   - add `gitclaw:disabled`,
   - comment on the issue,
   - assert no assistant response.

8. **Failure path**

   - intentionally remove `models: read`, disable GitHub Models access, or
     configure an invalid model in a sandbox branch/job,
   - assert a safe `gitclaw:error` comment and `gitclaw:error` label are
     produced,
   - assert no `gitclaw:assistant-turn` completion comment is produced,
   - assert the failure comment does not leak issue tokens, prompt content, or
     provider response bodies beyond a bounded diagnostic.

9. **Heartbeat conversation**

   - create an issue labeled `gitclaw:heartbeat` without the normal
     issue-chat trigger label or `@gitclaw` title prefix,
   - include an exact nonce token in the issue body,
   - dispatch `gitclaw-heartbeat.yml` with an explicit slot,
   - assert one heartbeat comment with the hidden `gitclaw:heartbeat` marker,
   - assert the comment includes the nonce token and
     `GITCLAW_HEARTBEAT_CONTEXT_V1` from `.gitclaw/HEARTBEAT.md`,
   - dispatch the same slot again,
   - assert no duplicate heartbeat comment is created.

10. **Workflow dispatch wakeup**

   - create an issue without the normal trigger label or `@gitclaw` title
     prefix,
   - add the `gitclaw` label after creation so `issues.opened` does not handle
     it,
   - dispatch the main `gitclaw.yml` workflow with `issue_number` and a stable
     `dispatch_id`,
   - assert one assistant comment with a `dispatch-...` event marker and exact
     nonce token,
   - dispatch the same `dispatch_id` again,
   - assert no duplicate assistant comment is created.

11. **Channel message reconstruction**

   - create an untriggered issue,
   - post a comment whose body starts with
     `<!-- gitclaw:channel-message ... -->`,
   - add the `gitclaw` label after the mirrored comment is written,
   - dispatch the main workflow with `dispatch_id` equal to the channel message
     ID,
   - assert the assistant sees the mirrored message body and returns its exact
     nonce token.

12. **Channel ingest workflow**

   - dispatch `gitclaw-channel-ingest.yml` with channel, thread, message id,
     author, and body,
   - assert it creates or reuses a canonical issue with
     `gitclaw:channel-thread`,
   - assert it posts a `gitclaw:channel-message` comment,
   - assert it dispatches the main workflow,
   - assert the assistant replies with the exact nonce from the ingested body.

13. **Proactive enqueue workflow**

   - create a generated proactive workflow fixture or dispatch a generic
     proactive enqueue command,
   - assert it creates or reuses a `gitclaw:proactive-run` issue for a stable
     slot,
   - assert it dispatches the main workflow with a proactive dispatch id,
   - assert rerunning the same slot does not create duplicate issues or
     duplicate assistant turns.

14. **Tool/context usage**

   - ask the assistant to read a concrete repository file such as `go.mod`,
   - assert the reply includes an exact expected token or module path,
   - ask the assistant to search for a fixture phrase and return the associated
     token from `gitclaw.search_files`,
   - ask for a selected local skill token,
   - assert the targeted skill is loaded and irrelevant skills stay unloaded.

15. **Context inspection**

   - create a real issue with `@gitclaw /context`,
   - assert the reply is marked `model="gitclaw/context"`,
   - assert the report lists repo context files, selected skills, and read-only
     tool output names,
   - assert the run succeeds without requiring a model provider response.

16. **Prompt inspection**

   - create a real issue with `@gitclaw /prompt`,
   - ask for a concrete file read, selected skill, and search fixture phrase,
   - assert the reply is marked `model="gitclaw/prompt"`,
   - assert the report lists prompt budget settings, final prompt size/hash,
     transcript inclusion/truncation counts, selected context files, selected
     skills, and active tool output metadata,
   - assert the report does not dump prompt text, issue body tokens, context
     bodies, skill bodies, or tool output bodies,
   - assert the run succeeds without requiring a model provider response.

17. **Memory inspection**

   - create a real issue with `@gitclaw /memory`,
   - assert the reply is marked `model="gitclaw/memory"`,
   - assert the report lists `.gitclaw/MEMORY.md`, dated memory note counts,
     loaded/omitted note counts, and memory file hashes,
   - assert the report does not dump memory file bodies or issue body tokens,
   - assert the run succeeds without requiring a model provider response.

18. **Memory search inspection**

   - create a real issue with `@gitclaw /memory search backup branch`,
   - assert the reply is marked `model="gitclaw/memory"`,
   - assert the report is marked `GitClaw Memory Search Report`,
   - assert it reports query hash/term count, scanned/matched counts, paths,
     line numbers, scores, loaded-for-turn state, and file/line hashes,
   - assert it does not dump the raw search query, issue body token, or memory
     file body tokens,
   - assert the run succeeds without requiring a model provider response.

19. **Skills inspection**

   - create a real issue with `@gitclaw /skills`,
   - assert the reply is marked `model="gitclaw/skills"`,
   - assert the report lists available skill metadata and selected skill paths,
   - assert hashes, frontmatter/description presence, and requirement counts
     are present,
   - assert skill validation status, duplicate-name count, invalid-name count,
     and folder/name mismatch count are present,
   - assert the report does not dump full skill bodies or verification tokens,
   - assert the run succeeds without requiring a model provider response.

20. **Skills search inspection**

   - create a real issue with `@gitclaw /skills search repository context`,
   - assert the reply is marked `model="gitclaw/skills"`,
   - assert the report is marked `GitClaw Skills Search Report`,
   - assert it reports query hash/term count, available skill count, matched
     skill count, match fields, selected-for-turn state, and skill hashes,
   - assert it does not dump the raw search query, issue body token, or full
     `SKILL.md` verification token,
   - assert the run succeeds without requiring a model provider response.

21. **Soul inspection**

   - create a real issue with `@gitclaw /soul`,
   - assert the reply is marked `model="gitclaw/soul"`,
   - assert the report lists loaded identity, policy, user, and memory paths
     with byte counts, line counts, and hashes,
   - assert soul validation status, required-file counts, memory-note counts,
     and noncanonical memory-note counts are present,
   - assert the report does not dump full soul or memory bodies,
   - assert the run succeeds without requiring a model provider response.

22. **Tools inspection**

   - create a real issue with `@gitclaw /tools`,
   - ask for a concrete file read and search fixture phrase,
   - assert the reply is marked `model="gitclaw/tools"`,
   - assert the report lists available tool contracts and active output
     metadata for list/search/read,
   - assert tool validation status, contract counts, active-output counts,
     unknown-output counts, unsafe-contract counts, and over-limit output
     counts, missing-guidance count, and duplicate-contract count are present,
   - assert the report does not dump full file or search output bodies,
   - assert the run succeeds without requiring a model provider response.

23. **Policy inspection**

   - create a real issue with `@gitclaw /policy` that also asks for write-mode
     work,
   - assert the reply is marked `model="gitclaw/policy"`,
   - assert the report shows trusted actor state, write-request detection,
     managed labels, expected workflow permissions, and `gitclaw.policy`
     metadata,
   - assert the report does not dump the issue body or policy output body,
   - assert `gitclaw:write-requested` and `gitclaw:done` are present without
     `gitclaw:running` or `gitclaw:error`.

24. **Session inspection**

   - create a real issue that gets one deterministic assistant turn,
   - post a follow-up comment with `@gitclaw /session`,
   - assert the reply is marked `model="gitclaw/session"`,
   - assert the report shows raw comment count, transcript message count,
     assistant-turn marker count, and per-message hashes,
   - assert the report does not dump issue or comment body tokens,
   - assert the run succeeds without requiring a model provider response.

25. **Backup index**

   - create a real deterministic GitClaw issue turn,
   - wait for the successful backup job,
   - assert the backup branch contains the issue JSON backup,
   - assert the repo-scoped `index.json` and `README.md` reference the issue
     number, title, and backup path,
   - assert the index contains metadata counts but not raw transcript bodies.

26. **Backup inspection**

   - create a real issue with `@gitclaw /backup`,
   - assert the reply is marked `model="gitclaw/backup"`,
   - assert the report lists the expected backup branch, issue backup path,
     index path, README path, and schema version,
   - wait for the successful backup job,
   - assert the backup branch contains the issue JSON backup and repo index
     entry for that same issue,
   - assert the report does not dump issue or comment body tokens.

27. **Backup verification**

   - create a real issue with `@gitclaw /backup`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup verify --root <fetched>/.gitclaw/backups --repo
     <owner/repo>`,
   - assert `backup_verify_status: ok`, zero verification failures, zero
     unindexed issue files, and an index entry for the just-created issue.

28. **Backup manifest**

   - create a real issue with `@gitclaw /backup`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup manifest --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --issue <issue-number>`,
   - assert the manifest lists index/README control file hashes plus the
     just-created issue payload path, bytes, hash, schema, event, comment
     count, and transcript count,
   - assert it does not dump the issue body token or raw transcript bodies.

29. **Backup JSONL export**

   - create a real issue with `@gitclaw /backup`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup export-jsonl --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --issue <issue-number>`,
   - assert the JSONL contains exactly the new issue transcript records,
   - assert the first record contains the issue body token and the second record
     contains the assistant backup report body, proving the command is an
     explicit raw recovery/export path rather than an issue-visible report.

30. **Backup restore plan**

   - create a real issue with `@gitclaw /backup`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup restore-plan --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --issue <issue-number>`,
   - assert the report is marked `restore_mode: dry-run`,
   - assert it lists backup path, schema version, label/comment/transcript
     counts, assistant-turn/error counts, and body hashes,
   - assert it does not dump the issue body token or raw transcript bodies.

31. **Backup retention plan**

   - create a real issue with `@gitclaw /backup`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup retention-plan --root <fetched>/.gitclaw/backups
     --repo <owner/repo> --keep-latest 1`,
   - assert the report is marked `retention_mode: dry-run`,
   - assert it lists verify status, total issue count, kept count,
     prune-candidate count, kept backups, prune candidates, paths, timestamps,
     and title hashes,
   - assert the just-created issue is included without dumping the issue body
     token or raw title.

32. **Proactive init generator**

   - run `gitclaw proactive init` against a temporary repo root,
   - assert it writes the expected prompt file and scheduled workflow,
   - assert the init report includes hashes and file status but not the prompt
     body token,
   - lint the generated workflow when `actionlint` is available,
   - dispatch the real generic proactive workflow with the generated job name
     and a `/proactive` prompt body,
   - assert it creates a real proactive issue and receives one deterministic
     proactive report without leaking the hidden prompt token.

### Example Live Commands

The script can use commands in this shape:

```bash
issue_url="$(gh issue create \
  --repo "$GITCLAW_E2E_REPO" \
  --title "@gitclaw e2e $(date -u +%Y%m%dT%H%M%SZ)" \
  --body "Explain this sandbox repo in one sentence." \
  --label gitclaw)"

issue_number="${issue_url##*/}"

gh run list --repo "$GITCLAW_E2E_REPO" --workflow GitClaw --limit 10
gh run watch --repo "$GITCLAW_E2E_REPO" "$(gh run list --repo "$GITCLAW_E2E_REPO" --workflow GitClaw --json databaseId --jq '.[0].databaseId')"

gh issue comment "$issue_number" \
  --repo "$GITCLAW_E2E_REPO" \
  --body "Follow up: include the workflow run id."

gh issue view "$issue_number" \
  --repo "$GITCLAW_E2E_REPO" \
  --comments
```

The real harness should avoid brittle sleeps. Poll GitHub's API with a deadline
and assert on markers, actor login, issue number, run URL, and idempotency key.

### E2E Acceptance Bar

MVP is not complete until:

- the fixture test suite passes,
- the dry-run command tests pass,
- the live GitHub E2E harness creates an issue and receives a GitClaw reply,
- the live harness comments again and receives exactly one additional reply,
- the live harness verifies actual conversation content, including exact
  nonce tokens across turns, repository file context from `go.mod`, and
  `gitclaw.search_files` context from the search fixture,
- the heartbeat harness dispatches a real workflow, receives one heartbeat
  comment, and proves same-slot idempotency,
- the workflow-dispatch harness dispatches the main handler against a real
  issue and proves same-dispatch-id idempotency,
- the channel-message harness verifies a hidden `gitclaw:channel-message`
  comment is reconstructed as user input during a dispatched run,
- the channel-ingest harness verifies the generic bridge workflow mirrors a
  message into an issue and dispatches the main handler,
- the proactive enqueue harness verifies manual/scheduled job primitives can
  create their own work issues idempotently,
- the proactive-init harness verifies the generator writes ordinary repo files
  without leaking prompt bodies and backs that up with a real deterministic
  proactive issue conversation,
- the backup-manifest harness verifies a real backed-up issue has a compact
  file-level manifest with hashes and counts but no raw body leakage,
- the backup JSONL export harness verifies a real backed-up issue can be
  exported as one JSONL transcript record per reconstructed message,
- the backup restore-plan harness verifies a real backed-up issue can produce
  a non-mutating restore plan with counts and hashes but no raw body leakage,
- the backup retention-plan harness verifies a fetched backup branch can
  produce a dry-run keep-latest plan with kept/prune-candidate paths and hashes
  but no raw title/body leakage,
- the live harness verifies status labels end at `gitclaw:done` without
  `gitclaw:running` or `gitclaw:error`,
- the failure harness forces a real invalid-model run and verifies a bounded
  `gitclaw:error` comment plus final `gitclaw:error` label without leaking an
  issue-secret token,
- the prompt-budget harness creates a large real issue and verifies the
  assistant still sees the preserved tail request under the bounded prompt,
- the prompt-report harness creates a large real issue and verifies
  `/prompt` reports prompt size/hash, truncation state, context contributors,
  and tool output metadata without dumping prompt or body contents,
- the prompt-artifact harness downloads a real Actions artifact and verifies
  redaction plus untrusted-input warnings,
- the write-request harness creates a real write-intent issue and verifies the
  `gitclaw:write-requested` label plus a read-only response,
- bot loop prevention is verified live,
- the issue is cleaned up or labeled with an E2E retention label.

## Security Model

Inspired by the OpenClaw/Hermes security lessons, but simplified:

- GitHub issue thread is the only chat channel.
- GitHub Actions job is the only runtime.
- Repository checkout is the workspace.
- GitHub token is scoped and expires with the job.
- The default LLM provider is GitHub Models using the Actions job token, not a
  long-lived third-party LLM secret.
- No hidden memory mutation.
- No self-authored skills.
- No host daemon.
- Read-only by default.

Hard rules:

- Issue text is untrusted input.
- Comments from external contributors are untrusted even if they trigger a workflow.
- Durable context writes require human review.
- Write mode requires explicit approval.
- Bot comments must contain machine-readable provenance.
- Use least-privilege `permissions` in every workflow.
- Grant `models: read` only to jobs that actually call GitHub Models.
- Set `timeout-minutes`.
- Use per-issue concurrency.

## Language Decision

Recommendation: **Go** for the main implementation.

Why Go wins for this product:

- GitHub Actions runtime cost will be dominated by checkout, network calls, and LLM latency, not CPU micro-optimizations.
- Go compiles quickly in Actions and can also ship as a single static binary.
- The standard library is strong for HTTP, JSON, CLI, templates, and process execution.
- GitHub API support is mature.
- Cross-compilation and release automation are simple.
- Contributor onboarding is easier than Rust or Zig for this kind of automation tool.
- Memory safety is good enough for a short-lived CLI handling untrusted text if we avoid shell execution and use well-scoped parsers.

Language comparison:

| Language | Fit | Pros | Cons | Verdict |
| --- | --- | --- | --- | --- |
| Go | Excellent | Fast compile, static binary, mature GitHub API, simple concurrency, easy ops | Larger binary than Zig/Rust, less type-rich than Rust | Best default |
| Rust | Good | Strong safety, excellent CLI crates, efficient runtime | Slower compile, more complexity, async/API friction | Good if we need stronger core correctness later |
| Zig | Poor for MVP | Tiny binary, fast startup, strong "nano" aesthetic | Immature GitHub/LLM ecosystem, more hand-rolled API/client code | Not worth MVP friction |
| TypeScript | Good wrapper | Native GitHub Actions ecosystem, Octokit, easy action publishing | Node dependency, less "mini binary", packaging/runtime churn | Good only for thin action wrapper |
| Python | Good prototype | Fastest to script, AI SDKs easy | Packaging and cold start can get messy, weaker single-binary story | Useful for experiments, not core |

Implementation split:

- Core: Go CLI.
- GitHub Action: composite action or workflow snippet that invokes the Go binary.
- Future optional wrapper: small TypeScript action only if Marketplace ergonomics require it.

## Suggested Package Layout

```text
cmd/gitclaw/main.go
internal/agent/
internal/comment/
internal/config/
internal/context/
internal/github/
internal/llm/
internal/policy/
internal/session/
internal/version/
docs/
examples/workflows/gitclaw.yml
```

## Acceptance Criteria For MVP

- A new issue can trigger GitClaw and receive one assistant comment.
- A new issue comment can trigger GitClaw and receive one assistant comment.
- The assistant sees prior GitClaw replies and user comments in order.
- Bot replies do not recursively trigger new agent runs when using `GITHUB_TOKEN`.
- PR comments are ignored by the issue-chat workflow.
- Workflow permissions are least-privilege: preflight uses `contents: read` and
  `issues: read` so dispatch events can resolve the target issue; handle uses
  `contents: read`, `issues: write`, and `models: read`.
- External/untrusted users do not invoke the LLM by default.
- The run has a timeout.
- Per-issue runs are serialized.
- The code is packaged as a Go CLI with a documented workflow.
- A `gh`-driven live E2E harness verifies the new-issue and follow-up-comment flows against a real GitHub repository.
- Bot-loop prevention, PR-comment ignore, disabled-label behavior, and idempotent retry behavior are covered by automated tests; bot-loop prevention is verified live.
- A `gh`-driven heartbeat E2E harness verifies a real scheduled-workflow path
  via `workflow_dispatch`, including issue transcript context,
  `.gitclaw/HEARTBEAT.md`, exact token content, and same-slot idempotency.
- A `gh`-driven workflow-dispatch E2E harness verifies the main handler can be
  woken for a specific issue and deduped by dispatch ID.
- A `gh`-driven channel-message E2E harness verifies a mirrored channel
  comment is included in the dispatched conversation transcript.
- A `gh`-driven channel-ingest E2E harness verifies the generic channel ingress
  workflow end to end.
- A `gh`-driven channels-report E2E harness verifies `@gitclaw /channels`
  reports workflow dispatch, channel labels, provider keys, and mirrored
  message marker counts without a model call.
- A `gh`-driven proactive E2E harness verifies the generic proactive enqueue
  workflow end to end.
- A `gh`-driven proactive-init E2E harness verifies
  `gitclaw proactive init` generates a scheduled workflow and prompt file
  without leaking prompt bodies, then dispatches a real proactive conversation.
- A `gh`-driven proactive-report E2E harness verifies `@gitclaw /proactive`
  reports workflow triggers and prompt metadata without a model call.
- A `gh`-driven model-report E2E harness verifies `@gitclaw /models` reports
  GitHub Models provider and retry settings without a model call.
- A `gh`-driven config-report E2E harness verifies `@gitclaw /config` reports
  effective labels, prompt budgets, commands, and workflow metadata without a
  model call.
- A `gh`-driven commands-report E2E harness verifies `@gitclaw /help` reports
  deterministic commands, aliases, and local CLI helpers without a model call
  or issue-body leakage.
- A `gh`-driven doctor-report E2E harness verifies `@gitclaw /doctor` reports
  config validation, workflow presence, context files, skills, memory notes,
  proactive prompts, and skill/soul/tool validation rollups without a model
  call.
- A `gh`-driven backup-index E2E harness verifies the dedicated backup branch
  contains issue JSON plus a repo-scoped `index.json` and `README.md`.
- A `gh`-driven backup-report E2E harness verifies `@gitclaw /backup`
  publishes deterministic backup paths without a model call and that the
  backup branch receives the corresponding issue JSON and index entry.
- A `gh`-driven backup-verify E2E harness verifies the fetched
  `gitclaw-backups` branch with `gitclaw backup verify` after a real issue
  backup job succeeds.
- A `gh`-driven backup-manifest E2E harness verifies the fetched
  `gitclaw-backups` branch can produce a file-level manifest with control-file
  and issue-payload hashes for one real issue, without dumping raw bodies.
- A `gh`-driven backup-stats E2E harness verifies the fetched
  `gitclaw-backups` branch can produce a repo-wide backup stats report with
  verification status, aggregate counts, latest backup metadata, and event
  counts, without dumping raw bodies or titles.
- A `gh`-driven backup-export-jsonl E2E harness verifies the fetched
  `gitclaw-backups` branch can be exported into raw JSONL transcript records
  for one real issue.
- A `gh`-driven backup-restore-plan E2E harness verifies the fetched
  `gitclaw-backups` branch can produce a dry-run restore plan for one real
  issue without dumping raw bodies.
- A `gh`-driven backup-retention-plan E2E harness verifies the fetched
  `gitclaw-backups` branch can produce a dry-run keep-latest retention plan
  with kept/prune-candidate metadata and hashes, without dumping raw titles or
  bodies.
- A `gh`-driven context-report E2E harness verifies `@gitclaw /context`
  produces a deterministic context summary without a model call.
- A `gh`-driven prompt-report E2E harness verifies `@gitclaw /prompt`
  produces a deterministic prompt budget, hash, truncation, context, and tool
  metadata report without a model call or prompt/body leakage.
- A `gh`-driven memory-report E2E harness verifies `@gitclaw /memory`
  produces a deterministic memory inventory without a model call or body
  leakage.
- A `gh`-driven memory-search E2E harness verifies
  `@gitclaw /memory search backup branch` searches git-backed memory files
  without a model call, raw query leakage, or memory-body leakage.
- A `gh`-driven memory-validate E2E harness verifies
  `@gitclaw /memory validate` reports memory hygiene without a model call or
  memory-body leakage.
- A `gh`-driven skills-report E2E harness verifies `@gitclaw /skills`
  produces a deterministic local skill inventory with provenance and
  requirement and validation metadata, without a model call.
- A `gh`-driven skills-info E2E harness verifies
  `@gitclaw /skills info repo-reader` produces focused skill metadata without
  a model call or full `SKILL.md` body leakage.
- A `gh`-driven skills-search E2E harness verifies
  `@gitclaw /skills search repository context` searches local skill metadata
  without a model call, raw query leakage, or full `SKILL.md` body leakage.
- A `gh`-driven soul-report E2E harness verifies `@gitclaw /soul` produces a
  deterministic high-authority context file audit with validation metadata,
  without a model call or body leakage.
- A `gh`-driven tools-report E2E harness verifies `@gitclaw /tools` produces a
  deterministic tool contract and active-output audit with validation metadata,
  without a model call or output-body leakage.
- A `gh`-driven policy-report E2E harness verifies `@gitclaw /policy` produces
  a deterministic preflight/label/write-policy audit without a model call or
  issue-body leakage.
- A `gh`-driven session-report E2E harness verifies `@gitclaw /session`
  reconstructs a real multi-turn GitHub issue session without a model call or
  transcript-body leakage.
- A `gh`-driven failure E2E harness verifies the safe failure path against a
  real Actions/model failure.
- A `gh`-driven prompt-budget E2E harness verifies a large real issue still
  produces a bounded, correct assistant reply.
- A `gh`-driven prompt-artifact E2E harness verifies opt-in redacted prompt
  artifacts against a real Actions artifact download.
- A `gh`-driven write-request E2E harness verifies deterministic write-intent
  labeling and the read-only policy response.

## Open Questions

1. Should the first user-facing default be all issues in a dedicated inbox repo, or label/prefix-triggered issues in any repo?
2. Should the default move from `openai/gpt-5-mini` to
   `openai/gpt-5.4-mini` once the GitHub Models catalog exposes that ID to
   Actions?
3. Should v0 include read-only repo file search, or should it be pure issue-thread chat first?
4. Do we want GitClaw to support GitHub App authentication in v1, or rely on `GITHUB_TOKEN` until PR automation exists?
5. Should write mode create draft PRs only, or also allow direct commits on non-protected branches?
6. Should downstream users default the live E2E harness to their main repo, or
   should the template recommend a disposable sandbox repo?
7. Which channel bridge should ship first: Telegram polling, Slack Socket Mode in Actions, or an external dispatcher?
8. Where should durable channel offsets and dedupe state live: bridge state issue, state branch, or repository variables?
9. What proactive jobs should ship as first-class templates: reminders, email
   triage, dependency health, CI failure follow-up, or repository hygiene?
10. Now that proactive job generation exists as a local CLI command, should a
   future write-approved mode propose a PR containing the generated workflow
   and prompt files?

## Sources

- Research brief: `docs/research-openclaw-hermes-landscape.md`
- GitHub Actions events: https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows
- `GITHUB_TOKEN` behavior: https://docs.github.com/en/actions/concepts/security/github_token
- `GITHUB_TOKEN` permissions: https://docs.github.com/en/actions/tutorials/authenticate-with-github_token
- GitHub Actions workflow syntax and concurrency: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
- GitHub Actions limits: https://docs.github.com/actions/reference/limits
- GitHub workflow dispatch REST API: https://docs.github.com/en/rest/actions/workflows#create-a-workflow-dispatch-event
- GitHub Models quickstart: https://docs.github.com/en/github-models/quickstart
- GitHub Models REST inference API: https://docs.github.com/en/rest/models/inference
- GitHub Models for Actions issue summaries: https://docs.github.com/en/github-models/github-models-at-scale/use-models-at-scale
- GitHub Models billing and rate-limit notes: https://docs.github.com/en/billing/concepts/product-billing/github-models
- GitHub Models `models:read` changelog: https://github.blog/changelog/2025-05-15-modelsread-now-required-for-github-models-access/
- OpenClaw heartbeat docs: https://openclawlab.com/en/docs/agent/heartbeat/
- OpenClaw automation docs: https://docs.openclaw.ai/automation/index
- OpenClaw scheduled tasks docs: https://docs.openclaw.ai/automation/cron-jobs
- OpenClaw memory docs: https://docs.openclaw.ai/concepts/memory
- OpenClaw creating skills docs: https://docs.openclaw.ai/tools/creating-skills
- OpenClaw skill format docs: https://docs.openclaw.ai/clawhub/skill-format
- OpenClaw models CLI docs: https://docs.openclaw.ai/cli/models
- OpenClaw config CLI docs: https://docs.openclaw.ai/cli/config
- OpenClaw configure docs: https://docs.openclaw.ai/cli/configure
- OpenClaw doctor docs: https://docs.openclaw.ai/doctor
- OpenClaw backup docs: https://docs.openclaw.ai/cli/backup
- Hermes memory docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/memory.md
- Hermes cron docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/cron.md
- Hermes profiles docs: https://hermes-agent.nousresearch.com/docs/user-guide/profiles
- Slack Socket Mode: https://api.slack.com/apis/connections/socket
- Slack Events API: https://docs.slack.dev/apis/events-api/
- Telegram Bot API `getUpdates`: https://core.telegram.org/bots/api#getupdates
