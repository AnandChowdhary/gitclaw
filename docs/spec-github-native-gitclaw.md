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
command catalog without making a model call. `@gitclaw /heartbeat` exposes the
scheduled heartbeat surface, while the actual heartbeat runner remains a
separate workflow/CLI path that may call GitHub Models.

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
          GITCLAW_MODEL: openai/gpt-5-nano
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
          GITCLAW_MODEL: openai/gpt-5-nano
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

### Heartbeat Status Report

GitClaw also supports a deterministic heartbeat status command:

```text
@gitclaw /heartbeat
```

and the local equivalent:

```bash
gitclaw heartbeat status
```

This report is intentionally not the heartbeat runner. It runs after preflight
and before model inference, posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/heartbeat"`, and summarizes:

- the heartbeat label, disabled label, and trigger label,
- `.github/workflows/gitclaw-heartbeat.yml` presence, schedule trigger,
  workflow-dispatch trigger, inputs, and permissions,
- `.gitclaw/HEARTBEAT.md` presence and hash,
- heartbeat marker/idempotency contract,
- the quiet response contract, `HEARTBEAT_OK`,
- whether the current issue has the heartbeat label,
- existing heartbeat marker count for the current issue.

It never scans heartbeat issues, calls the model, mutates repository contents,
or prints issue/comment/workflow/heartbeat context bodies. The report carries
`model_call_required: false` and `runner_model_call_required: true` so E2E can
distinguish the operator report from the real scheduled model-backed runner.

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
  --prompt-file .gitclaw/proactive/email-triage.md \
  --not-before 2026-05-29T08:17:00Z
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
  --skill repo-reader \
  --prompt-file .gitclaw/proactive/email-triage.md \
  --prompt-body "Summarize inbox state and open an issue only when action is needed."
```

`--prompt` is accepted as a path alias for `--prompt-file`. If no prompt file
is supplied, the generator defaults to `.gitclaw/proactive/<name>.md`; if no
workflow file is supplied, it defaults to
`.github/workflows/gitclaw-proactive-<name>.yml`. The command refuses to
overwrite differing files unless `--force` is used, supports `--dry-run`, and
prints a body-free `GitClaw Proactive Init Report` with file paths, write
status, skill-hint counts, byte counts, and hashes. `--skill <name>` can be
passed more than once, and `--skills a,b` is accepted for comma-separated skill
hints. The generated prompt file records the hints in a
`gitclaw:proactive-skills` marker and a short "Suggested GitClaw skills"
section. When the proactive issue is later created, those skill names are part
of the canonical issue transcript, so GitClaw's normal progressive skill
loading can select the corresponding local `SKILL.md` files without a hidden
cron database or runtime-specific state. Generated files are:

```text
.github/workflows/gitclaw-proactive-email-triage.yml
.gitclaw/proactive/email-triage.md
```

`--not-before` is optional. When present, it accepts RFC3339 or `YYYY-MM-DD`
UTC dates and turns the enqueue primitive into a reminder due gate. If the
current Actions run is before the due gate, the command writes
`due=false`, `skipped=true`, and `issue_number=0` to `GITHUB_OUTPUT`, performs
no GitHub issue writes, and does not dispatch the main agent workflow. When
the same scheduled workflow runs at or after the due gate, it creates or
reuses the normal proactive issue and dispatches GitClaw with the usual
`proactive-<name>-<slot>` idempotency key. This keeps reminders serverless and
auditable while accepting GitHub Actions' best-effort schedule timing.

Reference proactive workflow shape:

```yaml
name: GitClaw Proactive Email Triage

on:
  workflow_dispatch:
    inputs:
      not_before:
        description: Optional RFC3339 or YYYY-MM-DD due gate
        required: false
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
        env:
          GITCLAW_PROACTIVE_NOT_BEFORE: ${{ github.event.inputs.not_before }}
      - if: ${{ steps.enqueue.outputs.issue_number != '' && steps.enqueue.outputs.issue_number != '0' }}
        run: >
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
@gitclaw /proactive list
@gitclaw /proactive risk
@gitclaw /proactive info repo-hygiene
@gitclaw /cron
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

The risk form:

```text
@gitclaw /proactive risk
@gitclaw /cron risk
```

posts a `GitClaw Proactive Risk Report` without model inference. It scans the
generic proactive workflow and `.gitclaw/proactive/*.md` prompt files for
prompt-boundary overrides, credential material, raw prompt logging, host
execution of prompt bodies, missing workflow-dispatch/permission boundaries,
and unbounded scheduling loops. It reports counts, paths, trigger/permission
metadata, risk codes, severities, and line hashes only; proactive prompt
bodies, workflow bodies, issue bodies, comments, credentials, and secret values
are not included. The report includes
`llm_e2e_required_after_proactive_risk_change=true`, so changes to the risk
surface must be paired with a live GitHub Models follow-up test, not only a
deterministic report assertion.

The focused info form:

```text
@gitclaw /proactive info <name>
@gitclaw /cron info <name>
```

posts a `GitClaw Proactive Info Report` for one job name. It reports the
matched prompt file path, size, line count, skill hints, and hash; the generic
proactive workflow trigger metadata; the generated workflow candidate path
`.github/workflows/gitclaw-proactive-<name>.yml`; whether that generated
workflow exists and has `workflow_dispatch`/`schedule`; and the exact enqueue
command shape. It includes `proactive_info_status` as `ok`, `not_found`, or
`ambiguous`, plus `raw_bodies_included=false`.

Local operators can inspect the same proactive surface without opening an
issue:

```bash
gitclaw proactive list
gitclaw proactive risk
gitclaw proactive info repo-hygiene
```

The local reports omit repository and issue metadata, report workflow and
prompt-file metadata with short hashes, and do not include proactive prompt
bodies.

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
- model: `openai/gpt-5-nano`
- default model policy: smallest OpenAI model currently visible in the
  authenticated GitHub Models catalog API
- auth token lookup: `GITHUB_TOKEN`, then `GH_TOKEN`, then optional
  `GITCLAW_LLM_API_KEY` for local/manual runs
- output token parameter: `max_completion_tokens` for GPT-5-family model IDs,
  `max_tokens` otherwise
- fallback models: `openai/gpt-4.1-nano` by default in the repository
  template config
- base URL override: `GITCLAW_LLM_BASE_URL`
- model override: `GITCLAW_MODEL`
- fallback override: `GITCLAW_MODEL_FALLBACKS`, comma/space/newline-separated;
  set to `none`, `false`, or `[]` to disable fallback for negative tests or
  provider migrations
- fallback retry policy: try the primary once by default on retryable provider
  statuses, then try configured fallbacks with the normal bounded retry budget;
  tune with `GITCLAW_LLM_PRIMARY_ATTEMPTS_BEFORE_FALLBACK`

2026-05-30 catalog check: GitHub's authenticated Models catalog API documents
`https://models.github.ai/catalog/models`, and the live catalog for this repo
currently shows OpenAI entries including `openai/gpt-4.1`,
`openai/gpt-4.1-mini`, `openai/gpt-4.1-nano`, `openai/gpt-4o-mini`,
`openai/gpt-5`, `openai/gpt-5-mini`, and `openai/gpt-5-nano`, but not
`openai/gpt-5.4-mini`. `openai/gpt-5-nano` is therefore the default because it
is the smallest OpenAI GPT-5-family model currently exposed through the GitHub
Models path usable with the Actions token. The first assistant version is
issue-thread chat plus repository context summarization, where latency and cost
matter more than maximum reasoning depth. Repositories can override to
`openai/gpt-5.4-mini`, a newer small model, or another GitHub Models catalog
model when that model is available to the repository.

2026-05-30 reliability check: real local GitHub Models probes with the same
GitHub identity returned `429` for `openai/gpt-5-nano` while
`openai/gpt-4.1-nano` and `openai/gpt-4o-mini` returned successful tiny
responses. The runtime should therefore record the actual selected model in
the assistant marker, keep `openai/gpt-5-nano` as the configured primary, and
allow a repo-reviewed fallback to preserve end-to-end conversation behavior
when the hosted preview service rate-limits one model.

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
- If the primary GitHub Models request receives a retryable provider response,
  the runtime can switch to configured fallback models. Non-retryable provider
  errors, including invalid model IDs, fail safely without fallback so negative
  E2E tests still prove the error path.

### Model Inspection Command

GitClaw supports a deterministic model/provider audit command:

```text
@gitclaw /models
@gitclaw /models list
```

The command runs after normal preflight and context loading, but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/models"` and summarizes:

- provider family,
- selected model,
- fallback models,
- default model policy and catalog endpoint host,
- endpoint host without URL credentials,
- token source name without token value,
- selected output-token parameter,
- request timeout,
- retry attempts,
- retry base and maximum delay,
- retryable status categories,
- fallback enablement and primary attempts before fallback,
- prompt-artifact enablement.

It never dumps issue/comment bodies, API keys, full prompts, or raw provider
error bodies. This gives operators a safe way to inspect GitHub Models and
OpenAI-compatible provider wiring from the issue thread before burning model
quota on a real assistant turn.

Local operators can inspect the same model wiring without opening an issue:

```bash
gitclaw models list
```

The local report omits repository, issue number, and issue-title hash while
retaining provider family, model ID, endpoint host, token-source name, timeout,
retry settings, prompt-artifact status, and environment knob names.

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
- Subcommands: `preflight`, `handle`, `backup`, `backup search`,
  `backup info`, `backup retention-plan`,
  `heartbeat`, `heartbeat status`,
  `channel-ingest`, `channel-state`, `channel-gateway`, `channel-delivery`,
  `channels list`, `channels verify`, `channels risk`, `channels info`,
  `checkpoints status`, `checkpoints list`, `checkpoints verify`,
  `rollback list`,
  `proactive enqueue`, `proactive init`, `proactive info`, `proactive risk`,
  `approvals list`, `approvals verify`,
  `artifacts list`, `artifacts verify`,
  `diffs summary`, `diffs verify`,
  `workspace summary`, `workspace verify`,
  `hooks list`, `hooks verify`,
  `plugins list`, `plugins verify`,
  `tasks list`, `tasks verify`,
  `migrate plan`,
  `orders list`, `orders verify`,
  `profile show`, `profile verify`,
  `runs current`, `runs verify`,
  `sandbox explain`, `sandbox verify`,
  `memory verify`, `memory risk`, `memory validate`, `memory list`,
  `memory promote-plan`, `memory info`, `memory search`,
  `skills validate`,
  `skills list`, `skills select-plan`, `skills info`, `skills search`,
  `bundles list`, `bundles info`,
  `soul verify`, `soul risk`, `soul validate`, `soul list`,
  `soul edit-plan`, `soul info`, `soul search`,
  `tools verify`, `tools risk`, `tools validate`, `tools list`, `tools run-plan`,
  `tools info`, `tools search`, `doctor`,
  `policy verify`,
  `secrets audit`, `secrets scan`, `secrets list`,
  `commands`, `version`.

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
- Expands repo-bounded `@file:<path>[:line-range]`, `@folder:<path>`,
  `@diff`, `@staged`, and `@git:N` context references from issue text into
  bounded read-only prompt context.
- Runs bounded read-only repository tools before the model turn.
- Supports deterministic `@gitclaw /context` reports so maintainers can inspect
  which context files, context references, skills, and tool outputs were
  assembled without making a model call.

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
<!-- gitclaw:assistant-turn run_id=123 event_id=456 model=openai/gpt-5-nano -->
```

For real model-backed assistant turns, the marker also includes body-free
prompt provenance:

```html
<!-- gitclaw:assistant-turn run_id="123" event_id="456" model="openai/gpt-5-nano" idempotency_key="..." prompt_context_sha256_12="..." context_documents="7" selected_skills="1" tool_outputs="3" skills="repo-reader" tools="gitclaw.list_files,gitclaw.search_files" -->
```

The provenance hash is computed from prompt-visible context paths, selected
skill paths, tool names, sizes, and body hashes. It never includes raw issue
text, comments, context bodies, skill bodies, tool inputs, or tool outputs. E2E
tests that claim tool usage should assert these marker fields in addition to
the assistant's answer text.

Hidden status marker:

```html
<!-- gitclaw:status run_id=123 state=running -->
```

## Session Inspection Command

GitClaw supports a deterministic session audit command inspired by OpenClaw's
transcript/session CLIs and Hermes' saved/searchable sessions:

```text
@gitclaw /session
@gitclaw /session list
@gitclaw /session search deployment window
```

The command runs after normal preflight authorization and transcript
reconstruction, but before model inference. It posts a `gitclaw:assistant-turn`
comment with `model="gitclaw/session"` and summarizes:

- raw comment count and reconstructed transcript message count,
- user/assistant and trusted/untrusted message counts,
- GitClaw assistant, heartbeat, error, and channel-message marker counts,
- assistant-turn prompt provenance counts, unique prompt-context hashes,
  prompt-visible skill names, and prompt-visible tool names,
- whether the issue is a channel-thread or proactive-run issue,
- per-transcript-message source, actor, trust state, size, line count, and
  short hash,
- per-assistant-turn prompt provenance cards without raw prompt, skill, or tool
  bodies.

It never dumps issue/comment bodies. The hashes make session reconstruction
debuggable without turning the issue-visible report into another raw transcript
copy.

Local operators can inspect a backed-up issue session without calling GitHub:

```bash
gitclaw session list --backup .gitclaw/backups/owner/repo/issues/000123.json
```

The local report reads the canonical backup JSON, uses `scope: local-backup`,
and emits repo/issue backup metadata, marker counts, transcript counts, trust
states, sources, sizes, and hashes without dumping issue bodies, comment bodies,
or assistant replies.

Backed-up sessions can also be searched locally without a GitHub API call:

```bash
gitclaw session search deployment window --backup .gitclaw/backups/owner/repo/issues/000123.json
```

The local search report uses the same body-free matcher and returns
`scope: local-backup`, backup metadata, query hash/term count, transcript and
match counts, result limits, sources, trust metadata, scores, and hashes.

When called as `@gitclaw /session search <query>`, the command searches the
current reconstructed GitHub issue transcript with a local lexical matcher. It
reports the query hash and term count, transcript and match counts, result
limits, message indexes, roles, sources, actor/trust metadata, line numbers,
scores, message hashes, and line hashes. It never emits raw issue bodies,
comment bodies, assistant replies, prompts, or raw search queries. This is the
GitHub-native version of OpenClaw/Hermes session search: enough recall and
debugging signal to find the relevant turn while preserving the issue thread as
the canonical session store.

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
.gitclaw/STANDING_ORDERS.md  # durable operating authority boundaries
.gitclaw/HOOKS.md            # declarative hook safety policy
.gitclaw/hooks/*.md          # declarative hook specs, metadata-only in v1
.gitclaw/PLUGINS.md          # declarative plugin safety policy
.gitclaw/plugins/*.md        # declarative plugin specs, metadata-only in v1
.gitclaw/TASKS.md            # declarative task/flow safety policy
.gitclaw/tasks/*.md          # declarative task/flow specs, issue-native in v1
.gitclaw/AGENTS.md           # declarative agent/routing safety policy
.gitclaw/agents/*.md         # declarative agent specs, metadata-only in v1
.gitclaw/NODES.md            # declarative runtime/node safety policy
.gitclaw/nodes/*.md          # declarative node specs, metadata-only in v1
.gitclaw/ARTIFACTS.md        # declarative artifact safety policy
.gitclaw/artifacts/*.md      # declarative artifact specs, metadata-only in v1
.gitclaw/DIFFS.md            # declarative diff/reporting safety policy
.gitclaw/diffs/*.md          # declarative diff specs, metadata-only in v1
.gitclaw/WORKSPACE.md        # declarative workspace/checkout safety policy
.gitclaw/workspaces/*.md     # declarative workspace specs, metadata-only in v1
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
- `.gitclaw/STANDING_ORDERS.md`, if present, as persistent repo-reviewed
  authority boundaries for normal and proactive turns
- `.gitclaw/HOOKS.md`, if present, as hook safety policy for event-driven
  automation; individual `.gitclaw/hooks/*.md` specs are audited by metadata
  reports and are not executed by the model runtime
- `.gitclaw/PLUGINS.md`, if present, as plugin safety policy for runtime
  extension boundaries; individual `.gitclaw/plugins/*.md` specs are audited
  by metadata reports and are not installed or executed by the model runtime
- `.gitclaw/TASKS.md`, if present, as task/flow safety policy for
  issue-native work tracking; individual `.gitclaw/tasks/*.md` specs are
  audited by metadata reports and do not spawn workers or open a task DB
- `.gitclaw/AGENTS.md`, if present, as the repo-reviewed agent/routing safety
  policy; individual `.gitclaw/agents/*.md` specs are audited by metadata
  reports and do not spawn child agents, gateways, nodes, or remote workers
- `.gitclaw/NODES.md`, if present, as the repo-reviewed runtime/node safety
  policy; individual `.gitclaw/nodes/*.md` specs are audited by metadata
  reports and do not pair devices, open WebSockets, start services, or expose
  remote host capabilities
- `.gitclaw/ARTIFACTS.md`, if present, as the repo-reviewed artifact safety
  policy; individual `.gitclaw/artifacts/*.md` specs are audited by metadata
  reports and do not upload files, read artifacts, or turn artifact bodies into
  prompt or issue-comment content by themselves
- `.gitclaw/DIFFS.md`, if present, as the repo-reviewed diff/reporting safety
  policy; individual `.gitclaw/diffs/*.md` specs are audited by metadata
  reports and do not render raw patches, stage files, reset files, or expose
  untracked file contents
- `.gitclaw/WORKSPACE.md`, if present, as the repo-reviewed workspace safety
  policy; individual `.gitclaw/workspaces/*.md` specs are audited by metadata
  reports and do not create private memory, mount external paths, write
  workspace state, or change the Actions checkout
- `.gitclaw/SKILLS/*/SKILL.md`, if selected by the issue thread or marked
  always-on
- bounded `@file:<repo-path>[:start-end]` context references explicitly named
  in issue text
- bounded `@folder:<repo-path>` references rendered as file metadata, not file
  bodies
- bounded Git references: `@diff`, `@staged`, and `@git:N`, clamped to the
  latest 10 commits
- issue thread transcript
- small repository summary from a read-only file listing
- bounded `gitclaw.read_file` output for files explicitly mentioned in the
  issue thread

GitClaw's repo-bounded context references are inspired by Hermes' context
reference UX, but intentionally narrower:

```text
@file:docs/search-fixture.md
@file:docs/search-fixture.md:1-20
@folder:.gitclaw
@diff
@staged
@git:1
```

`@file` loads the referenced text file, optionally reduced to a 1-indexed
inclusive line range. `@folder` loads only a bounded metadata listing with file
paths, byte counts, line counts, and hashes. References that escape the repo,
point at symlinks/binary files, or target common credential locations such as
`.env`, `.git/`, `.ssh/`, `.aws/`, `.gnupg/`, `.kube/`, `.npmrc`, `.netrc`,
or private-key filenames are skipped and reported as body-free metadata. The
`@gitclaw /context` report shows reference kind, normalized path, range, status,
counts, and hashes without dumping referenced content, issue text, comments, or
tool outputs.

`@diff` and `@staged` run read-only `git diff` commands against the checked-out
workspace and include bounded patch text only in model prompt context. Empty
working-tree or staged diffs are reported as `empty` context references and are
not loaded. `@git:N` runs a bounded read-only log/patch view for the latest N
commits, clamps N to 10, and includes commit hashes, subjects, stats, and
patches in prompt context. The `/context` report remains body-free for all Git
references: it shows kind, path/ref, requested count, status, byte/line counts,
and hashes, but never dumps commit patches or diff hunks.

Do not let the agent write these files in MVP. Skills, soul, tools notes, and
memory are git-backed source files: edits happen through normal human-reviewed
commits, not hidden agent mutation.

## Memory Inspection Command

GitClaw supports a deterministic memory audit command inspired by OpenClaw's
Markdown memory files and Hermes' split between compact prompt memory and
larger session recall:

```text
@gitclaw /memory
@gitclaw /memory list
@gitclaw /memory verify
@gitclaw /memory risk
@gitclaw /memory validate
@gitclaw /memory promote-plan long-term
@gitclaw /memory info .gitclaw/memory/2026-05-29.md
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
`@gitclaw /memory list` is an explicit inventory alias for the same report,
matching the local `gitclaw memory list` helper.

