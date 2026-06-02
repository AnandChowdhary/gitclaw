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
- **Per-repo assistant mode:** only issues with the label `gitclaw`, the title/body prefix `@gitclaw`, or both trigger the agent.

Default for public repositories should be per-repo assistant mode with
`trigger.mode: label-or-prefix`, requiring either the trigger label or prefix.
Shared repositories that want tighter routing can choose `label-only` or
`prefix-only`; dedicated assistant inbox repositories can choose `inbox`.
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
- include a hidden `gitclaw:heartbeat` marker with run id, run URL,
  idempotency slot, selected model, prompt-context hash, prompt-visible
  context counts, and normalized token usage.

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
gitclaw heartbeat risk
```

This report is intentionally not the heartbeat runner. It runs after preflight
and before model inference, posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/heartbeat"`, and summarizes:

- the heartbeat label, disabled label, and trigger label,
- `.github/workflows/gitclaw-heartbeat.yml` presence, schedule trigger,
  workflow-dispatch trigger, inputs, and permissions,
- `.gitclaw/HEARTBEAT.md` presence and hash,
- heartbeat marker/idempotency contract,
- heartbeat marker model, prompt provenance, and usage telemetry contract,
- the quiet response contract, `HEARTBEAT_OK`,
- whether the current issue has the heartbeat label,
- existing heartbeat marker count for the current issue.

It never scans heartbeat issues, calls the model, mutates repository contents,
or prints issue/comment/workflow/heartbeat context bodies. The report carries
`model_call_required: false` and `runner_model_call_required: true` so E2E can
distinguish the operator report from the real scheduled model-backed runner.
Heartbeat marker changes also carry
`llm_e2e_required_after_heartbeat_marker_change: true` and must be proven by
the live heartbeat workflow-dispatch harness. The heartbeat report E2E must
also pair this deterministic, body-free inventory with a normal GitHub Models
issue-comment follow-up that selects `repo-reader`, exposes
`gitclaw.search_files`, and recovers a bounded repository-search fixture token,
so report changes do not drift away from ordinary conversation behavior.

When called as `@gitclaw /heartbeat risk` or `gitclaw heartbeat risk`, GitClaw
posts a body-free risk audit for the scheduled heartbeat surface. It scans the
heartbeat workflow and `.gitclaw/HEARTBEAT.md` for schedule-amplification,
top-of-hour reliability, missing `workflow_dispatch`, missing `schedule`,
missing or excessive permissions, missing concurrency/idempotency guards,
prompt-boundary overrides, credential exfiltration, unreviewed persistent-state
mutation, workflow self-dispatch loops, and raw-input/body leakage. The report
emits only metadata, counts, hashes, risk codes, severities, and runtime gates;
it never prints workflow bodies, heartbeat context bodies, issue bodies,
comments, raw inputs, credentials, or secret values. Any implementation batch
touching this risk surface must also run a live GitHub Models follow-up that
proves normal LLM inference and repository tool use still work.

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
  --not-before 2026-05-29T08:17:00Z \
  --notify-route team-ops
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
status, skill-hint counts, byte counts, hashes, and
`llm_e2e_required_after_proactive_init_change=true`. Changes to the generator
must be paired with a live GitHub workflow-dispatch run that creates a
proactive issue, posts the deterministic proactive report, then continues with
a normal GitHub Models follow-up using `repo-reader` and bounded repository
search. `--skill <name>` can be passed more than once, and `--skills a,b` is
accepted for comma-separated skill hints. The generated prompt file records
the hints in a `gitclaw:proactive-skills` marker and a short
"Suggested GitClaw skills" section. When the proactive issue is later created,
those skill names are part
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
auditable while accepting GitHub Actions' best-effort schedule timing. The
enqueue CLI output includes
`llm_e2e_required_after_proactive_not_before_change=true`, and changes to the
due gate require both skipped-run log evidence and a live due-run
GitHub Models follow-up that uses `repo-reader` and bounded repository search.

`--notify-route <route>` and `--notify-routes <a,b>` make proactive jobs useful
in Slack/Telegram-style channels without adding sockets, webhooks, or provider
API calls to the enqueue step. After a due proactive issue exists, GitClaw
resolves reviewed routes from `.gitclaw/channels/routes.yaml`, queues one
`gitclaw:channel-outbound` notification per destination, and writes only route
counts, short hashes, queue counts, duplicate counts, and target issue counts to
workflow outputs/logs. The notification body may name the proactive run issue,
normalized job name, slot, due state, creation state, and not-before gate; it
must not copy the raw proactive prompt, prompt file body, route names, provider
thread IDs, provider tokens, or model/tool output. If `--not-before` skips the
run, no issue or channel notification is created. Delivery to Telegram, Slack,
or another provider remains delegated to `gitclaw channel-outbox` and
`gitclaw channel-delivery`. Workflow-dispatch prompt and route inputs must be
read from `$GITHUB_EVENT_PATH` inside the step and copied to temporary files or
local shell variables; they must not be passed as step `env:` entries because
GitHub Actions prints those values in the run log header.

Changes to proactive channel notifications must run focused local tests and a
live E2E that proves all of the following: a real proactive workflow/CLI enqueue
creates or reuses a proactive issue, queues a channel notification, suppresses a
duplicate notification for the same `name + slot + message_id`, exposes pending
delivery through `channel-outbox` without leaking prompt text or route names,
and continues the proactive issue with a normal GitHub Models turn that uses
`repo-reader` and bounded repository search.

Reference proactive workflow shape:

```yaml
name: GitClaw Proactive Email Triage

on:
  workflow_dispatch:
    inputs:
      not_before:
        description: Optional RFC3339 or YYYY-MM-DD due gate
        required: false
      notify_routes:
        description: Optional comma-separated channel routes
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
        run: |
          set -euo pipefail
          input_not_before="$(jq -r '(.inputs // {}).not_before // ""' "$GITHUB_EVENT_PATH")"
          input_notify_routes="$(jq -r '(.inputs // {}).notify_routes // ""' "$GITHUB_EVENT_PATH")"
          export GITCLAW_PROACTIVE_NOT_BEFORE="$input_not_before"
          export GITCLAW_PROACTIVE_NOTIFY_ROUTES="$input_notify_routes"
          go run ./cmd/gitclaw proactive enqueue \
            --name email-triage \
            --slot "$(date -u +%Y-%m-%d)" \
            --prompt-file .gitclaw/proactive/email-triage.md
        env:
          GH_TOKEN: ${{ github.token }}
          GITHUB_TOKEN: ${{ github.token }}
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
@gitclaw /proactive chain
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
operator visibility before adding or editing scheduled jobs. The root report
includes `llm_e2e_required_after_proactive_report_change=true`, and the
explicit `/proactive list` alias also includes
`llm_e2e_required_after_proactive_list_change=true`, so changes to either
surface must prove the body-free report and a normal GitHub Models tool-using
follow-up.

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

The schedule-calendar form:

```text
@gitclaw /proactive schedule
@gitclaw /cron schedule
```

posts a `GitClaw Proactive Schedule Report` for the reviewed GitHub Actions
cron surface. It reports workflow files indexed, workflow-dispatch support,
schedule support, cron-entry counts, cron cadence classification, prompt-file
counts, skill-hint counts, not-before support, issue-thread strategy, and the
explicit gate that heartbeat-style wakeups are approximate while cron schedules
are the exact timing boundary. It borrows OpenClaw's cron-vs-heartbeat split
and Hermes' skill-backed cron job shape, but never prints workflow bodies,
prompt bodies, issue/comment bodies, tool outputs, credentials, or secret
values. The report includes
`llm_e2e_required_after_proactive_schedule_change=true`, so changes to this
surface must be paired with a live GitHub Models follow-up that selects
`repo-reader`, exposes `gitclaw.search_files`, and recovers a bounded
repository-search fixture token.

The chain form:

```text
@gitclaw /proactive chain
@gitclaw /cron chain
```

posts a `GitClaw Proactive Chain Report` for reviewed proactive job dependency
metadata. A prompt can declare upstream context with
`<!-- gitclaw:proactive-context-from repo-hygiene -->`; the report resolves
those names against `.gitclaw/proactive/*.md`, emits prompt paths, prompt
hashes, skill hints, edge counts, resolved job names, missing-source hashes,
self-reference/cycle gates, and no runtime writes. This adapts Hermes cron's
`context_from` chaining to GitHub issue-native proactive runs without adding a
hidden scheduler database. It never prints prompt bodies, workflow bodies,
issue/comment bodies, tool outputs, credentials, raw missing-source text, or
secret values. The report includes
`llm_e2e_required_after_proactive_chain_change=true`, so changes to this
surface require deterministic local tests, a live issue E2E on the main repo,
and a normal GitHub Models follow-up proving `repo-reader`,
`gitclaw.search_files`, prompt provenance, and usage telemetry.

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
`ambiguous`, plus `raw_bodies_included=false` and
`llm_e2e_required_after_proactive_info_change=true`. Changes to this operator
surface must pair the deterministic body-free report with a normal GitHub
Models follow-up that uses `repo-reader` and bounded repository search, so the
scheduled-job inspection path is tested with real model/tool context.

Local operators can inspect the same proactive surface without opening an
issue:

```bash
gitclaw proactive list
gitclaw proactive schedule
gitclaw proactive chain
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
@gitclaw /models catalog
@gitclaw /models usage
@gitclaw /models cost
@gitclaw /models risk
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

`@gitclaw /models catalog` adds the reviewed GitHub Models catalog/default
surface. It reports the configured model, fallback models, default candidate,
catalog snapshot date, catalog API version, catalog/inference documentation
URLs, reviewed OpenAI/GPT-5 entry counts, whether `openai/gpt-5.4-mini` is
present, whether the configured model matches the reviewed default candidate,
and no-live-probe/no-raw-response gates. It never prints raw catalog responses,
provider responses, prompts, issue bodies, comments, credentials, or secret
values. Changes to this surface require a deterministic catalog issue E2E plus
a normal GitHub Models follow-up that proves selected `repo-reader`,
`gitclaw.search_files`, prompt provenance, and usage telemetry.

`@gitclaw /models usage` adds the body-free token telemetry view inspired by
OpenClaw's `/status` and `/usage` split and Hermes' use of API-reported token
counts for context management. GitClaw normalizes usage fields returned by
GitHub Models/OpenAI-compatible chat completions (`prompt_tokens`/`input_tokens`,
`completion_tokens`/`output_tokens`, `total_tokens`, and common cached-token
aliases) and persists those counters as assistant-marker attributes on
model-backed turns. The deterministic usage report never performs a live model
probe. It reads existing assistant markers, prompt-projection metadata, model
config, and context counts, then reports recorded prompt/completion/total/cache
tokens when present.

Current references: OpenClaw token-use documentation
(`https://docs.openclaw.ai/reference/token-use`), Hermes context compression
and caching docs (`https://hermes-agent.nousresearch.com/docs/developer-guide/context-compression-and-caching/`),
and GitHub Models Actions quickstart
(`https://docs.github.com/en/github-models/quickstart`).

`@gitclaw /models cost` is the separate body-free cost surface. GitHub's direct
GitHub Models billing docs define token units, a fixed `$0.00001` unit price,
and per-model input/cached-input/output multipliers
(`https://docs.github.com/en/billing/reference/costs-for-github-models`;
`https://docs.github.com/en/enterprise-cloud@latest/billing/concepts/product-billing/github-models`).
GitClaw keeps a reviewed multiplier snapshot and estimates only when a usage
marker's model appears in that snapshot. The current smallest default,
`openai/gpt-5-nano`, is intentionally reported as `projected_usd: unavailable`
until GitHub publishes a matching direct-use multiplier. The report does not
query billing APIs, inspect account paid-usage or budget state, or run a live
inference probe.

It never dumps issue/comment bodies, API keys, full prompts, or raw provider
error bodies. This gives operators a safe way to inspect GitHub Models and
OpenAI-compatible provider wiring from the issue thread before burning model
quota on a real assistant turn.

Local operators can inspect the same model wiring without opening an issue:

```bash
gitclaw models list
gitclaw models catalog
gitclaw models usage
gitclaw models cost
gitclaw models risk
```

The local report omits repository, issue number, and issue-title hash while
retaining provider family, model ID, endpoint host, token-source name, timeout,
retry settings, prompt-artifact status, and environment knob names.

`@gitclaw /models risk` and `gitclaw models risk` provide the stricter
body-free model/provider risk audit. The report follows OpenClaw's separation
between model status and quota-spending probes: it does not call the GitHub
Models catalog or inference endpoint. It also follows Hermes' profile boundary
by treating model config as high-authority profile state, not as something the
agent can silently rewrite.

The model risk report publishes:

- provider family, endpoint host, token-source name, and output-token
  parameter,
- primary model, fallback models, known GitHub Models catalog snapshot matches,
  and whether the primary matches the repo's small-model default,
- retry timeout, attempts, delay, retryable status categories, and fallback
  enablement,
- config-file metadata and prompt-artifact state,
- explicit `model_catalog_probe_performed=false` and
  `live_inference_probe_performed=false`,
- finding codes, severities, paths, fields, and hashes for unsafe provider
  boundaries, insecure model endpoints, missing/unsafe budgets, duplicated or
  unknown fallback models, credential material in model config, raw prompt
  logging, live-probe requirements, and raw provider-error leakage.

It never prints config bodies, issue/comment bodies, prompts, raw provider
errors, API keys, tokens, or secret values. Any change to this surface requires
local tests plus a live GitHub issue E2E that includes a normal GitHub Models
follow-up turn with repo-reader/tool usage.

### Research Catalog Command

GitClaw treats the OpenClaw/Hermes research as a maintained product surface,
not only as prose in a design note:

```text
@gitclaw /research
@gitclaw /research catalog
@gitclaw /research sources
@gitclaw /research coverage
@gitclaw /research verify
@gitclaw /landscape
```

The command runs after preflight and repo-context loading, but before model
inference. It posts a deterministic `GitClaw Research Catalog Report` with
`model="gitclaw/research"`. The report maps official OpenClaw and Hermes
sources to GitClaw's repo-native design choices:

- OpenClaw gateway, channel, skill, and security docs;
- Hermes skills, memory, cron, and checkpoint/rollback docs;
- local research/spec/README file hashes;
- adopted patterns such as issue-native sessions, workflow-dispatch channels,
  repo-local soul/memory, progressive skill disclosure, proactive scheduled
  jobs, checkpoint readiness, and backup durability;
- rejected v1 patterns such as long-running sockets, agent-managed skill
  writes, remote registry installs, delegation/subagents, and destructive
  rollback.

It does not browse or fetch sources at runtime. It reports source IDs, URLs,
counts, local file hashes, pattern names, and gate decisions only. It never
prints raw research notes, source bodies, issue/comment bodies, prompts,
tool outputs, credentials, or secret values. Changes to this surface require a
deterministic live `@gitclaw /research catalog` issue plus a normal GitHub
Models repo-reader/search follow-up proving inference, selected skill,
`gitclaw.search_files`, prompt provenance, and usage telemetry.

Local operators can inspect the same map without opening an issue:

```bash
gitclaw research catalog
gitclaw research sources
gitclaw research coverage
gitclaw research verify
```

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
- Subcommands: `preflight`, `handle`, `backup`, `backup coverage`,
  `backup search`, `backup provenance`, `backup snapshot`, `backup timeline`,
  `backup info`, `backup freshness`, `backup continuity`, `backup retention-plan`,
  `session provenance`, `session tools`, `session skills`, `session usage`,
  `session trajectory`, `session compaction`, `session resume`,
  `session status`,
  `session coverage`,
  `heartbeat`, `heartbeat status`, `heartbeat risk`,
  `channel-ingest`, `channel-send`, `channel-send --route`,
  `channel-state`, `channel-gateway`,
  `channel-outbox`, `channel-delivery`,
  `channels list`, `channels verify`, `channels risk`, `channels info`,
  `checkpoints catalog`, `checkpoints status`, `checkpoints list`,
  `checkpoints preview`, `checkpoints risk`, `checkpoints verify`,
  `rollback catalog`, `rollback diff`, `rollback list`, `rollback risk`,
  `proactive enqueue`, `proactive init`, `proactive info`, `proactive risk`,
  `approvals list`, `approvals verify`, `approvals provenance`,
  `approvals risk`,
  `artifacts catalog`, `artifacts list`, `artifacts risk`, `artifacts verify`,
  `diffs summary`, `diffs risk`, `diffs verify`,
  `workspace catalog`, `workspace summary`, `workspace risk`,
  `workspace verify`,
  `hooks catalog`, `hooks list`, `hooks risk`, `hooks verify`,
  `plugins list`, `plugins risk`, `plugins verify`,
  `tasks list`, `tasks risk`, `tasks verify`, `tasks ledger`,
  `agents catalog`, `agents list`, `agents provenance`, `agents risk`, `agents verify`,
  `nodes catalog`, `nodes list`, `nodes risk`, `nodes verify`,
  `migrate plan`, `migrate risk`,
  `orders list`, `orders verify`, `orders risk`,
  `models list`, `models catalog`, `models usage`, `models cost`, `models risk`,
  `profile show`, `profile verify`,
  `context list`, `context risk`, `context info`,
  `research catalog`, `research sources`, `research coverage`, `research verify`,
  `prompt list`, `prompt pack`, `prompt context`, `prompt cache`,
  `prompt compression`, `prompt risk`,
  `runs current`, `runs verify`, `runs history`,
  `sandbox explain`, `sandbox verify`, `sandbox risk`,
  `security audit`, `security risk`,
  `memory catalog`, `memory snapshot`, `memory provenance`, `memory verify`, `memory risk`, `memory validate`,
  `memory timeline`, `memory list`, `memory promote-plan`, `memory info`, `memory search`,
  `skills validate`,
  `skills list`, `skills catalog`, `skills snapshot`, `skills provenance`, `skills select-plan`, `skills refresh-plan`,
  `skills proposals`, `skills proposal-plan`, `skills install-plan`,
  `skills upgrade-plan`, `skills info`, `skills search`,
  `bundles list`, `bundles risk`, `bundles info`,
  `soul catalog`, `soul anchors`, `soul snapshot`, `soul provenance`, `soul verify`, `soul risk`,
  `soul validate`, `soul list`, `soul edit-plan`, `soul info`, `soul search`,
  `tools verify`, `tools risk`, `tools validate`, `tools list`,
  `tools exposure`, `tools exposure risk`, `tools defer-plan`,
  `tools boundary`, `tools provenance`, `tools approval-plan`,
  `tools run-plan`, `tools request-run`, `tools info`, `tools search`, `doctor`,
  `policy verify`, `policy risk`,
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
- Supports deterministic `@gitclaw /context risk` reports so maintainers can
  audit prompt-visible context risk without printing context bodies.

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
@gitclaw /session catalog
@gitclaw /session list
@gitclaw /session provenance
@gitclaw /session tools
@gitclaw /session skills
@gitclaw /session usage
@gitclaw /session trajectory
@gitclaw /session compaction
@gitclaw /session resume
@gitclaw /session handoff --id <id>
@gitclaw /session status
@gitclaw /session readback
@gitclaw /session stats
@gitclaw /session coverage
@gitclaw /session risk
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
- model-backed versus deterministic assistant-turn counts and model names,
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
gitclaw session catalog
gitclaw session list --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session provenance --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session tools --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session skills --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session usage --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session trajectory --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session compaction --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session resume --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session status --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session stats --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session coverage --backup .gitclaw/backups/owner/repo/issues/000123.json
```

The local report reads the canonical backup JSON, uses `scope: local-backup`,
and emits repo/issue backup metadata, marker counts, transcript counts, trust
states, sources, sizes, and hashes without dumping issue bodies, comment bodies,
or assistant replies.

`gitclaw session catalog` is the compact discovery surface for session
inspection. It lists issue intents, local backup commands, execution locations,
recall gates, GitHub-issue canonical session storage, and disabled export/delete
authority without printing raw issue bodies, comment bodies, assistant replies,
prompts, tool outputs, or raw search queries. `@gitclaw /session catalog`
publishes the same command/gate map for the current issue before model
inference and carries `llm_e2e_required_after_session_catalog_change: true`.
Catalog changes require a live deterministic issue-command E2E plus a normal
GitHub Models repo-reader/search follow-up.

`gitclaw session provenance --backup <issue.json>` is the named
OpenClaw/Hermes-inspired prompt provenance surface. It emits assistant-turn
marker counts, prompt-context hashes, model-backed versus deterministic turn
counts, model names, prompt-visible skill/tool names, tool-output counts, and
token usage telemetry. The issue-side `@gitclaw /session provenance` form runs
before model inference and audits the current GitHub issue thread using the
same marker attributes. It never prints issue bodies, comment bodies, assistant
replies, prompts, raw search queries, or tool outputs, and carries
`llm_e2e_required_after_session_provenance_change: true`.

`gitclaw session tools --backup <issue.json>` is the named
OpenClaw/Hermes-inspired tool-use ledger for a backed-up issue session. It
aggregates prompt-visible tool names across assistant-turn markers, model-backed
tool turns, deterministic tool turns, prompt-context hash counts, and token
usage telemetry. The issue-side `@gitclaw /session tools` form runs before
model inference and audits the current GitHub issue thread using the same
marker attributes. It never prints issue bodies, comment bodies, assistant
replies, prompts, tool inputs, raw search queries, or tool outputs, and carries
`llm_e2e_required_after_session_tools_change: true`.

`gitclaw session skills --backup <issue.json>` is the matching
OpenClaw/Hermes-inspired skill-use ledger for a backed-up issue session. It
aggregates prompt-visible skill names across assistant-turn markers, selected
skill marker counts, model-backed skill turns, deterministic skill turns,
prompt-context hash counts, and token usage telemetry. The issue-side
`@gitclaw /session skills` form runs before model inference and audits the
current GitHub issue thread using the same marker attributes. It never prints
issue bodies, comment bodies, assistant replies, prompts, raw skill bodies, raw
search queries, or tool outputs, and carries
`llm_e2e_required_after_session_skills_change: true`.

`gitclaw session usage --backup <issue.json>` is the named
OpenClaw/Hermes-inspired token/cache usage ledger for a backed-up issue
session. It aggregates normalized assistant-turn usage telemetry by model and
by turn: prompt tokens, completion tokens, total tokens, cache-read tokens, and
cache-write tokens. The issue-side `@gitclaw /session usage` form runs before
model inference and audits the current GitHub issue thread using marker
attributes only. It never prints issue bodies, comment bodies, assistant
replies, prompts, raw provider usage payloads, provider responses, raw search
queries, or tool outputs, and carries
`llm_e2e_required_after_session_usage_change: true`.

`gitclaw session trajectory --backup <issue.json>` is the named
OpenClaw trajectory-bundle and Hermes-session inspired manifest for a backed-up
issue session. It exports assistant-turn marker metadata as a body-free
trajectory: run/idempotency hashes, model names, prompt-context hashes,
prompt-visible skills/tools, context document counts, selected skill counts,
tool-output counts, and usage counters. The issue-side
`@gitclaw /session trajectory` form runs before model inference and audits the
current GitHub issue thread using marker attributes only. It never prints issue
bodies, comment bodies, assistant replies, prompts, raw provider responses, raw
search queries, raw run URLs, or tool outputs, and carries
`llm_e2e_required_after_session_trajectory_change: true`.

`gitclaw session compaction --backup <issue.json>` is the named
Hermes-threshold and OpenClaw-trajectory inspired compaction-readiness audit for
a backed-up issue session. It reports transcript byte/line/token estimates,
bounded transcript counts, omitted/truncated message counts, per-message
compaction cards, assistant-turn prompt provenance, model-backed turn evidence,
usage telemetry, and summary/mutation gates. The issue-side
`@gitclaw /session compaction` form runs before model inference and audits the
current GitHub issue thread using hashes, counts, and marker attributes only. It
does not create summaries, split sessions, mutate memory, write compressed
state, or print issue bodies, comment bodies, assistant replies, prompts, raw
provider payloads, raw search queries, raw run URLs, or tool outputs, and
carries `llm_e2e_required_after_session_compaction_change: true`.

`gitclaw session resume --backup <issue.json>` is the Hermes-resume and
OpenClaw-session inspired continuation-readiness audit for a backed-up issue
session. It reports the GitHub issue-thread resume key, labels, latest user and
assistant message hashes, assistant-turn prompt provenance, model-backed turn
evidence, usage telemetry, and reentry gates. The issue-side
`@gitclaw /session resume` form runs before model inference and proves the next
GitHub issue comment can continue the same canonical session through the
`issue_comment` workflow. It does not require workflow dispatch, a server, a
socket, or a hidden external session database, and it does not print issue
bodies, comment bodies, assistant replies, prompts, raw provider payloads, raw
search queries, raw run URLs, or tool outputs. It carries
`llm_e2e_required_after_session_resume_change: true`.

`@gitclaw /session handoff --id <id>` is the issue-native session fork. It runs
before model inference, computes the same body-free session resume/provenance
counts for the current issue thread, creates or reuses a labeled handoff issue,
and posts a source receipt with only issue numbers, counts, hashes, duplicate
status, and reentry gates. The handoff issue stores the raw handoff id and
metadata marker, not raw source issue bodies, comments, assistant replies,
prompts, tool outputs, provider payloads, or search queries. A normal
`@gitclaw` comment on the handoff issue then uses the existing `issue_comment`
workflow and GitHub Models runner; no server, socket, workflow-dispatch session
bridge, or external session database is required. It carries
`llm_e2e_required_after_session_handoff_issue_change: true`.

`gitclaw session status --backup <issue.json>` is the compact Hermes-inspired
readback surface. It emits session labels, transcript/comment counts, latest
user and assistant message sources with sizes and hashes, latest assistant
marker model/provenance fields, model-backed versus deterministic turn counts,
prompt-visible skill/tool names, and skill/tool turn counts. It never prints the
latest user request, assistant reply, issue body, prompt text, search query, or
tool output body. The issue-side `@gitclaw /session status` form runs before
model inference and posts the same body-free status for the current GitHub issue
conversation.

`gitclaw session stats --backup <issue.json>` is the compact Hermes-inspired
summary surface. It emits counts for comments, transcript roles, trust/edited
state, body byte/line totals, assistant-turn provenance, model-backed versus
deterministic turns, model names, prompt-visible skill/tool counts, and marker
origins without listing individual transcript messages or raw message bodies.
The issue-side `@gitclaw /session stats` form runs before model inference and
posts the same body-free summary for the current GitHub issue conversation.

`gitclaw session coverage --backup <issue.json>` is the stricter E2E gate. By
default it requires at least one assistant turn, at least one assistant marker
with prompt provenance, and at least one non-`gitclaw/*` model-backed turn. It
can also require specific prompt-visible skills or tools such as
`--require-skill repo-reader` and `--require-tool gitclaw.search_files`. It
reports only counts, model names, skill names, tool names, missing requirement
sets, and boolean evidence; it exits non-zero when coverage is missing. This is
the operator proof that a test exercised a real GitHub Models conversation and
tool context rather than only a deterministic report.

Backed-up sessions can also be searched locally without a GitHub API call:

```bash
gitclaw session coverage --backup .gitclaw/backups/owner/repo/issues/000123.json --require-tool gitclaw.search_files
gitclaw session provenance --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session tools --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session skills --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session usage --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session trajectory --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session compaction --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session resume --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session status --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session stats --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session risk --backup .gitclaw/backups/owner/repo/issues/000123.json
gitclaw session search deployment window --backup .gitclaw/backups/owner/repo/issues/000123.json
```

`gitclaw session risk --backup <issue.json>` audits a backed-up issue session
without calling GitHub or a model. It reports marker/provenance counts,
trusted/untrusted and edited transcript counts, channel/proactive origin flags,
risk finding codes, severities, sources, and evidence hashes only. It flags
empty transcripts, assistant turns missing prompt provenance, error markers,
untrusted or edited prompt-visible messages, channel/proactive session origins,
and reused prompt-context hashes without printing issue bodies, comments,
assistant replies, prompts, tool outputs, search queries, credentials, or secret
values.

The local search report uses the same body-free matcher and returns
`scope: local-backup`, backup metadata, query hash/term count, transcript and
match counts, result limits, sources, trust metadata, scores, and hashes.

When called as `@gitclaw /session risk`, the command audits the current
GitHub issue session after transcript reconstruction and before model
inference. The issue report publishes the same body-free risk cards plus
`current_issue_session_request=true`. Any change to this surface requires local
tests plus a live GitHub issue E2E that includes a normal GitHub Models turn
with prompt provenance before the deterministic session-risk report.

When called as `@gitclaw /session coverage`, the command checks the current
issue thread for the same model-backed prompt provenance requirements. It does
not fail the workflow if coverage is missing; it posts a body-free warning
report so maintainers can see whether the current conversation is useful as an
LLM/tool E2E fixture. The local backup form is the enforceable gate because it
can exit non-zero in scripts.

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
.gitclaw/mcp/*.yaml          # declarative MCP server specs, metadata-only in v1
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
.gitclaw/skill-sources/*.yaml # reviewed local skill provenance pins
.gitclaw/toolsets/*.yaml     # reviewed deterministic tool profiles, advisory in v1
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
@gitclaw /memory catalog
@gitclaw /memory snapshot
@gitclaw /memory provenance
@gitclaw /memory verify
@gitclaw /memory risk
@gitclaw /memory validate
@gitclaw /memory timeline
@gitclaw /memory promote-plan long-term
@gitclaw /memory remember --target long-term --id weekly-ops-memory
@gitclaw /memory rehearse --target long-term --id weekly-ops-rehearsal
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

When called as `@gitclaw /memory catalog`, the command posts a compact
body-free discovery catalog for repo-local memory. It follows the OpenClaw and
Hermes split between durable memory, procedural skills, and searchable session
recall: `.gitclaw/MEMORY.md` and dated `.gitclaw/memory/*.md` notes are
reported as durable-memory entries, procedural memory stays in the skills
catalog, and session recall stays in GitHub issues/backups. The report includes
prompt-visible/load-mode metadata, memory roles, reason codes, validation/risk
rollups, short hashes, disabled write/provider/background-promotion gates, and
`llm_e2e_required_after_memory_catalog_change: true`. It never includes raw
memory bodies, issue bodies, comments, prompts, session transcripts, embedding
vectors, credentials, or secret values. Local operators can run the same report
with `gitclaw memory catalog`.

When called as `@gitclaw /memory snapshot`, `@gitclaw /memory fingerprint`, or
`@gitclaw /memory lock`, the command posts a stable, body-free fingerprint for
durable repo-local memory. It combines `.gitclaw/MEMORY.md` and dated
`.gitclaw/memory/*.md` notes into per-entry metadata plus one composite
`snapshot_sha256_12`, including prompt-visible load state, first/latest note
metadata, gap counts, validation/risk rollups, and disabled write/provider/
background-promotion gates. It never includes raw memory bodies, issue bodies,
comments, prompts, session transcripts, embedding vectors, credentials, or
secret values. Local operators can run the same report with
`gitclaw memory snapshot`; `fingerprint`, `lock`, and `lockfile` are accepted
aliases.

When called as `@gitclaw /memory provenance`, the command posts a body-free git
history map for repo-local memory files. It reports durable memory counts,
prompt-visible load state, validation/risk rollups, repo-local source counts,
tracked/untracked/dirty state, last commit IDs/dates, commit-subject hashes,
disabled write/provider/background-promotion gates, and
`llm_e2e_required_after_memory_provenance_change: true`. It never includes raw
memory bodies, issue bodies, comments, prompts, git subjects, author identities,
provider payloads, credentials, or secret values. Local operators can run the
same report with `gitclaw memory provenance`; `git-history` is accepted as a
CLI alias.

When called as `@gitclaw /memory timeline`, the command posts a body-free
chronology of `.gitclaw/MEMORY.md` and `.gitclaw/memory/*.md`. It reports
repo-local authority, prompt-visible load state, first/latest dated note,
timeline span, largest dated-note gap, per-file byte/line/hash metadata,
validation/risk gates, and the explicit LLM-backed E2E requirement for changes
to the timeline surface. It never includes raw memory bodies, issue bodies,
comments, prompts, or secret values. Local operators can run the same report
with `gitclaw memory timeline`; `history` and `ledger` are accepted CLI aliases.

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
explicit live-LLM E2E requirement, including
`llm_e2e_required_after_memory_promote_plan_change: true`. It never generates
the candidate memory, calls a model, writes files, mutates the repository, or
dumps issue bodies, comments, transcript bodies, current memory bodies, or
candidate memory text. User-profile promotions route to `/soul edit-plan user`.
Any implementation change to the planner must pair the deterministic report
check with a normal issue-comment follow-up that uses GitHub Models, the
repo-reader skill, and bounded repository search to prove tool visibility.

When called as `@gitclaw /memory remember --target <target> --id <id>`,
GitClaw performs the issue-native memory proposal action. It opens or reuses a
dedicated GitHub issue marked with `gitclaw:memory-proposal-issue`, records the
safe proposal id, source request hash, source issue/comment identity, target
kind/path, current target hash/size, memory budget, latest dated note, and
memory validation rollup, then posts a body-free receipt in the source
conversation. Duplicate open memory proposal issues are suppressed by marker
and proposal id.

The memory proposal action also accepts `--notify-route <route>` and
`--notify-routes <a,b>`. When present, GitClaw resolves reviewed channel routes
from `.gitclaw/channels/routes.yaml`, queues one metadata-safe
`gitclaw:channel-outbound` notification per destination, and records only
counts and hashes in the source receipt. The channel notification body may name
the safe proposal id, review issue, source issue, target kind/path, validation
status, and review boundary, but it must not copy raw source request text,
candidate memory, existing memory bodies, or route names. Provider delivery
remains outside the action and is handled by `gitclaw channel-outbox` plus
`gitclaw channel-delivery`.

The memory proposal action does not write `.gitclaw/MEMORY.md`, does not write
`.gitclaw/memory/*.md`, does not generate candidate memory, does not copy raw
issue/comment text into the proposal issue or receipt, and does not call a
model. When channel notifications are requested, it still does not call
provider APIs. It is a GitHub-native review queue entry: after review, a
maintainer can draft compact memory on a normal branch, run memory
validation/verification, and let the memory affect future prompts only through
a reviewed git change.

When called as `@gitclaw /memory rehearse --target <target> --id <id>`,
GitClaw performs the issue-native memory rehearsal action. It opens or reuses a
dedicated GitHub issue marked with `gitclaw:memory-rehearsal-issue`, labels it
with `gitclaw` so the conversation can continue normally, records the safe
rehearsal id, source request hash, source issue/comment identity, target
kind/path, current target hash/size, memory budget, latest dated note, and
memory validation rollup, then posts a body-free receipt in the source
conversation. Duplicate open memory rehearsal issues are suppressed by marker
and rehearsal id.

The memory rehearsal action does not write `.gitclaw/MEMORY.md`, does not write
`.gitclaw/memory/*.md`, does not generate candidate memory, does not copy raw
issue/comment text or current memory bodies into the source receipt, and does
not call a model. It is a GitHub-native conversation lane for trying the
current prompt-visible memory. Proposed memory changes still belong in
`/memory remember` or a normal reviewed branch. Any implementation change must
pair the deterministic source action with a real GitHub Models follow-up on the
rehearsal issue itself.

When called as `@gitclaw /memory validate`, the command renders only the
memory-hygiene report. Local operators can run the same validation with:

```bash
gitclaw memory catalog
gitclaw memory snapshot
gitclaw memory provenance
gitclaw memory verify
gitclaw memory risk
gitclaw memory validate
gitclaw memory timeline
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
gitclaw skills install-plan <target>
gitclaw skills upgrade-plan <target>
gitclaw bundles catalog
gitclaw bundles list
gitclaw bundles risk
gitclaw bundles provenance
gitclaw bundles info <name>
gitclaw bundles search <query>
@gitclaw /bundles rehearse <name> --id <id>
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
@gitclaw /skills catalog
@gitclaw /skills eligible
@gitclaw /skills snapshot
@gitclaw /skills provenance
@gitclaw /skills select-plan repo-reader
@gitclaw /skills refresh-plan
@gitclaw /skills sources
@gitclaw /skills sources verify
@gitclaw /skills sources lock
@gitclaw /skills sources update-plan
@gitclaw /skills sources provenance
@gitclaw /skills sources risk
@gitclaw /skills sources info repo-reader
@gitclaw /skills sources search repo-local
@gitclaw /skills sources propose repo-reader --source github:owner/repo/path
@gitclaw /skills runtime
@gitclaw /skills requirements
@gitclaw /skills metadata
@gitclaw /skills proposals
@gitclaw /skills proposals risk
@gitclaw /skills proposal-plan repo-reader
@gitclaw /skills propose repo-reader
@gitclaw /skills rehearse repo-reader --id repo-reader-lab
@gitclaw /skills install-plan repo-reader
@gitclaw /skills upgrade-plan repo-reader
@gitclaw /bundles
@gitclaw /bundles catalog
@gitclaw /bundles risk
@gitclaw /bundles provenance
@gitclaw /bundles info repo-context
@gitclaw /bundles rehearse repo-context --id repo-context-lab
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
- git tracked state, dirty state, commit IDs/dates, and commit-subject hashes
  when explicitly requested.
- declared env/bin requirement counts and whether any are missing.
- validation status, error/warning counts, duplicate-name count, invalid-name
  count, folder/name mismatch count, and body-free findings.
- risk-audit status, risky-instruction category counts, finding codes, and
  line hashes without raw `SKILL.md` text.
- source-pin counts, expected/current skill hashes, source kind, trust level,
  install mode, no-fetch gates, and body-free provenance risk findings.
- OpenClaw-compatible runtime metadata counts for env/bin/install declarations,
  primary env hashes, inert install specs, and no-install/no-registry gates.
- compact catalog eligibility metadata when explicitly requested: eligible and
  ineligible counts, load modes, reason codes, selected state, always-on state,
  description hashes, body hashes, and disabled registry/install/update gates.
- lockfile-style skill snapshot metadata when explicitly requested: composite
  snapshot hash, local skill cards, selected-skill prompt hashes, bundle
  instruction hashes, source-pin hashes, and validation/risk/source gates.
- dry-run selection planning metadata when explicitly requested.
- skill refresh-boundary planning metadata when explicitly requested.
- dry-run install/upgrade planning metadata when explicitly requested.

It does not dump full skill bodies. Full `SKILL.md` content remains a prompt
input only when selected by the normal progressive-disclosure rules.
`@gitclaw /skills list` is an explicit inventory alias for the same report,
matching the local `gitclaw skills list` helper.

When called as `@gitclaw /skills catalog`, `@gitclaw /skills eligible`, or
locally as `gitclaw skills catalog`, GitClaw posts a compact
`GitClaw Skill Catalog Report`. This is the GitHub-native equivalent of the
OpenClaw/Hermes discovery split: `skills_list`-style metadata is visible
first, while `skill_view`-style full bodies are only loaded when progressive
disclosure selects a skill. The catalog reports prompt eligibility, load mode,
reason codes, requirement counts, selected/always state, risk counts,
validation/risk rollups, and description/body hashes. It never prints raw
skill bodies, raw descriptions, issue bodies, comments, prompts, tool inputs,
tool outputs, env names, or installer targets.

When called as `@gitclaw /skills snapshot`, `@gitclaw /skills fingerprint`,
`@gitclaw /skills lock`, or locally as `gitclaw skills snapshot`, GitClaw
posts a `GitClaw Skill Snapshot Report`. This is the OpenClaw/Hermes-style
skill-state lockfile for the serverless Actions runtime: it fingerprints
repo-local skills, prompt-visible selected skill bodies, skill bundles, and
reviewed source pins as counts, paths, short hashes, and one composite
`snapshot_sha256_12`. The report carries validation, risk, and source-pin
gates plus `llm_e2e_required_after_skill_snapshot_change: true`; every change
to this surface must include deterministic report tests and a live GitHub
Models follow-up that proves actual conversation, skill selection, and
`gitclaw.search_files` tool visibility still work. It never prints raw skill
bodies, descriptions, bundle instructions, source refs, issue/comment bodies,
prompts, provider payloads, credentials, or secret values.

When called as `@gitclaw /skills provenance`, `@gitclaw /skills history`, or
`@gitclaw /skills timeline`, the command posts a body-free git provenance map
for repo-local `SKILL.md` files. It reports available/enabled/selected skill
counts, source roots, tracked/untracked counts, working-tree dirty state,
commit availability, last commit SHA/date metadata, commit-subject hashes,
validation/risk gates, installer-disabled state, and mutation-disabled state.
It does not print raw skill bodies, issue bodies, comments, prompts, commit
subjects, author names, author emails, requirement names, installer output, or
secret values. This mirrors `gitclaw skills provenance` for local review.

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

`@gitclaw /bundles search <query>` and `gitclaw bundles search <query>` provide
the bundle-level analogue to skills search. They search only repo-local bundle
metadata and skill references, represent the raw query by hash and term count,
and report match fields, paths, skill refs, instruction hashes, and bundle
hashes without printing raw bundle YAML, instructions, skills, issue text,
comments, prompts, or query text.

`@gitclaw /bundles catalog`, `@gitclaw /bundles index`, and
`gitclaw bundles catalog` produce the compact orchestration catalog for
Hermes-style skill bundles. The report treats bundles as procedural-memory
task profiles over existing reviewed skills and shows selected/load state,
instruction load mode, skill-ref resolution, instruction hashes, risk rollups,
reason codes, and disabled external-registry, installer, and agent-authored
mutation gates. It includes
`llm_e2e_required_after_bundle_catalog_change: true` and never prints raw
bundle YAML, bundle instructions, skill bodies, issue bodies, comments,
prompts, credentials, provider payloads, or secret values.

`@gitclaw /bundles provenance`, `@gitclaw /bundles history`,
`@gitclaw /bundles timeline`, and `gitclaw bundles provenance` map
repo-local bundle YAML files to body-free git provenance. The report shows
bundle and skill-ref counts, instruction hashes, bundle body hashes, tracked
state, dirty state, last commit IDs/dates, commit-subject hashes, and explicit
no-mutation gates. It never prints bundle YAML, bundle instructions, skill
bodies, issue/comment bodies, prompts, commit subjects, author names, author
emails, provider payloads, credentials, or secret values. This is the
GitHub-native adaptation of Hermes' bundle lifecycle: bundles are useful
task-profile aliases, but in GitClaw they remain reviewed files whose history
is inspectable before they influence a model turn.

`@gitclaw /bundles risk` and `gitclaw bundles risk` scan repo-local bundle YAML
and optional bundle instructions internally for prompt-boundary overrides,
secret-exfiltration instructions, hidden persistence, unreviewed shell
execution, unbounded tool loops, external delivery, remote-install language,
missing skill refs, empty bundles, parse errors, and duplicate bundle names.
The report publishes only bundle counts, skill-ref counts, finding codes,
severities, paths, bundle hashes, and line hashes. It never prints bundle YAML
bodies, bundle instructions, skill bodies, issue/comment bodies, prompts, raw
provider errors, credentials, or secret values. Any change to this surface
requires focused local tests plus a live GitHub Models follow-up E2E proving
normal inference, selected skills, and prompt-visible tools still work.

`@gitclaw /bundles rehearse <name> --id <id>` opens or reuses a dedicated
GitHub issue for trying a repo-local bundle as a task-profile conversation
lane. The source action is deterministic and posts only metadata: request
hashes, bundle path/hash, skill-ref counts, resolved/missing skill counts,
instruction presence/hash, risk rollups, duplicate state, and disabled
mutation/install gates. It does not call a model, install or update skills,
edit bundle YAML, print raw source text, print bundle instructions, or print
skill bodies. The created issue is labeled for normal GitClaw conversation;
the required E2E must then comment on that issue and prove a real GitHub
Models turn with selected `repo-reader`, prompt-context provenance,
`gitclaw.search_files`, and usage telemetry.

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
It also includes `llm_e2e_required_after_skill_select_plan_change=true`;
changes to the selection planner must be paired with a live follow-up that
selects `repo-reader`, exposes `gitclaw.search_files`, and recovers a bounded
repository-search fixture token without echoing skill-body, request-text, or
issue-body sentinels. Search extraction must prioritize the newest user turn
so an earlier report command cannot crowd the current search request out of the
bounded `gitclaw.search_files` output.

When called as `@gitclaw /skills refresh-plan` or
`gitclaw skills refresh-plan`, GitClaw posts a body-free refresh-boundary
report inspired by OpenClaw's explicit skill snapshots and watcher-driven
refresh behavior, but adapted to GitHub Actions. GitClaw has no resident skill
watcher, no mid-run hot reload, and no persisted skill snapshot shared across
runs. Each issue, comment, or reviewed workflow dispatch turn rebuilds the
skill index from the current Actions checkout. The report exposes only
metadata: current checkout scope, available/enabled/selected skill counts,
skill hashes, validation status, and the exact refresh boundary.

The refresh planner never installs, updates, deletes, commits, pushes, polls a
remote registry, mutates `.gitclaw/SKILLS`, calls a model, prints prompts, or
dumps skill/issue/comment bodies. Skill edits become prompt-visible only after
they land in the branch used by the next Actions checkout. Any change to skill
refresh behavior must be paired with `gitclaw skills validate`, `gitclaw skills
verify`, `gitclaw skills risk`, and a live GitHub Models conversation E2E that
proves normal skill selection and tool usage still work.

Skill source pins are the no-registry GitClaw analogue of OpenClaw ClawHub
trust envelopes and Hermes Hub/tap provenance. Files live in
`.gitclaw/skill-sources/*.yaml`:

```yaml
name: repo-reader
skill_path: .gitclaw/SKILLS/repo-reader/SKILL.md
source_kind: repo-local
source_ref: .gitclaw/SKILLS/repo-reader/SKILL.md
trust_level: repo-local
install_mode: manual-review
expected_sha256_12: 2f9e68a57bd6
requires_approval: true
remote_fetch_allowed: false
```

`@gitclaw /skills sources` and `gitclaw skills sources` list source pins by
path, normalized name, skill path, source kind, trust level, install mode,
expected/current skill hash, match state, and no-fetch/no-install runtime
gates. `@gitclaw /skills sources verify` and
`gitclaw skills sources verify` provide the stricter body-free trust envelope:
source-pin hashes, source-ref hashes, current skill hashes, risk rollups,
registry-not-configured status, static-only remote-fetch verification,
disabled install verification, and no-registry/no-fetch/no-install/no-mutation
gates. `@gitclaw /skills sources lock` and
`gitclaw skills sources lock` project a reproducibility lock from reviewed
source pins: lock state, stale/unpinned/missing counts, expected/current skill
hashes, aggregate lock hash, `.clawhub/lock.json` presence/hash when present,
and disabled registry/fetch/install/mutation gates without loading registry
state or printing raw lockfiles. `@gitclaw /skills sources provenance` and
`gitclaw skills sources provenance` map reviewed source pins to body-free git
history: source-pin paths, source kind, trust level, install mode, match/hash
state, risk codes, tracked/dirty state, last commit IDs/dates, and
commit-subject hashes only. `@gitclaw /skills sources risk` and
`gitclaw skills sources risk` scan source-pin YAML for parse errors, missing
skill matches, missing or mismatched hashes, unsafe remote-fetch gates,
installer-like install modes, missing approval gates, untrusted source kinds,
credential material, prompt-boundary overrides, host execution, repository
mutation, remote exfiltration, and unbounded loops. `@gitclaw /skills sources
info <name>` and `gitclaw skills sources info <name>` show one focused source
pin with no-registry/no-fetch/no-install/no-mutation gates, hash-only raw-body
boundaries, risk rollups, and
`llm_e2e_required_after_skill_source_info_change=true`.
`@gitclaw /skills sources search <query>` and
`gitclaw skills sources search <query>` search only reviewed source-pin
metadata: source name, path, skill path, source kind, trust level, install
mode, expected/current hashes, and risk codes. Search reports hash the raw
query, return match fields and scores, and include
`llm_e2e_required_after_skill_source_search_change=true` without printing raw
query text, source refs, source YAML, skill bodies, prompts, issue text, or
credentials.
`@gitclaw /skills sources update-plan` and
`gitclaw skills sources update-plan` are the no-fetch companion to
OpenClaw/ClawHub-style updates: they report stale, unpinned, missing, remote,
or risky pins as manual-review candidates, show source hash state and disabled
registry/fetch/install/mutation gates, and never contact registries or mutate
skills.

When called as `@gitclaw /skills sources propose <name> --source <ref>`,
GitClaw performs the issue-native source-pin proposal action. It opens or
reuses a dedicated GitHub issue marked with
`gitclaw:skill-source-proposal-issue`, labels it for normal GitClaw follow-up
conversation, records the proposed source-pin path, proposed skill path,
source kind, source-ref hash, source issue/comment identity, current source
risk rollup, and approval/fetch/install gates, then posts a body-free receipt
in the source conversation. Duplicate open source proposal issues are
suppressed by hidden marker and proposal ID. The action never writes
`.gitclaw/skill-sources`, never writes `.gitclaw/SKILLS`, never copies raw
source refs or source request text into the proposal issue or receipt, never
contacts registries, never fetches remote sources, never runs installers, and
never calls a model. Reviewers continue on the proposal issue for a real
GitHub Models discussion before drafting the source-pin YAML on a normal
code-review branch.

The source-pin proposal action also accepts `--notify-route <route>` and
`--notify-routes <a,b>`. When present, GitClaw resolves reviewed channel routes
from `.gitclaw/channels/routes.yaml`, queues a metadata-safe
`gitclaw:channel-outbound` notification for each route, and records only
counts and hashes in the source receipt. The outbound channel body may include
the safe source name, source kind, source-ref hash, source-pin path, proposed
skill path, and review issue URL, but it must not include raw source refs,
raw source request text, candidate skill bodies, or route names. Provider
delivery remains delegated to `gitclaw channel-outbox` and
`gitclaw channel-delivery`; the proposal action itself never calls provider
APIs.

Skill source reports never contact ClawHub, Hermes Hub, skills.sh, GitHub, or
well-known endpoints; never fetch remote sources; never run installers; never
install dependencies; never write `.gitclaw/SKILLS`; and never print raw
source refs, raw lockfiles, raw source-pin bodies, raw skill bodies, issue
bodies, comments, prompts, git subjects, author identities, provider payloads,
credentials, or secret values. The reports include
`llm_e2e_required_after_skill_source_change=true` or
`llm_e2e_required_after_skill_source_verify_change=true` or
`llm_e2e_required_after_skill_source_lock_change=true` or
`llm_e2e_required_after_skill_source_update_plan_change=true` or
`llm_e2e_required_after_skill_source_info_change=true` or
`llm_e2e_required_after_skill_source_search_change=true` or
`llm_e2e_required_after_skill_source_provenance_change=true`; every source-pin
behavior change must ship with a live GitHub Models follow-up E2E.

When called as `@gitclaw /skills runtime`,
`@gitclaw /skills requirements`, `@gitclaw /skills metadata`, or
`gitclaw skills runtime`, GitClaw posts a body-free runtime metadata audit for
repo-local `SKILL.md` frontmatter. The report parses OpenClaw-compatible
`metadata.openclaw` declarations plus Hermes/mini-claw style compatibility
namespaces such as `metadata.clawdbot` and `metadata.clawdis`, then summarizes:

- skills with frontmatter and runtime metadata,
- required and optional env declaration counts,
- primary env declaration counts, match counts, and short hashes,
- required binary declaration counts,
- inert install spec counts, install kind names, install target hashes, and
  install-bin totals,
- missing env/bin requirement counts for enabled skills,
- explicit `registry_contact_allowed=false`, `installer_scripts_run=false`,
  `dependency_install_allowed=false`, and `repository_mutation_allowed=false`
  gates.

The runtime report never contacts ClawHub, Hermes Hub, GitHub, package
registries, or well-known endpoints; never runs installers; never installs
dependencies; never mutates `.gitclaw/SKILLS`; and never prints raw skill
bodies, raw env names, raw install targets, issue/comment bodies, prompts,
provider payloads, tool outputs, credentials, or secret values. Findings are
limited to codes such as `missing_runtime_requirements`,
`primary_env_not_declared`, and `declared_install_specs_inert`, with paths and
body-free detail. The report includes
`llm_e2e_required_after_skill_runtime_change=true`; every runtime metadata
behavior change must ship with a live GitHub Models follow-up E2E that proves
normal model inference, repo-local skill selection, and prompt-visible tool
usage still work.

When called as `@gitclaw /skills proposal-plan <name>` or
`gitclaw skills proposal-plan <name>`, GitClaw posts a non-mutating proposal
planner inspired by OpenClaw's Skills Workshop proposal lifecycle and Hermes'
reviewable skill reuse model. The planner derives a safe lower hyphen-case
candidate, reports the proposal path
`.gitclaw/skill-proposals/<name>/PROPOSAL.md`, the future active
`.gitclaw/SKILLS/<name>/SKILL.md` path, whether the plan is a proposed create
or update, existing skill matches, request hashes, validation rollups, and
review steps.

The proposal planner never writes proposal files, never updates active skills,
never fetches remote sources, never runs installers or dependency setup, never
creates commits, never auto-applies a proposal, and never performs autonomous
skill creation or self-improvement. It reports only metadata, hashes, counts,
safe paths, and finding codes; raw proposal text, issue/comment bodies, and
full skill bodies stay out of the issue-visible report. Accepted proposal work
must land through normal reviewed git changes and then pass skill validation,
skill risk checks, and a live GitHub Models conversation E2E.

When called as `@gitclaw /skills propose <name>`, GitClaw performs the
issue-native proposal action. It opens or reuses a dedicated GitHub issue
marked with `gitclaw:skill-proposal-issue`, records the safe proposal name,
source request hash, source issue/comment identity, proposal path, destination
skill path, and create/update intent, then posts a body-free receipt in the
source conversation. Duplicate open proposal issues are suppressed by marker
and safe name.

The proposal action also accepts `--notify-route <route>` and
`--notify-routes <a,b>`. When present, GitClaw resolves reviewed channel routes
from `.gitclaw/channels/routes.yaml`, queues one metadata-safe
`gitclaw:channel-outbound` notification per destination, and records only
counts and hashes in the source receipt. The channel notification body may name
the safe proposal name, review issue, proposal path, and destination skill
path, but it must not copy raw source request text, candidate skill bodies, or
route names. Provider delivery remains outside the action and is handled by
`gitclaw channel-outbox` plus `gitclaw channel-delivery`.

The proposal issue action does not write `.gitclaw/skill-proposals`, does not
write `.gitclaw/SKILLS`, does not copy raw issue/comment text into the
proposal issue or receipt, and does not call a model. When channel
notifications are requested, it still does not call provider APIs. It is a
GitHub-native review queue entry: after review, a maintainer can draft the
proposal file on a normal branch, run validation/risk checks, and promote it
to an active skill only through a reviewed git change.

When called as `@gitclaw /skills rehearse <name> --id <id>`, GitClaw performs
the issue-native rehearsal action. It opens or reuses a dedicated GitHub issue
marked with `gitclaw:skill-rehearsal-issue`, labels it with the GitClaw trigger
label so normal comments become ordinary agent turns, records the requested
skill, source issue/comment identity, source hash, validation rollup, and
matched-skill counts, then posts a body-free receipt in the source
conversation. Duplicate open rehearsal issues are suppressed by the hidden
marker and safe rehearsal id.

The rehearsal action does not install skills, edit `.gitclaw/SKILLS`, fetch
remote registries, run setup scripts, copy raw issue/comment text, copy raw
skill bodies, or call a model. It is a conversation lane for trying the current
reviewed skill behavior: after creation, a maintainer comments on the
rehearsal issue, and the live E2E must prove a real GitHub Models response
with `prompt_context_sha256_12`, selected skill metadata, prompt-visible
tools, `gitclaw.search_files`, and token usage telemetry.

When called as `@gitclaw /skills proposals`, `@gitclaw /skills proposals
risk`, or `gitclaw skills proposals [risk]`, GitClaw inventories the reviewed
proposal store at `.gitclaw/skill-proposals/*/PROPOSAL.md`. This is the
GitHub-native analogue of OpenClaw Skills Workshop `status`, `list_pending`,
and `list_quarantine`, but backed by ordinary repo files instead of gateway
state. The report publishes proposal counts by lifecycle status, safe path and
frontmatter metadata, proposal hashes, risk finding codes, and line hashes.

The proposal inventory never activates proposals, never writes proposal files,
never updates active skills, never runs support scripts, never fetches remote
sources, and never dumps proposal or skill bodies. It treats the proposal
store as inert review material until a maintainer manually converts an accepted
proposal into `.gitclaw/SKILLS/<name>/SKILL.md` on a reviewed branch.

When called as `@gitclaw /skills install-plan <target>` or
`@gitclaw /skills upgrade-plan <target>`, the command switches to a
non-mutating install planner inspired by OpenClaw's ClawHub/AgentSkills
install UX and Hermes' skill trust posture. The planner classifies the target
as a registry name, local skill path, GitHub shorthand, GitHub URL, generic
HTTPS URL, insecure HTTP URL, unsupported URL, unsafe path, or empty target. It
reports only safe metadata: target hash, target type, derived safe
repo-local name, destination path candidate, existing repo-local matches,
existing skill hashes, upgrade target requirements, validation rollup, and
findings.

The install planner never fetches remote targets, never contacts a registry,
never runs installer scripts, never installs dependencies, never writes
`.gitclaw/SKILLS`, and never commits or pushes. Remote URLs are classified only
and require manual review. Existing skill matches are reported as upgrade or
overwrite risk. For `install-plan`, the report includes
`llm_e2e_required_after_skill_install_plan_change=true`; for `upgrade-plan`,
an existing repo-local skill match is required, the report includes
`existing_skill_required=true` and
`llm_e2e_required_after_skill_upgrade_plan_change=true`. The live E2E for
each planner must post a model-backed follow-up that proves selected skill
metadata, prompt-context provenance, `gitclaw.search_files`, and token-usage
markers.
The report includes `llm_e2e_required_after_change=true` to make the release
rule explicit: after a skill is actually changed, maintainers must run a live
GitHub Models conversation E2E in addition to deterministic skill-report
tests.

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
@gitclaw /profile catalog
@gitclaw /profile provenance
@gitclaw /profile search <query>
@gitclaw /profile diff [base-ref]
@gitclaw /profile snapshot
@gitclaw /profile manifest
@gitclaw /profile export-plan
@gitclaw /profile risk
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
- soul, skill, and tool validation rollups,
- body-free profile git provenance metadata when explicitly requested,
- body-free profile search, git-diff, snapshot, and portability manifest
  metadata when explicitly requested.

It never dumps profile file bodies, skill bodies, tool outputs, issue/comment
bodies, prompts, or secrets. It is an operator confidence surface; the
manifest view is a dry-run metadata plan, not a packaging, install, switch, or
mutation command.

Local operators can inspect the same profile envelope without opening an issue:

```bash
gitclaw profile catalog
gitclaw profile show
gitclaw profile verify
gitclaw profile provenance
gitclaw profile search <query>
gitclaw profile diff [base-ref]
gitclaw profile snapshot
gitclaw profile manifest
gitclaw profile export-plan
gitclaw profile risk
```

`show`, `verify`, and `list` intentionally return the same body-free envelope
in v1. `manifest` and `export-plan` return the same body-free portability
manifest.

`@gitclaw /profile catalog` and `gitclaw profile catalog` add a compact
profile discovery surface before the manifest and risk views. The catalog maps
supported profile commands and repo-local layers across identity, user, soul,
memory, skills, bundles, tools, models, proactive prompts, hooks, channels,
backups, and sessions. It reports counts, gates, and command availability only;
raw profile file bodies, skill bodies, tool outputs, issue/comment bodies,
prompts, credentials, sessions, and backup payloads are excluded. Any catalog
change must include live GitHub issue E2E plus a GitHub Models follow-up that
selects `repo-reader`, exposes `gitclaw.search_files`, and recovers a
repository-search fixture token.

`@gitclaw /profile provenance`, `@gitclaw /profile history`, and
`gitclaw profile provenance` map the repo-local `.gitclaw/` profile envelope
to body-free git provenance. The report enumerates profile paths, categories,
content hashes, tracked state, dirty state, last commit IDs/dates, and
commit-subject hashes only. It intentionally excludes raw profile files, skill
bodies, memory bodies, tool outputs, issue/comment bodies, prompts, git commit
subjects, author identities, sessions, backup payloads, credentials, and
secret values. Any change to this surface requires focused local tests, a live
deterministic issue E2E, and a normal GitHub Models follow-up proving
repo-reader selection, prompt-visible `gitclaw.search_files`, usage telemetry,
and repository-search fixture recovery.

`@gitclaw /profile search <query>` and `gitclaw profile search <query>` search
the repo-local `.gitclaw/` profile envelope with a deterministic lexical
matcher. This is the GitHub-native, body-free sibling of OpenClaw-style memory
search and Hermes-style session/tool search: it can inspect reviewed profile
files locally, but the posted report includes only the query hash, term count,
profile manifest counts, searched paths, line numbers, scores, matched-term
counts, file hashes, and line hashes. Raw profile files, skill bodies, memory
bodies, tool outputs, issue/comment bodies, prompts, sessions, backup payloads,
credentials, and raw search queries are excluded. Any change to this surface
requires focused local tests, a live deterministic issue E2E, and a normal
GitHub Models follow-up proving repo-reader selection, prompt-visible
`gitclaw.search_files`, usage telemetry, and repository-search fixture
recovery.

`@gitclaw /profile diff [base-ref]`, `@gitclaw /profile changes`, and
`gitclaw profile diff [base-ref]` compare repo-local `.gitclaw/` profile files
against a git base ref using `git diff --name-status` and `git diff --numstat`
metadata only. This is GitClaw's repo-native profile review loop inspired by
Hermes checkpoints/worktree diffs and OpenClaw's preview-before-migration
posture. The report emits the requested-ref hash, resolved base/head commit
hashes, changed profile counts, path/status cards, numstat counts, and file
hashes. It excludes raw patches, profile bodies, skill bodies, memory bodies,
issue/comment bodies, prompts, requested ref text, git subjects, author
identities, sessions, backup payloads, credentials, and secret values. Any
change to this surface requires focused local tests, a live deterministic issue
E2E, and a normal GitHub Models follow-up proving repo-reader selection,
prompt-visible `gitclaw.search_files`, usage telemetry, and repository-search
fixture recovery.

`@gitclaw /profile snapshot`, `@gitclaw /profile fingerprint`, and
`gitclaw profile snapshot` produce a composite body-free fingerprint for the
repo-local profile envelope. The report binds the profile manifest, soul
snapshot, memory snapshot, skill snapshot, and tool snapshot with one
`snapshot_sha256_12` plus component hashes. It reports component statuses,
entry counts, prompt-visible counts, portability/selection/enabled counts, and
disabled export/import/switching/mutation/session/backup/raw-body gates only.
Raw profile files, skill bodies, memory bodies, tool outputs, prompts,
issue/comment bodies, sessions, backup payloads, credentials, and secret
values are excluded. Any change to this surface requires focused local tests,
a live deterministic issue E2E, and a normal GitHub Models follow-up proving
repo-reader selection, prompt-visible `gitclaw.search_files`, usage telemetry,
and repository-search fixture recovery.

`@gitclaw /profile manifest`, `@gitclaw /profile portability`, and
`@gitclaw /profile export-plan` produce a deterministic dry-run manifest for
the repo-local profile. The manifest maps the reviewed `.gitclaw/` profile
surface into metadata-only cards:

- config metadata,
- loaded soul/user/identity/tool/memory/heartbeat/policy files,
- dated memory notes,
- local skills and skill bundles,
- reviewed proactive prompts, hook specs, MCP specs, toolsets, task specs,
  node specs, artifact specs, diff specs, agent specs, and workspace specs,
- deterministic tool contracts.

It also names state that is deliberately excluded: credentials, external
profile homes, sessions, backup payloads, and profile mutation/install/switch
operations. It reports paths, categories, inclusion policies, portability
flags, selected/enabled flags, counts, and short hashes only. Raw profile
files, config bodies, issue/comment bodies, prompts, sessions, backups, and
credential values are never printed. Any change to the manifest surface must
include live GitHub issue E2E with a normal GitHub Models follow-up proving
repo-reader and prompt-visible tool provenance.

`@gitclaw /profile risk` and `gitclaw profile risk` add a deterministic
profile-isolation audit on top of the visibility report. The audit is inspired
by Hermes' profile separation and OpenClaw's workspace-as-memory model, but
keeps GitClaw's v1 boundary stricter: a repository is one reviewed profile, and
GitClaw does not support profile import/export, profile switching, profile
installation, profile credential storage, or profile mutation commands.

The risk report scans only repo-local profile metadata and bounded file bodies
already loaded into the GitClaw context plus `.gitclaw/config.yml` metadata. It
publishes:

- required profile document coverage,
- profile/config/skill card metadata,
- profile isolation flags for import/export, switching, mutation, credential
  storage, and sandbox-boundary claims,
- finding codes, severities, paths, fields, and line hashes for prompt
  boundary overrides, credential material, external profile state, unsafe
  profile portability, switching, mutation, sandbox-boundary confusion, and raw
  body leakage.

It never prints profile file bodies, config bodies, skill bodies, tool outputs,
issue/comment bodies, prompts, credentials, or secret values. Any change to
this surface requires both local unit coverage and a live GitHub issue E2E that
includes a normal GitHub Models follow-up turn with repo-reader/tool usage.

## Migration Plan Command

GitClaw supports a deterministic migration planner inspired by OpenClaw's
preview-first migration model and Hermes' isolated profile directories:

```text
@gitclaw /migrate plan hermes
@gitclaw /migration openclaw
@gitclaw /migrate risk hermes
```

```bash
gitclaw migrate plan hermes
gitclaw migrate plan openclaw
gitclaw migrate risk hermes
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

When called as `@gitclaw /migrate risk <source>` or
`gitclaw migrate risk <source>`, the command posts a
`GitClaw Migration Risk Report`. It keeps the same no-source-read boundary but
classifies the provider import map into credential, executable-state, memory,
skill, session-archive, identity, config, and declarative-state risk cards. The
report emits counts for manual-copy, reviewed-merge, reviewed-append,
manual-rewrite, manual-review, archive-only, skipped, credential, executable,
memory, skill, and session-archive items; it also reports disabled apply,
repository mutation, credential import, installer execution, MCP autoload, raw
source-body, raw issue-body, raw comment-body, and raw secret-value flags. Risk
findings are code-and-hash metadata only, such as
`credential_import_disabled`, `executable_state_quarantined`,
`raw_state_archive_only`, `skill_manual_review_required`,
`memory_review_required`, and `manual_review_required`.

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
@gitclaw /runs history
@gitclaw /runs timeline
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
The report includes `llm_e2e_required_after_runs_report_change=true`; changes
to this surface must be paired with a live GitHub Models follow-up that selects
`repo-reader`, exposes `gitclaw.search_files`, and recovers a bounded
repository-search fixture token without echoing issue-body or run sentinels.

Local operators can inspect the same body-free local run envelope without
opening an issue:

```bash
gitclaw runs current
gitclaw runs verify
```

Run history is a second body-free view over the same issue-native ledger. It is
inspired by OpenClaw's trajectory/progress record, which treats execution steps
and tool calls as inspectable run metadata, and Hermes' session-list/session-show
model, which makes prior sessions replayable without requiring a resident
server. GitClaw's cut is intentionally smaller: `@gitclaw /runs history` reads
only prior `gitclaw:assistant-turn` markers from the issue comments and emits:

- assistant turn count, model-backed count, deterministic count, unique run ID
  count, and prompt-provenance coverage,
- model names and prompt-visible skill/tool names,
- one timeline entry per prior assistant marker with comment source, run ID,
  event ID, deterministic/model-backed classification, prompt-context hash,
  context/skill/tool-output counts, selected skill/tool names, and comment hash,
- idempotency key and Actions run URL hashes, never their raw values,
- explicit `raw_bodies_included=false`, `raw_run_payloads_included=false`,
  `raw_tool_outputs_included=false`, and `raw_prompts_included=false` gates.

Local operators can reconstruct the same history from fetched backup JSON:

```bash
gitclaw runs history --backup <issue.json>
```

The live `github-runs-history-report.sh` E2E must create a real issue, wait for
an initial GitHub Models conversation that uses `repo-reader` and
`gitclaw.search_files`, post `@gitclaw /runs history`, assert that the report
lists the previous model-backed turn without leaking the model reply or request
bodies, and then post another normal comment that performs a second GitHub
Models tool-backed turn. This keeps the deterministic report honest: it proves
the history surface against actual LLM calls, not just synthetic markers.

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
gitclaw soul search <query> --max-results 10
```

The validation output includes only paths, counts, and short finding details.
It never dumps full soul, user, memory, tool, or heartbeat file bodies.

## Soul Inspection Command

GitClaw supports a deterministic high-authority context audit command inspired
by OpenClaw and Hermes' portable workspace files:

```text
@gitclaw /soul
@gitclaw /soul catalog
@gitclaw /soul anchors
@gitclaw /soul snapshot
@gitclaw /soul provenance
@gitclaw /soul list
@gitclaw /soul verify
@gitclaw /soul risk
@gitclaw /soul validate
@gitclaw /soul edit-plan soul
@gitclaw /soul propose --target soul --id warm-tone-soul
@gitclaw /soul rehearse --target soul --id warm-tone-soul
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
- high-authority anchor roles, authority layers, prompt-visible flags,
  canonical path flags, and mutation-disabled gates when explicitly requested.
- compact high-authority catalog metadata with load modes, reason codes,
  profile/export gates, and body/description-free hash boundaries when
  explicitly requested.
- high-authority git provenance with tracked state, last commit IDs/dates, and
  commit-subject hashes when explicitly requested.
- high-authority edit planning metadata when explicitly requested.

It never dumps full file bodies. The hashes make the issue-visible report
verifiable without exposing private user, memory, or policy text.
`@gitclaw /soul list` is an explicit inventory alias for the same report,
matching the local `gitclaw soul list` helper.

When called as `@gitclaw /soul anchors`, `@gitclaw /soul authority`, or
`@gitclaw /soul map`, the command posts a body-free authority map for the
repo-local context hierarchy. The report treats OpenClaw workspace files and
Hermes profile files as anchors: `SOUL.md` for persona/boundaries,
`IDENTITY.md` for agent identity, `USER.md` for user profile, `TOOLS.md` for
tool guidance, `MEMORY.md` and dated memory notes for memory, `HEARTBEAT.md`
for proactive checks, and optional policy files such as standing orders,
hooks, plugins, tasks, nodes, artifacts, diffs, and workspace policy. It
reports anchor names, roles, authority layers, sources, required/loaded/
prompt-visible/canonical/latest flags, byte and line counts, short hashes,
frontmatter/description presence, validation gates, risk gates, and mutation
gates only. It never emits raw file, issue, comment, prompt, or secret bodies.
Changes to this report must be paired with a live GitHub Models follow-up E2E
that proves normal inference and prompt-visible tool provenance.

When called as `@gitclaw /soul snapshot`, `@gitclaw /soul fingerprint`, or
`@gitclaw /soul lock`, the command posts a deterministic body-free fingerprint
of the same anchor graph. It emits per-anchor paths, categories, roles,
authority layers, load states, short body hashes, risk counts, and one
composite `snapshot_sha256_12` over the metadata. This is the OpenClaw/Hermes
profile-lock equivalent for GitClaw: enough to compare whether the repo-stored
agent identity, user profile, memory, tools, heartbeat, and policy surface
changed, without exporting or mutating the profile. Registry contact,
profile export, repository mutation, and raw-body gates remain disabled.
Snapshot changes require a live GitHub issue E2E plus a normal GitHub Models
repo-reader/search follow-up.

When called as `@gitclaw /soul catalog`, `@gitclaw /soul index`,
`@gitclaw /soul profile-catalog`, or `@gitclaw /soul authority-catalog`, the
command posts a compact authority-discovery catalog. The report reuses the
soul anchor graph, then adds catalog-level counts, authority-layer names, load
modes, reason codes, profile isolation metadata, `raw_bodies_included=false`,
`raw_descriptions_included=false`, `profile_export_allowed=false`, and
mutation-disabled gates. It is the OpenClaw/Hermes-inspired progressive
disclosure view: maintainers can see which high-authority files exist, which
ones are loaded for the turn, and which authority layer each anchor belongs
to without printing raw soul, identity, user, memory, tool, heartbeat, issue,
comment, prompt, description, or secret bodies. This mirrors
`gitclaw soul catalog` for local review and must ship with a live GitHub
Models follow-up E2E that proves repo-reader search, selected skill metadata,
prompt-visible tools, and usage telemetry.

When called as `@gitclaw /soul provenance`, `@gitclaw /soul history`, or
`@gitclaw /soul timeline`, the command posts a body-free git provenance map
for loaded high-authority context. It reports repo-local document counts,
tracked/untracked counts, commit availability, last commit SHA/date metadata,
commit-subject hashes, validation/risk gates, and mutation-disabled state. It
does not print raw file bodies, issue bodies, comments, prompts, commit
subjects, author names, author emails, or secret values. This mirrors
`gitclaw soul provenance` for local review.

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
`llm_e2e_required_after_change=true` and
`llm_e2e_required_after_soul_edit_plan_change=true` to make the release rule
explicit: after a soul file is actually changed, maintainers must run a live
GitHub Models conversation E2E in addition to deterministic soul-report tests.
Changes to the planner itself must also run the live follow-up that proves
selected skill metadata, prompt-context provenance, `gitclaw.search_files`,
and token-usage markers.

When called as `@gitclaw /soul propose --target <path> --id <id>`, GitClaw
performs the issue-native soul proposal action. It opens or reuses a dedicated
GitHub issue marked with `gitclaw:soul-proposal-issue`, records the safe
proposal id, source request hash, source issue/comment identity, normalized
target path, category, current target hash/size/load state, validation rollup,
and risk rollup, then posts a body-free receipt in the source conversation.
Duplicate open soul proposal issues are suppressed by marker and proposal id.

When the same command includes `--notify-route <route>` or
`--notify-routes <route-a,route-b>`, GitClaw also queues one reviewed outbound
channel notification per route. The source receipt records only route/message
body hashes, destination issue numbers, queue counts, duplicate counts, channel
names, and provider-delivery gates; it never prints raw route names, raw
provider thread ids, raw outbound body text, source text, candidate soul text,
or existing soul text. The outbound channel comment may include the proposal
issue, source issue, proposal id, target path, and review status so the
external channel receives useful context, while actual provider delivery stays
delegated to `gitclaw channel-outbox` and `gitclaw channel-delivery`.

The soul proposal action does not write `.gitclaw/SOUL.md`,
`.gitclaw/IDENTITY.md`, `.gitclaw/USER.md`, `.gitclaw/TOOLS.md`, memory files,
or any related high-authority context; it does not generate candidate soul
text, does not copy raw issue/comment text into the proposal issue or receipt,
and does not call a model. It is a GitHub-native review queue entry: after
review, a maintainer can draft the high-authority context diff on a normal
branch, run soul/profile validation and risk checks, and let the change affect
future prompts only through a reviewed git change.

When called as `@gitclaw /soul rehearse --target <path> --id <id>`, GitClaw
performs the issue-native soul rehearsal action. It opens or reuses a dedicated
GitHub issue marked with `gitclaw:soul-rehearsal-issue`, labels it with the
normal trigger label, records the source hash, source issue/comment identity,
normalized target path, category, current target hash/size/load state,
validation rollup, risk rollup, and no-write/no-generation gates, then posts a
body-free receipt in the source conversation. Duplicate open soul rehearsal
issues are suppressed by marker and rehearsal id.

The soul rehearsal action does not write `.gitclaw/SOUL.md`,
`.gitclaw/IDENTITY.md`, `.gitclaw/USER.md`, `.gitclaw/TOOLS.md`, memory files,
or any related high-authority context; it does not generate candidate soul
text, does not copy raw target/source bodies into the rehearsal issue or
receipt, does not mutate the repository, and does not call a model in the
source action. The new issue is a GitHub-native conversation lane for trying
the current high-authority behavior. Every change to this surface must create a
real rehearsal issue, suppress a duplicate request, and then continue on the
rehearsal issue with a GitHub Models turn that proves prompt provenance,
selected `repo-reader`, prompt-visible `gitclaw.search_files`, and usage
telemetry.

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
- `gitclaw.skill_index`: exposes local skill names, paths, gates, hashes, and
  requirement counts.
- `gitclaw.policy`: exposes read-only policy output when the issue thread
  contains write intent.

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
gitclaw tools search <query> --max-results 10
```

The validation output includes only names, counts, and short finding details.
It never dumps full tool outputs, file bodies, or search result bodies.

## Tools Inspection Command

GitClaw supports a deterministic tool-surface audit command inspired by
OpenClaw's tool policy visibility and Hermes' toolset inventory:

```text
@gitclaw /tools
@gitclaw /tools catalog
@gitclaw /tools snapshot
@gitclaw /tools list
@gitclaw /tools verify
@gitclaw /tools risk
@gitclaw /tools validate
@gitclaw /tools exposure
@gitclaw /tools exposure risk
@gitclaw /tools defer-plan
@gitclaw /tools boundary
@gitclaw /tools provenance
@gitclaw /tools toolsets
@gitclaw /tools toolsets risk
@gitclaw /tools toolsets provenance
@gitclaw /tools toolsets info repo-read
@gitclaw /tools approval-plan search_files
@gitclaw /tools run-plan search_files
@gitclaw /tools rehearse search_files --id search-contract-lab
@gitclaw /tools request-run search_files --id reviewed-search-run
@gitclaw /tools request-run search_files --id reviewed-search-run --notify-route team-alerts
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
- tool provenance status, prompt-visible tool names, active-output counts,
  read-only versus metadata-only output counts, input/output hashes, and
  hash-only provenance gates for the current turn.

It never dumps full tool output bodies. Tool output bodies remain prompt inputs
only; the issue-visible report exposes enough metadata to debug whether
`gitclaw.list_files`, `gitclaw.search_files`, `gitclaw.read_file`,
`gitclaw.skill_index`, or `gitclaw.policy` ran for the turn.
The report includes `llm_e2e_required_after_tool_report_change=true`; changes
to this surface must be paired with a live GitHub Models follow-up that selects
`repo-reader`, exposes `gitclaw.search_files`, and recovers a bounded
repository-search fixture token without echoing tool-output or issue-body
sentinels.
`@gitclaw /tools list` is an explicit inventory alias for the same report,
matching the local `gitclaw tools list` helper.

`@gitclaw /tools catalog` and `gitclaw tools catalog` expose the compact
progressive-disclosure catalog inspired by OpenClaw's tool policy visibility
and Hermes' Tool Search catalog. It combines built-in deterministic contracts,
repo-reviewed toolset profiles, and MCP allowlist entries into one body-free
index with direct/deferred mode, schema-visibility mode, activation decision,
reason codes, gate state, counts, and hashes. It never prints raw tool schemas,
toolset instructions, MCP command args, tool inputs, tool outputs, issue
bodies, comments, prompts, credentials, or secret values. The report includes
`llm_e2e_required_after_tool_catalog_change=true`; every change must ship with
a live GitHub issue E2E for the catalog plus a GitHub Models follow-up that
selects `repo-reader`, exposes `gitclaw.search_files`, and recovers a fresh
repository-search fixture token.

`@gitclaw /tools snapshot` and `gitclaw tools snapshot` expose a stable,
body-free fingerprint for the whole deterministic tool surface. It combines
built-in contracts, repo-reviewed toolsets, MCP allowlist entries,
`.gitclaw/TOOLS.md` guidance metadata, and prompt-visible active-output
metadata into per-entry hashes plus one composite `snapshot_sha256_12`. The
`fingerprint`, `fingerprints`, `lock`, and `lockfile` aliases route to the same
report. It never prints raw tool schemas, toolset instructions, MCP command
args, tool inputs, tool outputs, issue bodies, comments, prompts, credentials,
or secret values, and keeps registry contact, dynamic MCP discovery, runtime
MCP launch, structured tools, shell execution, and repository mutation gates
disabled. The report includes
`llm_e2e_required_after_tool_snapshot_change=true`; every change must ship with
a live GitHub issue E2E plus a GitHub Models follow-up that selects
`repo-reader`, exposes `gitclaw.search_files`, and recovers a fresh repository
search fixture token.

`@gitclaw /tools exposure` and `gitclaw tools exposure` make the model-visible
tool boundary explicit. Inspired by OpenClaw's tool allow/deny/profile
visibility and Hermes' Tool Search progressive-disclosure design, the report
lists static GitClaw tool contracts, enabled/disabled/allowlist-blocked gate
state, prompt-visible output counts, structured-tool bridge status, and
fail-closed status. GitClaw v1 does not expose model-callable structured tool
schemas and does not defer schemas behind a Hermes-style bridge; it provides
bounded pre-model tool outputs and hashed provenance instead.

`@gitclaw /tools exposure risk` and `gitclaw tools exposure risk` add finding
codes for explicit allowlists that resolve to zero enabled tools, validation
errors or warnings, unknown active outputs, mutating contracts, and the
static/bridge boundary. They never print raw tool schemas, tool inputs, tool
outputs, issue bodies, comments, prompts, credentials, or secret values. The
report includes `llm_e2e_required_after_tool_exposure_change=true`; every
change to this surface must ship with a live GitHub Models follow-up E2E.

`@gitclaw /tools defer-plan` and `gitclaw tools defer-plan` provide a
body-free advisory plan for Hermes-style progressive disclosure. The report
combines built-in deterministic tool contracts, repo-reviewed toolset profiles,
and MCP allowlist entries, then estimates whether a future bridge should keep
entries direct or defer them behind `tool_search` / `tool_describe` /
`tool_call`-style controls. GitClaw v1 keeps this as analysis only:
`model_callable_structured_tools=false`,
`tool_search_bridge_runtime_enabled=false`,
`mcp_server_launch_allowed=false`, and
`toolset_activation_supported=false`. The report emits catalog counts,
thresholds, activation decision, card metadata, risk codes, and hashes; it
never prints raw tool schemas, toolset instructions, MCP spec bodies, command
args, issue/comment bodies, prompts, credentials, or tool outputs. The report
includes `llm_e2e_required_after_tool_defer_plan_change=true`; every change to
this surface must pair the deterministic issue-command E2E with a live GitHub
Models follow-up that proves prompt-visible tools still reach inference.

`@gitclaw /tools boundary` and `gitclaw tools boundary [query]` focus on the
prompt-side delimiter between deterministic tool output and model instructions.
The report treats active tool outputs as untrusted prompt-visible data, scans
tool inputs/outputs/guidance for prompt-boundary override, credential,
exfiltration, host-exec, mutation, network, and loop-risk phrases, and emits
only counts, tool names, modes, hashes, risk codes, gate results, and line
hashes. It records that GitClaw v1 uses `[tool_output ...]` prompt blocks,
does not expose model-callable structured tools, and does not allow shell,
network, or repository-mutation tools. Raw tool inputs, raw outputs, search
queries, issue bodies, comments, prompts, credentials, and secrets are never
included. The report includes
`llm_e2e_required_after_tool_boundary_change=true`; every change to this surface
must pair the deterministic issue-command E2E with a live GitHub Models
follow-up that proves prompt-visible tool outputs still reach inference.

`@gitclaw /tools provenance` and `gitclaw tools provenance [query]` provide
the turn-level audit of which deterministic tool outputs fed the prompt. The
report is the body-free equivalent of inspecting an OpenClaw workspace/tool
run or a Hermes toolset/session preview: it lists active tool names, contract
modes, enabled/disabled/allowlist gate state, prompt-visible names, output
sizes, input/output hashes, per-output risk codes, and explicit hash-only
input/output gates. It never prints raw inputs, raw outputs, file bodies,
search result bodies, issue/comment bodies, prompts, credentials, or secrets.
The report includes
`llm_e2e_required_after_tool_provenance_change=true`; every change to this
surface must pair the deterministic issue-command E2E with a live GitHub
Models follow-up that proves prompt-visible tools still reach the model path.

### Toolset Profiles

GitClaw also supports repo-reviewed toolset profile files:

```yaml
name: repo-read
description: Read-only repository context tools for ordinary issue answers.
mode: read-only
tools:
  - gitclaw.list_files
  - gitclaw.search_files
  - gitclaw.read_file
  - gitclaw.skill_index
  - gitclaw.policy
instruction: |
  Prefer bounded repository search and explicit file references.
```

Toolsets live in `.gitclaw/toolsets/*.yaml`. They mirror the useful part of
Hermes/OpenClaw toolsets: a named, reviewed task profile that declares which
tool contracts belong together. In GitClaw v1 they are advisory inventory and
risk surfaces only. They do not dynamically activate tools, bypass
`tools.allowed`/`tools.disabled`, call providers, execute shell commands, or
grant repository write permissions.

`@gitclaw /tools toolsets` and `gitclaw tools toolsets` list the profiles by
path, normalized tool refs, resolved/unknown refs, config gate state, hashes,
and whether an instruction/description exists. `@gitclaw /tools toolsets risk`
and `gitclaw tools toolsets risk` scan toolset YAML for unknown tool refs,
disabled/allowlist-blocked refs, non-read-only modes, prompt-boundary
overrides, secret exfiltration instructions, credential material, host
execution, repository mutation, remote exfiltration, and unbounded loops.
`@gitclaw /tools toolsets provenance`,
`gitclaw tools toolsets provenance`, and the `history`/`timeline` aliases map
repo-local toolset YAML files to git history without exposing their bodies.
They report profile names, paths, normalized/resolved tool refs, config gates,
risk codes, file hashes, tracked/dirty state, last commit IDs/dates, and
commit-subject hashes only. They never print raw toolset YAML, reviewed
instructions, tool outputs, git commit subjects, author identities,
issue/comment bodies, prompts, credentials, or secret values.
`@gitclaw /tools toolsets info <name>` and
`gitclaw tools toolsets info <name>` show one profile with the normalized
lookup hash, match status, activation/mutation/network gates, resolved tool
refs, risk codes, and `llm_e2e_required_after_toolset_info_change=true`. All
four reports are body-free, and changes require a live GitHub Models follow-up
E2E.

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
The report includes `llm_e2e_required_after_tool_verify_change=true`; changes
to this trust envelope must be paired with a live GitHub Models follow-up that
selects `repo-reader`, exposes `gitclaw.search_files`, and recovers a bounded
repository-search fixture token without echoing trust-card, tool-input, or
issue-body sentinels.

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

When called as `@gitclaw /tools approval-plan <name>`, the command posts a
body-free approval dry run for one declared tool contract. It is the
GitHub-native slice of OpenClaw's exec approval posture and Hermes' explicit
tool authorization boundary: the report shows the normalized tool, match
count, enabled/disabled/allowlist gate state, contract mode, trigger, mutation
flag, active-output hashes, validation summary, per-issue approval labels, and
whether approval would be required before a future write-capable mode. In v1
all built-in contracts are read-only or metadata-only, so known enabled tools
report `approval_required=false`,
`approval_decision=no_approval_required_read_only`,
`model_callable_structured_tools=false`, `shell_execution_allowed=false`, and
`repository_mutation_allowed=false`. It never approves, executes shell,
calls a model, makes network calls, mutates the repository, or emits raw tool
names from the issue, raw tool inputs, raw outputs, approval payloads,
issue/comment bodies, prompts, credentials, or file bodies. Any implementation
change to tool approval behavior must pair the deterministic approval-plan E2E
with a live GitHub Models conversation E2E.

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

When called as `@gitclaw /tools rehearse <name> --id <id>`, the command opens
or reuses a dedicated GitHub issue marked `gitclaw:tool-rehearsal-issue` and
labeled with `gitclaw` so a normal model-backed conversation can continue
there. This is not a run request and not tool execution: it records the
rehearsal id, normalized tool, matched contract, gate state, validation
summary, source hashes, and no-execution flags in a separate conversation
issue, then posts a body-free receipt back to the source issue. It never calls
a model, executes a tool, generates raw tool inputs, creates a reviewed
tool-run request, makes network calls, mutates the repository, or emits the
raw source request, raw requested tool text, raw tool inputs, raw tool outputs,
prompts, credentials, or file bodies. Duplicate open rehearsals are suppressed
by rehearsal id. Any implementation change to this action must pair the
deterministic create/reuse E2E with a live GitHub Models follow-up on the
rehearsal issue using `repo-reader` and bounded repository search.

When called as `@gitclaw /tools request-run <name> --id <id>`, the command
opens or reuses a dedicated GitHub issue marked
`gitclaw:tool-run-request-issue`. This is the first issue-native tool action:
it records the request id, normalized tool, matched contract, gate state,
validation summary, source hashes, and review decision in a separate review
issue, then posts a body-free receipt back to the source issue. It never calls
a model, executes a tool, makes network calls, mutates the repository, or emits
the raw source request, raw requested tool text, raw tool inputs, raw tool
outputs, prompts, credentials, or file bodies. Duplicate open requests are
suppressed by request id. Any implementation change to this action must pair
the deterministic create/reuse E2E with a live GitHub Models follow-up using
`repo-reader` and bounded repository search.

When the same command includes `--notify-route <route>` or
`--notify-routes <route-a,route-b>`, GitClaw also queues a channel notification
after the review issue is created or found. The notification body contains the
review issue number/URL, source issue number/URL, request id, normalized tool,
review decision, and run-allowed gate. It is sent through the reviewed
`.gitclaw/channels/routes.yaml` routebook and the existing
`channel-outbox`/`channel-delivery` provider path. The source receipt reports
only route/message/body hashes, target issue/comment IDs, provider keys,
duplicate counts, and delivery instructions; it does not print raw route
names, raw source request text, raw tool inputs/outputs, raw notification
bodies, thread IDs, message IDs, prompts, credentials, or provider responses.
Duplicate notifications are suppressed per route by `channel + message_id`.
Any implementation change to this notification path must prove the review
issue, channel queue item, metadata-only outbox discovery, duplicate
suppression, and a real GitHub Models repo-reader/search follow-up.

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
@gitclaw /sandbox risk
@gitclaw /exec-policy risk
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
gitclaw sandbox risk
```

`@gitclaw /sandbox risk` and `gitclaw sandbox risk` produce a stricter
body-free risk audit for the same boundary. The report emits runtime, tool,
workflow, and skill risk cards with stable finding codes, explicitly records
that raw issue/comment/prompt/workflow/tool bodies and secrets were not
printed, and treats future shell, repository mutation, elevated execution, or
workflow-permission drift as high-severity findings.

## Security Audit Command

GitClaw supports an OpenClaw-style personal-assistant security audit that
aggregates the deterministic risk surfaces already available in the repo:

```text
@gitclaw /security
@gitclaw /sec
@gitclaw /security audit
@gitclaw /security risk
```

The command runs after normal preflight authorization and repo-context loading,
but before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/security"` and summarizes:

- trust model: single trusted operator, not hostile multi-tenant delegation,
- runtime boundary: ephemeral GitHub Actions runner, no gateway server
  required,
- aggregate status for config, policy, sandbox, channels, tools, skills,
  plugins, and secrets,
- high/warning/info finding counts across those surfaces,
- workflow permission and channel workflow presence gates,
- host-exec, repository-mutation, mutating-tool, and plaintext-secret gates,
- explicit body-free and credential-free audit boundaries.

It never calls a model, mutates the repository, executes shell tools, prints
raw config/workflow/issue/comment/prompt/tool-output bodies, resolves GitHub
Secrets, or prints credential values. This is intentionally closer to a
control-plane posture card than to a vulnerability scanner: it tells the
operator whether GitClaw's GitHub-native assistant boundary still matches the
OpenClaw/Hermes lessons before more channel or proactive surfaces are added.

Local operators can run the same aggregate audit without opening an issue:

```bash
gitclaw security audit
gitclaw security risk
```

Any implementation change to this surface must pair the deterministic report
with a live GitHub Models follow-up conversation that selects `repo-reader`,
exposes prompt-visible `gitclaw.search_files`, recovers the security-audit
fixture token, and avoids hidden issue/comment/body leakage.

## Context Inspection Command

GitClaw supports a deterministic context inspection command inspired by
OpenClaw's `/context` diagnostics:

```text
@gitclaw /context
@gitclaw /context list
@gitclaw /context risk
@gitclaw /context info <path>
```

The command runs after normal preflight authorization and context assembly, but
before model inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/context"` and summarizes:

- selected context files,
- selected full skill documents,
- read-only tool outputs and their input hash, size, and output hash,
- transcript and prompt-budget settings.

It never dumps full file bodies, skill bodies, prompts, raw tool inputs, or
tool output contents into the issue. This makes context visibility cheap enough
for routine E2E debugging and avoids burning GitHub Models quota for a
diagnostic turn.

When called as `@gitclaw /context info <path>`, the command posts a focused
body-free card for one requested context item. The lookup covers loaded context
documents, selected skill documents, deterministic `gitclaw.read_file` outputs
for explicitly mentioned repository files, and active tool outputs addressed by
tool name. It reports only the matched kind, path/tool name, byte/line counts,
short hashes, match source, and safety flags. It never emits raw file bodies,
skill bodies, tool output bodies, raw tool inputs, issue/comment bodies, prompts,
or secrets.

When called as `@gitclaw /context risk`, the command posts a body-free risk
audit for the prompt-visible context boundary. It scans loaded context files,
explicit context references, selected skill bodies, and deterministic tool
outputs for prompt-boundary, credential-exfiltration, hidden-instruction,
host-exec, and unbounded-context patterns, but emits only metadata, counts,
paths, hashes, risk codes, and severities. It also reports prompt budget
pressure, reference status counts, bounded-reference limits, and runtime gates
for external URL fetches, repository mutation, and host exec. Any change to
this surface requires a live GitHub issue E2E that first asserts the
deterministic report and then performs a normal GitHub Models conversation with
repo-reader/tool usage.

Local operators can inspect the same repository context surface without opening
an issue:

```bash
gitclaw context list
gitclaw context risk
gitclaw context info .gitclaw/SOUL.md
gitclaw context info go.mod
```

The local report omits repository and issue metadata, reports zero transcript
messages, and lists body-free context files, selected always-on skills, and
deterministic tool-output metadata with short hashes. The focused local
`context info` variant seeds context assembly with the requested path, so
ordinary repository files can be inspected through the same body-free
`gitclaw.read_file` metadata that would be prompt-visible in an issue turn.
The local `context risk` variant performs the same body-free risk audit without
repository or issue metadata.

## Prompt Inspection And Risk Commands

GitClaw supports deterministic prompt-budget inspection and prompt-risk audit
commands inspired by OpenClaw's context diagnostics and Hermes' bounded
memory/context posture:

```text
@gitclaw /prompt
@gitclaw /prompt list
@gitclaw /prompt pack
@gitclaw /prompt context
@gitclaw /prompt cache
@gitclaw /prompt compression
@gitclaw /prompt risk
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
The report includes `llm_e2e_required_after_prompt_report_change=true`;
changes to this surface must be paired with a live GitHub Models follow-up
that selects `repo-reader`, exposes `gitclaw.search_files`, and recovers a
bounded repository-search fixture token without echoing prompt/body sentinels.

When called as `@gitclaw /prompt pack`, the command posts a body-free packing
projection for the same prompt envelope. It follows the exact deterministic
user-prompt assembly order used before model inference: run header, repository
context, selected skill bodies, deterministic tool-output blocks, transcript
omission marker, and bounded transcript messages. The report emits only
component kind/name, byte and line counts, short hashes, prompt byte ranges,
pack status, pack reason, source-size metadata, and body/input inclusion flags.
It also reports the system-prompt byte/hash metadata separately, because the
system prompt is a distinct model input and not part of the user-prompt
head/tail truncation projection.

The packing report borrows OpenClaw's token/context diagnostics and Hermes'
dual compression thresholds without copying either runtime model wholesale:

- estimate input pressure with the OpenAI-style 4-chars-per-token heuristic,
- report the configured GitClaw byte budget and output-token budget,
- evaluate a 50% agent compression warning threshold,
- evaluate an 85% gateway/session-hygiene warning threshold,
- project the existing fixed head/tail truncation behavior when the prompt
  exceeds `model.max_prompt_bytes`,
- never print raw prompt text, issue/comment bodies, context file bodies, skill
  bodies, tool outputs, raw tool inputs, credentials, or secret values.

Any change to this surface requires a focused live E2E that first verifies
`@gitclaw /prompt pack` without an LLM call and then posts a normal follow-up
that uses GitHub Models, a selected skill, and `gitclaw.search_files`. This
keeps the deterministic budget map from becoming a substitute for testing real
model/tool behavior.

When called as `@gitclaw /prompt context`, the command posts a body-free
prompt-context manifest for the same context inventory that normal model turns
stamp into assistant-turn markers. It reports the `prompt_context_sha256_12`,
context file paths, selected skill paths, prompt-visible skill/tool names,
tool-output names, input/output hashes, byte/line counts, bounded transcript
counts, and no-write gates. It never prints prompt text, issue/comment bodies,
context bodies, skill bodies, tool output bodies, raw tool inputs,
credentials, or secret values. Any change to this surface requires a live E2E
that verifies the deterministic manifest and then performs a normal GitHub
Models follow-up with a selected skill and `gitclaw.search_files`.

When called as `@gitclaw /prompt cache`, the command posts a body-free
cache-readiness report for the same prompt envelope. It does not enable
provider cache controls, does not infer cache hits from headers, and does not
pretend GitHub Models exposes cache telemetry to the current client. Instead it
models the stable same-issue prefix and dynamic suffix that affect exact-prefix
cache reuse:

- system prompt as a separate stable model prefix,
- run header plus repo context and selected skills as same-issue stable user
  prefix,
- deterministic tool outputs and transcript history as dynamic suffix,
- provider cache mode as observe-only,
- request-field gaps for `prompt_cache_key`, retention, and cache-control
  markers,
- usage-counter gaps for cache read/write token accounting,
- heartbeat workflow presence as a possible keep-warm surface, not proof of a
  warm cache.

This adapts OpenClaw's cache-boundary and keep-warm lessons and Hermes'
compression/cache interaction to GitClaw's serverless shape. Reports emit only
component names, sizes, estimated token counts, hashes, cache regions,
boundary roles, and findings. They never print prompt text, issue/comment
bodies, context bodies, skill bodies, tool outputs, credentials, or secret
values. Any change to this surface requires a live E2E that first verifies the
deterministic report and then performs a normal GitHub Models follow-up with a
selected skill and `gitclaw.search_files`.

When called as `@gitclaw /prompt compression`, the command posts a body-free
compression-readiness report for the same prompt envelope. It does not create
lossy summaries, does not split issue sessions, does not write memory, and does
not persist compressed state. Instead it audits the current stateless
GitHub-issue prompt shape against the context-management lessons from Hermes
and OpenClaw:

- Hermes-style 50% in-loop compression and 85% gateway-hygiene thresholds,
- OpenClaw-style session-pruning/cache-boundary discipline without enabling
  provider-specific pruning knobs,
- final head/tail truncation state from GitClaw's existing prompt packer,
- bounded transcript messages, omitted older messages, and per-message body
  truncation counts,
- whether GitClaw would need an actual reviewed compression engine before
  claiming lossy summarization support,
- canonical storage stance: GitHub issue threads plus `gitclaw-backups` replay,
  not an external session database.

The report emits segment kinds, names, compression regions/actions, pack
statuses, byte/line/token estimates, hashes, and findings. It never prints
prompt text, issue/comment bodies, context bodies, skill bodies, tool outputs,
credentials, or secret values. Any change to this surface requires a live E2E
that first verifies the deterministic report and then performs a normal GitHub
Models follow-up with a selected skill and `gitclaw.search_files`.

When called as `@gitclaw /prompt risk`, the command posts a body-free risk
audit for the same prompt envelope. It scans the prompt-visible transcript,
loaded context files, selected skills, and deterministic tool outputs for
prompt-boundary overrides, credential exfiltration instructions, hidden
instructions, host-execution requests, and unbounded-context requests, then
reports only metadata, counts, hashes, risk codes, and severities. The prompt
risk report also records prompt budget pressure, transcript omission/truncation
state, prompt artifact gates, no-write runtime boundaries, and an explicit
`llm_e2e_required_after_prompt_risk_change` flag. It must never print raw
prompt text, raw issue/comment bodies, context bodies, skill bodies, raw tool
inputs, tool-output bodies, credentials, or secret values.

Local operators can inspect the same prompt-budget and prompt-input surface
without opening an issue:

```bash
gitclaw prompt list
gitclaw prompt pack
gitclaw prompt context
gitclaw prompt cache
gitclaw prompt compression
gitclaw prompt risk
```

The local report omits repository and issue metadata, reports zero transcript
messages, and still summarizes provider/model, prompt hash/size, prompt
budgets, context file metadata, selected always-on skills, deterministic
tool-output metadata, prompt packing/truncation projection, cache-readiness
gaps, and prompt-risk posture without dumping prompt text or any loaded bodies.

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
@gitclaw /approvals catalog
@gitclaw /approvals provenance
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
gitclaw approvals catalog
gitclaw approvals list
gitclaw approvals verify
gitclaw approvals provenance
gitclaw approvals risk
```

The local report omits repository, issue, actor, trigger, and write-intent
state. It still reports the approval label names, trusted association source,
per-issue GitHub-label approval store, and read-only write-mode gate.

`@gitclaw /approvals catalog` and local `gitclaw approvals catalog` switch
from per-issue readiness to a compact approval command and layer map. The
catalog exposes the available approval commands, trusted-association layer,
write-request label layer, approval-label layer, managed-label collision audit,
assistant-marker evidence layer, GitHub Actions runtime boundary, and explicit
payload/body-free gate. It is modeled after OpenClaw's split between requested
exec policy, host-local approvals, allowlists, and human decisions, plus
Hermes' defense-in-depth approval boundary. The catalog never approves,
executes, mutates, calls a model, prints issue/comment bodies, dumps approval
payloads, or exposes prompt/tool output bodies. It includes
`llm_e2e_required_after_approvals_catalog_change=true`; every change to this
surface must pass a deterministic live approvals-catalog issue plus a real
GitHub Models follow-up proving prompt context hashing, selected skills,
prompt-visible repository search tools, and usage telemetry.

`@gitclaw /approvals provenance` and local
`gitclaw approvals provenance` switch from gate inventory to body-free evidence
provenance. The report explains the current approval evidence chain:
GitHub event trust, per-issue label state, write-request detection source,
assistant-turn marker history, and the read-only runtime boundary. It reports
counts and hashes for issue labels, active commands, idempotency keys,
transcript shape, and assistant markers. `write_requested_label_present` is the
current label snapshot, while `write_request_evidence_present` combines current
label state with transcript write-intent detection. The report may show
configured managed label names and recognized assistant marker model names, but
unrecognized marker model attributes are reported only by hash. It never prints
raw issue bodies, comments, prompts, approval payloads, run URLs, credentials,
or secret values.
The report includes
`llm_e2e_required_after_approval_provenance_change=true`; every change to this
surface must be tested by first creating a model-backed GitHub issue
conversation, then posting the deterministic provenance command, then posting a
second model-backed follow-up that proves prompt context hashing, selected
skill metadata, prompt-visible repository search tools, and usage markers.

`@gitclaw /approvals risk` and local `gitclaw approvals risk` switch from
readiness inventory to the approval-boundary risk audit. The risk report checks
trusted association breadth, approval-label collisions, managed-label
collisions, the per-issue approval store/scope, write-request detection, and
the hard runtime gate that keeps write actions, repository mutation, host exec,
approval payload dumping, and raw body output disabled. It includes
`llm_e2e_required_after_approval_risk_change=true`; every change to this
approval-risk surface must be tested with a live deterministic approvals-risk
issue and a follow-up GitHub Models conversation that proves inference, prompt
context hashing, selected skill metadata, and prompt-visible repository search
tool usage.

## Policy Inspection Command

GitClaw supports a deterministic policy audit command inspired by OpenClaw's
sandbox/tool-policy/elevated split and Hermes' authorization and approval
posture:

```text
@gitclaw /policy
@gitclaw /policy list
@gitclaw /policy verify
@gitclaw /policy risk
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

When called as `@gitclaw /policy risk`, the command keeps the same body-free
discipline but frames the output as a control-plane risk audit. It verifies
trusted-association breadth, managed-label collisions, the checked-in workflow
permission contract, backup concurrency, active `gitclaw.policy` hashes, and
the hard runtime boundary that keeps write actions, repository mutation, and
host exec disabled. It reports severities, risk codes, counts, and hashes only.
Any change to this risk surface requires a live GitHub issue E2E that first
asserts the deterministic report and then performs a normal GitHub Models
conversation with repo-reader/tool usage.

Local operators can inspect static policy shape without opening an issue:

```bash
gitclaw policy list
gitclaw policy verify
gitclaw policy risk
```

The local report omits event-only fields such as repository, issue number,
preflight result, actor association, trigger state, event labels, and
write-request detection. It still reports trusted associations, managed labels,
expected workflow permissions, model/run mode, and active policy-output
metadata if present.
`gitclaw policy verify` additionally checks the local workflow permission
contract and returns a non-body verification report suitable for CI.
`gitclaw policy risk` returns the local body-free risk audit with the same
trust, label, workflow, policy-output, and read-only runtime boundary cards.

## Secrets Audit Command

GitClaw supports a deterministic repo secrets audit command inspired by
OpenClaw's `openclaw secrets audit --check` operator loop and Hermes' default
secret-isolation posture:

```text
@gitclaw /secrets
@gitclaw /secret
@gitclaw /secrets audit
@gitclaw /secrets risk
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
gitclaw secrets risk
```

The audit/list/scan aliases intentionally return the same body-free report in
v1. GitClaw does not yet configure, migrate, apply, reload, or resolve secrets.
The safe MVP is visibility first: find possible checked-in residue without
giving an LLM or an issue comment the underlying secret material.

When called as `@gitclaw /secrets risk` or `gitclaw secrets risk`, GitClaw
renders the risk-oriented view of the same bounded scan. It reports plaintext
residue counts, known-token counts, sensitive-assignment counts, severity
totals, GitHub Actions secret-reference counts, runtime secret-resolution
boundaries, and configure/apply/reload non-goals. It never resolves GitHub
Secrets, reads environment values, calls a model, mutates files, prints matched
values, prints source lines, or prints referenced secret names. The report
includes `llm_e2e_required_after_secrets_risk_change=true`, so any change to
this surface must be paired with a live GitHub Models conversation E2E after
the deterministic report.

## Checkpoints And Rollback Readiness

GitClaw supports a deterministic checkpoint/rollback-readiness command inspired
by Hermes' checkpoint/rollback feature and OpenClaw's separation between
approval, sandboxing, and mutation:

```text
@gitclaw /checkpoints
@gitclaw /checkpoint
@gitclaw /rollback
@gitclaw /checkpoints catalog
@gitclaw /rollback catalog
@gitclaw /checkpoints preview HEAD~1
@gitclaw /rollback diff HEAD~1
@gitclaw /checkpoints rehearse --id rollback-lab --target HEAD~1
@gitclaw /checkpoints risk
@gitclaw /rollback risk
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
gitclaw checkpoints catalog
gitclaw checkpoints status
gitclaw checkpoints list
gitclaw checkpoints preview HEAD~1
gitclaw checkpoints risk
gitclaw checkpoints verify
gitclaw rollback catalog
gitclaw rollback diff HEAD~1
gitclaw rollback list
gitclaw rollback risk
```

Readiness aliases intentionally return the same body-free report in v1.
Preview aliases return the rollback diff-stat preview; reviewed recovery still
happens through ordinary git history, pull requests, and fetched backup
manifests.
The report includes
`llm_e2e_required_after_checkpoint_report_change=true`; changes to this surface
must be paired with a live GitHub Models follow-up that selects `repo-reader`,
exposes `gitclaw.search_files`, and recovers a bounded repository-search
fixture token without echoing issue-body sentinels.

`@gitclaw /checkpoints catalog`, `@gitclaw /rollback catalog`, and local
`gitclaw checkpoints catalog`/`gitclaw rollback catalog` switch from readiness
state to a compact rollback command and layer map. The catalog exposes
checkpoint/status/list/verify/preview/risk commands,
rollback catalog/diff/list/risk aliases, git history metadata, worktree status
counts, backup-branch evidence, recent commit metadata, implemented
restore-preview gates, inspect-only operation boundaries, and the disabled
reset/clean/checkout gate. It follows Hermes'
checkpoint manager posture: shadow-store rollback is useful only when preview,
scope, and restore boundaries are explicit. It also follows OpenClaw backup
verification: restore-like operations should require manifest evidence before
mutation. The catalog never restores, resets, cleans, checks out, prints diffs,
prints file bodies, prints commit subjects, calls a model, or exposes issue,
comment, prompt, tool-output, credential, or secret bodies. It includes
`llm_e2e_required_after_checkpoint_catalog_change=true`; changes to this surface
must pass a deterministic live checkpoints-catalog issue plus a real GitHub
Models follow-up proving prompt context hashing, selected skills,
prompt-visible repository search tools, and usage telemetry.

`@gitclaw /rollback diff HEAD~1`, `@gitclaw /checkpoints preview HEAD~1`,
local `gitclaw rollback diff HEAD~1`, and local
`gitclaw checkpoints preview HEAD~1` are GitClaw's inspect-only adaptation of
Hermes `/rollback diff <N>`. Because GitClaw has no always-on shadow store in
v1, the target is a git ref visible in the Actions checkout instead of a
Hermes checkpoint number. The handle job checks out fetch depth `2` so
`HEAD~1` is available for the common previous-commit preview. The report
compares the target ref to `HEAD` with git numstat/name-status metadata, then
prints changed-file counts, insertion/deletion totals, binary-file counts,
status codes, extensions, and path hashes only. It does not print file paths,
raw patch hunks, file bodies, commit subjects, issue/comment bodies, prompts,
tool outputs, credentials, or secrets. It also never restores, resets, cleans,
checks out, or mutates files. The report includes
`llm_e2e_required_after_rollback_preview_change=true`; changes to this surface
must pass a deterministic live rollback-preview issue plus a real GitHub
Models follow-up proving prompt context hashing, selected repo-reader skill,
prompt-visible repository search tools, and usage telemetry.

`@gitclaw /checkpoints rehearse --id <stable-id> --target HEAD~1`,
`@gitclaw /rollback rehearsal --id <stable-id> --target HEAD~1`, and
`@gitclaw /rollback drill --id <stable-id> --target HEAD~1` create or reuse one
open GitHub issue carrying a hidden `gitclaw:checkpoint-rehearsal-issue`
marker. This is the issue-native rollback lab: the source issue gets only a
body-free, model-free receipt with the target ref hash, worktree/checkpoint
metadata, duplicate status, and disabled mutation gates. The rehearsal issue
stores the stable rehearsal id, readable target ref, dry-run checkpoint and
rollback commands, and current preview metadata so normal GitClaw conversation
can continue there. It must not call a model in the deterministic source
action, run restore/reset/clean/checkout commands, mutate the repository, print
raw diffs, print file bodies, print raw target refs in the source receipt, or
copy raw source issue/comment bodies. Duplicates are suppressed by rehearsal
id. Changes to this surface require a live E2E that creates the rehearsal
issue, checks duplicate suppression, runs local checkpoint preview commands,
and then continues on the rehearsal issue with a normal GitHub Models
repo-reader/search follow-up.

When called as `@gitclaw /checkpoints risk` or `@gitclaw /rollback risk`, the
command posts a `GitClaw Checkpoint Risk Report`. It scans git checkpoint
metadata for missing git auditability, dirty worktrees, raw diff/file-body
exposure, restore/reset/clean/checkout authority, shadow-store path leakage,
and missing rollback safety gates. It reports status counts, commit hashes,
codes, severities, and risk cards only; it does not print diffs, file bodies,
commit subjects, issue bodies, comments, prompts, tool outputs, credentials, or
secret values. This surface must ship with deterministic tests and a live
GitHub Models follow-up E2E because rollback safety is a future write-mode gate.

## Diffs Inspection Command

GitClaw supports a deterministic diff audit inspired by OpenClaw's read-only
diff artifact plugin and Hermes' checkpoint `/rollback diff` preview:

```text
@gitclaw /diffs
@gitclaw /diff
@gitclaw /changes
@gitclaw /diffs risk
@gitclaw /diff risk
@gitclaw /changes risk
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

When called as `@gitclaw /diffs risk` or `@gitclaw /diffs risk-audit`, the
command posts a `GitClaw Diff Risk Report`. It scans diff policy, diff specs,
and git worktree metadata for prompt-boundary overrides, credential material,
raw patch leakage, destructive git actions, hidden-state use, untracked-file
body context, external storage, missing approval gates, unsafe raw-patch modes,
and unbounded diff collection. The report only exposes metadata, changed paths,
counts, codes, severities, and line hashes; it does not print patches, file
bodies, issue bodies, comments, prompts, tool outputs, credentials, or secret
values. Changes to this surface must include deterministic tests plus a live
GitHub Models follow-up E2E.

Local operators can inspect the same change surface without opening an issue:

```bash
gitclaw diffs summary
gitclaw diffs risk
gitclaw diffs verify
```

## Workspace Inspection Command

GitClaw supports a deterministic workspace audit inspired by OpenClaw's agent
workspace and Hermes' git-worktree isolation model:

```text
@gitclaw /workspace
@gitclaw /workdir
@gitclaw /repo
@gitclaw /workspace catalog
@gitclaw /workdir catalog
@gitclaw /repo catalog
@gitclaw /workspace risk
@gitclaw /workdir risk
@gitclaw /repo risk
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

When called as `@gitclaw /workspace catalog`, `@gitclaw /workdir catalog`, or
`@gitclaw /repo catalog`, the command posts a `GitClaw Workspace Catalog
Report`. The catalog is a compact command/layer/gate map for the GitHub Actions
checkout workspace: catalog, summary, verify, and risk commands; policy/spec
stores; git/workflow/context/repository-inventory layers; runtime and
durable-state boundaries; private-memory, external-mount, daemon, and
long-running socket suppression; and body-free output gates. It exists so
operators can see the workspace surface without printing workspace file bodies,
workflow bodies, issue/comment bodies, prompts, tool outputs, credentials, or
secret values. Changes to this surface must include deterministic tests plus a
live GitHub Models follow-up E2E that makes an actual model call.

When called as `@gitclaw /workspace risk` or `@gitclaw /workspace risk-audit`,
the command posts a `GitClaw Workspace Risk Report`. It scans workspace policy,
workspace specs, and workflow checkout metadata for prompt-boundary overrides,
credential material, private workspace memory, external mounts, destructive
workspace mutation, long-running services, raw body leakage, checkout/setup
version drift, missing approval gates, and unbounded repository inventory. The
report only exposes metadata, paths, counts, codes, severities, and line hashes;
it does not print policy/spec bodies, workflow bodies, file bodies, issue bodies,
comments, prompts, tool outputs, credentials, or secret values. Changes to this
surface must include deterministic tests plus a live GitHub Models follow-up E2E
that makes an actual model call.

Local operators can inspect the same workspace surface without opening an
issue:

```bash
gitclaw workspace catalog
gitclaw workspace summary
gitclaw workspace risk
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
- `trigger.mode`, one of `label-or-prefix`, `label-only`, `prefix-only`, or
  `inbox`,
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
@gitclaw /config risk
```

The command runs after normal preflight and context loading, but before model
inference. It posts a `gitclaw:assistant-turn` comment with
`model="gitclaw/config"` and summarizes:

- effective config source,
- expected `.gitclaw/config.yml` path and file presence,
- trigger mode, label, and prefix,
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
gitclaw config risk
```

The local report omits repository, issue number, and issue-title hash while
retaining effective config source, labels, trusted associations, prompt
budgets, deterministic slash commands, and config/workflow file metadata.

`@gitclaw /config risk` and `gitclaw config risk` provide the stricter
body-free config/control-plane audit. The report follows OpenClaw's
config/schema discipline and Hermes' profile boundary by treating
`.gitclaw/config.yml` and GitHub workflow files as high-authority reviewed
state. It does not call the model or rewrite config.

The config risk report publishes:

- config source, config-file presence, workflow presence, and file hashes,
- trigger mode, label/prefix, disabled label, trusted associations, broad
  association counts, managed-label collisions, and slash-command count,
- model provider, primary model, fallback count, prompt/output/transcript
  budgets, and run mode,
- skill/tool gate counts and allow/deny conflicts,
- finding codes, severities, paths, fields, and line hashes for missing config
  or workflow files, broad actor trust, label collisions, unsafe model budgets,
  missing fallback coverage, credential material, raw prompt logging, external
  webhook/socket/daemon config, write-mode config, risky workflow permissions,
  `pull_request_target`, raw secret echoing, and unbounded background loops.

It never prints config bodies, workflow bodies, issue/comment bodies, prompts,
provider errors, API keys, tokens, credentials, or secret values. Any change to
this surface requires local tests plus a live GitHub issue E2E that includes a
normal GitHub Models follow-up turn with repo-reader/tool usage.

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
The report includes `llm_e2e_required_after_commands_report_change=true`;
changes to this surface must be paired with a live GitHub Models follow-up that
selects `repo-reader`, exposes `gitclaw.search_files`, and recovers a bounded
repository-search fixture token without echoing issue-body sentinels.

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
@gitclaw /orders risk
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

When called as `@gitclaw /orders risk`, the command scans the repo-reviewed
standing-order file for durable-authority risk categories: unbounded authority,
prompt-boundary overrides, credential transfer, external delivery, hidden
persistence, host execution, unbounded retries, skipped verification, missing
structure, and missing proactive enforcement. The report publishes only status,
counts, finding codes, severities, paths, title hashes, and line hashes. It
never prints standing-order bodies, proactive prompt bodies, issue/comment
bodies, prompts, credentials, or secret values, and it never mutates schedules
or orders. Any change to this risk surface requires focused local tests plus a
live GitHub Models follow-up E2E proving normal inference, selected skills, and
prompt-visible tools still work.

Local operators can inspect the same surface with:

```bash
gitclaw orders list
gitclaw orders verify
gitclaw orders risk
```

### Hooks Command

GitClaw supports declarative hooks inspired by OpenClaw's file-based hook
surface:

```text
@gitclaw /hooks
@gitclaw /hooks catalog
@gitclaw /hooks risk
@gitclaw /hooks provenance
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

The catalog form:

```text
@gitclaw /hooks catalog
@gitclaw /hook catalog
```

posts a `GitClaw Hooks Catalog Report` without model inference. It maps the
hook command surface, repo-reviewed hook policy, declarative specs, frontmatter
events, approval gates, ignored executable-looking handler files, git
provenance, and explicit provider-payload negative capability. The catalog is
the GitHub-native adaptation of OpenClaw/Hermes hook discovery: hooks are
useful event-automation metadata, but GitClaw v1 does not run handlers, ingest
external provider payloads, dispatch workflows from hook specs, mutate the
repository, or print raw hook/payload/issue/comment/prompt/tool bodies. It
includes `llm_e2e_required_after_hook_catalog_change=true`; changes to this
surface must pass a deterministic live hooks-catalog issue plus a real GitHub
Models follow-up proving prompt provenance, selected skills, prompt-visible
repository search tools, and usage telemetry.

The risk form:

```text
@gitclaw /hooks risk
@gitclaw /hook risk
```

posts a `GitClaw Hook Risk Report` without model inference. It scans
`.gitclaw/HOOKS.md`, `.gitclaw/hooks/*.md`, and ignored executable-looking
hook handler files for prompt-boundary overrides, credential material,
untrusted issue-body execution, raw payload logging, external webhook bridges,
repository mutation, missing approval/audit-only boundaries, and unbounded
loops. It reports paths, metadata, risk counts, codes, severities, and line
hashes only; hook bodies, handler bodies, issue bodies, comments, provider
payloads, credentials, and secret values are not included. Changes to this risk
surface must include a live GitHub Models follow-up E2E so deterministic
coverage does not accidentally replace actual inference coverage.

The provenance form:

```text
@gitclaw /hooks provenance
@gitclaw /hooks history
@gitclaw /hooks timeline
```

posts a `GitClaw Hook Provenance Report` without model inference. It maps
`.gitclaw/HOOKS.md`, `.gitclaw/hooks/*.md`, and ignored executable-looking
handler files to git history. The report shows hook status, risk status,
approval/audit-only metadata, tracked/dirty state, last commit IDs/dates, and
commit-subject hashes only. It never prints raw hook policy bodies, hook spec
bodies, handler bodies, issue bodies, comments, prompts, git subjects, author
identities, provider payloads, or secret values. This mirrors OpenClaw's
file-based hook discovery while preserving GitClaw's reviewed-repo and
hash-only provenance boundary.

Local operators can inspect the same surface with:

```bash
gitclaw hooks catalog
gitclaw hooks list
gitclaw hooks risk
gitclaw hooks verify
gitclaw hooks provenance
```

### Plugins Command

GitClaw supports declarative plugin audits inspired by OpenClaw's manifest and
runtime extension model, and by Hermes' toolset/MCP filtering:

```text
@gitclaw /plugins
@gitclaw /plugins risk
@gitclaw /plugins mcp
@gitclaw /plugins mcp risk
@gitclaw /plugins mcp provenance
@gitclaw /plugins mcp info github-read
@gitclaw /plugin
```

The command runs after preflight and before model inference. It posts a
`gitclaw:assistant-turn` comment with `model="gitclaw/plugins"` and summarizes:

- whether `.gitclaw/PLUGINS.md` exists and is loaded into model context,
- declarative plugin specs in `.gitclaw/plugins/*.md`,
- declarative MCP server specs in `.gitclaw/mcp/*.yaml`,
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

MCP specs are a narrower plugin-adjacent inventory surface. They live in
`.gitclaw/mcp/*.yaml` and can declare a reviewed name, transport, source,
metadata-only activation, tool allowlist/denylist, secret-name refs, and prompt
or resource gates. In v1, `@gitclaw /plugins mcp` and
`gitclaw plugins mcp` list these specs by path, counts, hashes, filters, and
runtime gates. `@gitclaw /plugins mcp risk` and
`gitclaw plugins mcp risk` scan the YAML for unsafe activation, command/url
launch surfaces, missing tool allowlists, mutating tool refs, env passthrough,
prompt/resource exposure, prompt-boundary overrides, credential material, host
execution, repository mutation, remote exfiltration, and unbounded loops.
`@gitclaw /plugins mcp provenance`, `gitclaw plugins mcp provenance`, and the
`history`/`timeline` aliases map repo-local MCP spec YAML files to git history
without exposing their bodies. They report server names, paths, transport,
activation, tool filters, secret/env ref counts and hashes, launch-surface
hashes, risk codes, tracked/dirty state, last commit IDs/dates, and
commit-subject hashes only.
`@gitclaw /plugins mcp info <name>` and
`gitclaw plugins mcp info <name>` show one spec. These reports are metadata
only: they do not launch MCP servers, connect clients, dynamically discover
tools, expose MCP tools to the model, mutate the repository, or print raw spec
bodies, command values, URL values, args, env values, git commit subjects,
author identities, issue bodies, comments, prompts, provider payloads,
credentials, or secret values. The reports include
`llm_e2e_required_after_mcp_change=true`; every MCP metadata change must ship
with a live GitHub Models follow-up E2E that makes an actual model call.

The risk form:

```text
@gitclaw /plugins risk
@gitclaw /plugin risk
```

posts a `GitClaw Plugin Risk Report` without model inference. It scans
`.gitclaw/PLUGINS.md`, `.gitclaw/plugins/*.md`, and ignored package/runtime
files for prompt-boundary overrides, credential material, automatic package or
ClawHub installs, MCP/runtime connections, untrusted issue-body execution, raw
payload logging, external webhook bridges, repository mutation, missing
approval/metadata-only boundaries, and unbounded loops. It reports paths,
metadata, risk counts, codes, severities, and line hashes only; plugin bodies,
package bodies, issue bodies, comments, provider payloads, credentials, and
secret values are not included. Changes to this risk surface must include a
live GitHub Models follow-up E2E so plugin safety remains tested against actual
inference and prompt-visible tools.

Local operators can inspect the same surface with:

```bash
gitclaw plugins list
gitclaw plugins risk
gitclaw plugins verify
gitclaw plugins mcp
gitclaw plugins mcp risk
gitclaw plugins mcp provenance
gitclaw plugins mcp info <name>
```

### Tasks Command

GitClaw supports a deterministic task-board audit inspired by OpenClaw
background tasks, Task Flow, and Hermes Kanban:

```text
@gitclaw /tasks
@gitclaw /tasks ledger
@gitclaw /tasks risk
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

The risk form:

```text
@gitclaw /tasks risk
@gitclaw /task risk
```

posts a `GitClaw Task Risk Report` without model inference. It scans
`.gitclaw/TASKS.md` and `.gitclaw/tasks/*.md` for prompt-boundary overrides,
credential material, untrusted issue-body execution, detached-worker or
subagent spawn instructions, external task databases, raw task payload logging,
webhook bridges, repository mutation, missing approval/issue-native
boundaries, and unbounded loops. It reports paths, metadata, counts, risk
codes, severities, and line hashes only; task bodies, issue bodies, comments,
transcript messages, flow outputs, worker outputs, credentials, and secret
values are not included. Changes to this risk surface must include a live
GitHub Models follow-up E2E so task safety is tested against actual inference
and prompt-visible tools.

The ledger form:

```text
@gitclaw /tasks ledger
@gitclaw /task ledger
```

posts a `GitClaw Task Ledger Report` without model inference. It treats the
current GitHub issue as the task row and issue comments as the task handoff log,
then reports current label-derived status, comment counts, transcript counts,
assistant-turn marker counts, deterministic versus model-backed turn counts,
prompt-provenance counts, channel/proactive marker presence, and per-entry
hashes. It is deliberately not a full historical label timeline because GitHub's
issue event feed is not in the v1 runtime path; the report says
`status_history_available=false` and `status_transition_source=current-labels-and-markers`.
It never prints raw task policy, task spec, issue, comment, transcript, assistant
reply, prompt, tool-output, or worker-output bodies. Changes to this surface
must include a live GitHub Models follow-up E2E.

Local operators can inspect the same policy/spec surface with:

```bash
gitclaw tasks list
gitclaw tasks risk
gitclaw tasks verify
gitclaw tasks ledger --backup <issue.json>
```

### Agents Command

GitClaw supports a deterministic agent-surface audit inspired by OpenClaw
multi-agent routing, OpenClaw nodes, Hermes `delegate_task`, and Hermes Kanban
workers:

```text
@gitclaw /agents
@gitclaw /agents catalog
@gitclaw /agents provenance
@gitclaw /agents risk
@gitclaw /agent
@gitclaw /agent catalog
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

The catalog form:

```text
@gitclaw /agents catalog
@gitclaw /agent catalog
```

posts a `GitClaw Agents Catalog Report` without model inference. It maps the
agent command surface, `.gitclaw/AGENTS.md`, `.gitclaw/agents/*.md`, the
GitHub Actions runtime, GitHub issue/comment conversation boundary, reviewed
tool-name intent, approval frontmatter, and explicit no-delegation/no-subagent
gates. It does not print agent policy/spec bodies, issue bodies, comments,
prompts, tool outputs, credentials, channel payloads, or session bodies.
Changes to this surface must include deterministic tests plus a live GitHub
Models follow-up E2E that makes an actual model call.

The provenance form:

```text
@gitclaw /agents provenance
@gitclaw /agent provenance
@gitclaw /agents git-history
```

posts a `GitClaw Agent Provenance Report` without model inference. It maps
`.gitclaw/AGENTS.md` and `.gitclaw/agents/*.md` to repo-local git history,
tracked state, dirty state, last commit IDs/dates, risk metadata, validation
counts, and commit-subject hashes. It is the GitClaw v1 version of the
OpenClaw/Hermes agent/profile boundary: agent identity and authority live in
reviewed repository files, while delegation, subagents, gateways, shared
profile state, and agent-to-agent messaging remain disabled. The report does
not print agent bodies, issue bodies, comments, prompts, tool outputs, git
subjects, author identities, channel payloads, or secret values. Changes to
this provenance surface must include deterministic tests, local CLI assertions,
and a live GitHub Models follow-up E2E that proves model inference, prompt
provenance, selected skills, prompt-visible `gitclaw.search_files`, usage
telemetry, and recovery of a distinct repository-search fixture token.

The risk form:

```text
@gitclaw /agents risk
@gitclaw /agent risk
```

posts a `GitClaw Agent Risk Report` without model inference. It scans
`.gitclaw/AGENTS.md` and `.gitclaw/agents/*.md` for prompt-boundary overrides,
credential material, untrusted issue-body execution, subagent/delegation
enablement, external agent processes, shared credential/session/memory state,
raw agent payload logging, webhook bridges, repository mutation, missing
approval/single-assistant boundaries, and unbounded loops. It reports paths,
metadata, counts, risk codes, severities, and line hashes only; agent bodies,
issue bodies, comments, transcript messages, channel payloads, worker outputs,
credentials, and secret values are not included. Changes to this risk surface
must include a live GitHub Models follow-up E2E so agent safety is tested
against actual inference and prompt-visible tools.

Local operators can inspect the same policy/spec surface with:

```bash
gitclaw agents catalog
gitclaw agents list
gitclaw agents provenance
gitclaw agents risk
gitclaw agents verify
```

### Nodes Command

GitClaw supports a deterministic node-surface audit inspired by OpenClaw node
hosts and paired device capabilities, plus Hermes' durable workers and
delegation runtime boundaries:

```text
@gitclaw /nodes
@gitclaw /nodes catalog
@gitclaw /nodes risk
@gitclaw /node
@gitclaw /node catalog
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

The catalog form:

```text
@gitclaw /nodes catalog
@gitclaw /node catalog
```

posts a `GitClaw Nodes Catalog Report` without model inference. It maps the
node command surface, `.gitclaw/NODES.md`, `.gitclaw/nodes/*.md`, the GitHub
Actions ephemeral-job runtime, GitHub-native wake paths, GitHub issue/comment
conversation boundary, reviewed capability-name intent, approval frontmatter,
and explicit no-gateway/no-pairing/no-RPC/no-browser-proxy/no-media-device/
no-remote-exec gates. It does not print node policy/spec bodies, issue bodies,
comments, prompts, tool outputs, credentials, channel payloads, worker payloads,
or session bodies. Changes to this surface must include deterministic tests
plus a live GitHub Models follow-up E2E that makes an actual model call.

The risk form:

```text
@gitclaw /nodes risk
@gitclaw /node risk
```

posts a `GitClaw Node Risk Report` without model inference. It scans
`.gitclaw/NODES.md` and `.gitclaw/nodes/*.md` for prompt-boundary overrides,
credential material, untrusted issue-body execution, Gateway WebSocket node
hosts, remote node execution, device pairing or auto-approval, browser-proxy
surfaces, camera/screen/location/SMS/notification capabilities, external
worker lanes, raw node payload logging, repository mutation, missing
approval/ephemeral-job boundaries, and unbounded loops. It reports paths,
metadata, counts, risk codes, severities, and line hashes only; node bodies,
issue bodies, comments, transcript messages, channel payloads, worker outputs,
credentials, and secret values are not included. Changes to this risk surface
must include a live GitHub Models follow-up E2E so node safety is tested
against actual inference and prompt-visible tools.

Local operators can inspect the same policy/spec surface with:

```bash
gitclaw nodes catalog
gitclaw nodes list
gitclaw nodes risk
gitclaw nodes verify
```

### Artifacts Command

GitClaw supports a deterministic artifact-governance audit inspired by
OpenClaw backup/migration exports, Hermes sessions and checkpoints, and
GitHub Actions artifacts:

```text
@gitclaw /artifacts
@gitclaw /artifacts catalog
@gitclaw /artifact
@gitclaw /artifact catalog
@gitclaw /artifacts risk
@gitclaw /artifact risk
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

The catalog form:

```text
@gitclaw /artifacts catalog
@gitclaw /artifact catalog
```

posts a `GitClaw Artifacts Catalog Report` without model inference. It maps the
artifact command surface, `.gitclaw/ARTIFACTS.md`,
`.gitclaw/artifacts/*.md`, reviewed `actions/upload-artifact` workflow steps,
GitHub Actions artifact storage, redaction metadata, explicit short retention,
the durable git backup branch boundary, and explicit no-hidden-state,
no-external-storage, no-long-term-artifact-memory, and no-raw-payload gates. It
does not print artifact payloads, prompt bodies, issue bodies, comments, tool
outputs, credentials, channel payloads, backup payloads, or session bodies.
Changes to this surface must include deterministic tests plus a live GitHub
Models follow-up E2E that makes an actual model call.

When called as `@gitclaw /artifacts risk` or
`@gitclaw /artifacts risk-audit`, the command posts a `GitClaw Artifact Risk
Report`. It scans artifact policy, artifact specs, and workflow upload metadata
for prompt-boundary overrides, credential material, unredacted prompt artifacts,
raw artifact payload logging, hidden-state use, external storage, unreviewed
repository mutation, unbounded artifact collection, long retention, missing
label gates, missing `if-no-files-found: error`, and missing approval/redaction
metadata. The report only exposes paths, counts, codes, severities, line hashes,
and metadata; it does not print artifact bodies, issue bodies, comments,
uploaded files, prompt bodies, tool outputs, credentials, or secret values.
Changes to this surface must include deterministic tests plus a live GitHub
Models follow-up E2E.

Local operators can inspect the same policy/spec/upload surface with:

```bash
gitclaw artifacts catalog
gitclaw artifacts list
gitclaw artifacts risk
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
- E2E harness counts for checked-in scripts, live issue scripts, cleanup
  coverage, model-backed coverage, real model follow-up coverage, session
  coverage, backup gates, and workflow-dispatch coverage,
- proactive prompt count,
- managed label count,
- validation error/warning totals,
- skill, soul, memory, and tool validation statuses plus error/warning counts,
- pass/warn checks for the core control plane and validation rollups.

It never dumps file bodies, issue bodies, comments, prompts, or secrets. This
is the GitHub-native equivalent of `openclaw config validate` plus the cold,
read-only parts of `openclaw doctor`: useful health diagnostics inside the
canonical issue log without introducing an auto-repair mode.

The E2E harness check is intentionally conservative. It warns if the repository
has no harness scripts, no live issue harnesses, any harness without cleanup, or
no evidence of model-backed, model-follow-up, session-coverage, or backup-gate
tests. `model_coverage` means the script asserts model/provenance evidence;
`model_followup` is stricter and requires a real follow-up comment path that
waits for an `issue_comment` run and a second assistant turn with prompt
provenance and prompt-visible tool evidence. The report lists only harness
paths, byte/line counts, short hashes, and boolean coverage flags; it never
prints script bodies or test prompt text.

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

`@gitclaw /channels` is the body-free operator view for the whole bridge. It
reports workflow presence, dispatch inputs, permissions, supported providers,
channel-thread markers, and mirrored channel-message counts only, and carries
`llm_e2e_required_after_channel_report_change=true`. Changes to this surface
must pair the deterministic report with a normal GitHub Models follow-up that
uses `repo-reader` plus bounded repository search, so Slack/Telegram bridge
metadata changes keep proving real model/tool context without exposing channel
message bodies or provider credentials.

`@gitclaw /channels list` is the explicit alias for the same body-free bridge
inventory. It also carries
`llm_e2e_required_after_channel_list_change=true`, and changes to that alias
must pair the deterministic workflow-dispatch inventory with a normal GitHub
Models follow-up that selects `repo-reader`, exposes `gitclaw.search_files`,
and recovers the bounded channels-list repository-search fixture token without
echoing hidden issue/comment sentinels.

`@gitclaw /channels info <provider>` is the body-free operator view for one
provider contract. It reports secret names, offset/thread/message keys,
workflow-dispatch bridge metadata, gateway runtime, state storage, and command
shapes only, and carries
`llm_e2e_required_after_channel_info_change=true`. Changes to this surface must
pair the deterministic report with a normal GitHub Models follow-up that uses
`repo-reader` plus bounded repository search, so the Slack/Telegram bridge path
keeps proving real model/tool context without exposing credentials or mirrored
message bodies.

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

### Channel Send Command

Channel bridges also need a GitHub-originated outbound path for proactive jobs,
operator-triggered notifications, and future scheduled channel nudges. GitClaw
exposes this as `channel-send`, which queues a deliverable message on the
canonical channel thread without calling the model:

```bash
gitclaw channel-send \
  --repo OWNER/REPO \
  --channel slack \
  --thread-id <provider-thread-or-chat-id> \
  --message-id <stable-outbound-id> \
  --body "message to send"
```

For scheduled jobs and proactive nudges, the same command can resolve a
repo-reviewed route instead of carrying raw provider thread IDs in each
dispatch:

```bash
gitclaw channel-send \
  --repo OWNER/REPO \
  --route team-alerts \
  --message-id <stable-outbound-id> \
  --body "message to send"
```

Routes live in `.gitclaw/channels/routes.yaml`. Each route has a `name`,
`channel`, and either `thread_id` or `thread_id_template`; templates may use
`{message_id}` and `{route}` so E2E and proactive jobs can create stable
per-message GitHub channel threads without a server-side router.

The same reviewed routebook is available from ordinary GitHub conversation:

```text
@gitclaw /channels send --route team-alerts --message-id <stable-outbound-id>
message to send
```

`/channels send`, `/channels say`, and `/channels notify` run after normal
preflight, require a trusted actor, do not call the model, and post a
body-free receipt on the source issue. If `--message-id` is omitted, GitClaw
derives a deterministic id from the triggering issue/comment and body hash.
Receipts report target issue/comment IDs, route/thread/message/body hashes,
duplicate status, and delivery instructions, but never print raw outbound
bodies, raw thread IDs, raw message IDs, provider credentials, or provider API
responses.

For a deterministic route test that does not copy arbitrary issue text into the
provider queue, GitClaw also supports:

```text
@gitclaw /channels probe --route team-alerts --message-id <stable-probe-id>
```

`/channels probe`, `/channels test`, `/channels ping`, and `/channels check`
resolve one reviewed route and queue a generated provider-facing probe body
through the same `channel-send` machinery. The source receipt reports only
target issue/comment IDs, route/thread/message/probe/source hashes, duplicate
status, and delivery instructions. It does not call a model, call provider
APIs, print raw route names, print raw thread/message IDs, copy raw source
text, print raw probe bodies, or perform provider delivery. Duplicates are
suppressed by `channel + message_id`. Changes to this surface require a live
E2E that verifies outbox visibility, delivery receipt recording, duplicate
suppression, body-free receipts, and a real GitHub Models repo-reader/search
follow-up.

For multi-route announcements, GitClaw also supports:

```text
@gitclaw /channels broadcast team-alerts,ops-alerts --message-id <stable-outbound-id>
message to send
```

`/channels broadcast`, `/channels fanout`, and `/channels announce` require
reviewed route names from `.gitclaw/channels/routes.yaml`. They queue one
`gitclaw:channel-outbound` comment per route, report route/thread/message/body
hashes, duplicate counts, and target issue/comment IDs, and never call a model,
call provider APIs, print raw route names, print raw thread/message IDs, print
raw outbound bodies, or perform provider delivery. Duplicates are suppressed
per route by the same `channel + message_id` queue semantics as single-route
sends.

For inviting an external channel into an existing GitHub issue, GitClaw also
supports:

```text
@gitclaw /channels invite team-alerts,ops-alerts --message-id <stable-outbound-id>
optional note to include in the routed channel invitation
```

`/channels invite`, `/channels share`, and `/channels summon` compose a
provider-facing invitation body from the source issue number, source issue URL,
source issue title, and optional note, then queue that body through the same
reviewed-route broadcast machinery. The source receipt reports only hashes,
counts, target issue/comment IDs, provider keys, duplicate status, and delivery
instructions. It does not call a model, call provider APIs, print raw route
names, print raw source titles, print raw notes, print raw thread/message IDs,
or print the outbound invitation body. Duplicates are suppressed per route by
the same `channel + message_id` queue semantics as other outbound channel
messages.

For opening a dedicated GitHub discussion room and inviting channels into it,
GitClaw also supports:

```text
@gitclaw /channels room team-alerts,ops-alerts --room-id <stable-room-id> --message-id <stable-outbound-id>
Topic: short room topic
Notes:
optional notes to pin in the room and routed channel invitation
```

`/channels room` and `/channels space` create or reuse one open issue carrying
a hidden `gitclaw:channel-room` marker for the stable room id, label that issue
with `gitclaw` so the conversation can continue normally, and queue one
provider-facing room invitation per reviewed route. The room issue is the
human-readable shared space: it contains the topic, notes, source issue number,
source issue URL, route count, and route hash. The source receipt remains
body-free, reporting only the room issue number/URL, room id/topic/notes hashes,
route counts, duplicate status, target issue/comment IDs, and delivery
instructions. It does not call a model, call provider APIs, print raw route
names, print raw room ids, print raw topic/notes text, print raw thread/message
IDs, or print the outbound invitation body. Duplicates are suppressed first by
`room_id` for the GitHub room and then by `channel + message_id` for each routed
outbound invitation. Changes to this surface require a live E2E that creates
the room, validates queued route invites and body-free receipts, and then
continues on the room issue with a normal GitHub Models repo-reader/search
follow-up.

For smaller ad-hoc meetings that should have an agenda rather than a persistent
room notebook, GitClaw also supports:

```text
@gitclaw /channels huddle team-alerts,ops-alerts --huddle-id <stable-huddle-id> --message-id <stable-outbound-id>
Topic: short huddle topic
Agenda:
optional agenda to include in the huddle and routed channel invitation
```

`/channels huddle` and `/channels jam` create or reuse one open issue carrying
a hidden `gitclaw:channel-huddle` marker for the stable huddle id, label that
issue with `gitclaw` so the conversation can continue normally, and queue one
provider-facing huddle invitation per reviewed route. The huddle issue contains
the topic, agenda, source issue number, source issue URL, route count, and
route hash. The source receipt remains body-free, reporting only the huddle
issue number/URL, huddle id/topic/agenda hashes, route counts, duplicate status,
target issue/comment IDs, and delivery instructions. It does not call a model,
call provider APIs, print raw route names, print raw huddle ids, print raw
topic/agenda text, print raw thread/message IDs, or print the outbound
invitation body. Duplicates are suppressed first by `huddle_id` for the GitHub
huddle issue and then by
`channel + message_id` for each routed outbound invitation. Changes to this
surface require a live E2E that creates the huddle, validates queued route
invites and body-free receipts, and then continues on the huddle issue with a
normal GitHub Models repo-reader/search follow-up.

For lightweight decisions that should happen where participants already are,
GitClaw also supports:

```text
@gitclaw /channels poll team-alerts,ops-alerts --poll-id <stable-poll-id> --message-id <stable-outbound-id>
Question: Which option should we choose?
Options:
- First option
- Second option
```

`/channels poll`, `/channels vote`, `/channels ballot`, and `/channels ask`
create or reuse one open issue carrying a hidden `gitclaw:channel-poll` marker
for the stable poll id, label that issue with `gitclaw` so the conversation can
continue normally, and queue one provider-facing poll invitation per reviewed
route. The poll issue is the human-readable decision room: it contains the
question, numbered options, source issue number, source issue URL, route count,
and route hash. The source receipt remains body-free, reporting only the poll
issue number/URL, poll id/question/options hashes, option count, route counts,
duplicate status, target issue/comment IDs, and delivery instructions. It does
not call a model, call provider APIs, print raw route names, print raw poll
ids, print raw questions/options, print raw thread/message IDs, or print the
outbound invitation body. Duplicates are suppressed first by `poll_id` for the
GitHub poll issue and then by `channel + message_id` for each routed outbound
invitation. Changes to this surface require a live E2E that creates the poll,
validates queued route invites and body-free receipts, and then continues on
the poll issue with a normal GitHub Models repo-reader/search follow-up.

For lightweight status checks that should collect responses where participants
already are, GitClaw also supports:

```text
@gitclaw /channels rollcall team-alerts,ops-alerts --rollcall-id <stable-rollcall-id> --message-id <stable-outbound-id>
Prompt: What did you ship today?
Instructions:
- done
- doing
- blocked
```

`/channels rollcall`, `/channels check-in`, `/channels checkin`,
`/channels standup`, and `/channels attendance` create or reuse one open issue
carrying a hidden `gitclaw:channel-rollcall` marker for the stable rollcall id,
label that issue with `gitclaw` so the conversation can continue normally, and
queue one provider-facing check-in invitation per reviewed route. The rollcall
issue is the human-readable room: it contains the prompt, optional
instructions, source issue number, source issue URL, route count, and route
hash. The source receipt remains body-free, reporting only the rollcall issue
number/URL, rollcall id/prompt/instruction hashes, route counts, duplicate
status, target issue/comment IDs, and delivery instructions. It does not call a
model, call provider APIs, print raw route names, print raw rollcall ids, print
raw prompt/instruction text, print raw thread/message IDs, or print the outbound
invitation body. Duplicates are suppressed first by `rollcall_id` for the
GitHub check-in issue and then by `channel + message_id` for each routed
outbound invitation. Changes to this surface require a live E2E that creates
the rollcall, validates queued route invites and body-free receipts, and then
continues on the rollcall issue with a normal GitHub Models repo-reader/search
follow-up.

When the source issue is itself a `gitclaw:channel-thread`, GitClaw also
accepts a structured reaction form:

```text
@gitclaw /channels status --message-id <provider-message-id> --status-id <stable-status-id> --state working
@gitclaw /channels edit --message-id <provider-message-id> --edit-id <stable-edit-id>
@gitclaw /channels react --message-id <provider-message-id> --reaction eyes
@gitclaw /channels pin --message-id <provider-message-id>
@gitclaw /channels deliverable --deliverable-id <stable-deliverable-id> --message-id <provider-message-id> --filename <name> --media-type <mime> --url <download-url>
@gitclaw /channels task --task-id <stable-task-id> --message-id <provider-message-id>
@gitclaw /channels clip --clip-id <stable-clip-id> --message-id <provider-message-id>
@gitclaw /channels attachment --attachment-id <stable-attachment-id> --message-id <provider-message-id> --filename <name> --media-type <mime> --bytes <n>
@gitclaw /channels decision --decision-id <stable-decision-id> --message-id <provider-message-id>
@gitclaw /channels digest --digest-id <stable-digest-id> --message-id <provider-message-id>
@gitclaw /channels handoff --id <stable-handoff-id> --message-id <provider-message-id>
@gitclaw /channels request-run <tool-name> --id <stable-request-id> --message-id <provider-message-id>
@gitclaw /channels approval-plan <tool-name> --id <stable-approval-plan-id> --message-id <provider-message-id>
@gitclaw /channels rehearse-tool <tool-name> --id <stable-rehearsal-id> --message-id <provider-message-id>
@gitclaw /channels propose-skill <skill-name> --message-id <provider-message-id>
@gitclaw /channels rehearse-skill <skill-name> --id <stable-rehearsal-id> --message-id <provider-message-id>
@gitclaw /channels propose-soul --target <soul-path> --id <stable-proposal-id> --message-id <provider-message-id>
@gitclaw /channels rehearse-soul --target <soul-path> --id <stable-rehearsal-id> --message-id <provider-message-id>
@gitclaw /channels rehearse-memory --target <memory-target> --id <stable-rehearsal-id> --message-id <provider-message-id>
@gitclaw /channels rehearse-backup --id <stable-rehearsal-id> --message-id <provider-message-id>
@gitclaw /channels remind --reminder-id <stable-reminder-id> --message-id <provider-message-id> --at <RFC3339-or-date>
@gitclaw /channels done --message-id <stable-outbound-id>
```

`/channels status`, `/channels progress`, and `/channels typing` infer the
current channel and thread id from the issue marker when no explicit
route/channel/thread target is provided, then post one
`gitclaw:channel-status` comment on the same canonical channel issue. This is
the GitHub-native progress/streaming substitute: provider gateways can deliver
small "queued", "typing", "working", "blocked", or "done" updates through
`channel-outbox` without a resident socket. Duplicate updates are suppressed
by `channel + status_id`. The source receipt reports only target issue/comment
ids, target message hash, status id hash, status state hash, status body hash,
route/thread hashes, duplicate status, and delivery gates. It does not call a
model, call provider APIs, print raw route names, print raw thread ids, print
raw target message ids, print raw status ids, print raw status states, print
status bodies, or print channel message bodies. The pending update appears in
`channel-outbox` as kind `channel-status`, and `channel-delivery` records the
provider receipt for the status comment.

`/channels edit`, `/channels update`, and `/channels replace` infer the current
channel and thread id from the issue marker when no explicit route/channel/thread
target is provided, then post one `gitclaw:channel-edit` comment on the same
canonical channel issue. This gives provider gateways a serverless edit/update
operation for messages that already exist in Slack, Telegram, or a future
provider surface. Duplicate edits are suppressed by `channel + edit_id`. The
source receipt reports only target issue/comment ids, target message hash, edit
id hash, edit body hash, route/thread hashes, duplicate status, and delivery
gates. It does not call a model, call provider APIs, print raw route names,
print raw thread ids, print raw target message ids, print raw edit ids, print
replacement bodies, or print channel message bodies. The pending update appears
in `channel-outbox` as kind `channel-edit`, and `channel-delivery` records the
provider receipt for the edit comment.

`/channels react`, `/channels reaction`, `/channels emoji`, `/channels ack`,
`/channels pin`, `/channels star`, and `/channels bookmark` infer the current
channel and thread id from the issue marker when no explicit route/channel/thread
target is provided, then post one
`gitclaw:channel-reaction` comment on the same canonical channel issue. The
provider gateway delivers that as a native Slack/Telegram acknowledgement
instead of a full text reply. Duplicate reactions are suppressed by
`channel + target_message_id + reaction`. The source receipt reports only the
target issue, comment id, target message hash, reaction hash, route/thread
hashes, duplicate status, and delivery gates. It does not call a model, call
provider APIs, print raw route names, print raw thread ids, print raw target
message ids, print raw reactions, or print channel message bodies. The pending
reaction appears in `channel-outbox` as kind `channel-reaction`, and
`channel-delivery` records the provider receipt for the reaction comment.
The short forms provide default reaction names: `pin` uses `pushpin`, `star`
uses `star`, and `bookmark` uses `bookmark`, so operators can mark mirrored
messages without typing provider-specific emoji names.

The same channel-thread issue also accepts a shorter reply form:

```text
@gitclaw /channels reply --message-id <stable-outbound-id>
message to send back to the mirrored channel
```

`/channels reply` infers the current channel and thread id from the issue
marker when no explicit route/channel/thread target is provided. It reuses the
same `channel-send` queue, duplicate suppression, outbox, and delivery receipt
machinery, but the target issue is the source issue. This makes the mirrored
Slack/Telegram issue an operator console rather than a passive report surface.
The receipt reports `target_from_current_channel_issue=true` and
`target_issue_is_source=true` without printing the raw outbound body, thread id,
message id, or provider payloads.

The same channel-thread issue can queue provider-native file/link deliverables:

```text
@gitclaw /channels deliverable --deliverable-id <stable-deliverable-id> --message-id <provider-message-id> --filename <name> --media-type <mime> --url <download-url>
Caption:
optional provider-visible caption
```

`/channels deliverable`, `/channels deliver`, `/channels send-file`,
`/channels share-file`, `/channels deliver-file`, `/channels media-deliver`,
and `/channels artifact-deliver` infer the current channel and thread id from
the issue marker when no explicit route/channel/thread target is provided.
They post one `gitclaw:channel-deliverable` comment on the same canonical
channel issue. This is the GitHub-native equivalent of Hermes deliverable mode:
provider gateways can call `gitclaw channel-outbox --include-body --out <file>`
to receive the visible filename, URL, media type, checksum, and caption, upload
the file natively to Slack/Telegram/etc., and then record the provider receipt
with `gitclaw channel-delivery`. GitClaw itself does not upload files, fetch
the URL, call provider APIs, or call a model in the deterministic action. The
source receipt remains body-free, reporting only deliverable/thread/message/
filename/caption/URL/checksum hashes, byte-count metadata, duplicate status,
and delivery/upload gates. Duplicates are suppressed by `channel +
deliverable_id`. Changes to this surface require a live E2E that queues a
deliverable, validates metadata-only outbox does not leak the URL/body,
validates include-body outbox exposes the provider-visible payload, records a
channel-delivery receipt, checks duplicate suppression, and then continues on
the channel issue with a normal GitHub Models repo-reader/search follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
normal GitHub task issue:

```text
@gitclaw /channels task --task-id <stable-task-id> --message-id <provider-message-id>
Title: short task title
Notes:
optional human-readable task notes
```

`/channels task`, `/channels todo`, `/channels ticket`, and `/channels issue`
infer the current channel and thread id from the issue marker when no explicit
route/channel/thread target is provided. They create or reuse one open GitHub
issue carrying a hidden `gitclaw:channel-task` marker for the stable task id,
label it with `gitclaw` so normal conversation can continue there, and queue a
provider-facing task link back to the mirrored channel thread. The task issue
contains the human-readable title and notes because it is the work item; the
source receipt remains body-free, reporting only task/thread/message/title/note
hashes, duplicate status, notification queue metadata, and delivery gates. It
does not call a model, call provider APIs, print raw task ids, print raw thread
ids, print raw source or notification message ids, print channel message
bodies, or print raw task title/notes in the source receipt. Duplicates are
suppressed first by `task_id` for the GitHub task issue and then by
`channel + notify_message_id` for the provider-facing task link. Changes to
this surface require a live E2E that creates the task, validates the
metadata-only task-link outbox, checks duplicate suppression, and then
continues on the task issue with a normal GitHub Models repo-reader/search
follow-up.

The same channel-thread issue can also save a mirrored provider message without
turning it into work:

```text
@gitclaw /channels clip --clip-id <stable-clip-id> --message-id <provider-message-id>
Title: short saved-context title
Notes:
optional human-readable clip notes
```

`/channels clip`, `/channels save`, `/channels capture`, `/channels remember`,
and `/channels archive` infer the current channel and thread id from the issue
marker when no explicit route/channel/thread target is provided. They create or
reuse one open GitHub issue carrying a hidden `gitclaw:channel-clip` marker for
the stable clip id, label it with `gitclaw` so normal conversation can continue
there, and queue a provider-facing clip link back to the mirrored channel
thread. The clip issue contains the human-readable title and notes because it
is the durable saved context; the source receipt remains body-free, reporting
only clip/thread/message/title/note hashes, duplicate status, notification
queue metadata, and delivery gates. It does not call a model, call provider
APIs, print raw clip ids, print raw thread ids, print raw source or
notification message ids, print channel message bodies, or print raw clip
title/notes in the source receipt. Duplicates are suppressed first by `clip_id`
for the GitHub clip issue and then by `channel + notify_message_id` for the
provider-facing clip link. Changes to this surface require a live E2E that
creates the clip, validates the metadata-only clip-link outbox, checks duplicate
suppression, and then continues on the clip issue with a normal GitHub Models
repo-reader/search follow-up.

The same channel-thread issue can also record channel-origin file/media
metadata without fetching provider bytes:

```text
@gitclaw /channels attachment --attachment-id <stable-attachment-id> --message-id <provider-message-id> --filename <name> --media-type <mime> --bytes <n>
Caption:
optional human-readable attachment caption
```

`/channels attachment`, `/channels attach`, `/channels file`,
`/channels media`, `/channels upload`, and `/channels document` infer the
current channel and thread id from the issue marker when no explicit
route/channel/thread target is provided. They create or reuse one open GitHub
issue carrying a hidden `gitclaw:channel-attachment` marker for the stable
attachment id, label it with `gitclaw` so normal conversation can continue
there, and queue a provider-facing attachment link back to the mirrored channel
thread. The attachment issue contains readable metadata that humans need for
inspection: filename, media type, byte count, checksum if supplied, source URL
hash, and optional caption. The action does not fetch source URLs, copy file
bytes, upload attachments, or call a model. The source receipt remains
body-free, reporting only attachment/thread/message/filename/caption/source
URL/checksum hashes, duplicate status, notification queue metadata, byte-count
metadata, and delivery/fetch gates. It does not print raw attachment ids, raw
thread ids, raw source or notification message ids, raw filenames, raw
captions, raw source URLs, raw checksums, channel message bodies, or file
bytes. Duplicates are suppressed first by `attachment_id` for the GitHub
attachment metadata issue and then by `channel + notify_message_id` for the
provider-facing attachment link. Changes to this surface require a live E2E
that records the attachment metadata, validates the metadata-only
attachment-link outbox, checks duplicate suppression, verifies no file/source
URL material leaks into receipts, and then continues on the attachment issue
with a normal GitHub Models repo-reader/search follow-up.

The same channel-thread issue can also record a durable decision:

```text
@gitclaw /channels decision --decision-id <stable-decision-id> --message-id <provider-message-id>
Decision: short decision statement
Rationale:
optional human-readable rationale
```

`/channels decision`, `/channels decisions`, `/channels decide`,
`/channels record-decision`, and `/channels capture-decision` infer the current
channel and thread id from the issue marker when no explicit route/channel/
thread target is provided. They create or reuse one open GitHub issue carrying
a hidden `gitclaw:channel-decision` marker for the stable decision id, label it
with `gitclaw` so normal conversation can continue there, and queue a
provider-facing decision link back to the mirrored channel thread. The decision
issue contains the human-readable decision and rationale because it is the
durable decision log; the source receipt remains body-free, reporting only
decision/thread/message/text/rationale hashes, duplicate status, notification
queue metadata, and delivery gates. It does not call a model, call provider
APIs, print raw decision ids, print raw thread ids, print raw source or
notification message ids, print channel message bodies, or print raw decision
text/rationale in the source receipt. Duplicates are suppressed first by
`decision_id` for the GitHub decision issue and then by
`channel + notify_message_id` for the provider-facing decision link. Changes to
this surface require a live E2E that records the decision, validates the
metadata-only decision-link outbox, checks duplicate suppression, and then
continues on the decision issue with a normal GitHub Models repo-reader/search
follow-up.

The same channel-thread issue can also record a durable digest:

```text
@gitclaw /channels digest --digest-id <stable-digest-id> --message-id <provider-message-id>
Summary: short digest summary
Highlights:
optional human-readable highlights
```

`/channels digest`, `/channels brief`, `/channels recap`,
`/channels summary`, and `/channels summarize` infer the current channel and
thread id from the issue marker when no explicit route/channel/thread target is
provided. They create or reuse one open GitHub issue carrying a hidden
`gitclaw:channel-digest` marker for the stable digest id, label it with
`gitclaw` so normal conversation can continue there, and queue a
provider-facing digest link back to the mirrored channel thread. The digest
issue contains the human-readable summary and highlights because it is the
durable recap log; the source receipt remains body-free, reporting only
digest/thread/message/summary/highlight hashes, duplicate status, notification
queue metadata, and delivery gates. It does not call a model, call provider
APIs, print raw digest ids, print raw thread ids, print raw source or
notification message ids, print channel message bodies, or print raw summary/
highlights in the source receipt. Duplicates are suppressed first by
`digest_id` for the GitHub digest issue and then by `channel +
notify_message_id` for the provider-facing digest link. Changes to this
surface require a live E2E that records the digest, validates the metadata-only
digest-link outbox, checks duplicate suppression, and then continues on the
digest issue with a normal GitHub Models repo-reader/search follow-up.

The same channel-thread issue can also fork the mirrored conversation into a
normal GitHub session lane:

```text
@gitclaw /channels handoff --id <stable-handoff-id> --message-id <provider-message-id>
```

`/channels handoff`, `/channels session-handoff`, `/channels handoff-session`,
`/channels fork-session`, `/channels session-fork`, `/channels new-session`,
`/channels new-issue`, and `/channels continue-github` infer the current
channel and thread id from the issue marker when no explicit channel/thread
target is provided. They create or reuse one open GitHub issue carrying the
existing `gitclaw:session-handoff-issue` marker, label it with `gitclaw` so
normal conversation can continue there, and queue a provider-facing handoff
link back to the mirrored channel thread. The channel action does not call a
model, call provider APIs, keep a server/socket open, copy raw channel bodies,
copy raw issue/comment bodies, copy raw assistant replies, copy prompts, copy
tool outputs, or mutate the repository. The linked handoff issue stores counts,
hashes, marker provenance, reentry gates, model/skill/tool names, and usage
totals from the source issue thread; a normal `@gitclaw` follow-up on that
linked issue is the actual resumed LLM conversation. Duplicates are suppressed
first by handoff id for the GitHub handoff issue and then by `channel +
notify_message_id` for the provider-facing handoff link. The source receipt
reports only source issue/comment ids, thread/message/notification/handoff
hashes, duplicate state, notification queue metadata, transcript counts,
model/skill/tool/usage telemetry, and delivery gates, including
`llm_e2e_required_after_channel_session_handoff_change=true`. Changes to this
surface require a live E2E that first gets a model-backed assistant turn on the
channel issue, creates the handoff, validates the metadata-only handoff-link
outbox, checks duplicate suppression, and then continues on the handoff issue
with a normal GitHub Models repo-reader/search follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
reviewed tool-run request:

```text
@gitclaw /channels request-run <tool-name> --id <stable-request-id> --message-id <provider-message-id>
```

`/channels request-run`, `/channels run-request`, `/channels tool-run`,
`/channels request-tool`, and `/channels tool-request` infer the current
channel and thread id from the issue marker when no explicit channel/thread
target is provided. They create or reuse one open GitHub issue carrying the
existing `gitclaw:tool-run-request-issue` marker, then queue a provider-facing
review link back to the mirrored channel thread. This is not execution: the
action does not call a model, execute the requested tool, run shell commands,
call provider APIs, or mutate the repository. The tool-run request issue stores
the normalized tool, request id, review decision, and body-free source hashes
needed for human review; the channel source receipt remains body-free,
reporting only request/thread/message/tool hashes, validation and review
metadata, duplicate status, notification queue metadata, and delivery gates. It
does not print raw request ids, raw provider thread/message ids, raw requested
tool arguments, raw channel message bodies, raw tool inputs, or raw tool
outputs in the source receipt. Duplicates are suppressed first by request id
for the GitHub review issue and then by `channel + notify_message_id` for the
provider-facing review link. Changes to this surface require a live E2E that
records the request, validates the metadata-only review-link outbox, checks
duplicate suppression, and then continues on the review issue with a normal
GitHub Models repo-reader/search follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
dry-run tool approval gate:

```text
@gitclaw /channels approval-plan <tool-name> --id <stable-approval-plan-id> --message-id <provider-message-id>
```

`/channels approval-plan`, `/channels tool-approval`,
`/channels approve-plan`, `/channels approval-gate`, `/channels tool-gate`,
and `/channels approve-tool` infer the current channel and thread id from the
issue marker when no explicit channel/thread target is provided. They create
or reuse one open GitHub issue carrying the `gitclaw:tool-approval-plan-issue`
marker, label that issue for normal GitClaw conversation, and queue a
provider-facing approval-plan link back to the mirrored channel thread.

This is not approval or execution: the action does not grant approval, call a
model, execute the requested tool, generate tool inputs, create a tool-run
request, run shell commands, call provider APIs, or mutate the repository. The
approval-plan issue stores the normalized tool, approval-plan id, enabled
state, approval-required decision, run-allowed-now decision, validation status,
and body-free source hashes needed for a normal follow-up conversation; the
channel source receipt remains body-free, reporting only
approval-plan/thread/message/tool hashes, validation and gate metadata,
duplicate status, notification queue metadata, and delivery gates. It does not
print raw approval-plan ids, raw provider thread/message ids, raw requested
tool names, raw channel message bodies, raw tool inputs, raw tool outputs, or
raw approval payloads in the source receipt. Duplicates are suppressed first
by approval-plan id for the GitHub approval issue and then by
`channel + notify_message_id` for the provider-facing approval link. Changes
to this surface require a live E2E that records the approval dry-run,
validates the metadata-only approval-link outbox, checks duplicate
suppression, and then continues on the approval issue with a normal GitHub
Models repo-reader/search follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
GitHub tool rehearsal lane:

```text
@gitclaw /channels rehearse-tool <tool-name> --id <stable-rehearsal-id> --message-id <provider-message-id>
```

`/channels rehearse-tool`, `/channels tool-rehearse`,
`/channels tool-rehearsal`, `/channels try-tool`, `/channels tool-trial`,
`/channels practice-tool`, `/channels tool-lab`, and
`/channels tool-sandbox` infer the current channel and thread id from the
issue marker when no explicit channel/thread target is provided. They create
or reuse one open GitHub issue carrying the existing
`gitclaw:tool-rehearsal-issue` marker, label that issue for normal GitClaw
conversation, and queue a provider-facing rehearsal link back to the mirrored
channel thread.

This is not execution: the action does not call a model, execute the requested
tool, generate tool inputs, create a tool-run request, run shell commands, call
provider APIs, or mutate the repository. The tool rehearsal issue stores the
normalized tool, rehearsal id, validation status, and body-free source hashes
needed for a normal follow-up conversation; the channel source receipt remains
body-free, reporting only rehearsal/thread/message/tool hashes, validation
metadata, duplicate status, notification queue metadata, and delivery gates. It
does not print raw rehearsal ids, raw provider thread/message ids, raw
requested tool names, raw channel message bodies, raw tool inputs, or raw tool
outputs in the source receipt. Duplicates are suppressed first by rehearsal id
for the GitHub rehearsal issue and then by `channel + notify_message_id` for
the provider-facing rehearsal link. Changes to this surface require a live E2E
that records the rehearsal request, validates the metadata-only rehearsal-link
outbox, checks duplicate suppression, and then continues on the rehearsal issue
with a normal GitHub Models repo-reader/search follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
GitHub skill proposal intake lane:

```text
@gitclaw /channels propose-skill <skill-name> --message-id <provider-message-id>
```

`/channels propose-skill`, `/channels skill-propose`,
`/channels skill-proposal`, `/channels propose-skill-create`, and
`/channels propose-skill-update` infer the current channel and thread id from
the issue marker when no explicit channel/thread target is provided. They
create or reuse one open GitHub issue carrying the
`gitclaw:skill-proposal-issue` marker, then queue a provider-facing proposal
link back to the mirrored channel thread.

This is proposal intake, not skill generation or installation: the action does
not call a model, generate a skill body, write `.gitclaw/skill-proposals`,
write active `SKILL.md` files, run installers, call provider APIs, or mutate
the repository. The proposal issue stores the safe proposal name, planned
proposal action, review paths, target metadata, and body-free source hashes
needed for a normal follow-up conversation; the channel source receipt remains
body-free, reporting only proposal/thread/message/name/path hashes, validation
and review metadata, duplicate status, notification queue metadata, and
delivery gates. It does not print raw proposal names, raw provider
thread/message ids, raw proposal paths, raw destination paths, raw channel
message bodies, raw skill bodies, or generated skill text in the source
receipt. Duplicates are suppressed first by proposal name for the GitHub
proposal issue and then by `channel + notify_message_id` for the
provider-facing proposal link. Changes to this surface require a live E2E that
records the proposal intake, validates the metadata-only proposal-link outbox,
checks duplicate suppression, and then continues on the proposal issue with a
normal GitHub Models repo-reader/search follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
GitHub skill rehearsal lane:

```text
@gitclaw /channels rehearse-skill <skill-name> --id <stable-rehearsal-id> --message-id <provider-message-id>
```

`/channels rehearse-skill`, `/channels skill-rehearse`,
`/channels skill-rehearsal`, `/channels try-skill`,
`/channels skill-trial`, and `/channels practice-skill` infer the current
channel and thread id from the issue marker when no explicit channel/thread
target is provided. They create or reuse one open GitHub issue carrying the
existing `gitclaw:skill-rehearsal-issue` marker, label that issue for normal
GitClaw conversation, and queue a provider-facing rehearsal link back to the
mirrored channel thread. This is not skill installation or execution: the
channel action does not call a model, install skills, edit `SKILL.md`, fetch
registries, call provider APIs, or mutate the repository. The rehearsal issue
stores the requested skill, rehearsal id, match counts, validation summary, and
body-free source hashes needed for review; the channel source receipt remains
body-free, reporting only rehearsal/thread/message/skill hashes, validation
metadata, duplicate status, notification queue metadata, and delivery gates. It
does not print raw rehearsal ids, raw provider thread/message ids, raw channel
message bodies, or raw skill bodies in the source receipt. Duplicates are
suppressed first by rehearsal id for the GitHub rehearsal issue and then by
`channel + notify_message_id` for the provider-facing rehearsal link. Changes
to this surface require a live E2E that records the rehearsal, validates the
metadata-only rehearsal-link outbox, checks duplicate suppression, and then
continues on the rehearsal issue with a normal GitHub Models repo-reader/search
follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
high-authority-context proposal intake lane:

```text
@gitclaw /channels propose-soul --target <soul-path> --id <stable-proposal-id> --message-id <provider-message-id>
```

`/channels propose-soul`, `/channels soul-propose`,
`/channels soul-proposal`, `/channels edit-soul`, `/channels change-soul`, and
`/channels soul-change` infer the current channel and thread id from the issue
marker when no explicit channel/thread target is provided. They create or reuse
one open GitHub issue carrying the existing `gitclaw:soul-proposal-issue`
marker, then queue a provider-facing proposal link back to the mirrored channel
thread.

This is proposal intake, not a high-authority context edit: the action does
not call a model, generate candidate soul text, edit `.gitclaw/SOUL.md`,
`.gitclaw/IDENTITY.md`, `.gitclaw/USER.md`, or related context files, call
provider APIs, or mutate the repository. The proposal issue stores the proposal
id, target path, target category, validation and risk summaries, and body-free
source hashes needed for a normal follow-up conversation; the channel source
receipt remains body-free, reporting only proposal/thread/message/target
hashes, validation/risk metadata, duplicate status, notification queue
metadata, and delivery gates. It does not print raw proposal ids, raw provider
thread/message ids, raw target paths, raw channel message bodies, existing soul
content, or candidate soul text in the source receipt. Duplicates are
suppressed first by proposal id for the GitHub proposal issue and then by
`channel + notify_message_id` for the provider-facing proposal link. Changes to
this surface require a live E2E that records the proposal intake, validates the
metadata-only proposal-link outbox, checks duplicate suppression, and then
continues on the proposal issue with a normal GitHub Models repo-reader/search
follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
high-authority-context rehearsal lane:

```text
@gitclaw /channels rehearse-soul --target <soul-path> --id <stable-rehearsal-id> --message-id <provider-message-id>
```

`/channels rehearse-soul`, `/channels soul-rehearse`,
`/channels soul-rehearsal`, `/channels try-soul`, `/channels voice-test`,
`/channels tone-test`, and `/channels practice-soul` infer the current channel
and thread id from the issue marker when no explicit channel/thread target is
provided. They create or reuse one open GitHub issue carrying the existing
`gitclaw:soul-rehearsal-issue` marker, label that issue for normal GitClaw
conversation, and queue a provider-facing rehearsal link back to the mirrored
channel thread. This is not a context edit: the channel action does not call a
model, generate candidate soul text, edit `.gitclaw/` files, call provider
APIs, or mutate the repository. The rehearsal issue stores the target path,
validation and risk summaries, rehearsal id, and body-free source hashes needed
for review; the channel source receipt remains body-free, reporting only
rehearsal/thread/message/target hashes, validation/risk metadata, duplicate
status, notification queue metadata, and delivery gates. It does not print raw
rehearsal ids, raw provider thread/message ids, raw channel message bodies, raw
target bodies, or candidate soul text in the source receipt. Duplicates are
suppressed first by rehearsal id for the GitHub rehearsal issue and then by
`channel + notify_message_id` for the provider-facing rehearsal link. Changes
to this surface require a live E2E that records the rehearsal, validates the
metadata-only rehearsal-link outbox, checks duplicate suppression, and then
continues on the rehearsal issue with a normal GitHub Models repo-reader/search
follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
current-memory rehearsal lane:

```text
@gitclaw /channels rehearse-memory --target <memory-target> --id <stable-rehearsal-id> --message-id <provider-message-id>
```

`/channels rehearse-memory`, `/channels memory-rehearse`,
`/channels memory-rehearsal`, `/channels try-memory`,
`/channels memory-trial`, `/channels practice-memory`,
`/channels recall-test`, and `/channels memory-test` infer the current channel
and thread id from the issue marker when no explicit channel/thread target is
provided. They create or reuse one open GitHub issue carrying the existing
`gitclaw:memory-rehearsal-issue` marker, label that issue for normal GitClaw
conversation, and queue a provider-facing rehearsal link back to the mirrored
channel thread. This is not a memory edit: the channel action does not call a
model, generate candidate memory, edit `.gitclaw/` files, call provider APIs,
or mutate the repository. The rehearsal issue stores the normalized memory
target path, validation summary, rehearsal id, and body-free source hashes
needed for review; the channel source receipt remains body-free, reporting only
rehearsal/thread/message/target hashes, validation metadata, duplicate status,
notification queue metadata, and delivery gates. It does not print raw
rehearsal ids, raw provider thread/message ids, raw channel message bodies, raw
target memory bodies, or candidate memory text in the source receipt.
Duplicates are suppressed first by rehearsal id for the GitHub rehearsal issue
and then by `channel + notify_message_id` for the provider-facing rehearsal
link. Changes to this surface require a live E2E that records the rehearsal,
validates the metadata-only rehearsal-link outbox, checks duplicate
suppression, and then continues on the rehearsal issue with a normal GitHub
Models repo-reader/search follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
backup recovery rehearsal lane:

```text
@gitclaw /channels rehearse-backup --id <stable-rehearsal-id> --message-id <provider-message-id>
```

`/channels rehearse-backup`, `/channels backup-rehearse`,
`/channels backup-rehearsal`, `/channels rehearse-recovery`,
`/channels recovery-rehearsal`, `/channels backup-drill`, and
`/channels recovery-drill` infer the current channel and thread id from the
issue marker when no explicit channel/thread target is provided. They create or
reuse one open GitHub issue carrying the existing
`gitclaw:backup-rehearsal-issue` marker, label that issue for normal GitClaw
conversation, and queue a provider-facing rehearsal link back to the mirrored
channel thread. This is not a restore: the channel action does not call a
model, read raw backup payloads, restore files, replay GitHub API calls, call
provider APIs, or mutate the repository. The rehearsal issue stores the backup
issue number, expected backup branch/root/path metadata, dry-run command
suggestions, rehearsal id, and body-free source hashes needed for review; the
channel source receipt remains body-free, reporting only
rehearsal/thread/message hashes, backup path hashes, duplicate status,
notification queue metadata, and delivery gates. It does not print raw
rehearsal ids, raw provider thread/message ids, raw channel message bodies,
raw backup paths, source bodies, or backup payload bodies in the source
receipt. Duplicates are suppressed first by rehearsal id for the GitHub
rehearsal issue and then by `channel + notify_message_id` for the
provider-facing rehearsal link. Changes to this surface require a live E2E
that records the rehearsal, validates the metadata-only rehearsal-link outbox,
checks duplicate suppression, verifies the source issue backup appears on the
`gitclaw-backups` branch, and then continues on the rehearsal issue with a
normal GitHub Models repo-reader/search follow-up.

The same channel-thread issue can also turn a mirrored provider message into a
GitHub-native reminder:

```text
@gitclaw /channels remind --reminder-id <stable-reminder-id> --message-id <provider-message-id> --at <RFC3339-or-date>
Title: short reminder title
Notes:
optional human-readable reminder notes
```

`/channels remind`, `/channels reminder`, `/channels remind-me`,
`/channels snooze`, `/channels follow-up`, and `/channels followup` infer the
current channel and thread id from the issue marker when no explicit
route/channel/thread target is provided. They create or reuse one open GitHub
issue carrying a hidden `gitclaw:channel-reminder` marker for the stable
reminder id and normalized `not_before` due gate, label it with `gitclaw` so
normal conversation can continue there, and queue a provider-facing reminder
link back to the mirrored channel thread. The reminder issue contains the
human-readable due time, title, and notes because it is the scheduled nudge;
the source receipt remains body-free, reporting only reminder/thread/message/
due/title/note hashes, duplicate status, notification queue metadata, and
delivery gates. It does not call a model, call provider APIs, print raw
reminder ids, print raw due times, print raw thread ids, print raw source or
notification message ids, print channel message bodies, or print raw reminder
title/notes in the source receipt. Duplicates are suppressed first by
`reminder_id` for the GitHub reminder issue and then by
`channel + notify_message_id` for the provider-facing reminder link. Scheduled
GitHub Actions can later scan reminder issues by `not_before` and wake the
normal issue conversation with `workflow_dispatch`, avoiding sockets and
webhooks. Changes to this surface require a live E2E that creates the reminder,
validates the metadata-only reminder-link outbox, checks duplicate suppression,
and then continues on the reminder issue with a normal GitHub Models
repo-reader/search follow-up.

The channel-created task, clip, attachment, decision, digest, and reminder
issues also accept a completion form:

```text
@gitclaw /channels done --message-id <stable-outbound-id>
```

`/channels done`, `/channels complete`, `/channels close`, and
`/channels resolve` read the channel artifact marker from the current GitHub
issue, load the original `gitclaw:channel-thread` source issue to recover the
provider thread target, queue one provider-facing done acknowledgement through
the normal `channel-send` path, and close the current artifact issue. The
artifact issue receipt remains body-free: it reports artifact kind, artifact
issue, close status, source channel issue, notification queue metadata, hashes,
and delivery gates. It does not call a model, call provider APIs, print raw
artifact ids, print raw thread ids, print raw notification message ids, print
artifact titles or bodies, print channel message bodies, or perform provider
delivery. Duplicate done acknowledgements are suppressed by
`channel + notify_message_id`. Changes to this surface require a live E2E that
creates a channel task, completes it from the task issue, validates
metadata-only done acknowledgement outbox discovery, checks duplicate
suppression, confirms the task issue is closed, and then continues on the
original channel issue with a normal GitHub Models repo-reader/search
follow-up.

Behavior:

- find or create the same `gitclaw:channel-thread` issue used by inbound
  channel ingest,
- if `--route` is provided, resolve the route before validation and reject
  conflicting explicit `--channel` or `--thread-id` values,
- label it with `gitclaw:channel` but do not apply the normal `gitclaw`
  trigger label,
- post one `gitclaw:channel-outbound` comment per `channel + message_id`,
- post one `gitclaw:channel-deliverable` comment per `channel +
  deliverable_id`,
- post one `gitclaw:channel-reaction` comment per
  `channel + target_message_id + reaction`,
- post one `gitclaw:channel-status` comment per `channel + status_id`,
- post one `gitclaw:channel-edit` comment per `channel + edit_id`,
- create or reuse one `gitclaw:channel-task` issue per task id and queue one
  provider-facing task-link outbound comment per `channel + notify_message_id`,
- create or reuse one `gitclaw:channel-clip` issue per clip id and queue one
  provider-facing clip-link outbound comment per `channel + notify_message_id`,
- create or reuse one `gitclaw:channel-attachment` issue per attachment id and
  queue one provider-facing attachment-link outbound comment per
  `channel + notify_message_id`,
- create or reuse one `gitclaw:channel-decision` issue per decision id and
  queue one provider-facing decision-link outbound comment per
  `channel + notify_message_id`,
- create or reuse one `gitclaw:channel-digest` issue per digest id and queue
  one provider-facing digest-link outbound comment per
  `channel + notify_message_id`,
- create or reuse one `gitclaw:tool-run-request-issue` issue per channel
  request id and queue one provider-facing review-link outbound comment per
  `channel + notify_message_id` without executing a model or tool,
- create or reuse one `gitclaw:skill-rehearsal-issue` issue per channel
  rehearsal id and queue one provider-facing rehearsal-link outbound comment
  per `channel + notify_message_id` without executing a model or editing a
  skill,
- create or reuse one `gitclaw:soul-rehearsal-issue` issue per channel
  rehearsal id and queue one provider-facing rehearsal-link outbound comment
  per `channel + notify_message_id` without executing a model or editing
  high-authority context,
- create or reuse one `gitclaw:channel-reminder` issue per reminder id and
  queue one provider-facing reminder-link outbound comment per
  `channel + notify_message_id`,
- close one `gitclaw:channel-task`, `gitclaw:channel-clip`,
  `gitclaw:channel-attachment`, `gitclaw:channel-decision`,
  `gitclaw:channel-digest`, or `gitclaw:channel-reminder` issue and queue one
  provider-facing done acknowledgement per `channel + notify_message_id`,
- suppress duplicate outbound message IDs,
- for issue-native `/channels send`, post a `model="gitclaw/channels"`
  receipt back to the source issue,
- leave provider delivery to `channel-outbox` plus `channel-delivery`, so
  provider credentials and message send APIs stay outside the core assistant
  turn.

`.github/workflows/gitclaw-channel-send.yml` wraps the command with
`workflow_dispatch`, optional `route`, and `issues: write`. This makes
scheduled jobs and manual bridge tests able to queue Slack/Telegram work
without a webhook server, socket service, or hidden delivery database. Changes
to this workflow must prove duplicate suppression, named-route resolution,
pending outbox discovery, delivery receipt retry suppression, and a normal
GitHub Models repo-reader/search follow-up on the same issue.

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
webhook endpoint or a local machine with credentials. Changes to this workflow
must prove three things in live E2E: hash-only state issue contents, duplicate
offset suppression, and two normal GitHub Models repo-reader/search turns on
the state issue to prove continued conversation.

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
state durable in GitHub issues. Changes to this workflow must prove hash-only
lease state, duplicate lease suppression, and two normal GitHub Models
repo-reader/search turns on the lease state issue.

### Channel Outbox Command

Provider gateways need a way to discover assistant replies that are ready to
send back to Telegram, Slack, or another channel without scraping every issue
comment themselves. GitClaw exposes that outbound queue as a body-safe outbox:

```bash
gitclaw channel-outbox \
  --repo OWNER/REPO \
  --channel telegram \
  --account-id <provider-account-or-workspace-id> \
  --issue-number <github-issue> \
  --out outbox.json
```

Behavior:

- verify the source issue carries a matching `gitclaw:channel-thread` marker,
- list assistant comments carrying `gitclaw:assistant-turn` plus channel
  action comments carrying `gitclaw:channel-outbound`,
  `gitclaw:channel-deliverable`, `gitclaw:channel-reaction`,
  `gitclaw:channel-status`, or `gitclaw:channel-edit`,
- find the matching `gitclaw:channel-state` issue and read
  `gitclaw:channel-delivery` receipts,
- return only deliverable comments that have not yet been delivered for
  `channel + account_sha256_12 + source issue + source comment`,
- write provider-visible bodies only to an explicit local `--out` JSON file
  when `--include-body` is set,
- keep stdout, `GITHUB_OUTPUT`, logs, state issues, and receipts metadata-only
  by default.

`.github/workflows/gitclaw-channel-outbox.yml` wraps the same command with
`workflow_dispatch`, `issues: read`, and no artifact upload. A provider gateway
can dispatch it, read the local runner file, send pending replies through the
provider API, and then dispatch `gitclaw-channel-delivery.yml` for each sent
comment. This gives the no-server bridge an explicit outbound edge without
turning GitHub Actions logs into a transcript store.

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
  --comment-id <github-assistant-or-outbound-comment-id> \
  --external-message-id <provider-message-id>
```

Behavior:

- verify the source comment exists and carries a provider-deliverable marker
  such as `gitclaw:assistant-turn`, `gitclaw:channel-outbound`,
  `gitclaw:channel-deliverable`, `gitclaw:channel-reaction`,
  `gitclaw:channel-status`, or `gitclaw:channel-edit`,
- find or create the matching `gitclaw:channel-state` issue,
- post one `gitclaw:channel-delivery` receipt for
  `channel + account_sha256_12 + source issue + source comment`,
- store the external provider message id only as `external_message_sha256_12`,
- treat repeated delivery receipts for the same source comment as duplicates.

`.github/workflows/gitclaw-channel-delivery.yml` exposes the same receipt path
through `workflow_dispatch`, so a gateway can send a reply through Telegram or
Slack and then use the repository `GITHUB_TOKEN` to record exactly what GitHub
assistant or queued outbound comment was delivered without writing channel
credentials or reply bodies into the state issue. Changes to this workflow
must prove source assistant/outbound verification, hash-only outbound receipt
state, duplicate receipt suppression, and normal GitHub Models repo-reader/
search turns that do not leak source bodies or provider message IDs.

### Channel Inspection Command

GitClaw supports a deterministic channel/control-plane audit command:

```text
@gitclaw /channels
@gitclaw /channels list
@gitclaw /channels verify
@gitclaw /channels risk
@gitclaw /channels info telegram
@gitclaw /channels send --route team-alerts --message-id alert-123
@gitclaw /channels status --message-id provider-msg-1 --status-id status-1 --state working
@gitclaw /channels edit --message-id provider-msg-1 --edit-id edit-1
@gitclaw /channels deliverable --deliverable-id channel-file-1 --message-id provider-msg-1 --filename launch-report.pdf --media-type application/pdf --url https://example.invalid/launch-report.pdf
@gitclaw /channels task --task-id channel-task-1 --message-id provider-msg-1
@gitclaw /channels clip --clip-id channel-clip-1 --message-id provider-msg-1
@gitclaw /channels attachment --attachment-id channel-attachment-1 --message-id provider-msg-1 --filename launch-brief.pdf --media-type application/pdf --bytes 4242
@gitclaw /channels decision --decision-id channel-decision-1 --message-id provider-msg-1
@gitclaw /channels digest --digest-id channel-digest-1 --message-id provider-msg-1
@gitclaw /channels request-run search_files --id channel-tool-request-1 --message-id provider-msg-1
@gitclaw /channels approval-plan search_files --id channel-tool-approval-1 --message-id provider-msg-1
@gitclaw /channels rehearse-tool search_files --id channel-tool-rehearsal-1 --message-id provider-msg-1
@gitclaw /channels propose-skill weekly-review --message-id provider-msg-1
@gitclaw /channels rehearse-skill repo-reader --id channel-skill-rehearsal-1 --message-id provider-msg-1
@gitclaw /channels propose-soul --target soul --id channel-soul-proposal-1 --message-id provider-msg-1
@gitclaw /channels rehearse-soul --target soul --id channel-soul-rehearsal-1 --message-id provider-msg-1
@gitclaw /channels rehearse-memory --target long-term --id channel-memory-rehearsal-1 --message-id provider-msg-1
@gitclaw /channels rehearse-backup --id channel-backup-rehearsal-1 --message-id provider-msg-1
@gitclaw /channels room team-alerts,ops-alerts --room-id design-room --message-id design-room-1
@gitclaw /channels huddle team-alerts,ops-alerts --huddle-id design-room --message-id design-room-1
@gitclaw /channels rollcall team-alerts,ops-alerts --rollcall-id standup --message-id standup-1
```

The command runs after normal preflight and context loading, but before model
inference. Audit subcommands post a `gitclaw:assistant-turn` comment with
`model="gitclaw/channels"` and summarizes:

- the canonical channel label,
- the generic channel-ingest workflow path and metadata,
- the channel outbox workflow path and metadata,
- whether `workflow_dispatch` is configured,
- whether the ingest workflow has `actions: write` and `issues: write`,
- normalized workflow input count,
- whether the current issue is a channel thread,
- how many existing comments carry `gitclaw:channel-message`,
- supported provider keys and the dispatch-id contract.

It never dumps channel message, issue, command, or workflow bodies. This keeps
Slack/Telegram bridge debugging GitHub-native and auditable without making the
channel bridge itself a privileged hidden conversation store.

`@gitclaw /channels send --route <name>` is the action subcommand. It uses the
same `model="gitclaw/channels"` marker for the source-issue receipt, but it
queues a `gitclaw:channel-outbound` comment on the canonical channel issue
instead of producing only an audit report. This is the GitHub-native operator
equivalent of asking the assistant to "say this in Slack/Telegram" while
keeping route resolution, duplicate suppression, and provider delivery
auditable in issues.

`@gitclaw /channels reply --message-id <id>` is the channel-thread shortcut.
When posted on a mirrored Slack/Telegram issue, it infers the current
`gitclaw:channel-thread` marker and queues the outbound comment on the same
issue. The source receipt is still body-free and deterministic, but the channel
thread now supports a real back-channel action instead of only reports.

`@gitclaw /channels verify` uses the same data surface but switches from
inventory to health posture. It reports `channel_verify_status`, confirms the
channel-ingest workflow has `workflow_dispatch`, `actions: write`,
`issues: write`, and the five normalized inputs required for channel mirroring,
then lists body-free verification findings. It carries
`llm_e2e_required_after_channel_verify_change=true`; changes to this surface
must pair the deterministic bridge health report with a normal GitHub Models
follow-up that selects `repo-reader`, exposes `gitclaw.search_files`, and
recovers a bounded repository-search fixture token without echoing channel or
command sentinels.

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
gitclaw channel-send --route team-alerts --message-id alert-123 --body "message to send"
gitclaw channel-state --channel telegram --account-id <id> --offset <offset>
gitclaw channel-gateway --channel telegram --account-id <id> --renew
gitclaw channel-outbox --channel telegram --account-id <id> --issue-number <issue> --out <file>
gitclaw channel-delivery --channel telegram --account-id <id> --issue-number <issue> --comment-id <comment> --external-message-id <message>
```

The local report omits issue-only fields such as repository, issue number,
channel-thread status, marker counts, and title hash, but still verifies the
workflow-dispatch bridge shape, provider keys, labels, permissions, ingest
contract, outbox contract, and delivery receipt contract.

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
  -> read pending outbound GitClaw comments with channel-outbox
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

`@gitclaw /artifacts risk` is the stricter body-free risk audit. It treats
GitHub Actions artifacts as ephemeral evidence, the git backup branch as the
durable transcript backend, and flags any artifact path that tries to smuggle
conversation state, raw prompts, tool outputs, secrets, or external storage into
the agent loop.

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
Because every downstream backup command depends on this index, index-surface
changes also require a normal GitHub Models follow-up that proves repo-reader
skill selection, `gitclaw.search_files` visibility, usage telemetry, and a
bounded repository-search fixture token without echoing issue/comment
sentinels.

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

- requested backup command intent (`summary`, `catalog`, `verify`, `coverage`,
  `manifest`, `list`, `timeline`, `info`, `stats`, `search`, `export-jsonl`,
  `restore-plan`, or `retention-plan`),
- the recovery rehearsal issue action
  (`@gitclaw /backup rehearse --id <stable-rehearsal-id>`),
- the restore request issue action
  (`@gitclaw /backup restore-request --id <stable-request-id>`),
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
backup job. Summary report changes carry
`llm_e2e_required_after_backup_report_change: true` and require a fetched-branch
backup proof plus a normal GitHub Models repo-reader/search follow-up.

Issue-side backup subcommands intentionally mirror OpenClaw's manifest-oriented
backup verification and Hermes' exportable session artifacts without pretending
the issue handler can verify a branch that has not been written yet. For
example, `@gitclaw /backup catalog` records a compact recovery-surface catalog:
issue intents, local commands, fetched-branch gates, body-free output policy,
and explicit no-mutation restore/retention boundaries. Catalog changes carry
`llm_e2e_required_after_backup_catalog_change: true` and require three proofs:
the deterministic issue-side catalog, a post-turn `gitclaw-backups` branch
update for the same issue, and a normal GitHub Models repo-reader/search
follow-up. `@gitclaw /backup verify` records the exact local verification
command and the backup paths, then the post-turn backup job commits the raw
issue JSON and index to `gitclaw-backups`. `@gitclaw /backup risk` records the
exact local risk-audit command and risk categories while making clear that raw
payload scanning is deferred to a fetched backup branch.
`@gitclaw /backup provenance` records the exact local git-provenance command
and the body-free gates for verifying tracked, clean, committed backup files
without printing raw commit subjects or author identities.
`@gitclaw /backup info <issue-number>` records the exact focused-inspection
command for one backed-up issue, defaulting to the current issue when no number
is supplied; info-surface changes carry
`llm_e2e_required_after_backup_info_change: true` and require a fetched-branch
info proof plus a normal GitHub Models repo-reader/search follow-up. The
`@gitclaw /backup snapshot` command records the exact local lockfile-style
fingerprint command, defers branch inspection to a fetched backup checkout, and
advertises `llm_e2e_required_after_backup_snapshot_change: true` because every
snapshot-surface change must be proven with both a fetched-branch snapshot and
a normal GitHub Models repo-reader/search follow-up. The
`@gitclaw /backup timeline` command records the exact chronological timeline
command for a fetched backup branch without trying to inspect raw backup
payloads from the pre-backup issue handler. `@gitclaw /backup search <query>`
records only a query hash and term count; raw search terms and raw backup
bodies stay out of the issue-visible comment. Search-surface changes carry
`llm_e2e_required_after_backup_search_change: true` and must be proven with a
fetched-branch backup search plus a normal GitHub Models repo-reader/search
follow-up.

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
counts, paths, failures, and
`llm_e2e_required_after_backup_verify_change=true`. It exits non-zero when
verification fails. It does not print issue bodies, comments, or transcript
text. Verifier changes must be proven with both a fetched-branch backup audit
and a normal GitHub Models follow-up using repo-reader search.

## Backup Coverage Command

GitClaw also supports a focused backup coverage command for one conversation:

```bash
gitclaw backup coverage --root .gitclaw/backups --repo <owner/repo> --issue 123
```

Coverage verifies one requested issue against the fetched `gitclaw-backups`
tree. It checks that the issue is indexed, points at the canonical
`issues/000123.json` path, has a readable payload, and exposes only path,
count, timestamp, and hash metadata. It exits non-zero when the backup is
missing or the backup tree is not cleanly verified, making it useful in E2E and
disaster-recovery runbooks.

Issue-side `@gitclaw /backup coverage` defaults to the current issue. If a
numeric issue argument such as `#123` is present, GitClaw records that target;
otherwise trailing prose in an issue title is ignored so E2E labels and run
descriptions do not turn into invalid issue numbers.

## Backup Drill Command

GitClaw supports a local restore-readiness drill for one backed-up
conversation:

```bash
gitclaw backup drill --root .gitclaw/backups --repo <owner/repo> --issue 123
```

The drill composes three non-mutating gates against a fetched
`gitclaw-backups` tree:

- verify the repo-scoped backup index, README, schema, and canonical payload
  paths,
- prove the requested issue is indexed, canonical, and readable,
- build a dry-run restore plan for the same issue and target repository.

The deterministic `GitClaw Backup Drill Report` prints gate statuses,
backup paths, schema/timestamp metadata, counts, payload hashes, and body
hashes. It does not print issue titles, issue bodies, comments, transcript
messages, prompts, restored content, or provider responses, and it does not
call GitHub APIs or mutate the repository.

Issue-side `@gitclaw /backup drill` is intentionally deferred just like other
backup inspection commands: the assistant records the concrete local command
to run after the post-turn backup job writes the `gitclaw-backups` branch.
Changes to this surface must include a live GitHub Models follow-up E2E that
proves normal inference, repo-reader skill selection, and prompt-visible tool
usage after the deterministic drill report.

## Backup Rehearsal Issue Action

GitClaw supports a conversation-creating recovery rehearsal action:

```text
@gitclaw /backup rehearse --id <stable-rehearsal-id>
```

Aliases are `/backup rehearsal`, `/backup restore-rehearsal`,
`/backup recovery`, and `/backup recover`; `/backups` may be used instead of
`/backup`. The action runs before model inference, creates or reuses one open
GitHub issue marked with `gitclaw:backup-rehearsal-issue`, and labels that
issue with the normal trigger label so the operator can continue the rehearsal
as a GitHub Models conversation.

The source receipt is body-free and model-free. It includes the rehearsal issue
number, duplicate suppression status, backup issue number, backup branch/root,
hashes for repo backup paths, and explicit gates:

- `restore_mode: dry-run`,
- `repository_mutation_allowed: false`,
- `backup_branch_write_allowed: false`,
- `github_api_replay_allowed: false`,
- `raw_source_body_included: false`,
- `raw_backup_bodies_included: false`.

The action does not read raw backup payloads, restore files, mutate the
repository, replay GitHub API calls, or call a model. The rehearsal issue body
may include the stable rehearsal id and expected backup paths because it is the
operator-facing lane, but it must not copy raw source text. Duplicate
suppression is keyed by the hidden rehearsal marker and sanitized id.

Every change to this surface must run a live E2E that creates a real source
issue, verifies the rehearsal issue was opened and labeled, fetches the real
`gitclaw-backups` branch, runs `gitclaw backup coverage`, `gitclaw backup
drill`, and `gitclaw backup restore-plan` for the source issue, posts a
duplicate rehearsal request, and finally comments on the rehearsal issue with a
repo-reader search task that must make a GitHub Models call with prompt
context, selected skill, prompt-visible `gitclaw.search_files`, and usage
telemetry.

```text
@gitclaw /backup restore-request --id <stable-request-id>
```

Aliases are `/backup request-restore`, `/backup restore-issue`,
`/backup recovery-request`, and `/backup request-recovery`; `/backups` may be
used instead of `/backup`. The action runs before model inference, creates or
reuses one open GitHub issue marked with
`gitclaw:backup-restore-request-issue`, and labels that issue with the normal
trigger label so the operator can continue the restore review as a GitHub
Models conversation.

The source receipt is body-free and model-free. It includes the restore request
issue number, duplicate suppression status, backup issue number, target
repository, backup branch/root, hashes for repo backup paths, and explicit
restore gates:

- `approval_required: true`,
- `restore_pr_required: true`,
- `restore_mode: dry-run-first`,
- `repository_mutation_allowed: false`,
- `backup_branch_write_allowed: false`,
- `github_api_replay_allowed: false`,
- `raw_source_body_included: false`,
- `raw_backup_bodies_included: false`.

The action does not read raw backup payloads, restore files, mutate the
repository, write the backup branch, replay GitHub API calls, or call a model.
The generated restore request issue body may include the stable request id,
expected backup paths, and dry-run commands because it is the operator-facing
review lane, but it must not copy raw source text. Duplicate suppression is
keyed by the hidden restore-request marker and sanitized id.

When the same command includes `--notify-route <route>` or
`--notify-routes <route-a,route-b>`, GitClaw also queues one reviewed outbound
channel notification per route. The source receipt records only route/message
body hashes, destination issue numbers, queue counts, duplicate counts, channel
names, and provider-delivery gates; it never prints raw route names, raw
provider thread ids, raw outbound body text, source text, or raw backup bodies.
The outbound channel comment may include the restore request issue, source
issue, request id, backup issue, target repo, backup branch, and restore gate
status so the external channel receives useful recovery context, while actual
provider delivery stays delegated to `gitclaw channel-outbox` and
`gitclaw channel-delivery`.

Every change to this surface must run a live E2E that creates a real source
issue, verifies the restore-request issue was opened and labeled, fetches the
real `gitclaw-backups` branch, runs `gitclaw backup verify`, `gitclaw backup
coverage`, `gitclaw backup drill`, `gitclaw backup restore-plan`, and
`gitclaw backup manifest` for the source issue, posts a duplicate restore
request, and finally comments on the restore-request issue with a repo-reader
search task that must make a GitHub Models call with prompt context, selected
skill, prompt-visible `gitclaw.search_files`, and usage telemetry.

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

## Backup Provenance Command

GitClaw supports a local backup git-provenance audit for fetched backup
branches:

```bash
gitclaw backup provenance --root .gitclaw/backups --repo <owner/repo>
```

The command first runs the normal backup verifier, then inspects the fetched
git worktree for the repo-scoped `index.json`, `README.md`, and indexed issue
payload files. The deterministic `GitClaw Backup Provenance Report` includes:

- backup provenance and verify status,
- schema version, index generation time, issue count, and provenance-file
  counts,
- git availability, current branch, tracked/untracked counts, dirty-file
  counts, commit availability, and branch-history gate status,
- per-file kind, canonical backup path, git path, byte/line counts, file hash,
  issue number when applicable, backup timestamp/event metadata, commit
  availability, commit SHA hash, short commit SHA, commit date, and commit
  subject hash,
- explicit `raw_backup_bodies_included: false`,
  `raw_git_subjects_included: false`, and
  `author_identities_included: false` markers.

It never prints raw issue titles, issue bodies, comment bodies, transcript
messages, commit subjects, or commit author identities. This is the
git-native counterpart to OpenClaw-style backup verification and Hermes-style
session provenance: before trusting a backup branch, GitClaw can prove that the
backup files are tracked, clean, and committed.

Issue-side `@gitclaw /backup provenance` is intentionally deferred: the issue
handler records `gitclaw backup provenance --root .gitclaw/backups --repo
<owner/repo>` and the body-free provenance gates. The real audit runs after
fetching the post-turn `gitclaw-backups` branch. Its live E2E also posts a
second normal comment that forces a GitHub Models call and repo-reader tool
usage, so provenance changes are tested with deterministic backup metadata and
real model inference.

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
uses the explicit raw `export-jsonl` path. Manifest-surface changes carry
`llm_e2e_required_after_backup_manifest_change: true` and require a
fetched-branch manifest proof plus a normal GitHub Models repo-reader/search
follow-up.

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
every raw JSON file would be noisy. Stats-surface changes carry
`llm_e2e_required_after_backup_stats_change: true` and require a fetched-branch
stats proof plus a normal GitHub Models repo-reader/search follow-up.

## Backup Snapshot Command

GitClaw supports a local backup snapshot command inspired by OpenClaw's
manifest/verify posture and Hermes' portable session export fingerprints:

```bash
gitclaw backup snapshot --root .gitclaw/backups --repo <owner/repo>
```

The command reads a fetched `gitclaw-backups` tree, verifies it, and prints a
deterministic `GitClaw Backup Snapshot Report` with:

- backup snapshot and verify status,
- snapshot version, scope, and composite snapshot hash,
- control-file entries for `index.json` and `README.md`,
- issue payload entries with paths, bytes, short hashes, event names,
  generated timestamps, schema versions, title hashes, and count telemetry,
- aggregate issue, comment, transcript, user-message, assistant-message,
  assistant-turn, error-comment, and payload-byte counts,
- explicit gates showing restore and mutation are disabled.

It never prints raw issue titles, issue bodies, comments, transcript messages,
prompts, search queries, or tool outputs. The snapshot report is the
lockfile-style proof that a fetched backup branch has the expected shape before
restore planning, retention planning, export, or off-repo mirroring. Snapshot
changes carry `llm_e2e_required_after_backup_snapshot_change: true` and require
a fetched-branch snapshot proof plus a normal GitHub Models repo-reader/search
follow-up.

## Backup Freshness Command

GitClaw supports a local backup freshness command inspired by OpenClaw's
verify-before-restore posture and Hermes' session-freshness operational checks:

```bash
gitclaw backup freshness --root .gitclaw/backups --repo <owner/repo> --max-age-hours 24
```

The command reads a fetched `gitclaw-backups` tree, verifies it, finds the
latest indexed backup by backup generation time, and prints a deterministic
`GitClaw Backup Freshness Report` with:

- backup freshness and verify status,
- freshness gate result (`pass` when the latest verified backup is within the
  configured max age),
- schema version and index generation time,
- indexed issue count, max-age seconds, and `as_of` timestamp,
- latest issue path, backup timestamp, age seconds, clock-skew seconds,
  payload byte count, payload hash, event name, and title hash.

It never prints raw issue titles, issue bodies, comments, transcript messages,
prompts, search queries, or tool outputs. The issue-side `/backup freshness`
report is metadata-only because the backup branch is written after the current
assistant turn; it records the concrete local command and a deferred
`latest-backup-age <= max-age` gate for a later fetched-branch check.
Freshness-surface changes carry
`llm_e2e_required_after_backup_freshness_change: true` and require a
fetched-branch freshness proof plus a normal GitHub Models repo-reader/search
follow-up.

## Backup Continuity Command

GitClaw supports a local backup continuity command inspired by OpenClaw's
scheduled-run history inspection and Hermes' timestamped session metadata:

```bash
gitclaw backup continuity --root .gitclaw/backups --repo <owner/repo> --max-gap-hours 168
```

The command reads a fetched `gitclaw-backups` tree, verifies it, sorts indexed
backups chronologically, and prints a deterministic
`GitClaw Backup Continuity Report` with:

- backup continuity and verify status,
- continuity gate result (`pass` when the longest chronological backup gap is
  within the configured max gap),
- schema version and index generation time,
- indexed issue count, scanned point count, max-gap seconds, gap count over
  threshold, and reported gap count,
- first/latest issue numbers and timestamps,
- total span seconds,
- longest gap duration and its source/destination issue numbers and
  timestamps,
- gap cards with issue numbers, paths, timestamps, event names, gap seconds,
  and title hashes for gaps that exceed the threshold.

It never prints raw issue titles, issue bodies, comments, transcript messages,
prompts, search queries, or tool outputs. The issue-side `/backup continuity`
report is metadata-only because the backup branch is written after the current
assistant turn; it records the concrete local command and a deferred
`longest-backup-gap <= max-gap` gate for a later fetched-branch check.
Continuity-surface changes carry
`llm_e2e_required_after_backup_continuity_change: true` and require a
fetched-branch continuity proof plus a normal GitHub Models repo-reader/search
follow-up.

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
`backup info`, `backup snapshot`, `backup manifest`, `backup search`,
`backup restore-plan`, or raw `backup export-jsonl` command. List-surface changes carry
`llm_e2e_required_after_backup_list_change: true` and require a fetched-branch
list proof plus a normal GitHub Models repo-reader/search follow-up.

## Backup Timeline Command

GitClaw supports a local backup timeline command inspired by OpenClaw's session
management/export surface and Hermes' cross-session recall posture:

```bash
gitclaw backup timeline --root .gitclaw/backups --repo <owner/repo> --limit 20
```

The command reads a fetched `gitclaw-backups` tree, verifies it, selects the
latest backups by backup timestamp, and renders those selected backups in
chronological order. The deterministic `GitClaw Backup Timeline Report`
includes:

- backup timeline and verify status,
- schema version and index generation time,
- indexed issue count, requested limit, returned timeline-point count, and
  chronological window metadata,
- first/latest issue numbers and backup timestamps for the selected window,
- total span seconds across the selected window,
- per-point issue number, canonical path, timestamp, event name, gap from the
  previous point, payload size/hash, comment/transcript counts,
  assistant-turn/error counts, and issue-title hash.

It never prints raw issue titles, issue bodies, comment bodies, transcript
messages, prompts, search queries, or tool outputs. The timeline is a body-safe
continuity view for restoring or auditing a repo's backed-up conversation
history without opening raw JSON payloads.

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
- `llm_e2e_required_after_backup_retention_plan_change=true`, so retention-plan
  changes must be paired with a live GitHub Models follow-up that proves prompt
  context hashing, selected skills, prompt-visible repo-reader search, and
  usage markers.

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
- `llm_e2e_required_after_backup_restore_plan_change=true`, so restore-plan
  changes must be paired with a live GitHub Models follow-up that proves prompt
  context hashing, selected skills, prompt-visible repo-reader search, and
  usage markers.

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
   For ordinary issue conversation changes, `scripts/e2e/github-issue-chat.sh`
   is the baseline proof: the second comment must preserve transcript
   continuity and also force a fresh repo-reader/search result with prompt
   provenance and usage telemetry, even when earlier file-reference context
   remains prompt-visible in the continuous issue thread.

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
   - assert transcript includes prior assistant reply,
   - assert the second turn is a real GitHub Models call with usage telemetry,
   - assert the second turn selects `repo-reader`, exposes prompt-context
     provenance, and includes `gitclaw.search_files` in the assistant marker,
   - assert the second turn can recover a fresh repository-search fixture token
     from the follow-up comment, not only echo tokens from the original issue,
   - make the follow-up prompt protocol-shaped with fixed output labels and a
     token-prefix guard so small GitHub Models copy the search-result token
     instead of the search phrase,
   - allow earlier prompt-visible tools such as `gitclaw.read_file` to remain
     in the marker when prior turns requested file context, but require
     `gitclaw.search_files` evidence for the fresh follow-up fixture.

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
   - assert the marker includes the GitHub Models model id, prompt-context
     hash, context counts, and usage telemetry,
   - dispatch the same slot again,
   - assert no duplicate heartbeat comment is created,
   - post a normal `@gitclaw` issue-comment follow-up requiring `repo-reader`
     and bounded repository search,
   - assert the follow-up assistant turn is GitHub Models-backed, selects
     `repo-reader`, exposes `gitclaw.search_files`, recovers
     `GITCLAW_HEARTBEAT_FOLLOWUP_CONTEXT_V1`, and includes prompt provenance
     plus usage telemetry.

10. **Workflow dispatch wakeup**

   - create an issue without the normal trigger label or `@gitclaw` title
     prefix,
   - wait for the untriggered `issues.opened` workflow run to complete and
     assert it produced zero assistant comments,
   - add the `gitclaw` label only after that preflight run, so the manual
     dispatch owns the first assistant turn,
   - dispatch the main `gitclaw.yml` workflow with `issue_number` and a stable
     `dispatch_id`,
   - assert one assistant comment with a `dispatch-...` event marker and exact
     nonce token, plus GitHub Models model id, prompt-context hash, and usage
     telemetry,
   - dispatch the same `dispatch_id` again,
   - assert no duplicate assistant comment is created,
   - post a normal `@gitclaw` issue-comment follow-up requiring `repo-reader`
     and `gitclaw.search_files`,
   - assert the second assistant turn is GitHub Models-backed, selects
     `repo-reader`, exposes `gitclaw.search_files`, recovers
     `GITCLAW_WORKFLOW_DISPATCH_CONTEXT_V1` from `docs/search-fixture.md`, and
     does not leak follow-up sentinels.

11. **Channel message reconstruction**

   - create an untriggered issue,
   - post a comment whose body starts with
     `<!-- gitclaw:channel-message ... -->`,
   - add the `gitclaw` label after the mirrored comment is written,
   - dispatch the main workflow with `dispatch_id` equal to the channel message
     ID,
   - assert the assistant sees the mirrored message body, uses `repo-reader`
     and `gitclaw.search_files`, returns the bounded repository-search fixture
     token, includes model/prompt/usage telemetry, and does not echo the hidden
     channel sentinel.

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
     duplicate assistant turns,
   - post a normal `@gitclaw` follow-up in the proactive issue requiring
     `repo-reader` and bounded repository search,
   - assert the follow-up assistant turn is GitHub Models-backed, selects
     `repo-reader`, exposes `gitclaw.search_files`, recovers
     `GITCLAW_PROACTIVE_RUN_FOLLOWUP_CONTEXT_V1`, and does not leak hidden
     proactive prompt tokens.

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
   - create a real issue with `@gitclaw /prompt pack`,
   - create a real issue with `@gitclaw /prompt cache`,
   - create a real issue with `@gitclaw /prompt compression`,
   - ask for a concrete file read, selected skill, and search fixture phrase,
   - assert the reply is marked `model="gitclaw/prompt"`,
   - assert the report lists prompt budget settings, final prompt size/hash,
     transcript inclusion/truncation counts, selected context files, selected
     skills, and active tool output metadata,
   - assert the prompt-pack report lists fixed component order, head/tail
     projection status, 50% and 85% threshold findings, and body-free component
     ranges/hashes,
   - assert the prompt-cache report lists stable-prefix bytes/tokens,
     cache-control request gaps, usage-counter gaps, dynamic suffix boundary,
     heartbeat keep-warm workflow presence, and body-free segment hashes,
   - assert the prompt-compression report lists 50% and 85% threshold gates,
     disabled lossy-summary/session-split/write-memory gates, issue-thread
     canonical storage, backup replay posture, and body-free segment hashes,
   - assert the report does not dump prompt text, issue body tokens, context
     bodies, skill bodies, or tool output bodies,
   - assert deterministic report runs succeed without requiring a model
     provider response,
   - post a normal follow-up after the pack report that requires repo-reader
     search and assert the second assistant turn is model-backed by GitHub
     Models with prompt context, selected skill, and `gitclaw.search_files`.

18. **Memory inspection**

   - create a real issue with `@gitclaw /memory`,
   - create a second real issue with `@gitclaw /memory list`,
   - create a third real issue with `@gitclaw /memory verify`,
   - create a fourth real issue with `@gitclaw /memory info latest`,
   - create a fifth real issue with `@gitclaw /memory timeline`,
   - create a sixth real issue with `@gitclaw /memory catalog`,
   - create a seventh real issue with `@gitclaw /memory provenance`,
   - assert the reply is marked `model="gitclaw/memory"`,
   - assert the report lists `.gitclaw/MEMORY.md`, dated memory note counts,
     loaded/omitted note counts, and memory file hashes,
   - assert the verify report includes repo-local provenance, loaded state,
     external-provider/session-index/background-promotion non-goals, and
     body-free trust cards,
   - assert the info report includes the normalized memory path, match status,
     kind/source/canonical/latest/loaded metadata, and file hash without a
     body,
   - assert the timeline report includes first/latest note, span/gap metadata,
     prompt-visible state, validation/risk gates, and body-free file hashes,
   - assert the catalog report includes memory-layer roles, prompt-visible
     load modes, reason codes, validation/risk gates, body-hash gates, and the
     live-LLM E2E requirement,
   - assert the provenance report includes git-tracked/dirty state, commit
     hashes/dates, commit-subject hashes, disabled write/provider gates, and no
     raw memory or git subject bodies,
   - assert the report does not dump memory file bodies or issue body tokens,
   - assert the deterministic report succeeds without requiring a model
     provider response,
   - post normal follow-ups on the catalog and provenance issues that require repo-reader
     search and assert the second assistant turn is model-backed by GitHub
     Models with prompt context, selected skill, prompt-visible tool markers,
     and usage telemetry.

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
   - create another real issue with `@gitclaw /skills catalog`,
   - create another real issue with `@gitclaw /skills snapshot`,
   - create another real issue with `@gitclaw /skills provenance`,
   - create another real issue with `@gitclaw /bundles catalog`,
   - create a third real issue with `@gitclaw /bundles info repo-context`,
   - create a fourth real issue with `@gitclaw /bundles risk`,
   - create another real issue with
     `@gitclaw /bundles rehearse repo-context --id <id>`,
   - assert the reply is marked `model="gitclaw/skills"`,
   - assert the report lists available skill metadata and selected skill paths,
   - assert the catalog report lists compact eligibility counts, load modes,
     reason codes, selected/always state, description hashes, body hashes,
     no-registry/no-install gates, and no raw descriptions or skill bodies,
   - assert the snapshot report lists composite snapshot hash, skill cards,
     selected-skill prompt hashes, bundle instruction hashes, source-pin hashes,
     validation/risk/source gates, and no raw skill/source/bundle bodies,
   - assert the bundle info report lists bundle path, referenced/resolved
     skills, selected-for-turn state, instruction presence, and hashes,
   - assert the bundle catalog report lists compact orchestration metadata,
     selected/load state, instruction load mode, skill-ref gates, reason codes,
     risk rollups, and no raw bundle/skill/instruction bodies,
   - assert the bundle risk report lists body-free bundle risk status, counts,
     finding codes, severities, bundle hashes, and line hashes,
   - assert hashes, frontmatter/description presence, and requirement counts
     are present,
   - assert the provenance report includes tracked git state, working-tree
     dirty state, last commit IDs/dates, commit-subject hashes,
     validation/risk/git-history gates, installer-disabled gates, and no raw
     commit subjects or author identities,
   - assert skill validation status, duplicate-name count, invalid-name count,
     and folder/name mismatch count are present,
   - assert the report does not dump full skill bodies or verification tokens,
   - assert the run succeeds without requiring a model provider response,
   - post a normal follow-up after the catalog report that requires repo-reader
     search and assert the second assistant turn is model-backed by GitHub
     Models with prompt context, selected skill, usage telemetry, and
     `gitclaw.search_files`.

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
   - create another real issue with `@gitclaw /soul catalog`,
   - create another real issue with `@gitclaw /soul anchors`,
   - create another real issue with `@gitclaw /soul provenance`,
   - assert the reply is marked `model="gitclaw/soul"`,
   - assert the report lists loaded identity, policy, user, and memory paths
     with byte counts, line counts, and hashes,
   - assert the verify report includes repo-local source counts, required-file
     presence, soul frontmatter/description status, registry/profile export
     verification status, trust cards, and verification findings,
   - assert the risk report includes status/counts, risk cards, risk codes,
     line hashes, and `llm_e2e_required_after_soul_risk_change=true`,
   - assert the anchors report includes anchor roles, authority layers,
     required/loaded/prompt-visible/canonical flags, validation gates, risk
     gates, mutation-disabled gates, and
     `llm_e2e_required_after_soul_anchors_change=true`,
   - assert the catalog report includes compact authority-discovery counts,
     authority-layer names, load modes, reason codes, raw-body/raw-description
     exclusion gates, profile-export-disabled state, validation/risk gates, and
     `llm_e2e_required_after_soul_catalog_change=true`,
   - assert the provenance report includes tracked git state, last commit
     IDs/dates, commit-subject hashes, validation/risk/git-history gates, and
     no raw commit subjects or author identities,
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
   - create another real issue with
     `@gitclaw /tools rehearse search_files --id <id>`,
   - create another real issue with
     `@gitclaw /tools request-run search_files --id <id>`,
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
   - assert the rehearse action opens a dedicated labeled conversation issue,
     suppresses a duplicate rehearsal id, records validation/gate metadata,
     performs no model/tool execution, generates no tool inputs, creates no run
     request, and keeps raw source/tool bodies out of both the receipt and
     rehearsal issue,
   - assert the request-run action opens a dedicated review issue, suppresses a
     duplicate request id, records validation/gate metadata, performs no
     model/tool execution, and keeps raw source/tool bodies out of both the
     receipt and review issue,
   - assert tool validation status, contract counts, active-output counts,
     unknown-output counts, unsafe-contract counts, and over-limit output
     counts, missing-guidance count, and duplicate-contract count are present,
   - assert the report does not dump full file or search output bodies,
   - assert the run succeeds without requiring a model provider response.

28. **Diff inspection**

   - create a real issue with `@gitclaw /diffs`,
   - create a real issue with `@gitclaw /diffs risk`,
   - assert the reply is marked `model="gitclaw/diffs"`,
   - assert the report lists `.gitclaw/DIFFS.md`, diff spec metadata, git
     availability, repository state, clean/dirty state, change counts, numstat
     totals, raw-diff suppression, and changed-file metadata,
   - assert the risk report lists risk status/counts, policy/spec/git cards,
     raw-patch, destructive-action, hidden-state, external-storage, approval,
     and unbounded-collection boundaries,
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

30. **Workspace catalog inspection**

   - create a real issue with `@gitclaw /workspace catalog`,
   - assert the reply is marked `model="gitclaw/workspace"`,
   - assert the catalog lists catalog/summary/verify/risk commands, workspace
     policy/spec stores, git/workflow/context/repository-inventory layers,
     runtime/durable-state layers, and private-memory, external-mount, daemon,
     socket, raw-body, mutation, and model-E2E gates,
   - assert workspace policy/spec body tokens, workflow bodies, file bodies,
     issue body tokens, prompts, tool outputs, and secret values are not
     printed,
   - assert local `gitclaw workspace catalog` exposes the same body-free
     command/layer/gate surface,
   - run a real GitHub Models follow-up conversation that proves model
     inference, prompt provenance, selected skills, prompt-visible
     `gitclaw.search_files`, usage telemetry, and recovery of the
     workspace-catalog repository-search fixture token.

31. **Workspace risk inspection**

   - create a real issue with `@gitclaw /workspace risk`,
   - assert the reply is marked `model="gitclaw/workspace"`,
   - assert the risk report lists policy, spec, workflow, git, repository
     inventory, and current-request risk cards,
   - assert the report lists prompt-boundary, credential, private-memory,
     external-mount, destructive-mutation, long-running-service, raw-body,
     checkout/setup/fetch-depth, approval, and unbounded-inventory boundaries,
   - assert policy/spec body tokens, workflow bodies, file bodies, issue body
     tokens, prompts, tool outputs, and secret values are not printed,
   - assert the run succeeds without requiring a model provider response,
   - run a real GitHub Models conversation E2E in the same feature batch.

31. **Policy inspection**

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
   - assert the index contains metadata counts but not raw transcript bodies,
   - post a normal model-backed repo-reader/search follow-up using no-echo
     sentinels with a distinct prefix from the expected search fixture token,
     then assert the reply returns only the `gitclaw.search_files` token and
     does not echo issue/comment sentinels.

33. **Backup inspection**

   - create a real issue with `@gitclaw /backup`,
   - assert the reply is marked `model="gitclaw/backup"`,
   - assert the report lists the expected backup branch, issue backup path,
     index path, README path, and schema version,
   - assert the report carries
     `llm_e2e_required_after_backup_report_change: true`,
   - wait for the successful backup job,
   - assert the backup branch contains the issue JSON backup and repo index
     entry for that same issue,
   - assert the report does not dump issue or comment body tokens,
   - post a normal model-backed repo-reader/search follow-up using no-echo
     sentinels with a distinct prefix from the expected search fixture token,
     then assert the reply returns only the `gitclaw.search_files` token and
     does not echo issue/comment sentinels.

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
     unindexed issue files, and an index entry for the just-created issue,
   - post a normal model-backed repo-reader/search follow-up using no-echo
     sentinels with a distinct prefix from the expected search fixture token,
     then assert the reply returns only the `gitclaw.search_files` token and
     does not echo issue/comment sentinels.

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

36. **Backup provenance**

   - create a real issue with `@gitclaw /backup provenance`,
   - assert the issue-side report lists `requested_backup_command:
     provenance`, `backup_provenance_status: deferred`, the deferred execution
     marker, the concrete local provenance command, and no raw backup bodies,
     commit subjects, or author identities,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch with branch history,
   - run `gitclaw backup provenance --root <fetched>/.gitclaw/backups --repo
     <owner/repo>`,
   - assert the local provenance report lists verify status, git availability,
     clean/tracked counts, commit availability, index/README cards, and the
     just-created issue payload path,
   - assert it does not dump the issue body token, raw title, raw comments, raw
     transcript messages, git commit subjects, or author identities,
   - post a normal follow-up on the same issue that requires a repo-reader
     repository search and assert the second assistant turn is model-backed by
     GitHub Models with prompt context, selected skill, and prompt-visible tool
     markers.

37. **Backup manifest**

   - create a real issue with `@gitclaw /backup manifest`,
   - assert the issue-side report lists `requested_backup_command: manifest`,
     `issue_side_execution: deferred_to_post_turn_backup_branch`, and the
     concrete local manifest command without dumping body/title tokens,
   - assert the issue-visible report includes
     `llm_e2e_required_after_backup_manifest_change: true`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup manifest --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --issue <issue-number>`,
   - assert the manifest lists index/README control file hashes plus the
     just-created issue payload path, bytes, hash, schema, event, comment
     count, and transcript count,
   - assert it does not dump the issue body token or raw transcript bodies,
   - post a normal follow-up that requires repo-reader search and assert the
     second assistant turn is GitHub Models-backed with prompt context,
     selected skill, prompt-visible `gitclaw.search_files`, usage telemetry,
     and the backup-manifest repository-search fixture token.

38. **Backup stats**

   - create a real issue with `@gitclaw /backup stats`,
   - assert the issue-side report lists `requested_backup_command: stats`,
     the deferred execution marker, and the concrete local stats command
     without dumping body/title tokens,
   - assert the issue-visible report includes
     `llm_e2e_required_after_backup_stats_change: true`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup stats --root <fetched>/.gitclaw/backups --repo
     <owner/repo>`,
   - assert the report is marked `backup_stats_status: ok` and
     `backup_verify_status: ok`,
   - assert it lists aggregate issue/comment/message counts, latest backup
     metadata, event counts, and payload bytes,
   - assert it does not dump the issue body token or raw title,
   - post a normal follow-up that requires repo-reader search and assert the
     second assistant turn is GitHub Models-backed with prompt context,
     selected skill, prompt-visible `gitclaw.search_files`, usage telemetry,
     and the backup-stats repository-search fixture token.

39. **Backup snapshot**

   - create a real issue with `@gitclaw /backup snapshot`,
   - assert the issue-side report lists `requested_backup_command: snapshot`,
     the deferred execution marker, the concrete local snapshot command, and
     the deferred composite-lockfile gate without dumping body/title tokens,
   - assert the issue-visible report includes
     `llm_e2e_required_after_backup_snapshot_change: true`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup snapshot --root <fetched>/.gitclaw/backups --repo
     <owner/repo>`,
   - assert the report is marked `backup_snapshot_status: ok` and
     `backup_verify_status: ok`,
   - assert it lists control-file entries, issue-payload entries, aggregate
     counts, the composite `snapshot_sha256_12`, body-free gates, and the
     current issue payload path,
   - assert it does not dump the issue body token or raw title,
   - post a normal follow-up that requires repo-reader search and assert the
     second assistant turn is GitHub Models-backed with prompt context,
     selected skill, prompt-visible `gitclaw.search_files`, usage telemetry,
     and the backup-snapshot repository-search fixture token.

40. **Backup freshness**

   - create a real issue with `@gitclaw /backup freshness`,
   - assert the issue-side report lists `requested_backup_command: freshness`,
     the deferred execution marker, the concrete local freshness command, and
     the deferred `latest-backup-age <= max-age` gate without dumping
     body/title tokens,
   - assert the issue-visible report includes
     `llm_e2e_required_after_backup_freshness_change: true`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup freshness --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --max-age-hours 24`,
   - assert the report is marked `backup_freshness_status: ok`,
     `backup_verify_status: ok`, and `freshness_gate: pass`,
   - assert it lists the latest issue path, backup timestamp, age seconds,
     max-age seconds, payload hash, and title hash,
   - assert it does not dump the issue body token or raw title,
   - post a normal follow-up that requires repo-reader search and assert the
     second assistant turn is GitHub Models-backed with prompt context,
     selected skill, prompt-visible `gitclaw.search_files`, usage telemetry,
     and the backup-freshness repository-search fixture token.

41. **Backup continuity**

   - create a real issue with `@gitclaw /backup continuity`,
   - assert the issue-side report lists `requested_backup_command:
     continuity`, the deferred execution marker, the concrete local continuity
     command, and the deferred `longest-backup-gap <= max-gap` gate without
     dumping body/title tokens,
   - assert the issue-visible report includes
     `llm_e2e_required_after_backup_continuity_change: true`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup continuity --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --max-gap-hours 168`,
   - assert the report is marked `backup_continuity_status: ok`,
     `backup_verify_status: ok`, and `continuity_gate: pass`,
   - assert it lists scanned point count, longest gap seconds, first/latest
     issue timestamps, gap-threshold metadata, and hash-only gap cards,
   - assert it does not dump the issue body token or raw title,
   - post a normal follow-up that requires repo-reader search and assert the
     second assistant turn is GitHub Models-backed with prompt context,
     selected skill, prompt-visible `gitclaw.search_files`, usage telemetry,
     and the backup-continuity repository-search fixture token.

41. **Backup list**

   - create a real issue with `@gitclaw /backup list`,
   - assert the issue-side report lists `requested_backup_command: list`,
     the deferred execution marker, and the concrete local list command without
     dumping body/title tokens,
   - assert the issue-visible report includes
     `llm_e2e_required_after_backup_list_change: true`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup list --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --limit 5`,
   - assert the report is marked `backup_list_status: ok` and
     `backup_verify_status: ok`,
   - assert it lists the just-created issue number, canonical path, timestamp,
     event name, label/comment/transcript counts, and title hash,
   - assert it does not dump the issue body token or raw title,
   - post a normal follow-up that requires repo-reader search and assert the
     second assistant turn is GitHub Models-backed with prompt context,
     selected skill, prompt-visible `gitclaw.search_files`, usage telemetry,
     and the backup-list repository-search fixture token.

42. **Backup timeline**

   - create a real issue with `@gitclaw /backup timeline`,
   - assert the issue-side report lists `requested_backup_command: timeline`,
     the deferred execution marker, and the concrete local timeline command
     without dumping body/title tokens,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup timeline --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --limit 5`,
   - assert the report is marked `backup_timeline_status: ok` and
     `backup_verify_status: ok`,
   - assert it lists the just-created issue number, canonical path, timestamp,
     event name, gap seconds, counts, payload hash, and title hash,
   - assert it does not dump the issue body token or raw title,
   - post a normal follow-up that requires repo-reader search and assert the
     second assistant turn is GitHub Models-backed with prompt context,
     selected skill, and prompt-visible tool markers.

43. **Backup info**

   - create a real issue with `@gitclaw /backup info`,
   - assert the issue-side report lists `requested_backup_command: info`, the
     deferred execution marker, and the concrete local info command for the
     current issue without dumping body/title tokens,
   - assert the issue-visible report includes
     `llm_e2e_required_after_backup_info_change: true`,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup info --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --issue <issue-number>`,
   - assert the report is marked `backup_info_status: ok` and
     `backup_verify_status: ok`,
   - assert it lists the canonical payload path, payload hash, event name,
     label/comment/transcript counts, assistant-turn/error counts, and body
     hashes,
   - assert it does not dump the issue body token or raw title,
   - post a normal follow-up that requires repo-reader search and assert the
     second assistant turn is GitHub Models-backed with prompt context,
     selected skill, prompt-visible `gitclaw.search_files`, usage telemetry,
     and the backup-info repository-search fixture token.

44. **Backup JSONL export**

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
     explicit raw recovery/export path rather than an issue-visible report,
   - post a normal model-backed repo-reader/search follow-up and assert prompt
     provenance, selected skill metadata, prompt-visible `gitclaw.search_files`,
     usage telemetry, and no hidden issue/comment sentinel leakage.

45. **Backup restore plan**

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
   - assert it does not dump the issue body token or raw transcript bodies,
   - post a normal follow-up comment that requires repo-reader search and
     assert the next assistant turn used GitHub Models with prompt provenance,
     selected skills, prompt-visible tool names, and usage markers.

46. **Backup drill**

   - create a real issue with `@gitclaw /backup drill`,
   - assert the issue-side report lists `requested_backup_command: drill`, the
     deferred execution marker, and the concrete local drill command without
     dumping body/title tokens,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup drill --root <fetched>/.gitclaw/backups --repo
     <owner/repo> --issue <issue-number>`,
   - assert the report includes verify, coverage, and dry-run restore-plan
     gates, plus schema, path, count, assistant-turn/error, and hash metadata,
   - assert it does not dump issue/comment/transcript bodies,
   - post a normal follow-up comment that requires repo-reader search and
     assert the next assistant turn used GitHub Models with prompt provenance,
     selected skills, and prompt-visible tool names.

47. **Backup rehearsal issue**

   - create a real issue with `@gitclaw /backup rehearse --id <id>`,
   - assert the source receipt is generated without a model call, creates a
     labeled rehearsal issue, suppresses raw source text, and exposes only
     hashes for backup paths,
   - inspect the rehearsal issue body and assert it contains the stable
     marker, backup branch/root, expected issue backup path, dry-run gates, and
     no raw source body,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup coverage`, `gitclaw backup drill`, and
     `gitclaw backup restore-plan` for the source issue,
   - assert all three local reports are body-free and non-mutating,
   - post a duplicate `@gitclaw /backup rehearse --id <id>` comment and assert
     it reuses the same rehearsal issue,
   - post a normal follow-up comment on the rehearsal issue that requires
     repo-reader search and assert the next assistant turn used GitHub Models
     with prompt provenance, selected skills, prompt-visible tool names, and
     usage markers.

48. **Backup restore request issue**

   - create a real issue with `@gitclaw /backup restore-request --id <id>`,
   - assert the source receipt is generated without a model call, creates a
     labeled restore-request issue, suppresses raw source text, and exposes
     only hashes for backup paths,
   - inspect the restore-request issue body and assert it contains the stable
     marker, backup branch/root, expected issue backup path, approval gates,
     dry-run commands, and no raw source body,
   - wait for the successful backup job,
   - fetch the real `gitclaw-backups` branch,
   - run `gitclaw backup verify`, `gitclaw backup coverage`,
     `gitclaw backup drill`, `gitclaw backup restore-plan`, and
     `gitclaw backup manifest` for the source issue,
   - assert all local reports are body-free where applicable and non-mutating,
   - post a duplicate `@gitclaw /backup restore-request --id <id>` comment and
     assert it reuses the same restore-request issue,
   - post a normal follow-up comment on the restore-request issue that requires
     repo-reader search and assert the next assistant turn used GitHub Models
     with prompt provenance, selected skills, prompt-visible tool names, and
     usage markers.

49. **Backup restore request channel notification**

   - create a real issue with
     `@gitclaw /backup restore-request --id <id> --notify-route <route>`,
   - assert the source receipt is generated without a model call, opens or
     reuses the labeled restore-request issue, queues exactly one channel
     notification, exposes only hashes/counts for route, message, body, backup
     paths, and suppresses raw source text,
   - inspect the restore-request issue body and assert it contains the stable
     marker, backup branch/root, expected issue backup path, approval gates, and
     no raw source or route body,
   - inspect the channel issue and assert it carries `gitclaw:channel`, not the
     normal model trigger label, and contains one
     `gitclaw:channel-outbound` comment for the reviewed route,
   - run metadata-only `gitclaw channel-outbox` and assert the pending provider
     work is visible without leaking the outbound body,
   - post a duplicate restore-request notification command and assert it reuses
     the same restore-request issue and suppresses duplicate channel message
     ids,
   - post a normal follow-up comment on the restore-request issue that requires
     repo-reader search and assert the next assistant turn used GitHub Models
     with prompt provenance, selected skills, prompt-visible tool names, and
     usage markers.

50. **Backup retention plan**

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
     token or raw title,
   - post a normal follow-up comment that requires repo-reader search and
     assert the next assistant turn used GitHub Models with prompt provenance,
     selected skills, prompt-visible tool names, and usage markers.

48. **Backup search**

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
     raw comments, raw transcript messages, or raw query text,
   - post a normal model-backed repo-reader/search follow-up and assert prompt
     provenance, selected skill metadata, prompt-visible `gitclaw.search_files`,
     usage telemetry, and no hidden issue/comment sentinel leakage.

49. **Proactive init generator**

   - run `gitclaw proactive init` against a temporary repo root,
   - assert it writes the expected prompt file and scheduled workflow,
   - assert the init report includes hashes and file status but not the prompt
     body token,
   - lint the generated workflow when `actionlint` is available,
   - dispatch the real generic proactive workflow with the generated job name
     and a `/proactive` prompt body,
   - assert it creates a real proactive issue and receives the deterministic
     `gitclaw/proactive` report without leaking the hidden prompt token,
   - post a normal issue-comment follow-up that uses `repo-reader` and
     `gitclaw.search_files`, returns the bounded repository-search fixture
     token, includes model/prompt/usage telemetry, and does not leak hidden
     prompt/comment tokens.

50. **Proactive info report**

   - create a real issue with `@gitclaw /proactive info repo-hygiene`,
   - include a unique hidden token in the issue body,
   - wait for the issue-opened workflow,
   - assert the assistant posts exactly one `GitClaw Proactive Info Report`
     with `proactive_info_status: ok`,
   - assert the report lists prompt, generic workflow, generated workflow
     candidate, trigger metadata, and enqueue command hashes/paths,
   - assert no issue body, prompt body, or workflow body content is leaked.

51. **Proactive risk report with model follow-up**

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
  issue, proves same-dispatch-id idempotency, then continues the same issue
  with a normal GitHub Models repo-reader/search follow-up,
- the channel-message harness verifies a hidden `gitclaw:channel-message`
  comment is reconstructed as user input during a dispatched run and can drive
  repo-reader search with model/prompt/usage telemetry, then continues the same
  issue through a normal issue-comment follow-up that proves repo-reader search
  again,
- the channel-ingest harness verifies the generic bridge workflow mirrors a
  message into an issue, dispatches the main handler, suppresses duplicate
  provider-message retries, and then proves a normal model/tool follow-up on
  the canonical issue,
- the proactive enqueue harness verifies manual/scheduled job primitives can
  create their own work issues idempotently and drive repo-reader search
  through the model-backed main handler,
- the proactive-init harness verifies the generator writes ordinary repo files
  without leaking prompt bodies, dispatches a real proactive issue conversation,
  and then proves a normal model/tool follow-up on that generated job issue,
- the backup-manifest harness verifies a real backed-up issue has a compact
  file-level manifest with hashes and counts but no raw body leakage,
- the backup JSONL export harness verifies a real backed-up issue can be
  exported as one JSONL transcript record per reconstructed message,
- the backup restore-plan harness verifies a real backed-up issue can produce
  a non-mutating restore plan with counts and hashes but no raw body leakage,
- the backup restore-request harness verifies a restore approval lane opens a
  labeled GitHub issue, validates the real backup branch with local dry-run
  commands, suppresses duplicate restore requests, and then proves a normal
  model-backed repo-reader/search follow-up on that generated issue,
- the backup restore-request channel-notify harness verifies the same approval
  lane can queue a reviewed channel notification, expose it through
  metadata-only outbox, suppress duplicate notification message IDs, and then
  prove a normal model-backed repo-reader/search follow-up on that generated
  issue,
- the backup retention-plan harness verifies a fetched backup branch can
  produce a dry-run keep-latest plan with kept/prune-candidate paths and hashes
  but no raw title/body leakage,
- the backup-search harness verifies a fetched backup branch can search actual
  backed-up conversation content and return only paths, sources, trust
  metadata, scores, and hashes without leaking the searched token or bodies,
  then proves a normal model-backed repo-reader/search follow-up,
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
  `.gitclaw/HEARTBEAT.md`, exact token content, and same-slot idempotency. The
  same live issue then receives a normal `@gitclaw` follow-up that must make a
  GitHub Models call, select `repo-reader`, expose `gitclaw.search_files`, and
  recover a heartbeat follow-up repository-search fixture token.
- A `gh`-driven heartbeat-report E2E harness verifies `@gitclaw /heartbeat`
  reports workflow triggers, permissions, heartbeat context metadata, label
  state, marker counts, and the runner/model-call contract without a model call
  or body leakage. The same live issue then receives a normal GitHub Models
  follow-up that must select `repo-reader`, expose `gitclaw.search_files`,
  recover a bounded repository-search fixture token, and publish usage
  telemetry without leaking hidden issue tokens or `HEARTBEAT.md` contents.
- A `gh`-driven heartbeat-risk E2E harness verifies
  `@gitclaw /heartbeat risk` reports body-free workflow schedule, permission,
  idempotency, heartbeat context, and runtime-gate risk metadata, then runs a
  real GitHub Models follow-up conversation that proves repo-reader selection
  and prompt-visible repository search tool usage.
- A `gh`-driven workflow-dispatch E2E harness verifies the main handler can be
  woken for a specific issue and deduped by dispatch ID. The same live issue
  must then receive a normal issue-comment follow-up that makes a GitHub Models
  call, selects `repo-reader`, exposes `gitclaw.search_files`, recovers the
  workflow-dispatch repository-search fixture token, and avoids hidden
  follow-up sentinel leakage.
- A `gh`-driven channel-message E2E harness verifies a mirrored channel
  comment is included in the dispatched conversation transcript and can force a
  real GitHub Models repo-reader/search turn with prompt provenance and usage
  telemetry. The same harness then posts a normal issue-comment follow-up that
  must make another GitHub Models call, select `repo-reader`, expose
  `gitclaw.search_files`, recover a distinct channel-message follow-up fixture
  token, and avoid hidden channel/comment sentinels.
- A `gh`-driven channel-ingest E2E harness verifies the generic channel ingress
  workflow end to end, including duplicate provider-message retries. The same
  harness then posts a normal issue-comment follow-up that must make a GitHub
  Models call, select `repo-reader`, expose `gitclaw.search_files`, recover the
  channel-ingest fixture token, and avoid hidden channel/message sentinels.
- A `gh`-driven channel-state E2E harness verifies real GitHub issue-backed
  channel offset storage, duplicate offset suppression, `gitclaw:channel`
  labeling, and no raw account/offset leakage. The workflow harness then posts
  normal issue-comment follow-ups that must make GitHub Models calls, select
  `repo-reader`, expose `gitclaw.search_files`, recover distinct channel-state
  fixture tokens, and avoid hidden account/offset/comment sentinels.
- A `gh`-driven channel-state-workflow E2E harness dispatches
  `.github/workflows/gitclaw-channel-state.yml`, verifies the state issue and
  update comment, then dispatches the same offset again to prove retry
  idempotency in GitHub Actions.
- A `gh`-driven channel-gateway-workflow E2E harness dispatches
  `.github/workflows/gitclaw-channel-gateway.yml`, verifies the gateway lease is
  persisted through channel-state hashes, then repeats the same lease to prove
  duplicate gateway runs are idempotent. The same harness then posts normal
  issue-comment follow-ups that must make GitHub Models calls, select
  `repo-reader`, expose `gitclaw.search_files`, recover distinct
  channel-gateway fixture tokens, and avoid hidden account/lease sentinels.
- A `gh`-driven channel-send-workflow E2E harness dispatches
  `.github/workflows/gitclaw-channel-send.yml`, verifies a GitHub-originated
  `gitclaw:channel-outbound` message is queued on a canonical channel issue,
  repeats the dispatch to prove duplicate suppression, discovers the pending
  outbound message through `channel-outbox`, records delivery through
  `channel-delivery`, then dispatches outbox again to prove the receipt
  suppresses retries. The same harness posts a normal issue-comment follow-up
  that must make a GitHub Models call, select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-send fixture token, and avoid
  hidden account/provider/channel sentinels.
- A `gh`-driven channel-send-route-workflow E2E harness dispatches the same
  workflow with only `route`, `message_id`, and `body`, verifies
  `.gitclaw/channels/routes.yaml` resolution, duplicate suppression, and a
  metadata-only pending outbox entry, then posts a normal issue-comment
  follow-up that must make a GitHub Models call, select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-send-route fixture token, and
  avoid hidden route/account/channel sentinels.
- A `gh`-driven channel-send-slash E2E harness creates an ordinary GitHub issue
  with `@gitclaw /channels send --route ...`, verifies the routebook-backed
  outbound comment, body-free source receipt, duplicate suppression from a
  later issue comment with the same message id, and metadata-only outbox
  discovery. The same source issue then gets a normal GitHub Models
  issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-send-slash fixture token, and
  avoid hidden route/account/channel sentinels.
- A `gh`-driven channel-probe-slash E2E harness creates an ordinary GitHub
  issue with `@gitclaw /channels probe --route ...`, verifies the generated
  routebook-backed probe message, body-free source receipt, metadata-only
  outbox discovery, channel-delivery receipt recording, post-delivery outbox
  suppression, and duplicate probe suppression from a later issue comment with
  the same message id. The same source issue then gets a normal GitHub Models
  issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-probe fixture token, and avoid
  hidden route/account/provider/channel sentinels.
- A `gh`-driven channel-broadcast-slash E2E harness creates an ordinary GitHub
  issue with `@gitclaw /channels broadcast ...`, verifies one routebook-backed
  outbound comment per reviewed route, body-free source receipt metadata,
  duplicate broadcast suppression from a later issue comment with the same
  message id, and metadata-only outbox discovery for each target issue. The
  same source issue then gets a normal GitHub Models issue-comment follow-up
  that must select `repo-reader`, expose `gitclaw.search_files`, recover the
  channel-broadcast fixture token, and avoid hidden route/account/channel
  sentinels.
- A `gh`-driven channel-invite-slash E2E harness creates an ordinary GitHub
  issue with `@gitclaw /channels invite ...`, verifies routebook-backed issue
  invitations on each target channel issue, body-free source receipt metadata,
  duplicate invite suppression from a later issue comment with the same message
  id, and metadata-only outbox discovery for each target issue. The same source
  issue then gets a normal GitHub Models issue-comment follow-up that must
  select `repo-reader`, expose `gitclaw.search_files`, recover the
  channel-invite fixture token, and avoid hidden route/account/channel
  sentinels.
- A `gh`-driven channel-room-slash E2E harness creates an ordinary GitHub issue
  with `@gitclaw /channels room ...`, verifies a labeled durable GitHub room
  issue, routebook-backed room invitations on each target channel issue,
  body-free source receipt metadata, duplicate room suppression from a later
  issue comment with the same message id, and metadata-only outbox discovery
  for each target issue. The room issue then gets a normal GitHub Models
  issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-room fixture token, and avoid
  hidden route/account/channel sentinels.
- A `gh`-driven channel-huddle-slash E2E harness creates an ordinary GitHub
  issue with `@gitclaw /channels huddle ...`, verifies a labeled GitHub huddle
  issue, routebook-backed huddle invitations on each target channel issue,
  body-free source receipt metadata, duplicate huddle suppression from a later
  issue comment with the same message id, and metadata-only outbox discovery
  for each target issue. The huddle issue then gets a normal GitHub Models
  issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-huddle fixture token, and avoid
  hidden route/account/channel sentinels.
- A `gh`-driven channel-poll-slash E2E harness creates an ordinary GitHub issue
  with `@gitclaw /channels poll ...`, verifies a labeled GitHub poll issue,
  routebook-backed poll invitations on each target channel issue, body-free
  source receipt metadata, duplicate poll suppression from a later issue
  comment with the same message id, and metadata-only outbox discovery for each
  target issue. The poll issue then gets a normal GitHub Models issue-comment
  follow-up that must select `repo-reader`, expose `gitclaw.search_files`,
  recover the channel-poll fixture token, and avoid hidden route/account/channel
  sentinels.
- A `gh`-driven channel-rollcall-slash E2E harness creates an ordinary GitHub
  issue with `@gitclaw /channels rollcall ...`, verifies a labeled GitHub
  rollcall issue, routebook-backed check-in invitations on each target channel
  issue, body-free source receipt metadata, duplicate rollcall suppression from
  a later issue comment with the same message id, and metadata-only outbox
  discovery for each target issue. The rollcall issue then gets a normal GitHub
  Models issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-rollcall fixture token, and avoid
  hidden route/account/channel sentinels.
- A `gh`-driven channel-done-slash E2E harness creates a real channel task from
  a mirrored channel issue, comments `@gitclaw /channels done ...` on the task
  issue, verifies the task issue closes, queues a metadata-safe done
  acknowledgement back to the original channel thread, checks duplicate
  acknowledgement suppression and metadata-only outbox discovery, and then
  continues on the original channel issue with a normal GitHub Models
  issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-done fixture token, and avoid
  hidden task/thread/account/channel sentinels.
- A `gh`-driven channel-status-slash E2E harness creates a real channel-thread
  issue, posts `@gitclaw /channels status --message-id ... --status-id ...
  --state working` on that mirrored thread, verifies same-issue structured
  status queueing, body-free receipt metadata, duplicate suppression,
  metadata-only outbox discovery with kind `channel-status`, and
  channel-delivery workflow receipts. The same issue then gets a normal GitHub
  Models issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-status fixture token, and avoid
  hidden channel, account, provider, status, and body sentinels.
- A `gh`-driven channel-edit-slash E2E harness creates a real channel-thread
  issue, posts `@gitclaw /channels edit --message-id ... --edit-id ...` on
  that mirrored thread, verifies same-issue structured edit queueing,
  body-free receipt metadata, duplicate suppression, metadata-only outbox
  discovery with kind `channel-edit`, and channel-delivery workflow receipts.
  The same issue then gets a normal GitHub Models issue-comment follow-up that
  must select `repo-reader`, expose `gitclaw.search_files`, recover the
  channel-edit fixture token, and avoid hidden channel, account, provider,
  edit, target-message, and replacement-body sentinels.
- A `gh`-driven channel-reaction-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels react --message-id ... --reaction ...` on that mirrored
  thread, verifies same-issue structured reaction queueing, body-free receipt
  metadata, duplicate suppression, metadata-only outbox discovery with kind
  `channel-reaction`, and channel-delivery workflow receipts. The same issue
  then gets a normal GitHub Models issue-comment follow-up that must select
  `repo-reader`, expose `gitclaw.search_files`, recover the channel-reaction
  fixture token, and avoid hidden channel, account, provider, and reaction
  sentinels.
- A `gh`-driven channel-pin-slash E2E harness creates a real channel-thread
  issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels pin --message-id ...`, verifies the default `pushpin`
  reaction queue, body-free receipt metadata, duplicate suppression,
  metadata-only outbox discovery with kind `channel-reaction`, and
  channel-delivery receipts. The same issue then gets a normal GitHub Models
  issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-pin fixture token, and avoid
  hidden channel, account, provider, and reaction sentinels.
- A `gh`-driven channel-reply-slash E2E harness creates a real channel-thread
  issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels reply --message-id ...` on that mirrored thread, verifies
  same-issue outbound queueing, body-free receipt metadata, duplicate
  suppression, metadata-only outbox discovery, and channel-delivery workflow
  receipts. The same issue then gets a normal GitHub Models issue-comment
  follow-up that must select `repo-reader`, expose `gitclaw.search_files`,
  recover the channel-reply-slash fixture token, and avoid hidden channel,
  account, provider, and outbound-body sentinels.
- A `gh`-driven channel-deliverable-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels deliverable --deliverable-id ... --message-id ...
  --filename ... --url ...` on that mirrored thread, verifies
  `gitclaw:channel-deliverable` queueing, body-free source receipt metadata,
  metadata-only outbox leak boundaries, include-body outbox exposure for the
  provider gateway payload, channel-delivery workflow receipts, and duplicate
  deliverable suppression. The same issue then gets a normal GitHub Models
  issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-deliverable fixture token, and
  avoid hidden channel, account, provider, URL, caption, filename, and
  deliverable sentinels.
- A `gh`-driven channel-task-slash E2E harness creates a real channel-thread
  issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels task --task-id ... --message-id ...` on that mirrored
  thread, verifies GitHub task issue creation, body-free source receipt
  metadata, provider-facing task-link queueing, duplicate task and notification
  suppression, and metadata-only outbox discovery. The task issue then gets a
  normal GitHub Models issue-comment follow-up that must select `repo-reader`,
  expose `gitclaw.search_files`, recover the channel-task fixture token, and
  avoid hidden channel, account, provider, message, and task sentinels.
- A `gh`-driven channel-clip-slash E2E harness creates a real channel-thread
  issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels clip --clip-id ... --message-id ...` on that mirrored
  thread, verifies GitHub clip issue creation, body-free source receipt
  metadata, provider-facing clip-link queueing, duplicate clip and notification
  suppression, and metadata-only outbox discovery. The clip issue then gets a
  normal GitHub Models issue-comment follow-up that must select `repo-reader`,
  expose `gitclaw.search_files`, recover the channel-clip fixture token, and
  avoid hidden channel, account, provider, message, and clip sentinels.
- A `gh`-driven channel-attachment-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels attachment --attachment-id ... --message-id ...
  --filename ...` on that mirrored thread, verifies GitHub attachment metadata
  issue creation, body-free source receipt metadata, provider-facing
  attachment-link queueing, duplicate attachment and notification suppression,
  source URL/file-byte leak boundaries, and metadata-only outbox discovery. The
  attachment issue then gets a normal GitHub Models issue-comment follow-up
  that must select `repo-reader`, expose `gitclaw.search_files`, recover the
  channel-attachment fixture token, and avoid hidden channel, account,
  provider, message, filename, caption, source URL, and attachment sentinels.
- A `gh`-driven channel-decision-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels decision --decision-id ... --message-id ...` on that
  mirrored thread, verifies GitHub decision issue creation, body-free source
  receipt metadata, provider-facing decision-link queueing, duplicate decision
  and notification suppression, and metadata-only outbox discovery. The
  decision issue then gets a normal GitHub Models issue-comment follow-up that
  must select `repo-reader`, expose `gitclaw.search_files`, recover the
  channel-decision fixture token, and avoid hidden channel, account, provider,
  message, and decision sentinels.
- A `gh`-driven channel-digest-slash E2E harness creates a real channel-thread
  issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels digest --digest-id ... --message-id ...` on that mirrored
  thread, verifies GitHub digest issue creation, body-free source receipt
  metadata, provider-facing digest-link queueing, duplicate digest and
  notification suppression, and metadata-only outbox discovery. The digest
  issue then gets a normal GitHub Models issue-comment follow-up that must
  select `repo-reader`, expose `gitclaw.search_files`, recover the
  channel-digest fixture token, and avoid hidden channel, account, provider,
  message, and digest sentinels.
- A `gh`-driven channel-tool-run-request-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels request-run search_files --id ... --message-id ...` on
  that mirrored thread, verifies GitHub reviewed tool-run request issue
  creation, body-free source receipt metadata, provider-facing review-link
  queueing, duplicate request and notification suppression, and metadata-only
  outbox discovery. The review issue then gets a normal GitHub Models
  issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-tool-request fixture token, and
  avoid hidden channel, account, provider, message, request, and tool
  sentinels.
- A `gh`-driven channel-tool-approval-plan-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels approval-plan search_files --id ... --message-id ...`
  on that mirrored thread, verifies GitHub tool approval-plan issue creation,
  body-free source receipt metadata, provider-facing approval-link queueing,
  duplicate approval-plan and notification suppression, and metadata-only
  outbox discovery. The approval issue then gets a normal GitHub Models
  issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-tool-approval-plan fixture
  token, and avoid hidden channel, account, provider, message, approval-plan,
  and tool sentinels.
- A `gh`-driven channel-tool-rehearsal-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels rehearse-tool search_files --id ... --message-id ...` on
  that mirrored thread, verifies GitHub tool rehearsal issue creation,
  body-free source receipt metadata, provider-facing rehearsal-link queueing,
  duplicate rehearsal and notification suppression, and metadata-only outbox
  discovery. The rehearsal issue then gets a normal GitHub Models
  issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-tool-rehearsal fixture token,
  and avoid hidden channel, account, provider, message, rehearsal, and tool
  sentinels.
- A `gh`-driven channel-skill-proposal-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels propose-skill weekly-review --message-id ...` on that
  mirrored thread, verifies GitHub skill proposal issue creation, body-free
  source receipt metadata, provider-facing proposal-link queueing, duplicate
  proposal and notification suppression, and metadata-only outbox discovery.
  The proposal issue then gets a normal GitHub Models issue-comment follow-up
  that must select `repo-reader`, expose `gitclaw.search_files`, recover the
  channel-skill-proposal fixture token, and avoid hidden channel, account,
  provider, message, proposal, and skill sentinels.
- A `gh`-driven channel-skill-rehearsal-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels rehearse-skill repo-reader --id ... --message-id ...` on
  that mirrored thread, verifies GitHub skill rehearsal issue creation,
  body-free source receipt metadata, provider-facing rehearsal-link queueing,
  duplicate rehearsal and notification suppression, and metadata-only outbox
  discovery. The rehearsal issue then gets a normal GitHub Models issue-comment
  follow-up that must select `repo-reader`, expose `gitclaw.search_files`,
  recover the channel-skill-rehearsal fixture token, and avoid hidden channel,
  account, provider, message, rehearsal, and skill sentinels.
- A `gh`-driven channel-soul-proposal-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels propose-soul --target soul --id ... --message-id ...` on
  that mirrored thread, verifies GitHub soul proposal issue creation,
  body-free source receipt metadata, provider-facing proposal-link queueing,
  duplicate proposal and notification suppression, and metadata-only outbox
  discovery. The proposal issue then gets a normal GitHub Models issue-comment
  follow-up that must select `repo-reader`, expose `gitclaw.search_files`,
  recover the channel-soul-proposal fixture token, and avoid hidden channel,
  account, provider, message, proposal, target-body, and candidate-soul
  sentinels.
- A `gh`-driven channel-soul-rehearsal-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels rehearse-soul --target soul --id ... --message-id ...`
  on that mirrored thread, verifies GitHub soul rehearsal issue creation,
  body-free source receipt metadata, provider-facing rehearsal-link queueing,
  duplicate rehearsal and notification suppression, and metadata-only outbox
  discovery. The rehearsal issue then gets a normal GitHub Models issue-comment
  follow-up that must select `repo-reader`, expose `gitclaw.search_files`,
  recover the channel-soul-rehearsal fixture token, and avoid hidden channel,
  account, provider, message, rehearsal, target-body, and candidate-soul
  sentinels.
- A `gh`-driven channel-memory-rehearsal-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels rehearse-memory --target long-term --id ... --message-id
  ...` on that mirrored thread, verifies GitHub memory rehearsal issue
  creation, body-free source receipt metadata, provider-facing rehearsal-link
  queueing, duplicate rehearsal and notification suppression, and
  metadata-only outbox discovery. The rehearsal issue then gets a normal
  GitHub Models issue-comment follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the channel-memory-rehearsal fixture token,
  and avoid hidden channel, account, provider, message, rehearsal,
  target-memory-body, and candidate-memory sentinels.
- A `gh`-driven channel-backup-rehearsal-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels rehearse-backup --id ... --message-id ...` on that
  mirrored thread, verifies GitHub backup rehearsal issue creation, body-free
  source receipt metadata, provider-facing rehearsal-link queueing, duplicate
  rehearsal and notification suppression, backup-branch capture of the source
  issue, dry-run coverage/drill/restore-plan reports, and metadata-only outbox
  discovery. The rehearsal issue then gets a normal GitHub Models issue-comment
  follow-up that must select `repo-reader`, expose `gitclaw.search_files`,
  recover the channel-backup-rehearsal fixture token, and avoid hidden channel,
  account, provider, message, rehearsal, backup-path, source-body, and backup
  sentinels.
- A `gh`-driven channel-reminder-slash E2E harness creates a real
  channel-thread issue through `gitclaw-channel-ingest.yml`, posts
  `@gitclaw /channels remind --reminder-id ... --message-id ... --at ...` on
  that mirrored thread, verifies GitHub reminder issue creation with a
  normalized `not_before` gate, body-free source receipt metadata,
  provider-facing reminder-link queueing, duplicate reminder and notification
  suppression, and metadata-only outbox discovery. The reminder issue then gets
  a normal GitHub Models issue-comment follow-up that must select
  `repo-reader`, expose `gitclaw.search_files`, recover the channel-reminder
  fixture token, and avoid hidden channel, account, provider, message,
  reminder, and due-time sentinels.
- A `gh`-driven channel-delivery-workflow E2E harness dispatches
  `.github/workflows/gitclaw-channel-delivery.yml`, verifies a source
  `gitclaw:assistant-turn` comment can be recorded as delivered, checks that
  only hashes are stored for channel account/provider message IDs, and repeats
  the same delivery to prove outbound idempotency. The same harness then posts
  normal issue-comment follow-ups that must make GitHub Models calls, select
  `repo-reader`, expose `gitclaw.search_files`, recover distinct
  channel-delivery fixture tokens, and avoid hidden source/provider sentinels.
- A `gh`-driven channel-outbox-workflow E2E harness dispatches
  `.github/workflows/gitclaw-channel-outbox.yml`, verifies a real
  channel-ingested GitHub Models/tool reply appears as one pending outbound
  assistant comment, records delivery through
  `.github/workflows/gitclaw-channel-delivery.yml`, then dispatches outbox again
  to prove the receipt suppresses retries. The same harness posts a normal
  issue-comment follow-up that must make another GitHub Models call, select
  `repo-reader`, expose `gitclaw.search_files`, recover the channel-outbox
  follow-up fixture token, and avoid hidden account/provider/channel sentinels.
- A `gh`-driven channels-report E2E harness verifies `@gitclaw /channels`
  reports workflow dispatch, channel labels, provider keys, mirrored message
  marker counts, and `llm_e2e_required_after_channel_report_change: true`
  without a model call or hidden-token leakage. The same live harness then
  posts a normal issue-comment follow-up that must make a GitHub Models call,
  select `repo-reader`, expose `gitclaw.search_files`, recover a bounded
  repository-search fixture token, and publish usage telemetry without leaking
  hidden issue/message tokens.
- A `gh`-driven channels-list E2E harness verifies `@gitclaw /channels list`
  is an explicit report alias, while local `gitclaw channels list` exposes the
  same bridge contract without issue-only fields. The live harness then posts a
  normal issue-comment follow-up that must make a GitHub Models call, select
  `repo-reader`, expose `gitclaw.search_files`, recover the channels-list
  repository-search fixture token, and publish usage telemetry without leaking
  hidden issue tokens.
- A `gh`-driven channels-verify E2E harness verifies
  `@gitclaw /channels verify` reports the workflow-dispatch channel bridge
  health, permissions, required inputs, provider keys, and marker counts
  without a model call or body leakage. The same live harness then posts a
  normal issue-comment follow-up that must make a GitHub Models call, select
  `repo-reader`, expose `gitclaw.search_files`, recover a bounded
  repository-search fixture token, and publish usage telemetry without leaking
  hidden issue tokens.
- A `gh`-driven channels-risk E2E harness verifies
  `@gitclaw /channels risk` reports provider, workflow, and channel-message
  risk cards with only counts, hashes, codes, and severities, then posts a
  normal follow-up that requires repo-reader search so GitHub Models performs a
  real LLM call with prompt context and prompt-visible tool provenance.
- A `gh`-driven channels-info E2E harness verifies
  `@gitclaw /channels info <provider>` and local
  `gitclaw channels info <provider>` expose one provider's secret names,
  offset/thread/message keys, workflow metadata, gateway strategy, and command
  shapes, including `llm_e2e_required_after_channel_info_change: true`,
  without a model call or body/credential leakage. The same live harness then
  posts a normal issue-comment follow-up that must make a GitHub Models call,
  select `repo-reader`, expose `gitclaw.search_files`, recover a bounded
  repository-search fixture token, and publish usage telemetry without leaking
  hidden issue tokens.
- A `gh`-driven proactive E2E harness verifies the generic proactive enqueue
  workflow end to end, including duplicate-slot idempotency and a normal
  issue-comment GitHub Models follow-up that selects `repo-reader`, exposes
  `gitclaw.search_files`, recovers a distinct proactive follow-up fixture
  token, and avoids hidden prompt-token leakage.
- A `gh`-driven proactive-not-before E2E harness verifies future reminders
  write `due=false`, `skipped=true`, `issue_number=0`, and
  `llm_e2e_required_after_proactive_not_before_change=true` without creating an
  issue, then verifies a due run creates a proactive issue, posts the
  deterministic proactive report, and continues with a normal GitHub Models
  follow-up that selects `repo-reader`, exposes `gitclaw.search_files`,
  recovers a bounded repository-search fixture token, and publishes usage
  telemetry without leaking hidden reminder tokens.
- A `gh`-driven proactive-init E2E harness verifies
  `gitclaw proactive init` generates a scheduled workflow and prompt file
  without leaking prompt bodies and includes
  `llm_e2e_required_after_proactive_init_change: true`; it then dispatches a
  real proactive conversation and posts a normal GitHub Models follow-up that
  must select `repo-reader`, expose `gitclaw.search_files`, recover a bounded
  repository-search fixture token, and publish usage telemetry without leaking
  hidden prompt or comment tokens.
- A `gh`-driven proactive-report E2E harness verifies `@gitclaw /proactive`
  reports workflow triggers, prompt metadata, and
  `llm_e2e_required_after_proactive_report_change: true` without a model call;
  the same live issue then receives a normal GitHub Models follow-up that must
  select `repo-reader`, expose `gitclaw.search_files`, recover a bounded
  repository-search fixture token, and publish usage telemetry.
- A `gh`-driven proactive-list E2E harness verifies `@gitclaw /proactive list`
  is an explicit report alias, while local `gitclaw proactive list` exposes
  workflow and prompt-file metadata without issue-only fields or prompt bodies.
  The live issue form also proves
  `llm_e2e_required_after_proactive_list_change: true` and the same
  model/tool follow-up contract as the root proactive report.
- A `gh`-driven proactive-schedule E2E harness verifies
  `@gitclaw /proactive schedule` and local `gitclaw proactive schedule` expose
  the reviewed GitHub Actions cron calendar, dispatch/not-before support,
  prompt skill hints, heartbeat-vs-cron timing boundary, and
  `llm_e2e_required_after_proactive_schedule_change: true` without a model call
  or body leakage. The same live harness then posts a normal issue-comment
  follow-up that must make a GitHub Models call, select `repo-reader`, expose
  `gitclaw.search_files`, recover a bounded repository-search fixture token,
  and publish usage telemetry without leaking hidden issue tokens.
- A `gh`-driven proactive-chain E2E harness verifies
  `@gitclaw /proactive chain` and local `gitclaw proactive chain` expose
  reviewed prompt dependency metadata for
  `gitclaw:proactive-context-from`, including edge counts, resolved-source
  names, missing-source hashes, self-reference/cycle gates, and
  `llm_e2e_required_after_proactive_chain_change: true` without a model call or
  body leakage. The same live harness then posts a normal issue-comment
  follow-up that must make a GitHub Models call, select `repo-reader`, expose
  `gitclaw.search_files`, recover a bounded repository-search fixture token,
  and publish usage telemetry without leaking hidden issue tokens.
- A `gh`-driven proactive-info E2E harness verifies
  `@gitclaw /proactive info <name>` and local `gitclaw proactive info <name>`
  expose one proactive job definition, generic workflow metadata, generated
  workflow candidate metadata, enqueue command shape, and
  `llm_e2e_required_after_proactive_info_change: true` without a model call or
  body leakage. The same live harness then posts a normal issue-comment
  follow-up that must make a GitHub Models call, select `repo-reader`, expose
  `gitclaw.search_files`, recover a bounded repository-search fixture token,
  and publish usage telemetry without leaking hidden issue tokens.
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
- A `gh`-driven model-catalog E2E harness verifies
  `@gitclaw /models catalog` and local `gitclaw models catalog` expose the
  reviewed GitHub Models catalog/default snapshot, `openai/gpt-5-nano`
  default candidate, fallback catalog membership, `openai/gpt-5.4-mini`
  absence, no-live-probe gates, and raw-response/body exclusions without a
  model call, then runs a real GitHub Models follow-up proving repo-reader
  search and normalized usage-marker persistence on the model-backed turn.
- A `gh`-driven model-usage E2E harness verifies `@gitclaw /models usage` and
  local `gitclaw models usage` expose normalized token telemetry, prompt
  projection, raw-payload exclusion, and cost-estimation gaps without a model
  call, then runs a real GitHub Models follow-up that proves repo-reader search
  and normalized usage-marker persistence on the model-backed turn.
- A `gh`-driven model-cost E2E harness first runs a real GitHub Models
  repo-reader/tool turn, then verifies `@gitclaw /models cost` and local
  `gitclaw models cost` convert recorded usage only through the reviewed
  GitHub Models multiplier snapshot, refuse unknown default-model dollar
  estimates, exclude raw bodies/payloads, and finally run another real
  GitHub Models follow-up conversation.
- A `gh`-driven config-report E2E harness verifies `@gitclaw /config` reports
  effective labels, prompt budgets, commands, and workflow metadata without a
  model call.
- A `gh`-driven config-list E2E harness verifies `@gitclaw /config list` is an
  explicit report alias, while local `gitclaw config list` exposes the same
  control-plane metadata without issue-only fields.
- A `gh`-driven commands-report E2E harness verifies `@gitclaw /help` reports
  deterministic commands, aliases, and every advertised local CLI helper
  without a model call or issue-body leakage. The same live issue then receives
  a normal issue-comment follow-up that must make a GitHub Models call, select
  `repo-reader`, expose `gitclaw.search_files`, recover the commands-report
  repository-search fixture token, and avoid hidden sentinel leakage.
- A `gh`-driven orders-report E2E harness verifies `@gitclaw /orders`
  reports standing-order file metadata, model-context loading, program clause
  coverage, proactive enforcement metadata, and body-free findings without a
  model call or standing-order body leakage. Each standing-orders feature batch
  must still run a live GitHub Models conversation E2E.
- A `gh`-driven orders-risk E2E harness verifies `@gitclaw /orders risk` and
  local `gitclaw orders risk` expose body-free durable-authority risk metadata,
  then runs a real GitHub Models follow-up conversation that proves model
  inference, prompt provenance, selected skills, and prompt-visible tool usage.
- A `gh`-driven hooks-report E2E harness verifies `@gitclaw /hooks` reports
  hook policy metadata, model-context loading, declarative hook spec metadata,
  approval/audit-only gates, ignored executable handler state, and body-free
  findings without a model call or hook body leakage. Each hooks feature batch
  must still run a live GitHub Models conversation E2E.
- A `gh`-driven hooks-catalog E2E harness verifies `@gitclaw /hooks catalog`
  and local `gitclaw hooks catalog` expose hook command and layer metadata,
  policy/spec/event/approval/provenance gates, disabled handler execution,
  provider-payload non-ingest, and no hook/provider/issue/comment body leakage.
  It then posts a real GitHub Models follow-up proving prompt provenance,
  selected skill, prompt-visible repository search tool usage, and usage
  telemetry.
- A `gh`-driven hooks-risk E2E harness verifies `@gitclaw /hooks risk` and
  local `gitclaw hooks risk` expose body-free hook risk metadata, then runs a
  real GitHub Models follow-up conversation that proves model inference,
  prompt provenance, selected skills, and prompt-visible tool usage.
- A `gh`-driven hooks-provenance E2E harness verifies
  `@gitclaw /hooks provenance` and local `gitclaw hooks provenance` expose
  body-free hook git history, hook/risk status, approval/audit-only metadata,
  commit-subject hashes, execution/mutation gates, and no hook/handler/issue
  body leakage. It then posts a normal follow-up comment that requires
  repo-reader search so GitHub Models performs a real LLM call with prompt
  context, selected skill, and prompt-visible tool provenance.
- A `gh`-driven plugins-report E2E harness verifies `@gitclaw /plugins`
  reports plugin policy metadata, model-context loading, declarative plugin
  spec metadata, metadata-only activation, approval gates, ignored package or
  runtime file state, MCP/plugin execution boundaries, and body-free findings
  without a model call or plugin body leakage. Each plugins feature batch must
  still run a live GitHub Models conversation E2E.
- A `gh`-driven plugins-risk E2E harness verifies `@gitclaw /plugins risk`
  and local `gitclaw plugins risk` expose body-free plugin policy/spec/package
  risk metadata, then runs a real GitHub Models follow-up conversation that
  proves model inference, prompt provenance, selected skills, and
  prompt-visible tool usage.
- A `gh`-driven plugins-MCP E2E harness verifies
  `@gitclaw /plugins mcp risk` and local `gitclaw plugins mcp risk` expose
  body-free MCP spec metadata, no-launch/no-connect runtime gates, tool filters,
  secret-name refs, risk counts, and hashes, then runs a real GitHub Models
  follow-up conversation that proves model inference, prompt provenance,
  selected skills, and prompt-visible tool usage.
- A `gh`-driven plugins-MCP-provenance E2E harness verifies
  `@gitclaw /plugins mcp provenance` maps repo-local MCP specs to body-free git
  history without launching or connecting MCP servers, then runs a real GitHub
  Models follow-up conversation that proves model inference, prompt provenance,
  selected skills, and prompt-visible tool usage.
- A `gh`-driven tasks-report E2E harness verifies `@gitclaw /tasks` reports
  task policy metadata, model-context loading, declarative task/flow spec
  metadata, issue-native status/label mapping, current issue task status,
  comment/transcript counts, Task Flow/Kanban execution boundaries, and
  body-free findings without a model call or task body leakage. Each tasks
  feature batch must still run a live GitHub Models conversation E2E.
- A `gh`-driven tasks-risk E2E harness verifies `@gitclaw /tasks risk` and
  local `gitclaw tasks risk` expose body-free task policy/spec/thread risk
  metadata, then runs a real GitHub Models follow-up conversation that proves
  model inference, prompt provenance, selected skills, and prompt-visible tool
  usage.
- A `gh`-driven tasks-ledger E2E harness verifies `@gitclaw /tasks ledger`
  exposes the body-free issue-native task ledger, current status, comment and
  transcript counts, assistant marker counts, hash-only entries, and raw-body
  gates. The same harness then posts a normal follow-up comment that requires
  repo-reader search so GitHub Models performs a real LLM call with prompt
  context, selected skill, and prompt-visible tool provenance.
- A `gh`-driven agents-risk E2E harness verifies `@gitclaw /agents risk` and
  local `gitclaw agents risk` expose body-free agent policy/spec/request risk
  metadata, then runs a real GitHub Models follow-up conversation that proves
  model inference, prompt provenance, selected skills, and prompt-visible tool
  usage.
- A `gh`-driven agents-catalog E2E harness verifies
  `@gitclaw /agents catalog` and local `gitclaw agents catalog` expose
  body-free agent command, policy/spec, runtime, conversation, tool-intent,
  approval, and no-delegation gate metadata, then runs a real GitHub Models
  follow-up conversation that proves model inference, prompt provenance,
  selected skills, prompt-visible tool usage, usage telemetry, and recovery of
  the bounded agents-catalog repository-search fixture token.
- A `gh`-driven agents-provenance E2E harness verifies
  `@gitclaw /agents provenance` and local `gitclaw agents provenance` expose
  body-free repo-local git provenance for agent policy/spec files, including
  tracked state, dirty state, commit availability, risk metadata, validation
  counts, no-delegation gates, and raw-body gates, then runs a real GitHub
  Models follow-up conversation that proves model inference, prompt
  provenance, selected skills, prompt-visible tool usage, usage telemetry, and
  recovery of the bounded agents-provenance repository-search fixture token.
- A `gh`-driven nodes-risk E2E harness verifies `@gitclaw /nodes risk` and
  local `gitclaw nodes risk` expose body-free node policy/spec/request risk
  metadata, then runs a real GitHub Models follow-up conversation that proves
  model inference, prompt provenance, selected skills, and prompt-visible tool
  usage.
- A `gh`-driven nodes-catalog E2E harness verifies
  `@gitclaw /nodes catalog` and local `gitclaw nodes catalog` expose body-free
  node command, policy/spec, runtime, wake-path, conversation, capability,
  approval, and no-remote-exec gate metadata, then runs a real GitHub Models
  follow-up conversation that proves model inference, prompt provenance,
  selected skills, prompt-visible tool usage, usage telemetry, and recovery of
  the bounded nodes-catalog repository-search fixture token.
- A `gh`-driven artifacts-risk E2E harness verifies
  `@gitclaw /artifacts risk` and local `gitclaw artifacts risk` expose
  body-free artifact policy/spec/workflow risk metadata, then runs a real
  GitHub Models follow-up conversation that proves model inference, prompt
  provenance, selected skills, and prompt-visible tool usage.
- A `gh`-driven artifacts-catalog E2E harness verifies
  `@gitclaw /artifacts catalog` and local `gitclaw artifacts catalog` expose
  body-free artifact command, policy/spec, upload-workflow, storage, redaction,
  retention, durable-backup, and no-hidden-state gate metadata, then runs a real
  GitHub Models follow-up conversation that proves model inference, prompt
  provenance, selected skills, prompt-visible tool usage, usage telemetry, and
  recovery of the bounded artifacts-catalog repository-search fixture token.
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
  leakage. The same live issue then receives a normal issue-comment follow-up
  that must make a GitHub Models call, select `repo-reader`, expose
  `gitclaw.search_files`, recover the runs-report repository-search fixture
  token, and avoid hidden sentinel leakage.
- A `gh`-driven runs-history E2E harness first creates a real GitHub Models
  issue conversation with repo-reader and `gitclaw.search_files`, then posts
  `@gitclaw /runs history` and verifies a body-free timeline of the prior
  model-backed assistant marker. It then posts a second normal follow-up so the
  same harness proves the report against actual conversation history and fresh
  LLM/tool usage.
- A `gh`-driven doctor-report E2E harness verifies `@gitclaw /doctor` reports
  config validation, workflow presence, context files, skills, memory notes,
  E2E harness inventory, proactive prompts, and skill/soul/memory/tool
  validation rollups without a model call. The same harness then posts a normal
  follow-up that requires repo-reader search so the batch proves a real GitHub
  Models turn, prompt provenance, selected skill names, and prompt-visible tool
  names.
- A `gh`-driven doctor-list E2E harness verifies `@gitclaw /doctor list` is an
  explicit report alias, while local `gitclaw doctor list` exposes the same
  body-free health and E2E-harness metadata without issue-only fields.
- A `gh`-driven toolsets-provenance E2E harness verifies
  `@gitclaw /tools toolsets provenance` maps repo-local toolset YAML to
  body-free git history without a model call. The same harness then posts a
  normal repo-reader follow-up that must make a real GitHub Models call and
  prove prompt provenance, selected skill names, and prompt-visible tool names.
- A `gh`-driven toolsets-info E2E harness verifies
  `@gitclaw /tools toolsets info repo-read` and local
  `gitclaw tools toolsets info repo-read` expose one repo-reviewed toolset
  profile, activation/mutation/network gates, resolved tool refs, risk codes,
  and `llm_e2e_required_after_toolset_info_change=true` without a model call
  or body leakage. The same live issue then receives a normal repo-reader
  follow-up that must make a GitHub Models call, expose `gitclaw.search_files`,
  recover a bounded repository-search fixture token, and publish usage
  telemetry.
- A `gh`-driven backup-index E2E harness verifies the dedicated backup branch
  contains issue JSON plus a repo-scoped `index.json` and `README.md` after a
  deterministic context turn, without leaking hidden issue-body tokens. The
  same harness posts a normal model-backed follow-up that proves repo-reader
  search, prompt provenance, selected skill metadata, prompt-visible tool
  names, usage markers, and the bounded backup-index repository-search fixture
  token.
- A `gh`-driven backup-report E2E harness verifies `@gitclaw /backup`
  publishes deterministic backup paths and
  `llm_e2e_required_after_backup_report_change: true` without a model call,
  then proves the backup branch receives the corresponding issue JSON and index
  entry without hidden issue-token leakage. The same harness posts a normal
  model-backed follow-up that proves repo-reader search, prompt provenance,
  selected skill metadata, prompt-visible tool names, usage markers, and the
  bounded backup-report repository-search fixture token.
- A `gh`-driven backup-catalog E2E harness verifies
  `@gitclaw /backup catalog` publishes a body-free command/gate catalog with
  `llm_e2e_required_after_backup_catalog_change: true`, checks the post-turn
  backup branch for the same issue, runs local `gitclaw backup catalog`, and
  then posts a normal GitHub Models repo-reader/search follow-up that recovers
  the bounded backup-catalog repository-search fixture token.
- A `gh`-driven backup-verify E2E harness verifies `@gitclaw /backup verify`
  records the deferred issue-side command intent, then verifies the fetched
  `gitclaw-backups` branch with `gitclaw backup verify` after the real backup
  job succeeds. The issue-side intent and fetched-branch verifier both include
  `llm_e2e_required_after_backup_verify_change: true`, and the same harness
  then posts a normal issue-comment follow-up that must make a GitHub Models
  call, select `repo-reader`, expose `gitclaw.search_files`, recover a bounded
  repository-search fixture token, and publish usage telemetry without leaking
  hidden issue tokens.
- A `gh`-driven backup-risk E2E harness verifies `@gitclaw /backup risk`
  records the deferred issue-side command intent, then scans the fetched
  `gitclaw-backups` branch with `gitclaw backup risk` for integrity,
  path-safety, credential-handling, prompt-boundary, restore-safety, and
  retention findings without dumping raw payloads. It also posts a normal
  follow-up that must use GitHub Models and repo-reader search.
- A `gh`-driven backup-provenance E2E harness verifies
  `@gitclaw /backup provenance` records the deferred issue-side command
  intent, then audits the fetched `gitclaw-backups` branch with
  `gitclaw backup provenance` for verified backup files that are tracked,
  clean, and committed without dumping raw backup bodies, commit subjects, or
  author identities. It also posts a normal follow-up that must use GitHub
  Models and repo-reader search.
- A `gh`-driven backup-manifest E2E harness verifies
  `@gitclaw /backup manifest` records the deferred issue-side command intent,
  then verifies the fetched `gitclaw-backups` branch can produce a file-level
  manifest with control-file and issue-payload hashes for one real issue,
  without dumping raw bodies. The same harness posts a normal model-backed
  follow-up that proves repo-reader search, prompt provenance, selected skill
  metadata, prompt-visible tool names, usage markers, and the bounded
  backup-manifest repository-search fixture token.
- A `gh`-driven backup-stats E2E harness verifies
  `@gitclaw /backup stats` records the deferred issue-side command intent,
  then verifies the fetched `gitclaw-backups` branch can produce a repo-wide
  backup stats report with verification status, aggregate counts, latest backup
  metadata, and event counts, without dumping raw bodies or titles. The same
  harness posts a normal model-backed follow-up that proves repo-reader search,
  prompt provenance, selected skill metadata, prompt-visible tool names, usage
  markers, and the bounded backup-stats repository-search fixture token.
- A `gh`-driven backup-snapshot E2E harness verifies
  `@gitclaw /backup snapshot` records the deferred issue-side command intent,
  then verifies the fetched `gitclaw-backups` branch can produce a body-free
  lockfile-style snapshot with control-file hashes, issue-payload hashes,
  aggregate counts, gate evidence, and a composite `snapshot_sha256_12`. The
  same harness posts a normal model-backed follow-up that proves repo-reader
  search, prompt provenance, selected skill metadata, prompt-visible tool
  names, usage markers, and the bounded backup-snapshot repository-search
  fixture token.
- A `gh`-driven backup-freshness E2E harness verifies
  `@gitclaw /backup freshness` records the deferred issue-side command intent,
  then verifies the fetched `gitclaw-backups` branch can produce a body-free
  latest-backup freshness report with verify status, max-age seconds, latest
  backup age, payload hash, and `freshness_gate: pass`. The same harness posts
  a normal model-backed follow-up that proves repo-reader search, prompt
  provenance, selected skill metadata, prompt-visible tool names, usage
  markers, and the bounded backup-freshness repository-search fixture token.
- A `gh`-driven backup-continuity E2E harness verifies
  `@gitclaw /backup continuity` records the deferred issue-side command
  intent, then verifies the fetched `gitclaw-backups` branch can produce a
  body-free chronological gap report with verify status, scanned point count,
  longest gap seconds, threshold counts, hash-only gap cards, and
  `continuity_gate: pass`. The same harness posts a normal model-backed
  follow-up that proves repo-reader search, prompt provenance, selected skill
  metadata, prompt-visible tool names, usage markers, and the bounded
  backup-continuity repository-search fixture token.
- A `gh`-driven backup-list E2E harness verifies
  `@gitclaw /backup list` records the deferred issue-side command intent, then
  verifies the fetched `gitclaw-backups` branch can produce a timestamp-sorted
  indexed backup list with paths, counts, event names, and title hashes,
  without dumping raw bodies or titles. The same harness posts a normal
  model-backed follow-up that proves repo-reader search, prompt provenance,
  selected skill metadata, prompt-visible tool names, usage markers, and the
  bounded backup-list repository-search fixture token.
- A `gh`-driven backup-timeline E2E harness verifies
  `@gitclaw /backup timeline` records the deferred issue-side command intent,
  then verifies the fetched `gitclaw-backups` branch can produce a
  chronological, body-free backup timeline with gap seconds, payload hashes,
  assistant-turn counts, and title hashes. It also posts a normal follow-up
  that must use GitHub Models and repo-reader search.
- A `gh`-driven backup-info E2E harness verifies
  `@gitclaw /backup info` records the deferred issue-side command intent, then
  verifies the fetched `gitclaw-backups` branch can produce a focused
  single-issue backup info card with payload hashes, counts, marker totals, and
  body hashes, without dumping raw bodies or titles. The same harness posts a
  normal model-backed follow-up that proves repo-reader search, prompt
  provenance, selected skill metadata, prompt-visible tool names, usage markers,
  and the bounded backup-info repository-search fixture token.
- A `gh`-driven backup-export-jsonl E2E harness verifies
  `@gitclaw /backup export-jsonl` records the deferred issue-side command
  intent, then verifies the fetched `gitclaw-backups` branch can be exported
  into raw JSONL transcript records for one real issue. The same harness posts
  a normal model-backed follow-up that proves repo-reader search, prompt
  provenance, selected skill metadata, prompt-visible tool names, and
  normalized usage markers.
- A `gh`-driven backup-restore-plan E2E harness verifies
  `@gitclaw /backup restore-plan` records the deferred issue-side command
  intent, then verifies the fetched `gitclaw-backups` branch can produce a
  dry-run restore plan for one real issue without dumping raw bodies. The same
  harness posts a normal model-backed follow-up that proves repo-reader search,
  prompt provenance, selected skill metadata, prompt-visible tool names, and
  normalized usage markers.
- A `gh`-driven backup-rehearse-issue E2E harness verifies
  `@gitclaw /backup rehearse` opens or reuses a labeled recovery rehearsal
  issue, checks duplicate suppression and body-free source receipts, verifies
  the fetched `gitclaw-backups` branch with coverage/drill/restore-plan, and
  then posts a normal model-backed follow-up on the rehearsal issue that proves
  repo-reader search, prompt provenance, selected skill metadata,
  prompt-visible tool names, and normalized usage markers.
- A `gh`-driven backup-retention-plan E2E harness verifies
  `@gitclaw /backup retention-plan` records the deferred issue-side command
  intent, then verifies the fetched `gitclaw-backups` branch can produce a
  dry-run keep-latest retention plan with kept/prune-candidate metadata and
  hashes, without dumping raw titles or bodies. The same harness posts a
  normal model-backed follow-up that proves repo-reader search, prompt
  provenance, selected skill metadata, prompt-visible tool names, and
  normalized usage markers.
- A `gh`-driven context-report E2E harness verifies `@gitclaw /context`
  produces a deterministic context summary without a model call.
- A `gh`-driven context-list E2E harness verifies `@gitclaw /context list` is
  an explicit report alias, while local `gitclaw context list` exposes the same
  body-free repository context metadata without issue-only fields.
- A `gh`-driven context-info E2E harness verifies `@gitclaw /context info
  .gitclaw/SOUL.md` returns exactly one focused, body-free context card, while
  local `gitclaw context info <path>` covers both loaded context documents and
  repo files surfaced through deterministic `gitclaw.read_file` metadata.
- A `gh`-driven context-risk E2E harness verifies
  `@gitclaw /context risk` reports body-free context file, explicit reference,
  selected skill, deterministic tool-output, prompt-budget, and runtime gate
  risk metadata. The same harness must then run a real GitHub Models follow-up
  conversation that proves model inference, prompt provenance, selected skills,
  and prompt-visible repository search tool usage.
- A `gh`-driven research-catalog E2E harness verifies
  `@gitclaw /research catalog` maps official OpenClaw/Hermes sources, local
  research-file hashes, adopted GitHub-native patterns, and rejected v1
  surfaces without a model call or body leakage. The same live issue must then
  receive a normal GitHub Models follow-up that selects `repo-reader`, exposes
  `gitclaw.search_files`, recovers the research-catalog repository-search
  fixture token, and publishes prompt provenance plus usage telemetry.
- A `gh`-driven context-references E2E harness verifies
  `@gitclaw /context references` reports `@file:` line ranges and `@folder:`
  metadata without dumping referenced bodies, issue text, or fixture tokens.
- A `gh`-driven context-reference chat E2E harness verifies a normal model turn
  can answer from an explicit `@file:` reference while ignoring a hidden issue
  token. The referenced path must not also be widened through `read_file`; a
  second normal issue-comment turn must select `repo-reader`, expose
  `gitclaw.search_files`, recover a distinct high-entropy repository-search
  fixture token, and avoid hidden sentinel leakage.
- A `gh`-driven search-tool chat E2E harness verifies a normal model turn can
  recover a token from `gitclaw.search_files` output without explicit `@file`
  or `@folder` context references. This keeps the live E2E suite honest about
  actual tool-context usage, not just deterministic reports. The same issue
  must then receive a second normal issue-comment turn with a distinct
  high-entropy search needle, proving `repo-reader` and `gitclaw.search_files`
  remain prompt-visible during continued conversation.
- A `gh`-driven git-reference report E2E harness verifies
  `@gitclaw /context ... @git:1` reports body-free commit-reference metadata
  without dumping commit subjects, patches, or issue text.
- A `gh`-driven git-reference chat E2E harness verifies a normal model turn can
  answer from an explicit `@git:1` reference by copying the latest commit hash,
  then continue in the same issue with a second model-backed repo-reader search
  turn that recovers a distinct high-entropy fixture token and records prompt
  provenance, selected skill metadata, `gitclaw.search_files`, and usage
  telemetry.
- A `gh`-driven prompt-report E2E harness verifies `@gitclaw /prompt`
  produces a deterministic prompt budget, hash, truncation, context, and tool
  metadata report without a model call or prompt/body leakage. The same live
  issue then receives a normal issue-comment follow-up that must make a GitHub
  Models call, select `repo-reader`, expose `gitclaw.search_files`, recover
  the prompt-report repository-search fixture token, and avoid hidden sentinel
  leakage.
- A `gh`-driven prompt-list E2E harness verifies `@gitclaw /prompt list` is an
  explicit report alias, while local `gitclaw prompt list` exposes the same
  body-free prompt-budget, prompt-input, context, skill, and tool metadata
  without issue-only fields. The live alias harness also posts a normal
  GitHub Models follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the prompt-list repository-search fixture
  token, and avoid hidden sentinel leakage.
- A `gh`-driven prompt-risk E2E harness verifies `@gitclaw /prompt risk`
  reports body-free prompt budget, transcript, context contributor, selected
  skill, deterministic tool-output, prompt artifact, and runtime-boundary risk
  metadata. The same harness must then run a real GitHub Models follow-up
  conversation that proves model inference, prompt provenance, selected skills,
  and prompt-visible repository search tool usage.
- A `gh`-driven memory-report E2E harness verifies `@gitclaw /memory`
  produces a deterministic memory inventory without a model call or body
  leakage.
- A `gh`-driven memory-list E2E harness verifies `@gitclaw /memory list`
  is an explicit inventory alias with the same body-free memory-file,
  loaded-note, hash, and validation metadata.
- A `gh`-driven memory-catalog E2E harness verifies
  `@gitclaw /memory catalog` exposes the OpenClaw/Hermes-inspired compact
  memory-layer catalog with durable/procedural/session boundaries,
  prompt-visible/load-mode metadata, reason codes, hashes, validation/risk
  gates, no raw memory/session/prompt/body leakage, and
  `llm_e2e_required_after_memory_catalog_change: true`. It then posts a normal
  issue-comment follow-up that must make a GitHub Models call, select
  `repo-reader`, expose `gitclaw.search_files`, recover a bounded
  repository-search fixture token, and publish usage telemetry.
- A `gh`-driven memory-snapshot E2E harness verifies
  `@gitclaw /memory snapshot` exposes the body-free durable-memory fingerprint,
  composite snapshot hash, prompt-visible load state, validation/risk gates,
  and disabled write/provider/session gates without a model call. It then
  posts a normal issue-comment follow-up that must make a GitHub Models call,
  select `repo-reader`, expose `gitclaw.search_files`, recover the
  memory-snapshot repository-search fixture token, and publish usage telemetry.
- A `gh`-driven memory-provenance E2E harness verifies
  `@gitclaw /memory provenance` maps repo-local memory files to body-free git
  history with tracked/dirty state, commit IDs/dates, subject hashes,
  validation/risk gates, disabled write/provider/background-promotion gates,
  no raw memory/git-subject leakage, and
  `llm_e2e_required_after_memory_provenance_change: true`. It then posts a
  normal issue-comment follow-up that must make a GitHub Models call, select
  `repo-reader`, expose `gitclaw.search_files`, recover a bounded repository
  search fixture token, and publish usage telemetry.
- A `gh`-driven memory-timeline E2E harness verifies
  `@gitclaw /memory timeline` reports repo-local memory chronology, prompt
  visibility, dated-note gaps, validation/risk gates, and hashes without a
  model call or body leakage. It also posts a normal follow-up that must use
  GitHub Models and repo-reader search.
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
  disabled, and includes
  `llm_e2e_required_after_memory_promote_plan_change: true`. The same harness
  then posts a normal issue-comment follow-up that must make a GitHub Models
  call, select `repo-reader`, expose `gitclaw.search_files`, recover a bounded
  repository-search fixture token, and publish usage telemetry without leaking
  hidden issue tokens.
- A `gh`-driven memory-remember E2E harness verifies
  `@gitclaw /memory remember --target long-term --id <id>` opens a body-free
  GitHub memory proposal issue, posts a source receipt, suppresses duplicate
  open proposal issues, avoids raw source/request/candidate leakage, and still
  runs a live GitHub Models follow-up proving repo-local skill selection,
  prompt-visible repository search tool usage, and usage telemetry after the
  deterministic issue action.
- A `gh`-driven memory-remember channel-notify E2E harness verifies
  the notify-route form of `@gitclaw /memory remember` opens the same
  body-free memory proposal issue, queues a reviewed Slack/Telegram channel
  notification through the routebook and channel issue, suppresses duplicate
  outbound comments, exposes pending provider work through `channel-outbox`,
  avoids raw memory/source/route leakage, and still runs a live GitHub Models
  follow-up proving repo-local skill selection, prompt-visible repository
  search tool usage, and usage telemetry.
- A `gh`-driven memory-rehearse E2E harness verifies
  `@gitclaw /memory rehearse --target long-term --id <id>` opens a labeled
  GitHub memory rehearsal issue, posts a source receipt, suppresses duplicate
  open rehearsal issues, avoids raw source/current-memory/candidate leakage,
  and then runs a live GitHub Models follow-up on the rehearsal issue proving
  repo-local skill selection, prompt-visible repository search tool usage, and
  usage telemetry after the deterministic issue action.
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
- A `gh`-driven skills-catalog E2E harness verifies
  `@gitclaw /skills catalog` exposes the compact progressive-disclosure
  catalog with eligibility counts, load modes, reason codes, selected/always
  state, description hashes, body hashes, validation/risk rollups, and
  no-registry/no-install gates without raw description, skill body, issue body,
  prompt, or tool-output leakage. It then posts a normal issue-comment
  follow-up requiring repo-reader search so GitHub Models proves model
  inference, prompt provenance, selected skills, usage telemetry, and
  prompt-visible `gitclaw.search_files`.
- A `gh`-driven skills-provenance E2E harness verifies
  `@gitclaw /skills provenance` exposes tracked git provenance for repo-local
  `SKILL.md` files, including dirty state, commit IDs/dates, and
  commit-subject hashes without raw skill bodies, raw subjects, requirement
  names, installer output, or author identities. It then posts a normal
  follow-up requiring repo-reader search so GitHub Models proves model
  inference, prompt provenance, selected skills, and prompt-visible tool names.
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
  The same live issue then receives a normal issue-comment follow-up that must
  make a GitHub Models call, select `repo-reader`, expose
  `gitclaw.search_files`, recover the skills-select-plan repository-search
  fixture token from a distinct high-entropy needle, avoid explicit
  fixture-file reads, and avoid hidden sentinel leakage.
- A `gh`-driven skills-refresh-plan E2E harness verifies
  `@gitclaw /skills refresh-plan` explains per-turn skill discovery,
  current-checkout snapshot metadata, no resident watcher, no mid-run hot
  reload, no registry polling, no install/update/repo mutation, and no
  issue/comment/prompt/skill body leakage. It then runs a live GitHub Models
  follow-up conversation that proves repo-local skill selection and tool usage
  still work.
- A `gh`-driven skills-snapshot E2E harness verifies
  `@gitclaw /skills snapshot` exposes a body-free composite skill fingerprint
  across repo-local skills, selected prompt-visible skill bodies, skill
  bundles, and reviewed source pins without a model call. The same live issue
  then receives a normal GitHub Models follow-up that must select
  `repo-reader`, expose `gitclaw.search_files`, recover the skills-snapshot
  repository-search fixture token, emit usage telemetry, and avoid hidden
  sentinel leakage.
- A `gh`-driven skills-sources E2E harness verifies
  `@gitclaw /skills sources risk` and local `gitclaw skills sources risk`
  expose body-free source-pin provenance, expected/current skill hashes,
  no-registry/no-fetch/no-install runtime gates, and risk counts, then runs a
  real GitHub Models follow-up conversation that proves repo-local skill
  selection and prompt-visible tool usage still work.
- A `gh`-driven skills-sources-verify E2E harness verifies
  `@gitclaw /skills sources verify` and local
  `gitclaw skills sources verify` expose all reviewed source pins as a
  body-free trust envelope: source-pin hashes, source-ref hashes, current skill
  hashes, risk rollups, registry-not-configured status, static-only
  remote-fetch verification, disabled install verification, and no source or
  skill body leakage. It then runs a real GitHub Models repo-reader/search
  follow-up with prompt, skill, tool, and usage telemetry.
- A `gh`-driven skills-sources-lock E2E harness verifies
  `@gitclaw /skills sources lock` and local
  `gitclaw skills sources lock` expose a body-free reproducibility lock
  derived from reviewed source pins: lock state, stale/unpinned/missing
  counts, expected/current hashes, aggregate lock hash, `.clawhub/lock.json`
  presence/hash, and disabled registry/fetch/install/mutation gates. It then
  runs a real GitHub Models repo-reader/search follow-up with prompt, skill,
  `gitclaw.search_files`, and usage telemetry.
- A `gh`-driven skills-sources-update-plan E2E harness verifies
  `@gitclaw /skills sources update-plan` and local
  `gitclaw skills sources update-plan` expose a body-free no-fetch update
  planner for reviewed source pins: update actions/reasons, stale/unpinned/
  missing/remote/risk counts, source hash state, and disabled registry/fetch/
  install/mutation gates. It then runs a real GitHub Models repo-reader/search
  follow-up with prompt, skill, `gitclaw.search_files`, and usage telemetry.
- A `gh`-driven skills-sources-provenance E2E harness verifies
  `@gitclaw /skills sources provenance` and local
  `gitclaw skills sources provenance` expose body-free source-pin git history,
  tracked/dirty state, commit availability, source-pin gates, and raw-body
  exclusion, then runs a real GitHub Models follow-up conversation that proves
  repo-local skill selection, `gitclaw.search_files`, and usage telemetry.
- A `gh`-driven skills-sources-info E2E harness verifies
  `@gitclaw /skills sources info repo-reader` and local
  `gitclaw skills sources info repo-reader` expose one reviewed source pin
  with body-free source metadata, hash match state, risk rollups,
  no-registry/no-fetch/no-install/no-mutation gates, and the required
  live-LLM follow-up marker. The same live issue then runs a real GitHub
  Models repo-reader/search follow-up with prompt, skill, tool, and usage
  telemetry.
- A `gh`-driven skills-sources-search E2E harness verifies
  `@gitclaw /skills sources search repo-local manual-review` and local
  `gitclaw skills sources search repo-local manual-review` expose body-free
  progressive-disclosure search over reviewed source-pin metadata, hash the
  raw query, report match fields and scores, and exclude source refs, source
  YAML, skill bodies, prompts, issue text, and credentials. The same live
  issue then runs a real GitHub Models repo-reader/search follow-up with
  prompt, skill, `gitclaw.search_files`, and usage telemetry.
- A `gh`-driven skills-runtime E2E harness verifies
  `@gitclaw /skills runtime` and local `gitclaw skills runtime` expose
  body-free OpenClaw-compatible env/bin/install runtime metadata counts,
  hashes, inert install gates, and no raw skill/env/install body leakage. It
  then runs a real GitHub Models follow-up conversation that proves repo-local
  skill selection and prompt-visible repository search tool usage.
- A `gh`-driven skill-bundle provenance E2E harness verifies
  `@gitclaw /bundles provenance` and local `gitclaw bundles provenance`
  expose body-free bundle git history, instruction hashes, tracked/dirty state,
  commit-subject hashes, no agent-authored mutation gates, and no raw
  bundle/skill/issue body leakage. It then runs a real GitHub Models follow-up
  conversation that proves repo-local skill selection and prompt-visible
  repository search tool usage. Its planted no-echo sentinels must use a
  distinct prefix from the expected search-fixture token so the model cannot
  satisfy the E2E by echoing issue/comment context instead of repository search
  context.
- A `gh`-driven skill-bundle catalog E2E harness verifies
  `@gitclaw /bundles catalog` and local `gitclaw bundles catalog` expose
  compact Hermes-style bundle orchestration metadata, selected/load state,
  skill-ref resolution, instruction hashes, reason codes, risk gates, and
  disabled registry/install/mutation gates without raw bundle YAML,
  instruction, skill, issue, prompt, credential, or provider-payload leakage.
  It then runs a real GitHub Models follow-up proving selected `repo-reader`
  context, prompt-context provenance, `gitclaw.search_files`, and usage
  telemetry.
- A `gh`-driven skills-proposal-plan E2E harness verifies
  `@gitclaw /skills proposal-plan repo-reader` produces a body-free,
  non-mutating OpenClaw Skills Workshop-style proposal plan with review paths,
  request hashes, existing skill matches, no autonomous skill creation or
  improvement, no proposal/skill writes, and no raw body leakage. It then runs
  a live GitHub Models follow-up conversation that proves repo-local skill
  selection and prompt-visible repository search tool usage.
- A `gh`-driven skills-propose E2E harness verifies
  `@gitclaw /skills propose <name>` opens a body-free GitHub proposal issue,
  posts a source receipt, suppresses duplicate open proposal issues, avoids
  raw source/request leakage, and still runs a live GitHub Models follow-up
  proving repo-local skill selection, prompt-visible repository search tool
  usage, and usage telemetry after the deterministic issue action.
- A `gh`-driven skills-propose channel-notify E2E harness verifies
  `@gitclaw /skills propose <name> --notify-route <route>` opens the same
  body-free proposal issue, queues a reviewed Slack/Telegram channel
  notification through the routebook and channel issue, suppresses duplicate
  channel outbound comments, exposes pending provider work through
  `channel-outbox`, avoids raw source/request/route leakage in receipts, and
  still runs a live GitHub Models follow-up proving repo-local skill selection,
  prompt-visible repository search tool usage, and usage telemetry.
- A `gh`-driven skills-source-propose E2E harness verifies
  `@gitclaw /skills sources propose <name> --source <ref>` opens a body-free,
  labeled GitHub source-pin proposal issue, hashes the source ref instead of
  copying it, posts a source receipt, suppresses duplicate source proposal
  requests, avoids raw source/request leakage, and then runs the live GitHub
  Models follow-up on the proposal issue itself to prove selected skill
  context, prompt-visible repository search tool usage, and usage telemetry.
- A `gh`-driven skills-source-propose channel-notify E2E harness verifies
  the notify-route form of `@gitclaw /skills sources propose` opens the same
  body-free, labeled source-pin proposal issue, queues a reviewed
  Slack/Telegram channel notification through the routebook and channel issue
  without raw source-ref leakage, suppresses duplicate outbound comments,
  exposes pending provider work through `channel-outbox`, and still runs the
  live GitHub Models follow-up on the proposal issue itself.
- A `gh`-driven skills-rehearse E2E harness verifies
  `@gitclaw /skills rehearse <name> --id <id>` opens a body-free GitHub
  rehearsal issue labeled for normal GitClaw conversation, posts a source
  receipt, suppresses duplicate open rehearsal issues, avoids raw source and
  skill-body leakage, and then runs the live GitHub Models follow-up on the
  rehearsal issue itself to prove selected skill context, prompt-visible
  repository search tool usage, and usage telemetry.
- A `gh`-driven skill-bundle rehearse E2E harness verifies
  `@gitclaw /bundles rehearse <name> --id <id>` opens a body-free GitHub
  rehearsal issue for a Hermes-style task profile, posts a source receipt,
  suppresses duplicate open rehearsal issues, avoids raw source, bundle YAML,
  bundle instruction, and skill-body leakage, and then runs the live GitHub
  Models follow-up on the rehearsal issue itself to prove selected skill
  context, prompt-visible repository search tool usage, and usage telemetry.
- A `gh`-driven skills-proposals E2E harness verifies
  `@gitclaw /skills proposals risk` inventories the repo-local proposal store,
  reports lifecycle counts, risk gates, no auto-apply support, no proposal or
  active-skill mutation, and no raw proposal/skill/issue body leakage. It then
  runs a live GitHub Models follow-up conversation that proves repo-local skill
  selection and prompt-visible repository search tool usage.
- A `gh`-driven skills-install-plan E2E harness verifies
  `@gitclaw /skills install-plan repo-reader` produces a body-free,
  non-mutating install/upgrade plan with remote fetches, installer scripts,
  dependency installs, repository mutations, raw targets, manifests, and skill
  bodies all disabled. The same harness then posts a live GitHub Models
  follow-up proving selected `repo-reader` context, prompt-context provenance,
  `gitclaw.search_files`, and usage telemetry.
- A `gh`-driven skills-upgrade-plan E2E harness verifies
  `@gitclaw /skills upgrade-plan repo-reader` requires an existing repo-local
  skill match, publishes only target/match metadata and hashes, disables
  remote fetches, installer scripts, dependency installs, repository
  mutations, raw targets, manifests, and skill bodies, then posts a live
  GitHub Models follow-up proving selected `repo-reader` context,
  prompt-context provenance, `gitclaw.search_files`, and usage telemetry.
- A `gh`-driven skill-bundle info E2E harness verifies
  `@gitclaw /bundles info repo-context` produces focused repo-local bundle
  metadata, resolved/missing skill refs, instruction presence, hashes, and no
  bundle instruction or skill body leakage.
- A `gh`-driven skill-bundle search E2E harness verifies
  `@gitclaw /bundles search questions` searches repo-local bundle metadata by
  query hash/term count, returns match fields and bundle hashes without raw
  query or body leakage, and pairs the deterministic report with a real GitHub
  Models follow-up that proves selected skills and repo-search tool usage.
- A `gh`-driven skill-bundle risk E2E harness verifies
  `@gitclaw /bundles risk` scans repo-local bundle YAML and instructions for
  body-free risk categories, reports zero findings for the current
  `repo-context` bundle, and pairs the deterministic report with a real GitHub
  Models follow-up that proves selected skills and repo-search tool usage.
- A `gh`-driven skills-search E2E harness verifies
  `@gitclaw /skills search repository context` searches local skill metadata
  without a model call, raw query leakage, or full `SKILL.md` body leakage.
- A `gh`-driven profile-report E2E harness verifies `@gitclaw /profile`
  produces a deterministic repo-local profile envelope across identity,
  memory, skills, tools, model, and validation state without a model call or
  profile-body leakage.
- A `gh`-driven profile-catalog E2E harness verifies
  `@gitclaw /profile catalog` produces a body-free profile command/layer
  inventory across identity, memory, skills, tools, model, proactive, channel,
  backup, and session gates. The same issue then receives a live GitHub Models
  follow-up that proves `repo-reader` selection, prompt-visible
  `gitclaw.search_files`, usage telemetry, and repository-search fixture
  recovery.
- A `gh`-driven profile-provenance E2E harness verifies
  `@gitclaw /profile provenance` produces a body-free git-history map for
  repo-local `.gitclaw/` profile surfaces, including tracked/dirty state, last
  commit IDs/dates, and commit-subject hashes without raw profile bodies,
  commit subjects, or author identities. The same issue then receives a live
  GitHub Models follow-up that proves `repo-reader` selection, prompt-visible
  `gitclaw.search_files`, usage telemetry, and repository-search fixture
  recovery.
- A `gh`-driven profile-search E2E harness verifies
  `@gitclaw /profile search <query>` produces a body-free lexical search
  report over repo-local `.gitclaw/` profile files with query hashes, line
  hashes, scores, and no raw profile, issue, comment, prompt, tool, session,
  backup, credential, or raw query leakage. The same issue then receives a live
  GitHub Models follow-up that proves `repo-reader` selection, prompt-visible
  `gitclaw.search_files`, usage telemetry, and repository-search fixture
  recovery.
- A `gh`-driven profile-diff E2E harness verifies
  `@gitclaw /profile diff [base-ref]` produces a body-free profile git-diff
  report using status/numstat metadata, requested-ref hashes, resolved commit
  hashes, and no raw patches, profile bodies, issue/comment text, prompts,
  requested ref text, git subjects, author identities, sessions, backup
  payloads, credentials, or secrets. The same issue then receives a live GitHub
  Models follow-up that proves `repo-reader` selection, prompt-visible
  `gitclaw.search_files`, usage telemetry, and repository-search fixture
  recovery.
- A `gh`-driven profile-snapshot E2E harness verifies
  `@gitclaw /profile snapshot` produces a body-free composite profile
  fingerprint across the profile manifest, soul snapshot, memory snapshot,
  skill snapshot, and tool snapshot. The same issue then receives a live
  GitHub Models follow-up that proves `repo-reader` selection, prompt-visible
  `gitclaw.search_files`, usage telemetry, and repository-search fixture
  recovery.
- A `gh`-driven profile-manifest E2E harness verifies
  `@gitclaw /profile manifest` produces a body-free portability manifest for
  repo-local profile files, skills, bundles, proactive prompts, toolsets, and
  deterministic tool contracts while excluding credentials, sessions, backup
  payloads, external profile homes, and mutation/install/switch operations. It
  then runs a live GitHub Models follow-up conversation that proves repo-reader
  selection and prompt-visible repository search tool usage.
- A `gh`-driven migration-plan E2E harness verifies
  `@gitclaw /migrate plan hermes` produces a body-free, non-mutating import
  plan for OpenClaw/Hermes/Codex/Claude-style state, with source scanning,
  credential import, executable-state import, repository mutation, and model
  calls disabled. The same issue must then receive a normal issue-comment
  follow-up that makes a GitHub Models call, selects `repo-reader`, exposes
  `gitclaw.search_files`, recovers a distinct high-entropy repository-search
  fixture token, and avoids hidden sentinel leakage.
- A `gh`-driven migration-risk E2E harness verifies
  `@gitclaw /migrate risk hermes` produces a body-free import-boundary risk
  report for OpenClaw/Hermes/Codex/Claude-style state, including credential,
  MCP/plugin/hook, skill, memory, and session-archive classifications. The
  harness also posts a normal follow-up on the same issue and requires a real
  GitHub Models response with repo-reader skill provenance and
  `gitclaw.search_files` tool-output metadata.
- A `gh`-driven soul-report E2E harness verifies `@gitclaw /soul` produces a
  deterministic high-authority context file audit with validation metadata,
  without a model call or body leakage.
- A `gh`-driven soul-list E2E harness verifies `@gitclaw /soul list` is an
  explicit inventory alias with the same body-free context file, memory-note,
  hash, and validation metadata.
- A `gh`-driven soul-catalog E2E harness verifies
  `@gitclaw /soul catalog` exposes the compact body-free authority catalog
  across repo-local soul/profile/memory/policy anchors, including load modes,
  reason codes, authority-layer names, raw-description exclusion, and disabled
  mutation/profile-export gates. It then posts a normal follow-up requiring
  repo-reader search so GitHub Models proves model inference, prompt
  provenance, selected skills, prompt-visible tool names, and usage telemetry.
- A `gh`-driven soul-anchors E2E harness verifies
  `@gitclaw /soul anchors` exposes the body-free authority map across
  repo-local soul/profile/memory/policy anchors, then posts a normal follow-up
  requiring repo-reader search so GitHub Models proves model inference, prompt
  provenance, selected skills, and prompt-visible tool names.
- A `gh`-driven soul-snapshot E2E harness verifies
  `@gitclaw /soul snapshot` exposes the body-free composite fingerprint for
  repo-local high-authority context, including anchor load states, short
  hashes, disabled export/mutation gates, and a `snapshot_sha256_12`. It then
  posts a normal repo-reader/search follow-up so GitHub Models proves model
  inference, prompt provenance, selected skills, prompt-visible tools, and
  usage telemetry.
- A `gh`-driven soul-provenance E2E harness verifies
  `@gitclaw /soul provenance` exposes tracked git provenance for loaded
  high-authority context files, including commit IDs/dates and commit-subject
  hashes without raw bodies, raw subjects, or author identities. It then posts
  a normal follow-up requiring repo-reader search so GitHub Models proves
  model inference, prompt provenance, selected skills, and prompt-visible tool
  names.
- A `gh`-driven soul-info E2E harness verifies `@gitclaw /soul info soul`
  exposes one body-free high-authority context metadata card with normalized
  path, category, repo-local source, selected-for-turn state, hashes, and
  write-disabled metadata.
- A `gh`-driven soul-edit-plan E2E harness verifies
  `@gitclaw /soul edit-plan soul` produces a body-free, non-mutating plan for
  a high-authority context change with edit operations, branch creation,
  commits, pushes, model self-modification, raw targets, raw requested changes,
  and raw context bodies disabled. The same harness then posts a live GitHub
  Models follow-up proving selected `repo-reader` context, prompt-context
  provenance, `gitclaw.search_files`, and usage telemetry.
- A `gh`-driven soul-propose E2E harness verifies
  `@gitclaw /soul propose --target soul --id <id>` opens a body-free GitHub
  soul proposal issue, posts a source receipt, suppresses duplicate open
  proposal issues, avoids raw source/request/candidate leakage, and still runs
  a live GitHub Models follow-up proving repo-local skill selection,
  prompt-visible repository search tool usage, and usage telemetry after the
  deterministic issue action.
- A `gh`-driven soul-propose channel-notify E2E harness verifies
  `@gitclaw /soul propose --target soul --id <id> --notify-route <route>`
  opens or reuses the soul proposal issue, queues exactly one reviewed channel
  notification, exposes it through metadata-only `channel-outbox`, suppresses
  duplicate proposal ids and duplicate channel message IDs, performs no model
  call during the deterministic action, and keeps source/soul bodies out of the
  source receipt, proposal issue, channel receipt, and default outbox. The same
  source issue then gets a normal GitHub Models follow-up that selects
  `repo-reader`, exposes `gitclaw.search_files`, recovers the soul-notify
  fixture token, and records usage telemetry.
- A `gh`-driven soul-rehearse E2E harness verifies
  `@gitclaw /soul rehearse --target soul --id <id>` opens a body-free GitHub
  rehearsal issue labeled for normal GitClaw conversation, posts a source
  receipt, suppresses duplicate open rehearsal issues, avoids raw
  source/target/candidate leakage, and then continues on the rehearsal issue
  itself with a live GitHub Models follow-up proving repo-local skill
  selection, prompt-visible repository search tool usage, and usage telemetry.
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
  leakage. The same live issue then receives a normal issue-comment follow-up
  that must make a GitHub Models call, select `repo-reader`, expose
  `gitclaw.search_files`, recover the tools-report repository-search fixture
  token, and avoid hidden sentinel leakage.
- A `gh`-driven tools-list E2E harness verifies `@gitclaw /tools list`
  is an explicit inventory alias with the same body-free tool contract,
  active-output, guidance, gate-state, and validation metadata. The live alias
  harness also posts a normal GitHub Models follow-up that must select
  `repo-reader`, expose `gitclaw.search_files`, recover the tools-list
  repository-search fixture token, and avoid hidden sentinel leakage.
- A `gh`-driven tools-snapshot E2E harness verifies
  `@gitclaw /tools snapshot` exposes the body-free composite tool fingerprint
  across built-in contracts, repo-reviewed toolsets, MCP allowlists, guidance
  files, and active-output hashes without a model call. The same live issue
  then receives a normal issue-comment follow-up that must make a GitHub
  Models call, select `repo-reader`, expose `gitclaw.search_files`, recover
  the tools-snapshot repository-search fixture token, and avoid hidden
  sentinel leakage.
- A `gh`-driven tools-validate E2E harness verifies
  `@gitclaw /tools validate` exposes the body-free validation report without
  falling back to the full inventory.
- A `gh`-driven tools-verify E2E harness verifies
  `@gitclaw /tools verify` exposes the body-free deterministic tool trust
  envelope, contract modes, gate-state metadata, guidance provenance,
  active-output hashes, raw input suppression, and verification findings
  without a model call. The same live issue then receives a normal
  issue-comment follow-up that must make a GitHub Models call, select
  `repo-reader`, expose `gitclaw.search_files`, recover the tools-verify
  repository-search fixture token, and avoid hidden sentinel leakage.
- A `gh`-driven tools-exposure E2E harness verifies
  `@gitclaw /tools exposure risk` exposes the body-free prompt-visible tool
  exposure boundary, static pre-model context strategy, structured-tool bridge
  non-goals, fail-closed allowlist metadata, raw schema/input/output
  suppression, and the live-LLM E2E requirement. The same harness then posts a
  normal follow-up comment that requires repo-reader search so GitHub Models
  performs a real LLM call with prompt context, selected skill, and
  prompt-visible tool provenance.
- A `gh`-driven tools-defer-plan E2E harness verifies
  `@gitclaw /tools defer-plan` exposes the body-free advisory
  progressive-disclosure plan across built-in tools, repo-reviewed toolsets,
  and MCP allowlists, including threshold metadata, direct/deferred entry
  counts, bridge non-goals, raw schema/body suppression, and the live-LLM E2E
  requirement. The same harness then posts a normal follow-up comment that
  requires repo-reader search so GitHub Models performs a real LLM call with
  prompt context, selected skill, and prompt-visible tool provenance.
- A `gh`-driven tools-boundary E2E harness verifies
  `@gitclaw /tools boundary` exposes the body-free prompt-side tool-output
  boundary, delimiter strategy, prompt-injection scan, hash-only raw
  input/output gates, and the live-LLM E2E requirement. The same harness then
  posts a normal follow-up comment that requires repo-reader search so GitHub
  Models performs a real LLM call with prompt context, selected skill, and
  prompt-visible tool provenance.
- A `gh`-driven tools-approval-plan E2E harness verifies
  `@gitclaw /tools approval-plan search_files` exposes the body-free
  approval/interlock decision for one deterministic tool contract, including
  config/allowlist/mode gates, per-issue label names, current no-approval
  decision for read-only tools, raw input/output/approval-payload suppression,
  and the live-LLM E2E requirement. The same harness then posts a normal
  follow-up comment that requires repo-reader search so GitHub Models performs
  a real LLM call with prompt context, selected skill, prompt-visible tool
  provenance, and usage markers.
- A `gh`-driven tools-provenance E2E harness verifies
  `@gitclaw /tools provenance` exposes the body-free current-turn tool
  provenance map, prompt-visible tool names, input/output hashes, hash-only
  gates, raw body suppression, and the live-LLM E2E requirement. The same
  harness then posts a normal follow-up comment that requires repo-reader
  search so GitHub Models performs a real LLM call with prompt context,
  selected skill, and prompt-visible tool provenance.
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
  execution flags, and explicit reminder coverage. The same harness then posts
  a normal follow-up comment that requires repo-reader search so GitHub Models
  performs a real LLM call with prompt context, selected skill, and
  prompt-visible tool provenance.
- A `gh`-driven tools-rehearse E2E harness verifies
  `@gitclaw /tools rehearse search_files --id <id>` opens or reuses one labeled
  GitHub rehearsal issue marked `gitclaw:tool-rehearsal-issue`, records
  normalized tool, validation, gate, source-hash, and no-execution metadata,
  suppresses duplicate rehearsal ids, performs no model/tool execution,
  generates no raw tool inputs, creates no run request, and keeps source/tool
  bodies out of both the source receipt and rehearsal issue. The rehearsal
  issue then gets a normal GitHub Models issue-comment follow-up that must
  select `repo-reader`, expose `gitclaw.search_files`, recover the
  tools-rehearsal fixture token, and avoid hidden source/tool sentinels.
- A `gh`-driven tools-run-request E2E harness verifies
  `@gitclaw /tools request-run search_files --id <id>` opens or reuses one
  GitHub review issue marked `gitclaw:tool-run-request-issue`, records
  normalized tool, validation, gate, source-hash, and review-decision metadata,
  suppresses duplicate request ids, performs no model/tool execution, and keeps
  source/tool bodies out of both the source receipt and review issue. The same
  harness then posts a normal follow-up comment that requires repo-reader
  search so GitHub Models performs a real LLM call with prompt context,
  selected skill, prompt-visible tool provenance, and usage markers.
- A `gh`-driven tools-run-request channel-notify E2E harness verifies
  `@gitclaw /tools request-run search_files --id <id> --notify-route <route>`
  opens or reuses the review issue, queues exactly one reviewed channel
  notification, exposes it through metadata-only `channel-outbox`, suppresses
  duplicate request ids and duplicate channel message IDs, performs no
  model/tool execution during the deterministic action, and keeps source/tool
  bodies out of the source receipt, review issue, channel receipt, and default
  outbox. The same source issue then gets a normal GitHub Models follow-up that
  selects `repo-reader`, exposes `gitclaw.search_files`, recovers the
  channel-notify fixture token, and records usage telemetry.
- A `gh`-driven sandbox-report E2E harness verifies `@gitclaw /sandbox`
  exposes the current GitHub Actions runtime boundary, denied host exec,
  read-only tool modes, workflow permission cards, and backup-job-only write
  scope without a model call or body leakage.
- A `gh`-driven sandbox-risk E2E harness verifies `@gitclaw /sandbox risk`
  exposes runtime, tool, workflow, skill, backup-concurrency, and raw-body
  leakage risk cards without a model call. The same harness then posts a
  normal follow-up comment that requires repo-reader search so GitHub Models
  performs a real LLM call with prompt context, selected skill metadata, and
  prompt-visible tool provenance.
- A `gh`-driven security-audit E2E harness verifies `@gitclaw /security audit`
  aggregates config, policy, sandbox, channels, tools, skills, plugins, and
  secrets into one body-free OpenClaw-style security posture card without a
  model call or issue-body leakage. The same live issue then receives a normal
  GitHub Models follow-up that must select `repo-reader`, expose
  `gitclaw.search_files`, recover the security-audit repository-search fixture
  token, and avoid hidden issue/comment sentinels.
- A `gh`-driven policy-report E2E harness verifies `@gitclaw /policy` produces
  a deterministic preflight/label/write-policy audit without a model call or
  issue-body leakage.
- A `gh`-driven policy-list E2E harness verifies `@gitclaw /policy list` is an
  explicit report alias, while local `gitclaw policy list` exposes static
  policy metadata without issue-only fields.
- A `gh`-driven policy-verify E2E harness verifies `@gitclaw /policy verify`
  checks the checked-in workflow permission contract, reports active policy
  output hashes, and avoids raw policy input/output leakage.
- A `gh`-driven policy-risk E2E harness verifies `@gitclaw /policy risk`
  exposes body-free trust, managed-label, workflow-permission,
  policy-output-hash, and read-only runtime-boundary risk metadata. The same
  harness must then run a real GitHub Models follow-up conversation that proves
  model inference, prompt provenance, selected skills, and prompt-visible
  repository search tool usage.
- A `gh`-driven approvals-report E2E harness verifies
  `@gitclaw /approvals` detects real write intent, observes
  `gitclaw:approved`, reports the approval gates as read-only, applies
  `gitclaw:write-requested`, and avoids issue body leakage.
- A `gh`-driven approvals-catalog E2E harness verifies
  `@gitclaw /approvals catalog` exposes the compact approval command map,
  approval layers, collision/risk gates, and read-only runtime boundary without
  leaking issue bodies or approval payloads. The same harness must then run a
  real GitHub Models follow-up conversation that proves model inference,
  prompt provenance, selected skills, prompt-visible repository search tool
  usage, and usage markers.
- A `gh`-driven approvals-provenance E2E harness verifies
  `@gitclaw /approvals provenance` runs after a real model-backed seed turn,
  exposes the body-free approval evidence chain, counts current labels,
  transcript messages, and assistant markers, hashes active command and marker
  evidence, suppresses raw bodies/prompts/run URLs/approval payloads, and
  applies `gitclaw:write-requested` when the provenance request contains write
  intent. The same harness must then run another real GitHub Models follow-up
  conversation that proves model inference, prompt provenance, selected skills,
  prompt-visible repository search tool usage, and usage markers.
- A `gh`-driven approvals-risk E2E harness verifies
  `@gitclaw /approvals risk` exposes body-free approval-boundary risk metadata,
  approval/managed-label collision counts, trusted association breadth, and the
  hard read-only runtime gate. The same harness must then run a real GitHub
  Models follow-up conversation that proves model inference, prompt provenance,
  selected skills, and prompt-visible repository search tool usage.
- A `gh`-driven secrets-report E2E harness verifies
  `@gitclaw /secrets audit` scans the real checked-out repository, reports
  plaintext-like findings and GitHub Actions secret references with path, line,
  code, count, and hash metadata only, and does not leak matched values, issue
  body tokens, or referenced secret names.
- A `gh`-driven secrets-risk E2E harness verifies
  `@gitclaw /secrets risk` exposes body-free plaintext-residue,
  secret-reference, runtime-boundary, and apply-boundary risk cards. The same
  harness then runs a real GitHub Models follow-up conversation that proves
  inference, prompt provenance, selected skills, and prompt-visible repository
  search tool usage.
- A `gh`-driven checkpoints-report E2E harness verifies
  `@gitclaw /rollback` inspects the real checked-out repository's git
  checkpoint state, reports HEAD/worktree/backup-branch metadata, and does not
  leak issue body text, diffs, file bodies, commit subjects, or perform restore
  operations. The same live issue then receives a normal issue-comment
  follow-up that must make a GitHub Models call, select `repo-reader`, expose
  `gitclaw.search_files`, recover the checkpoints-report repository-search
  fixture token, and avoid hidden sentinel leakage.
- A `gh`-driven checkpoints-catalog E2E harness verifies
  `@gitclaw /checkpoints catalog` and local `gitclaw checkpoints catalog` /
  `gitclaw rollback catalog` expose body-free checkpoint and rollback commands,
  git/worktree/backup/recent-commit/operation-boundary layers, disabled restore
  gates, and no raw diffs or file bodies. The same issue must then run a real
  GitHub Models follow-up proving model inference, prompt provenance, selected
  skills, prompt-visible repository search tool usage, and usage markers.
- A `gh`-driven rollback-preview E2E harness verifies
  `@gitclaw /rollback diff HEAD~1` and local `gitclaw rollback diff HEAD~1`
  expose body-free numstat/name-status rollback preview metadata, path hashes,
  disabled restore/reset/clean/checkout gates, and no raw diffs, file bodies,
  file paths, or issue-body leakage. The same issue must then run a real
  GitHub Models follow-up proving model inference, prompt provenance, selected
  skills, prompt-visible repository search tool usage, and usage markers.
- A `gh`-driven checkpoints-rehearse E2E harness verifies
  `@gitclaw /checkpoints rehearse --id ... --target HEAD~1` opens or reuses a
  labeled rollback rehearsal issue, keeps the source receipt body-free and
  mutation-free, verifies duplicate suppression, runs local checkpoint preview
  commands, and then continues on the rehearsal issue with a real GitHub Models
  repo-reader/search follow-up that must recover the checkpoint-rehearsal
  fixture token.
- A `gh`-driven checkpoints-risk E2E harness verifies
  `@gitclaw /rollback risk` exposes body-free checkpoint risk metadata, then
  runs a real GitHub Models follow-up conversation that proves model inference,
  prompt provenance, selected skills, and prompt-visible tool usage.
- A `gh`-driven diffs-report E2E harness verifies `@gitclaw /diffs` inspects
  the real checked-out repository's git change metadata, reports policy/spec
  state, clean/dirty state, changed-file counts, numstat totals, and raw-patch
  suppression without leaking issue body text, patch hunks, or file bodies.
- A `gh`-driven diffs-risk E2E harness verifies `@gitclaw /diffs risk` and
  local `gitclaw diffs risk` expose body-free diff policy/spec/worktree risk
  metadata, then runs a real GitHub Models follow-up conversation that proves
  model inference, prompt provenance, selected skills, and prompt-visible tool
  usage.
- A `gh`-driven workspace-report E2E harness verifies `@gitclaw /workspace`
  inspects the real GitHub Actions checkout, reports policy/spec metadata,
  git repository state, context counts, checkout/setup-go action versions,
  fetch-depth metadata, and private-memory/external-mount suppression without
  leaking issue body text, workflow bodies, or file bodies.
- A `gh`-driven workspace-catalog E2E harness verifies
  `@gitclaw /workspace catalog` and local `gitclaw workspace catalog` expose
  body-free workspace command, layer, and gate metadata, then runs a real
  GitHub Models follow-up conversation that proves model inference, prompt
  provenance, selected skills, prompt-visible tool usage, usage telemetry, and
  recovery of the bounded workspace-catalog repository-search fixture token.
- A `gh`-driven workspace-risk E2E harness verifies
  `@gitclaw /workspace risk` and local `gitclaw workspace risk` expose
  body-free workspace policy/spec/workflow risk metadata, then runs a real
  GitHub Models follow-up conversation that proves model inference, prompt
  provenance, selected skills, and prompt-visible tool usage.
- A `gh`-driven session-report E2E harness verifies `@gitclaw /session`
  reconstructs a real multi-turn GitHub issue session without a model call or
  transcript-body leakage.
- A `gh`-driven session-list E2E harness verifies `@gitclaw /session list` is
  an explicit report alias, while local
  `gitclaw session list --backup <issue.json>` inspects a backed-up issue
  session without dumping raw issue, comment, assistant, or transcript bodies.
- A `gh`-driven session-catalog E2E harness verifies
  `@gitclaw /session catalog` publishes a body-free command/gate map with
  `llm_e2e_required_after_session_catalog_change: true`, while local
  `gitclaw session catalog` exposes the same surface without issue metadata.
  The same harness posts a normal GitHub Models repo-reader/search follow-up
  that recovers the bounded session-catalog repository-search fixture token.
- A `gh`-driven session-provenance E2E harness first runs a normal GitHub
  Models repo-reader/search conversation, then verifies
  `@gitclaw /session provenance` reports assistant-turn marker provenance,
  prompt-context hashes, selected skill/tool telemetry, model names, and token
  usage without leaking hidden issue or comment tokens.
- A `gh`-driven session-tools E2E harness first runs a normal GitHub Models
  repo-reader/search conversation, then verifies `@gitclaw /session tools`
  reports the session-level tool ledger, model-backed tool turns,
  prompt-visible tool names, prompt-context evidence, and token usage without
  leaking hidden issue or comment tokens.
- A `gh`-driven session-skills E2E harness first runs a normal GitHub Models
  repo-reader/search conversation, then verifies `@gitclaw /session skills`
  reports the session-level skill ledger, selected repo-reader skill,
  model-backed skill turns, prompt-context evidence, and token usage without
  leaking hidden issue or comment tokens.
- A `gh`-driven session-usage E2E harness first runs a normal GitHub Models
  repo-reader/search conversation, then verifies `@gitclaw /session usage`
  reports normalized prompt/completion/total/cache token telemetry,
  model-backed usage turns, prompt-context evidence, and raw-provider/body-free
  gates without leaking hidden issue or comment tokens.
- A `gh`-driven session-trajectory E2E harness first runs a normal GitHub
  Models repo-reader/search conversation, then verifies
  `@gitclaw /session trajectory` reports an export-like assistant-turn
  manifest with model, run-hash, prompt-context, skill/tool, and usage evidence
  without leaking hidden issue/comment tokens or raw run URLs.
- A `gh`-driven session-compaction E2E harness first runs a normal GitHub
  Models repo-reader/search conversation, then verifies
  `@gitclaw /session compaction` reports Hermes/OpenClaw threshold readiness,
  bounded-transcript cards, model-backed provenance, usage telemetry, and
  disabled summary/mutation gates without leaking hidden issue/comment tokens
  or raw run URLs.
- A `gh`-driven session-resume E2E harness first runs a normal GitHub Models
  repo-reader/search conversation, then verifies `@gitclaw /session resume`
  reports GitHub issue-thread continuation readiness, resume anchors, latest
  assistant marker provenance, usage telemetry, and issue-comment reentry gates
  without leaking hidden issue/comment tokens or raw run URLs.
- A `gh`-driven session-handoff E2E harness first runs a normal GitHub Models
  repo-reader/search conversation, then verifies
  `@gitclaw /session handoff --id <id>` creates or reuses a labeled handoff
  issue, reports body-free duplicate/reentry/session metadata on the source
  issue, and then continues on the handoff issue with another real GitHub
  Models repo-reader/search conversation.
- A `gh`-driven session-stats E2E harness first runs a normal GitHub Models
  conversation with repo-reader and `gitclaw.search_files`, then verifies
  `@gitclaw /session stats` reports model/provenance/session totals without
  leaking hidden issue or comment tokens.
- A `gh`-driven session-status E2E harness first runs a normal GitHub Models
  conversation with repo-reader and `gitclaw.search_files`, then verifies
  `@gitclaw /session status` reports latest-message hashes, labels, model
  provenance, and skill/tool turn counts without leaking hidden issue/comment
  tokens or assistant reply text. The same harness posts a second normal
  follow-up to prove fresh LLM/tool usage after the deterministic readback.
- A `gh`-driven session-coverage E2E harness verifies an actual GitHub Models
  conversation with repo-reader skill and `gitclaw.search_files` tool
  provenance, then checks both issue-side `@gitclaw /session coverage` and
  local `gitclaw session coverage --backup <issue.json> --require-tool
  gitclaw.search_files` against the fetched backup branch.
- Local `gitclaw session search <query> --backup <issue.json>` searches the
  same backed-up issue transcript and reports only query hashes, counts,
  sources, trust metadata, scores, and message/line hashes.
- A `gh`-driven failure E2E harness verifies the safe failure path against a
  real Actions/model failure.
- A `gh`-driven prompt-budget E2E harness verifies a large real issue still
  produces a bounded, correct assistant reply.
- A `gh`-driven prompt-pack E2E harness verifies `@gitclaw /prompt pack`
  reports body-free component order, threshold findings, and truncation
  projection metadata without a model call, then posts a normal GitHub Models
  follow-up that proves selected skill and `gitclaw.search_files` tool usage.
- A `gh`-driven prompt-context E2E harness verifies
  `@gitclaw /prompt context` reports the body-free prompt-context hash,
  context/skill/tool inventory, selected skill snapshot, tool-output hashes,
  and raw-body gates without a model call, then posts a normal GitHub Models
  follow-up that proves selected skill and `gitclaw.search_files` tool usage.
- A `gh`-driven prompt-cache E2E harness verifies `@gitclaw /prompt cache`
  reports body-free stable-prefix/cache-control/telemetry gap metadata without
  a model call, then posts a normal GitHub Models follow-up that proves
  selected skill and `gitclaw.search_files` tool usage.
- A `gh`-driven prompt-compression E2E harness verifies
  `@gitclaw /prompt compression` reports body-free compression thresholds,
  disabled lossy-summary/session-split gates, issue-thread canonical storage,
  backup replay posture, segment hashes, and truncation metadata without a
  model call, then posts a normal GitHub Models follow-up that proves selected
  skill and `gitclaw.search_files` tool usage.
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
- GitHub Models direct-use costs and multipliers: https://docs.github.com/en/billing/reference/costs-for-github-models
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
- OpenClaw prompt caching docs: https://docs.openclaw.ai/reference/prompt-caching
- OpenClaw token use and costs docs: https://docs.openclaw.ai/reference/token-use
- OpenClaw backup docs: https://docs.openclaw.ai/cli/backup
- OpenClaw transcripts CLI docs: https://docs.openclaw.ai/cli/transcripts
- OpenClaw exec approvals docs: https://docs.openclaw.ai/tools/exec-approvals
- OpenClaw sandboxing docs: https://docs.openclaw.ai/gateway/sandboxing
- Hermes tools and toolsets docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/tools/
- Hermes Tool Search docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/tool-search
- Hermes memory docs: https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/memory.md
- Hermes context compression and caching docs: https://hermes-agent.nousresearch.com/docs/developer-guide/context-compression-and-caching/
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
- Hermes Tool Search docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/tool-search
- Hermes MCP docs: https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp
- Slack Socket Mode: https://api.slack.com/apis/connections/socket
- Slack Events API: https://docs.slack.dev/apis/events-api/
- Telegram Bot API `getUpdates`: https://core.telegram.org/bots/api#getupdates
