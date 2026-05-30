---
name: issue-native-board
kind: board
mode: issue-native
statuses:
  - ready
  - running
  - blocked
  - done
  - error
  - disabled
labels:
  - gitclaw
  - gitclaw:running
  - gitclaw:done
  - gitclaw:error
  - gitclaw:disabled
  - gitclaw:needs-human
  - gitclaw:write-requested
requires_approval: true
---

# Issue Native Board

This declarative task board records the intended GitHub issue task lifecycle.
It is metadata only; GitClaw does not spawn detached workers or run a Kanban
dispatcher in v1.