When called as `@gitclaw /memory info <path>`, the command posts a focused
body-free card for one memory file. It accepts `.gitclaw/MEMORY.md`,
`.gitclaw/memory/YYYY-MM-DD.md`, a bare date, or `latest`, and reports the
normalized path, match status, kind, repo-local source, canonicality,
latest-note state, loaded-for-this-turn state, byte/line counts, short hash,
context-limit state, validation rollup, and read-only write status.

When called as `@gitclaw /memory search <query>`, the command searches
git-backed memory files with a local lexical matcher. It reports query hash,
term count, scanned file count, matched file/line counts, paths, line numbers,
scores, loaded-for-this-turn state, and file/line hashes. It does not echo the
raw query because query text comes from issues and may contain secrets.

When called as `@gitclaw /memory verify`, the command posts a body-free trust
envelope for repo-local memory provenance. It reports memory-file counts,
repo-local source counts, long-term memory presence/loading, dated-note
canonicality, loaded/omitted note counts, latest note path, memory hashes,
hygiene rollups, read-only write status, and explicit external-memory-provider,
session-index, and background-promotion verification non-goals.

When called as `@gitclaw /memory risk`, the command scans repo-local memory
files for durable-state risk categories without dumping memory bodies. It
reports long-term/dated-note counts, loaded state, memory write boundaries,
external-provider non-goals, risk counts, risk codes, categories, paths, and
line hashes only. The initial rules cover prompt-boundary overrides,
credential-looking material in memory, hidden persistence instructions,
unbounded automation, unreviewed host execution, and credential-handling notes.
Local operators can run the same audit with `gitclaw memory risk`.

When called as `@gitclaw /memory promote-plan <target>`, the command posts a
body-free dry-run plan for turning the current issue thread into reviewed
durable memory. Supported targets are `long-term` for `.gitclaw/MEMORY.md` and
`daily-note` for `.gitclaw/memory/YYYY-MM-DD.md`. The report includes request
hashes, transcript-message count, target kind/path, current target metadata,
memory budget, validation rollup, promotion boundaries, review steps, and an
explicit live-LLM E2E requirement. It never generates the candidate memory,
calls a model, writes files, mutates the repository, or dumps issue bodies,
comments, transcript bodies, current memory bodies, or candidate memory text.
User-profile promotions route to `/soul edit-plan user`.

When called as `@gitclaw /memory validate`, the command renders only the
memory-hygiene report. Local operators can run the same validation with:

```bash
gitclaw memory verify
gitclaw memory risk
gitclaw memory validate
gitclaw memory list
gitclaw memory promote-plan [long-term|daily-note]
gitclaw memory info <path>
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
and line counts, short hash, frontmatter/description presence, configured gate
state, `always`, and counts of declared/missing runtime requirements. Full skill
instructions are
loaded only when:

- the skill is enabled by repo config,
- the user mentions the skill name, folder, path, or relevant description terms;
- a selected skill bundle references the skill;
- the skill declares `always: true` or `metadata.openclaw.always: true`.

Repo owners can gate skill loading without deleting reviewed skill files:

```yaml
skills:
  allowed:
    - repo-reader
  disabled:
    - deploy-helper
```

`skills.allowed` is an optional allowlist; if present, only those skill names or
folder names can load into prompt context. `skills.disabled` is an optional
denylist and wins over `skills.allowed`. Both lists accept lower hyphen-case
skill names only. Disabled or allowlist-blocked skills remain visible in
metadata reports and `gitclaw.skill_index`, but their full `SKILL.md` bodies are
not selected even when `always: true` or explicitly mentioned.

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
- missing declared env/bin requirements are warned about for enabled skills.

Validation is visible in the `/skills` report and locally through:

```bash
gitclaw skills verify
gitclaw skills risk
gitclaw skills validate
gitclaw skills check
gitclaw skills list
gitclaw skills select-plan <name>
gitclaw skills install-plan <target>
gitclaw skills upgrade-plan <target>
gitclaw bundles list
gitclaw bundles info <name>
gitclaw skills info <name>
gitclaw skills search <query>
```

The validation output includes only paths, counts, hashes, and short finding
details. It never dumps full `SKILL.md` bodies.

`gitclaw skills verify` is a stronger body-free trust envelope for the same
local skills. It does not contact an external registry or execute installers.
It reports repo-local source roots, content hashes, requirement status, and the
validation rollup so reviewers can audit skill influence as code.

## Skills Inspection Command

GitClaw supports a deterministic skill inventory command inspired by
OpenClaw's `openclaw skills` commands and Hermes' `skills_list` /
`skill_view` split:

```text
@gitclaw /skills
@gitclaw /skills list
@gitclaw /skills verify
@gitclaw /skills risk
@gitclaw /skills validate
@gitclaw /skills check
@gitclaw /skills select-plan repo-reader
@gitclaw /skills install-plan repo-reader
@gitclaw /skills upgrade-plan repo-reader
@gitclaw /bundles
@gitclaw /bundles info repo-context
@gitclaw /skills info repo-reader
@gitclaw /skills search repository context
```

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/skills"` and summarizes:

- available local skills from git-tracked `SKILL.md` files,
- enabled, config-disabled, and allowlist-blocked skill counts,
- selected skills for the current issue/comment,
- configured gate state, `always` activation state, and frontmatter
  descriptions,
- short hashes and size metadata for review,
- declared env/bin requirement counts and whether any are missing.
- validation status, error/warning counts, duplicate-name count, invalid-name
  count, folder/name mismatch count, and body-free findings.
- risk-audit status, risky-instruction category counts, finding codes, and
  line hashes without raw `SKILL.md` text.
- dry-run selection planning metadata when explicitly requested.
- dry-run install/upgrade planning metadata when explicitly requested.

It does not dump full skill bodies. Full `SKILL.md` content remains a prompt
input only when selected by the normal progressive-disclosure rules.
`@gitclaw /skills list` is an explicit inventory alias for the same report,
matching the local `gitclaw skills list` helper.

Skill bundles are repo-local YAML task profiles inspired by Hermes' skill
bundle feature. GitClaw reads `.gitclaw/skill-bundles/*.yaml` and
`.gitclaw/skill-bundles/*.yml` files with this schema:

```yaml
name: repo-context
description: Repository context questions using repo-reader.
skills:
  - repo-reader
instruction: |
  Prefer repository context and deterministic tool outputs before answering.
```

`@gitclaw /bundles` and `gitclaw bundles list` produce a body-free inventory:
bundle path, normalized bundle name, referenced skills, resolved and missing
skill refs, selected-for-this-turn status, instruction presence, byte/line
counts, and short hash. `@gitclaw /bundles info <name>` and
`gitclaw bundles info <name>` show one focused bundle card. Bundle YAML bodies,
bundle instructions, skill bodies, issue bodies, comments, prompts, and secret
values are never printed in reports.

When a user invokes a repo-local bundle slash command such as
`@gitclaw /repo-context explain go.mod`, GitClaw selects every enabled,
resolved skill referenced by that bundle for the model turn and includes the
bundle instruction as bounded prompt context. Missing skills are reported as
metadata and skipped, not treated as fatal. Bundles do not install skills,
execute scripts, or change the system prompt.

When called as `@gitclaw /skills validate`, the command posts only the
validation report: status, error/warning totals, duplicate-name count,
invalid-name count, folder/name mismatch count, and body-free findings. This
mirrors `gitclaw skills validate` for issue-side audits without the full skill
inventory. `@gitclaw /skills check` and `gitclaw skills check` are OpenClaw
compatibility aliases for the same validation-only report.

When called as `@gitclaw /skills verify`, the command posts the repo-local
skill trust envelope. It includes `verification_scope=repo-local-metadata`,
enabled/disabled/allowlist-blocked counts, source-root counts, per-skill trust
cards with short body hashes, declared and missing requirement counts, and an
explicit
`registry_verification=not_configured` field. This mirrors OpenClaw's
verification posture while preserving GitClaw's no-registry, no-installer MVP
boundary.

When called as `@gitclaw /skills risk`, the command posts a body-free skill
risk audit inspired by OpenClaw skill/plugin-hook safety and Hermes toolset
filtering. GitClaw scans repo-local `SKILL.md` bodies internally for risky
instruction categories such as prompt-boundary override, secret exfiltration,
unbounded tool loops, hidden persistence, and unreviewed shell execution. The
report publishes only status, category counts, finding codes, skill paths,
content hashes, and line hashes. It never dumps skill bodies, issue/comment
bodies, prompts, secrets, registry metadata, installer output, or raw matched
lines, and it never executes skills or contacts a registry.

When called as `@gitclaw /skills select-plan <name>`, the command posts a
body-free dry-run explanation for one repo-local skill's influence on the
current turn. It reports the requested-skill hash, request-text hash, term
count, available/enabled/matched/selected skill counts, selected bundle count,
enabled/disabled/allowlist gate state, always-on state, validation summary,
metadata-only skill match, and selection reason codes such as
`request_metadata_match`, `always`, or `selected_bundle`.

The selection planner never calls a model, mutates the repository, prints the
raw requested skill string, prints raw request text, or dumps full `SKILL.md`
bodies. It includes `llm_e2e_required_after_change=true` because skill
selection changes must be proven with a live GitHub Models conversation E2E,
not only deterministic report tests.

When called as `@gitclaw /skills install-plan <target>` or
`@gitclaw /skills upgrade-plan <target>`, the command switches to a
non-mutating install planner inspired by OpenClaw's ClawHub/AgentSkills
install UX and Hermes' skill trust posture. The planner classifies the target
as a registry name, local skill path, GitHub shorthand, GitHub URL, generic
HTTPS URL, insecure HTTP URL, unsupported URL, unsafe path, or empty target. It
reports only safe metadata: target hash, target type, derived safe
repo-local name, destination path candidate, existing repo-local matches,
validation rollup, and findings.

The install planner never fetches remote targets, never contacts a registry,
never runs installer scripts, never installs dependencies, never writes
`.gitclaw/SKILLS`, and never commits or pushes. Remote URLs are classified only
and require manual review. Existing skill matches are reported as upgrade or
overwrite risk. The report includes `llm_e2e_required_after_change=true` to
make the release rule explicit: after a skill is actually changed, maintainers
must run a live GitHub Models conversation E2E in addition to deterministic
skill-report tests.

When called as `@gitclaw /skills info <name>`, the same deterministic command
switches from inventory mode to focused skill-info mode. The info report shows
one skill's safe metadata:

- requested name and match count,
- path, folder, byte/line counts, and content hash,
- enabled/disabled/allowlist-blocked state and whether the skill was selected
  for this turn,
- `always`, frontmatter, and description presence,
- declared and missing env/bin requirement names/counts,
- validation findings for matching skill files only.

This mirrors OpenClaw's `skills info <name>` and Hermes' progressive
`skills_list()` / `skill_view(name)` split while preserving GitClaw's rule that
issue-visible diagnostics never dump full skill bodies or secret values.

When called as `@gitclaw /skills search <query>`, the command switches to
body-safe metadata search. It searches skill names, leaf folders, paths, and
frontmatter descriptions, then reports match counts, match fields, selection
state, configured gate state, hashes, sizes, and requirement counts. The raw
search query is
represented only by a short hash and term count because the query itself comes
from issue text and may contain secrets.

## Profile Inspection Command

GitClaw supports a deterministic repo-local profile envelope inspired by
Hermes profiles and OpenClaw workspace files:

```text
@gitclaw /profile
@gitclaw /profiles
```

Hermes profiles isolate config, memory, sessions, skills, cron jobs, and other
agent state per named agent. OpenClaw's equivalent durable shape is its
workspace folder: `SOUL.md`, `IDENTITY.md`, `USER.md`, `TOOLS.md`,
`MEMORY.md`, memory notes, and skills. GitClaw's GitHub-native version is one
profile per repository, stored under `.gitclaw/` and reviewed like code.

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/profile"` and summarizes:

- repo-local profile strategy and store,
- model provider, model, run mode, trigger label, and trigger prefix,
- loaded profile document counts, identity/policy file counts, and memory-note
  counts,
- available and selected skills plus skill bundle count,
- deterministic tool contract and active-output counts,
- soul, skill, and tool validation rollups.

It never dumps profile file bodies, skill bodies, tool outputs, issue/comment
bodies, prompts, or secrets. It is an operator confidence surface, not a
profile export or profile mutation command.

Local operators can inspect the same profile envelope without opening an issue:

```bash
gitclaw profile show
gitclaw profile verify
```

The aliases intentionally return the same body-free report in v1.

## Migration Plan Command

GitClaw supports a deterministic migration planner inspired by OpenClaw's
preview-first migration model and Hermes' isolated profile directories:

