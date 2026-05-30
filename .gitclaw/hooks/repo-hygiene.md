---
name: repo-hygiene-audit
events:
  - issue:opened
  - proactive:completed
mode: audit-only
delivery: issue-comment
requires_approval: true
---

# Repo Hygiene Audit

This declarative hook records the intended event surface for repo hygiene
checks. It is metadata only; GitClaw does not execute hook handlers in v1.
