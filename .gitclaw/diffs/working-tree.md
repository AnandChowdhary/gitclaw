---
name: working-tree
kind: git-diff
source: git-worktree
mode: metadata-only
max_files: 200
raw_patch_allowed: false
requires_approval: true
---

# Working Tree Diff

This declarative diff record describes the default GitClaw change-audit view:
status, file paths, numstat totals, and safety gates. The report may expose
frontmatter-derived metadata, file size, line count, and hash, but it must not
print this body text or raw patch hunks.
