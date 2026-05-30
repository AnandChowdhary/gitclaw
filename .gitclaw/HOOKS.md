# GitClaw Hooks

Hook policy verification token: `GITCLAW_HOOKS_CONTEXT_V1`.

GitClaw hooks are declarative, repo-reviewed event bindings. They describe
where operator automation could attach, but GitClaw does not execute hook
handlers in v1.

Allowed hook behavior:

- Declare lifecycle or issue events that may later wake GitHub Actions.
- Require explicit approval before external side effects.
- Prefer deterministic reports, issue comments, and workflow dispatch over
  local scripts.
- Keep hook bodies and provider payloads out of issue-visible diagnostics.

Disallowed hook behavior:

- Running local shell, JavaScript, Python, or arbitrary handler files during an
  assistant turn.
- Mutating repository files, schedules, labels, or external systems without a
  reviewed workflow and approval gate.
- Treating untrusted issue/comment text as executable hook configuration.
