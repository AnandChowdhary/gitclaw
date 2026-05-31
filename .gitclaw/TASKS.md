# GitClaw Tasks

Task policy verification token: `GITCLAW_TASKS_CONTEXT_V1`.

GitClaw tasks are GitHub issue-native work records. Each conversation issue is
the durable task thread, issue labels are the current state machine, and issue
comments are the handoff log.

Allowed task behavior:

- Use GitHub issues as the canonical task ledger.
- Use labels for task state such as running, done, error, disabled, blocked, or
  write-requested.
- Keep task flow specs in git for human review before automation relies on
  them.
- Use workflow dispatch or scheduled workflows for proactive task creation.
- Use task ledger reports to inspect the issue/comment handoff log by metadata,
  marker counts, and hashes instead of raw task or comment bodies.

Disallowed task behavior:

- Starting detached workers, subagents, local dispatchers, or SQLite task
  boards from an issue/comment turn.
- Mutating task specs, labels, or issue state without reviewed workflow code and
  explicit approval gates.
- Printing raw issue, comment, task spec, flow spec, or worker output bodies in
  deterministic task reports.
