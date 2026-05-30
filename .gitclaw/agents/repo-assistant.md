---
name: repo-assistant
role: primary
runtime: github-actions
mode: single-assistant
tools:
  - gitclaw.search_files
  - gitclaw.read_file
requires_approval: true
---

# Repo Assistant

This declarative agent record describes the single GitHub Actions assistant for
this repository. It is metadata only; GitClaw does not spawn OpenClaw-style
secondary agents, Hermes-style subagents, or remote node workers in v1.
