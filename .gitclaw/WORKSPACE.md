# Workspace

GITCLAW_WORKSPACE_CONTEXT_V1

GitClaw v1 treats the GitHub Actions repository checkout as the active
workspace. This workspace is ephemeral runner state, not private durable
memory. Durable state belongs in reviewed git-tracked files, issue backups, or
explicit artifacts with their own retention and redaction rules.

Issue-visible workspace reports may show metadata such as policy/spec paths,
file counts, hashes, branch names, workflow checkout settings, and runtime
boundaries. They must not print raw file bodies, prompt bodies, issue/comment
bodies, tool outputs, backup payloads, secrets, or hidden fixture tokens.

The workspace report is inspect-only. It must not change checkout depth, switch
refs, stage files, delete files, write workspace state, clean directories,
mount external paths, or promote runner-local state into memory.

Private workspace memory, external workspace mounts, long-running sockets, and
mutable filesystem workspaces are outside v1. Any future expansion needs a
reviewed spec, explicit permissions, body-free audit output, and a live GitHub
Models conversation E2E in the same implementation batch.
