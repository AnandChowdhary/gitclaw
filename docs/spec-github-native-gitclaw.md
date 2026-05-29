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
      - uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - uses: actions/setup-go@v5
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
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
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
- Do not dump raw prompts into logs by default; if prompt artifacts are enabled,
  redact secrets and mark issue text as untrusted input.
- GitHub Models has free but rate-limited usage and optional paid usage, so
  the E2E harness should tolerate rate-limit failures as an explicit
  environment failure rather than treating them as product logic failures.

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
- Subcommands: `preflight`, `handle`, `backup`, `version`.

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

## Context Contract

Borrow the useful parts of OpenClaw and Hermes, but make them repo-native:

```text
AGENTS.md                    # existing coding-agent instructions, if present
.gitclaw/GITCLAW.md          # GitClaw-specific repo instructions
.gitclaw/POLICY.md           # repo-local permission and behavior policy
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
---
```

GitClaw inserts a compact `gitclaw.skill_index` tool output listing all
discovered local skills. Full skill instructions are loaded only when:

- the user mentions the skill name, folder, path, or relevant description terms;
- the skill declares `always: true` or `metadata.openclaw.always: true`.

Remote skill installation, skill execution scripts, dependency installation,
and agent-authored skill edits remain out of scope.

## Read-Only Tool Context

GitClaw v1 adds a small deterministic tool layer before the model call:

- `gitclaw.list_files`: lists a bounded set of repository files in the checkout.
- `gitclaw.search_files`: searches bounded text files for explicit quoted
  phrases or identifiers from the issue thread and returns matching lines.
- `gitclaw.read_file`: reads a bounded text file only when the issue thread
  explicitly mentions that repository-relative path.

Tool outputs are inserted into the prompt as auditable context blocks. They are
not autonomous shell execution, and they do not mutate the repository.

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
      - uses: actions/checkout@v4
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
backup file to a dedicated `gitclaw-backups` branch.

The backup branch is intentionally separate from `main`:

- assistant conversation code keeps least-privilege `contents: read`;
- backup writes do not churn the product branch;
- raw issue transcript snapshots can have different retention/privacy rules;
- recovery remains a normal `git fetch origin gitclaw-backups` operation.

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

12. **Tool/context usage**

   - ask the assistant to read a concrete repository file such as `go.mod`,
   - assert the reply includes an exact expected token or module path,
   - ask the assistant to search for a fixture phrase and return the associated
     token from `gitclaw.search_files`,
   - ask for a selected local skill token,
   - assert the targeted skill is loaded and irrelevant skills stay unloaded.

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
- the live harness verifies status labels end at `gitclaw:done` without
  `gitclaw:running` or `gitclaw:error`,
- the failure harness forces a real invalid-model run and verifies a bounded
  `gitclaw:error` comment plus final `gitclaw:error` label without leaking an
  issue-secret token,
- the prompt-budget harness creates a large real issue and verifies the
  assistant still sees the preserved tail request under the bounded prompt,
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
- Hermes cron docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/cron.md
- Slack Socket Mode: https://api.slack.com/apis/connections/socket
- Slack Events API: https://docs.slack.dev/apis/events-api/
- Telegram Bot API `getUpdates`: https://core.telegram.org/bots/api#getupdates
