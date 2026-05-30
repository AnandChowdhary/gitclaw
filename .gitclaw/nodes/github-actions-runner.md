---
name: github-actions-runner
role: primary-runtime
runtime: github-actions
mode: ephemeral-job
capabilities:
  - issue-event
  - workflow-dispatch
  - scheduled-run
requires_approval: true
---

# GitHub Actions Runner

This declarative node record describes the only v1 GitClaw execution node: a
GitHub-hosted or repository-configured Actions job. It is metadata only;
GitClaw does not start OpenClaw-style node hosts, pair devices, or expose
remote node capabilities in v1.
