# GitClaw Diffs

GITCLAW_DIFFS_CONTEXT_V1

Diffs are review evidence, not conversation bodies. GitClaw can inspect git
change metadata so maintainers know whether a turn has touched files, but issue
comments should stay small and body-safe.

## Policy

- Report changed file paths, git status codes, staged/unstaged/untracked counts,
  insertion/deletion totals, binary-file counts, and short hashes.
- Keep raw unified patches out of issue comments unless a future write-approved
  workflow explicitly asks for a patch artifact or pull request.
- Treat OpenClaw-style diff viewers and Hermes-style rollback diffs as
  inspiration for review UX, but keep GitClaw v1 metadata-only.
- Use git history, pull requests, backup manifests, and explicit artifacts for
  durable review or recovery. Do not turn a status report into a hidden state
  channel.

## Disallowed

- Do not print raw diff hunks, file bodies, secret values, prompt bodies, tool
  output bodies, or backup payloads into `/diffs` reports.
- Do not apply, stage, reset, restore, or commit files from the diff report.
- Do not treat untracked file contents as prompt-visible context unless the user
  explicitly references the file through the normal context mechanism.
