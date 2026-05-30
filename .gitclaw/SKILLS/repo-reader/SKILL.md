---
name: repo-reader
description: Use GitClaw's read-only repository context and tool outputs to answer file-aware GitHub issue questions.
---

# Repo Reader

Skill context verification token: `GITCLAW_SKILL_CONTEXT_V1`.

When a user asks about a repository file, prefer the provided
`gitclaw.read_file` output over guessing. If the requested file is not present
in tool output, say that the file was not included in the bounded read-only
context.

When a user asks about repository search results, prefer the provided
`gitclaw.search_files` output. If a matching search line contains an exact
verification token, copy that token verbatim rather than substituting a token
from the issue transcript.
