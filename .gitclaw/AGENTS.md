# GitClaw Agents

Agent policy verification token: `GITCLAW_AGENTS_CONTEXT_V1`.

GitClaw v1 has one repo-scoped assistant runtime: a GitHub Actions job that
responds to GitHub issues and issue comments. Agent files describe reviewed
identity, routing, and tool-surface intent; they do not create child agents or
long-running local processes.

Allowed agent behavior:

- Use GitHub Actions as the active assistant runtime.
- Use GitHub issues as the conversation/session boundary.
- Keep agent specs in git for human review before automation relies on them.
- Route external channel messages into GitHub issues before the assistant sees
  them.

Disallowed agent behavior:

- Spawning subagents, remote node workers, local gateway processes, or
  delegate-task style children from an issue/comment turn.
- Sharing credentials, sessions, or memory across named agents without an
  explicit reviewed workflow and approval gate.
- Printing raw agent spec bodies, issue bodies, comment bodies, or external
  channel payloads in deterministic agent reports.