```text
@gitclaw /migrate plan hermes
@gitclaw /migration openclaw
```

```bash
gitclaw migrate plan hermes
gitclaw migrate plan openclaw
```

OpenClaw's migration CLI previews a plan before apply, redacts secrets, and
backs apply with a verified backup. Hermes profiles keep config, `.env`,
`SOUL.md`, memories, sessions, skills, cron jobs, and gateway state in a
profile-specific home. GitClaw's serverless version keeps the same safety
shape but narrows v1 to a body-free plan for importing declarative state into
the repository.

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/migration"` and summarizes:

- requested source hash and normalized source (`openclaw`, `hermes`, `codex`,
  or `claude`),
- the fixed repo-local migration scope,
- disabled source scanning, apply, model-call, repository mutation, credential
  import, and executable-state import flags,
- current GitClaw target inventory: loaded context documents, required context
  files, skills, bundles, memory notes, tool contracts, active tool outputs,
  backup branch, and backup schema version,
- soul, skill, and tool validation rollups,
- source-specific import-map rows for reviewed manual copy, reviewed merge,
  archive-only, manual review, or skipped-secret state.

It never scans `~/.hermes`, `~/.openclaw`, `~/.codex`, or `~/.claude` from an
issue command; never imports secrets; never executes hooks, installers, MCP
servers, or plugins; never writes files; and never dumps source bodies,
credentials, issue/comment bodies, prompts, or raw source payloads. Any actual
migration change must be made through reviewed repository edits, then followed
by `/soul verify`, `/skills verify`, `/tools verify`, backup verification, and
a live GitHub Models conversation E2E that performs an actual model call.

## Run Ledger Command

GitClaw supports a deterministic current-turn provenance report inspired by
OpenClaw's visible runtime/audit surfaces and Hermes' session/checkpoint
metadata:

```text
@gitclaw /runs
@gitclaw /run
@gitclaw /ledger
```

The command runs after normal preflight authorization, transcript
reconstruction, and repo-context assembly, but before model inference. It posts
a `gitclaw:assistant-turn` comment with `model="gitclaw/runs"` and summarizes:

- repository, issue, event kind/name/action, event ID, active command, and
  idempotency key,
- Actions run ID, attempt, run URL presence/hash, event SHA hash, and a compact
  run-environment hash,
- preflight decision, trusted actor state, trigger state, disabled-label state,
  and write-intent detection,
- raw comment count, reconstructed transcript counts, and GitClaw marker
  counts before the turn,
- context document, selected skill, available skill, skill bundle, and active
  tool-output counts,
- label presence across the managed run/status/channel/proactive labels,
- prompt-visible context/skill document paths with byte, line, and hash
  metadata,
- active tool-output names with input/output hashes and output sizes.

It never dumps issue bodies, comments, prompt text, context bodies, skill
bodies, tool output bodies, workflow payloads, or secrets. The report is a
read-only ledger view: the canonical conversation log remains GitHub issue
comments, the canonical execution trace remains GitHub Actions, and the
canonical post-turn archive remains the `gitclaw-backups` branch when enabled.

Local operators can inspect the same body-free local run envelope without
opening an issue:

```bash
gitclaw runs current
gitclaw runs verify
```

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
gitclaw soul verify
gitclaw soul risk
gitclaw soul validate
gitclaw soul list
gitclaw soul edit-plan <path>
gitclaw soul info <path>
gitclaw soul search <query> --max-results 10
```

The validation output includes only paths, counts, and short finding details.
It never dumps full soul, user, memory, tool, or heartbeat file bodies.

## Soul Inspection Command

GitClaw supports a deterministic high-authority context audit command inspired
by OpenClaw and Hermes' portable workspace files:

```text
@gitclaw /soul
@gitclaw /soul list
@gitclaw /soul verify
@gitclaw /soul risk
@gitclaw /soul validate
@gitclaw /soul edit-plan soul
@gitclaw /soul info soul
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
- soul risk status, risk finding counts, risk codes, and line hashes for
  prompt-boundary, secret exfiltration, persistent-state, channel-control,
  automation-amplification, host-execution, and credential persistence risks.
- high-authority edit planning metadata when explicitly requested.

It never dumps full file bodies. The hashes make the issue-visible report
verifiable without exposing private user, memory, or policy text.
`@gitclaw /soul list` is an explicit inventory alias for the same report,
matching the local `gitclaw soul list` helper.

When called as `@gitclaw /soul info <path>`, the command posts one focused
high-authority context metadata card. Supported targets include `soul`,
`identity`, `user`, `tools`, `memory`, `heartbeat`, `.gitclaw/SOUL.md`,
`.gitclaw/IDENTITY.md`, `.gitclaw/USER.md`, `.gitclaw/TOOLS.md`,
`.gitclaw/MEMORY.md`, `.gitclaw/HEARTBEAT.md`, `latest`, and
`.gitclaw/memory/YYYY-MM-DD.md`. The report includes normalized path,
category, repo-local source, present/required/canonical/latest flags,
selected-for-this-turn state, byte count, line count, short hash, and
at-context-limit status. It never emits raw file, issue, comment, prompt, or
secret bodies. This mirrors `gitclaw soul info <path>` for local review.

When called as `@gitclaw /soul edit-plan <path>` or
`@gitclaw /soul plan <path>`, the command switches to a dry-run edit planner
for high-authority context. Supported targets use the same aliases as
`/soul info`: `soul`, `identity`, `user`, `tools`, `memory`, `heartbeat`,
`latest`, and `.gitclaw/memory/YYYY-MM-DD.md`. The planner reports only
target hash, target term count, normalized path, category, present/required/
canonical/loaded flags, validation rollup, and findings. It never emits the
raw requested change, raw target text, raw context bodies, issue bodies,
comments, prompts, or secret values.

The soul edit planner never writes `.gitclaw/` files, creates branches,
applies patches, commits, pushes, or lets the model rewrite its own identity,
memory, tools, heartbeat, or policy context. The report includes
`llm_e2e_required_after_change=true` to make the release rule explicit: after
a soul file is actually changed, maintainers must run a live GitHub Models
conversation E2E in addition to deterministic soul-report tests.

When called as `@gitclaw /soul validate`, the command posts only the
validation report: status, error/warning totals, required-file counts,
memory-note counts, noncanonical memory-note count, and body-free findings.
This mirrors `gitclaw soul validate` for issue-side audits without the full
context inventory.

When called as `@gitclaw /soul verify`, the command posts a body-free trust
envelope for high-authority context. It reports repo-local versus unknown
context sources, required-file presence, soul frontmatter and description
presence, identity/policy and memory-note counts, short hashes for loaded
files, and explicit `registry_verification=not_configured` and
`profile_export_verification=not_configured` findings. This mirrors
`gitclaw soul verify` and makes the OpenClaw/Hermes-inspired soul provenance
audit visible in GitHub without dumping raw context.

When called as `@gitclaw /soul risk` or `@gitclaw /soul risk-audit`, the
command posts a body-free high-authority context risk audit. It scans loaded
SOUL, identity, user, tool, memory, heartbeat, and dated memory files for
prompt-boundary override language, secret exfiltration instructions,
persistent-state backdoors, attacker-controlled channel setup, unbounded
automation loops, unreviewed host execution, and credential persistence
instructions. It reports only status, counts, paths, categories, finding
codes, max severity, and short line hashes. It never emits raw soul, user,
memory, tools, heartbeat, issue, comment, prompt, or secret bodies. The report
includes `llm_e2e_required_after_soul_risk_change=true`; every change to this
risk surface must ship with a live GitHub Models conversation E2E that makes an
actual model call after the deterministic risk report.

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
  phrases or identifiers from the issue thread and returns matching lines with
  file, query, per-query match, total match, and line-length limits so one broad
  query cannot starve later explicit phrases.
- `gitclaw.read_file`: reads a bounded text file only when the issue thread
  explicitly mentions that repository-relative path.

Tool outputs are inserted into the prompt as auditable context blocks. They are
not autonomous shell execution, and they do not mutate the repository.

Repo owners can gate deterministic tool outputs from reviewed config:

```yaml
tools:
  allowed:
    - list_files
    - read_file
  disabled:
    - search_files
```

`tools.allowed` is an optional allowlist; if present, only those tool names can
emit prompt-visible tool outputs. `tools.disabled` is an optional denylist and
wins over `tools.allowed`. Both lists accept the full `gitclaw.read_file` form
or the short `read_file` suffix, and unknown names are rejected at config load
time. Disabled or allowlist-blocked tools remain visible in deterministic tool
reports, but their output bodies are not generated for model context.

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
gitclaw tools verify
gitclaw tools risk
gitclaw tools validate
gitclaw tools list
gitclaw tools run-plan <name>
gitclaw tools info <name>
gitclaw tools search <query> --max-results 10
```

The validation output includes only names, counts, and short finding details.
It never dumps full tool outputs, file bodies, or search result bodies.

## Tools Inspection Command

GitClaw supports a deterministic tool-surface audit command inspired by
OpenClaw's tool policy visibility and Hermes' toolset inventory:

```text
@gitclaw /tools
@gitclaw /tools list
@gitclaw /tools verify
@gitclaw /tools risk
@gitclaw /tools validate
@gitclaw /tools run-plan search_files
@gitclaw /tools info read_file
@gitclaw /tools search read_file
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
  counts, missing-guidance count, duplicate-contract count, enabled/disabled/
  allowlist-blocked tool counts, per-tool gate state, and body-free findings.
- tool risk status, risk finding counts, risk codes, severities, and hashes for
  prompt-boundary, secret exfiltration, credential exposure, host execution,
  repository mutation, remote exfiltration, unbounded-loop, and tool-provenance
  risks.

It never dumps full tool output bodies. Tool output bodies remain prompt inputs
only; the issue-visible report exposes enough metadata to debug whether
`gitclaw.list_files`, `gitclaw.search_files`, `gitclaw.read_file`,
`gitclaw.skill_index`, or `gitclaw.policy` ran for the turn.
`@gitclaw /tools list` is an explicit inventory alias for the same report,
matching the local `gitclaw tools list` helper.

When called as `@gitclaw /tools validate`, the command posts only the
validation report: tool contract counts, active-output counts, status,
error/warning totals, and body-free findings. This mirrors
`gitclaw tools validate` for issue-side audits without the full inventory.

When called as `@gitclaw /tools verify`, the command posts a stricter
body-free trust envelope for deterministic tool contracts. It reports built-in
contract modes, enabled/disabled/allowlist-blocked gate state,
read-only/metadata-only/mutating counts, active output counts, known versus
unknown outputs, `.gitclaw/TOOLS.md` provenance and hash metadata,
active-output input/output hashes, and explicit external-registry and runtime
permission verification status. Unlike the inventory report, it does not print
raw tool input values such as file paths or search phrases.

When called as `@gitclaw /tools risk` or `@gitclaw /tools risk-audit`, the
command posts a body-free tool-surface risk audit. It scans built-in
deterministic tool contracts, repo-local `.gitclaw/TOOLS.md` guidance, and
active prompt-visible tool input/output metadata for prompt-boundary overrides,
secret exfiltration instructions, exposed credential material, host execution,
repository mutation, remote exfiltration, unbounded tool loops, and unknown
tool-output provenance. It reports only names, paths, fields, counts,
categories, finding codes, severities, and short hashes. It never emits raw
tool outputs, raw tool inputs, file bodies, issue bodies, comments, prompts, or
secret values. The report includes
`llm_e2e_required_after_tool_risk_change=true`; every change to this risk
surface must ship with a live GitHub Models conversation E2E that makes an
actual model call after the deterministic risk report.

When called as `@gitclaw /tools info <name>`, the command posts a focused
body-free card for one declared tool contract. The lookup accepts either the
full `gitclaw.read_file` name or the short `read_file` suffix, reports the
contract source, mode, mutation status, trigger condition, active-output count,
active-output input hashes, output sizes, output hashes, and match-scoped
validation findings. It never emits raw tool inputs, tool output bodies,
issue/comment bodies, prompts, or file bodies. This mirrors the skill-info
workflow for tools: one contract can be inspected without dumping the full
tool inventory.

When called as `@gitclaw /tools run-plan <name>`, the command posts a
body-free dry-run plan for one declared tool contract. It reports the
normalized tool name, match count, active-output count, enabled/disabled/
allowlist gate state, contract mode, trigger, mutation flag, validation
summary, active-output hashes, and review steps. It never calls a model,
executes shell commands, makes network calls, mutates the repository, or emits
raw tool names from the issue, raw inputs, raw outputs, issue/comment bodies,
prompts, or file bodies. Any implementation change to tool behavior must pair
the deterministic run-plan E2E with a live GitHub Models conversation E2E so
the real model path stays tested too.

When called as `@gitclaw /tools search <query>`, the command searches declared
tool-contract metadata and active tool-output metadata. It reports the query
hash and term count, matched contract and output counts, result limits,
matched fields, contract mode/trigger metadata, active-output input hashes,
output byte/line counts, and output hashes. It never emits raw tool inputs,
tool output bodies, issue/comment bodies, prompts, or raw search queries. This
keeps tool debugging aligned with OpenClaw's tool-policy visibility and
Hermes' explicit toolset inventory without turning issue comments into a
secondary prompt dump.

## Sandbox Inspection Command

GitClaw supports a deterministic sandbox/exec-policy report inspired by
OpenClaw's sandbox versus tool-policy split and Hermes' explicit security
boundary:

```text
@gitclaw /sandbox
@gitclaw /sandboxes
@gitclaw /exec-policy
```

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/sandbox"` and summarizes:

- GitHub Actions as the current ephemeral runtime boundary,
- absence of host shell, file-write, pull-request, or elevated execution tools,
- read-only write mode, disabled host exec, and non-applicable approval mode,
- deterministic tool counts by read-only, metadata-only, and mutating modes,
- active tool-output counts and input/output hashes,
- checked-in workflow permission status and per-job expected/actual
  permission cards,
- skill binary requirement counts without running those binaries,
- backup write permission as isolated to the post-handle backup job.

It never dumps issue bodies, comments, prompts, workflow bodies, tool output
bodies, or secrets. The report is intentionally honest about the MVP: GitClaw
does not claim a Docker/OpenShell-style sandbox because it does not expose a
host execution tool. Future host exec support must add allowlists, approval
storage, hard blocklists, and body-free audit cards before enabling execution.

Local operators can inspect the same boundary before opening an issue:

```bash
gitclaw sandbox explain
gitclaw sandbox verify
```

## Context Inspection Command

GitClaw supports a deterministic context inspection command inspired by
OpenClaw's `/context` diagnostics:

```text
@gitclaw /context
@gitclaw /context list
@gitclaw /context info <path>
```

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/context"` and summarizes:

- selected context files,
- selected full skill documents,
- read-only tool outputs and their input/size/hash,
- transcript and prompt-budget settings.

It never dumps full file bodies, skill bodies, prompts, or tool output contents
into the issue. This makes context visibility cheap enough for routine E2E
debugging and avoids burning GitHub Models quota for a diagnostic turn.

When called as `@gitclaw /context info <path>`, the command posts a focused
body-free card for one requested context item. The lookup covers loaded context
documents, selected skill documents, deterministic `gitclaw.read_file` outputs
for explicitly mentioned repository files, and active tool outputs addressed by
tool name. It reports only the matched kind, path/tool name, byte/line counts,
short hashes, match source, and safety flags. It never emits raw file bodies,
skill bodies, tool output bodies, raw tool inputs, issue/comment bodies, prompts,
or secrets.

Local operators can inspect the same repository context surface without opening
an issue:

```bash
gitclaw context list
gitclaw context info .gitclaw/SOUL.md
gitclaw context info go.mod
```

The local report omits repository and issue metadata, reports zero transcript
messages, and lists body-free context files, selected always-on skills, and
deterministic tool-output metadata with short hashes. The focused local
`context info` variant seeds context assembly with the requested path, so
ordinary repository files can be inspected through the same body-free
`gitclaw.read_file` metadata that would be prompt-visible in an issue turn.

## Prompt Inspection Command

GitClaw supports a deterministic prompt-budget audit command inspired by
OpenClaw's context diagnostics and Hermes' bounded memory/context posture:

