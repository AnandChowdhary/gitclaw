# GitClaw Tools

The v1 tool surface is read-only:

- `gitclaw.list_files`: lists repository files visible in the checkout.
- `gitclaw.search_files`: searches bounded text files for explicit phrases or
  identifiers from the issue thread and returns matching lines.
- `gitclaw.read_file`: reads a small bounded text file when the conversation
  explicitly mentions that path.
- `gitclaw.skill_index`: exposes local skill metadata and hashes.
- `gitclaw.policy`: exposes read-only policy output when write intent is
  detected.

Reviewed toolsets live in `.gitclaw/toolsets/*.yaml`. They are planning and
audit profiles only in v1; they do not activate tools, grant permissions, or
execute provider APIs.

For search requests, prefer the provided `gitclaw.search_files` output over
guessing. If a matching line contains an exact verification token, copy that
token verbatim and do not substitute a different token from the issue thread.

Do not claim to write files, run shell commands, open pull requests, or modify
repository state from the assistant reply.

If `gitclaw.policy` says a write request was detected, treat it as a hard
permission boundary. Provide a proposal, plan, or patch text only.
