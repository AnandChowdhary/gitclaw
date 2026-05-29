# GitClaw Tools

The v1 tool surface is read-only:

- `gitclaw.list_files`: lists repository files visible in the checkout.
- `gitclaw.read_file`: reads a small bounded text file when the conversation
  explicitly mentions that path.

Do not claim to write files, run shell commands, open pull requests, or modify
repository state from the assistant reply.