```text
@gitclaw /prompt
@gitclaw /prompt list
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

Local operators can inspect the same prompt-budget and prompt-input surface
without opening an issue:

```bash
gitclaw prompt list
```

The local report omits repository and issue metadata, reports zero transcript
messages, and still summarizes provider/model, prompt hash/size, prompt
budgets, context file metadata, selected always-on skills, and deterministic
tool-output metadata without dumping prompt text or any loaded bodies.

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

## Approvals Inspection Command

GitClaw supports a deterministic approval-readiness command inspired by
OpenClaw's exec-approval gates and Hermes' explicit command approval posture:

```text
@gitclaw /approvals
@gitclaw /approval
```

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/approvals"` and summarizes:

- preflight result, trigger state, actor association, and trust decision,
- whether write intent was detected and labeled with `gitclaw:write-requested`,
- whether per-issue approval labels are present,
- the future approval store and scope, currently GitHub issue labels per issue,
- the hard read-only write-mode gate.

It never enables write mode, approves anything, executes commands, dumps issue
or comment bodies, prints prompt text, or exposes approval payloads. In v1, an
issue with both write intent and `gitclaw:approved` reports
`approved_but_write_mode_disabled`: the approval signal is visible, but mutation
remains blocked until a later reviewed write-mode implementation exists.

Local operators can inspect the static approval shape without opening an issue:

```bash
gitclaw approvals list
gitclaw approvals verify
```

The local report omits repository, issue, actor, trigger, and write-intent
state. It still reports the approval label names, trusted association source,
per-issue GitHub-label approval store, and read-only write-mode gate.

## Policy Inspection Command

GitClaw supports a deterministic policy audit command inspired by OpenClaw's
sandbox/tool-policy/elevated split and Hermes' authorization and approval
posture:

```text
@gitclaw /policy
@gitclaw /policy list
@gitclaw /policy verify
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

When called as `@gitclaw /policy verify`, the command switches from inventory
to a stricter body-free permission audit. It verifies the checked-in
`.github/workflows/gitclaw.yml` jobs against the expected contract:
`preflight` has `contents:read` and `issues:read`, `handle` has
`contents:read`, `issues:write`, and `models:read`, and `backup` has
`contents:write` plus `issues:read`. It reports workflow file hashes,
present job counts, matched/missing permissions, unexpected uncontracted write
grants, active `gitclaw.policy` input/output hashes, and findings. It never
emits workflow bodies, issue/comment bodies, raw policy inputs, or policy
output bodies.

Local operators can inspect static policy shape without opening an issue:

```bash
gitclaw policy list
gitclaw policy verify
```

The local report omits event-only fields such as repository, issue number,
preflight result, actor association, trigger state, event labels, and
write-request detection. It still reports trusted associations, managed labels,
expected workflow permissions, model/run mode, and active policy-output
metadata if present.
`gitclaw policy verify` additionally checks the local workflow permission
contract and returns a non-body verification report suitable for CI.

## Secrets Audit Command

GitClaw supports a deterministic repo secrets audit command inspired by
OpenClaw's `openclaw secrets audit --check` operator loop and Hermes' default
secret-isolation posture:

```text
@gitclaw /secrets
@gitclaw /secret
@gitclaw /secrets audit
```

The command runs after normal preflight authorization but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/secrets"` and performs a bounded, read-only scan of the checked
out repository for:

- known token shapes such as GitHub PATs, OpenAI-style keys, Slack tokens, and
  Telegram bot tokens,
- heuristic sensitive assignments using key names such as `token`, `secret`,
  `password`, `credential`, `authorization`, and `api_key`,
- GitHub Actions secret references such as `${{ secrets.NAME }}`.

The report includes file counts, skipped-file counts, finding counts, reference
counts, path, line number, severity, finding code, and short hashes of matched
values, source lines, and referenced secret names. It never prints matched
values, source lines, secret names, issue bodies, comments, prompts, or
environment values. Secret references are reported separately from plaintext
findings because `${{ secrets.NAME }}` is usually expected config, while
plaintext residues are actionable.

Local operators can run the same audit without opening an issue:

```bash
gitclaw secrets audit
gitclaw secrets scan
gitclaw secrets list
```

The aliases intentionally return the same body-free report in v1. GitClaw does
not yet configure, migrate, apply, reload, or resolve secrets. The safe MVP is
visibility first: find possible checked-in residue without giving an LLM or an
issue comment the underlying secret material.

## Checkpoints And Rollback Readiness

GitClaw supports a deterministic checkpoint/rollback-readiness command inspired
by Hermes' checkpoint/rollback feature and OpenClaw's separation between
approval, sandboxing, and mutation:

```text
@gitclaw /checkpoints
@gitclaw /checkpoint
@gitclaw /rollback
```

The command runs after normal preflight authorization but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/checkpoints"` and inspects the checked-out repository's git
metadata:

- whether `git` is available and the workdir is inside a git repository,
- current branch or detached-HEAD state,
- current HEAD short SHA,
- total commit count visible in the checkout,
- recent commit count with commit subjects represented only by hashes,
- staged, unstaged, and untracked change counts,
- whether a local ref for the dedicated `gitclaw-backups` branch is visible.

The report never prints diffs, file bodies, commit subjects, issue bodies,
comments, prompts, or secret values. It also never runs restore, reset,
checkout, or branch mutation commands. `@gitclaw /rollback` is therefore a
readiness report, not a restore command.

Local operators can inspect the same checkpoint state without opening an issue:

```bash
gitclaw checkpoints status
gitclaw checkpoints list
gitclaw checkpoints verify
gitclaw rollback list
```

The aliases intentionally return the same body-free report in v1. Reviewed
recovery still happens through ordinary git history, pull requests, and fetched
backup manifests.

## Diffs Inspection Command

GitClaw supports a deterministic diff audit inspired by OpenClaw's read-only
diff artifact plugin and Hermes' checkpoint `/rollback diff` preview:

```text
@gitclaw /diffs
@gitclaw /diff
@gitclaw /changes
```

The command runs after preflight and context loading, but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/diffs"` and summarizes:

- whether `.gitclaw/DIFFS.md` exists and is loaded into model context,
- declarative diff specs in `.gitclaw/diffs/*.md`,
- git availability, repository detection, branch, and HEAD short SHA,
- worktree clean/dirty state,
- changed, staged, unstaged, untracked, renamed, deleted, and binary-file
  counts,
- staged and unstaged insertion/deletion totals from `git diff --numstat`,
- changed file paths and git status codes, capped by a fixed file limit,
- body-free findings for missing policy, unsafe specs, or git inspection
  failures.

It never prints raw unified patch hunks, file bodies, issue/comment bodies,
prompts, tool outputs, backups, or secret values. It also never stages, resets,
restores, commits, opens pull requests, or writes artifacts. Raw patches belong
in explicit artifacts, pull requests, or local export paths; `/diffs` stays an
issue-safe map of what changed.

Local operators can inspect the same change surface without opening an issue:

```bash
gitclaw diffs summary
gitclaw diffs verify
```

## Workspace Inspection Command

GitClaw supports a deterministic workspace audit inspired by OpenClaw's agent
workspace and Hermes' git-worktree isolation model:

```text
@gitclaw /workspace
@gitclaw /workdir
@gitclaw /repo
```

The command runs after preflight and context loading, but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/workspace"` and summarizes:

- whether `.gitclaw/WORKSPACE.md` exists and is loaded into model context,
- declarative workspace specs in `.gitclaw/workspaces/*.md`,
- git availability, repository detection, branch, and HEAD short SHA,
- bounded repository file inventory counts without path or body dumps,
- context allowlist/document counts,
- configured GitClaw workflow files and their checkout/setup-go action
  versions,
- fetch-depth configuration and the Actions runner isolation boundary,
- body-free findings for missing policy, unsafe workspace specs, missing
  workflow checkout, or git inspection failures.

It never prints raw file bodies, issue/comment bodies, prompts, tool outputs,
backup payloads, workflow bodies, or secrets. It also never writes files,
cleans directories, changes refs, dispatches workflows, mounts external
workspaces, or treats the Actions checkout as private durable memory.

Local operators can inspect the same workspace surface without opening an
issue:

```bash
gitclaw workspace summary
gitclaw workspace verify
```

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
  model: openai/gpt-5-nano
  fallbacks:
    - openai/gpt-4.1-nano
  base_url: https://models.github.ai/inference/chat/completions
  max_input_tokens: 60000
  max_output_tokens: 4000

skills:
  allowed:
    - repo-reader
  disabled: []

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
- `skills.allowed`, optional lower hyphen-case skill allowlist,
- `skills.disabled`, optional lower hyphen-case skill denylist,
- `tools.allowed`, optional `gitclaw.` tool allowlist,
- `tools.disabled`, optional `gitclaw.` tool denylist,
- `actions.mode`, which must currently be `read_only`.

Unknown YAML fields are rejected. This mirrors OpenClaw's schema/validate
discipline without adding agent-authored config writes. Secrets do not belong
in this file; model auth continues to come from GitHub Actions tokens or
environment variables.

### Config Inspection Command

GitClaw supports a deterministic config/control-plane audit command:

