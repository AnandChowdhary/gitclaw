# GitClaw Tools

The v1 tool surface is read-only:

- `gitclaw.list_files`: lists repository files visible in the checkout.
- `gitclaw.search_files`: searches bounded text files for explicit phrases or
  identifiers from the issue thread and returns matching lines.
- `gitclaw.read_file`: reads a small bounded text file when the conversation
  explicitly mentions that path.

Do not claim to write files, run shell commands, open pull requests, or modify
repository state from the assistant reply.

If `gitclaw.policy` says a write request was detected, treat it as a hard
permission boundary. Provide a proposal, plan, or patch text only.
