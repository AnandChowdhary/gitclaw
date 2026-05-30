---
name: repository-checkout
kind: git-workspace
runtime: github-actions
storage: repository-checkout
mode: metadata-only
root: .
isolation: ephemeral-actions-runner
durable_state: git-tracked-files-and-backup-branch
requires_approval: true
---

# Repository Checkout Workspace

This declarative workspace record describes the default GitHub Actions
repository checkout used by GitClaw. It is metadata for audit reports only and
does not grant write, cleanup, mount, daemon, socket, or private memory powers.