```text
@gitclaw /config
@gitclaw /config list
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

Local operators can inspect the same effective control plane without opening an
issue:

```bash
gitclaw config list
```

The local report omits repository, issue number, and issue-title hash while
retaining effective config source, labels, trusted associations, prompt
budgets, deterministic slash commands, and config/workflow file metadata.

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

### Standing Orders Command

GitClaw supports repo-reviewed standing orders inspired by OpenClaw's
persistent authority model:

```text
@gitclaw /orders
@gitclaw /standing-orders
```

The command runs after preflight and before model inference. It posts a
`gitclaw:assistant-turn` comment with `model="gitclaw/orders"` and summarizes:

- whether `.gitclaw/STANDING_ORDERS.md` exists,
- whether the file is loaded into model context,
- whether `AGENTS.md` links to standing orders,
- number of standing-order programs,
- how many programs include `Authority`, `Trigger`, `Approval gate`, and
  `Escalation` clauses,
- proactive workflow and prompt metadata that can enforce scheduled programs,
- body-free findings for missing structure.

It never executes standing orders, creates schedules, changes repository files,
calls the model, or prints raw order, issue, comment, workflow, or proactive
prompt bodies. This keeps OpenClaw-style durable authority inspectable through
GitHub before it becomes automation.

Local operators can inspect the same surface with:

```bash
gitclaw orders list
gitclaw orders verify
```

### Hooks Command

GitClaw supports declarative hooks inspired by OpenClaw's file-based hook
surface:

```text
@gitclaw /hooks
@gitclaw /hook
```

The command runs after preflight and before model inference. It posts a
`gitclaw:assistant-turn` comment with `model="gitclaw/hooks"` and summarizes:

- whether `.gitclaw/HOOKS.md` exists and is loaded into model context,
- declarative hook specs in `.gitclaw/hooks/*.md`,
- declared event counts,
- whether specs are `audit-only`,
- whether specs require approval before side effects,
- whether executable-looking handler files are present,
- body-free findings for missing metadata or unsafe-looking files.

GitClaw v1 does not execute hook handlers. Hook specs are reviewed repo
metadata, not runtime code. The report never dispatches workflows, mutates the
repo, calls the model, or prints raw hook policy, hook spec, issue, comment, or
provider payload bodies. Future executable hooks require explicit workflow
permissions, approval gates, and audit cards before any handler can run.

Local operators can inspect the same surface with:

```bash
gitclaw hooks list
gitclaw hooks verify
```

### Plugins Command

GitClaw supports declarative plugin audits inspired by OpenClaw's manifest and
runtime extension model, and by Hermes' toolset/MCP filtering:

```text
@gitclaw /plugins
@gitclaw /plugin
```

The command runs after preflight and before model inference. It posts a
`gitclaw:assistant-turn` comment with `model="gitclaw/plugins"` and summarizes:

- whether `.gitclaw/PLUGINS.md` exists and is loaded into model context,
- declarative plugin specs in `.gitclaw/plugins/*.md`,
- plugin kind, source, and metadata-only activation state,
- declared capability and optional capability counts,
- whether specs require approval before side effects or new tool exposure,
- whether executable/package/runtime-looking files are present,
- body-free findings for missing metadata or unsafe-looking files.

GitClaw v1 does not install plugins, connect MCP servers, invoke package
managers, start webhooks, or expose new model-visible tools from plugin specs.
Plugin specs are reviewed repo metadata, not runtime code. The report never
mutates the repo, calls the model, or prints raw plugin policy, plugin spec,
issue, comment, config, credential, or provider payload bodies. Future
executable plugins require reviewed workflows, explicit permissions, approval
gates, and audit cards before any runtime can activate.

Local operators can inspect the same surface with:

```bash
gitclaw plugins list
gitclaw plugins verify
```

### Tasks Command

GitClaw supports a deterministic task-board audit inspired by OpenClaw
background tasks, Task Flow, and Hermes Kanban:

```text
@gitclaw /tasks
@gitclaw /task
```

The command runs after preflight and before model inference. It posts a
`gitclaw:assistant-turn` comment with `model="gitclaw/tasks"` and summarizes:

- whether `.gitclaw/TASKS.md` exists and is loaded into model context,
- declarative task/flow specs in `.gitclaw/tasks/*.md`,
- declared issue-native statuses and labels,
- whether specs require approval before side effects or worker dispatch,
- the current issue's task status derived from GitHub labels,
- current issue comment and transcript counts,
- body-free findings for missing task policy or unsafe-looking specs.

GitClaw v1 does not create a separate task database, start a dispatcher, spawn
detached workers, create child agents, or execute Task Flow/Kanban-style
pipelines. GitHub issues are the task rows, issue labels are the state
machine, and issue comments are the handoff log. The report never mutates the
repo, calls the model, opens SQLite, or prints raw task policy, task spec,
issue, comment, transcript, flow, or worker-output bodies.

Local operators can inspect the same policy/spec surface with:

```bash
gitclaw tasks list
gitclaw tasks verify
```

### Agents Command

GitClaw supports a deterministic agent-surface audit inspired by OpenClaw
multi-agent routing, OpenClaw nodes, Hermes `delegate_task`, and Hermes Kanban
workers:

```text
@gitclaw /agents
@gitclaw /agent
```

The command runs after preflight and before model inference. It posts a
`gitclaw:assistant-turn` comment with `model="gitclaw/agents"` and summarizes:

- whether `.gitclaw/AGENTS.md` exists and is loaded into model context,
- declarative agent specs in `.gitclaw/agents/*.md`,
- declared agent roles, runtime, mode, and requested tool names,
- whether specs require approval before delegation, routing, or side effects,
- the active runtime boundary: one GitHub Actions repo assistant in v1,
- body-free findings for missing policy or unsafe-looking specs.

GitClaw v1 does not run OpenClaw-style multi-agent routing, start node hosts,
spawn Hermes-style subagents, create Kanban workers, or send agent-to-agent
messages. Agent specs are reviewed repo metadata, not process definitions. The
report never mutates the repo, calls the model, starts a gateway, or prints raw
agent policy, agent spec, issue, comment, channel, credential, or tool-output
bodies. Future multi-agent support requires reviewed workflows, explicit
permissions, approval gates, body-free audit cards, and a live GitHub Models
conversation E2E in the same implementation batch.

Local operators can inspect the same policy/spec surface with:

```bash
gitclaw agents list
gitclaw agents verify
```

### Nodes Command

GitClaw supports a deterministic node-surface audit inspired by OpenClaw node
hosts and paired device capabilities, plus Hermes' durable workers and
delegation runtime boundaries:

```text
@gitclaw /nodes
@gitclaw /node
```

The command runs after preflight and before model inference. It posts a
`gitclaw:assistant-turn` comment with `model="gitclaw/nodes"` and summarizes:

- whether `.gitclaw/NODES.md` exists and is loaded into model context,
- declarative node specs in `.gitclaw/nodes/*.md`,
- declared node roles, runtime, mode, and GitHub-native capabilities,
- whether specs require approval before pairing, remote execution, or new host
  capabilities,
- the active runtime boundary: ephemeral GitHub Actions jobs in v1,
- body-free findings for missing policy or unsafe-looking specs.

GitClaw v1 does not run OpenClaw-style node hosts, pair devices, maintain a
Gateway WebSocket, invoke node RPC commands, expose browser proxies, access
camera/screen/location/SMS/notification surfaces, or forward shell execution to
remote nodes. Node specs are reviewed repo metadata, not service definitions.
The report never mutates the repo, calls the model, starts a node service, or
prints raw node policy, node spec, issue, comment, channel, credential, or
provider payload bodies. Future remote-node execution requires reviewed
workflows, explicit permissions, approval gates, body-free audit cards, and a
live GitHub Models conversation E2E in the same implementation batch.

Local operators can inspect the same policy/spec surface with:

```bash
gitclaw nodes list
gitclaw nodes verify
```

### Artifacts Command

GitClaw supports a deterministic artifact-governance audit inspired by
OpenClaw backup/migration exports, Hermes sessions and checkpoints, and
GitHub Actions artifacts:

```text
@gitclaw /artifacts
@gitclaw /artifact
```

The command runs after preflight and before model inference. It posts a
`gitclaw:assistant-turn` comment with `model="gitclaw/artifacts"` and
summarizes:

- whether `.gitclaw/ARTIFACTS.md` exists and is loaded into model context,
- declarative artifact specs in `.gitclaw/artifacts/*.md`,
- artifact kind, storage backend, filename, workflow, label gate, retention,
  redaction, and approval metadata,
- workflow upload metadata for `actions/upload-artifact`, including version,
  retention days, prompt-artifact label gates, and missing-file behavior,
- the runtime boundary between short-lived Actions artifacts and durable
  git-backed backups,
- body-free findings for missing policy, missing metadata, unsafe storage, or
  missing retention/redaction gates.

GitClaw v1 does not treat artifacts as hidden state, durable memory, or a
second conversation transcript. Issue comments may include artifact metadata,
hashes, run links, and findings, but must not dump raw prompt, model, tool,
backup, transcript, channel, secret, or artifact bodies. Future artifact types
require reviewed workflows, explicit retention, redaction rules when needed,
body-free audit cards, and a live GitHub Models conversation E2E in the same
implementation batch.

Local operators can inspect the same policy/spec/upload surface with:

```bash
gitclaw artifacts list
gitclaw artifacts verify
```

### Doctor Command

GitClaw supports a deterministic doctor/health audit command:

```text
@gitclaw /doctor
@gitclaw /doctor list
@gitclaw /health
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

Local operators can run the same body-free health check before opening an
issue:

```bash
gitclaw doctor list
```

`gitclaw doctor` remains the short form. The local report uses
`scope: local-cli` and omits repository, issue, and issue-title metadata.

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

For `workflow_dispatch` wakeups from channel ingest, the active request text is
the mirrored `gitclaw:channel-message` comment whose provider ID matches the
dispatch ID. That lets Telegram/Slack users invoke deterministic commands such
as `@gitclaw /channels` through the same bridge without a model call.

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
- if the same `channel + message_id` has already been mirrored, do not post a
  second `gitclaw:channel-message` comment and skip the downstream main
  workflow dispatch.

This workflow is useful for E2E, manual bridge experiments, and tiny external
dispatchers. Provider-specific pollers can later call the same CLI path after
they read Telegram/Slack events.

### Channel State Command

Provider-specific bridges need durable state before GitClaw can safely poll
Telegram or Slack without a server. GitClaw exposes this as a GitHub-native
state issue instead of a database:

```bash
gitclaw channel-state \
  --repo OWNER/REPO \
  --channel telegram \
  --account-id <provider-account-or-workspace-id> \
  --offset <provider-offset-or-update-id>
```

Behavior:

- find or create one open issue with a hidden `gitclaw:channel-state` marker
  for `channel + account_sha256_12`,
- label it with `gitclaw:channel` but do not apply the normal `gitclaw`
  trigger label,
- store account IDs and offsets only as `sha256_12` hashes in issue titles,
  issue bodies, comments, CLI output, and GitHub Actions output,
- post a `gitclaw:channel-state-update` comment for a new offset,
- treat a repeated `channel + account_sha256_12 + offset_sha256_12` as a
  duplicate and avoid posting a second state update comment.

This gives polling and long-running gateway experiments an auditable offset and
dedupe primitive without a webhook server, socket service, runner filesystem
state, or plaintext provider state in GitHub issues.

The same primitive is available through `.github/workflows/gitclaw-channel-state.yml`.
Provider pollers or manually dispatched bridge jobs can call it with
`workflow_dispatch` inputs for `channel`, `account_id`, `offset`, and optional
`lease_run_id`. The workflow writes the same body-free issue state through
`gitclaw channel-state`, so bridge state updates do not need a server-side
webhook endpoint or a local machine with credentials.

### Channel Gateway Command

GitClaw also exposes a minimal gateway lease command for the no-server,
long-running Actions strategy:

```bash
gitclaw channel-gateway \
  --repo OWNER/REPO \
  --channel telegram \
  --account-id <provider-account-or-workspace-id> \
  --gateway-slot <slot> \
  --lease-run-id <run-id> \
  --renew
```

The command does not yet open provider sockets or poll Telegram/Slack APIs.
Instead, it records the gateway lease through the same
`gitclaw:channel-state-update` mechanism, hashing a lease payload derived from
`channel`, `account_id`, `gateway_slot`, and `lease_run_id`. Repeating the same
lease run is idempotent, while a renewed run gets a new `lease_run_id` and
therefore a new auditable state update.

`.github/workflows/gitclaw-channel-gateway.yml` wraps this command with
`workflow_dispatch`. With `renew=false`, it records one interrupt-safe gateway
lease and exits. With `renew=true`, it dispatches a successor gateway run before
the job exits, using `actions: write`. This is the first executable version of
the long-running Actions gateway idea: no webhook server, no always-on VM, and
state durable in GitHub issues.

### Channel Delivery Command

Outbound channel bridges need the same idempotency discipline as inbound
ingress. A future Telegram/Slack sender should not resend the same GitHub
assistant reply after a retry, but GitHub issue comments should remain the
canonical assistant transcript. GitClaw records that edge with a delivery
receipt:

```bash
gitclaw channel-delivery \
  --repo OWNER/REPO \
  --channel telegram \
  --account-id <provider-account-or-workspace-id> \
  --issue-number <github-issue> \
  --comment-id <github-assistant-comment-id> \
  --external-message-id <provider-message-id>
```

Behavior:

- verify the source comment exists and carries a `gitclaw:assistant-turn`
  marker,
- find or create the matching `gitclaw:channel-state` issue,
- post one `gitclaw:channel-delivery` receipt for
  `channel + account_sha256_12 + source issue + source comment`,
- store the external provider message id only as `external_message_sha256_12`,
- treat repeated delivery receipts for the same source comment as duplicates.

`.github/workflows/gitclaw-channel-delivery.yml` exposes the same receipt path
through `workflow_dispatch`, so a gateway can send a reply through Telegram or
Slack and then use the repository `GITHUB_TOKEN` to record exactly what GitHub
assistant comment was delivered without writing channel credentials or reply
bodies into the state issue.

### Channel Inspection Command

GitClaw supports a deterministic channel/control-plane audit command:

```text
@gitclaw /channels
@gitclaw /channels list
@gitclaw /channels verify
@gitclaw /channels risk
@gitclaw /channels info telegram
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

`@gitclaw /channels verify` uses the same data surface but switches from
inventory to health posture. It reports `channel_verify_status`, confirms the
channel-ingest workflow has `workflow_dispatch`, `actions: write`,
`issues: write`, and the five normalized inputs required for channel mirroring,
then lists body-free verification findings.

`@gitclaw /channels risk` and `@gitclaw /channels risk-audit` post a
body-free risk audit for the Slack/Telegram workflow-dispatch bridge. They scan
provider contracts, channel bridge workflows, and prompt-visible
`gitclaw:channel-message` comments for prompt-boundary overrides, secret
exfiltration instructions, credential exposure, raw channel-body logging,
channel-body execution, external webhook exposure, and unbounded gateway loops.
The report emits only provider names, workflow paths, comment IDs, hashes,
counts, finding codes, categories, and severities. It never emits channel
message bodies, issue bodies, workflow bodies, prompts, provider credentials, or
secret values. The report includes
`llm_e2e_required_after_channel_risk_change=true`; every channel-risk
implementation batch must also run a real GitHub Models conversation E2E after
the deterministic report.

`@gitclaw /channels info <provider>` is the focused provider contract card for
`telegram`, `slack`, or `generic`. It reports `channel_info_status`, required
secret names without values, provider offset/thread/message keys, the
workflow-dispatch wake strategy, state issue storage, gateway runtime,
ingest/state/gateway/delivery workflow metadata, and exact local command
shapes for that provider. This makes the no-server Telegram/Slack strategy
operator-visible before provider-specific pollers are implemented.

Local operators can inspect the same bridge contract without opening an issue:

```bash
gitclaw channels list
gitclaw channels verify
gitclaw channels risk
gitclaw channels info telegram
gitclaw channel-state --channel telegram --account-id <id> --offset <offset>
gitclaw channel-gateway --channel telegram --account-id <id> --renew
gitclaw channel-delivery --channel telegram --account-id <id> --issue-number <issue> --comment-id <comment> --external-message-id <message>
```

The local report omits issue-only fields such as repository, issue number,
channel-thread status, marker counts, and title hash, but still verifies the
workflow-dispatch bridge shape, provider keys, labels, permissions, and ingest
contract.

### Tier 2: Long-Running Actions Gateway

Run `.github/workflows/gitclaw-channel-gateway.yml` via `workflow_dispatch`.
Today, the job records a durable gateway lease and can self-dispatch a
successor run. Later, the same job can open a Telegram long-poll loop and/or
Slack Socket Mode WebSocket, mirror channel messages into GitHub
issues/comments, and mirror GitClaw replies back to the channel.

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
  -> record outbound delivery receipt with channel-delivery
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
- outbound delivery markers from `gitclaw:channel-delivery`,
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
- every delivered outbound reply records one `gitclaw:channel-delivery`
  receipt and retries do not create a second receipt,
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

`@gitclaw /artifacts` is the issue-visible audit surface for this contract. It
reports the artifact policy, artifact spec metadata, `actions/upload-artifact`
version, retention settings, and prompt-artifact label gate without reading or
printing any uploaded artifact body.

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

The backup job must use a repository-wide concurrency group such as
`gitclaw-backups-${{ github.repository }}` with `cancel-in-progress: false`.
Different issue conversations can finish at the same time, but the backup
branch is one shared git ref; serializing only that job avoids non-fast-forward
push races without slowing the read-only assistant turns.

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

- requested backup command intent (`summary`, `verify`, `manifest`, `list`,
  `info`, `stats`, `search`, `export-jsonl`, `restore-plan`, or
  `retention-plan`),
- the matching local `gitclaw backup ...` command to run against a fetched
  `gitclaw-backups` branch,
- that issue-side execution is metadata-only because the backup branch is
  written after the assistant turn,
- dedicated backup branch name,
- backup root and repo-scoped backup directory,
- expected issue backup JSON path,
- repo-scoped `index.json` and `README.md` paths,
- backup schema version,
- current raw comment, transcript, and assistant-turn counts,
- a short hash of the issue title for path/report correlation.

It never dumps issue/comment bodies. The report is navigational metadata; the
raw transcript copy remains the canonical backup JSON written by the post-turn
backup job.

Issue-side backup subcommands intentionally mirror OpenClaw's manifest-oriented
backup verification and Hermes' exportable session artifacts without pretending
the issue handler can verify a branch that has not been written yet. For
example, `@gitclaw /backup verify` records the exact local verification command
and the backup paths, then the post-turn backup job commits the raw issue JSON
and index to `gitclaw-backups`. `@gitclaw /backup risk` records the exact local
risk-audit command and risk categories while making clear that raw payload
scanning is deferred to a fetched backup branch. `@gitclaw /backup info
<issue-number>` records the exact focused-inspection command for one backed-up
issue, defaulting to the current issue when no number is supplied. `@gitclaw
/backup search <query>` records only a query hash and term count; raw search
terms and raw backup bodies stay out of the issue-visible comment.

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

## Backup Risk Command

GitClaw also supports a local backup risk audit for fetched backup branches:

```bash
gitclaw backup risk --root .gitclaw/backups --repo <owner/repo>
```

The command first runs the normal backup verifier, then scans indexed issue
payloads for recovery risks without echoing raw bodies. It reports only paths,
fields, severities, risk codes, categories, and short line hashes. The initial
rules cover integrity failures, path traversal, credential-looking material,
prompt-boundary text, restore-side-effect instructions, and oversized payloads.
This keeps the raw JSONL/export path explicit while still making it easy to
spot a backup tree that should not be blindly restored or copied into a prompt.

Issue-side `@gitclaw /backup risk` is intentionally deferred: the issue handler
records `gitclaw backup risk --root .gitclaw/backups --repo <owner/repo>` and
the fact that raw backup payloads are not scanned in the visible comment. The
real audit runs after fetching the post-turn `gitclaw-backups` branch. The live
E2E harness also posts a second normal comment that forces a GitHub Models call
and repo-reader tool usage, so the feature is tested with both deterministic
metadata and real model inference.

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

## Backup List Command

GitClaw supports a local backup list command inspired by Hermes' `sessions
list` inventory and OpenClaw's verified backup inspection posture:

```bash
gitclaw backup list --root .gitclaw/backups --repo <owner/repo> --limit 20
```

The command reads a fetched `gitclaw-backups` tree, verifies it, sorts indexed
backups by backup timestamp descending, and prints a deterministic
`GitClaw Backup List Report` with:

- backup list and verify status,
- schema version and index generation time,
- total indexed issue count, requested limit, and returned backup count,
- per-backup issue number, payload path, timestamp, event name, label count,
  comment count, transcript message count, and issue-title hash.

It never prints raw issue titles, issue bodies, comments, or transcript bodies.
The list report is the body-safe navigation layer before a more specific
`backup info`, `backup manifest`, `backup search`, `backup restore-plan`, or raw
`backup export-jsonl` command.

## Backup Info Command

GitClaw supports a focused single-backup inspection command inspired by
Hermes' session detail view and OpenClaw's body-safe backup verification
posture:

```bash
gitclaw backup info --root .gitclaw/backups --repo <owner/repo> --issue 123
```

The command reads a fetched `gitclaw-backups` tree, verifies it, locates one
indexed issue payload, and prints a deterministic `GitClaw Backup Info Report`
with:

- backup info and verify status,
- schema version and index generation time,
- issue number, canonical payload path, payload bytes, and payload hash,
- backup timestamp and event name,
- label, comment, transcript, user-message, assistant-message,
  assistant-turn, and error-comment counts,
- issue-title, issue-body, comment-body, and transcript-message hashes.

It never prints raw issue titles, issue bodies, comments, transcript messages,
prompts, or restore content. This is the narrow body-safe card to run after
`backup list` and before choosing `backup export-jsonl` or `backup
restore-plan`.

## Backup Search Command

GitClaw supports a local backup search command inspired by OpenClaw's
transcript/session inspection CLIs and Hermes' cross-session search posture,
but without adding a hidden database:

```bash
gitclaw backup search --root .gitclaw/backups --repo <owner/repo> <query>
```

The command reads a fetched `gitclaw-backups` tree, verifies it, searches issue
titles, issue bodies, raw comment bodies, and reconstructed transcript messages,
then prints a deterministic `GitClaw Backup Search Report` with:

- backup search and verify status,
- schema version and index generation time,
- query hash and query term count without echoing the raw query,
- indexed issue count plus searched issue-field/comment/transcript counts,
- matched issue count, matched line count, and returned result count,
- per-result issue number, backup path, source, role, actor/trust metadata,
  line number, score, body hash, line hash, timestamp, and event name.

It never prints raw issue titles, issue bodies, comments, transcript bodies, or
search query text. This gives operators a body-safe way to find old
conversations in durable git backups before choosing an explicit raw recovery
path such as `backup export-jsonl`.

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

   Live E2E coverage must not drift into deterministic reports only. Each
   implementation batch must include at least one LLM-backed conversation E2E
   through GitHub Models, such as `github-issue-chat.sh`,
   `github-search-tool-chat.sh`, `github-context-reference-chat.sh`, or
   `github-git-reference-chat.sh`, unless the change is provably unrelated to
   assistant turns. That test must exercise an actual model call and assert the
   assistant marker/model plus a real answer. If the primary model is
   rate-limited and the configured fallback answers, the test still counts as
   LLM-backed because it exercised GitHub Models inference; the assistant
   marker must record the actual model used. Report-only E2Es validate command
   surfaces; they do not prove inference, prompt assembly, transcript
   reconstruction, tool-output injection, fallback behavior, or GitHub Models
   permissions still work.

   Release rule: a feature batch is not done when only deterministic commands
   pass. The final validation set must include the feature-specific report E2E
   plus one live GitHub Models conversation E2E, and the result should be
   reported with the issue number and workflow run URL. For changes involving
   tools, context loading, memory, skills, hooks, standing orders, prompts, or
   policy, prefer `scripts/e2e/github-search-tool-chat.sh` because it proves a
   real model turn consumed deterministic repository tool output.

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
   - assert a mirrored `@gitclaw /channels` command produces the deterministic
     channel report without leaking the mirrored channel body.
   - dispatch the same channel message again,
   - assert it reuses the same issue, creates no duplicate channel-message
     comment, creates no duplicate assistant reply, and skips the redundant
     main workflow dispatch.

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
   - keep the search-result token prefix distinct from issue-thread nonce
     tokens so the test proves tool-output grounding rather than token echoing,
   - upload a redacted prompt artifact for the live chat E2E and assert it
     includes the `gitclaw.search_files` tool-output block and fixture token,
   - ask for a selected local skill token,
   - assert the targeted skill is loaded and irrelevant skills stay unloaded.

15. **Artifact governance**

   - create a real issue with `@gitclaw /artifacts`,
   - assert the reply is marked `model="gitclaw/artifacts"`,
   - assert the report lists `.gitclaw/ARTIFACTS.md`, artifact spec metadata,
     `actions/upload-artifact@v6`, retention days, prompt-artifact label gate,
     and missing-file behavior,
   - assert artifact policy/spec body tokens and uploaded artifact bodies are
     not printed,
   - run a real GitHub Models conversation E2E in the same feature batch.

16. **Context inspection**

   - create a real issue with `@gitclaw /context`,
   - create a real issue with `@gitclaw /context references` plus explicit
     `@file:` and `@folder:` references,
   - create a real issue with `@gitclaw /context ... @git:1`,
   - assert the reply is marked `model="gitclaw/context"`,
   - assert the report lists repo context files, selected skills, and read-only
     tool output names,
   - assert context reference metadata includes kind, path, line range, status,
     byte/line counts, folder-entry counts, and hashes,
   - assert Git reference metadata includes requested commit count, status,
     byte/line counts, and hashes,
   - assert referenced file bodies and issue body tokens are not dumped in the
     report,
   - assert the run succeeds without requiring a model provider response.

17. **Prompt inspection**

   - create a real issue with `@gitclaw /prompt`,
   - create a real issue with `@gitclaw /prompt list` as the explicit alias,
   - ask for a concrete file read, selected skill, and search fixture phrase,
   - assert the reply is marked `model="gitclaw/prompt"`,
   - assert the report lists prompt budget settings, final prompt size/hash,
     transcript inclusion/truncation counts, selected context files, selected
     skills, and active tool output metadata,
   - assert the report does not dump prompt text, issue body tokens, context
     bodies, skill bodies, or tool output bodies,
   - assert the run succeeds without requiring a model provider response.

18. **Memory inspection**

   - create a real issue with `@gitclaw /memory`,
   - create a second real issue with `@gitclaw /memory list`,
   - create a third real issue with `@gitclaw /memory verify`,
   - create a fourth real issue with `@gitclaw /memory info latest`,
   - assert the reply is marked `model="gitclaw/memory"`,
   - assert the report lists `.gitclaw/MEMORY.md`, dated memory note counts,
     loaded/omitted note counts, and memory file hashes,
   - assert the verify report includes repo-local provenance, loaded state,
     external-provider/session-index/background-promotion non-goals, and
     body-free trust cards,
   - assert the info report includes the normalized memory path, match status,
     kind/source/canonical/latest/loaded metadata, and file hash without a
     body,
   - assert the report does not dump memory file bodies or issue body tokens,
   - assert the run succeeds without requiring a model provider response.

19. **Memory search inspection**

   - create a real issue with `@gitclaw /memory search backup branch`,
   - assert the reply is marked `model="gitclaw/memory"`,
   - assert the report is marked `GitClaw Memory Search Report`,
   - assert it reports query hash/term count, scanned/matched counts, paths,
     line numbers, scores, loaded-for-turn state, and file/line hashes,
   - assert it does not dump the raw search query, issue body token, or memory
     file body tokens,
   - assert the run succeeds without requiring a model provider response.

20. **Memory risk inspection**

   - create a real issue with `@gitclaw /memory risk`,
   - assert the reply is marked `model="gitclaw/memory"`,
   - assert the report is marked `GitClaw Memory Risk Report`,
   - assert it reports memory-file counts, loaded state, write boundaries,
     external-provider non-goals, risk counts, risk codes, paths, and line
     hashes,
   - assert it does not dump memory bodies, issue body tokens, prompt text, or
     credential-looking values,
   - assert the run succeeds without requiring a model provider response,
   - post a normal follow-up on the same issue that requires repo-reader search
     and assert the second assistant turn is model-backed by GitHub Models with
     prompt context, selected skill, and prompt-visible tool markers.

21. **Memory promotion plan**

   - create a real issue with `@gitclaw /memory promote-plan long-term`,
   - assert the reply is marked `model="gitclaw/memory"`,
   - assert the report is marked `GitClaw Memory Promote Plan Report`,
   - assert it reports request hashes, transcript count, target kind/path,
     target metadata, memory budget, validation status, no model call, no
     repository mutation, no memory writes, no candidate generation, and the
     live-LLM E2E rule,
   - assert it does not dump issue body tokens, transcript text, memory body
     tokens, or candidate memory text,
   - assert the run succeeds without requiring a model provider response.

22. **Skills inspection**

   - create a real issue with `@gitclaw /skills`,
   - create a second real issue with `@gitclaw /skills list`,
   - create a third real issue with `@gitclaw /bundles info repo-context`,
   - assert the reply is marked `model="gitclaw/skills"`,
   - assert the report lists available skill metadata and selected skill paths,
   - assert the bundle info report lists bundle path, referenced/resolved
     skills, selected-for-turn state, instruction presence, and hashes,
   - assert hashes, frontmatter/description presence, and requirement counts
     are present,
   - assert skill validation status, duplicate-name count, invalid-name count,
     and folder/name mismatch count are present,
   - assert the report does not dump full skill bodies or verification tokens,
   - assert the run succeeds without requiring a model provider response.

23. **Skills search inspection**

   - create a real issue with `@gitclaw /skills search repository context`,
   - assert the reply is marked `model="gitclaw/skills"`,
   - assert the report is marked `GitClaw Skills Search Report`,
   - assert it reports query hash/term count, available skill count, matched
     skill count, match fields, selected-for-turn state, and skill hashes,
   - assert it does not dump the raw search query, issue body token, or full
     `SKILL.md` verification token,
   - assert the run succeeds without requiring a model provider response.

24. **Skills risk audit**

   - create a real issue with `@gitclaw /skills risk`,
   - assert the reply is marked `model="gitclaw/skills"`,
   - assert the report is marked `GitClaw Skills Risk Report`,
   - assert it reports skill risk status, scanned skill count, risky-finding
     counts, risk codes, skill hashes, line hashes, no registry verification,
     no installer execution, and no raw bodies,
   - assert it does not dump the issue body token, raw `SKILL.md` body token,
     or raw matched risky lines,
   - add a normal follow-up comment that requires the repo-reader skill and
     `gitclaw.search_files`, then assert the follow-up assistant marker records
     a real GitHub Models model, selected skill, prompt provenance, and
     `gitclaw.search_files`.

25. **Skills selection plan**

   - create a real issue with `@gitclaw /skills select-plan repo-reader`,
   - assert the reply is marked `model="gitclaw/skills"`,
   - assert the report is marked `GitClaw Skill Select Plan Report`,
   - assert it reports requested-skill hash, request-text hash, matched skill
     count, selected-for-turn state, gate state, selection reasons, validation
     status, no model call, no repository mutation, and the live-LLM E2E rule,
   - assert it does not dump the raw issue body token, raw request text, or
     full `SKILL.md` verification token,
   - assert the run succeeds without requiring a model provider response.

26. **Soul inspection**

   - create a real issue with `@gitclaw /soul`,
   - create a second real issue with `@gitclaw /soul list`,
   - create a third real issue with `@gitclaw /soul verify`,
   - create a fourth real issue with `@gitclaw /soul risk`,
   - create a fifth real issue with `@gitclaw /soul info soul`,
   - assert the reply is marked `model="gitclaw/soul"`,
   - assert the report lists loaded identity, policy, user, and memory paths
     with byte counts, line counts, and hashes,
   - assert the verify report includes repo-local source counts, required-file
     presence, soul frontmatter/description status, registry/profile export
     verification status, trust cards, and verification findings,
   - assert the risk report includes status/counts, risk cards, risk codes,
     line hashes, and `llm_e2e_required_after_soul_risk_change=true`,
   - assert the info report includes exactly one matched soul file, normalized
     path, category, source, loaded-for-turn state, short hash, and
     body-free/write-disabled flags,
   - assert soul validation status, required-file counts, memory-note counts,
     and noncanonical memory-note counts are present,
   - assert the report does not dump full soul or memory bodies,
   - assert the run succeeds without requiring a model provider response.

27. **Tools inspection**

   - create a real issue with `@gitclaw /tools`,
   - create a second real issue with `@gitclaw /tools list`,
   - create a third real issue with `@gitclaw /tools verify`,
   - create another real issue with `@gitclaw /tools risk`,
   - create another real issue with `@gitclaw /tools info read_file`,
   - create another real issue with `@gitclaw /tools run-plan search_files`,
   - ask for a concrete file read and search fixture phrase,
   - assert the reply is marked `model="gitclaw/tools"`,
   - assert the report lists available tool contracts and active output
     metadata for list/search/read,
   - assert the verify report includes contract modes, repo-local guidance
     provenance, known/unknown output counts, active-output hashes, raw input
     suppression, and verification findings,
   - assert the risk report includes contract/guidance/active-output risk
     cards, status/counts, risk codes, line hashes, raw input/output
     suppression, and `llm_e2e_required_after_tool_risk_change=true`,
   - assert the info report includes one contract, active-output hashes,
     validation scoped to the match, and no raw inputs,
   - assert the run-plan report includes one contract, gate state, active-output
     hashes, review steps, no shell/network/repository/model execution, and
     no raw inputs or outputs,
   - assert tool validation status, contract counts, active-output counts,
     unknown-output counts, unsafe-contract counts, and over-limit output
     counts, missing-guidance count, and duplicate-contract count are present,
   - assert the report does not dump full file or search output bodies,
   - assert the run succeeds without requiring a model provider response.

28. **Diff inspection**

   - create a real issue with `@gitclaw /diffs`,
   - assert the reply is marked `model="gitclaw/diffs"`,
   - assert the report lists `.gitclaw/DIFFS.md`, diff spec metadata, git
     availability, repository state, clean/dirty state, change counts, numstat
     totals, raw-diff suppression, and changed-file metadata,
   - assert policy/spec body tokens, raw patch hunks, file bodies, and issue
     body tokens are not printed,
   - assert the run succeeds without requiring a model provider response,
   - run a real GitHub Models conversation E2E in the same feature batch.

29. **Workspace inspection**

   - create a real issue with `@gitclaw /workspace`,
   - assert the reply is marked `model="gitclaw/workspace"`,
   - assert the report lists `.gitclaw/WORKSPACE.md`, workspace spec metadata,
     git repository state, repository inventory counts, context allowlist
     counts, workflow checkout/setup-go action versions, fetch-depth metadata,
     and private-memory/external-mount suppression,
   - assert policy/spec body tokens, workflow bodies, file bodies, and issue
     body tokens are not printed,
   - assert the run succeeds without requiring a model provider response,
   - run a real GitHub Models conversation E2E in the same feature batch.

30. **Policy inspection**

   - create a real issue with `@gitclaw /policy` that also asks for write-mode
     work,
   - create another real issue with `@gitclaw /policy verify` that asks for
     write-mode work,
   - assert the reply is marked `model="gitclaw/policy"`,
   - assert the report shows trusted actor state, write-request detection,
     managed labels, expected workflow permissions, and `gitclaw.policy`
     metadata,
   - assert the verify report checks actual workflow jobs and permissions,
     reports matched/missing permission counts, and flags no unexpected write
     permissions,
   - assert the report does not dump the issue body or policy output body,
   - assert `gitclaw:write-requested` and `gitclaw:done` are present without
     `gitclaw:running` or `gitclaw:error`.

31. **Session inspection**

   - create a real issue that gets one deterministic assistant turn,
   - post a follow-up comment with `@gitclaw /session`,
   - post a follow-up comment with `@gitclaw /session list`,
   - assert the reply is marked `model="gitclaw/session"`,
   - assert the report shows raw comment count, transcript message count,
     assistant-turn marker count, and per-message hashes,
   - assert the report does not dump issue or comment body tokens,
   - assert the run succeeds without requiring a model provider response.

32. **Backup index**

   - create a real deterministic GitClaw issue turn,
   - wait for the successful backup job,
   - assert the backup branch contains the issue JSON backup,
   - assert the repo-scoped `index.json` and `README.md` reference the issue
     number, title, and backup path,
   - assert the index contains metadata counts but not raw transcript bodies.

33. **Backup inspection**

   - create a real issue with `@gitclaw /backup`,
   - assert the reply is marked `model="gitclaw/backup"`,
   - assert the report lists the expected backup branch, issue backup path,
     index path, README path, and schema version,
   - wait for the successful backup job,
   - assert the backup branch contains the issue JSON backup and repo index
     entry for that same issue,
   - assert the report does not dump issue or comment body tokens.

34. **Backup verification**

   - create a real issue with `@gitclaw /backup verify`,
   - assert the issue-side report lists `requested_backup_command: verify`,
     `issue_side_execution: deferred_to_post_turn_backup_branch`, and the
     concrete local verify command without dumping body tokens,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup verify --root <fetched>/.gitclaw/backups --repo
     <owner/repo>`,
   - assert `backup_verify_status: ok`, zero verification failures, zero
     unindexed issue files, and an index entry for the just-created issue.

35. **Backup risk audit**

   - create a real issue with `@gitclaw /backup risk`,
   - assert the issue-side report lists `requested_backup_command: risk`,
     `backup_risk_status: deferred`, the deferred execution marker, the
     concrete local risk command, and no raw backup payload bodies,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup risk --root <fetched>/.gitclaw/backups --repo
     <owner/repo>`,
   - assert the local risk report lists verify status, indexed/scanned issue
     counts, scanned comment/transcript counts, risk counts, categories, and
     hashes only,
   - assert it does not dump the issue body token, raw comments, raw
     transcript messages, prompt text, or credential-looking values,
   - post a normal follow-up on the same issue that requires a repo-reader
     repository search and assert the second assistant turn is model-backed by
     GitHub Models with prompt context, selected skill, and prompt-visible tool
     markers.

36. **Backup manifest**

   - create a real issue with `@gitclaw /backup manifest`,
   - assert the issue-side report lists `requested_backup_command: manifest`,
     `issue_side_execution: deferred_to_post_turn_backup_branch`, and the
     concrete local manifest command without dumping body/title tokens,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup manifest --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --issue <issue-number>`,
   - assert the manifest lists index/README control file hashes plus the
     just-created issue payload path, bytes, hash, schema, event, comment
     count, and transcript count,
   - assert it does not dump the issue body token or raw transcript bodies.

37. **Backup stats**

   - create a real issue with `@gitclaw /backup stats`,
   - assert the issue-side report lists `requested_backup_command: stats`,
     the deferred execution marker, and the concrete local stats command
     without dumping body/title tokens,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup stats --root <fetched>/.gitclaw/backups --repo
     <owner/repo>`,
   - assert the report is marked `backup_stats_status: ok` and
     `backup_verify_status: ok`,
   - assert it lists aggregate issue/comment/message counts, latest backup
     metadata, event counts, and payload bytes,
   - assert it does not dump the issue body token or raw title.

38. **Backup list**

   - create a real issue with `@gitclaw /backup list`,
   - assert the issue-side report lists `requested_backup_command: list`,
     the deferred execution marker, and the concrete local list command without
     dumping body/title tokens,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup list --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --limit 5`,
   - assert the report is marked `backup_list_status: ok` and
     `backup_verify_status: ok`,
   - assert it lists the just-created issue number, canonical path, timestamp,
     event name, label/comment/transcript counts, and title hash,
   - assert it does not dump the issue body token or raw title.

39. **Backup info**

   - create a real issue with `@gitclaw /backup info`,
   - assert the issue-side report lists `requested_backup_command: info`, the
     deferred execution marker, and the concrete local info command for the
     current issue without dumping body/title tokens,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup info --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --issue <issue-number>`,
   - assert the report is marked `backup_info_status: ok` and
     `backup_verify_status: ok`,
   - assert it lists the canonical payload path, payload hash, event name,
     label/comment/transcript counts, assistant-turn/error counts, and body
     hashes,
   - assert it does not dump the issue body token or raw title.

40. **Backup JSONL export**

   - create a real issue with `@gitclaw /backup export-jsonl`,
   - assert the issue-side report lists `requested_backup_command:
     export-jsonl`, the deferred execution marker, and the concrete local
     export command without dumping body/title tokens,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup export-jsonl --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --issue <issue-number>`,
   - assert the JSONL contains exactly the new issue transcript records,
   - assert the first record contains the issue body token and the second record
     contains the assistant backup report body, proving the command is an
     explicit raw recovery/export path rather than an issue-visible report.

41. **Backup restore plan**

   - create a real issue with `@gitclaw /backup restore-plan`,
   - assert the issue-side report lists `requested_backup_command:
     restore-plan`, the deferred execution marker, and the concrete local
     restore-plan command without dumping body/title tokens,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup restore-plan --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --issue <issue-number>`,
   - assert the report is marked `restore_mode: dry-run`,
   - assert it lists backup path, schema version, label/comment/transcript
     counts, assistant-turn/error counts, and body hashes,
   - assert it does not dump the issue body token or raw transcript bodies.

42. **Backup retention plan**

   - create a real issue with `@gitclaw /backup retention-plan`,
   - assert the issue-side report lists `requested_backup_command:
     retention-plan`, the deferred execution marker, and the concrete local
     retention-plan command without dumping body/title tokens,
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

43. **Backup search**

   - create a real issue with `@gitclaw /backup search <query>`,
   - include a unique hidden token in the issue body,
   - assert the issue-side report lists `requested_backup_command: search`,
     the concrete local search command with `<query>`, a query hash, a query
     term count, and no raw query text,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup search --root <fetched>/.gitclaw/backups --repo
     <owner/repo> <hidden-token>`,
   - assert the report is marked `backup_search_status: ok` and
     `backup_verify_status: ok`,
   - assert it lists query hash, issue/search counts, the just-created issue,
     source metadata, scores, and hashes,
   - assert it does not dump the hidden token, raw issue body, raw issue title,
     raw comments, raw transcript messages, or raw query text.

44. **Proactive init generator**

   - run `gitclaw proactive init` against a temporary repo root,
   - assert it writes the expected prompt file and scheduled workflow,
   - assert the init report includes hashes and file status but not the prompt
     body token,
   - lint the generated workflow when `actionlint` is available,
   - dispatch the real generic proactive workflow with the generated job name
     and a `/proactive` prompt body,
   - assert it creates a real proactive issue and receives one deterministic
     proactive report without leaking the hidden prompt token.

45. **Proactive info report**

   - create a real issue with `@gitclaw /proactive info repo-hygiene`,
   - include a unique hidden token in the issue body,
   - wait for the issue-opened workflow,
   - assert the assistant posts exactly one `GitClaw Proactive Info Report`
     with `proactive_info_status: ok`,
   - assert the report lists prompt, generic workflow, generated workflow
     candidate, trigger metadata, and enqueue command hashes/paths,
   - assert no issue body, prompt body, or workflow body content is leaked.

46. **Proactive risk report with model follow-up**

   - create a real issue with `@gitclaw /proactive risk`,
   - include a unique hidden token in the issue body,
   - wait for the issue-opened workflow,
   - assert the assistant posts exactly one deterministic
     `GitClaw Proactive Risk Report` with `proactive_risk_status: ok`,
   - assert the report lists workflow trigger/permission metadata, prompt
     skill hints, risk counts, risk codes, and body-inclusion flags,
   - assert no issue body, proactive prompt body, workflow body, or hidden
     token content is leaked,
   - add a normal follow-up comment that asks the agent to use repo search,
   - wait for the issue-comment workflow and assert the second assistant
     comment used GitHub Models, records prompt context provenance, selects
     `repo-reader`, exposes `gitclaw.search_files`, and returns the expected
     search token without leaking hidden issue/comment tokens.

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
  `gitclaw.search_files` context from the search fixture with a distinct
  search-token prefix plus redacted prompt-artifact proof,
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
- the backup-search harness verifies a fetched backup branch can search actual
  backed-up conversation content and return only paths, sources, trust
  metadata, scores, and hashes without leaking the searched token or bodies,
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
- A `gh`-driven heartbeat-report E2E harness verifies `@gitclaw /heartbeat`
  reports workflow triggers, permissions, heartbeat context metadata, label
  state, marker counts, and the runner/model-call contract without a model call
  or body leakage. Each heartbeat-report implementation batch must still run a
  live GitHub Models conversation E2E that makes an actual LLM call.
- A `gh`-driven workflow-dispatch E2E harness verifies the main handler can be
  woken for a specific issue and deduped by dispatch ID.
- A `gh`-driven channel-message E2E harness verifies a mirrored channel
  comment is included in the dispatched conversation transcript.
- A `gh`-driven channel-ingest E2E harness verifies the generic channel ingress
  workflow end to end, including duplicate provider-message retries.
- A `gh`-driven channel-state E2E harness verifies real GitHub issue-backed
  channel offset storage, duplicate offset suppression, `gitclaw:channel`
  labeling, and no raw account/offset leakage.
- A `gh`-driven channel-state-workflow E2E harness dispatches
  `.github/workflows/gitclaw-channel-state.yml`, verifies the state issue and
  update comment, then dispatches the same offset again to prove retry
  idempotency in GitHub Actions.
- A `gh`-driven channel-gateway-workflow E2E harness dispatches
  `.github/workflows/gitclaw-channel-gateway.yml`, verifies the gateway lease is
  persisted through channel-state hashes, then repeats the same lease to prove
  duplicate gateway runs are idempotent.
- A `gh`-driven channel-delivery-workflow E2E harness dispatches
  `.github/workflows/gitclaw-channel-delivery.yml`, verifies a source
  `gitclaw:assistant-turn` comment can be recorded as delivered, checks that
  only hashes are stored for channel account/provider message IDs, and repeats
  the same delivery to prove outbound idempotency.
- A `gh`-driven channels-report E2E harness verifies `@gitclaw /channels`
  reports workflow dispatch, channel labels, provider keys, and mirrored
  message marker counts without a model call.
- A `gh`-driven channels-list E2E harness verifies `@gitclaw /channels list`
  is an explicit report alias, while local `gitclaw channels list` exposes the
  same bridge contract without issue-only fields.
- A `gh`-driven channels-verify E2E harness verifies
  `@gitclaw /channels verify` reports the workflow-dispatch channel bridge
  health, permissions, required inputs, provider keys, and marker counts
  without a model call or body leakage.
- A `gh`-driven channels-risk E2E harness verifies
  `@gitclaw /channels risk` reports provider, workflow, and channel-message
  risk cards with only counts, hashes, codes, and severities, then posts a
  normal follow-up that requires repo-reader search so GitHub Models performs a
  real LLM call with prompt context and prompt-visible tool provenance.
- A `gh`-driven channels-info E2E harness verifies
  `@gitclaw /channels info <provider>` and local
  `gitclaw channels info <provider>` expose one provider's secret names,
  offset/thread/message keys, workflow metadata, gateway strategy, and command
  shapes without a model call or body/credential leakage.
- A `gh`-driven proactive E2E harness verifies the generic proactive enqueue
  workflow end to end.
- A `gh`-driven proactive-init E2E harness verifies
  `gitclaw proactive init` generates a scheduled workflow and prompt file
  without leaking prompt bodies, then dispatches a real proactive conversation.
- A `gh`-driven proactive-report E2E harness verifies `@gitclaw /proactive`
  reports workflow triggers and prompt metadata without a model call.
- A `gh`-driven proactive-list E2E harness verifies `@gitclaw /proactive list`
  is an explicit report alias, while local `gitclaw proactive list` exposes
  workflow and prompt-file metadata without issue-only fields or prompt bodies.
- A `gh`-driven proactive-info E2E harness verifies
  `@gitclaw /proactive info <name>` and local `gitclaw proactive info <name>`
  expose one proactive job definition, generic workflow metadata, generated
  workflow candidate metadata, and enqueue command shape without a model call
  or body leakage.
- A `gh`-driven proactive-risk E2E harness verifies
  `@gitclaw /proactive risk` and local `gitclaw proactive risk` expose
  body-free workflow/prompt risk metadata, then runs a real GitHub Models
  follow-up conversation that proves model inference, prompt provenance,
  selected skills, and prompt-visible tool usage.
- A `gh`-driven model-report E2E harness verifies `@gitclaw /models` reports
  GitHub Models provider and retry settings without a model call.
- A `gh`-driven models-list E2E harness verifies `@gitclaw /models list` is
  an explicit report alias, while local `gitclaw models list` exposes the same
  provider wiring without issue-only fields.
- A `gh`-driven config-report E2E harness verifies `@gitclaw /config` reports
  effective labels, prompt budgets, commands, and workflow metadata without a
  model call.
- A `gh`-driven config-list E2E harness verifies `@gitclaw /config list` is an
  explicit report alias, while local `gitclaw config list` exposes the same
  control-plane metadata without issue-only fields.
- A `gh`-driven commands-report E2E harness verifies `@gitclaw /help` reports
  deterministic commands, aliases, and every advertised local CLI helper
  without a model call or issue-body leakage.
- A `gh`-driven orders-report E2E harness verifies `@gitclaw /orders`
  reports standing-order file metadata, model-context loading, program clause
  coverage, proactive enforcement metadata, and body-free findings without a
  model call or standing-order body leakage. Each standing-orders feature batch
  must still run a live GitHub Models conversation E2E.
- A `gh`-driven hooks-report E2E harness verifies `@gitclaw /hooks` reports
  hook policy metadata, model-context loading, declarative hook spec metadata,
  approval/audit-only gates, ignored executable handler state, and body-free
  findings without a model call or hook body leakage. Each hooks feature batch
  must still run a live GitHub Models conversation E2E.
- A `gh`-driven plugins-report E2E harness verifies `@gitclaw /plugins`
  reports plugin policy metadata, model-context loading, declarative plugin
  spec metadata, metadata-only activation, approval gates, ignored package or
  runtime file state, MCP/plugin execution boundaries, and body-free findings
  without a model call or plugin body leakage. Each plugins feature batch must
  still run a live GitHub Models conversation E2E.
- A `gh`-driven tasks-report E2E harness verifies `@gitclaw /tasks` reports
  task policy metadata, model-context loading, declarative task/flow spec
  metadata, issue-native status/label mapping, current issue task status,
  comment/transcript counts, Task Flow/Kanban execution boundaries, and
  body-free findings without a model call or task body leakage. Each tasks
  feature batch must still run a live GitHub Models conversation E2E.
- A `gh`-driven agents-report E2E harness verifies `@gitclaw /agents` reports
  agent policy metadata, model-context loading, declarative agent spec
  metadata, single-assistant GitHub Actions runtime boundaries, no-delegation
  gates, and body-free findings without a model call or agent body leakage.
  Each agents feature batch must still run a live GitHub Models conversation
  E2E that makes an actual LLM call; report-only coverage is not enough.
- A `gh`-driven nodes-report E2E harness verifies `@gitclaw /nodes` reports
  node policy metadata, model-context loading, declarative node spec metadata,
  GitHub Actions ephemeral-job boundaries, no-WebSocket/no-pairing/no-remote-exec
  gates, and body-free findings without a model call or node body leakage.
  Each nodes feature batch must still run a live GitHub Models conversation E2E
  that makes an actual LLM call; report-only coverage is not enough.
- A `gh`-driven runs-report E2E harness verifies `@gitclaw /runs` reports
  current turn/run provenance, managed labels, marker counts, prompt-visible
  input hashes, and active tool-output hashes without a model call or body
  leakage. Each run-ledger feature batch must still run at least one
  LLM-backed GitHub Models conversation E2E in addition to this deterministic
  report.
- A `gh`-driven doctor-report E2E harness verifies `@gitclaw /doctor` reports
  config validation, workflow presence, context files, skills, memory notes,
  proactive prompts, and skill/soul/tool validation rollups without a model
  call.
- A `gh`-driven doctor-list E2E harness verifies `@gitclaw /doctor list` is an
  explicit report alias, while local `gitclaw doctor list` exposes the same
  body-free health metadata without issue-only fields.
- A `gh`-driven backup-index E2E harness verifies the dedicated backup branch
  contains issue JSON plus a repo-scoped `index.json` and `README.md`.
- A `gh`-driven backup-report E2E harness verifies `@gitclaw /backup`
  publishes deterministic backup paths without a model call and that the
  backup branch receives the corresponding issue JSON and index entry.
- A `gh`-driven backup-verify E2E harness verifies `@gitclaw /backup verify`
  records the deferred issue-side command intent, then verifies the fetched
  `gitclaw-backups` branch with `gitclaw backup verify` after the real backup
  job succeeds.
- A `gh`-driven backup-risk E2E harness verifies `@gitclaw /backup risk`
  records the deferred issue-side command intent, then scans the fetched
  `gitclaw-backups` branch with `gitclaw backup risk` for integrity,
  path-safety, credential-handling, prompt-boundary, restore-safety, and
  retention findings without dumping raw payloads. It also posts a normal
  follow-up that must use GitHub Models and repo-reader search.
- A `gh`-driven backup-manifest E2E harness verifies
  `@gitclaw /backup manifest` records the deferred issue-side command intent,
  then verifies the fetched `gitclaw-backups` branch can produce a file-level
  manifest with control-file and issue-payload hashes for one real issue,
  without dumping raw bodies.
- A `gh`-driven backup-stats E2E harness verifies
  `@gitclaw /backup stats` records the deferred issue-side command intent,
  then verifies the fetched `gitclaw-backups` branch can produce a repo-wide
  backup stats report with verification status, aggregate counts, latest backup
  metadata, and event counts, without dumping raw bodies or titles.
- A `gh`-driven backup-list E2E harness verifies
  `@gitclaw /backup list` records the deferred issue-side command intent, then
  verifies the fetched `gitclaw-backups` branch can produce a timestamp-sorted
  indexed backup list with paths, counts, event names, and title hashes,
  without dumping raw bodies or titles.
- A `gh`-driven backup-info E2E harness verifies
  `@gitclaw /backup info` records the deferred issue-side command intent, then
  verifies the fetched `gitclaw-backups` branch can produce a focused
  single-issue backup info card with payload hashes, counts, marker totals, and
  body hashes, without dumping raw bodies or titles.
- A `gh`-driven backup-export-jsonl E2E harness verifies
  `@gitclaw /backup export-jsonl` records the deferred issue-side command
  intent, then verifies the fetched `gitclaw-backups` branch can be exported
  into raw JSONL transcript records for one real issue.
- A `gh`-driven backup-restore-plan E2E harness verifies
  `@gitclaw /backup restore-plan` records the deferred issue-side command
  intent, then verifies the fetched `gitclaw-backups` branch can produce a
  dry-run restore plan for one real issue without dumping raw bodies.
- A `gh`-driven backup-retention-plan E2E harness verifies
  `@gitclaw /backup retention-plan` records the deferred issue-side command
  intent, then verifies the fetched `gitclaw-backups` branch can produce a
  dry-run keep-latest retention plan with kept/prune-candidate metadata and
  hashes, without dumping raw titles or bodies.
- A `gh`-driven context-report E2E harness verifies `@gitclaw /context`
  produces a deterministic context summary without a model call.
- A `gh`-driven context-list E2E harness verifies `@gitclaw /context list` is
  an explicit report alias, while local `gitclaw context list` exposes the same
  body-free repository context metadata without issue-only fields.
- A `gh`-driven context-info E2E harness verifies `@gitclaw /context info
  .gitclaw/SOUL.md` returns exactly one focused, body-free context card, while
  local `gitclaw context info <path>` covers both loaded context documents and
  repo files surfaced through deterministic `gitclaw.read_file` metadata.
- A `gh`-driven context-references E2E harness verifies
  `@gitclaw /context references` reports `@file:` line ranges and `@folder:`
  metadata without dumping referenced bodies, issue text, or fixture tokens.
- A `gh`-driven context-reference chat E2E harness verifies a normal model turn
  can answer from an explicit `@file:` reference while ignoring a hidden issue
  token.
- A `gh`-driven search-tool chat E2E harness verifies a normal model turn can
  recover a token from `gitclaw.search_files` output without explicit `@file`
  or `@folder` context references. This keeps the live E2E suite honest about
  actual tool-context usage, not just deterministic reports.
- A `gh`-driven git-reference report E2E harness verifies
  `@gitclaw /context ... @git:1` reports body-free commit-reference metadata
  without dumping commit subjects, patches, or issue text.
- A `gh`-driven git-reference chat E2E harness verifies a normal model turn can
  answer from an explicit `@git:1` reference by copying the latest commit hash.
- A `gh`-driven prompt-report E2E harness verifies `@gitclaw /prompt`
  produces a deterministic prompt budget, hash, truncation, context, and tool
  metadata report without a model call or prompt/body leakage.
- A `gh`-driven prompt-list E2E harness verifies `@gitclaw /prompt list` is an
  explicit report alias, while local `gitclaw prompt list` exposes the same
  body-free prompt-budget, prompt-input, context, skill, and tool metadata
  without issue-only fields.
- A `gh`-driven memory-report E2E harness verifies `@gitclaw /memory`
  produces a deterministic memory inventory without a model call or body
  leakage.
- A `gh`-driven memory-list E2E harness verifies `@gitclaw /memory list`
  is an explicit inventory alias with the same body-free memory-file,
  loaded-note, hash, and validation metadata.
- A `gh`-driven memory-info E2E harness verifies `@gitclaw /memory info
  latest` returns one focused body-free memory file card with normalized path,
  kind/source/canonical/latest/loaded metadata, and hashes.
- A `gh`-driven memory-search E2E harness verifies
  `@gitclaw /memory search backup branch` searches git-backed memory files
  without a model call, raw query leakage, or memory-body leakage.
- A `gh`-driven memory-risk E2E harness verifies `@gitclaw /memory risk`
  reports durable-state risk counts, codes, memory write boundaries,
  external-provider non-goals, and line hashes without a model call or
  memory-body leakage. It also posts a normal follow-up that must use GitHub
  Models and repo-reader search.
- A `gh`-driven memory-promote-plan E2E harness verifies
  `@gitclaw /memory promote-plan long-term` produces a body-free,
  non-mutating reviewed-memory promotion plan with model calls, candidate
  generation, transcript dumping, memory-body dumping, and memory writes
  disabled. This deterministic check must be paired in the same implementation
  batch with a live GitHub Models conversation E2E that makes an actual LLM
  call.
- A `gh`-driven memory-validate E2E harness verifies
  `@gitclaw /memory validate` reports memory hygiene without a model call or
  memory-body leakage.
- A `gh`-driven memory-verify E2E harness verifies
  `@gitclaw /memory verify` reports the body-free repo-local memory trust
  envelope, loaded-state metadata, hashes, and explicit external memory
  verification non-goals without a model call.
- A `gh`-driven skills-report E2E harness verifies `@gitclaw /skills`
  produces a deterministic local skill inventory with provenance and
  requirement, config-gate, and validation metadata, without a model call.
- A `gh`-driven skills-list E2E harness verifies `@gitclaw /skills list`
  is an explicit inventory alias with the same body-free skill metadata and
  selected-skill provenance, including enabled/disabled/allowlist-blocked
  counts.
- A `gh`-driven skills-verify E2E harness verifies
  `@gitclaw /skills verify` exposes the repo-local skill trust envelope,
  hashes, config-gate state, requirement status, and no-registry boundary
  without body leakage.
- A `gh`-driven skills-risk E2E harness verifies
  `@gitclaw /skills risk` exposes body-free risky-instruction category counts,
  finding codes, skill hashes, and line hashes without a model call, and then
  runs a live GitHub Models follow-up conversation that proves repo-local skill
  selection and tool usage still work on the real model path.
- A `gh`-driven skills-validate E2E harness verifies
  `@gitclaw /skills validate` and the OpenClaw-style
  `@gitclaw /skills check` alias expose the body-free validation report
  without falling back to the full inventory.
- A `gh`-driven skills-info E2E harness verifies
  `@gitclaw /skills info repo-reader` produces focused skill metadata without
  a model call or full `SKILL.md` body leakage.
- A `gh`-driven skills-select-plan E2E harness verifies
  `@gitclaw /skills select-plan repo-reader` explains selected-for-turn state,
  gate metadata, selection reasons, validation status, and the live-LLM E2E
  rule without a model call, raw request text, or full `SKILL.md` body leakage.
- A `gh`-driven skills-install-plan E2E harness verifies
  `@gitclaw /skills install-plan repo-reader` produces a body-free,
  non-mutating install/upgrade plan with remote fetches, installer scripts,
  dependency installs, repository mutations, raw targets, manifests, and skill
  bodies all disabled. This deterministic check must be paired in the same
  implementation batch with a live GitHub Models conversation E2E.
- A `gh`-driven skill-bundle info E2E harness verifies
  `@gitclaw /bundles info repo-context` produces focused repo-local bundle
  metadata, resolved/missing skill refs, instruction presence, hashes, and no
  bundle instruction or skill body leakage.
- A `gh`-driven skills-search E2E harness verifies
  `@gitclaw /skills search repository context` searches local skill metadata
  without a model call, raw query leakage, or full `SKILL.md` body leakage.
- A `gh`-driven profile-report E2E harness verifies `@gitclaw /profile`
  produces a deterministic repo-local profile envelope across identity,
  memory, skills, tools, model, and validation state without a model call or
  profile-body leakage.
- A `gh`-driven migration-plan E2E harness verifies
  `@gitclaw /migrate plan hermes` produces a body-free, non-mutating import
  plan for OpenClaw/Hermes/Codex/Claude-style state, with source scanning,
  credential import, executable-state import, repository mutation, and model
  calls disabled. This deterministic check must be paired in the same
  implementation batch with a live GitHub Models conversation E2E that makes
  an actual LLM call, such as `github-search-tool-chat.sh`.
- A `gh`-driven soul-report E2E harness verifies `@gitclaw /soul` produces a
  deterministic high-authority context file audit with validation metadata,
  without a model call or body leakage.
- A `gh`-driven soul-list E2E harness verifies `@gitclaw /soul list` is an
  explicit inventory alias with the same body-free context file, memory-note,
  hash, and validation metadata.
- A `gh`-driven soul-info E2E harness verifies `@gitclaw /soul info soul`
  exposes one body-free high-authority context metadata card with normalized
  path, category, repo-local source, selected-for-turn state, hashes, and
  write-disabled metadata.
- A `gh`-driven soul-edit-plan E2E harness verifies
  `@gitclaw /soul edit-plan soul` produces a body-free, non-mutating plan for
  a high-authority context change with edit operations, branch creation,
  commits, pushes, model self-modification, raw targets, raw requested changes,
  and raw context bodies disabled. This deterministic check must be paired in
  the same implementation batch with a live GitHub Models conversation E2E.
- A `gh`-driven soul-validate E2E harness verifies
  `@gitclaw /soul validate` exposes the body-free validation report without
  falling back to the full context inventory.
- A `gh`-driven soul-verify E2E harness verifies
  `@gitclaw /soul verify` exposes the body-free repo-local trust envelope,
  trust cards, hashes, required-file coverage, and explicit registry/profile
  verification non-goals without a model call or context-body leakage.
- A `gh`-driven soul-risk E2E harness verifies `@gitclaw /soul risk` exposes
  body-free persistent-state risk status, risk cards, codes, line hashes, and
  the live-LLM E2E requirement. The same harness must then post a normal
  follow-up comment that requires repo-reader search so GitHub Models performs
  a real LLM call with prompt context, skill selection, and prompt-visible tool
  provenance in the assistant marker.
- A `gh`-driven tools-report E2E harness verifies `@gitclaw /tools` produces a
  deterministic tool contract and active-output audit with validation metadata,
  repo-reviewed tool-gate metadata, without a model call or output-body
  leakage.
- A `gh`-driven tools-list E2E harness verifies `@gitclaw /tools list`
  is an explicit inventory alias with the same body-free tool contract,
  active-output, guidance, gate-state, and validation metadata.
- A `gh`-driven tools-validate E2E harness verifies
  `@gitclaw /tools validate` exposes the body-free validation report without
  falling back to the full inventory.
- A `gh`-driven tools-verify E2E harness verifies
  `@gitclaw /tools verify` exposes the body-free deterministic tool trust
  envelope, contract modes, gate-state metadata, guidance provenance,
  active-output hashes, raw input suppression, and verification findings
  without a model call.
- A `gh`-driven tools-risk E2E harness verifies `@gitclaw /tools risk`
  exposes body-free contract, guidance, active-output, and active-input risk
  status, cards, codes, hashes, and the live-LLM E2E requirement. The same
  harness must then post a normal follow-up comment that requires repo-reader
  search so GitHub Models performs a real LLM call with prompt context, skill
  selection, and prompt-visible tool provenance in the assistant marker.
- A `gh`-driven tools-info E2E harness verifies
  `@gitclaw /tools info read_file` exposes one body-free tool contract card,
  active-output hashes, and match-scoped validation without raw inputs or
  output-body leakage.
- A `gh`-driven tools-run-plan E2E harness verifies
  `@gitclaw /tools run-plan search_files` exposes one body-free dry-run plan,
  gate-state metadata, active-output hashes, no shell/network/repository/model
  execution flags, and explicit reminder coverage that tool-behavior changes
  must also run a live GitHub Models conversation E2E.
- A `gh`-driven sandbox-report E2E harness verifies `@gitclaw /sandbox`
  exposes the current GitHub Actions runtime boundary, denied host exec,
  read-only tool modes, workflow permission cards, and backup-job-only write
  scope without a model call or body leakage.
- A `gh`-driven policy-report E2E harness verifies `@gitclaw /policy` produces
  a deterministic preflight/label/write-policy audit without a model call or
  issue-body leakage.
- A `gh`-driven policy-list E2E harness verifies `@gitclaw /policy list` is an
  explicit report alias, while local `gitclaw policy list` exposes static
  policy metadata without issue-only fields.
- A `gh`-driven policy-verify E2E harness verifies `@gitclaw /policy verify`
  checks the checked-in workflow permission contract, reports active policy
  output hashes, and avoids raw policy input/output leakage.
- A `gh`-driven approvals-report E2E harness verifies
  `@gitclaw /approvals` detects real write intent, observes
  `gitclaw:approved`, reports the approval gates as read-only, applies
  `gitclaw:write-requested`, and avoids issue body leakage.
- A `gh`-driven secrets-report E2E harness verifies
  `@gitclaw /secrets audit` scans the real checked-out repository, reports
  plaintext-like findings and GitHub Actions secret references with path, line,
  code, count, and hash metadata only, and does not leak matched values, issue
  body tokens, or referenced secret names.
- A `gh`-driven checkpoints-report E2E harness verifies
  `@gitclaw /rollback` inspects the real checked-out repository's git
  checkpoint state, reports HEAD/worktree/backup-branch metadata, and does not
  leak issue body text, diffs, file bodies, commit subjects, or perform restore
  operations.
- A `gh`-driven diffs-report E2E harness verifies `@gitclaw /diffs` inspects
  the real checked-out repository's git change metadata, reports policy/spec
  state, clean/dirty state, changed-file counts, numstat totals, and raw-patch
  suppression without leaking issue body text, patch hunks, or file bodies.
- A `gh`-driven workspace-report E2E harness verifies `@gitclaw /workspace`
  inspects the real GitHub Actions checkout, reports policy/spec metadata,
  git repository state, context counts, checkout/setup-go action versions,
  fetch-depth metadata, and private-memory/external-mount suppression without
  leaking issue body text, workflow bodies, or file bodies.
- A `gh`-driven session-report E2E harness verifies `@gitclaw /session`
  reconstructs a real multi-turn GitHub issue session without a model call or
  transcript-body leakage.
- A `gh`-driven session-list E2E harness verifies `@gitclaw /session list` is
  an explicit report alias, while local
  `gitclaw session list --backup <issue.json>` inspects a backed-up issue
  session without dumping raw issue, comment, assistant, or transcript bodies.
- Local `gitclaw session search <query> --backup <issue.json>` searches the
  same backed-up issue transcript and reports only query hashes, counts,
  sources, trust metadata, scores, and message/line hashes.
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
2. Should the default move from `openai/gpt-5-nano` to
   `openai/gpt-5.4-mini` or a newer small model once the GitHub Models catalog
   exposes that ID to Actions?
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
- GitHub Actions artifact storage docs: https://docs.github.com/en/actions/how-tos/writing-workflows/choosing-what-your-workflow-does/storing-and-sharing-data-from-a-workflow
- `actions/upload-artifact` action: https://github.com/actions/upload-artifact
- GitHub workflow dispatch REST API: https://docs.github.com/en/rest/actions/workflows#create-a-workflow-dispatch-event
- GitHub Models quickstart: https://docs.github.com/en/github-models/quickstart
- GitHub Models catalog REST API: https://docs.github.com/en/rest/models/catalog
- GitHub Models REST inference API: https://docs.github.com/en/rest/models/inference
- GitHub Models for Actions issue summaries: https://docs.github.com/en/github-models/github-models-at-scale/use-models-at-scale
- GitHub Models billing and rate-limit notes: https://docs.github.com/en/billing/concepts/product-billing/github-models
- GitHub Models `models:read` changelog: https://github.blog/changelog/2025-05-15-modelsread-now-required-for-github-models-access/
- OpenClaw secrets CLI docs: https://docs.openclaw.ai/cli/secrets
- OpenClaw secrets management docs: https://docs.openclaw.ai/gateway/secrets
- OpenClaw heartbeat docs: https://openclawlab.com/en/docs/agent/heartbeat/
- OpenClaw automation docs: https://docs.openclaw.ai/automation/index
- OpenClaw scheduled tasks docs: https://docs.openclaw.ai/automation/cron-jobs
- OpenClaw standing orders docs: https://docs.openclaw.ai/automation/standing-orders
- OpenClaw hooks docs: https://docs.openclaw.ai/automation/hooks
- OpenClaw background tasks docs: https://docs.openclaw.ai/automation/tasks
- OpenClaw Task Flow docs: https://docs.openclaw.ai/automation/taskflow
- OpenClaw tasks CLI docs: https://docs.openclaw.ai/cli/tasks
- OpenClaw capabilities overview: https://docs.openclaw.ai/tools
- OpenClaw building plugins docs: https://docs.openclaw.ai/plugins/building-plugins
- OpenClaw memory docs: https://docs.openclaw.ai/concepts/memory
- OpenClaw agent workspace docs: https://docs.openclaw.ai/agent-workspace
- OpenClaw creating skills docs: https://docs.openclaw.ai/tools/creating-skills
- OpenClaw skill format docs: https://docs.openclaw.ai/clawhub/skill-format
- OpenClaw models CLI docs: https://docs.openclaw.ai/cli/models
- OpenClaw node host CLI docs: https://docs.openclaw.ai/cli/node
- OpenClaw multi-agent routing docs: https://docs.openclaw.ai/concepts/multi-agent
- OpenClaw nodes CLI docs: https://docs.openclaw.ai/cli/nodes
- OpenClaw diffs plugin docs: https://docs.openclaw.ai/vi/tools/diffs
- OpenClaw config CLI docs: https://docs.openclaw.ai/cli/config
- OpenClaw configure docs: https://docs.openclaw.ai/cli/configure
- OpenClaw doctor docs: https://docs.openclaw.ai/doctor
- OpenClaw backup docs: https://docs.openclaw.ai/cli/backup
- OpenClaw exec approvals docs: https://docs.openclaw.ai/tools/exec-approvals
- OpenClaw sandboxing docs: https://docs.openclaw.ai/gateway/sandboxing
- Hermes memory docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/memory.md
- Hermes checkpoints and rollback docs: https://hermes-agent.nousresearch.com/docs/user-guide/checkpoints-and-rollback
- Hermes git worktrees docs: https://hermes-agent.nousresearch.com/docs/user-guide/git-worktrees
- Hermes cron docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/cron.md
- Hermes cron internals docs: https://hermes-agent.nousresearch.com/docs/developer-guide/cron-internals
- Hermes security docs: https://hermes-agent.nousresearch.com/docs/user-guide/security
- Hermes working with skills docs: https://hermes-agent.nousresearch.com/docs/guides/work-with-skills/
- Hermes profiles docs: https://hermes-agent.nousresearch.com/docs/user-guide/profiles
- Hermes Kanban docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/kanban
- Hermes subagent delegation docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/delegation
- Hermes toolsets reference: https://hermes-agent.nousresearch.com/docs/reference/toolsets-reference
- Hermes MCP docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp
- Slack Socket Mode: https://api.slack.com/apis/connections/socket
- Slack Events API: https://docs.slack.dev/apis/events-api/
- Telegram Bot API `getUpdates`: https://core.telegram.org/bots/api#getupdates
